// internal/middleware/auth.go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type AuthMiddleware struct {
	redisClient *redis.Client
}

func NewAuthMiddleware(redisClient *redis.Client) *AuthMiddleware {
	return &AuthMiddleware{redisClient: redisClient}
}

func (m *AuthMiddleware) Handle() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader("Authorization")
		if token == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization token required"})
			return
		}

		// 检查 Redis 中是否存在该 token
		_, err := m.redisClient.Get(ctx, token).Result()
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		ctx.Next()
	}
}
