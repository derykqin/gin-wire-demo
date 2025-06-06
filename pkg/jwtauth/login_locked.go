package jwtauth

import (
	"context"
	"fmt"
	"gin-wire-demo/internal/config"

	"github.com/go-redis/redis/v8"
)

var (
	loginFailureKey = "cache:%s:login_failures:%s" // 登录失败计数器键格式
	accountLockKey  = "cache:%s:account_lock:%s"   // 账户锁定键格式
)

type LoginLocked struct {
	RedisClient *redis.Client
	Config      *config.Config
}

func NewLoginLocked(
	client *redis.Client,
	config *config.Config,
) *LoginLocked {
	return &LoginLocked{
		RedisClient: client,
		Config:      config,
	}
}

// 获取登录失败计数器键
func (ll *LoginLocked) GetLoginFailureKey(username string) string {
	return fmt.Sprintf(loginFailureKey, ll.Config.App.Name, username)
}

// 获取账户锁定键
func (ll *LoginLocked) GetAccountLockKey(username string) string {
	return fmt.Sprintf(accountLockKey, ll.Config.App.Name, username)

}

// 检查账户是否被锁定
func (ll *LoginLocked) IsAccountLocked(username string) (bool, error) {
	key := ll.GetAccountLockKey(username)
	exists, err := ll.RedisClient.Exists(context.Background(), key).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

// 增加登录失败计数
func (ll *LoginLocked) IncrementLoginFailure(username string) error {
	key := ll.GetLoginFailureKey(username)
	ctx := context.Background()

	// 使用事务保证原子操作
	txf := func(tx *redis.Tx) error {
		// 获取当前计数
		count, err := tx.Get(ctx, key).Int()
		if err != nil && err != redis.Nil {
			return err
		}

		// 增加计数
		newCount := count + 1
		if newCount >= ll.Config.JWT.MaxLoginAttempts {
			// 达到阈值，设置锁定
			lockKey := ll.GetAccountLockKey(username)
			if err := tx.Set(ctx, lockKey, "1", ll.Config.JWT.LockDuration).Err(); err != nil {
				return err
			}
			// 重置失败计数
			return tx.Del(ctx, key).Err()
		} else {
			// 更新失败计数
			return tx.Set(ctx, key, newCount, 0).Err()
		}
	}

	// 使用Watch实现乐观锁
	return ll.RedisClient.Watch(ctx, txf, key)
}

// 清除登录失败计数和锁定
func (ll *LoginLocked) ClearLoginFailures(username string) error {
	ctx := context.Background()
	_, err := ll.RedisClient.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		// 清除失败计数
		pipe.Del(ctx, ll.GetLoginFailureKey(username))
		// 清除锁定状态
		pipe.Del(ctx, ll.GetAccountLockKey(username))
		return nil
	})
	return err
}
