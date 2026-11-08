package testsupport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

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

type Repos struct {
	Users         *repository.UserRepo
	Roles         *repository.RoleRepo
	Menus         *repository.MenuRepo
	Depts         *repository.DepartmentRepo
	Dicts         *repository.DictRepo
	Params        *repository.ParameterRepo
	Files         *repository.FileRepo
	Jobs          *repository.JobRepo
	Logs          *repository.LogRepo
	Notifications *repository.NotificationRepo
	Passkeys      *repository.PasskeyRepo
	Exports       *repository.ExportTaskRepo
}

type Services struct {
	Auth          *service.AuthService
	Users         *service.UserService
	Roles         *service.RoleService
	Menus         *service.MenuService
	Depts         *service.DepartmentService
	Dicts         *service.DictService
	Params        *service.ParameterService
	Files         *service.FileService
	Jobs          *service.JobService
	Logs          *service.LogService
	Dashboard     *service.DashboardService
	Profile       *service.ProfileService
	Notifications *service.NotificationService
	Passkeys      *service.PasskeyService
	Exports       *service.ExportService
}

type App struct {
	Config    config.Config
	DB        *gorm.DB
	Cache     *cache.Memory
	Storage   *storage.Local
	Repos     Repos
	Services  Services
	Router    *gin.Engine
	Hasher    security.PasswordHasher
	Tokens    *security.TokenManager
	Scheduler *jobs.Cron
	Captcha   *captcha.Generator
}

// New builds a complete application on an isolated SQLite database and an
// in-memory cache. It is the shared harness for unit and HTTP tests.
func New(t *testing.T) *App {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	cfg := config.Default()
	cfg.JWT.Secret = "test-secret-test-secret"
	cfg.JWT.Issuer = "x-admin-test"
	cfg.JWT.AccessTTL = 2 * time.Hour
	cfg.JWT.RefreshTTL = time.Hour
	cfg.Security.BcryptCost = 4
	cfg.Security.PasswordMinLen = 8
	cfg.Storage.Root = t.TempDir()
	cfg.Storage.MaxUploadMB = 1

	local, err := storage.NewLocal(cfg.Storage.Root, cfg.Storage.URLPrefix)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	store := cache.NewMemory()
	hasher := security.NewBcryptHasher(4)
	tokens := security.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL)
	captchaGen := captcha.New(store, cfg.Security.CaptchaTTL)

	repos := Repos{
		Users: repository.NewUserRepo(db), Roles: repository.NewRoleRepo(db),
		Menus: repository.NewMenuRepo(db), Depts: repository.NewDepartmentRepo(db),
		Dicts: repository.NewDictRepo(db), Params: repository.NewParameterRepo(db),
		Files: repository.NewFileRepo(db), Jobs: repository.NewJobRepo(db), Logs: repository.NewLogRepo(db),
		Notifications: repository.NewNotificationRepo(db), Passkeys: repository.NewPasskeyRepo(db),
		Exports: repository.NewExportTaskRepo(db),
	}
	logs := service.NewLogService(repos.Logs, 64)
	t.Cleanup(logs.Close)
	notifications := service.NewNotificationService(repos.Notifications)

	authSvc := service.NewAuthService(repos.Users, repos.Menus, repos.Logs, store, tokens, hasher, captchaGen, notifications,
		service.AuthOptions{
			MaxLoginFails: 5, LockDuration: 15 * time.Minute,
			RefreshTTL: cfg.JWT.RefreshTTL, SessionTTL: cfg.JWT.SessionTTL,
		})
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.WebAuthn.RPDisplayName,
		RPID:          cfg.WebAuthn.RPID,
		RPOrigins:     cfg.WebAuthn.RPOrigins,
	})
	if err != nil {
		t.Fatalf("webauthn: %v", err)
	}
	passkeySvc := service.NewPasskeyService(repos.Users, repos.Passkeys, hasher, store, wa, 5*time.Minute, notifications)
	services := Services{
		Auth:          authSvc,
		Users:         service.NewUserService(repos.Users, repos.Roles, repos.Menus, repos.Depts, hasher, 8, notifications),
		Roles:         service.NewRoleService(repos.Roles, repos.Menus),
		Menus:         service.NewMenuService(repos.Menus),
		Depts:         service.NewDepartmentService(repos.Depts),
		Dicts:         service.NewDictService(repos.Dicts),
		Params:        service.NewParameterService(repos.Params),
		Files:         service.NewFileService(repos.Files, local, cfg.Storage.MaxUploadMB),
		Profile:       service.NewProfileService(repos.Users, hasher, 8, cfg.App.Name, notifications),
		Notifications: notifications,
		Passkeys:      passkeySvc,
		Dashboard:     service.NewDashboardService(repos.Users, repos.Roles, repos.Menus, repos.Files, repos.Jobs, repos.Logs),
	}
	scheduler := jobs.New()
	services.Jobs = service.NewJobService(repos.Jobs, scheduler, func(context.Context, *model.Job) error { return nil }, store)
	services.Logs = logs

	exportSvc := service.NewExportService(repos.Exports, local, notifications, repos.Params, service.ExportOptions{
		Workers: 1, QueueSize: 4, BatchSize: 10, MaxRows: 100,
	})
	for module, b := range services.Users.ExportBuilders() {
		exportSvc.Register(module, b)
	}
	for module, b := range services.Logs.ExportBuilders() {
		exportSvc.Register(module, b)
	}
	exportSvc.Boot()
	t.Cleanup(exportSvc.Close)
	services.Exports = exportSvc

	handlers := router.Handlers{
		Auth: handler.NewAuthHandler(authSvc, captchaGen, passkeySvc, tokens, false, cfg.JWT.RefreshTTL), Dashboard: handler.NewDashboardHandler(services.Dashboard),
		User: handler.NewUserHandler(services.Users), Role: handler.NewRoleHandler(services.Roles),
		Menu: handler.NewMenuHandler(services.Menus), Department: handler.NewDepartmentHandler(services.Depts),
		Dict: handler.NewDictHandler(services.Dicts), Parameter: handler.NewParameterHandler(services.Params),
		File: handler.NewFileHandler(services.Files), Job: handler.NewJobHandler(services.Jobs),
		Log: handler.NewLogHandler(logs), Profile: handler.NewProfileHandler(services.Profile, passkeySvc, authSvc),
		Notification: handler.NewNotificationHandler(notifications),
		Export:       handler.NewExportHandler(exportSvc),
	}
	engine := router.New(router.Deps{
		Config: cfg, Tokens: tokens, Auth: authSvc, Captcha: captchaGen, Cache: store,
		Recorder: logs, Handlers: handlers, Shares: services.Files, Health: func(context.Context) error { return nil },
	})

	return &App{
		Config: cfg, DB: db, Cache: store, Storage: local, Repos: repos, Services: services,
		Router: engine, Hasher: hasher, Tokens: tokens, Scheduler: scheduler, Captcha: captchaGen,
	}
}

