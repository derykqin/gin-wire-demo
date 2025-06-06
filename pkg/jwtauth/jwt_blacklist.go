package jwtauth

import (
	"context"
	"errors"
	"fmt"
	"gin-wire-demo/internal/config"
	"gin-wire-demo/pkg/logger"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var (
	blacklistKey = "cache:%s:jwt:bl:%s"
)

type JwtBlacklist struct {
	RedisClient *redis.Client
	Config      *config.Config
	Logger      logger.Logger
}

func NewJwtBlacklist(
	client *redis.Client,
	config *config.Config,
	logger logger.Logger,
) *JwtBlacklist {
	return &JwtBlacklist{
		RedisClient: client,
		Config:      config,
		Logger:      logger,
	}
}

// 检查jti是否在黑名单中
func (jb *JwtBlacklist) IsTokenBlacklisted(c *gin.Context) bool {
	claims := jwt.ExtractClaims(c)
	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		return false
	}

	key := fmt.Sprintf(blacklistKey, jb.Config.App.Name, jti)
	exists, err := jb.RedisClient.Exists(context.Background(), key).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

// 退出token加入和黑名单
func (jb *JwtBlacklist) AddTokenBlacklist(c *gin.Context) error {
	// 计算令牌剩余有效期
	claims := jwt.ExtractClaims(c)
	exp, ok := claims["exp"].(float64)
	if !ok {
		return errors.New("add blacklist fail:invalid token claims")

	}

	expireTime := time.Unix(int64(exp), 0)
	remaining := time.Until(expireTime)

	// 如果令牌还有效，加入黑名单（使用jti）
	if remaining > 0 {
		jti, ok := claims["jti"].(string)
		if !ok {
			return errors.New("add blacklist fail:invalid token claims")
		}
		key := fmt.Sprintf(blacklistKey, jb.Config.App.Name, jti)
		if err := jb.RedisClient.Set(
			context.Background(),
			key,
			1,         // 值可以是任意内容
			remaining, // 设置与令牌相同的TTL
		).Err(); err != nil {
			jb.Logger.Error(fmt.Sprintf("Failed to add jti to blacklist:%b", err))
			return errors.New("add blacklist fail:internal server error")
		}
	}
	return nil
}
