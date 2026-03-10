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

	// 1. 处理时间锚点 (latest_time)
	t := time.Now()
	if req.LatestTime != 0 {
		t = time.UnixMilli(req.LatestTime)
	}

	// 2. 从数据库查询 (按时间倒序取30条)
	err := database.DB.Where("created_at < ?", t).
		Order("created_at desc").
		Limit(30).
		Find(&videos).Error
	if err != nil {
		return &video.FeedResponse{StatusCode: 1, StatusMsg: "查询失败"}, nil
	}

	// 3. 封装返回数据
	var videoList []*video.Video
	var nextTime int64 = time.Now().UnixMilli()

	for _, v := range videos {
		// 【进阶思考】这里你可以直接查数据库，或者为了规范调用 UserClient 的 RPC 接口
		// 暂时先写死或者简单查下数据库，保证流程通
		var userModel model.User
		database.DB.First(&userModel, v.AuthorID) // 查询作者信息
		videoList = append(videoList, &video.Video{
			Id:            int64(v.ID),
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			FavoriteCount: v.FavoriteCount,
			CommentCount:  v.CommentCount,
			Title:         v.Title,
			Author: &user.User{
				Id:       v.AuthorID,
				Username: userModel.Username, // 之后可以通过 UserClient 查出真实姓名
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
