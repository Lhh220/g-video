package service

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/internal/mq"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	"gorm.io/gorm"
)

// VideoService 结构体，用于实现 video.proto 定义的接口
type VideoService struct {
	video.UnimplementedVideoServiceServer
}

// PublishVideo 实现发布视频接口
func (s *VideoService) PublishVideo(ctx context.Context, req *video.PublishRequest) (*video.PublishResponse, error) {
	// 1. 鉴权
	claims, err := utils.ParseToken(req.Token)
	if err != nil {
		return &video.PublishResponse{StatusCode: 1, StatusMsg: "Token无效"}, nil
	}

	// 2. 构造视频存储路径
	// 建议格式：videos/用户ID_时间戳_原文件名
	objectName := fmt.Sprintf("videos/%d_%d_%s", claims.UserID, time.Now().Unix(), req.Filename)

	// 3. 上传视频文件
	videoReader := bytes.NewReader(req.Data)
	playUrl, err := oss.UploadFile(objectName, videoReader)
	if err != nil {
		return &video.PublishResponse{StatusCode: 1, StatusMsg: "OSS上传失败"}, nil
	}

	// 4. 【核心改动】利用 OSS 参数自动生成封面 URL
	// t_1000 表示截取第 1000 毫秒（即第1秒）的画面
	coverUrl := playUrl + "?x-oss-process=video/snapshot,t_1000,f_jpg,w_0,h_0,m_fast"

	// 5. 写入数据库
	newVideo := model.Video{
		AuthorID: claims.UserID,
		PlayURL:  playUrl,
		CoverURL: coverUrl,
		Title:    req.Title,
	}

	if err := database.DB.Create(&newVideo).Error; err != nil {
		return &video.PublishResponse{StatusCode: 1, StatusMsg: "数据库保存失败"}, nil
	}

	// 发布事件进 MQ，由消费者异步扩散 (封面兜底/粉丝通知)，不阻塞用户上传主流程
	go func() {
		if err := mq.PublishVideoMessage(int64(newVideo.ID), claims.UserID, playUrl); err != nil {
			fmt.Printf("⚠️ [MQ] 视频发布事件发送失败 (不影响发布结果): %v\n", err)
		}
	}()

	return &video.PublishResponse{StatusCode: 0, StatusMsg: "发布成功"}, nil
}

func (s *VideoService) Feed(ctx context.Context, req *video.FeedRequest) (*video.FeedResponse, error) {
	var videos []model.Video

	// 1. 处理时间锚点
	t := time.Now()
	if req.LatestTime != 0 {
		t = time.UnixMilli(req.LatestTime)
	}

	// 2. 从数据库查询视频列表 (主表查询暂不建议放 Redis，除非是极热门榜单)
	// status = 1 才是审核通过的视频，待审(0)/驳回(2)绝不能出现在公共流里
	err := database.DB.Where("created_at < ? AND status = ?", t, 1).Order("created_at desc").Limit(30).Find(&videos).Error
	if err != nil {
		return &video.FeedResponse{StatusCode: 1, StatusMsg: "查询失败"}, nil
	}

	// 3. 鉴权并预取社交状态
	var currentUserID int64 = 0
	// 使用 Map 存储预取的结果，方便在循环中 O(1) 查找
	followingMap := make(map[int64]bool)
	favoriteMap := make(map[int64]bool)

	if req.Token != "" {
		claims, err := utils.ParseToken(req.Token)
		if err == nil {
			currentUserID = claims.UserID
			fmt.Printf("🎯 [Feed] 登录用户: %d，准备从 Redis 获取社交状态\n", currentUserID)

			// --- 【核心优化】从 Redis 批量获取该用户关注的人 ---
			followKey := fmt.Sprintf("user:following:%d", currentUserID)
			followIDs, _ := redis.RDB.SMembers(ctx, followKey).Result()
			for _, idStr := range followIDs {
				id, _ := strconv.ParseInt(idStr, 10, 64)
				followingMap[id] = true
			}

			// --- 【核心优化】从 Redis 批量获取该用户点赞过的视频 ---
			favoriteKey := fmt.Sprintf("user:liked:videos:%d", currentUserID)
			favIDs, _ := redis.RDB.SMembers(ctx, favoriteKey).Result()
			for _, idStr := range favIDs {
				id, _ := strconv.ParseInt(idStr, 10, 64)
				favoriteMap[id] = true
			}
		}
	}

	var videoList []*video.Video
	var nextTime int64 = time.Now().UnixMilli()

	// 读己之写：把尚未落库的 Redis 计数增量合并进展示值 (一次 MGET，无循环查库)
	videoIDs := make([]int64, 0, len(videos))
	for _, v := range videos {
		videoIDs = append(videoIDs, int64(v.ID))
	}
	favoriteDeltaMap := getFavoriteDeltas(ctx, videoIDs)

	// 4. 循环封装数据 (现在的循环里不再有任何 follows 和 likes 的 SQL)
	for _, v := range videos {
		// 获取作者信息 (优先走 Redis 缓存)
		authorInfo, err := GetUserWithCache(ctx, v.AuthorID)
		if err != nil {
			authorInfo = &user.User{Id: v.AuthorID, Username: "未知用户"}
		}

		// 直接从刚才准备好的 Map 里取状态，无需查数据库
		isFollow := followingMap[v.AuthorID]
		isFavorite := favoriteMap[int64(v.ID)]

		videoList = append(videoList, &video.Video{
			Id:            int64(v.ID),
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			HlsUrl:        v.HLSURL,
			Title:         v.Title,
			FavoriteCount: v.FavoriteCount + favoriteDeltaMap[int64(v.ID)],
			CommentCount:  v.CommentCount,
			IsFavorite:    isFavorite, // ✅ Redis 内存获取
			Author: &user.User{
				Id:            authorInfo.Id,
				Username:      authorInfo.Username,
				Avatar:        authorInfo.Avatar,
				FollowCount:   authorInfo.FollowCount,
				FollowerCount: authorInfo.FollowerCount,
				IsFollow:      isFollow, // ✅ Redis 内存获取
			},
		})
		nextTime = v.CreatedAt.UnixMilli()
	}

	return &video.FeedResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		VideoList:  videoList,
		NextTime:   nextTime,
	}, nil
}

