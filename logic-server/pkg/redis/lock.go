package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// 分布式锁：SET NX EX 抢锁 + Lua 比对 token 释放 (防误删他人的锁)。
// 不做续期：锁的 TTL 必须大于持锁任务的最大耗时，任务天然幂等时可接受锁过期后重复执行。

// TryLock 抢锁。返回 (是否成功, 释放用的 token, 错误)。
// Redis 异常时返回 err，由调用方决定降级策略。
func TryLock(ctx context.Context, key string, ttl time.Duration) (bool, string, error) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	ok, err := RDB.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, "", err
	}
	return ok, token, nil
}

var releaseLockScript = `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`

// ReleaseLock 释放锁；只有锁还是自己持有 (token 匹配) 时才删除
func ReleaseLock(ctx context.Context, key, token string) {
	if token == "" {
		return
	}
	_ = RDB.Eval(ctx, releaseLockScript, []string{key}, token).Err()
}
