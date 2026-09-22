package main

import (
	"net/http"
	"os"

	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	"github.com/Lhh220/g-video/web-server/internal/handler"
	"github.com/Lhh220/g-video/web-server/internal/middleware"
	"github.com/Lhh220/g-video/web-server/internal/rpc_client"
	"github.com/gin-gonic/gin"
)

func main() {
	// 结构化日志 (zap)：所有访问日志带 request_id
	logx.Init(true)

	// JWT 密钥与 logic 侧保持一致 (部署时通过 JWT_SECRET 环境变量注入)
	utils.SetSecret(os.Getenv("JWT_SECRET"))

	// 初始化 gRPC 客户端
	rpc_client.InitRPC()

	// gin.New() 不带默认中间件，改用自己的链：
	// Recovery 兜底 panic，Trace 注入 request_id + 访问日志，Metrics 上报 Prometheus
	r := gin.New()
	// gin 1.12 起 Context.Value 默认不委托 request context，
	// 打开 fallback 后 rpc 拦截器才能从 ctx 里读到 request_id
	r.ContextWithFallback = true
	r.Use(gin.Recovery(), middleware.Trace(), middleware.Metrics())

	// Prometheus 抓取端点
	r.GET("/metrics", gin.WrapH(middleware.MetricsHandler()))

	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 路由定义
	apiV1 := r.Group("/api/v1")
	{
		userV1 := apiV1.Group("/user")
		{
			// 限流防爆破：每 IP 每 2 秒 1 次、突发 5 次
			userV1.POST("/register", middleware.RateLimitLogin(), handler.Register)
			userV1.POST("/login", middleware.RateLimitLogin(), handler.Login)
			userV1.GET("/info", handler.GetUserInfo)
			userV1.POST("/update", handler.UpdateUserInfo)
		}
		videoV1 := apiV1.Group("/video")
		{
			videoV1.POST("/publish", middleware.RateLimitUpload(), handler.PublishVideo)
			// 大文件分片上传三步协议
			videoV1.POST("/upload/init", middleware.RateLimitUpload(), handler.InitUpload)
			videoV1.POST("/upload/part", middleware.RateLimitUpload(), handler.UploadPart)
			videoV1.POST("/upload/complete", middleware.RateLimitUpload(), handler.CompleteUpload)
			videoV1.GET("/feed", handler.GetFeed)
			videoV1.GET("/follow/feed", handler.GetFollowingFeed)
			videoV1.GET("/publish/list", handler.GetPublishList)
			videoV1.DELETE("/:id", handler.DeleteVideo)
		}

		apiV1.POST("/favorite/action", handler.FavoriteAction)
		apiV1.POST("/relation/action", handler.RelationAction)
		apiV1.POST("/comment/action", handler.CommentAction)
		apiV1.GET("/comment/list", handler.CommentList)
		apiV1.POST("/admin/audit", handler.AuditVideo)
		apiV1.GET("/admin/pending/list", handler.GetPendingList)

	}

	r.Run(":8080")
}
