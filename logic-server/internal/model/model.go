package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户表
type User struct {
	ID        int64          `gorm:"primaryKey;column:id"`
	Username  string         `gorm:"unique;not null;column:username;size:64"`
	Password  string         `gorm:"not null;column:password;size:255"`
	Avatar    string         `gorm:"column:avatar;size:255;default:''"`
	Role      int32          `gorm:"column:role;default:0"` // 0-用户, 1-管理员
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"` // 软删除支持

	// 关联字段：一个用户可以拥有多个视频
	Videos []Video `gorm:"foreignKey:AuthorID"`
}

// Video 视频表
type Video struct {
	ID            int64          `gorm:"primaryKey;column:id"`
	AuthorID      int64          `gorm:"index;column:author_id;not null"`
	Title         string         `gorm:"column:title;size:128;not null"`
	PlayURL       string         `gorm:"column:play_url;size:255;not null"`
	CoverURL      string         `gorm:"column:cover_url;size:255"`
	FavoriteCount int64          `gorm:"column:favorite_count;default:0"`
	CommentCount  int64          `gorm:"column:comment_count;default:0"`
	Status        int32          `gorm:"column:status;default:0"` // 0-待审, 1-通过, 2-拒绝
	CreatedAt     time.Time      `gorm:"column:created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index"`

	// 关联字段：属于某个作者
	Author User `gorm:"foreignKey:AuthorID"`
}

// Like 点赞表
type Like struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"uniqueIndex:idx_user_video;column:user_id;not null"`
	VideoID   int64     `gorm:"uniqueIndex:idx_user_video;index;column:video_id;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`

	// 关联
	User  User  `gorm:"foreignKey:UserID"`
	Video Video `gorm:"foreignKey:VideoID"`
}

// Comment 评论表
type Comment struct {
	ID        int64          `gorm:"primaryKey;column:id"`
	UserID    int64          `gorm:"column:user_id;not null;index"`
	VideoID   int64          `gorm:"column:video_id;not null;index"`
	Content   string         `gorm:"column:content;type:text;not null"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// 关联
	User User `gorm:"foreignKey:UserID"`
}

// AuditLog 审核日志表
type AuditLog struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	VideoID   int64     `gorm:"column:video_id;not null;index"`
	AdminID   int64     `gorm:"column:admin_id;not null"`
	Action    int32     `gorm:"column:action;not null"` // 1-通过, 2-驳回
	Reason    string    `gorm:"column:reason;size:255;default:''"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// Follow 关注表 (可选补齐)
type Follow struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	UserID    int64     `gorm:"uniqueIndex:idx_user_to;column:user_id;not null"`    // 关注者
	ToUserID  int64     `gorm:"uniqueIndex:idx_user_to;column:to_user_id;not null"` // 被关注者
	CreatedAt time.Time `gorm:"column:created_at"`
}
