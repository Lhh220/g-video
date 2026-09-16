package utils

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestTokenRoundTrip(t *testing.T) {
	token, err := GenerateToken(42, 1)
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken 失败: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("UserID = %d, 期望 42", claims.UserID)
	}
	if claims.Role != 1 {
		t.Errorf("Role = %d, 期望 1", claims.Role)
	}
}

func TestParseTokenRejectsGarbage(t *testing.T) {
	cases := []string{
		"",
		"not-a-jwt",
		"header.payload.sig", // 结构像但内容无效
	}
	for _, c := range cases {
		if _, err := ParseToken(c); err == nil {
			t.Errorf("ParseToken(%q) 应当报错，却返回了 claims", c)
		}
	}
}

func TestParseTokenRejectsExpired(t *testing.T) {
	// 手工构造一个已过期的 token，验证过期校验生效
	claims := Claims{
		UserID: 1,
		Role:   0,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "g-video-logic-server",
		},
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("构造过期 token 失败: %v", err)
	}

	if _, err := ParseToken(expired); err == nil {
		t.Error("过期的 token 应当被拒绝")
	}
}
