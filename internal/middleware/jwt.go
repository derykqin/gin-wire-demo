// internal/middleware/jwt.go
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gin-wire-demo/internal/config"
	"gin-wire-demo/internal/model"
	"gin-wire-demo/internal/service"
	"gin-wire-demo/pkg/logger"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	identityKey = "id"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user account is disabled")
)

type JWT struct {
	AuthMiddleware *jwt.GinJWTMiddleware
	logger         logger.Logger
	redisClient    *redis.Client
	config         *config.Config
	blacklistKey   string // 黑名单键前缀
}

func NewJWT(
	userService service.UserService,
	logger logger.Logger,
	config *config.Config,
	redisClient *redis.Client,
) (*JWT, error) {
	prefix_key := "cache:" + config.App.Name + ":jwt:mid:ui:"
	blacklistKey := "cache:" + config.App.Name + ":jwt:bl:"

	// 创建 JWT 中间件
	authMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       config.App.Name,
		Key:         []byte(config.JWT.SigningKey),
		Timeout:     config.JWT.Timeout,
		MaxRefresh:  config.JWT.MaxRefresh,
		IdentityKey: identityKey,

		// 登录回调函数
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var login struct {
				Username string `json:"username" binding:"required"`
				Password string `json:"password" binding:"required"`
			}

			if err := c.ShouldBindJSON(&login); err != nil {
				return nil, jwt.ErrMissingLoginValues
			}

			// 增加登录审计日志
			logger.Info(fmt.Sprintf("Login attempt: %s", login.Username))

			user, err := userService.GetUserByUsername(login.Username)
			if err != nil {
				logger.Warn(fmt.Sprintf("User not found: %s", login.Username))
				return nil, ErrInvalidCredentials
			}

			// 检查用户状态
			if user.Status != "active" {
				logger.Warn(fmt.Sprintf("User account disabled: %s", login.Username))
				return nil, ErrUserDisabled
			}

			// 使用 bcrypt 安全验证密码
			if err := bcrypt.CompareHashAndPassword(
				[]byte(user.Password),
				[]byte(login.Password),
			); err != nil {
				logger.Warn(fmt.Sprintf("Invalid password for user: %s", login.Username))
				return nil, ErrInvalidCredentials
			}
			// 登录成功后清除旧缓存
			key := prefix_key + fmt.Sprintf("%d", user.ID)
			clearCacheUserinfo(redisClient, key)
			return user, nil
		},

		// 登录成功后返回数据
		LoginResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			c.JSON(code, gin.H{
				"code":    code,
				"token":   token,
				"expire":  expire.Format(time.RFC3339),
				"message": "login successful",
			})
		},

		// 身份标识处理
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			// 将 float64 类型的 ID 转换为 uint
			if id, ok := claims[identityKey].(float64); ok {
				return uint(id)
			}
			return nil
		},

		// 授权处理
		Authorizator: func(data interface{}, c *gin.Context) bool {
			userID, ok := data.(uint)
			if !ok {
				logger.Warn("Invalid JWT identity type")
				return false
			}

			// 新增：检查令牌是否在黑名单中
			token := jwt.GetToken(c)
			if isTokenBlacklisted(redisClient, blacklistKey, token) {
				logger.Warn(fmt.Sprintf("Token revoked: %s", token))
				return false
			}

			// 从缓存或数据库获取用户
			key := fmt.Sprintf("%s%d", prefix_key, userID)
			ttl := config.JWT.CacheDuration
			user, fromCache, err := getUserWithCache(redisClient, userService, key, userID, ttl)
			if err != nil {
				logger.Error(fmt.Sprintf("User lookup error: %v", err))
				return false
			}

			// 用户不存在
			if user == nil {
				return false
			}

			// 3. 验证用户状态
			if user.Status != "active" {
				// 如果是缓存数据且状态不合法，清除缓存
				if fromCache {
					clearCacheUserinfo(redisClient, key)
				}

				logger.Info(fmt.Sprintf("Inactive user access: %d", userID))
				return false
			}

			// 将用户信息存入上下文，供后续使用
			c.Set("currentUser", user)
			c.Set("userID", user.ID) // 存储常用字段

			return true
		},

		// Token 提取器
		TokenLookup: "header: Authorization, query: token, cookie: jwt",

		// Token 前缀
		TokenHeadName: "Bearer",

		// 时间提供器
		TimeFunc: time.Now,

		// 统一错误响应
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},

		// 在 JWT 中存储额外信息
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if user, ok := data.(*model.User); ok {
				return jwt.MapClaims{
					identityKey: user.ID,
				}
			}
			return jwt.MapClaims{}
		},
	})

	if err != nil {
		logger.Error(fmt.Sprintf("JWT middleware creation failed: %v", err))
		return nil, fmt.Errorf("failed to create JWT middleware: %w", err)
	}

	// 初始化中间件（重要！）
	if err := authMiddleware.MiddlewareInit(); err != nil {
		logger.Error(fmt.Sprintf("JWT middleware init failed: %v", err))
		return nil, fmt.Errorf("failed to init JWT middleware: %w", err)
	}

	return &JWT{
		AuthMiddleware: authMiddleware,
		logger:         logger,
		redisClient:    redisClient,
		config:         config,
		blacklistKey:   blacklistKey,
	}, nil
}

