package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"

	"x-admin/internal/config"
	"x-admin/internal/database"
	"x-admin/internal/handler"
	"x-admin/internal/jobs"
	"x-admin/internal/middleware"
	"x-admin/internal/model"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/security"
	"x-admin/internal/pkg/storage"
	"x-admin/internal/pkg/sysinfo"
	"x-admin/internal/repository"
	"x-admin/internal/router"
	"x-admin/internal/service"
)

// 本文件是 wire 的提供者：把配置中的原语数值变成可注入的具名依赖，
// 并承接带生命周期的构造（缓存清理、任务 Boot）。
// 纯粹的 repo→service→handler 构造器分布在各层的 wire.go ProviderSet 中。

// ---- 基础设施 ----

// provideCache 创建缓存实现；返回的清理函数由 wire 在装配失败路径上调用。
func provideCache(cfg config.Config) (cache.Store, func(), error) {
	store, err := newCache(cfg.Redis)
	if err != nil {
		return nil, nil, err
	}
	return store, func() { _ = store.Close() }, nil
}

// provideStorage 创建本地文件存储。
func provideStorage(cfg config.Config) (storage.Store, error) {
	return storage.NewLocal(cfg.Storage.Root, cfg.Storage.URLPrefix)
}

func provideTokens(cfg config.Config) *security.TokenManager {
	jwt := cfg.API.Admin.JWTFor(cfg.JWT, "admin")
	return security.NewTokenManager(jwt.Secret, jwt.Issuer, jwt.AccessTTL)
}

// buildSurfaceTokens 为每个启用的 API 面创建独立的 JWT 管理器。
//
// admin 继续使用原来的 TokenManager，保证升级后已有后台令牌仍然有效；
// client / merchant 默认使用 jwt.issuer:<name>，签发的 access token 无法跨面使用。
// 面级 api.<name>.jwt 可以覆盖 secret、issuer 和 access_ttl。
func buildSurfaceTokens(cfg config.Config, admin *security.TokenManager) map[string]*security.TokenManager {
	out := make(map[string]*security.TokenManager, len(cfg.API.Surfaces()))
	for name, surface := range cfg.API.Surfaces() {
		if name == "admin" {
			out[name] = admin
			continue
		}
		if !surface.Enabled {
			continue
		}
		jwt := surface.JWTFor(cfg.JWT, name)
		out[name] = security.NewTokenManager(jwt.Secret, jwt.Issuer, jwt.AccessTTL)
	}
	return out
}

func provideHasher(cfg config.Config) *security.BcryptHasher {
	return security.NewBcryptHasher(cfg.Security.BcryptCost)
}

func provideCaptcha(store cache.Store, cfg config.Config) *captcha.Generator {
	return captcha.New(store, cfg.Security.CaptchaTTL)
}

// provideWebAuthn 初始化失败（例如 RP ID 与来源不匹配）不会阻止应用启动，
// 只是关闭 Passkey 相关功能。
func provideWebAuthn(cfg config.Config) *webauthn.WebAuthn {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.WebAuthn.RPDisplayName,
		RPID:          cfg.WebAuthn.RPID,
		RPOrigins:     cfg.WebAuthn.RPOrigins,
	})
	if err != nil {
		slog.Warn("passkey support disabled", "error", err)
		return nil
	}
	return wa
}

// ---- 服务适配：构造参数里的配置原语 / 字面量在这里现取 ----

func provideAuthOptions(cfg config.Config) service.AuthOptions {
	jwt := cfg.API.Admin.JWTFor(cfg.JWT, "admin")
	return service.AuthOptions{
		MaxLoginFails: cfg.Security.MaxLoginFails,
		LockDuration:  cfg.Security.LockDuration,
		RefreshTTL:    jwt.RefreshTTL,
		SessionTTL:    jwt.SessionTTL,
	}
}

// provideSurfaceAuthService 为 create api --auth 生成的自定义面提供认证服务。
// 账号表由生成命令创建，服务只负责按 scope.Table 读写；refresh/session 有效期
// 按 API 面从 api.<name>.jwt 读取，未配置时沿用全局 jwt。
func provideSurfaceAuthService(
	users service.APIUserRepository,
	store cache.Store,
	hasher security.PasswordHasher,
	cfg config.Config,
	opts service.AuthOptions,
) *service.SurfaceAuthService {
	svc := service.NewSurfaceAuthService(users, store, hasher, cfg.Security.PasswordMinLen, opts)
	svc.SetSurfaceOptions(buildSurfaceAuthOptions(cfg))
	return svc
}

