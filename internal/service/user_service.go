// internal/service/user_service.go
package service

import (
	"gin-wire-demo/internal/model"
	"gin-wire-demo/internal/repository"
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
	return s.userRepo.Create(user)
}

func (s *UserServiceImpl) GetUserByID(id uint) (*model.User, error) {
	return s.userRepo.FindByID(id)
}

func (s *UserServiceImpl) GetUserByUsername(username string) (*model.User, error) {
	return s.userRepo.FindByUsername(username)
}
