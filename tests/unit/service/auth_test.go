package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/totp"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

type stubCaptcha struct{ ok bool }

func (s stubCaptcha) Verify(context.Context, string, string) bool { return s.ok }

func newAuth(app *testsupport.App, captchaOK bool) *service.AuthService {
	return service.NewAuthService(app.Repos.Users, app.Repos.Menus, app.Repos.Logs, app.Cache, app.Tokens,
		app.Hasher, stubCaptcha{ok: captchaOK}, app.Services.Notifications, service.AuthOptions{
			MaxLoginFails: 5, LockDuration: 15 * time.Minute, RefreshTTL: time.Hour,
		})
}

func login(app *testsupport.App, auth *service.AuthService, username, password string) (*service.LoginResult, error) {
	return auth.Login(context.Background(), service.LoginInput{
		Username: username, Password: password, CaptchaID: "id", CaptchaCode: "code",
	}, service.LoginMeta{IP: "127.0.0.1", UserAgent: "test"})
}

func TestAuthLoginSuccessAndRefreshRotation(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	auth := newAuth(app, true)

	pair, err := login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, "Bearer", pair.TokenType)
	assert.Greater(t, pair.ExpiresIn, int64(0))

	claims, err := app.Tokens.Parse(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), claims.UserID)

	rotated, err := auth.Refresh(context.Background(), pair.RefreshToken, service.LoginMeta{})
	require.NoError(t, err)
	assert.NotEmpty(t, rotated.RefreshToken)
	assert.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)

	_, err = auth.Refresh(context.Background(), pair.RefreshToken, service.LoginMeta{})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	require.NoError(t, auth.Logout(context.Background(), 1, rotated.RefreshToken, ""))
	_, err = auth.Refresh(context.Background(), rotated.RefreshToken, service.LoginMeta{})
	require.Error(t, err)
}

func TestAuthLoginFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)
	auth := newAuth(app, false)

	_, err := login(app, auth, "admin", "Admin@123456")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	auth = newAuth(app, true)
	_, err = auth.Login(ctx, service.LoginInput{Username: " ", Password: "x"}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	_, err = login(app, auth, "ghost", "whatever")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)

	for i := 0; i < 4; i++ {
		_, err = login(app, auth, "admin", "wrong-password")
		require.Error(t, err)
		assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
	}
	_, err = login(app, auth, "admin", "wrong-password")
	assert.Equal(t, apperr.CodeLocked, apperr.From(err).Code)

	_, err = login(app, auth, "admin", "Admin@123456")
	assert.Equal(t, apperr.CodeLocked, apperr.From(err).Code)

	require.NoError(t, app.Cache.Del(ctx, "login:fail:admin"))
	_, err = login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)
}

func TestAuthLoginDisabledAccount(t *testing.T) {
	app := testsupport.New(t)
	app.CreateUser(t, "disabled", "Admin@123456", model.StatusDisabled)
	auth := newAuth(app, true)
	_, err := login(app, auth, "disabled", "Admin@123456")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
}

func TestAuthRefreshFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)
	auth := newAuth(app, true)

	_, err := auth.Refresh(ctx, "", service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
	_, err = auth.Refresh(ctx, "missing", service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	require.NoError(t, app.Cache.Set(ctx, "refresh:bad", "not-a-number", time.Hour))
	_, err = auth.Refresh(ctx, "bad", service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	require.NoError(t, app.Cache.Set(ctx, "refresh:ghost", "999", time.Hour))
	_, err = auth.Refresh(ctx, "ghost", service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	disabled := app.CreateUser(t, "frozen", "Admin@123456", model.StatusDisabled)
	require.NoError(t, app.Cache.Set(ctx, "refresh:frozen", fmt.Sprintf("%d:0:sid-frozen", disabled.ID), time.Hour))
	require.NoError(t, app.Cache.Set(ctx, "session:2:sid-frozen", `{"version":0}`, time.Hour))
	_, err = auth.Refresh(ctx, "frozen", service.LoginMeta{})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	assert.Equal(t, uint64(2), disabled.ID)
}

func TestAuthPrincipalAndMe(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	menu := &model.Menu{Name: "总览", Type: model.MenuMenu, Path: "/dashboard", Permission: "dashboard:view", Status: model.StatusEnabled, Visible: true}
	require.NoError(t, app.Repos.Menus.Create(ctx, menu))
	role := app.EnsureRole(t, "operator", "运营")
	require.NoError(t, app.Repos.Roles.SetMenus(ctx, role.ID, []uint64{menu.ID}))
	operator := app.CreateUser(t, "operator", "Admin@123456", model.StatusEnabled, "operator")
	_ = admin

	auth := newAuth(app, true)
	principal, err := auth.Authenticate(ctx, operator.ID)
	require.NoError(t, err)
	assert.False(t, principal.Super)
	assert.True(t, principal.Has("dashboard:view"))
	assert.False(t, principal.Has("system:user:delete"))
	assert.Equal(t, "operator", principal.Username)

	super, err := auth.Authenticate(ctx, admin.ID)
	require.NoError(t, err)
	assert.True(t, super.Super)
	assert.True(t, super.Has("anything"))

	me, err := auth.Me(ctx, operator.ID)
	require.NoError(t, err)
	assert.Equal(t, "operator", me.User.Username)
	assert.Contains(t, me.Permissions, "dashboard:view")
	require.Len(t, me.Menus, 1)
	assert.Equal(t, "总览", me.Menus[0].Name)
}

func TestAuthPrincipalErrors(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	auth := newAuth(app, true)

	_, err := auth.Authenticate(ctx, 404)
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)

	disabled := app.CreateUser(t, "off", "Admin@123456", model.StatusDisabled)
	_, err = auth.Authenticate(ctx, disabled.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)

	_, err = auth.Me(ctx, 404)
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
}

func TestAuthLogoutEmpty(t *testing.T) {
	app := testsupport.New(t)
	auth := newAuth(app, true)
	require.NoError(t, auth.Logout(context.Background(), 0, "", ""))
}

func TestAuthRecordedLogs(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	auth := newAuth(app, true)
	_, err := login(app, auth, "admin", "bad")
	require.Error(t, err)
	_, err = login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)

	logs, total, err := app.Repos.Logs.ListLogins(context.Background(), model.LoginLogFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, logs, 2)
}

func TestAuthErrorsAreBusinessErrors(t *testing.T) {
	app := testsupport.New(t)
	auth := newAuth(app, true)
	_, err := login(app, auth, "ghost", "x")
	require.Error(t, err)
	var target *apperr.Error
	assert.True(t, errors.As(err, &target))
}

func TestAuthRememberMeFlagPersistsAcrossRefresh(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)
	auth := newAuth(app, true)

	loginWithRemember := func(remember bool) *service.LoginResult {
		t.Helper()
		result, err := auth.Login(ctx, service.LoginInput{
			Username: "admin", Password: "Admin@123456",
			CaptchaID: "id", CaptchaCode: "code", RememberMe: remember,
		}, service.LoginMeta{IP: "127.0.0.1", UserAgent: "test"})
		require.NoError(t, err)
		require.NotNil(t, result.TokenPair)
		return result
	}

	remembered := loginWithRemember(true)
	raw, err := app.Cache.Get(ctx, "refresh:"+remembered.RefreshToken)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(raw, ":1"), raw)
	keys, err := app.Cache.Keys(ctx, "session:1:")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	sessionRaw, err := app.Cache.Get(ctx, keys[0])
	require.NoError(t, err)
	assert.Contains(t, sessionRaw, `"remember":true`)

	rotated, err := auth.Refresh(ctx, remembered.RefreshToken, service.LoginMeta{})
	require.NoError(t, err)
	raw, err = app.Cache.Get(ctx, "refresh:"+rotated.RefreshToken)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(raw, ":1"), raw)

	sessionOnly := loginWithRemember(false)
	raw, err = app.Cache.Get(ctx, "refresh:"+sessionOnly.RefreshToken)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(raw, ":0"), raw)
}

func TestAuthLoginPasskeyDirect(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := newAuth(app, true)
	result, err := auth.LoginPasskey(ctx, admin, service.LoginMeta{IP: "127.0.0.1", UserAgent: "test"}, true)
	require.NoError(t, err)
	require.NotNil(t, result.TokenPair)
	assert.NotEmpty(t, result.AccessToken)

	disabled := app.CreateUser(t, "passkey-off", "Passw0rd123", model.StatusDisabled)
	_, err = auth.LoginPasskey(ctx, disabled, service.LoginMeta{}, false)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
}

func TestAuthLoginMFAErrorsAndSuccess(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := newAuth(app, true)

	_, err := auth.LoginMFA(ctx, service.LoginMFAInput{}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "missing", Code: "123456"}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	require.NoError(t, app.Cache.Set(ctx, "mfa:challenge:bad", "not-a-number", time.Minute))
	_, err = auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "bad", Code: "123456"}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	setup, err := app.Services.Profile.SetupTOTP(ctx, admin.ID)
	require.NoError(t, err)
	code, err := totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	_, err = app.Services.Profile.EnableTOTP(ctx, admin.ID, "Admin@123456", code)
	require.NoError(t, err)

	// 错误验证码累计到上限后挑战作废。
	require.NoError(t, app.Cache.Set(ctx, "mfa:challenge:challenge-1", fmt.Sprint(admin.ID), time.Minute))
	for i := 0; i < 5; i++ {
		_, err = auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "challenge-1", Code: "000000"}, service.LoginMeta{})
		assert.Error(t, err)
	}
	_, err = auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "challenge-1", Code: "000000"}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	// 正确动态码完成登录，并保留 remember_me。
	require.NoError(t, app.Cache.Set(ctx, "mfa:challenge:challenge-2", fmt.Sprint(admin.ID), time.Minute))
	code, err = totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	result, err := auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "challenge-2", Code: code, RememberMe: true}, service.LoginMeta{})
	require.NoError(t, err)
	require.NotNil(t, result.TokenPair)
}

