package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

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
	// 直传路径同样做扩展名白名单校验 (分片路径在 InitUpload 已校验)
	if !allowedVideoExts[strings.ToLower(filepath.Ext(header.Filename))] {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "不支持的视频格式"})
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
	// 1. 获取 Authorization Header (但不强制要求)
	authHeader := c.GetHeader("Authorization")

	var token string
	if authHeader != "" {
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		} else {
			token = authHeader
		}
	}

	// 2. 获取可选参数 latest_time
	latestTimeStr := c.Query("latest_time")
	var latestTime int64
	if latestTimeStr != "" {
		fmt.Sscanf(latestTimeStr, "%d", &latestTime)
	}

	// 3. 调用 RPC (即便 token 是空的也传过去)
	resp, err := rpc_client.VideoClient.Feed(c, &video.FeedRequest{
		LatestTime: latestTime,
		Token:      token, // Logic 层解析失败会当做游客处理，不会报错
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC错误"})
		return
	}

	// 4. 返回结果
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

func AuditVideo(c *gin.Context) {
	// 1. 从参数中获取数据
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

	videoID, _ := strconv.ParseInt(c.Query("video_id"), 10, 64)
	action, _ := strconv.ParseInt(c.Query("action"), 10, 32)
	reason := c.Query("reason")

	// 2. 鉴权：解析 Token 拿到当前操作者的信息
	claims, err := utils.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, video.AuditResponse{StatusCode: 1, StatusMsg: "Token 无效"})
		return
	}

	// 3. 重要：在 Web 层拦截非管理员请求，减轻 Logic 层负担
	if claims.Role != 1 {
		c.JSON(http.StatusForbidden, video.AuditResponse{StatusCode: 1, StatusMsg: "只有管理员有权审核"})
		return
	}

	// 4. 发起 RPC 调用
	resp, err := rpc_client.VideoClient.AuditVideo(c, &video.AuditRequest{
		AdminId: claims.UserID,
		VideoId: videoID,
		Action:  int32(action),
		Reason:  reason,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, video.AuditResponse{StatusCode: 1, StatusMsg: "RPC 调用超时"})
		return
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, resp)
}

// 获取关注用户的视频流 (FollowingFeed)
func GetFollowingFeed(c *gin.Context) {
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

	claims, err := utils.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status_code": 1, "status_msg": "未登录"})
		return
	}

	resp, err := rpc_client.VideoClient.FollowingFeed(c, &video.FollowingFeedRequest{
		UserId: claims.UserID,
		Token:  token,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "服务繁忙"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetPendingList 管理员后台：分页拉取待审核视频列表
func GetPendingList(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	var token string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		token = authHeader
	}

	claims, err := utils.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status_code": 1, "status_msg": "未登录"})
		return
	}
	if claims.Role != 1 {
		c.JSON(http.StatusForbidden, gin.H{"status_code": 1, "status_msg": "只有管理员有权访问"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := rpc_client.VideoClient.ListPendingVideos(c, &video.PendingListRequest{
		AdminId:  claims.UserID,
		Token:    token,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC服务异常"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// extractToken 从 Authorization 头里提取 token (兼容无 Bearer 前缀)
func extractToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return h
}

// 上传文件硬限制
var (
	allowedVideoExts = map[string]bool{
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".flv": true, ".webm": true, ".m4v": true, ".ts": true,
	}
	maxVideoSize = int64(2 << 30) // 2GB
	maxPartSize  = 8 << 20        // 单分片 8MB
)

// InitUpload 分片上传第一步：秒传/断点续传判断，返回 upload_id 和已传分片
func InitUpload(c *gin.Context) {
	var reqData struct {
		Filename     string `json:"filename"`
		FileSize     int64  `json:"file_size"`
		FileMd5      string `json:"file_md5"`
		PrevUploadId string `json:"prev_upload_id"`
	}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "参数格式错误"})
		return
	}

	// 入参校验：扩展名白名单 + 大小上限 + 指纹格式，防止恶意文件与异常参数
	if !allowedVideoExts[strings.ToLower(filepath.Ext(reqData.Filename))] {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "不支持的视频格式 (仅限 mp4/mov/avi/mkv/flv/webm/m4v/ts)"})
		return
	}
	if reqData.FileSize <= 0 || reqData.FileSize > maxVideoSize {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "视频大小需在 2GB 以内"})
		return
	}
	if len(reqData.FileMd5) != 32 {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "文件指纹格式错误"})
		return
	}

	resp, err := rpc_client.VideoClient.InitUpload(c, &video.InitUploadRequest{
		Token:        extractToken(c),
		Filename:     reqData.Filename,
		FileSize:     reqData.FileSize,
		FileMd5:      reqData.FileMd5,
		PrevUploadId: reqData.PrevUploadId,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC调用失败"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UploadPart 分片上传第二步：上传单个分片
func UploadPart(c *gin.Context) {
	uploadID := c.PostForm("upload_id")
	partNumber, _ := strconv.Atoi(c.PostForm("part_number"))
	if uploadID == "" || partNumber <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "缺少 upload_id 或 part_number"})
		return
	}

	_, header, err := c.Request.FormFile("data")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "无法读取分片数据"})
		return
	}
	fileObj, _ := header.Open()
	defer fileObj.Close()

	data, err := io.ReadAll(fileObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "读取分片失败"})
		return
	}
	if len(data) > maxPartSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"status_code": 1, "status_msg": "单个分片超过 8MB 上限"})
		return
	}

	resp, err := rpc_client.VideoClient.UploadPart(c, &video.UploadPartRequest{
		Token:     extractToken(c),
		UploadId:  uploadID,
		PartNumber: int32(partNumber),
		Data:      data,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC调用失败"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CompleteUpload 分片上传第三步：合并分片并发布 (秒传时 upload_id 为空)
func CompleteUpload(c *gin.Context) {
	var reqData struct {
		UploadId string `json:"upload_id"`
		FileMd5  string `json:"file_md5"`
		Title    string `json:"title"`
	}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "参数格式错误"})
		return
	}

	resp, err := rpc_client.VideoClient.CompleteUpload(c, &video.CompleteUploadRequest{
		Token:    extractToken(c),
		UploadId: reqData.UploadId,
		FileMd5:  reqData.FileMd5,
		Title:    reqData.Title,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC调用失败"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func DeleteVideo(c *gin.Context) {
	// 1. 鉴权获取当前用户 ID
	authHeader := c.GetHeader("Authorization")
	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	claims, err := utils.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status_code": 1, "status_msg": "未登录"})
		return
	}

	// 2. 获取视频 ID
	videoID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if videoID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status_code": 1, "status_msg": "无效的视频ID"})
		return
	}

	// 3. RPC 调用 Logic 层
	resp, err := rpc_client.VideoClient.DeleteVideo(c, &video.DeleteRequest{
		UserId:  claims.UserID,
		VideoId: videoID,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status_code": 1, "status_msg": "RPC服务异常"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
