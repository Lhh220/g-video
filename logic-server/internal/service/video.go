package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
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

	// 2. 从数据库查询视频
	err := database.DB.Where("created_at < ?", t).Order("created_at desc").Limit(30).Find(&videos).Error
	if err != nil {
		return &video.FeedResponse{StatusCode: 1, StatusMsg: "查询失败"}, nil
	}

	// --- 核心修复：解析 Token 获取当前登录用户 ID ---
	var currentUserID int64 = 0
	if req.Token != "" {
		// 这里调用你项目里解析 JWT Token 的函数
		// 假设你的函数叫 ParseToken(token string) (int64, error)
		claims, err := utils.ParseToken(req.Token)
		if err == nil {
			currentUserID = claims.UserID // 拿到真实的当前登录用户ID
		}
	}

	// 3. 封装返回数据
	var videoList []*video.Video
	var nextTime int64 = time.Now().UnixMilli()

	for _, v := range videos {
		var userModel model.User
		database.DB.First(&userModel, v.AuthorID)

		// --- 查询关注状态 ---
		isFollow := false
		if req.Token != "" {
			claims, err := utils.ParseToken(req.Token)
			if err == nil {
				currentUserID = claims.UserID
				// 👈 加这一行，看看控制台打印的是不是 5 (或者你的登录ID)
				fmt.Printf("🎯 [Feed] 解析 Token 成功，当前登录用户 ID: %d\n", currentUserID)
			} else {
				fmt.Printf("⚠️ [Feed] Token 解析失败: %v\n", err)
			}
		}

		if currentUserID != 0 {
			var count int64
			// 使用独立的 tx 句柄查询
			tx := database.DB.Session(&gorm.Session{})
			err := tx.Model(&model.Follow{}).
				Where("user_id = ? AND to_user_id = ?", currentUserID, v.AuthorID).
				Count(&count).Error

			if err != nil {
				fmt.Printf("DEBUG: 查询关注表报错: %v\n", err)
			}
			isFollow = count > 0
			fmt.Printf("DEBUG: 最终关注结果: %v\n", isFollow)
		}
		//查询点赞状态
		isFavorite := false
		if currentUserID != 0 {
			var count int64
			// 使用独立的 tx 句柄查询
			tx := database.DB.Session(&gorm.Session{})
			err := tx.Model(&model.Like{}).
				Where("user_id = ? AND video_id = ?", currentUserID, v.ID).
				Count(&count).Error

			if err != nil {
				fmt.Printf("DEBUG: 查询点赞表报错: %v\n", err)
			}
			isFavorite = count > 0
			fmt.Printf("DEBUG: 最终点赞结果: %v\n", isFavorite)
		}

		videoList = append(videoList, &video.Video{
			Id:      int64(v.ID),
			PlayUrl: v.PlayURL,
			// ... 其他字段保持不变
			Author: &user.User{
				Id:       v.AuthorID,
				Username: userModel.Username,
				Avatar:   userModel.Avatar,
				IsFollow: isFollow,
			},
			IsFavorite: isFavorite,
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

	// 核心逻辑：
	// 1. 在 relations 表中找到所有 user_id = req.UserId 的 to_user_id (即关注的对象)
	// 2. 在 videos 表中找到 author_id 在上述名单中的视频
	// 3. 按时间倒序排列

	err := database.DB.Table("videos").
		Joins("JOIN follows ON follows.to_user_id = videos.author_id").
		Where("follows.user_id = ? AND videos.status = 1", req.UserId). // 别忘了 status=1 表示审核通过
		Order("videos.created_at DESC").
		Find(&videos).Error

	if err != nil {
		return &video.FollowingFeedResponse{StatusCode: 1, StatusMsg: "获取关注流失败"}, nil
	}

	// 将 model 转换为 proto 格式 (这里通常会封装一个通用转换函数)
	var protoVideos []*video.Video
	for _, v := range videos {
		protoVideos = append(protoVideos, &video.Video{
			Id:            int64(v.ID),
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			FavoriteCount: v.FavoriteCount,
			CommentCount:  v.CommentCount,
			Title:         v.Title,
			// 注意：这里可能还需要查询作者的具体信息填充 Author 字段
		})
	}

	return &video.FollowingFeedResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		VideoList:  protoVideos,
	}, nil
}
