package bootstrap

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"x-admin/internal/config"
	"x-admin/internal/database"
	"x-admin/internal/model"
	"x-admin/internal/repository"
)

func sqliteOpener(t *testing.T) func(config.DatabaseConfig, string) (*gorm.DB, error) {
	t.Helper()
	return func(config.DatabaseConfig, string) (*gorm.DB, error) {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bootstrap.db")), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			return nil, err
		}
		if err := database.AutoMigrate(db); err != nil {
			return nil, err
		}
		return db, nil
	}
}

func TestNewWithSQLite(t *testing.T) {
	original := openDB
	openDB = sqliteOpener(t)
	defer func() { openDB = original }()

	cfg := config.Default()
	cfg.App.Env = "test"
	cfg.Database.AutoMigrate = false
	cfg.Redis.Driver = "memory"
	cfg.Storage.Root = t.TempDir()

	app, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.NotNil(t, app.Router)
	assert.NotNil(t, app.Scheduler)
	assert.NotNil(t, app.Storage)

	// 健康检查覆盖数据库 + 缓存两条路径。
	assert.NoError(t, database.Health(app.DB))
	assert.NoError(t, app.Cache.Ping(context.Background()))

	app.Close()
}

func TestNewFailsWhenRedisUnavailable(t *testing.T) {
	original := openDB
	openDB = sqliteOpener(t)
	defer func() { openDB = original }()

	cfg := config.Default()
	cfg.App.Env = "test"
	cfg.Database.AutoMigrate = false
	cfg.Redis.Driver = "redis"
	cfg.Redis.Addr = "127.0.0.1:1"
	cfg.Redis.DB = 0
	cfg.Storage.Root = t.TempDir()

	_, err := New(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
}

func TestJobExecutorHandlers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "executor.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(db))

	logs := repository.NewLogRepo(db)
	notifications := repository.NewNotificationRepo(db)
	exec := jobExecutor(logs, notifications)

	old := time.Now().AddDate(0, 0, -10)
	require.NoError(t, logs.CreateLogin(context.Background(), &model.LoginLog{
		Base: model.Base{CreatedAt: old}, Username: "old", Status: 401,
	}))
	require.NoError(t, notifications.Create(context.Background(), &model.Notification{
		Base: model.Base{CreatedAt: old}, UserID: 1, Title: "old",
	}))

	require.NoError(t, exec(context.Background(), &model.Job{Handler: "clean_logs", Params: `{"days":1}`}))
	require.NoError(t, exec(context.Background(), &model.Job{Handler: "clean_notifications", Params: `{"days":1}`}))
	require.NoError(t, exec(context.Background(), &model.Job{Handler: "noop"}))
	require.Error(t, exec(context.Background(), &model.Job{Handler: "unknown"}))

	// 非法参数回退默认保留天数，不返回错误。
	require.NoError(t, exec(context.Background(), &model.Job{Handler: "clean_logs", Params: "not-json"}))
	_, total, err := logs.ListLogins(context.Background(), model.LoginLogFilter{})
	require.NoError(t, err)
	assert.Zero(t, total)
}
