package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Lhh220/g-video/api/proto/user" // 确保你的pb路径正确
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	user.UnimplementedUserServiceServer
}

// Register 用户注册
func (s *UserService) Register(ctx context.Context, req *user.RegisterRequest) (*user.RegisterResponse, error) {
	// 1. 检查用户是否已存在
	var existingUser model.User
	err := database.DB.Where("username = ?", req.Username).First(&existingUser).Error
	if err == nil {
		return &user.RegisterResponse{StatusCode: 400, StatusMsg: "用户已存在"}, nil
	}

	// 2. 密码加密 (不要存明文！)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	// 3. 生成默认头像 URL
	defaultAvatar := "https://g-video-assets.oss-cn-wuhan-lr.aliyuncs.com/default_avatar.png"
	// 3. 写入数据库
	newUser := model.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Role:     req.Role, // 将请求中的身份存入数据库
		Avatar:   defaultAvatar,
	}
	if err := database.DB.Create(&newUser).Error; err != nil {
		return &user.RegisterResponse{StatusCode: 500, StatusMsg: "注册失败"}, nil
	}

	return &user.RegisterResponse{
		StatusCode: 0,
		StatusMsg:  "注册成功",
		UserId:     newUser.ID,
	}, nil
}

func (s *UserService) Login(ctx context.Context, req *user.LoginRequest) (*user.LoginResponse, error) {
	// 1. 参数校验
	if req.Username == "" || req.Password == "" {
		return &user.LoginResponse{StatusCode: 1, StatusMsg: "用户名或密码为空"}, nil
	}

	// 2. 根据用户名查询用户信息
	var u model.User
	err := database.DB.Where("username = ?", req.Username).First(&u).Error
	if err != nil {
		// 如果找不到记录
		return &user.LoginResponse{StatusCode: 1, StatusMsg: "用户不存在"}, nil
	}

	// 3. 比对密码
	// bcrypt.CompareHashAndPassword(数据库里的密文, 用户输入的明文)
	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password))
	if err != nil {
		return &user.LoginResponse{StatusCode: 1, StatusMsg: "密码错误"}, nil
	}

	// 4. 验证通过，颁发 Token (身份证)
	token, err := utils.GenerateToken(u.ID, u.Role)
	if err != nil {
		return &user.LoginResponse{StatusCode: 500, StatusMsg: "生成Token失败"}, nil
	}

	// 5. 返回结果
	return &user.LoginResponse{
		StatusCode: 0,
		StatusMsg:  "登录成功",
		UserId:     u.ID,
		Token:      token, // 只有登录才返回 Token
	}, nil
}

func (s *UserService) GetUserInfo(ctx context.Context, req *user.UserInfoRequest) (*user.UserInfoResponse, error) {
	// 1. 鉴权：解析 Token
	// 虽然是看别人的信息，但通常系统要求必须是登录用户才能查看
	_, err := utils.ParseToken(req.Token)
	if err != nil {
		return &user.UserInfoResponse{
			StatusCode: 1,
			StatusMsg:  "Token 已过期或无效，请重新登录",
		}, nil
	}
	//先查redis
	cacheKey := fmt.Sprintf("user:info:%d", req.UserId)

	// --- 【同步过程】优先从 Redis 获取 ---
	// 查询 Redis 是同步的，因为我们需要它的结果来判断是否还要查数据库
	val, err := redis.RDB.Get(ctx, cacheKey).Result()
	if err == nil && val != "" {
		var cachedUser user.User
		if err := json.Unmarshal([]byte(val), &cachedUser); err == nil {
			fmt.Printf("🚀 [Cache Hit] 命中缓存，直接返回用户: %d\n", req.UserId)
			return &user.UserInfoResponse{
				StatusCode: 0,
				StatusMsg:  "查询成功(Cached)",
				User:       &cachedUser,
			}, nil
		}
	}

	// 2. 查询数据库
	var u model.User
	// 根据请求中的 user_id 查找
	if err := database.DB.First(&u, req.UserId).Error; err != nil {
		return &user.UserInfoResponse{
			StatusCode: 1,
			StatusMsg:  "该用户不存在",
		}, nil
	}
	var avatarUrl string
	if u.Avatar == "" {
		avatarUrl = "https://g-video-assets.oss-cn-wuhan-lr.aliyuncs.com/default_avatar.png"
	} else {
		avatarUrl = u.Avatar
	}
	// --- 【异步】写 Redis (👈 补上这一段) ---
	userInfoForCache := &user.User{
		Id:            u.ID,
		Username:      u.Username,
		Avatar:        avatarUrl,
		FollowCount:   u.FollowCount,
		FollowerCount: u.FollowerCount,
	}

	go func(data *user.User) {
		// 使用 Background 防止主进程返回后 Context 取消导致写入失败
		jsonData, _ := json.Marshal(data)
		// 设置 24 小时过期
		redis.RDB.Set(context.Background(), cacheKey, jsonData, 24*time.Hour)
		fmt.Printf("📝 [Redis Save] 数据已异步存入 Redis，Key: %s\n", cacheKey)
	}(userInfoForCache)

	fmt.Printf("🎯 [GetUserInfo] 查询用户 %d 成功 (来自 DB)\n", req.UserId)

	// 3. 组装返回结果
	// 注意：这里的 user.User 是你 proto 生成的结构体，不是 model.User
	return &user.UserInfoResponse{
		StatusCode: 0,
		StatusMsg:  "查询成功",
		User: &user.User{
			Id:            u.ID,
			Username:      u.Username,
			Avatar:        avatarUrl,
			FollowCount:   u.FollowCount,
			FollowerCount: u.FollowerCount,
		},
	}, nil
}

