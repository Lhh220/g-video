package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Lhh220/g-video/api/proto/social"
	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SocialService struct {
	social.UnimplementedSocialServiceServer
}

func (s *SocialService) FavoriteAction(ctx context.Context, req *social.FavoriteRequest) (*social.FavoriteResponse, error) {
	// 1. 处理数据库事务
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if req.ActionType == 1 { // 点赞
			fav := model.Like{UserID: req.UserId, VideoID: req.VideoId}

			// --- 【改进点】使用 OnConflict 处理重复插入 ---
			// 如果 user_id 和 video_id 已经存在，则什么都不做（DoNothing），不再报错 1062
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fav)
			if result.Error != nil {
				return result.Error
			}

			// 只有在真正插入了新行的情况下，才给视频点赞数 +1
			// 如果 RowsAffected 为 0，说明之前已经点过赞了，不再重复加数
			if result.RowsAffected > 0 {
				if err := tx.Model(&model.Video{}).Where("id = ?", req.VideoId).
					UpdateColumn("favorite_count", gorm.Expr("favorite_count + ?", 1)).Error; err != nil {
					return err
				}
			}
		} else { // 取消点赞
			// 删除点赞记录
			result := tx.Where("user_id = ? AND video_id = ?", req.UserId, req.VideoId).Delete(&model.Like{})
			if result.Error != nil {
				return result.Error
			}

			// 只有在真正删除了行的情况下，才给视频点赞数 -1
			if result.RowsAffected > 0 {
				if err := tx.Model(&model.Video{}).Where("id = ?", req.VideoId).
					UpdateColumn("favorite_count", gorm.Expr("favorite_count - ?", 1)).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})

	// 2. 检查数据库操作结果
	if err != nil {
		return &social.FavoriteResponse{
			StatusCode: 1,
			StatusMsg:  "操作失败: " + err.Error(),
		}, nil
	}

	// 3. --- 【核心改进】无论 DB 是新插还是已存在，强行同步 Redis ---
	// 这样可以确保只要用户点赞了，Redis 里的红心就一定会亮
	favoriteKey := fmt.Sprintf("user:liked:videos:%d", req.UserId)

	go func() {
		// 使用 context.Background() 确保主请求结束后协程仍能运行
		if req.ActionType == 1 {
			redis.RDB.SAdd(context.Background(), favoriteKey, req.VideoId)
			fmt.Printf("✅ [Redis Sync] 确保点赞状态同步: User %d, Video %d\n", req.UserId, req.VideoId)
		} else {
			redis.RDB.SRem(context.Background(), favoriteKey, req.VideoId)
			fmt.Printf("🗑️ [Redis Sync] 确保取消点赞同步: User %d, Video %d\n", req.UserId, req.VideoId)
		}
		// 设置 7 天有效期
		redis.RDB.Expire(context.Background(), favoriteKey, 7*24*time.Hour)
	}()

	return &social.FavoriteResponse{StatusCode: 0, StatusMsg: "success"}, nil
}
func (s *SocialService) RelationAction(ctx context.Context, req *social.RelationRequest) (*social.RelationResponse, error) {
	// 1. 不能关注自己
	if req.UserId == req.ToUserId {
		return &social.RelationResponse{StatusCode: 1, StatusMsg: "不能关注自己"}, nil
	}

	// 2. 开启事务处理数据库
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if req.ActionType == 1 { // 关注
			rel := model.Follow{UserID: req.UserId, ToUserID: req.ToUserId}

			// --- 【核心改进】使用 OnConflict 忽略唯一索引冲突 ---
			// 哪怕数据库里已经有关注记录，也不再报错，而是平滑向下执行
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rel)
			if result.Error != nil {
				return result.Error
			}

			// 只有在真正插入新记录（之前没关注过）时，才更新计数器
			if result.RowsAffected > 0 {
				// 自己关注数 +1
				if err := tx.Model(&model.User{}).Where("id = ?", req.UserId).
					UpdateColumn("follow_count", gorm.Expr("follow_count + ?", 1)).Error; err != nil {
					return err
				}
				// 对方粉丝数 +1
				if err := tx.Model(&model.User{}).Where("id = ?", req.ToUserId).
					UpdateColumn("follower_count", gorm.Expr("follower_count + ?", 1)).Error; err != nil {
					return err
				}
			}

		} else if req.ActionType == 2 { // 取消关注
			// 删除关系记录
			res := tx.Where("user_id = ? AND to_user_id = ?", req.UserId, req.ToUserId).Delete(&model.Follow{})
			if res.Error != nil {
				return res.Error
			}

			// 只有真正删除了记录，才减少计数
			if res.RowsAffected > 0 {
				tx.Model(&model.User{}).Where("id = ?", req.UserId).UpdateColumn("follow_count", gorm.Expr("follow_count - ?", 1))
				tx.Model(&model.User{}).Where("id = ?", req.ToUserId).UpdateColumn("follower_count", gorm.Expr("follower_count - ?", 1))
			}
		}
		return nil
	})

	// 3. 检查数据库事务结果
	if err != nil {
		return &social.RelationResponse{StatusCode: 1, StatusMsg: "操作失败: " + err.Error()}, nil
	}

	// --- 【核心改进】无论数据库是新插还是已存在，强行同步 Redis Set ---
	// 这样 Feed 流里的 followingMap 才能拿到正确的关注 ID，从而隐藏红色“+”号
	followKey := fmt.Sprintf("user:following:%d", req.UserId)

	go func() {
		// 使用 context.Background() 确保请求结束后协程仍能完成写入
		if req.ActionType == 1 {
			redis.RDB.SAdd(context.Background(), followKey, req.ToUserId)
			fmt.Printf("✅ [Follow Sync] 强刷关注状态: 用户 %d -> %d\n", req.UserId, req.ToUserId)
		} else {
			redis.RDB.SRem(context.Background(), followKey, req.ToUserId)
			fmt.Printf("🗑️ [Follow Sync] 移除关注状态: 用户 %d -> %d\n", req.UserId, req.ToUserId)
		}
		// 关注列表缓存设置 7 天有效期
		redis.RDB.Expire(context.Background(), followKey, 7*24*time.Hour)
	}()

	return &social.RelationResponse{StatusCode: 0, StatusMsg: "操作成功"}, nil
}

