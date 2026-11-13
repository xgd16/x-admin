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

func TestAPISurfaceJWTFor(t *testing.T) {
	base := config.JWTConfig{
		Secret: "global-secret-0123456789", Issuer: "x-admin",
		AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, SessionTTL: 30 * time.Minute,
	}
	surface := config.APISurfaceConfig{JWT: config.SurfaceJWTConfig{
		Secret: "surface-secret-0123456789", Issuer: "client-issuer",
		AccessTTL: 10 * time.Minute, RefreshTTL: 48 * time.Hour,
	}}
	effective := surface.JWTFor(base, "client")
	assert.Equal(t, "surface-secret-0123456789", effective.Secret)
	assert.Equal(t, "client-issuer", effective.Issuer)
	assert.Equal(t, 10*time.Minute, effective.AccessTTL)
	assert.Equal(t, 48*time.Hour, effective.RefreshTTL)
	// 未覆盖的 session_ttl 沿用全局值。
	assert.Equal(t, 30*time.Minute, effective.SessionTTL)

	// 面级 jwt 全部为空时沿用全局配置，issuer 仍按面隔离。
	effective = config.APISurfaceConfig{}.JWTFor(base, "merchant")
	assert.Equal(t, base.Secret, effective.Secret)
	assert.Equal(t, "x-admin:merchant", effective.Issuer)
	assert.Equal(t, base.AccessTTL, effective.AccessTTL)
}

func TestAPIConfigSurfaceJWT(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
api:
  client:
    enabled: true
    prefix: /client/v1
    table: client_users
    jwt:
      secret: client-secret-0123456789
      issuer: client-issuer
      access_ttl: 15m
      refresh_ttl: 720h
      session_ttl: 24h
`), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	surface := cfg.API.Surfaces()["client"]
	effective := surface.JWTFor(cfg.JWT, "client")
	assert.Equal(t, "client-secret-0123456789", effective.Secret)
	assert.Equal(t, "client-issuer", effective.Issuer)
	assert.Equal(t, 15*time.Minute, effective.AccessTTL)
	assert.Equal(t, 720*time.Hour, effective.RefreshTTL)
	assert.Equal(t, 24*time.Hour, effective.SessionTTL)
}

func TestAPIConfigRejectsInvalidSurfaceJWT(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "short secret",
			yaml: "api:\n  client:\n    enabled: true\n    prefix: /client/v1\n    jwt:\n      secret: short\n",
		},
		{
			name: "negative access ttl",
			yaml: "api:\n  client:\n    enabled: true\n    prefix: /client/v1\n    jwt:\n      access_ttl: -1h\n",
		},
		{
			name: "negative refresh ttl",
			yaml: "api:\n  client:\n    enabled: true\n    prefix: /client/v1\n    jwt:\n      refresh_ttl: -1h\n",
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

func TestAPISurfaceIssuedTokenUsesSurfaceTTL(t *testing.T) {
	cfg := config.Default()
	cfg.API.Extra = map[string]config.APISurfaceConfig{
		"client": {
			Enabled: true,
			Prefix:  "/client/v1",
			JWT: config.SurfaceJWTConfig{
				Secret:    "client-secret-0123456789",
				AccessTTL: 15 * time.Minute,
			},
		},
	}
	surface := cfg.API.Surfaces()["client"]
	jwt := surface.JWTFor(cfg.JWT, "client")
	tokens := security.NewTokenManager(jwt.Secret, jwt.Issuer, jwt.AccessTTL)

	token, _, err := tokens.Issue(7, "client-user", 0, "session-id")
	require.NoError(t, err)
	claims, err := tokens.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, claims.ExpiresAt.Sub(claims.IssuedAt.Time))

	// 独立 secret + issuer 的另一道隔离：后台 TokenManager 无法解析客户端 token。
	admin := security.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.AccessTTL)
	_, err = admin.Parse(token)
	assert.ErrorIs(t, err, security.ErrInvalidToken)
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
