package main

import (
	"log"

	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/web-server/internal/handler"
	"github.com/Lhh220/g-video/web-server/internal/rpc_client"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var userClient user.UserServiceClient

func initClient() {
	// 连接 logic-server 的 50051 端口
	conn, err := grpc.Dial("127.0.0.1:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("无法连接 logic-server: %v", err)
	}
	userClient = user.NewUserServiceClient(conn)
}

func main() {
	// 初始化 gRPC 客户端
	rpc_client.InitRPC()
	r := gin.Default()

	// 路由定义
	apiV1 := r.Group("/api/v1")
	{
		apiV1.POST("/user/register", handler.Register)
		apiV1.POST("/user/login", handler.Login)
		apiV1.GET("/user/info", handler.GetUserInfo)
	}

	r.Run(":8080")
}
