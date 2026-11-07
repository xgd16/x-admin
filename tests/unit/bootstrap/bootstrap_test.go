package bootstrap_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"x-admin/internal/bootstrap"
	"x-admin/internal/config"
	"x-admin/internal/database"
	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/repository"
)

func sqliteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bootstrap.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(db))
	return db
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.App.Env = "test"
	cfg.Database.AutoMigrate = false
	cfg.Redis.Driver = "memory"
	cfg.Storage.Root = t.TempDir()
	return cfg
}

func TestNewReturnsErrorForUnreachableDatabase(t *testing.T) {
	cfg := config.Default()
	cfg.Database.DSN = "invalid-dsn"
	_, err := bootstrap.New(cfg)
	require.Error(t, err)
}

func TestNewWithDB(t *testing.T) {
	db := sqliteDB(t)
	app, err := bootstrap.NewWithDB(testConfig(t), db)
	require.NoError(t, err)
	require.NotNil(t, app)
	require.NotNil(t, app.Router)
	require.NotNil(t, app.Scheduler)
	require.NotNil(t, app.Storage)
	require.NotNil(t, app.Jobs)

	// 健康检查覆盖数据库 + 缓存两条路径。
	assert.NoError(t, database.Health(app.DB))
	assert.NoError(t, app.Cache.Ping(context.Background()))

	app.Close()
}