func (j *JWT) MiddlewareFunc() gin.HandlerFunc {
	return j.AuthMiddleware.MiddlewareFunc()
}

func (j *JWT) LoginHandler(c *gin.Context) {
	j.AuthMiddleware.LoginHandler(c)
}

func (j *JWT) RefreshHandler(c *gin.Context) {
	// 获取当前旧令牌
	oldToken := jwt.GetToken(c)

	// 调用原始刷新处理
	j.AuthMiddleware.RefreshHandler(c)

	// 如果刷新成功（状态码200），将旧令牌加入黑名单
	if c.Writer.Status() == http.StatusOK && oldToken != "" {
		claims, err := j.AuthMiddleware.GetClaimsFromJWT(c)
		if err == nil {
			if exp, ok := claims["exp"].(float64); ok {
				expireTime := time.Unix(int64(exp), 0)
				remaining := time.Until(expireTime)
				if remaining > 0 {
					key := j.blacklistKey + oldToken
					j.redisClient.Set(
						context.Background(),
						key,
						1,
						remaining,
					)
				}
			}
		}
	}
}

// 新增注销处理函数
func (j *JWT) LogoutHandler(c *gin.Context) {
	// 获取当前令牌
	token := jwt.GetToken(c)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}
	// 计算令牌剩余有效期
	claims, err := j.AuthMiddleware.GetClaimsFromJWT(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token claims"})
		return
	}

	expireTime := time.Unix(int64(exp), 0)
	remaining := time.Until(expireTime)

	// 如果令牌还有效，加入黑名单
	if remaining > 0 {
		key := j.blacklistKey + token
		if err := j.redisClient.Set(
			context.Background(),
			key,
			1,         // 值可以是任意内容
			remaining, // 设置与令牌相同的TTL
		).Err(); err != nil {
			j.logger.Error(fmt.Sprintf("Failed to add token to blacklist:%b", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
}

// 获取用户信息（带缓存）
func getUserWithCache(client *redis.Client, userService service.UserService, key string, userID uint, ttl time.Duration) (*model.User, bool, error) {
	// 1. 尝试从缓存获取
	if user := getCacheUserinfo(client, key); user != nil {
		// 空对象表示用户不存在
		if user.ID == 0 {
			return nil, true, nil
		}
		return user, true, nil
	}

	// 2. 查询数据库
	user, err := userService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 缓存空对象防止穿透
			u := &model.User{}
			u.ID = 0
			setCacheUserinfo(client, key, u, ttl)
		}
		return nil, false, err
	}

	// 3. 设置缓存
	setCacheUserinfo(client, key, user, ttl)
	return user, false, nil
}

func getCacheUserinfo(client *redis.Client, key string) *model.User {
	cached, err := client.Get(context.Background(), key).Result()
	switch {
	case err == redis.Nil:
		return nil
	case err != nil:
		return nil
	case cached == "":
		clearCacheUserinfo(client, key)
		return nil
	}
	var user model.User
	if err := json.Unmarshal([]byte(cached), &user); err != nil {
		clearCacheUserinfo(client, key)
		return nil
	}

	return &user
}

func setCacheUserinfo(client *redis.Client, key string, u *model.User, ttl time.Duration) error {
	marshaled, err := json.Marshal(u)
	if err != nil {
		return err
	}
	client.Set(context.Background(), key, string(marshaled), ttl)
	return nil
}

func clearCacheUserinfo(client *redis.Client, key string) {
	client.Del(context.Background(), key)
}

// 检查令牌是否在黑名单中
func isTokenBlacklisted(rdb *redis.Client, prefix, token string) bool {
	if token == "" {
		return false
	}

	key := prefix + token
	exists, err := rdb.Exists(context.Background(), key).Result()
	if err != nil {
		// 错误处理，记录日志但允许访问
		return false
	}
	return exists > 0
}
