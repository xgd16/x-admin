package pkg_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/config"
	"x-admin/internal/pkg/security"
)

func TestAPIConfigDefaults(t *testing.T) {
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.True(t, cfg.API.Admin.Enabled)
	assert.Equal(t, "/admin/v1", cfg.API.Admin.Prefix)
	assert.Equal(t, "x-admin", cfg.API.Admin.IssuerFor(cfg.JWT.Issuer, "admin"))
	// 默认只有后台面；client/merchant 由 create api 按需生成。
	assert.Len(t, cfg.API.Surfaces(), 1)
}

func TestAPIConfigEnvOverrides(t *testing.T) {
	t.Setenv("X_ADMIN_API_ADMIN_ENABLED", "false")
	t.Setenv("X_ADMIN_API_ADMIN_PREFIX", "/backoffice/v1")
	t.Setenv("X_ADMIN_API_ADMIN_ISSUER", "x-admin-backoffice")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.False(t, cfg.API.Admin.Enabled)
	assert.Equal(t, "/backoffice/v1", cfg.API.Admin.Prefix)
	assert.Equal(t, "x-admin-backoffice", cfg.API.Admin.IssuerFor(cfg.JWT.Issuer, "admin"))
}

func TestAPIConfigExtraSurface(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
api:
  partner:
    enabled: true
    prefix: /partner/v1
`), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	// 内置面没有被 YAML 中的单个自定义面覆盖。
	assert.True(t, cfg.API.Admin.Enabled)
	assert.Len(t, cfg.API.Surfaces(), 2)

	surface, ok := cfg.API.Surfaces()["partner"]
	require.True(t, ok)
	assert.True(t, surface.Enabled)
	assert.Equal(t, "/partner/v1", surface.Prefix)
	assert.Equal(t, "x-admin:partner", surface.IssuerFor(cfg.JWT.Issuer, "partner"))
}

func TestAPIConfigRejectsDuplicateSurface(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "duplicate prefix",
			yaml: "api:\n  partner:\n    enabled: true\n    prefix: /admin/v1\n",
		},
		{
			name: "duplicate issuer",
			yaml: "api:\n  partner:\n    enabled: true\n    issuer: x-admin\n",
		},
		{
			name: "invalid prefix",
			yaml: "api:\n  partner:\n    enabled: true\n    prefix: partner/v1\n",
		},
		{
			name: "reserved prefix",
			yaml: "api:\n  partner:\n    enabled: true\n    prefix: /uploads\n",
		},
		{
			name: "reserved prefix child",
			yaml: "api:\n  partner:\n    enabled: true\n    prefix: /uploads/v1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.yaml), 0o600))
			_, err := config.Load(path)
			require.Error(t, err)
		})
	}
}

func TestAPISurfaceTokensAreIsolatedByIssuer(t *testing.T) {
	cfg := config.Default()
	adminTokens := security.NewTokenManager(
		cfg.JWT.Secret,
		cfg.API.Admin.IssuerFor(cfg.JWT.Issuer, "admin"),
		time.Hour,
	)
	clientSurface := config.APISurfaceConfig{}
	clientTokens := security.NewTokenManager(
		cfg.JWT.Secret,
		clientSurface.IssuerFor(cfg.JWT.Issuer, "client"),
		time.Hour,
	)

	token, _, err := clientTokens.Issue(7, "client-user", 0, "session-id")
	require.NoError(t, err)

	_, err = clientTokens.Parse(token)
	require.NoError(t, err)

	// 后台 TokenManager 不能接受客户端 token；issuer 隔离是跨面安全边界。
	_, err = adminTokens.Parse(token)
	assert.ErrorIs(t, err, security.ErrInvalidToken)
}
