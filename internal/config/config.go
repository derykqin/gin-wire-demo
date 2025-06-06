// internal/config/config.go
package config

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Log      LogConfig      `mapstructure:"log"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	DBname          string `mapstructure:"dbname"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr         string `mapstructure:"addr"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	PoolTimeout  int    `mapstructure:"pool_timeout"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type JWTConfig struct {
	SigningKey       string        `mapstructure:"signing_key"`        // JWT 签名密钥
	Timeout          time.Duration `mapstructure:"timeout"`            // Token 过期时间
	MaxRefresh       time.Duration `mapstructure:"max_refresh"`        // 最大刷新时间
	CacheDuration    time.Duration `mapstructure:"cache_duration"`     // 用户信息缓存时间
	MaxLoginAttempts int           `mapstructure:"max_login_attempts"` // 新增
	LockDuration     time.Duration `mapstructure:"lock_duration"`      // 新增
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml") // or json, toml etc.

	// Set default values
	setDefaults()

	// Read from environment variables
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Printf("config file not found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	log.Println("config loaded successfully")
	return &cfg, nil
}

func setDefaults() {
	// App defaults
	viper.SetDefault("app.port", "8080")
	viper.SetDefault("app.mode", "debug")

	// Database defaults
	viper.SetDefault("database.host", "127.0.0.1")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.max_open_conns", 100)
	viper.SetDefault("database.conn_max_lifetime", 60)

	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.dial_timeout", 10)
	viper.SetDefault("redis.read_timeout", 30)
	viper.SetDefault("redis.write_timeout", 30)
	viper.SetDefault("redis.pool_timeout", 30)

	//jwt defaults
	viper.SetDefault("jwt.timeout", time.Hour*8)           // 默认24小时
	viper.SetDefault("jwt.max_refresh", time.Hour*24)      // 默认7天
	viper.SetDefault("jwt.cache_duration", time.Second*60) //
	viper.SetDefault("jwt.max_login_attempts", 3)          //
	viper.SetDefault("jwt.lock_duration", time.Minute*5)   //

}

func validateConfig(cfg *Config) error {
	if cfg.App.Name == "" {
		return fmt.Errorf("app name cannot be empty")
	}

	if cfg.App.Port == "" {
		return fmt.Errorf("app port cannot be empty")
	}

	if cfg.Database.Host == "" {
		return fmt.Errorf("database host cannot be empty")
	}
	if cfg.Database.Username == "" {
		return fmt.Errorf("database username cannot be empty")
	}
	if cfg.Database.Password == "" {
		return fmt.Errorf("database password cannot be empty")
	}
	if cfg.Database.DBname == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis address cannot be empty")
	}

	// 验证 JWT 配置
	if cfg.JWT.SigningKey == "" {
		return fmt.Errorf("jwt signing key cannot be empty")
	}
	if len(cfg.JWT.SigningKey) < 32 {
		return fmt.Errorf("jwt signing key must be at least 32 characters")
	}
	if cfg.JWT.Timeout <= 0 {
		return fmt.Errorf("jwt timeout must be positive")
	}
	if cfg.JWT.MaxRefresh <= 0 {
		return fmt.Errorf("jwt max refresh must be positive")
	}
	if cfg.JWT.MaxLoginAttempts <= 0 {
		return fmt.Errorf("jwt max login attempts must be positive")
	}
	if cfg.JWT.LockDuration <= 0 {
		return fmt.Errorf("jwt lock duration must be positive")
	}
	return nil
}
