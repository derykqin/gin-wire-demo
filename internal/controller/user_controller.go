// internal/controller/user_controller.go
package controller

import (
	"errors"
	"net/http"

	"gin-wire-demo/internal/model"
	"gin-wire-demo/internal/service"
	"gin-wire-demo/internal/utils"
	"gin-wire-demo/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	ErrRegisterFail   = errors.New("用户注册失败")
	ErrValidationfail = errors.New("参数校验失败")
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
	var user model.User
	if err := ctx.ShouldBindJSON(&user); err != nil {
		utils.Error(ctx, http.StatusBadRequest, ErrValidationfail.Error())
		return
	}
	if err := c.userService.CreateUser(&user); err != nil {
		utils.Error(ctx, http.StatusInternalServerError, ErrRegisterFail.Error())
		return
	}

	utils.Success(ctx, "user created successfully")
}

func (c *UserController) GetUser(ctx *gin.Context) {
	username := ctx.Param("username")

	user, err := c.userService.GetUserByUsername(username)
	if err != nil {
		utils.Error(ctx, http.StatusNotFound, "user not found")
		return
	}
	user.Password = ""
	utils.Success(ctx, user)
}
