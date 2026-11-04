package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/totp"
	"x-admin/tests/testsupport"
)

func TestProfileTOTPLifecycleErrors(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	profile := app.Services.Profile
	user := app.CreateUser(t, "totp-life", "Passw0rd123", model.StatusEnabled)

	_, err := profile.TOTPStatus(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = profile.SetupTOTP(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	setup, err := profile.SetupTOTP(ctx, user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, setup.Secret)

	_, err = profile.EnableTOTP(ctx, user.ID, "wrong", "000000")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = profile.EnableTOTP(ctx, user.ID, "Passw0rd123", "000000")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	code, err := totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	codes, err := profile.EnableTOTP(ctx, user.ID, "Passw0rd123", code)
	require.NoError(t, err)
	assert.Len(t, codes, totp.RecoveryCodeCount)

	_, err = profile.SetupTOTP(ctx, user.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	_, err = profile.EnableTOTP(ctx, user.ID, "Passw0rd123", code)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	status, err := profile.TOTPStatus(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, status.Enabled)
	assert.Equal(t, totp.RecoveryCodeCount, status.RecoveryCodesLeft)

	err = profile.DisableTOTP(ctx, user.ID, "wrong", "000000")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	err = profile.DisableTOTP(ctx, user.ID, "Passw0rd123", "000000")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	code, err = totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, profile.DisableTOTP(ctx, user.ID, "Passw0rd123", code))
	status, err = profile.TOTPStatus(ctx, user.ID)
	require.NoError(t, err)
	assert.False(t, status.Enabled)
	assert.Zero(t, status.RecoveryCodesLeft)

	err = profile.DisableTOTP(ctx, user.ID, "Passw0rd123", code)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

func TestProfileTOTPRecoveryCodeAndCorruption(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	profile := app.Services.Profile
	user := app.CreateUser(t, "totp-recovery", "Passw0rd123", model.StatusEnabled)

	setup, err := profile.SetupTOTP(ctx, user.ID)
	require.NoError(t, err)
	code, err := totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	codes, err := profile.EnableTOTP(ctx, user.ID, "Passw0rd123", code)
	require.NoError(t, err)

	// 关闭时用恢复码而不是动态码。
	require.NoError(t, profile.DisableTOTP(ctx, user.ID, "Passw0rd123", codes[0]))
	status, err := profile.TOTPStatus(ctx, user.ID)
	require.NoError(t, err)
	assert.False(t, status.Enabled)

	// 恢复码字段损坏时按空处理，不能当作有效验证。
	require.NoError(t, app.Repos.Users.UpdateFields(ctx, user.ID, map[string]any{
		"totp_enabled": true, "totp_secret": setup.Secret, "totp_recovery_codes": "not-json",
	}))
	err = profile.DisableTOTP(ctx, user.ID, "Passw0rd123", "BADCODE0")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	status, err = profile.TOTPStatus(ctx, user.ID)
	require.NoError(t, err)
	assert.Zero(t, status.RecoveryCodesLeft)
}
