package service_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/security"
	"x-admin/internal/repository"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func newSurfaceAuth(t *testing.T, app *testsupport.App, scope service.SurfaceScope) (*service.SurfaceAuthService, *security.TokenManager) {
	t.Helper()
	require.NoError(t, app.DB.Table(scope.Table).AutoMigrate(&model.APIUser{}))
	repo := repository.NewAPIUserRepo(app.DB)
	svc := service.NewSurfaceAuthService(repo, app.Cache, app.Hasher, 8, service.AuthOptions{
		MaxLoginFails: 5,
		LockDuration:  15 * time.Minute,
		RefreshTTL:    time.Hour,
		SessionTTL:    time.Hour,
	})
	tokens := security.NewTokenManager(app.Config.JWT.Secret, app.Config.JWT.Issuer+":"+scope.Name, time.Hour)
	return svc, tokens
}

func TestSurfaceAuthRegisterLoginRefreshLogout(t *testing.T) {
	app := testsupport.New(t)
	scope := service.SurfaceScope{Name: "client", Table: "client_users"}
	auth, tokens := newSurfaceAuth(t, app, scope)
	ctx := context.Background()

	registered, err := auth.Register(ctx, scope, service.SurfaceRegisterInput{
		Username: "customer", Password: "Client@123", Nickname: "客户",
	}, tokens, service.LoginMeta{IP: "127.0.0.1", UserAgent: "test"})
	require.NoError(t, err)
	assert.NotEmpty(t, registered.AccessToken)
	assert.NotEmpty(t, registered.RefreshToken)
	assert.Equal(t, "customer", registered.User.Username)

	// 同一张表内用户名唯一。
	_, err = auth.Register(ctx, scope, service.SurfaceRegisterInput{
		Username: "customer", Password: "Client@123",
	}, tokens, service.LoginMeta{})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	loggedIn, err := auth.Login(ctx, scope, service.SurfaceLoginInput{
		Username: "customer", Password: "Client@123",
	}, tokens, service.LoginMeta{IP: "127.0.0.2", UserAgent: "test"})
	require.NoError(t, err)
	assert.NotEmpty(t, loggedIn.RefreshToken)

	// 旧 refresh token 一次性使用，换发后立即失效。
	refreshed, err := auth.Refresh(ctx, scope, loggedIn.RefreshToken, tokens, service.LoginMeta{IP: "127.0.0.2"})
	require.NoError(t, err)
	assert.NotEmpty(t, refreshed.AccessToken)
	assert.NotEmpty(t, refreshed.RefreshToken)
	_, err = auth.Refresh(ctx, scope, loggedIn.RefreshToken, tokens, service.LoginMeta{})
	require.Error(t, err)

	claims, err := tokens.Parse(refreshed.AccessToken)
	require.NoError(t, err)
	principal, err := auth.Authenticate(ctx, scope, claims.UserID)
	require.NoError(t, err)
	assert.Equal(t, "customer", principal.Username)
	assert.False(t, principal.Super)
	assert.Empty(t, principal.Permissions)

	active, err := auth.SessionActive(ctx, scope, claims.UserID, claims.SessionID)
	require.NoError(t, err)
	assert.True(t, active)

	require.NoError(t, auth.Logout(ctx, scope, claims.UserID, refreshed.RefreshToken, claims.SessionID))
	active, err = auth.SessionActive(ctx, scope, claims.UserID, claims.SessionID)
	require.NoError(t, err)
	assert.False(t, active)
}

