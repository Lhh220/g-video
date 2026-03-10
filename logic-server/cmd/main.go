package main

import (
	"fmt"
	"net"

	"github.com/Lhh220/g-video/api/proto/social"
	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/logic-server/internal/config"
	"github.com/Lhh220/g-video/logic-server/internal/service"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"google.golang.org/grpc"
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
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		panic(fmt.Sprintf("监听端口失败: %v", err))
	}

	// 3. 创建 gRPC Server 实例
	s := grpc.NewServer(
		grpc.MaxRecvMsgSize(50 * 1024 * 1024),
	)

	// 4. 注册服务：把你的逻辑关联到 Server 上
	// 这里的 &service.UserService{} 就是你写的处理注册登录的代码
	user.RegisterUserServiceServer(s, &service.UserService{})
	// 注册视频服务
	video.RegisterVideoServiceServer(s, &service.VideoService{})
	// 注册社交服务
	social.RegisterSocialServiceServer(s, &service.SocialService{})

	// 5. 启动！这里会阻塞，不会退出
	fmt.Println("🚀 Logic-Server 正在端口 :50051 持续监听中...")
	if err := s.Serve(lis); err != nil {
		panic(fmt.Sprintf("启动服务失败: %v", err))
	}
}
