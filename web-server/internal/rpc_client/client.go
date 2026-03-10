package rpc_client

import (
	"log"

	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video" // 1. 新增视频服务的导入
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var UserClient user.UserServiceClient
var VideoClient video.VideoServiceClient // 2. 新增视频客户端变量

func InitRPC() {
	conn, err := grpc.Dial("127.0.0.1:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(50*1024*1024)),
	)
	if err != nil {
		log.Fatalf("无法连接 Logic-Server: %v", err)
	}
	UserClient = user.NewUserServiceClient(conn)
	VideoClient = video.NewVideoServiceClient(conn) // 3. 初始化视频客户端
}
