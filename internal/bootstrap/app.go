// Package bootstrap 负责应用的依赖装配。
//
// 启动顺序：配置 → 数据库/迁移 → 缓存 → 存储 → 仓储 → 服务 → 路由。
// 装配图由 wire 生成（wire_gen.go）；提供者分布在各层 wire.go 与 providers.go。
// 该包是唯一允许同时依赖各层实现的地方，其他包只依赖接口。
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"x-admin/internal/config"
	"x-admin/internal/database"
	"x-admin/internal/jobs"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/security"
	"x-admin/internal/pkg/storage"
	"x-admin/internal/service"
)

// App 持有进程级的基础设施与路由，供 main 启动和关闭时使用。
type App struct {
	Config    config.Config
	DB        *gorm.DB
	Cache     cache.Store
	Storage   storage.Store
	Tokens    *security.TokenManager
	Router    *gin.Engine
	Scheduler *jobs.Cron
	Logs      *service.LogService
	Jobs      *service.JobService

	// cleanup 由 wire 注入器生成，关闭装配阶段创建的资源（审计队列、缓存）。
	cleanup func()
}

// New 创建并装配整个应用。
// 任一步骤失败都会按相反顺序回收已创建的资源，避免连接泄漏。
func New(cfg config.Config) (*App, error) {
	db, err := database.Open(cfg.Database, cfg.App.Env)
	if err != nil {
		return nil, err
	}
	if cfg.Database.AutoMigrate {
		if err := database.Migrate(db); err != nil {
			_ = database.Close(db)
			return nil, err
		}
		slog.Info("database migrations applied")
	}
	return newWithCleanup(cfg, db)
}

// NewWithDB 使用调用方提供的数据库连接装配应用，便于测试和嵌入式启动。
// 数据库迁移由调用方负责；App.Close 会关闭该连接。
func NewWithDB(cfg config.Config, db *gorm.DB) (*App, error) {
	return newWithCleanup(cfg, db)
}

// newWithCleanup 装配应用并接管 wire 的清理函数。
func newWithCleanup(cfg config.Config, db *gorm.DB) (*App, error) {
	app, cleanup, err := newApp(cfg, db)
	if err != nil {
		_ = database.Close(db)
		return nil, err
	}
	app.cleanup = cleanup
	return app, nil
}

// Close 按依赖顺序关闭应用资源：调度器 → 审计队列 → 缓存 → 数据库。
func (a *App) Close() {
	if a.Scheduler != nil {
		a.Scheduler.Stop()
	}
	if a.cleanup != nil {
		a.cleanup()
	}
	if a.DB != nil {
		_ = database.Close(a.DB)
	}
}

// newCache 根据配置创建缓存实现。
// 使用内存缓存时刷新令牌不会跨进程/重启保留，因此仅建议本地开发使用。
func newCache(cfg config.RedisConfig) (cache.Store, error) {
	if cfg.Driver == "memory" {
		slog.Warn("using in-memory cache; refresh tokens will not survive restart")
		return cache.NewMemory(), nil
	}
	client := redis.NewClient(&redis.Options{Addr: cfg.Addr, Password: cfg.Password, DB: cfg.DB})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}
	return cache.NewRedis(client), nil
}
