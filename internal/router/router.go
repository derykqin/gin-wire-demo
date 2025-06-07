package router

import (
	"strings"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"gin-wire-demo/internal/config"
	"gin-wire-demo/internal/controller"
	"gin-wire-demo/internal/middleware"
	"gin-wire-demo/pkg/logger"
)

type Router struct {
	Engine *gin.Engine
	Config *config.Config
	Logger logger.Logger
}

func NewRouter(
	userController *controller.UserController,
	authMiddleware *middleware.AuthMiddleware,
	authController *controller.AuthController,
	jwtMiddleware *middleware.JWT,
	rateLimiter *middleware.RateLimiterMiddleware,
	cfg *config.Config,
	logger logger.Logger,
) *Router {
	// 设置 Gin 模式
	switch strings.ToLower(cfg.App.Mode) {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	// 添加 Zap 日志中间件
	zapLogger, ok := logger.(interface {
		GetZapLogger() *zap.Logger
	})
	if ok {
		r.Use(ginzap.Ginzap(zapLogger.GetZapLogger(), time.RFC3339, true))
		r.Use(ginzap.RecoveryWithZap(zapLogger.GetZapLogger(), true))
	} else {
		logger.Warn("zap logger not available, using default gin logger")
		r.Use(gin.Logger(), gin.Recovery())
	}

	//注册自定义验证函数
	registerValidator()

	// 公共路由
	public := r.Group("/api")
	public.Use(rateLimiter.Handle(2, 5*time.Second))
	{
		//公共路由
		public.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		public.POST("/register", userController.Register)
		public.POST("/login", authController.LoginHandler)
		// public.POST("/refresh", authController.RefreshHandler)

	}
	// 需要 JWT 认证的路由
	auth := r.Group("/api")
	auth.Use(jwtMiddleware.MiddlewareFunc())
	{
		auth.POST("/logout", authController.LogoutHandler)
		auth.GET("/userinfo", authController.UserInfo)
		auth.GET("/users/:username", userController.GetUser)
	}
	return &Router{
		Engine: r,
		Config: cfg,
		Logger: logger,
	}
}
