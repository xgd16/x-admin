package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-webauthn/webauthn/webauthn"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

// failSetStore 让缓存写入失败，用于覆盖会话保存错误分支。
type failSetStore struct{ cache.Store }

func (f failSetStore) Set(context.Context, string, string, time.Duration) error {
	return errors.New("set failed")
}

func disabledPasskeyService(app *testsupport.App) *service.PasskeyService {
	return service.NewPasskeyService(app.Repos.Users, app.Repos.Passkeys, app.Hasher, app.Cache, nil,
		time.Minute, app.Services.Notifications)
}

func TestPasskeyServiceErrorBranches(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)

	disabled := disabledPasskeyService(app)
	_, err := disabled.BeginRegistration(ctx, admin.ID, "Admin@123456")
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	_, err = disabled.FinishRegistration(ctx, admin.ID, "x", "n", nil)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	_, err = disabled.BeginLogin(ctx)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	_, err = disabled.FinishLogin(ctx, "x", nil)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)

	pk := app.Services.Passkeys
	_, err = pk.BeginRegistration(ctx, 404, "x")
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	off := app.CreateUser(t, "off-passkey", "Passw0rd123", model.StatusDisabled)
	_, err = pk.BeginRegistration(ctx, off.ID, "Passw0rd123")
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)

	_, err = pk.BeginRegistration(ctx, admin.ID, "wrong-password")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	// 损坏的凭据会被跳过，不影响继续绑定新设备。
	require.NoError(t, app.Repos.Passkeys.Create(ctx, &model.Passkey{
		UserID: admin.ID, CredentialID: "corrupt", Name: "坏设备", Credential: "not-json",
	}))
	opts, err := pk.BeginRegistration(ctx, admin.ID, "Admin@123456")
	require.NoError(t, err)
	require.NotEmpty(t, opts.SessionID)
	assert.Equal(t, "admin", opts.Options.Response.User.Name)
	assert.Equal(t, admin.Nickname, opts.Options.Response.User.DisplayName)

	// 会话属于 admin，不能由其他用户完成。
	other := app.CreateUser(t, "other-passkey", "Passw0rd123", model.StatusEnabled)
	_, err = pk.FinishRegistration(ctx, other.ID, opts.SessionID, "设备", nil)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	_, err = pk.FinishRegistration(ctx, admin.ID, "", "设备", nil)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	// 非法认证器响应返回可读的绑定失败提示。
	fresh, err := pk.BeginRegistration(ctx, admin.ID, "Admin@123456")
	require.NoError(t, err)
	_, err = pk.FinishRegistration(ctx, admin.ID, fresh.SessionID, "设备", []byte(`{"id":"bad"}`))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "通行密钥绑定失败")

	require.NoError(t, app.Cache.Set(ctx, "passkey:register:broken", "not-json", time.Minute))
	_, err = pk.FinishRegistration(ctx, admin.ID, "broken", "设备", nil)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	// 缓存不可用时，应把错误包装成内部错误。
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "test", RPID: "localhost", RPOrigins: []string{"http://localhost"},
	})
	require.NoError(t, err)
	failing := service.NewPasskeyService(app.Repos.Users, app.Repos.Passkeys, app.Hasher,
		failSetStore{Store: app.Cache}, wa, time.Minute, app.Services.Notifications)
	_, err = failing.BeginRegistration(ctx, admin.ID, "Admin@123456")
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
}

func TestPasskeyManagementBranches(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	pk := app.Services.Passkeys

	rows, err := pk.List(ctx, admin.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)

	err = pk.Rename(ctx, admin.ID, 404, "")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	err = pk.Rename(ctx, admin.ID, 404, "设备")
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	err = pk.Delete(ctx, admin.ID, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	row := &model.Passkey{UserID: admin.ID, CredentialID: "rename-me", Name: "旧名", Credential: "{}"}
	require.NoError(t, app.Repos.Passkeys.Create(ctx, row))
	longName := strings.Repeat("长", 80)
	require.NoError(t, pk.Rename(ctx, admin.ID, row.ID, longName))
	got, err := app.Repos.Passkeys.GetForUser(ctx, admin.ID, row.ID)
	require.NoError(t, err)
	assert.Equal(t, 64, len([]rune(got.Name)))

	require.NoError(t, pk.Delete(ctx, admin.ID, row.ID))
	_, err = app.Repos.Passkeys.GetForUser(ctx, admin.ID, row.ID)
	assert.Error(t, err)

	// 至少存在一个凭据时可以生成登录参数。
	require.NoError(t, app.Repos.Passkeys.Create(ctx, &model.Passkey{
		UserID: admin.ID, CredentialID: "login-device", Name: "设备", Credential: "{}",
	}))
	login, err := pk.BeginLogin(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, login.SessionID)

	_, err = pk.FinishLogin(ctx, "missing", nil)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = pk.FinishLogin(ctx, login.SessionID, []byte(`{"id":"bad"}`))
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "通行密钥验证失败")
}
