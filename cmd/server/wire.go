// cmd/server/wire.go
//go:build wireinject
// +build wireinject

package main

import (
	"gin-wire-demo/internal/config"
	"gin-wire-demo/internal/controller"
	"gin-wire-demo/internal/middleware"
	"gin-wire-demo/internal/repository"
	"gin-wire-demo/internal/router"
	"gin-wire-demo/internal/service"
	"gin-wire-demo/pkg/db"
	"gin-wire-demo/pkg/logger"
	"gin-wire-demo/pkg/redis"

	"github.com/google/wire"
)

var dbSet = wire.NewSet(
	db.NewMySQL,
)

var redisSet = wire.NewSet(
	redis.NewRedisClient,
)

var configSet = wire.NewSet(
	config.LoadConfig,
)

var repositorySet = wire.NewSet(
	repository.NewUserRepository,
	wire.Bind(new(repository.UserRepository), new(*repository.UserRepositoryImpl)),
)

var serviceSet = wire.NewSet(
	service.NewUserService,
	wire.Bind(new(service.UserService), new(*service.UserServiceImpl)),
)

var controllerSet = wire.NewSet(
	controller.NewUserController,
	controller.NewAuthController, // 添加 AuthController

)

var middlewareSet = wire.NewSet(
	middleware.NewAuthMiddleware,
	middleware.NewRateLimiterMiddleware,
)

var routerSet = wire.NewSet(
	router.NewRouter,
)

var loggerSet = wire.NewSet(
	logger.NewZapLogger,
	wire.Bind(new(logger.Logger), new(*logger.ZapLogger)),
)

var jwtSet = wire.NewSet(
	middleware.NewJWT,
)

// InitializeApp 初始化应用
func InitializeApp(configPath string) (*router.Router, func(), error) {
	wire.Build(
		configSet,
		dbSet,
		redisSet,
		loggerSet, // 添加日志 Set
		repositorySet,
		serviceSet,
		controllerSet,
		jwtSet, // 添加 JWT Set
		middlewareSet,
		routerSet,
	)
	return nil, nil, nil
}
