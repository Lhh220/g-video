package rpc_client

import (
	"log"

	"github.com/Lhh220/g-video/api/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var UserClient user.UserServiceClient

func InitRPC() {
	conn, err := grpc.Dial("127.0.0.1:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("无法连接 Logic-Server: %v", err)
	}
	UserClient = user.NewUserServiceClient(conn)
}
