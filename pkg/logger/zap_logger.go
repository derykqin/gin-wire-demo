// pkg/logger/zap_logger.go
package logger

import (
	"gin-wire-demo/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLogger struct {
	logger *zap.Logger
}

func NewZapLogger(cfg *config.Config) (*ZapLogger, error) {
	// 设置日志级别
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		return nil, err
	}

	// 配置 Zap
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	// 构建 Logger
	zapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &ZapLogger{logger: zapLogger}, nil
}

func (l *ZapLogger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

func (l *ZapLogger) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

func (l *ZapLogger) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

func (l *ZapLogger) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

func (l *ZapLogger) Fatal(msg string, fields ...zap.Field) {
	l.logger.Fatal(msg, fields...)
}

func (l *ZapLogger) With(fields ...zap.Field) Logger {
	return &ZapLogger{logger: l.logger.With(fields...)}
}

func (l *ZapLogger) Sync() error {
	_ = l.logger.Sync()
	return nil
}

func (l *ZapLogger) GetZapLogger() *zap.Logger {
	return l.logger
}