func buildSurfaceAuthOptions(cfg config.Config) map[string]service.AuthOptions {
	out := make(map[string]service.AuthOptions, len(cfg.API.Surfaces()))
	for name, surface := range cfg.API.Surfaces() {
		jwt := surface.JWTFor(cfg.JWT, name)
		out[name] = service.AuthOptions{
			MaxLoginFails: cfg.Security.MaxLoginFails,
			LockDuration:  cfg.Security.LockDuration,
			RefreshTTL:    jwt.RefreshTTL,
			SessionTTL:    jwt.SessionTTL,
		}
	}
	return out
}

// provideLogService 审计日志队列长度固定为 2048；清理函数关闭审计队列。
func provideLogService(repo service.LogRepository) (*service.LogService, func()) {
	svc := service.NewLogService(repo, 2048)
	return svc, svc.Close
}

func provideUserService(
	users service.UserRepository,
	roles service.RoleRepository,
	menus service.MenuRepository,
	depts service.DepartmentRepository,
	hasher security.PasswordHasher,
	notifier service.Notifier,
	cfg config.Config,
) *service.UserService {
	return service.NewUserService(users, roles, menus, depts, hasher, cfg.Security.PasswordMinLen, notifier)
}

func provideFileService(files service.FileRepository, store storage.Store, cfg config.Config) *service.FileService {
	return service.NewFileService(files, store, cfg.Storage.MaxUploadMB)
}

func provideProfileService(users service.UserRepository, hasher security.PasswordHasher, notifier service.Notifier, cfg config.Config) *service.ProfileService {
	return service.NewProfileService(users, hasher, cfg.Security.PasswordMinLen, cfg.App.Name, notifier)
}

// dbStatsProvider 把 GORM 连接池转换成监控服务需要的数据库指标。
type dbStatsProvider struct {
	db *gorm.DB
}

// Stats 读取连接池统计并执行一次带上下文的 Ping。
func (p dbStatsProvider) Stats(ctx context.Context) (sysinfo.DatabaseStats, error) {
	sqlDB, err := p.db.DB()
	if err != nil {
		return sysinfo.DatabaseStats{Status: "down"}, err
	}
	stats := sqlDB.Stats()
	out := sysinfo.DatabaseStats{
		Status:         "up",
		Open:           stats.OpenConnections,
		InUse:          stats.InUse,
		Idle:           stats.Idle,
		MaxOpen:        stats.MaxOpenConnections,
		WaitCount:      stats.WaitCount,
		WaitDurationMS: stats.WaitDuration.Milliseconds(),
	}
	start := time.Now()
	if err := sqlDB.PingContext(ctx); err != nil {
		out.Status = "down"
		out.LatencyMS = float64(time.Since(start).Microseconds()) / 1000
		return out, err
	}
	out.LatencyMS = float64(time.Since(start).Microseconds()) / 1000
	return out, nil
}

// provideDBStatsProvider 将 GORM 连接绑定为监控端口实现。
func provideDBStatsProvider(db *gorm.DB) service.DBStatsProvider {
	return dbStatsProvider{db: db}
}

// provideMonitorService 创建系统监控服务；版本号为空时使用构建信息。
func provideMonitorService(store cache.Store, dbStats service.DBStatsProvider, cfg config.Config) *service.MonitorService {
	return service.NewMonitorService(store, cfg.Redis.Driver, dbStats, cfg.App.Name, cfg.App.Env, cfg.App.Version)
}

// providePasskeyService 会话有效期固定为 5 分钟。
func providePasskeyService(
	users service.UserRepository,
	passkeys service.PasskeyRepository,
	hasher security.PasswordHasher,
	store cache.Store,
	wa *webauthn.WebAuthn,
	notifier service.Notifier,
) *service.PasskeyService {
	return service.NewPasskeyService(users, passkeys, hasher, store, wa, 5*time.Minute, notifier)
}

// provideJobService 构造任务服务，并把数据库中启用的任务注册进调度器。
// Boot 失败会中止装配，wire 自动回收此前创建的缓存与审计队列。
func provideJobService(
	repo service.JobRepository,
	scheduler service.JobScheduler,
	executor func(ctx context.Context, job *model.Job) error,
	store cache.Store,
) (*service.JobService, error) {
	svc := service.NewJobService(repo, scheduler, executor, store)
	if err := svc.Boot(context.Background()); err != nil {
		return nil, err
	}
	return svc, nil
}

