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
		userV1 := apiV1.Group("/user")
		{
			userV1.POST("/register", handler.Register)
			userV1.POST("/login", handler.Login)
			userV1.GET("/info", handler.GetUserInfo)
			userV1.POST("/update", handler.UpdateUserInfo)
		}
		videoV1 := apiV1.Group("/video")
		{
			videoV1.POST("/publish", handler.PublishVideo)
			videoV1.GET("/feed", handler.GetFeed)
		}
	}

	r.Run(":8080")
}
