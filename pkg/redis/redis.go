// pkg/redis/redis.go
package redis

import (
	"context"
	"fmt"
	"gin-wire-demo/internal/config"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

func NewRedisClient(cfg *config.Config) (*redis.Client, func(), error) {
	options := &redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password, // 使用配置中的密码而不是硬编码空字符串
		DB:           cfg.Redis.DB,       // 使用配置中的DB编号
		PoolSize:     cfg.Redis.PoolSize, // 使用配置中的连接池大小
		DialTimeout:  time.Duration(cfg.Redis.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeout) * time.Second,
		PoolTimeout:  time.Duration(cfg.Redis.PoolTimeout) * time.Second,
	}

	client := redis.NewClient(options)

	// 使用独立的context进行ping测试
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if _, err := client.Ping(pingCtx).Result(); err != nil {
		// 如果ping失败，立即关闭连接
		_ = client.Close()
		return nil, nil, fmt.Errorf("redis ping failed: %w", err)
	}

	cleanup := func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close redis client: %v", err)
		}
	}

	return client, cleanup, nil
}