func (a *App) Do(method, path string, body any, token string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	switch v := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case []byte:
		reader = bytes.NewReader(v)
	case string:
		reader = bytes.NewReader([]byte(v))
	default:
		raw, _ := json.Marshal(v)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	a.Router.ServeHTTP(rr, req)
	return rr
}

// DoRaw 发送预构建的请求，供 multipart 等自定义请求场景使用。
func (a *App) DoRaw(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	a.Router.ServeHTTP(rr, req)
	return rr
}

type Envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func Decode(t *testing.T, rr *httptest.ResponseRecorder) Envelope {
	t.Helper()
	var env Envelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode response %q: %v", rr.Body.String(), err)
	}
	return env
}

func (a *App) EnsureRole(t *testing.T, code, name string) *model.Role {
	t.Helper()
	role, err := a.Repos.Roles.FindByCode(context.Background(), code)
	if err == nil {
		return role
	}
	role = &model.Role{Name: name, Code: code, Status: model.StatusEnabled, Sort: 1}
	if err := a.Repos.Roles.Create(context.Background(), role); err != nil {
		t.Fatalf("create role: %v", err)
	}
	return role
}

func (a *App) CreateUser(t *testing.T, username, password string, status int8, roleCodes ...string) *model.User {
	t.Helper()
	hash, err := a.Hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	user := &model.User{Username: username, Password: hash, Nickname: username, Status: status}
	if err := a.Repos.Users.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if len(roleCodes) > 0 {
		ids := make([]uint64, 0, len(roleCodes))
		for _, code := range roleCodes {
			ids = append(ids, a.EnsureRole(t, code, code).ID)
		}
		if err := a.Repos.Users.SetRoles(context.Background(), user.ID, ids); err != nil {
			t.Fatalf("set roles: %v", err)
		}
	}
	got, err := a.Repos.Users.Get(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return got
}

// SeedAdmin creates the built-in super administrator and returns it.
func (a *App) SeedAdmin(t *testing.T) *model.User {
	t.Helper()
	return a.CreateUser(t, "admin", "Admin@123456", model.StatusEnabled, "super_admin")
}

// Login performs the real captcha + login HTTP flow and returns the token pair.
func (a *App) Login(t *testing.T, username, password string) *service.TokenPair {
	t.Helper()
	rr := a.Do(http.MethodGet, "/admin/v1/auth/captcha", nil, "")
	env := Decode(t, rr)
	if env.Code != 0 {
		t.Fatalf("captcha failed: %s", rr.Body.String())
	}
	var challenge captcha.Challenge
	if err := json.Unmarshal(env.Data, &challenge); err != nil {
		t.Fatalf("captcha decode: %v", err)
	}
	code, err := a.Cache.Get(context.Background(), "captcha:"+challenge.ID)
	if err != nil {
		t.Fatalf("captcha code missing: %v", err)
	}
	rr = a.Do(http.MethodPost, "/admin/v1/auth/login", map[string]string{
		"username": username, "password": password,
		"captcha_id": challenge.ID, "captcha_code": code,
	}, "")
	env = Decode(t, rr)
	if env.Code != 0 {
		t.Fatalf("login failed: %s", rr.Body.String())
	}
	var pair service.TokenPair
	if err := json.Unmarshal(env.Data, &pair); err != nil {
		t.Fatalf("token decode: %v", err)
	}
	return &pair
}
