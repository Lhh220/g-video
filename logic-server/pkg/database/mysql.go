package database

import (
	"fmt"
	"log"

	"github.com/Lhh220/g-video/logic-server/internal/model" // 换成你实际的包名

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
