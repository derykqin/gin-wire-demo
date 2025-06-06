// internal/middleware/jwt.go
package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"gin-wire-demo/internal/config"
	"gin-wire-demo/internal/model"
	"gin-wire-demo/internal/service"
	"gin-wire-demo/pkg/jwtauth"
	"gin-wire-demo/pkg/logger"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	identityKey = "id"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user account is disabled")
	ErrAccountLocked      = errors.New("account locked due to too many failed attempts")
)

type JWT struct {
	AuthMiddleware   *jwt.GinJWTMiddleware
	Logger           logger.Logger
	RedisClient      *redis.Client
	Config           *config.Config
	JwtBlacklist     *jwtauth.JwtBlacklist
	JwtLoginLocked   *jwtauth.LoginLocked
	JwtCacheUserinfo *jwtauth.JwtCacheUserinfo
}

func NewJWT(
	userService service.UserService,
	logger logger.Logger,
	config *config.Config,
	redisClient *redis.Client,
	blacklist *jwtauth.JwtBlacklist,
	loginLock *jwtauth.LoginLocked,
	cacheUserinfo *jwtauth.JwtCacheUserinfo,
) (*JWT, error) {

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
			// 1. 检查账户是否被锁定
			if locked, err := loginLock.IsAccountLocked(login.Username); err != nil {
				logger.Error(fmt.Sprintf("Account lock check error: %v", err))
			} else if locked {
				return nil, ErrAccountLocked
			}
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
				// 2. 密码错误时增加失败计数
				if err := loginLock.IncrementLoginFailure(login.Username); err != nil {
					logger.Error(fmt.Sprintf("Failed to increment login counter: %v", err))
				}
				logger.Warn(fmt.Sprintf("Invalid password for user: %s", login.Username))
				return nil, ErrInvalidCredentials
			}
			// 3. 登录成功重置失败计数
			if err := loginLock.ClearLoginFailures(login.Username); err != nil {
				logger.Warn(fmt.Sprintf("Failed to clear login failures: %v", err))
			}
			// 登录成功后清除旧缓存
			key := fmt.Sprintf(jwtauth.Cacheuserinfokey, config.App.Name, user.ID)
			cacheUserinfo.ClearCacheUserinfo(key)
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

			// 新增：检查jti是否在黑名单中
			if blacklist.IsTokenBlacklisted(c) {
				claims := jwt.ExtractClaims(c)
				jti, _ := claims["jti"].(string)
				logger.Warn(fmt.Sprintf("Token revoked (jti: %s)", jti))
				return false
			}

			// 从缓存或数据库获取用户
			user, fromCache, err := cacheUserinfo.GetUserWithCache(userID)
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
					key := fmt.Sprintf(jwtauth.Cacheuserinfokey, config.App.Name, user.ID)
					cacheUserinfo.ClearCacheUserinfo(key)
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
				now := time.Now()
				return jwt.MapClaims{
					identityKey: user.ID,
					// 标准claims
					"iss": config.App.Name,                    // 签发者
					"sub": "authentication",                   // 主题
					"exp": now.Add(config.JWT.Timeout).Unix(), // 过期时间
					"nbf": now.Unix(),                         // 生效时间（立即生效）
					"iat": now.Unix(),                         // 签发时间
					"jti": uuid.NewString(),                   // 唯一标识符（防重放）

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
		Logger:         logger,
		RedisClient:    redisClient,
		Config:         config,
		JwtBlacklist:   blacklist,
		JwtLoginLocked: loginLock,
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

	// 如果刷新成功（状态码200），将旧令牌加入黑名单（使用jti）
	if c.Writer.Status() == http.StatusOK && oldToken != "" {
		if err := j.JwtBlacklist.AddTokenBlacklist(c); err != nil {
			j.Logger.Error(fmt.Sprintf("refresh token handler fail:%v", err))
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
	if err := j.JwtBlacklist.AddTokenBlacklist(c); err != nil {
		j.Logger.Error(fmt.Sprintf("refresh token handler fail:%v", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
}
