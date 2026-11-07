package database_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"x-admin/internal/config"
	"x-admin/internal/database"
)

// testMySQLConfig 依次从环境变量、configs/config.yaml、内置默认值解析测试用 MySQL 配置。
// 找不到可用的 MySQL 时测试跳过，不阻塞无外部依赖的 CI 环境。
func testMySQLConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join("..", "..", "..", "configs", "config.yaml"))
	require.NoError(t, err)
	if dsn := os.Getenv("X_ADMIN_TEST_MYSQL_DSN"); dsn != "" {
		cfg.Database.DSN = dsn
	}
	admin, err := sql.Open("mysql", serverDSN(cfg.Database.DSN))
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = admin.PingContext(ctx)
		admin.Close()
		cancel()
	}
	if err != nil {
		t.Skipf("mysql unavailable: %v", err)
	}
	return cfg
}

// serverDSN 去掉库名，得到可做服务器级操作（建库、迁移元数据）的 DSN。
func serverDSN(dsn string) string {
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		return dsn
	}
	parsed.DBName = ""
	return parsed.FormatDSN()
}

// TestOpenMigrateHealthWithMySQL 走通 Open 成功路径（含 Ping、连接池参数）
// 以及 Migrate / MigrateSQL 的正常升级路径。
func TestOpenMigrateHealthWithMySQL(t *testing.T) {
	cfg := testMySQLConfig(t)
	cfg.Database.MaxOpenConns = 2
	cfg.Database.MaxIdleConns = 1
	cfg.Database.ConnMaxLifetime = time.Minute

	db, err := database.Open(cfg.Database, "test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close(db) })

	require.NoError(t, database.Health(db))
	// 迁移幂等：重复执行等价于 ErrNoChange。
	require.NoError(t, database.Migrate(db))
	require.NoError(t, database.Migrate(db))
}
