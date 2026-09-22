package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// per-IP 令牌桶限流 (单实例内存版；多实例部署应换 Redis 集中计数)
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
	// 每分钟清理 10 分钟没出现的 IP，防止 map 无限膨胀
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

func (m *ipLimiterMap) allow(ip string, r rate.Limit, burst int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.limiters[ip]
	if !ok {
		l = &ipLimiter{limiter: rate.NewLimiter(r, burst)}
		m.limiters[ip] = l
	}
	l.lastSeen = time.Now()
	return l.limiter.Allow()
}

var (
	loginLimiters  = newIPLimiterMap()
	uploadLimiters = newIPLimiterMap()
)

// RateLimitLogin 登录/注册接口限流：每 IP 每 2 秒 1 次、突发 5 次，防口令爆破
func RateLimitLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !loginLimiters.allow(c.ClientIP(), 0.5, 5) {
			c.JSON(http.StatusTooManyRequests, gin.H{"status_code": 1, "status_msg": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitUpload 上传接口限流：每 IP 每秒 10 次、突发 20 次
func RateLimitUpload() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !uploadLimiters.allow(c.ClientIP(), 10, 20) {
			c.JSON(http.StatusTooManyRequests, gin.H{"status_code": 1, "status_msg": "上传请求过于频繁"})
			c.Abort()
			return
		}
		c.Next()
	}
}
