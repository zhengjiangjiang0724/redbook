package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthClaims 自定义 Claims 结构体（包含 UserID 和 Hashed Password）
type AuthClaims struct {
	UserID               uint64 `json:"user_id"`         // 用户唯一标识
	HashedPassword       string `json:"hashed_password"` // 密码哈希（如 bcrypt 结果）
	jwt.RegisteredClaims        // 继承 JWT 标准声明（ExpiresAt、IssuedAt 等）
}

// 生成 JWT 签名密钥（生产环境建议通过环境变量读取）
var jwtSecret = []byte("your-256-bit-secret-key-here") // 替换为安全生产密钥

// GenerateToken 生成用户 JWT 令牌
// 参数：
//
//	userID: 用户唯一标识
//
// 返回：
//
//	tokenString: JWT 令牌字符串
//	error: 错误信息（如密钥无效、参数错误）
func GenerateToken(userID uint64) (string, error) {
	// 构建标准声明
	// 创建 token 对象，使用 HS256 算法签名
	claims := AuthClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
