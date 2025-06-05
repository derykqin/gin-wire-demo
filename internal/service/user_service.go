// internal/service/user_service.go
package service

import (
	"errors"
	"gin-wire-demo/internal/model"
	"gin-wire-demo/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	CreateUser(user *model.User) error
	GetUserByID(id uint) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
}

type UserServiceImpl struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) *UserServiceImpl {
	return &UserServiceImpl{userRepo: userRepo}
}

func (s *UserServiceImpl) CreateUser(user *model.User) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码hash失败")
	}
	user.Password = string(hashed)
	user.Status = "active"
	return s.userRepo.Create(user)
}

func (s *UserServiceImpl) GetUserByID(id uint) (*model.User, error) {
	return s.userRepo.FindByID(id)
}

func (s *UserServiceImpl) GetUserByUsername(username string) (*model.User, error) {
	return s.userRepo.FindByUsername(username)
}
