package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
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

	rotated, err := auth.Refresh(context.Background(), pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated.RefreshToken)
	assert.NotEqual(t, pair.RefreshToken, rotated.RefreshToken)

	_, err = auth.Refresh(context.Background(), pair.RefreshToken)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	require.NoError(t, auth.Logout(context.Background(), rotated.RefreshToken))
	_, err = auth.Refresh(context.Background(), rotated.RefreshToken)
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

	_, err := auth.Refresh(ctx, "")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
	_, err = auth.Refresh(ctx, "missing")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	require.NoError(t, app.Cache.Set(ctx, "refresh:bad", "not-a-number", time.Hour))
	_, err = auth.Refresh(ctx, "bad")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	require.NoError(t, app.Cache.Set(ctx, "refresh:ghost", "999", time.Hour))
	_, err = auth.Refresh(ctx, "ghost")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	disabled := app.CreateUser(t, "frozen", "Admin@123456", model.StatusDisabled)
	require.NoError(t, app.Cache.Set(ctx, "refresh:frozen", fmt.Sprint(disabled.ID), time.Hour))
	_, err = auth.Refresh(ctx, "frozen")
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
	require.NoError(t, auth.Logout(context.Background(), ""))
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
