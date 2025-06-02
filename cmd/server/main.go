// cmd/server/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultPort          = "8080"
	defaultConfigPath    = "./configs/config.yaml" // 默认配置文件路径
	gracefulShutdownTime = 15 * time.Second
)

func main() {
	// 解析命令行参数
	var configPath string
	flag.StringVar(&configPath, "config", defaultConfigPath, "path to config file")
	flag.Parse()

	// 初始化应用
	app, cleanup, err := InitializeApp("./configs/config.yaml")
	if err != nil {
		// 使用标准日志，因为此时app.Logger可能未初始化
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize app: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	app.Logger.Info(fmt.Sprintf("📋 Using config file: %s", configPath))

	// 获取端口配置
	port := app.Config.App.Port
	if port == "" {
		port = defaultPort
	}
	addr := ":" + port

	server := &http.Server{
		Addr:    addr,
		Handler: app.Engine,
	}

	// 使用缓冲通道防止竞态
	serverErr := make(chan error, 1)
	shutdownSignal := make(chan os.Signal, 1)

	// 捕获更全面的关闭信号
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// 启动服务器
	go func() {
		app.Logger.Info(fmt.Sprintf("🚀 Starting server on %s", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.Logger.Error(fmt.Sprintf("Server failed: %v", err))
			serverErr <- err
		}
	}()

	// 等待关闭信号或服务器错误
	select {
	case sig := <-shutdownSignal:
		app.Logger.Info(fmt.Sprintf("🛑 Received signal: %s. Shutting down...", sig))
	case err := <-serverErr:
		app.Logger.Error(fmt.Sprintf("❌ Server error: %v. Shutting down...", err))
	}

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTime)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		app.Logger.Error(fmt.Sprintf("⚠️ Forced shutdown: %v (incomplete requests terminated)", err))
	} else {
		app.Logger.Info("✅ Server stopped gracefully")
	}
}
