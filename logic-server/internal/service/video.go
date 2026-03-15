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
	err := database.DB.Where("created_at < ?", t).Order("created_at desc").Limit(30).Find(&videos).Error
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
			Title:         v.Title,
			FavoriteCount: v.FavoriteCount,
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
	err := database.DB.Where("author_id = ?", req.UserId).
		Order("created_at desc").
		Find(&videoModels).Error

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

		// 2. 更新 Video 表的状态 (假设字段名为 status)
		res := tx.Model(&model.Video{}).Where("id = ?", req.VideoId).Update("status", req.Action)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("视频不存在")
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
		Where("follows.user_id = ? ", currentUserID). // 别忘了 status=1 表示审核通过
		Order("videos.created_at DESC").
		Find(&videos).Error

	if err != nil {
		return &video.FollowingFeedResponse{StatusCode: 1, StatusMsg: "获取关注流失败"}, nil
	}

	// 将 model 转换为 proto 格式 (这里通常会封装一个通用转换函数)
	var protoVideos []*video.Video
	for _, v := range videos {
		// 1. 查询作者信息
		var author model.User
		database.DB.First(&author, v.AuthorID)

		// 2. 查询点赞状态 (这是修复刷新消失的关键！)
		var isFavorite bool
		if currentUserID != 0 {
			var count int64
			database.DB.Model(&model.Like{}).
				Where("user_id = ? AND video_id = ?", currentUserID, v.ID).
				Count(&count)
			isFavorite = count > 0
		}

		// 3. 封装完整的视频对象
		protoVideos = append(protoVideos, &video.Video{
			Id:            int64(v.ID),
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			FavoriteCount: v.FavoriteCount,
			CommentCount:  v.CommentCount,
			Title:         v.Title,
			IsFavorite:    isFavorite, // ✅ 加上这个，红心就不会消失了
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

	// 3. 开启事务：删除数据库记录 + 尝试清理 OSS (可选)
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 删除视频记录
		if err := tx.Delete(&videoModel).Error; err != nil {
			return err
		}

		// 还可以顺便删除该视频相关的点赞和评论记录
		tx.Where("video_id = ?", req.VideoId).Delete(&model.Like{})
		// tx.Where("video_id = ?", req.VideoId).Delete(&model.Comment{})

		return nil
	})

	if err != nil {
		return &video.DeleteResponse{StatusCode: 1, StatusMsg: "数据库操作失败"}, nil
	}

	// 4. (可选) 异步清理 OSS 文件，避免浪费空间
	// 需在 oss 包中实现 DeleteFile(objectName string)
	// go oss.DeleteFile(videoModel.PlayURL)

	return &video.DeleteResponse{StatusCode: 0, StatusMsg: "删除成功"}, nil
}