func TestNewWithDBFailsWhenRedisUnavailable(t *testing.T) {
	db := sqliteDB(t)
	cfg := testConfig(t)
	cfg.Redis.Driver = "redis"
	cfg.Redis.Addr = "127.0.0.1:1"

	_, err := bootstrap.NewWithDB(cfg, db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
	require.NoError(t, database.Close(db))
}

func TestJobExecutorHandlers(t *testing.T) {
	ctx := context.Background()
	db := sqliteDB(t)
	logs := repository.NewLogRepo(db)
	notifications := repository.NewNotificationRepo(db)
	jobs := repository.NewJobRepo(db)

	old := time.Now().AddDate(0, 0, -10)
	require.NoError(t, logs.CreateLogin(ctx, &model.LoginLog{
		Base: model.Base{CreatedAt: old}, Username: "old", Status: 401,
	}))
	require.NoError(t, notifications.Create(ctx, &model.Notification{
		Base: model.Base{CreatedAt: old}, UserID: 1, Title: "old",
	}))

	cleanLogs := &model.Job{Name: "clean-logs", Cron: "0 1 * * *", Handler: "clean_logs", Params: `{"days":1}`, Status: 1}
	cleanNotifications := &model.Job{Name: "clean-notifications", Cron: "0 2 * * *", Handler: "clean_notifications", Params: `{"days":1}`, Status: 1}
	noop := &model.Job{Name: "noop", Cron: "0 3 * * *", Handler: "noop", Status: 1}
	unknown := &model.Job{Name: "unknown", Cron: "0 4 * * *", Handler: "unknown", Status: 1}
	for _, job := range []*model.Job{cleanLogs, cleanNotifications, noop, unknown} {
		require.NoError(t, jobs.Create(ctx, job))
	}

	app, err := bootstrap.NewWithDB(testConfig(t), db)
	require.NoError(t, err)
	t.Cleanup(app.Close)

	require.NoError(t, app.Jobs.RunNow(ctx, cleanLogs.ID))
	require.NoError(t, app.Jobs.RunNow(ctx, cleanNotifications.ID))
	require.NoError(t, app.Jobs.RunNow(ctx, noop.ID))
	require.Error(t, app.Jobs.RunNow(ctx, unknown.ID))

	_, total, err := logs.ListLogins(ctx, model.LoginLogFilter{})
	require.NoError(t, err)
	assert.Zero(t, total)

	// 手动触发记录为 manual，调度触发默认记录为 cron。
	rows, total, err := app.Jobs.Runs(ctx, cleanLogs.ID, pagination.Query{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	assert.Equal(t, "manual", rows[0].Trigger)

	require.NoError(t, app.Jobs.Execute(ctx, noop.ID))
	require.NoError(t, app.Jobs.Execute(ctx, noop.ID, ""))
	require.NoError(t, app.Jobs.Execute(ctx, noop.ID, "scheduler"))
	rows, total, err = app.Jobs.Runs(ctx, noop.ID, pagination.Query{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	assert.Equal(t, "cron", rows[0].Trigger)

	// 失败任务保留失败结果与状态。
	rows, total, err = app.Jobs.Runs(ctx, unknown.ID, pagination.Query{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	assert.Equal(t, model.StatusDisabled, rows[0].Status)
	assert.Contains(t, rows[0].Result, "unknown")
}

// testEnvConfig 读取开发配置（env > configs/config.yaml > 默认值）并调整为可自动跳过的环境。
// MySQL 或 Redis 不可用时测试跳过。
func testEnvConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join("..", "..", "..", "configs", "config.yaml"))
	if err != nil {
		t.Skipf("config unavailable: %v", err)
	}
	if dsn := os.Getenv("X_ADMIN_TEST_MYSQL_DSN"); dsn != "" {
		cfg.Database.DSN = dsn
	}
	cfg.App.Env = "test"
	cfg.Database.AutoMigrate = true
	cfg.Storage.Root = t.TempDir()
	return cfg
}

// TestNewFullStackWithMySQLAndRedis 走通 bootstrap.New 的完整启动路径：
// 真实 MySQL（含迁移）+ 真实 Redis + 调度器装配，最后完整关闭。
func TestNewFullStackWithMySQLAndRedis(t *testing.T) {
	cfg := testEnvConfig(t)
	app, err := bootstrap.New(cfg)
	if err != nil {
		t.Skipf("mysql/redis unavailable: %v", err)
	}
	require.NotNil(t, app.Router)
	require.NoError(t, database.Health(app.DB))
	require.NoError(t, app.Cache.Ping(context.Background()))
	app.Close()
}

// badPool 实现了 gorm.ConnPool 接口但不是 *sql.DB，用于触发 gorm.DB.DB() 的类型断言错误分支。
type badPool struct{}

func (badPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("bad pool")
}

func (badPool) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, errors.New("bad pool")
}

func (badPool) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("bad pool")
}

func (badPool) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return nil
}

func TestGormLevelErrorBranches(t *testing.T) {
	db := &gorm.DB{Config: &gorm.Config{ConnPool: badPool{}}}
	assert.Error(t, database.Migrate(db))
	assert.Error(t, database.Close(db))
	assert.Error(t, database.Health(db))
}

// TestNewWithDBPasskeyDisabled 覆盖 provideWebAuthn 初始化失败降级路径：
// RP ID 为空时 webauthn.New 返回错误，应用以"禁用 Passkey"模式正常启动。
func TestNewWithDBPasskeyDisabled(t *testing.T) {
	db := sqliteDB(t)
	cfg := testConfig(t)
	cfg.WebAuthn.RPID = ""
	app, err := bootstrap.NewWithDB(cfg, db)
	require.NoError(t, err)
	t.Cleanup(app.Close)
	require.NotNil(t, app.Router)
}

// TestNewWithDBFailsWhenJobBootFails 注册带非法 cron 的启用任务后装配应当失败，
// 该路径覆盖 wire 装配失败的清理链（缓存、审计队列按相反顺序回收）。
func TestNewWithDBFailsWhenJobBootFails(t *testing.T) {
	ctx := context.Background()
	db := sqliteDB(t)
	require.NoError(t, repository.NewJobRepo(db).Create(ctx, &model.Job{
		Name: "bad-cron", Cron: "not-a-cron", Handler: "noop", Status: 1,
	}))
	_, err := bootstrap.NewWithDB(testConfig(t), db)
	require.Error(t, err)
	require.NoError(t, database.Close(db))
}

// TestHealthEndpointOnAssembledApp 走通 bootstrap 组装出的路由里的 /healthz。
func TestHealthEndpointOnAssembledApp(t *testing.T) {
	db := sqliteDB(t)
	app, err := bootstrap.NewWithDB(testConfig(t), db)
	require.NoError(t, err)
	t.Cleanup(app.Close)

	rr := httptest.NewRecorder()
	app.Router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}
