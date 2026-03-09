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

	// 2. 查询数据库
	var u model.User
	// 根据请求中的 user_id 查找
	if err := database.DB.First(&u, req.UserId).Error; err != nil {
		return &user.UserInfoResponse{
			StatusCode: 1,
			StatusMsg:  "该用户不存在",
		}, nil
	}

	// 3. 组装返回结果
	// 注意：这里的 user.User 是你 proto 生成的结构体，不是 model.User
	return &user.UserInfoResponse{
		StatusCode: 0,
		StatusMsg:  "查询成功",
		User: &user.User{
			Id:       u.ID,
			Username: u.Username,
			Avatar:   u.Avatar,
			// 关注数和粉丝数目前没做逻辑，先预留 0
			FollowCount:   0,
			FollowerCount: 0,
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

	if req.Avatar != "" {
		updateMap["avatar"] = req.Avatar
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

	return &user.UpdateUserInfoResponse{
		StatusCode: 0,
		StatusMsg:  "修改成功",
	}, nil
}
