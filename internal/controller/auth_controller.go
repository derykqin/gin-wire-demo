// internal/controller/auth_controller.go
package controller

import (
	"net/http"

	"gin-wire-demo/internal/middleware"
	"gin-wire-demo/pkg/logger"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthController struct {
	jwtMiddleware *middleware.JWT
	logger        logger.Logger
}

func NewAuthController(
	jwtMiddleware *middleware.JWT,
	logger logger.Logger,
) *AuthController {
	return &AuthController{
		jwtMiddleware: jwtMiddleware,
		logger:        logger.With(zap.String("module", "auth_controller")),
	}
}

// LoginHandler 登录接口
func (c *AuthController) LoginHandler(ctx *gin.Context) {
	c.jwtMiddleware.LoginHandler(ctx)
}

// RefreshHandler 刷新 Token 接口
func (c *AuthController) RefreshHandler(ctx *gin.Context) {
	c.jwtMiddleware.RefreshHandler(ctx)
}

// UserInfo 获取用户信息
func (c *AuthController) UserInfo(ctx *gin.Context) {
	claims := jwt.ExtractClaims(ctx)

	c.logger.Info("user info requested",
		zap.Any("claims", claims),
	)

	ctx.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"data": claims,
	})
}
