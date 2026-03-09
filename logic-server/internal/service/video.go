package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"

	"github.com/Lhh220/g-video/api/proto/video"
)

// VideoService 结构体，用于实现 video.proto 定义的接口
type VideoService struct {
	video.UnimplementedVideoServiceServer
}

// PublishVideo 实现发布视频接口
func (s *VideoService) PublishVideo(ctx context.Context, req *video.PublishRequest) (*video.PublishResponse, error) {
	// 1. 生成唯一文件名 (例如: userId_timestamp.mp4)
	objectName := fmt.Sprintf("videos/%d_%d.mp4", req.UserId, time.Now().Unix())

	// 2. 调用 OSS 工具类上传
	// 注意：req.Data 是我们在 proto 里定义的 bytes 类型
	videoURL, err := oss.UploadFile(objectName, bytes.NewReader(req.Data))
	if err != nil {
		return &video.PublishResponse{
			StatusCode: 500,
			StatusMsg:  "上传 OSS 失败: " + err.Error(),
		}, nil
	}

	// 3. 将记录存入 MySQL
	newVideo := model.Video{
		AuthorID: req.UserId,
		Title:    req.Title,
		PlayURL:  videoURL,
		// CoverURL 暂时留空，或者你可以后续集成 ffmpeg 截帧
		CreatedAt: time.Now(),
	}

	if err := database.DB.Create(&newVideo).Error; err != nil {
		return &video.PublishResponse{
			StatusCode: 500,
			StatusMsg:  "数据库写入失败",
		}, nil
	}

	return &video.PublishResponse{
		StatusCode: 0,
		StatusMsg:  "发布成功",
	}, nil
}