func TestSurfaceAuthUsesPerSurfaceTTL(t *testing.T) {
	app := testsupport.New(t)
	scope := service.SurfaceScope{Name: "client", Table: "client_users"}
	require.NoError(t, app.DB.Table(scope.Table).AutoMigrate(&model.APIUser{}))

	recorder := &ttlRecorder{Store: app.Cache, ttls: make(map[string]time.Duration)}
	repo := repository.NewAPIUserRepo(app.DB)
	auth := service.NewSurfaceAuthService(repo, recorder, app.Hasher, 8, service.AuthOptions{
		MaxLoginFails: 5,
		LockDuration:  15 * time.Minute,
		RefreshTTL:    time.Hour,
		SessionTTL:    time.Hour,
	})
	auth.SetSurfaceOptions(map[string]service.AuthOptions{
		"client": {
			MaxLoginFails: 5,
			LockDuration:  15 * time.Minute,
			RefreshTTL:    3 * time.Hour,
			SessionTTL:    45 * time.Minute,
		},
	})
	tokens := security.NewTokenManager(app.Config.JWT.Secret, app.Config.JWT.Issuer+":client", time.Hour)
	ctx := context.Background()

	// 不勾选“记住我”时使用面级 session_ttl。
	_, err := auth.Register(ctx, scope, service.SurfaceRegisterInput{
		Username: "customer", Password: "Client@123",
	}, tokens, service.LoginMeta{})
	require.NoError(t, err)
	assert.Equal(t, 45*time.Minute, recorder.lastTTL("auth:client:refresh:"))

	// 勾选“记住我”时使用面级 refresh_ttl。
	recorder.reset()
	_, err = auth.Login(ctx, scope, service.SurfaceLoginInput{
		Username: "customer", Password: "Client@123", RememberMe: true,
	}, tokens, service.LoginMeta{})
	require.NoError(t, err)
	assert.Equal(t, 3*time.Hour, recorder.lastTTL("auth:client:refresh:"))
}

// ttlRecorder 记录写入 refresh token 时的有效期，用于验证面级 JWT 配置生效。
type ttlRecorder struct {
	cache.Store
	mu   sync.Mutex
	ttls map[string]time.Duration
}

func (r *ttlRecorder) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	r.mu.Lock()
	r.ttls[key] = ttl
	r.mu.Unlock()
	return r.Store.Set(ctx, key, value, ttl)
}

func (r *ttlRecorder) lastTTL(prefix string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, ttl := range r.ttls {
		if strings.HasPrefix(key, prefix) {
			return ttl
		}
	}
	return 0
}

func (r *ttlRecorder) reset() {
	r.mu.Lock()
	r.ttls = make(map[string]time.Duration)
	r.mu.Unlock()
}

func TestSurfaceAuthWrongPasswordAndDisabledUser(t *testing.T) {
	app := testsupport.New(t)
	scope := service.SurfaceScope{Name: "merchant", Table: "merchant_users"}
	auth, tokens := newSurfaceAuth(t, app, scope)
	ctx := context.Background()

	_, err := auth.Register(ctx, scope, service.SurfaceRegisterInput{
		Username: "shop", Password: "Shop@1234",
	}, tokens, service.LoginMeta{})
	require.NoError(t, err)

	_, err = auth.Login(ctx, scope, service.SurfaceLoginInput{
		Username: "shop", Password: "wrong-password",
	}, tokens, service.LoginMeta{})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)

	require.NoError(t, app.DB.Table(scope.Table).Model(&model.APIUser{}).
		Where("username = ?", "shop").Update("status", model.StatusDisabled).Error)
	_, err = auth.Login(ctx, scope, service.SurfaceLoginInput{
		Username: "shop", Password: "Shop@1234",
	}, tokens, service.LoginMeta{})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
}

func TestSurfaceAuthIsolatedByTable(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()
	clientScope := service.SurfaceScope{Name: "client", Table: "client_users"}
	merchantScope := service.SurfaceScope{Name: "merchant", Table: "merchant_users"}
	clientAuth, clientTokens := newSurfaceAuth(t, app, clientScope)
	merchantAuth, merchantTokens := newSurfaceAuth(t, app, merchantScope)

	_, err := clientAuth.Register(ctx, clientScope, service.SurfaceRegisterInput{
		Username: "same_name", Password: "Client@123",
	}, clientTokens, service.LoginMeta{})
	require.NoError(t, err)
	_, err = merchantAuth.Register(ctx, merchantScope, service.SurfaceRegisterInput{
		Username: "same_name", Password: "Merchant@123",
	}, merchantTokens, service.LoginMeta{})
	require.NoError(t, err)

	// 同用户名在不同 API 面可以共存。
	_, err = clientAuth.Login(ctx, clientScope, service.SurfaceLoginInput{
		Username: "same_name", Password: "Client@123",
	}, clientTokens, service.LoginMeta{})
	require.NoError(t, err)
	_, err = merchantAuth.Login(ctx, merchantScope, service.SurfaceLoginInput{
		Username: "same_name", Password: "Merchant@123",
	}, merchantTokens, service.LoginMeta{})
	require.NoError(t, err)
}