func (s *UserService) UpdateUserInfo(ctx context.Context, req *user.UpdateUserInfoRequest) (*user.UpdateUserInfoResponse, error) {
	// 1. 鉴权：解析 Token 拿到当前用户的 ID
	claims, err := utils.ParseToken(req.Token)
	if err != nil {
		return &user.UpdateUserInfoResponse{StatusCode: 1, StatusMsg: "登录已失效"}, nil
	}

	// 2. 使用 Map 构造更新数据（最灵活，避开 GORM 零值坑）
	updateMap := make(map[string]interface{})

	if req.Username != "" {
		updateMap["username"] = req.Username
	}

	if len(req.AvatarData) > 0 {
		// 构造唯一的文件名，防止多人上传同名文件冲突
		// 格式：avatar/时间戳_文件名.jpg
		objectName := fmt.Sprintf("avatar/%d_%s", time.Now().Unix(), req.Filename)

		// 【核心改动】：使用 bytes.NewReader 将 []byte 包装成 io.Reader
		reader := bytes.NewReader(req.AvatarData)

		// 调用你现有的 OSS 工具类
		avatarUrl, err := oss.UploadFile(objectName, reader)
		if err != nil {
			return &user.UpdateUserInfoResponse{
				StatusCode: 1,
				StatusMsg:  "上传OSS失败: " + err.Error(),
			}, nil
		}

		// 将生成的 URL 存入数据库更新 Map
		updateMap["avatar"] = avatarUrl
	}

	if req.Signature != "" {
		updateMap["signature"] = req.Signature
	}

	// 3. 特殊处理密码：必须加密！
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return &user.UpdateUserInfoResponse{StatusCode: 1, StatusMsg: "密码处理失败"}, nil
		}
		updateMap["password"] = string(hashedPassword)
	}

	// 4. 检查是否有实际改动
	if len(updateMap) == 0 {
		return &user.UpdateUserInfoResponse{StatusCode: 0, StatusMsg: "未提交任何修改内容"}, nil
	}

	// 5. 执行更新：根据 Token 里的 claims.UserID 定位用户
	err = database.DB.Model(&model.User{}).Where("id = ?", claims.UserID).Updates(updateMap).Error
	if err != nil {
		return &user.UpdateUserInfoResponse{StatusCode: 1, StatusMsg: "数据库更新失败"}, nil
	}
	if err == nil {
		// 异步删除缓存
		go func(uid int64) {
			cacheKey := fmt.Sprintf("user:info:%d", uid)
			redis.RDB.Del(context.Background(), cacheKey)
			fmt.Printf("🗑️ [Async Del] 检测到资料更新，异步清理缓存: %d\n", uid)
		}(claims.UserID)
	}

	return &user.UpdateUserInfoResponse{
		StatusCode: 0,
		StatusMsg:  "修改成功",
	}, nil
}

// 这是一个内部辅助函数，逻辑和你之前的 GetUserInfo 一致
func GetUserWithCache(ctx context.Context, userID int64) (*user.User, error) {
	cacheKey := fmt.Sprintf("user:info:%d", userID)

	// 1. 查 Redis
	val, err := redis.RDB.Get(ctx, cacheKey).Result()
	if err == nil && val != "" {
		var u user.User
		if json.Unmarshal([]byte(val), &u) == nil {
			return &u, nil
		}
	}

	// 2. 查数据库 (fallback)
	var u model.User
	if err := database.DB.First(&u, userID).Error; err != nil {
		return nil, err
	}

	pbUser := &user.User{
		Id:            u.ID,
		Username:      u.Username,
		Avatar:        u.Avatar,
		FollowCount:   u.FollowCount,
		FollowerCount: u.FollowerCount,
	}
	if pbUser.Avatar == "" {
		pbUser.Avatar = "https://g-video-assets.oss-cn-wuhan-lr.aliyuncs.com/default_avatar.png"
	}

	// 3. 异步存回 Redis
	go func() {
		data, _ := json.Marshal(pbUser)
		redis.RDB.Set(context.Background(), cacheKey, data, 24*time.Hour)
	}()

	return pbUser, nil
}
