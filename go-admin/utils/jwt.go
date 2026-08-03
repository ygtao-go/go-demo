package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// CustomClaims JWT 载体
type CustomClaims struct {
	UserId    uint   `json:"userId"`
	JTI       string `json:"jti"`       // 唯一 Token ID
	TokenType string `json:"tokenType"` // "access" 或 "refresh"
	jwt.RegisteredClaims
}

// 从环境变量读取密钥（避免硬编码）
func getAccessSecret() []byte {
	secret := os.Getenv("JWT_ACCESS_SECRET")
	if secret == "" {
		secret = "GoAdminAccessSecret2026" // 默认值
	}
	return []byte(secret)
}

func getRefreshSecret() []byte {
	secret := os.Getenv("JWT_REFRESH_SECRET")
	if secret == "" {
		secret = "GoAdminRefreshSecret2026" // 默认值
	}
	return []byte(secret)
}

// generateJTI 生成唯一 Token ID（16 位十六进制字符串）
func generateJTI() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// JTIHash 对 JTI 做 SHA256 摘要，缩短黑名单 key
func JTIHash(jti string) string {
	h := sha256.Sum256([]byte(jti))
	return hex.EncodeToString(h[:8]) // 取前 8 字节 = 16 位十六进制
}

// GenerateAccessToken 生成 Access Token（短时效）
func GenerateAccessToken(userId uint) (string, string, error) {
	jti := generateJTI()
	now := time.Now()

	claims := CustomClaims{
		UserId:    userId,
		JTI:       jti,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(getAccessSecret())
	if err != nil {
		return "", "", err
	}
	return tokenStr, jti, nil
}

// GenerateRefreshToken 生成 Refresh Token（长效）
func GenerateRefreshToken(userId uint) (string, string, error) {
	jti := generateJTI()
	now := time.Now()

	claims := CustomClaims{
		UserId:    userId,
		JTI:       jti,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(getRefreshSecret())
	if err != nil {
		return "", "", err
	}
	return tokenStr, jti, nil
}

// Deprecated: GenerateTokenPair 为 JWT 双 Token 迁移前的兼容函数，已无任何内部调用方。
// 保留以免外部调用方中断；新代码请使用 GenerateAccessToken + GenerateRefreshToken。
func GenerateTokenPair(userId uint) (accessToken, refreshToken string, err error) {
	accessToken, _, err = GenerateAccessToken(userId)
	if err != nil {
		return "", "", err
	}
	refreshToken, _, err = GenerateRefreshToken(userId)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

// ParseAccessToken 解析并验证 Access Token
func ParseAccessToken(tokenStr string) (*CustomClaims, error) {
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return getAccessSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token无效")
	}
	if claims.TokenType != "access" {
		return nil, errors.New("token类型错误")
	}
	return claims, nil
}

// ParseRefreshToken 解析并验证 Refresh Token
func ParseRefreshToken(tokenStr string) (*CustomClaims, error) {
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return getRefreshSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("token无效")
	}
	if claims.TokenType != "refresh" {
		return nil, errors.New("token类型错误")
	}
	return claims, nil
}
