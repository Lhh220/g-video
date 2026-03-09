package handler

import (
	"net/http"

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
