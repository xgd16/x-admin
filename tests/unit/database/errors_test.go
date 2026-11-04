package database_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"x-admin/internal/config"
	"x-admin/internal/database"
)

func TestOpenRejectsInvalidDSN(t *testing.T) {
	_, err := database.Open(config.DatabaseConfig{DSN: "invalid-dsn"}, "test")
	require.Error(t, err)
}

func TestMigrateErrorsAreReturned(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:migrate-error?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// MySQL 迁移驱动不接受 SQLite 连接，应返回错误而不是 panic。
	assert.Error(t, database.Migrate(db))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	assert.Error(t, database.MigrateSQL(sqlDB))
}