func (s *VideoService) GetPublishList(ctx context.Context, req *video.PublishListRequest) (*video.PublishListResponse, error) {
	var videoModels []model.Video

	// 1. 根据传入的 user_id 查询该用户的所有视频
	// 注意：这里不需要用 latest_time 过滤，通常是一次性展示（或按需分页）
	// 他人查看时只返回审核通过的作品；作者本人可以看到自己的待审/驳回视频
	isOwner := false
	if req.Token != "" {
		if claims, err := utils.ParseToken(req.Token); err == nil && claims.UserID == req.UserId {
			isOwner = true
		}
	}
	query := database.DB.Where("author_id = ?", req.UserId)
	if !isOwner {
		query = query.Where("status = ?", 1)
	}
	err := query.Order("created_at desc").Find(&videoModels).Error

	if err != nil {
		return &video.PublishListResponse{StatusCode: 1, StatusMsg: "查询列表失败"}, nil
	}

	// 2. 批量查询用户信息（复用你刚才想修的“真实用户名”逻辑）
	// 因为是同一个人的列表，直接查一次该 User 即可
	var userModel model.User
	database.DB.First(&userModel, req.UserId)

	// 3. 组装返回列表
	var videoList []*video.Video
	for _, v := range videoModels {
		videoList = append(videoList, &video.Video{
			Id: int64(v.ID),
			Author: &user.User{
				Id:       int64(userModel.ID),
				Username: userModel.Username,
				Avatar:   userModel.Avatar,
			},
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			HlsUrl:        v.HLSURL,
			FavoriteCount: v.FavoriteCount,
			CommentCount:  v.CommentCount,
			Title:         v.Title,
		})
	}

	return &video.PublishListResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		VideoList:  videoList,
	}, nil
}

