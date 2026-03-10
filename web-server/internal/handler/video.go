package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/Lhh220/g-video/api/proto/video" // 替换为你生成的 video 代码路径
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	"github.com/Lhh220/g-video/web-server/internal/rpc_client" // 替换为你的 rpc 引用路径
	"github.com/gin-gonic/gin"
)

func PublishVideo(c *gin.Context) {
	// 1. 从 Header 获取 Authorization
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status_code": 1, "status_msg": "请求头缺少 Authorization"})
		return
	}

	// 2. 提取真正的 Token 字符串 (去掉 "Bearer " 前缀)
	// 假设格式为 "Bearer xxxxx.yyyyy.zzzzz"
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		token = authHeader // 容错处理，万一没带前缀直接传了 token
	}
	title := c.PostForm("title")

	// 2. 获取视频文件
	_, header, err := c.Request.FormFile("data")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 1,
			"status_msg":  "无法读取视频文件: " + err.Error(),
		})
		return
	}

	// 3. 读取文件内容到内存 ([]byte)
	fileObj, _ := header.Open()
	defer fileObj.Close()

	videoData, err := io.ReadAll(fileObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 1,
			"status_msg":  "读取视频失败",
		})
		return
	}

	// 4. 调用 Logic-Server RPC
	// 注意：这里我们给 Context 加个超时时间，防止大视频上传太久导致连接断开
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	// defer cancel()

	resp, err := rpc_client.VideoClient.PublishVideo(c, &video.PublishRequest{
		Token:    token,
		Data:     videoData,
		Title:    title,
		Filename: header.Filename, // 传给 Logic 层拼接 OSS 后缀
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 1,
			"status_msg":  "RPC调用失败: " + err.Error(),
		})
		return
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, resp)
}

func GetFeed(c *gin.Context) {
	// 获取可选参数 latest_time
	latestTimeStr := c.Query("latest_time")
	var latestTime int64
	if latestTimeStr != "" {
		fmt.Sscanf(latestTimeStr, "%d", &latestTime)
	}

	// 调用 RPC
	resp, err := rpc_client.VideoClient.Feed(c, &video.FeedRequest{
		LatestTime: latestTime,
		Token:      c.Query("token"), // 传入 token 以便后续判断是否点赞
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC错误"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func GetPublishList(c *gin.Context) {
	userIDStr := c.Query("user_id")
	authHeader := c.GetHeader("Authorization")

	// 1. 获取 Token
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		token = authHeader
	}

	// 2. 先尝试从 Query 解析 ID
	var targetUserID int64
	fmt.Sscanf(userIDStr, "%d", &targetUserID)

	// 3. 【核心修改】如果 URL 里没传 ID (即 targetUserID == 0)
	// 那么我们才去解析 Token 拿到当前登录人的 ID
	if targetUserID == 0 {
		claims, err := utils.ParseToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"status_code": 1, "status_msg": "Token无效且未指定用户ID"})
			return
		}
		targetUserID = claims.UserID
	}

	// 4. 调用 RPC 时，传入我们确定好的 targetUserID
	resp, err := rpc_client.VideoClient.GetPublishList(c, &video.PublishListRequest{
		UserId: targetUserID, // 这里用判断后的变量
		Token:  token,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC服务异常"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
