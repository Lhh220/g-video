package rpc_client

import (
	"log"
	"os" // 新增：导入 os 包读取环境变量

	"github.com/Lhh220/g-video/api/proto/social"
	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var UserClient user.UserServiceClient
var VideoClient video.VideoServiceClient
var SocialClient social.SocialServiceClient

func InitRPC() {
	// ✅ 核心修改：优先读取环境变量里的 LOGIC_SRV_ADDR
	addr := os.Getenv("LOGIC_SRV_ADDR")
	if addr == "" {
		// 本地开发默认值（保留原硬编码，不影响本地 go run）
		addr = "127.0.0.1:50051"
	}

	// 连接 gRPC 服务（地址用变量，适配 Docker/本地）
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(50*1024*1024)),
	)
	if err != nil {
		log.Fatalf("无法连接 Logic-Server: %v", err)
	}

	UserClient = user.NewUserServiceClient(conn)
	VideoClient = video.NewVideoServiceClient(conn)
	SocialClient = social.NewSocialServiceClient(conn)

	// 新增：日志提示，确认连接的地址（方便调试）
	log.Printf("✅ 成功连接 Logic-Server: %s", addr)
}
