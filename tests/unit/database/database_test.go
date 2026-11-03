package database_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"x-admin/internal/database"
)

func TestHealthAndClose(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:db-test?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(db))
	require.NoError(t, database.Health(db))
	require.NoError(t, database.Close(db))
	assert.NotNil(t, db)
}

func TestNormalizeDSN(t *testing.T) {
	assert.Equal(t, "dsn?multiStatements=true", database.NormalizeDSN("dsn"))
	assert.Equal(t, "dsn?a=b&multiStatements=true", database.NormalizeDSN("dsn?a=b"))
	assert.Equal(t, "dsn?multiStatements=false", database.NormalizeDSN("dsn?multiStatements=false"))
}
