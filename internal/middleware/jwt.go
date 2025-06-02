// internal/middleware/jwt.go
package middleware

import (
	"time"

	"gin-wire-demo/internal/config"
	"gin-wire-demo/internal/service"
	"gin-wire-demo/pkg/logger"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type JWT struct {
	AuthMiddleware *jwt.GinJWTMiddleware
	logger         logger.Logger
}

func NewJWT(
	userService service.UserService,
	logger logger.Logger,
	config *config.Config,
) (*JWT, error) {
	// 创建 JWT 中间件
	authMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "gin-wire-demo",
		Key:         []byte(config.JWT.SigningKey),
		Timeout:     config.JWT.Timeout,
		MaxRefresh:  config.JWT.MaxRefresh,
		IdentityKey: "id",

		// 登录回调函数
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var login struct {
				Username string `json:"username" binding:"required"`
				Password string `json:"password" binding:"required"`
			}

			if err := c.ShouldBindJSON(&login); err != nil {
				return nil, jwt.ErrMissingLoginValues
			}

			user, err := userService.GetUserByUsername(login.Username)
			if err != nil {
				return nil, jwt.ErrFailedAuthentication
			}

			// 实际项目中应该使用 bcrypt 等安全方式验证密码
			// 这里简化处理，实际应使用：bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(login.Password))
			if user.Password != login.Password {
				return nil, jwt.ErrFailedAuthentication
			}

			return user, nil
		},

		// 登录成功后返回数据
		LoginResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			c.JSON(code, gin.H{
				"code":   code,
				"token":  token,
				"expire": expire.Format(time.RFC3339),
			})
		},

		// 身份标识处理
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return claims["id"]
		},

		// 授权处理
		Authorizator: func(data interface{}, c *gin.Context) bool {
			// 这里可以添加更复杂的授权逻辑
			return true
		},

		// Token 提取器
		TokenLookup: "header: Authorization, query: token, cookie: jwt",

		// Token 前缀
		TokenHeadName: "Bearer",

		// 时间提供器
		TimeFunc: time.Now,
	})

	if err != nil {
		return nil, err
	}

	return &JWT{
		AuthMiddleware: authMiddleware,
		logger:         logger,
	}, nil
}

func (j *JWT) MiddlewareFunc() gin.HandlerFunc {
	return j.AuthMiddleware.MiddlewareFunc()
}

func (j *JWT) LoginHandler(c *gin.Context) {
	j.AuthMiddleware.LoginHandler(c)
}

func (j *JWT) RefreshHandler(c *gin.Context) {
	j.AuthMiddleware.RefreshHandler(c)
}
