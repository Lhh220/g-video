package service

import (
	"context"

	"github.com/Lhh220/g-video/api/proto/user" // 确保你的pb路径正确
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
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
