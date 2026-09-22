package rpc_client

import (
	"context"
	"log"
	"os" // 新增：导入 os 包读取环境变量

	"github.com/Lhh220/g-video/api/proto/social"
	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/web-server/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var UserClient user.UserServiceClient
var VideoClient video.VideoServiceClient
var SocialClient social.SocialServiceClient

// conn 保留连接引用，退出时统一关闭
var conn *grpc.ClientConn

// Close 关闭 gRPC 连接 (服务退出前调用)
func Close() {
	if conn != nil {
		_ = conn.Close()
	}
}

// injectRequestID 把 web 层的 request_id 塞进 gRPC metadata，logic 层日志可据此串联整条链路
func injectRequestID(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if v, ok := ctx.Value(middleware.RequestIDKey).(string); ok && v != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", v)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func InitRPC() {
	// ✅ 核心修改：优先读取环境变量里的 LOGIC_SRV_ADDR
	addr := os.Getenv("LOGIC_SRV_ADDR")
	if addr == "" {
		// 本地开发默认值（保留原硬编码，不影响本地 go run）
		addr = "127.0.0.1:50051"
	}

	// 连接 gRPC 服务（地址用变量，适配 Docker/本地）
	c, err := grpc.DialContext(context.Background(), addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(50*1024*1024)),
		grpc.WithUnaryInterceptor(injectRequestID),
	)
	if err != nil {
		log.Fatalf("无法连接 Logic-Server: %v", err)
	}
	conn = c

	UserClient = user.NewUserServiceClient(conn)
	VideoClient = video.NewVideoServiceClient(conn)
	SocialClient = social.NewSocialServiceClient(conn)

	// 新增：日志提示，确认连接的地址（方便调试）
	log.Printf("✅ 成功连接 Logic-Server: %s", addr)
}
