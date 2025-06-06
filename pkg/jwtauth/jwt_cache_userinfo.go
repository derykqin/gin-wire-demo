package jwtauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gin-wire-demo/internal/config"
	"gin-wire-demo/internal/model"
	"gin-wire-demo/internal/service"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

var (
	Cacheuserinfokey = "cache:%s:jwt:mid:ui:%d"
)

type JwtCacheUserinfo struct {
	RedisClient *redis.Client
	Config      *config.Config
	UserService service.UserService
}

func NewJwtCacheUserinfo(
	client *redis.Client,
	config *config.Config,
	userService service.UserService,
) *JwtCacheUserinfo {
	return &JwtCacheUserinfo{
		RedisClient: client,
		Config:      config,
		UserService: userService,
	}
}

// 获取用户信息（带缓存）
func (jc *JwtCacheUserinfo) GetUserWithCache(userID uint) (*model.User, bool, error) {
	key := fmt.Sprintf(Cacheuserinfokey, jc.Config.App.Name, userID)
	// 1. 尝试从缓存获取
	if user := jc.getCacheUserinfo(key); user != nil {
		// 空对象表示用户不存在
		if user.ID == 0 {
			return nil, true, nil
		}
		return user, true, nil
	}

	// 2. 查询数据库
	user, err := jc.UserService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 缓存空对象防止穿透
			u := &model.User{}
			u.ID = 0
			jc.setCacheUserinfo(key, u)
		}
		return nil, false, err
	}
	user.Password = ""
	// 3. 设置缓存
	jc.setCacheUserinfo(key, user)
	return user, false, nil
}

func (jc *JwtCacheUserinfo) getCacheUserinfo(key string) *model.User {
	cached, err := jc.RedisClient.Get(context.Background(), key).Result()
	switch {
	case err == redis.Nil:
		return nil
	case err != nil:
		return nil
	case cached == "":
		jc.ClearCacheUserinfo(key)
		return nil
	}
	var user model.User
	if err := json.Unmarshal([]byte(cached), &user); err != nil {
		jc.ClearCacheUserinfo(key)
		return nil
	}

	return &user
}

func (jc *JwtCacheUserinfo) setCacheUserinfo(key string, u *model.User) error {
	marshaled, err := json.Marshal(u)
	if err != nil {
		return err
	}
	jc.RedisClient.Set(context.Background(), key, string(marshaled), jc.Config.JWT.CacheDuration)
	return nil
}

func (jc *JwtCacheUserinfo) ClearCacheUserinfo(key string) {
	jc.RedisClient.Del(context.Background(), key)
}
