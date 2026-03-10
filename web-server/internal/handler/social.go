package handler

import (
	"net/http"
	"strconv"

	"github.com/Lhh220/g-video/api/proto/social"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	"github.com/Lhh220/g-video/web-server/internal/rpc_client" // 替换为你的 rpc 引用路径
	"github.com/gin-gonic/gin"
)

func FavoriteAction(c *gin.Context) {
	// 1. 从 Header 获取 Authorization
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status_code": 1, "status_msg": "请求头缺少 Authorization"})
		return
	}
	// 2. 提取真正的 Token 字符串
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		token = authHeader // 容错处理，万一没带前缀直接传了 token
	}
	// 3. 从 URL 获取 video_id和action_type
	videoIDStr := c.Query("video_id")
	actionTypeStr := c.Query("action_type")
	// 参数转换
	videoID, _ := strconv.ParseInt(videoIDStr, 10, 64)
	actionType, _ := strconv.ParseInt(actionTypeStr, 10, 32)
	//通过校验token获取用户ID
	claims, err := utils.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, social.FavoriteResponse{
			StatusCode: 1,
			StatusMsg:  "Token 解析失败，请先登录",
		})
		return
	}
	// 4. 发起 RPC 调用
	resp, err := rpc_client.SocialClient.FavoriteAction(c, &social.FavoriteRequest{
		UserId:     claims.UserID,
		VideoId:    videoID,
		ActionType: int32(actionType),
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, social.FavoriteResponse{
			StatusCode: 1,
			StatusMsg:  "社交服务调用失败: " + err.Error(),
		})
		return
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, resp)

}

func RelationAction(c *gin.Context) {
	//1、从Header获取Authorization
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status_code": 1, "status_msg": "请求头缺少 Authorization"})
		return
	}
	//2、提取真正的 Token 字符串
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		token = authHeader // 容错处理，万一没带前缀直接传了 token
	}
	//3、从URL获取follow_user_id和action_type
	followUserIDStr := c.Query("follow_user_id")
	actionTypeStr := c.Query("action_type")
	//参数转换
	followUserID, _ := strconv.ParseInt(followUserIDStr, 10, 64)
	actionType, _ := strconv.ParseInt(actionTypeStr, 10, 32)
	//通过校验token获取用户ID
	claims, err := utils.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, social.RelationResponse{
			StatusCode: 1,
			StatusMsg:  "Token 解析失败，请先登录",
		})
		return
	}
	//4、发起 RPC 调用
	resp, err := rpc_client.SocialClient.RelationAction(c, &social.RelationRequest{
		UserId:     claims.UserID,
		ToUserId:   followUserID,
		ActionType: int32(actionType),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, social.RelationResponse{
			StatusCode: 1,
			StatusMsg:  "社交服务调用失败: " + err.Error(),
		})
		return
	}
	// 5. 返回结果
	c.JSON(http.StatusOK, resp)
}
