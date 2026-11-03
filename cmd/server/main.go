// Command server 是 x-admin 管理后台的进程入口。
// 该文件只做装配与生命周期管理：加载配置、初始化应用、启动 HTTP 服务、
// 监听退出信号并优雅关闭，不包含任何业务逻辑。
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"x-admin/internal/bootstrap"
	"x-admin/internal/config"
)

// main 按“加载配置 → 装配应用 → 启动监听 → 等待退出信号”的顺序运行服务。
func main() {
	// -config 指定 YAML 配置文件路径；文件不存在时使用内置默认值。
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	setupLogger(cfg.App.Env)

	// bootstrap.New 负责数据库、缓存、仓储、服务、路由等依赖的创建与注入。
	app, err := bootstrap.New(cfg)
	if err != nil {
		slog.Error("bootstrap", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:         cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      app.Router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// HTTP 服务在独立 goroutine 中运行，主 goroutine 继续等待系统信号。
	go func() {
		slog.Info("x-admin listening", "addr", server.Addr, "env", cfg.App.Env, "version", cfg.App.Version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "error", err)
			os.Exit(1)
		}
	}()

	// 收到 SIGINT/SIGTERM 后停止接收新请求，并在超时时间内等待存量请求结束。
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("shutdown", "error", err)
	}
	// 依次关闭审计队列、定时任务、缓存和数据库连接。
	app.Close()
	slog.Info("x-admin stopped")
}

// setupLogger 根据运行环境选择日志格式：
// 生产环境使用 JSON 便于日志采集，其他环境使用可读文本并启用 Debug 级别。
func setupLogger(env string) {
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(handler))
}
