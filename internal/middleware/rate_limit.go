package middleware

import (
	"context"
	"fmt"
	"gin-wire-demo/internal/config"
	"gin-wire-demo/pkg/logger"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RateLimiter 限流器
type RateLimiterMiddleware struct {
	RedisClient *redis.Client
	KeyPrefix   string // Redis key前缀
	Logger      logger.Logger
}

func NewRateLimiterMiddleware(
	redisClient *redis.Client,
	config *config.Config,
	logger logger.Logger,
) *RateLimiterMiddleware {
	keyPrefix := "cache:" + config.App.Name + ":mid:rl"
	return &RateLimiterMiddleware{
		RedisClient: redisClient,
		KeyPrefix:   keyPrefix,
		Logger:      logger,
	}
}

// RateLimiter 限流中间件
func (rl *RateLimiterMiddleware) Handle(limit int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("%s:%s", rl.KeyPrefix, c.ClientIP())
		now := time.Now().UnixMilli() // 毫秒时间戳
		windowStart := now - window.Milliseconds()
		ttlSeconds := int64(math.Ceil(window.Seconds()))

		// 原子化执行Lua脚本
		script := `
        local key = KEYS[1]
        local now = tonumber(ARGV[1])
        local window_start = tonumber(ARGV[2])
        local limit = tonumber(ARGV[3])
        local window_ttl = tonumber(ARGV[4])
        local member = ARGV[5]
        
        -- 移除旧时间戳
        redis.call('ZREMRANGEBYSCORE', key, 0, window_start)
        
        -- 获取当前请求数
        local count = redis.call('ZCARD', key)
        
        if count >= limit then
            return 0
        end
        
        -- 记录当前请求
        redis.call('ZADD', key, now, member)
        redis.call('EXPIRE', key, window_ttl)
        return 1
        `

		// 生成唯一成员ID (时间戳+随机数)
		member := fmt.Sprintf("%d:%d", now, rand.Intn(10000))

		// 创建带超时的独立上下文
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// 使用请求的上下文
		res, err := rl.RedisClient.Eval(ctx, script, []string{key},
			now, windowStart, limit, ttlSeconds, member).Result()

		if err != nil {
			// 修复：添加详细的错误日志记录
			rl.Logger.Error("Redis rate limiter error",
				zap.String("key", key),
				zap.Int64("limit", limit),
				zap.Duration("window", window),
				zap.Error(err))

			// 修复：继续处理请求而不是拒绝（fail-open策略）
			// 在实际生产环境中，这取决于您的安全要求
			c.Next()
			return
		}

		// 处理Lua脚本返回结果
		if result, ok := res.(int64); !ok || result == 0 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests",
			})
			return
		}

		c.Next()
	}
}
