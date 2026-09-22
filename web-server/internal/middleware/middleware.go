package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// request_id 在 context 里的键；rpc_client 拦截器读取它注入 gRPC metadata，
// 实现 web → logic 的链路贯穿
type ctxKey string

const RequestIDKey ctxKey = "x-request-id"

var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "HTTP 请求耗时分布",
	Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
}, []string{"method", "path", "code"})

// NewRequestID 生成 16 位随机十六进制请求标识
func NewRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// MetricsHandler 暴露 /metrics 供 Prometheus 抓取
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// Trace 为每个请求注入 request_id (透传上游的 X-Request-ID)，输出访问日志
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = NewRequestID()
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), RequestIDKey, id))
		c.Header("X-Request-ID", id)

		start := time.Now()
		c.Next()

		fields := []zap.Field{
			zap.String("request_id", id),
			zap.String("method", c.Request.Method),
			zap.String("path", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("gin_errors", c.Errors.String()))
		}
		logx.L().Info("http_request", fields...)
	}
}

// Metrics 记录 HTTP 请求耗时到 Prometheus
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		httpDuration.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			strconv.Itoa(c.Writer.Status()),
		).Observe(time.Since(start).Seconds())
	}
}
