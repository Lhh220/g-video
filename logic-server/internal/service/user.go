package service

import (
	"context"

	"github.com/Lhh220/g-video/api/proto/user" // 确保你的pb路径正确
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
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