// provideExportService 构造导出服务：注册各业务模块的导出构建器并启动 worker。
// 清理函数关闭队列并等待在途任务收尾。
func provideExportService(
	tasks service.ExportTaskRepository,
	store storage.Store,
	notifier service.Notifier,
	params service.ParameterRepository,
	users *service.UserService,
	logs *service.LogService,
	cfg config.Config,
) (*service.ExportService, func()) {
	svc := service.NewExportService(tasks, store, notifier, params, service.ExportOptions{
		Workers:   cfg.Export.Workers,
		QueueSize: cfg.Export.QueueSize,
		BatchSize: cfg.Export.BatchSize,
		MaxRows:   cfg.Export.MaxRows,
	})
	for module, b := range users.ExportBuilders() {
		svc.Register(module, b)
	}
	for module, b := range logs.ExportBuilders() {
		svc.Register(module, b)
	}
	svc.Boot()
	return svc, svc.Close
}

// provideJobExecutor 把数据库中的 handler 名称映射到具体执行逻辑。
// 参数统一为 JSON，例如 {"days": 90}；解析失败时使用默认保留天数。
func provideJobExecutor(
	logs *repository.LogRepo,
	notifications *repository.NotificationRepo,
	exports *service.ExportService,
	cfg config.Config,
) func(ctx context.Context, job *model.Job) error {
	return func(ctx context.Context, job *model.Job) error {
		switch job.Handler {
		case "clean_logs":
			return logs.CleanBefore(ctx, daysFrom(job.Params))
		case "clean_notifications":
			return notifications.CleanBefore(ctx, daysFrom(job.Params))
		case "clean_export_tasks":
			before := time.Now().AddDate(0, 0, -keepDaysFrom(job.Params, cfg.Export.KeepDays))
			return exports.CleanBefore(ctx, before)
		case "noop":
			// 空任务，用于验证调度链路。
			return nil
		default:
			return fmt.Errorf("unknown job handler %q", job.Handler)
		}
	}
}

// daysFrom 解析任务参数中的保留天数，缺省 90。
func daysFrom(params string) int {
	var p struct {
		Days int `json:"days"`
	}
	if params != "" && json.Unmarshal([]byte(params), &p) == nil && p.Days > 0 {
		return p.Days
	}
	return 90
}

// keepDaysFrom 解析任务参数中的保留天数；缺省回退到导出配置值。
func keepDaysFrom(params string, fallback int) int {
	var p struct {
		Days int `json:"days"`
	}
	if params != "" && json.Unmarshal([]byte(params), &p) == nil && p.Days > 0 {
		return p.Days
	}
	if fallback <= 0 {
		return 7
	}
	return fallback
}

// ---- 处理器与路由 ----

func provideAuthHandler(
	auth *service.AuthService,
	gen *captcha.Generator,
	passkeys *service.PasskeyService,
	tokens *security.TokenManager,
	cfg config.Config,
) *handler.AuthHandler {
	jwt := cfg.API.Admin.JWTFor(cfg.JWT, "admin")
	return handler.NewAuthHandler(auth, gen, passkeys, tokens, cfg.App.Env == "production", jwt.RefreshTTL)
}

// provideRouter 装配 Gin 引擎；健康检查同时探测数据库与缓存。
func provideRouter(
	cfg config.Config,
	db *gorm.DB,
	tokens *security.TokenManager,
	auth *service.AuthService,
	surfaceAuth *service.SurfaceAuthService,
	gen *captcha.Generator,
	store cache.Store,
	recorder middleware.OperationRecorder,
	handlers router.Handlers,
) *gin.Engine {
	return router.New(router.Deps{
		Config:             cfg,
		Tokens:             tokens,
		Auth:               auth,
		SurfaceAuthService: surfaceAuth,
		Captcha:            gen,
		Cache:              store,
		Recorder:           recorder,
		Handlers:           handlers,
		SurfaceTokens:      buildSurfaceTokens(cfg, tokens),
		Health: func(ctx context.Context) error {
			if err := database.Health(db); err != nil {
				return fmt.Errorf("database: %w", err)
			}
			return store.Ping(ctx)
		},
	})
}

// assembleApp 是注入器的收尾：启动调度器并组装 App。
func assembleApp(
	cfg config.Config,
	db *gorm.DB,
	store cache.Store,
	local storage.Store,
	tokens *security.TokenManager,
	scheduler *jobs.Cron,
	logs *service.LogService,
	jobSvc *service.JobService,
	engine *gin.Engine,
) *App {
	scheduler.Start()
	return &App{
		Config: cfg, DB: db, Cache: store, Storage: local, Tokens: tokens,
		Router: engine, Scheduler: scheduler, Logs: logs, Jobs: jobSvc,
	}
}
