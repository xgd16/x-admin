// Package bootstrap 负责应用的依赖装配。
//
// 启动顺序：配置 → 数据库/迁移 → 缓存 → 存储 → 仓储 → 服务 → 路由。
// 该包是唯一允许同时依赖各层实现的地方，其他包只依赖接口。
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"x-admin/internal/config"
	"x-admin/internal/database"
	"x-admin/internal/handler"
	"x-admin/internal/jobs"
	"x-admin/internal/model"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/security"
	"x-admin/internal/pkg/storage"
	"x-admin/internal/repository"
	"x-admin/internal/router"
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
}

// openDB 是数据库连接入口，测试可替换为 SQLite 以覆盖完整装配流程。
var openDB = database.Open

// New 创建并装配整个应用。
// 任一步骤失败都会按相反顺序回收已创建的资源，避免连接泄漏。
func New(cfg config.Config) (*App, error) {
	db, err := openDB(cfg.Database, cfg.App.Env)
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

	store, err := newCache(cfg.Redis)
	if err != nil {
		_ = database.Close(db)
		return nil, err
	}
	local, err := storage.NewLocal(cfg.Storage.Root, cfg.Storage.URLPrefix)
	if err != nil {
		_ = store.Close()
		_ = database.Close(db)
		return nil, err
	}

	// 无状态基础设施：密码哈希、JWT、验证码。
	hasher := security.NewBcryptHasher(cfg.Security.BcryptCost)
	tokens := security.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL)
	captchaGen := captcha.New(store, cfg.Security.CaptchaTTL)

	// 仓储层：每个聚合一个仓储，只暴露 service 需要的方法。
	userRepo := repository.NewUserRepo(db)
	roleRepo := repository.NewRoleRepo(db)
	menuRepo := repository.NewMenuRepo(db)
	deptRepo := repository.NewDepartmentRepo(db)
	dictRepo := repository.NewDictRepo(db)
	paramRepo := repository.NewParameterRepo(db)
	fileRepo := repository.NewFileRepo(db)
	jobRepo := repository.NewJobRepo(db)
	logRepo := repository.NewLogRepo(db)
	notificationRepo := repository.NewNotificationRepo(db)
	passkeyRepo := repository.NewPasskeyRepo(db)

	// 服务层：通过接口注入仓储，方便单元测试替换为 fake。
	logs := service.NewLogService(logRepo, 2048)
	notifications := service.NewNotificationService(notificationRepo)
	authSvc := service.NewAuthService(userRepo, menuRepo, logRepo, store, tokens, hasher, captchaGen, notifications, service.AuthOptions{
		MaxLoginFails: cfg.Security.MaxLoginFails,
		LockDuration:  cfg.Security.LockDuration,
		RefreshTTL:    cfg.JWT.RefreshTTL,
		SessionTTL:    cfg.JWT.SessionTTL,
	})
	userSvc := service.NewUserService(userRepo, roleRepo, menuRepo, deptRepo, hasher, cfg.Security.PasswordMinLen, notifications)
	roleSvc := service.NewRoleService(roleRepo, menuRepo)
	menuSvc := service.NewMenuService(menuRepo)
	deptSvc := service.NewDepartmentService(deptRepo)
	dictSvc := service.NewDictService(dictRepo)
	paramSvc := service.NewParameterService(paramRepo)
	fileSvc := service.NewFileService(fileRepo, local, cfg.Storage.MaxUploadMB)
	profileSvc := service.NewProfileService(userRepo, hasher, cfg.Security.PasswordMinLen, cfg.App.Name, notifications)
	// WebAuthn 初始化失败（例如 RP ID 与来源不匹配）不会阻止应用启动，
	// 只是关闭 Passkey 相关功能。
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.WebAuthn.RPDisplayName,
		RPID:          cfg.WebAuthn.RPID,
		RPOrigins:     cfg.WebAuthn.RPOrigins,
	})
	if err != nil {
		slog.Warn("passkey support disabled", "error", err)
		wa = nil
	}
	passkeySvc := service.NewPasskeyService(userRepo, passkeyRepo, hasher, store, wa, 5*time.Minute, notifications)
	dashboardSvc := service.NewDashboardService(userRepo, roleRepo, menuRepo, fileRepo, jobRepo, logRepo)

	// 定时任务：启动时把数据库中启用的任务注册到调度器。
	scheduler := jobs.New()
	jobSvc := service.NewJobService(jobRepo, scheduler, jobExecutor(logRepo, notificationRepo), store)
	if err := jobSvc.Boot(context.Background()); err != nil {
		logs.Close()
		_ = store.Close()
		_ = database.Close(db)
		return nil, err
	}
	scheduler.Start()

	handlers := router.Handlers{
		Auth:         handler.NewAuthHandler(authSvc, captchaGen, passkeySvc, tokens, cfg.App.Env == "production", cfg.JWT.RefreshTTL),
		Dashboard:    handler.NewDashboardHandler(dashboardSvc),
		User:         handler.NewUserHandler(userSvc),
		Role:         handler.NewRoleHandler(roleSvc),
		Menu:         handler.NewMenuHandler(menuSvc),
		Department:   handler.NewDepartmentHandler(deptSvc),
		Dict:         handler.NewDictHandler(dictSvc),
		Parameter:    handler.NewParameterHandler(paramSvc),
		File:         handler.NewFileHandler(fileSvc),
		Job:          handler.NewJobHandler(jobSvc),
		Log:          handler.NewLogHandler(logs),
		Profile:      handler.NewProfileHandler(profileSvc, passkeySvc, authSvc),
		Notification: handler.NewNotificationHandler(notifications),
	}

	engine := router.New(router.Deps{
		Config:   cfg,
		Tokens:   tokens,
		Auth:     authSvc,
		Captcha:  captchaGen,
		Cache:    store,
		Recorder: logs,
		Handlers: handlers,
		Health: func(ctx context.Context) error {
			if err := database.Health(db); err != nil {
				return fmt.Errorf("database: %w", err)
			}
			return store.Ping(ctx)
		},
	})

	return &App{
		Config: cfg, DB: db, Cache: store, Storage: local, Tokens: tokens,
		Router: engine, Scheduler: scheduler, Logs: logs,
	}, nil
}

