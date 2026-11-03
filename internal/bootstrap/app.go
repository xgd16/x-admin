package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
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

	hasher := security.NewBcryptHasher(cfg.Security.BcryptCost)
	tokens := security.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL)
	captchaGen := captcha.New(store, cfg.Security.CaptchaTTL)

	userRepo := repository.NewUserRepo(db)
	roleRepo := repository.NewRoleRepo(db)
	menuRepo := repository.NewMenuRepo(db)
	deptRepo := repository.NewDepartmentRepo(db)
	dictRepo := repository.NewDictRepo(db)
	paramRepo := repository.NewParameterRepo(db)
	fileRepo := repository.NewFileRepo(db)
	jobRepo := repository.NewJobRepo(db)
	logRepo := repository.NewLogRepo(db)

	logs := service.NewLogService(logRepo, 2048)
	authSvc := service.NewAuthService(userRepo, menuRepo, logRepo, store, tokens, hasher, captchaGen, service.AuthOptions{
		MaxLoginFails: cfg.Security.MaxLoginFails,
		LockDuration:  cfg.Security.LockDuration,
		RefreshTTL:    cfg.JWT.RefreshTTL,
	})
	userSvc := service.NewUserService(userRepo, roleRepo, deptRepo, hasher, cfg.Security.PasswordMinLen)
	roleSvc := service.NewRoleService(roleRepo, menuRepo)
	menuSvc := service.NewMenuService(menuRepo)
	deptSvc := service.NewDepartmentService(deptRepo)
	dictSvc := service.NewDictService(dictRepo)
	paramSvc := service.NewParameterService(paramRepo)
	fileSvc := service.NewFileService(fileRepo, local, cfg.Storage.MaxUploadMB)
	profileSvc := service.NewProfileService(userRepo, hasher, cfg.Security.PasswordMinLen)
	dashboardSvc := service.NewDashboardService(userRepo, roleRepo, menuRepo, fileRepo, jobRepo, logRepo)

	scheduler := jobs.New()
	jobSvc := service.NewJobService(jobRepo, scheduler, jobExecutor(logRepo))
	if err := jobSvc.Boot(context.Background()); err != nil {
		logs.Close()
		_ = store.Close()
		_ = database.Close(db)
		return nil, err
	}
	scheduler.Start()

	handlers := router.Handlers{
		Auth:       handler.NewAuthHandler(authSvc, captchaGen),
		Dashboard:  handler.NewDashboardHandler(dashboardSvc),
		User:       handler.NewUserHandler(userSvc),
		Role:       handler.NewRoleHandler(roleSvc),
		Menu:       handler.NewMenuHandler(menuSvc),
		Department: handler.NewDepartmentHandler(deptSvc),
		Dict:       handler.NewDictHandler(dictSvc),
		Parameter:  handler.NewParameterHandler(paramSvc),
		File:       handler.NewFileHandler(fileSvc),
		Job:        handler.NewJobHandler(jobSvc),
		Log:        handler.NewLogHandler(logs),
		Profile:    handler.NewProfileHandler(profileSvc),
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

func jobExecutor(logs *repository.LogRepo) func(ctx context.Context, job *model.Job) error {
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
		case "noop":
			return nil
		default:
			return fmt.Errorf("unknown job handler %q", job.Handler)
		}
	}
}
