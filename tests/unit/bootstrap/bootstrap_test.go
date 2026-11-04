package bootstrap_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"x-admin/internal/bootstrap"
	"x-admin/internal/config"
)

func TestNewReturnsErrorForUnreachableDatabase(t *testing.T) {
	cfg := config.Default()
	cfg.Database.DSN = "invalid-dsn"
	_, err := bootstrap.New(cfg)
	require.Error(t, err)
}
