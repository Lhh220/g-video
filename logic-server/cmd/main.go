package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Lhh220/g-video/api/proto/social"
	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/logic-server/internal/config"
	"github.com/Lhh220/g-video/logic-server/internal/interceptors"
	"github.com/Lhh220/g-video/logic-server/internal/mq"
	"github.com/Lhh220/g-video/logic-server/internal/service"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// 0. 结构化日志 (zap)
	logx.Init(true)

	// 1. 加载配置
	config.InitConfig()
	// JWT 密钥优先用配置注入，避免密钥硬编码在源码里
	utils.SetSecret(config.GlobalConfig.JWT.Secret)

	// 2. 初始化数据库 (传入配置文件里的 DSN)
	database.InitDB(config.GlobalConfig.Database.DSN)
	// 引导创建管理员账号 (注册接口已禁止指定 role)
	database.EnsureAdmin(config.GlobalConfig.Admin.Username, config.GlobalConfig.Admin.Password)
	// 3. 初始化 OSS
	oss.InitOSS()
	// 4. 初始化 Redis
	redis.InitRedis() // 新增这一行
	// 5. 初始化 RabbitMQ 并启动消费者 (未配置/不可达时自动降级，不影响主服务)
	mq.InitRabbitMQ(config.GlobalConfig.RabbitMQ.URL)
	mq.RunConsumers()

	// 6. 启动点赞计数落库协程 (Redis 增量每 5 秒合并进 MySQL；ctx 取消时退出)
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go service.RunFavoriteCounterFlusher(rootCtx)
	// 7. 热门池预计算协程 (每分钟刷新推荐流的热度 ZSet)
	go service.RunHotPoolRefresher(rootCtx)

	fmt.Println("Logic-Server 基础设施启动成功！")
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		panic(fmt.Sprintf("监听端口失败: %v", err))
	}

	// Prometheus 指标端点 (独立小 HTTP 服务，不与 gRPC 抢端口)
	metricsSrv := &http.Server{Addr: ":9091", Handler: func() http.Handler {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		return mux
	}()}
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logx.L().Warn("metrics 端点启动失败", zap.Error(err))
		}
	}()

	// 7. 创建 gRPC Server：Recovery 兜底 panic，Log 上报访问日志与耗时指标
	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptors.UnaryRecovery(), interceptors.UnaryLog()),
		grpc.MaxRecvMsgSize(50*1024*1024),
	)

	// 8. 注册服务
	user.RegisterUserServiceServer(s, &service.UserService{})
	video.RegisterVideoServiceServer(s, &service.VideoService{})
	social.RegisterSocialServiceServer(s, &service.SocialService{})

	// 9. 启动并在收到 SIGINT/SIGTERM 后优雅退出：
	// 排空在途请求 → 停指标端点 → 关 MQ/Redis
	go func() {
		fmt.Println("🚀 Logic-Server 正在端口 :50051 持续监听中...")
		if err := s.Serve(lis); err != nil {
			logx.L().Error("gRPC 服务异常退出", zap.Error(err))
		}
	}()

	<-rootCtx.Done()
	logx.L().Info("收到退出信号，开始优雅关闭 (排空在途请求)...")

	s.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = metricsSrv.Shutdown(shutdownCtx)

	mq.Close()
	_ = redis.RDB.Close()
	_ = logx.L().Sync()
	logx.L().Info("✅ 优雅退出完成")
}