func (s *SocialService) CommentAction(ctx context.Context, req *social.CommentRequest) (*social.CommentResponse, error) {
	var newComment model.Comment

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if req.ActionType == 1 { // 1-发布
			// 参数检查：内容不能为空
			if req.CommentText == "" {
				return fmt.Errorf("评论内容不能为空")
			}
			newComment = model.Comment{
				UserID:  req.UserId,
				VideoID: req.VideoId,
				Content: req.CommentText,
			}
			if err := tx.Create(&newComment).Error; err != nil {
				return err
			}
			// 视频评论数 +1
			return tx.Model(&model.Video{}).Where("id = ?", req.VideoId).
				UpdateColumn("comment_count", gorm.Expr("comment_count + ?", 1)).Error

		} else if req.ActionType == 2 { // 2-删除
			// 1. 安全删除评论
			result := tx.Where("id = ? AND user_id = ?", req.CommentId, req.UserId).Delete(&model.Comment{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("评论不存在或无权删除")
			}

			// 2. 视频评论数 -1 (使用 GREATEST 避免出现 -1)
			// 这样即便原本是 0，减 1 后也会取 0 和 -1 之间的最大值，即 0
			return tx.Model(&model.Video{}).Where("id = ?", req.VideoId).
				UpdateColumn("comment_count", gorm.Expr("GREATEST(comment_count - 1, 0)")).Error
		}
		return fmt.Errorf("未定义的动作类型")
	})

	if err != nil {
		return &social.CommentResponse{StatusCode: 1, StatusMsg: err.Error()}, nil
	}

	// 如果是删除操作 (ActionType == 2)，返回体里的 Comment 可以传 nil
	var respComment *social.Comment
	if req.ActionType == 1 {
		var u model.User
		database.DB.First(&u, req.UserId)
		respComment = &social.Comment{
			Id:         int64(newComment.ID),
			Content:    newComment.Content,
			CreateDate: newComment.CreatedAt.Format("01-02"),
			User: &user.User{
				Id:            int64(u.ID),
				Username:      u.Username,
				Avatar:        u.Avatar,
				FollowCount:   u.FollowCount,
				FollowerCount: u.FollowerCount,
			},
		}
	}

	return &social.CommentResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Comment:    respComment,
	}, nil
}

func (s *SocialService) CommentList(ctx context.Context, req *social.CommentListRequest) (*social.CommentListResponse, error) {
	var comments []model.Comment

	// 1. 从数据库查询该视频下的所有评论，按时间倒序排
	// 使用 Preload("User") 可以关联查询出评论者的基本信息
	err := database.DB.Where("video_id = ?", req.VideoId).
		Order("created_at desc").
		Find(&comments).Error

	if err != nil {
		return &social.CommentListResponse{StatusCode: 1, StatusMsg: "获取评论列表失败"}, nil
	}

	// 2. 转换为 protobuf 要求的格式
	var protoComments []*social.Comment
	for _, c := range comments {
		// 这里需要根据 c.UserID 去查用户信息，或者在第一步直接用 Join/Preload
		var u model.User
		database.DB.First(&u, c.UserID)

		protoComments = append(protoComments, &social.Comment{
			Id:         int64(c.ID),
			Content:    c.Content,
			CreateDate: c.CreatedAt.Format("01-02"),
			User: &user.User{
				Id:       int64(u.ID),
				Username: u.Username,
				Avatar:   u.Avatar,
				// ... 其他字段
			},
		})
	}

	return &social.CommentListResponse{
		StatusCode:  0,
		StatusMsg:   "success",
		CommentList: protoComments,
	}, nil
}
