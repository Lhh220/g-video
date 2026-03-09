package redis

import (
	"context"
	"fmt"

	"github.com/Lhh220/g-video/logic-server/internal/config"
	"github.com/go-redis/redis/v8"
)

var RDB *redis.Client
var ctx = context.Background()

func InitRedis() {
	c := config.GlobalConfig.Redis

	RDB = redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})

	// 探活：尝试 Ping 一下 Redis
	_, err := RDB.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("Redis 连接失败: %v", err))
	}

	fmt.Println("✅ Redis 连接验证成功！")
}