func TestAuthLoginMFAUnavailableAccount(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	auth := newAuth(app, true)
	disabled := app.CreateUser(t, "mfa-disabled", "Passw0rd123", model.StatusDisabled)
	require.NoError(t, app.Cache.Set(ctx, "mfa:challenge:disabled", fmt.Sprint(disabled.ID), time.Minute))
	_, err := auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "disabled", Code: "123456"}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)

	// 用户不存在时挑战被清理。
	require.NoError(t, app.Cache.Set(ctx, "mfa:challenge:ghost", "999", time.Minute))
	_, err = auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "ghost", Code: "123456"}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
}

func TestAuthRefreshVersionAndSessionChecks(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := newAuth(app, true)
	pair, err := login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)

	// 会话被撤销后 refresh 立即失效。
	keys, err := app.Cache.Keys(ctx, "session:1:")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.NoError(t, app.Cache.Del(ctx, keys[0]))
	_, err = auth.Refresh(ctx, pair.RefreshToken, service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	// session_version 变化后旧 refresh 失效。
	second, err := login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)
	require.NoError(t, app.Repos.Users.BumpSessionVersion(ctx, admin.ID))
	_, err = auth.Refresh(ctx, second.RefreshToken, service.LoginMeta{})
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
}

func TestAuthRefreshSessionBoundaries(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := newAuth(app, true)
	pair, err := login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)

	// 空参数、错误用户、错误 token 都是静默 no-op。
	auth.RefreshSession(ctx, admin.ID, "")
	auth.RefreshSession(ctx, 0, pair.RefreshToken)
	auth.RefreshSession(ctx, 999, pair.RefreshToken)
	auth.RefreshSession(ctx, admin.ID, "missing")

	require.NoError(t, app.Repos.Users.BumpSessionVersion(ctx, admin.ID))
	auth.RefreshSession(ctx, admin.ID, pair.RefreshToken)
	raw, err := app.Cache.Get(ctx, "refresh:"+pair.RefreshToken)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(raw, ":0"), raw)
	assert.Contains(t, raw, ":"+fmt.Sprint(admin.SessionVersion+1)+":")

	// 会话不存在时也不再重建。
	keys, err := app.Cache.Keys(ctx, "session:1:")
	require.NoError(t, err)
	require.NotEmpty(t, keys)
	require.NoError(t, app.Cache.Del(ctx, keys...))
	auth.RefreshSession(ctx, admin.ID, pair.RefreshToken)
}

func TestAuthIssueFailure(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	auth := service.NewAuthService(app.Repos.Users, app.Repos.Menus, app.Repos.Logs,
		failSetStore{Store: app.Cache}, app.Tokens, app.Hasher, stubCaptcha{ok: true}, app.Services.Notifications,
		service.AuthOptions{MaxLoginFails: 5, LockDuration: time.Minute, RefreshTTL: time.Hour, SessionTTL: time.Hour})
	_, err := login(app, auth, "admin", "Admin@123456")
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
}

func TestAuthLoginValidationAndLockout(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)

	noCaptcha := newAuth(app, false)
	_, err := login(app, noCaptcha, "admin", "Admin@123456")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	auth := newAuth(app, true)
	_, err = auth.Login(ctx, service.LoginInput{}, service.LoginMeta{})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = login(app, auth, "missing-user", "Admin@123456")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)

	// 预先达到失败上限时直接锁定，不再校验密码。
	require.NoError(t, app.Cache.Set(ctx, "login:fail:admin", "5", time.Minute))
	_, err = login(app, auth, "admin", "Admin@123456")
	assert.Equal(t, apperr.CodeLocked, apperr.From(err).Code)
	require.NoError(t, app.Cache.Del(ctx, "login:fail:admin"))

	// 连续错误密码触发锁定并写入登录日志。
	for i := 0; i < 5; i++ {
		_, err = login(app, auth, "admin", "wrong-password")
		require.Error(t, err)
	}
	assert.Equal(t, apperr.CodeLocked, apperr.From(err).Code)

	// 双重验证开启但密钥缺失时拒绝登录。
	broken := app.CreateUser(t, "totp_broken", "Passw0rd123", model.StatusEnabled)
	require.NoError(t, app.Repos.Users.UpdateFields(ctx, broken.ID, map[string]any{
		"totp_enabled": true, "totp_secret": "",
	}))
	_, err = login(app, auth, "totp_broken", "Passw0rd123")
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
}