// Close 按依赖顺序关闭应用资源：审计队列 → 调度器 → 缓存 → 数据库。
func (a *App) Close() {
	if a.Logs != nil {
		a.Logs.Close()
	}
	if a.Scheduler != nil {
		a.Scheduler.Stop()
	}
	if a.Cache != nil {
		_ = a.Cache.Close()
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

// jobExecutor 把数据库中的 handler 名称映射到具体执行逻辑。
// 参数统一为 JSON，例如 {"days": 90}；解析失败时使用默认保留天数。
func jobExecutor(logs *repository.LogRepo, notifications *repository.NotificationRepo) func(ctx context.Context, job *model.Job) error {
	return func(ctx context.Context, job *model.Job) error {
		switch job.Handler {
		case "clean_logs":
			days := 90
			if job.Params != "" {
				var params struct {
					Days int `json:"days"`
				}
				if err := json.Unmarshal([]byte(job.Params), &params); err == nil && params.Days > 0 {
					days = params.Days
				}
			}
			return logs.CleanBefore(ctx, days)
		case "clean_notifications":
			days := 90
			if job.Params != "" {
				var params struct {
					Days int `json:"days"`
				}
				if err := json.Unmarshal([]byte(job.Params), &params); err == nil && params.Days > 0 {
					days = params.Days
				}
			}
			return notifications.CleanBefore(ctx, days)
		case "noop":
			// 空任务，用于验证调度链路。
			return nil
		default:
			return fmt.Errorf("unknown job handler %q", job.Handler)
		}
	}
}