func (s *VideoService) AuditVideo(ctx context.Context, req *video.AuditRequest) (*video.AuditResponse, error) {
	// 驳回时需要抹除云端文件，事务前先取出视频的 OSS 地址
	var playURL, hlsURL string
	if req.Action == 2 {
		var v model.Video
		if err := database.DB.First(&v, req.VideoId).Error; err != nil {
			return &video.AuditResponse{StatusCode: 1, StatusMsg: "视频不存在"}, nil
		}
		playURL = v.PlayURL
		hlsURL = v.HLSURL
	}

	// 使用事务包裹整个审核过程
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 权限校验
		var admin model.User
		if err := tx.First(&admin, req.AdminId).Error; err != nil {
			return fmt.Errorf("管理员不存在")
		}
		if admin.Role != 1 {
			return fmt.Errorf("权限不足，非管理员身份")
		}

		if req.Action == 2 {
			// 2a. 驳回：视频连同点赞、评论从数据库永久抹除
			res := tx.Unscoped().Where("id = ?", req.VideoId).Delete(&model.Video{})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("视频不存在")
			}
			tx.Unscoped().Where("video_id = ?", req.VideoId).Delete(&model.Like{})
			tx.Unscoped().Where("video_id = ?", req.VideoId).Delete(&model.Comment{})
		} else {
			// 2b. 通过：更新 Video 表的状态为已发布
			res := tx.Model(&model.Video{}).Where("id = ?", req.VideoId).Update("status", req.Action)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return fmt.Errorf("视频不存在")
			}
		}

		// 3. 写入 AuditLog 审核日志
		auditLog := model.AuditLog{
			VideoID:   req.VideoId,
			AdminID:   req.AdminId,
			Action:    req.Action,
			Reason:    req.Reason,
			CreatedAt: time.Now(), // 确保 model 里 import 了 time
		}
		if err := tx.Create(&auditLog).Error; err != nil {
			return fmt.Errorf("写入审核日志失败: %v", err)
		}

		return nil
	})

	if err != nil {
		return &video.AuditResponse{StatusCode: 1, StatusMsg: err.Error()}, nil
	}

	// 4. 驳回的事务提交后，同步删除 OSS 云端文件
	if req.Action == 2 && playURL != "" {
		if err := oss.DeleteFileByURL(playURL); err != nil {
			return &video.AuditResponse{
				StatusCode: 1,
				StatusMsg:  "审核已记录，但云端文件清理失败: " + err.Error(),
			}, nil
		}
		if hlsURL != "" {
			if err := oss.DeletePrefixByHLSURL(hlsURL); err != nil {
				return &video.AuditResponse{
					StatusCode: 1,
					StatusMsg:  "审核已记录，但 HLS 切片清理失败: " + err.Error(),
				}, nil
			}
		}
	}

	return &video.AuditResponse{
		StatusCode: 0,
		StatusMsg:  "审核成功并已记录日志",
	}, nil
}

// 获取关注用户的视频流 (FollowingFeed)
func (s *VideoService) FollowingFeed(ctx context.Context, req *video.FollowingFeedRequest) (*video.FollowingFeedResponse, error) {
	var videos []model.Video

	var currentUserID int64 = 0
	if req.Token != "" {
		// 这里调用你项目里解析 JWT Token 的函数
		// 假设你的函数叫 ParseToken(token string) (int64, error)
		claims, err := utils.ParseToken(req.Token)
		if err == nil {
			currentUserID = claims.UserID // 拿到真实的当前登录用户ID
		}
	}

	// 核心逻辑：
	// 1. 在 relations 表中找到所有 user_id = req.UserId 的 to_user_id (即关注的对象)
	// 2. 在 videos 表中找到 author_id 在上述名单中的视频
	// 3. 按时间倒序排列
	fmt.Printf("DEBUG: 当前用户ID: %v\n", currentUserID)
	err := database.DB.Table("videos").
		Joins("JOIN follows ON follows.to_user_id = videos.author_id").
		// status = 1 只展示审核通过的视频
		Where("follows.user_id = ? AND videos.status = ?", currentUserID, 1).
		Order("videos.created_at DESC").
		Find(&videos).Error

	if err != nil {
		return &video.FollowingFeedResponse{StatusCode: 1, StatusMsg: "获取关注流失败"}, nil
	}

	// 【性能】批量预取作者信息与点赞状态，消除循环内 N+1 查询
	// 旧实现每个视频要发 2 条 SQL (查作者 + 查点赞计数)，300 条视频就是 600 次数据库往返
	videoIDs := make([]int64, 0, len(videos))
	authorIDSet := make(map[int64]struct{}, len(videos))
	for _, v := range videos {
		videoIDs = append(videoIDs, int64(v.ID))
		authorIDSet[v.AuthorID] = struct{}{}
	}
	uniqueAuthorIDs := make([]int64, 0, len(authorIDSet))
	for id := range authorIDSet {
		uniqueAuthorIDs = append(uniqueAuthorIDs, id)
	}

	// 一次查出所有涉及作者
	var authors []model.User
	database.DB.Where("id IN ?", uniqueAuthorIDs).Find(&authors)
	authorMap := make(map[int64]*model.User, len(authors))
	for i := range authors {
		authorMap[authors[i].ID] = &authors[i]
	}

	// 一次查出当前用户在这些视频上的点赞记录
	favoriteMap := make(map[int64]bool, len(videos))
	if currentUserID != 0 && len(videoIDs) > 0 {
		var likes []model.Like
		database.DB.Where("user_id = ? AND video_id IN ?", currentUserID, videoIDs).Find(&likes)
		for _, l := range likes {
			favoriteMap[int64(l.VideoID)] = true
		}
	}

	// 组装返回 (循环内零 SQL)
	var protoVideos []*video.Video
	for _, v := range videos {
		author := authorMap[v.AuthorID]
		if author == nil {
			author = &model.User{ID: v.AuthorID, Username: "未知用户"}
		}

		protoVideos = append(protoVideos, &video.Video{
			Id:            int64(v.ID),
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			HlsUrl:        v.HLSURL,
			FavoriteCount: v.FavoriteCount,
			CommentCount:  v.CommentCount,
			Title:         v.Title,
			IsFavorite:    favoriteMap[int64(v.ID)], // ✅ 批量预取，红心不会消失
			Author: &user.User{
				Id:       int64(author.ID),
				Username: author.Username,
				Avatar:   author.Avatar,
				IsFollow: true, // ✅ 既然在关注流里，那肯定已经关注了
			},
		})
	}

	return &video.FollowingFeedResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		VideoList:  protoVideos,
	}, nil
}

