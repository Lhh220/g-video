package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// 限流：Redis 固定窗口计数 (多实例共享配额)；Redis 不可用时降级为单机内存令牌桶。
// 计数维度 per-IP；更精细的可按 用户ID+IP，当前场景 IP 粒度足够。

var rlClient *goredis.Client

// SetRateLimitRedis 注入共享 Redis 客户端 (部署时由 main 调用；不注入则走单机限流)
func SetRateLimitRedis(addr, password string, db int) {
	rlClient = goredis.NewClient(&goredis.Options{Addr: addr, Password: password, DB: db})
}

// redisFixedWindowAllow 固定窗口：每分钟 limit 次
func redisFixedWindowAllow(scope, ip string, limitPerMin int) (bool, error) {
	if rlClient == nil {
		return false, errors.New("redis client 未初始化")
	}
	ctx := context.Background()
	key := fmt.Sprintf("rl:%s:%s:%d", scope, ip, time.Now().Unix()/60)
	n, err := rlClient.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		rlClient.Expire(ctx, key, 90*time.Second)
	}
	return n <= int64(limitPerMin), nil
}

// ---- 单机降级实现：per-IP 令牌桶 ----

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipLimiterMap struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
}

func newIPLimiterMap() *ipLimiterMap {
	m := &ipLimiterMap{limiters: make(map[string]*ipLimiter)}
	// 每分钟清理 10 分钟未访问的 IP，防 map 膨胀
	go func() {
		for range time.Tick(time.Minute) {
			m.mu.Lock()
			for ip, l := range m.limiters {
				if time.Since(l.lastSeen) > 10*time.Minute {
					delete(m.limiters, ip)
				}
			}
			m.mu.Unlock()
		}
	}()
	return m
}

func (m *ipLimiterMap) allow(ip string, perSec float64, burst int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.limiters[ip]
	if !ok {
		l = &ipLimiter{limiter: rate.NewLimiter(rate.Limit(perSec), burst)}
		m.limiters[ip] = l
	}
	l.lastSeen = time.Now()
	return l.limiter.Allow()
}

var (
	loginLimiters  = newIPLimiterMap()
	uploadLimiters = newIPLimiterMap()
)

// allow 统一入口：先走 Redis 集中限流，异常降级单机令牌桶
func allow(scope, ip string, perMin int, fallbackPerSec float64, fallbackBurst int) bool {
	if ok, err := redisFixedWindowAllow(scope, ip, perMin); err == nil {
		return ok
	} else {
		logx.L().Warn("Redis 限流不可用，降级单机限流", zap.String("scope", scope), zap.Error(err))
	}
	// 突发容量对齐分钟配额，保证降级前后体感一致
	return loginFallbackAllow(scope, ip, fallbackPerSec, fallbackBurst)
}

func loginFallbackAllow(scope, ip string, perSec float64, burst int) bool {
	if scope == "login" {
		return loginLimiters.allow(ip, perSec, burst)
	}
	return uploadLimiters.allow(ip, perSec, burst)
}

// RateLimitLogin 登录/注册接口限流：每 IP 每分钟 5 次，防口令爆破
func RateLimitLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !allow("login", c.ClientIP(), 5, 0.5, 5) {
			c.JSON(http.StatusTooManyRequests, gin.H{"status_code": 1, "status_msg": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitUpload 上传接口限流：每 IP 每分钟 100 次
func RateLimitUpload() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !allow("upload", c.ClientIP(), 100, 2, 20) {
			c.JSON(http.StatusTooManyRequests, gin.H{"status_code": 1, "status_msg": "上传请求过于频繁"})
			c.Abort()
			return
		}
		c.Next()
	}
}
