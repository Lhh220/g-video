package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// 定义一个密钥，实际生产建议放入 config.yaml
var jwtSecret = []byte("g-video-project-secret-key-2026")

// Claims 定义 JWT 载荷中包含的信息
type Claims struct {
	UserID int64 `json:"user_id"`
	Role   int32 `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken 生成一个 JWT Token
func GenerateToken(userID int64, role int32) (string, error) {
	nowTime := time.Now()
	expireTime := nowTime.Add(24 * time.Hour) // 有效期 24 小时

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expireTime),
			IssuedAt:  jwt.NewNumericDate(nowTime),
			Issuer:    "g-video-logic-server",
		},
	}

	// 使用 HS256 算法加密
	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tokenClaims.SignedString(jwtSecret)
}

// ParseToken 解析并验证 JWT Token
func ParseToken(token string) (*Claims, error) {
	tokenClaims, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if tokenClaims != nil {
		if claims, ok := tokenClaims.Claims.(*Claims); ok && tokenClaims.Valid {
			return claims, nil
		}
	}

	return nil, errors.New("invalid token")
}