// ListPendingVideos 管理员后台：分页拉取待审核 (status=0) 的视频
func (s *VideoService) ListPendingVideos(ctx context.Context, req *video.PendingListRequest) (*video.PendingListResponse, error) {
	// 1. 鉴权：Web 层已拦截非管理员，这里再校验一次做纵深防御
	var admin model.User
	if err := database.DB.First(&admin, req.AdminId).Error; err != nil || admin.Role != 1 {
		return &video.PendingListResponse{StatusCode: 1, StatusMsg: "无管理员权限"}, nil
	}

	// 2. 分页参数兜底
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	// 3. 总数 + 分页查询，按投稿时间正序 (先投稿先审核)
	var total int64
	database.DB.Model(&model.Video{}).Where("status = ?", 0).Count(&total)

	var videos []model.Video
	if err := database.DB.Where("status = ?", 0).
		Order("created_at asc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&videos).Error; err != nil {
		return &video.PendingListResponse{StatusCode: 1, StatusMsg: "查询失败"}, nil
	}

	// 4. 组装，作者信息走缓存
	var videoList []*video.Video
	for _, v := range videos {
		authorInfo, err := GetUserWithCache(ctx, v.AuthorID)
		if err != nil {
			authorInfo = &user.User{Id: v.AuthorID, Username: "未知用户"}
		}
		videoList = append(videoList, &video.Video{
			Id:            int64(v.ID),
			Title:         v.Title,
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			HlsUrl:        v.HLSURL,
			FavoriteCount: v.FavoriteCount,
			CommentCount:  v.CommentCount,
			Status:        v.Status,
			Author:        authorInfo,
		})
	}

	return &video.PendingListResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		VideoList:  videoList,
		Total:      total,
	}, nil
}

// DeleteVideo 实现删除视频接口
func (s *VideoService) DeleteVideo(ctx context.Context, req *video.DeleteRequest) (*video.DeleteResponse, error) {
	var videoModel model.Video

	// 1. 查询视频信息
	if err := database.DB.First(&videoModel, req.VideoId).Error; err != nil {
		return &video.DeleteResponse{StatusCode: 1, StatusMsg: "视频不存在"}, nil
	}

	// 2. 鉴权：只有作者本人可以删除
	if videoModel.AuthorID != req.UserId {
		return &video.DeleteResponse{StatusCode: 1, StatusMsg: "无权删除他人视频"}, nil
	}

	// 3. 开启事务：物理删除视频记录 + 一并清理点赞、评论，避免孤儿数据
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Unscoped 绕过软删除，做到数据库记录永久移除
		if err := tx.Unscoped().Delete(&videoModel).Error; err != nil {
			return err
		}

		tx.Unscoped().Where("video_id = ?", req.VideoId).Delete(&model.Like{})
		tx.Unscoped().Where("video_id = ?", req.VideoId).Delete(&model.Comment{})

		return nil
	})

	if err != nil {
		return &video.DeleteResponse{StatusCode: 1, StatusMsg: "数据库操作失败"}, nil
	}

	// 4. 数据库删除成功后，同步删除 OSS 云端文件，确保存储空间不浪费
	if err := oss.DeleteFileByURL(videoModel.PlayURL); err != nil {
		// 记录已删除，仅云端清理失败：如实告知，方便人工补救
		return &video.DeleteResponse{
			StatusCode: 1,
			StatusMsg:  "记录已删除，但云端文件清理失败: " + err.Error(),
		}, nil
	}
	// HLS 切片目录一并清理 (未转码的视频为空，跳过)
	if videoModel.HLSURL != "" {
		if err := oss.DeletePrefixByHLSURL(videoModel.HLSURL); err != nil {
			return &video.DeleteResponse{
				StatusCode: 1,
				StatusMsg:  "记录已删除，但 HLS 切片清理失败: " + err.Error(),
			}, nil
		}
	}

	return &video.DeleteResponse{StatusCode: 0, StatusMsg: "删除成功"}, nil
}
