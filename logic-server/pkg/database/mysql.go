package database

import (
	"fmt"
	"log"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(dsn string) {
	var err error

	// 1. 连接数据库
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // 打印所有SQL语句，方便调试
	})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}

	// 2. 自动迁移 (AutoMigrate)
	// 这行代码会自动根据你的 struct 在数据库里创建/修改表结构
	err = DB.AutoMigrate(
		&model.User{},
		&model.Video{},
		&model.Like{},
		&model.Comment{},
		&model.AuditLog{},
		&model.Follow{},
	)
	if err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	fmt.Println("数据库连接成功并完成自动迁移！")
}

// EnsureAdmin 启动引导：确保系统存在至少一个管理员账号。
// 注册接口已禁止指定 role，这是创建管理员的唯一入口。
func EnsureAdmin(username, password string) {
	if username == "" || password == "" {
		fmt.Println("⚠️ 未配置 admin.username/admin.password，跳过管理员引导")
		return
	}

	var count int64
	DB.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		fmt.Printf("✅ 管理员账号已存在: %s\n", username)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("管理员密码加密失败: %v", err)
		return
	}

	admin := model.User{
		Username: username,
		Password: string(hashed),
		Role:     1,
		Avatar:   "https://g-video-assets.oss-cn-wuhan-lr.aliyuncs.com/default_avatar.png",
	}
	if err := DB.Create(&admin).Error; err != nil {
		log.Printf("引导创建管理员失败: %v", err)
		return
	}
	fmt.Printf("✅ 已引导创建管理员账号: %s (请尽快修改默认密码)\n", username)
}
