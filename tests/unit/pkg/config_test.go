package pkg_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/config"
)

func TestConfigDefaultsAndMissingFile(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "x-admin", cfg.App.Name)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 2*time.Hour, cfg.JWT.AccessTTL)
	assert.Equal(t, 12*time.Hour, cfg.JWT.SessionTTL)
	assert.Equal(t, 7*24*time.Hour, cfg.JWT.RefreshTTL)
	assert.Equal(t, 5, cfg.Security.MaxLoginFails)
}

func TestConfigFileAndEnvOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
app:
  name: custom
  env: development
server:
  port: 9090
jwt:
  secret: custom-secret-value
  access_ttl: 30m
storage:
  root: /tmp/custom
cors:
  origins:
    - http://a.test
`), 0o600))

	t.Setenv("X_ADMIN_SERVER_PORT", "9191")
	t.Setenv("X_ADMIN_REDIS_DRIVER", "memory")
	t.Setenv("X_ADMIN_REDIS_DB", "3")
	t.Setenv("X_ADMIN_STORAGE_MAX_MB", "42")
	t.Setenv("X_ADMIN_CORS_ORIGINS", "http://b.test, http://c.test")
	t.Setenv("X_ADMIN_JWT_ACCESS_TTL", "45m")
	t.Setenv("X_ADMIN_TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8")
	t.Setenv("X_ADMIN_JWT_SESSION_TTL", "3h")

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "custom", cfg.App.Name)
	assert.Equal(t, 9191, cfg.Server.Port)
	assert.Equal(t, 45*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 3*time.Hour, cfg.JWT.SessionTTL)
	assert.Equal(t, "memory", cfg.Redis.Driver)
	assert.Equal(t, 3, cfg.Redis.DB)
	assert.Equal(t, int64(42), cfg.Storage.MaxUploadMB)
	assert.Equal(t, []string{"http://b.test", "http://c.test"}, cfg.CORS.Origins)
	assert.Equal(t, []string{"127.0.0.1", "10.0.0.0/8"}, cfg.Server.TrustedProxies)
}

func TestConfigTrustedProxiesCanBeDisabled(t *testing.T) {
	t.Setenv("X_ADMIN_TRUSTED_PROXIES", "none")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Empty(t, cfg.Server.TrustedProxies)
}

func TestConfigInvalidDurationsAreIgnored(t *testing.T) {
	t.Setenv("X_ADMIN_JWT_ACCESS_TTL", "not-a-duration")
	t.Setenv("X_ADMIN_JWT_REFRESH_TTL", "also-bad")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 2*time.Hour, cfg.JWT.AccessTTL)
	assert.Equal(t, 168*time.Hour, cfg.JWT.RefreshTTL)
}

func TestConfigInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("app: ["), 0o600))
	_, err := config.Load(path)
	require.Error(t, err)
}

func TestConfigValidation(t *testing.T) {
	t.Setenv("X_ADMIN_SERVER_PORT", "70000")
	_, err := config.Load("")
	require.Error(t, err)
	t.Setenv("X_ADMIN_SERVER_PORT", "8080")

	t.Setenv("X_ADMIN_JWT_SECRET", "short")
	_, err = config.Load("")
	require.Error(t, err)
	t.Setenv("X_ADMIN_JWT_SECRET", "x-admin-dev-secret-change-me")

	t.Setenv("X_ADMIN_APP_ENV", "production")
	_, err = config.Load("")
	require.Error(t, err)
	t.Setenv("X_ADMIN_APP_ENV", "development")

	t.Setenv("X_ADMIN_DATABASE_DSN", "")
	_, err = config.Load("")
	require.Error(t, err)

	t.Setenv("X_ADMIN_DATABASE_DSN", "x:x@tcp(127.0.0.1:3306)/x")
	t.Setenv("X_ADMIN_TRUSTED_PROXIES", "not-a-cidr")
	_, err = config.Load("")
	require.Error(t, err)
}
