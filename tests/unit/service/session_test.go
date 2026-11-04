package service_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestSessionLifecycleAndChecks(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := newAuth(app, true)
	first, err := login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)
	second, err := login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)
	require.NotEqual(t, first.RefreshToken, second.RefreshToken)

	keys, err := app.Cache.Keys(ctx, "session:1:")
	require.NoError(t, err)
	require.Len(t, keys, 2)
	firstSID := keys[0][len("session:1:"):]
	secondSID := keys[1][len("session:1:"):]

	active, err := auth.SessionActive(ctx, admin.ID, firstSID)
	require.NoError(t, err)
	assert.True(t, active)
	active, err = auth.SessionActive(ctx, admin.ID, "")
	require.NoError(t, err)
	assert.False(t, active)
	active, err = auth.SessionActive(ctx, admin.ID, "missing")
	require.NoError(t, err)
	assert.False(t, active)

	require.NoError(t, auth.CheckFileAccess(ctx, admin.ID, admin.SessionVersion, firstSID))
	err = auth.CheckFileAccess(ctx, admin.ID, admin.SessionVersion+1, firstSID)
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
	err = auth.CheckFileAccess(ctx, admin.ID, admin.SessionVersion, "")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
	err = auth.CheckFileAccess(ctx, admin.ID, admin.SessionVersion, "missing")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
	require.NoError(t, app.Cache.Set(ctx, "session:404:sid-404", "{}", time.Hour))
	err = auth.CheckFileAccess(ctx, 404, 0, "sid-404")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)

	rows, err := auth.Sessions(ctx, admin.ID, firstSID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.True(t, rows[0].Current)
	assert.False(t, rows[0].Remember)

	err = auth.RevokeSession(ctx, admin.ID, "")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	require.NoError(t, auth.RevokeSession(ctx, admin.ID, secondSID))
	rows, err = auth.Sessions(ctx, admin.ID, firstSID)
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	revoked, err := auth.RevokeOtherSessions(ctx, admin.ID, firstSID)
	require.NoError(t, err)
	assert.Zero(t, revoked)
	revoked, err = auth.RevokeAllSessions(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, revoked)
	rows, err = auth.Sessions(ctx, admin.ID, firstSID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestSessionsSkipStaleAndCorruptRecords(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := newAuth(app, true)
	_, err := login(app, auth, "admin", "Admin@123456")
	require.NoError(t, err)

	// 损坏记录会被清理。
	require.NoError(t, app.Cache.Set(ctx, "session:1:corrupt", "not-json", time.Hour))
	// 旧版本会话会被清理。
	stale, err := json.Marshal(map[string]any{"version": 99, "created_at": time.Now(), "last_seen_at": time.Now()})
	require.NoError(t, err)
	require.NoError(t, app.Cache.Set(ctx, "session:1:stale", string(stale), time.Hour))

	rows, err := auth.Sessions(ctx, admin.ID, "")
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	for _, gone := range []string{"session:1:corrupt", "session:1:stale"} {
		exists, err := app.Cache.Exists(ctx, gone)
		require.NoError(t, err)
		assert.False(t, exists)
	}

	_, err = auth.Sessions(ctx, 404, "")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
}

func TestSessionDeviceMetadataAndRevokeAllEmpty(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)
	auth := newAuth(app, true)
	// 没有会话时批量撤销返回 0。
	revoked, err := auth.RevokeOtherSessions(ctx, 1, "")
	require.NoError(t, err)
	assert.Zero(t, revoked)
	revoked, err = auth.RevokeAllSessions(ctx, 1)
	require.NoError(t, err)
	assert.Zero(t, revoked)

	// 登录元数据写入设备名。
	result, err := auth.Login(ctx, service.LoginInput{
		Username: "admin", Password: "Admin@123456", CaptchaID: "id", CaptchaCode: "code", RememberMe: true,
	}, service.LoginMeta{IP: "10.0.0.1", UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) Chrome/120.0 Safari/537.36"})
	require.NoError(t, err)
	require.NotNil(t, result.TokenPair)
	keys, err := app.Cache.Keys(ctx, "session:1:")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	rows, err := auth.Sessions(ctx, 1, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "10.0.0.1", rows[0].IP)
	assert.Equal(t, "macOS · Chrome", rows[0].Device)
	assert.True(t, rows[0].Remember)
}

func TestCheckFileAccessDisabledUser(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	user := app.CreateUser(t, "frozen-file", "Passw0rd123", model.StatusDisabled)
	auth := newAuth(app, true)
	require.NoError(t, app.Cache.Set(ctx, "session:"+strconv.FormatUint(user.ID, 10)+":sid", "{}", time.Hour))
	err := auth.CheckFileAccess(ctx, user.ID, 0, "sid")
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
}

func TestSessionsDescribeDeviceLabels(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := newAuth(app, true)

	cases := []struct {
		ua     string
		device string
	}{
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) AppleWebKit/605.1.15 Version/17.0 Mobile Safari/604.1", "iPhone · Safari"},
		{"Mozilla/5.0 (iPad; CPU OS 17_0) AppleWebKit/605.1.15 Version/17.0 Safari/604.1", "iPad · Safari"},
		{"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", "Android · Chrome"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 Chrome/120.0 Safari/537.36", "macOS · Chrome"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Edg/120.0", "Windows · Edge"},
		{"Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/121.0", "Linux · Firefox"},
		{"curl/8.4.0", "未知设备"},
		{"Mozilla/5.0 (Windows NT 10.0)", "Windows"},
		{"Mozilla/5.0 Chrome/120.0", "Chrome"},
	}
	for _, tc := range cases {
		_, err := auth.Login(ctx, service.LoginInput{
			Username: "admin", Password: "Admin@123456", CaptchaID: "id", CaptchaCode: "code",
		}, service.LoginMeta{IP: "10.0.0.1", UserAgent: tc.ua})
		require.NoError(t, err, tc.ua)
	}

	rows, err := auth.Sessions(ctx, admin.ID, "")
	require.NoError(t, err)
	require.Len(t, rows, len(cases))
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Device)
	}
	want := make([]string, 0, len(cases))
	for _, tc := range cases {
		want = append(want, tc.device)
	}
	assert.ElementsMatch(t, want, got)
}
