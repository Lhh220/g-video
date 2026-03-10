package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Lhh220/g-video/api/proto/social"
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"gorm.io/gorm"
)

type SocialService struct {
	social.UnimplementedSocialServiceServer
}

func (s *SocialService) FavoriteAction(ctx context.Context, req *social.FavoriteRequest) (*social.FavoriteResponse, error) {
	// 1. 开启事务
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if req.ActionType == 1 { // 点赞
			// 插入点赞记录
			fav := model.Like{UserID: req.UserId, VideoID: req.VideoId}
			if err := tx.Create(&fav).Error; err != nil {
				return err // 如果已经点过赞（唯一索引冲突），这里会报错回滚
			}
			// 视频点赞数 +1 (使用原子操作防止并发冲突)
			if err := tx.Model(&model.Video{}).Where("id = ?", req.VideoId).
				UpdateColumn("favorite_count", gorm.Expr("favorite_count + ?", 1)).Error; err != nil {
				return err
			}
		} else { // 取消点赞
			// 删除点赞记录
			result := tx.Where("user_id = ? AND video_id = ?", req.UserId, req.VideoId).Delete(&model.Like{})
			if result.Error != nil || result.RowsAffected == 0 {
				return errors.New("取消点赞失败或记录不存在")
			}
			// 视频点赞数 -1
			if err := tx.Model(&model.Video{}).Where("id = ?", req.VideoId).
				UpdateColumn("favorite_count", gorm.Expr("favorite_count - ?", 1)).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return &social.FavoriteResponse{StatusCode: 1, StatusMsg: "操作失败: " + err.Error()}, nil
	}

	return &social.FavoriteResponse{StatusCode: 0, StatusMsg: "success"}, nil
}

func (s *SocialService) RelationAction(ctx context.Context, req *social.RelationRequest) (*social.RelationResponse, error) {
	// 1. 不能关注自己
	if req.UserId == req.ToUserId {
		return &social.RelationResponse{StatusCode: 1, StatusMsg: "不能关注自己"}, nil
	}

	// 2. 开启事务
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if req.ActionType == 1 { // 关注
			// A. 插入关系记录
			rel := model.Follow{UserID: req.UserId, ToUserID: req.ToUserId}
			if err := tx.Create(&rel).Error; err != nil {
				return fmt.Errorf("已关注或操作失败")
			}

			// B. 自己关注数 +1
			if err := tx.Model(&model.User{}).Where("id = ?", req.UserId).
				UpdateColumn("follow_count", gorm.Expr("follow_count + ?", 1)).Error; err != nil {
				return err
			}

			// C. 对方粉丝数 +1
			if err := tx.Model(&model.User{}).Where("id = ?", req.ToUserId).
				UpdateColumn("follower_count", gorm.Expr("follower_count + ?", 1)).Error; err != nil {
				return err
			}

		} else if req.ActionType == 2 { // 取消关注
			// A. 删除关系记录
			res := tx.Where("user_id = ? AND to_user_id = ?", req.UserId, req.ToUserId).Delete(&model.Follow{})
			if res.Error != nil || res.RowsAffected == 0 {
				return fmt.Errorf("未关注该用户")
			}

			// B. 自己关注数 -1
			tx.Model(&model.User{}).Where("id = ?", req.UserId).UpdateColumn("follow_count", gorm.Expr("follow_count - ?", 1))
			// C. 对方粉丝数 -1
			tx.Model(&model.User{}).Where("id = ?", req.ToUserId).UpdateColumn("follower_count", gorm.Expr("follower_count - ?", 1))
		}
		return nil
	})

	if err != nil {
		return &social.RelationResponse{StatusCode: 1, StatusMsg: err.Error()}, nil
	}
	return &social.RelationResponse{StatusCode: 0, StatusMsg: "操作成功"}, nil
}
