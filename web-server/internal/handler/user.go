package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Lhh220/g-video/api/proto/user" // 引入你的 pb 文件
	"github.com/Lhh220/g-video/web-server/internal/rpc_client"
	"github.com/gin-gonic/gin"
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     int32  `json:"role"`
}

// Register 处理注册请求
func Register(c *gin.Context) {
	var reqData RegisterRequest

	// 使用 ShouldBindJSON 自动解析 Body 里的 JSON
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 1,
			"status_msg":  "参数格式错误或用户名密码为空",
		})
		return
	}

	// 调用 RPC 时使用解析出来的 reqData
	resp, err := rpc_client.UserClient.Register(c, &user.RegisterRequest{
		Username: reqData.Username,
		Password: reqData.Password,
		Role:     reqData.Role,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_msg": "RPC调用失败"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func Login(c *gin.Context) {
	var reqData RegisterRequest
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status_msg": "参数格式错误"})
		return
	}

	// 调用 RPC 的 Login 接口
	resp, err := rpc_client.UserClient.Login(c, &user.LoginRequest{
		Username: reqData.Username,
		Password: reqData.Password,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_msg": "RPC服务不可用"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func GetUserInfo(c *gin.Context) {
	// 1. 获取 Query 参数
	userIDStr := c.Query("user_id")
	authHeader := c.GetHeader("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. 参数校验
	if userIDStr == "" || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 1,
			"status_msg":  "Missing user_id or token",
		})
		return
	}

	// 3. 转换 ID 类型
	uid, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "Invalid user_id"})
		return
	}

	// 4. 发起 RPC 调用
	resp, err := rpc_client.UserClient.GetUserInfo(c, &user.UserInfoRequest{
		UserId: uid,
		Token:  token,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 1,
			"status_msg":  "Internal RPC Error: " + err.Error(),
		})
		return
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, resp)
}
