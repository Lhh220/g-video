package main

import (
	"fmt"

	"github.com/Lhh220/g-video/logic-server/internal/config"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
)

func main() {
	// 1. 加载配置
	config.InitConfig()

	// 2. 初始化数据库 (传入配置文件里的 DSN)
	database.InitDB(config.GlobalConfig.Database.DSN)
	// 3. 初始化 OSS
	oss.InitOSS()
	// 4. 初始化 Redis
	redis.InitRedis() // 新增这一行

	fmt.Println("Logic-Server 基础设施启动成功！")
}
