// internal/controller/user_controller.go
package controller

import (
	"net/http"

	"gin-wire-demo/internal/model"
	"gin-wire-demo/internal/service"
	"gin-wire-demo/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserController struct {
	userService service.UserService
	logger      logger.Logger
}

func NewUserController(
	userService service.UserService,
	logger logger.Logger,
) *UserController {
	return &UserController{
		userService: userService,
		logger:      logger.With(zap.String("module", "user_controller")),
	}
}

func (c *UserController) Register(ctx *gin.Context) {
	c.logger.Info("register request received")
	var user model.User
	if err := ctx.ShouldBindJSON(&user); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.userService.CreateUser(&user); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "user created successfully"})
}

func (c *UserController) GetUser(ctx *gin.Context) {
	username := ctx.Param("username")

	user, err := c.userService.GetUserByUsername(username)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	ctx.JSON(http.StatusOK, user)
}
