package middleware

import (
	"net/http"
	"redbook/internal/auth" // 导入 auth 包以便使用 ParseToken
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 验证 token 是否有效
func AuthMiddleware(session *auth.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// 检查 token 是否在黑名单
		in, _ := session.InBlackList(token)
		if in {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token invalid"})
			c.Abort()
			return
		}

		// 解析 token
		claims, err := auth.ParseToken(token) // 使用 auth 包中的 ParseToken
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// 将用户信息写入上下文
		c.Set("user_id", claims.UserID)
		c.Set("device", claims.Device)
		c.Next()
	}
}
