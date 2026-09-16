package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/logic-server/internal/config"
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/internal/mq"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
)

// 分片上传会话状态放 Redis (带过期)，不为临时状态建表
type uploadSession struct {
	UserID    int64  `json:"user_id"`
	ObjectKey string `json:"object_key"`
	FileMD5   string `json:"file_md5"`
}

func sessionKey(uploadID string) string {
	return fmt.Sprintf("upload:session:%s", uploadID)
}

func saveSession(s *uploadSession, uploadID string) {
	data, _ := json.Marshal(s)
	redis.RDB.Set(context.Background(), sessionKey(uploadID), data, 24*time.Hour)
}

func loadSession(uploadID string) (*uploadSession, error) {
	val, err := redis.RDB.Get(context.Background(), sessionKey(uploadID)).Result()
	if err != nil {
		return nil, err
	}
	var s uploadSession
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// objectKeyForMD5 相同内容的文件永远落到同一个 OSS 对象 (秒传与断点续传都依赖这一点)
func objectKeyForMD5(fileMD5, filename string) string {
	return fmt.Sprintf("videos/%s%s", fileMD5, filepath.Ext(filename))
}

// InitUpload 分片上传第一步：判断秒传 / 断点续传 / 初始化新会话
func (s *VideoService) InitUpload(ctx context.Context, req *video.InitUploadRequest) (*video.InitUploadResponse, error) {
	claims, err := utils.ParseToken(req.Token)
	if err != nil {
		return &video.InitUploadResponse{StatusCode: 1, StatusMsg: "Token无效"}, nil
	}
	if req.FileMd5 == "" || req.FileSize <= 0 {
		return &video.InitUploadResponse{StatusCode: 1, StatusMsg: "缺少文件指纹或大小"}, nil
	}

	// 1. 秒传：库中已有相同指纹的视频，直接复用云上文件
	// (被驳回的视频连同记录会被物理删除，所以查到即为有效源)
	var cnt int64
	database.DB.Model(&model.Video{}).Where("file_md5 = ?", req.FileMd5).Count(&cnt)
	if cnt > 0 {
		return &video.InitUploadResponse{StatusCode: 0, StatusMsg: "秒传命中", Uploaded: true}, nil
	}

	objectKey := objectKeyForMD5(req.FileMd5, req.Filename)

	// 2. 断点续传：尝试恢复上次会话，返回已成功的分片号供客户端跳过
	if req.PrevUploadId != "" {
		if nums, err := oss.ListUploadedParts(req.PrevUploadId, objectKey); err == nil && len(nums) > 0 {
			saveSession(&uploadSession{UserID: claims.UserID, ObjectKey: objectKey, FileMD5: req.FileMd5}, req.PrevUploadId)
			parts := make([]int32, 0, len(nums))
			for _, n := range nums {
				parts = append(parts, int32(n))
			}
			return &video.InitUploadResponse{
				StatusCode:   0,
				StatusMsg:    "已恢复上次进度",
				UploadId:     req.PrevUploadId,
				ExistedParts: parts,
			}, nil
		}
		// 会话在 OSS 侧已过期/不存在，走全新初始化
	}

	// 3. 全新分片会话
	uploadID, err := oss.InitMultipartUpload(objectKey)
	if err != nil {
		return &video.InitUploadResponse{StatusCode: 1, StatusMsg: "初始化分片上传失败: " + err.Error()}, nil
	}
	saveSession(&uploadSession{UserID: claims.UserID, ObjectKey: objectKey, FileMD5: req.FileMd5}, uploadID)

	return &video.InitUploadResponse{StatusCode: 0, StatusMsg: "success", UploadId: uploadID}, nil
}

// UploadPart 分片上传第二步：上传单个分片
func (s *VideoService) UploadPart(ctx context.Context, req *video.UploadPartRequest) (*video.UploadPartResponse, error) {
	claims, err := utils.ParseToken(req.Token)
	if err != nil {
		return &video.UploadPartResponse{StatusCode: 1, StatusMsg: "Token无效"}, nil
	}

	// 会话校验：必须是本人发起的上传
	sess, err := loadSession(req.UploadId)
	if err != nil || sess.UserID != claims.UserID {
		return &video.UploadPartResponse{StatusCode: 1, StatusMsg: "上传会话不存在或已过期，请重新初始化"}, nil
	}

	if err := oss.UploadPart(req.UploadId, sess.ObjectKey, int(req.PartNumber),
		bytes.NewReader(req.Data), int64(len(req.Data))); err != nil {
		return &video.UploadPartResponse{StatusCode: 1, StatusMsg: "分片上传失败: " + err.Error()}, nil
	}

	return &video.UploadPartResponse{StatusCode: 0, StatusMsg: "success"}, nil
}

// CompleteUpload 分片上传第三步：合并分片并落库 (或秒传直接落库)
func (s *VideoService) CompleteUpload(ctx context.Context, req *video.CompleteUploadRequest) (*video.CompleteUploadResponse, error) {
	claims, err := utils.ParseToken(req.Token)
	if err != nil {
		return &video.CompleteUploadResponse{StatusCode: 1, StatusMsg: "Token无效"}, nil
	}

	var playURL, coverURL, fileMD5, hlsURL string

	if req.UploadId == "" {
		// --- 秒传：复用已有视频的云上文件 ---
		var src model.Video
		if err := database.DB.Where("file_md5 = ?", req.FileMd5).First(&src).Error; err != nil {
			return &video.CompleteUploadResponse{StatusCode: 1, StatusMsg: "秒传失败：源文件已不存在，请重新上传"}, nil
		}
		playURL, coverURL, fileMD5, hlsURL = src.PlayURL, src.CoverURL, src.FileMD5, src.HLSURL
	} else {
		// --- 正常分片：合并 → 生成封面 → 落库 ---
		sess, err := loadSession(req.UploadId)
		if err != nil || sess.UserID != claims.UserID {
			return &video.CompleteUploadResponse{StatusCode: 1, StatusMsg: "上传会话不存在或已过期"}, nil
		}

		if err := oss.CompleteMultipartUpload(req.UploadId, sess.ObjectKey); err != nil {
			return &video.CompleteUploadResponse{StatusCode: 1, StatusMsg: "合并分片失败: " + err.Error()}, nil
		}

		playURL = fmt.Sprintf("https://%s.%s/%s",
			config.GlobalConfig.OSS.BucketName, config.GlobalConfig.OSS.Endpoint, sess.ObjectKey)
		// OSS 截帧参数：取第 1 秒画面作为封面
		coverURL = playURL + "?x-oss-process=video/snapshot,t_1000,f_jpg,w_0,h_0,m_fast"
		fileMD5 = sess.FileMD5

		// 会话用完即弃
		redis.RDB.Del(context.Background(), sessionKey(req.UploadId))
	}

	newVideo := model.Video{
		AuthorID: claims.UserID,
		Title:    req.Title,
		PlayURL:  playURL,
		CoverURL: coverURL,
		FileMD5:  fileMD5,
		HLSURL:   hlsURL,
	}
	if err := database.DB.Create(&newVideo).Error; err != nil {
		return &video.CompleteUploadResponse{StatusCode: 1, StatusMsg: "数据库保存失败"}, nil
	}

	// 发布事件进 MQ，由消费者异步扩散，不阻塞用户上传主流程
	go func() {
		if err := mq.PublishVideoMessage(int64(newVideo.ID), claims.UserID, playURL); err != nil {
			fmt.Printf("⚠️ [MQ] 视频发布事件发送失败 (不影响发布结果): %v\n", err)
		}
	}()

	return &video.CompleteUploadResponse{StatusCode: 0, StatusMsg: "发布成功", VideoId: int64(newVideo.ID)}, nil
}
