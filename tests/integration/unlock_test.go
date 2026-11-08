package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/cache"
	"x-admin/tests/testsupport"
)

// unlockFailKey 与 service 层保持一致的失败计数键。
func unlockFailKey(userID uint64) string { return "unlock:fail:" + strconv.FormatUint(userID, 10) }

func TestUnlockWithPassword(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken
	ctx := context.Background()

	// 连续 4 次错误密码：每次 40101 且失败计数递增。
	for i := 0; i < 4; i++ {
		rr := app.Do(http.MethodPost, "/admin/v1/auth/unlock", map[string]string{"password": "wrong"}, token)
		assert.Equal(t, 40101, testsupport.Decode(t, rr).Code, "attempt %d", i+1)
	}
	n, err := app.Cache.Get(ctx, unlockFailKey(admin.ID))
	require.NoError(t, err)
	assert.Equal(t, "4", n)

	// 第 5 次失败触发解锁锁定；此后即使密码正确也被拒绝。
	rr := app.Do(http.MethodPost, "/admin/v1/auth/unlock", map[string]string{"password": "wrong"}, token)
	assert.Equal(t, 42301, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodPost, "/admin/v1/auth/unlock", map[string]string{"password": "Admin@123456"}, token)
	assert.Equal(t, 42301, testsupport.Decode(t, rr).Code)
}

func TestUnlockWithCorrectPassword(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken
	ctx := context.Background()

	rr := app.Do(http.MethodPost, "/admin/v1/auth/unlock", map[string]string{"password": "Admin@123456"}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	// 成功后失败计数清零。
	_, err := app.Cache.Get(ctx, unlockFailKey(admin.ID))
	assert.ErrorIs(t, err, cache.ErrNotFound)

	// 空密码直接 40001。
	rr = app.Do(http.MethodPost, "/admin/v1/auth/unlock", map[string]string{}, token)
	assert.Equal(t, 40001, testsupport.Decode(t, rr).Code)
}

func TestUnlockRateLimit(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken

	// 1-4 次密码错误，5-10 次处于解锁锁定，第 11 次请求命中 rl:unlock 限流。
	want := []int{40101, 40101, 40101, 40101, 42301, 42301, 42301, 42301, 42301, 42301, 42901}
	for i, code := range want {
		rr := app.Do(http.MethodPost, "/admin/v1/auth/unlock", map[string]string{"password": "wrong"}, token)
		assert.Equal(t, code, testsupport.Decode(t, rr).Code, "attempt %d", i+1)
	}
}

func TestUnlockPasskey(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken
	ctx := context.Background()

	// 未绑定任何通行密钥时直接 404。
	rr := app.Do(http.MethodPost, "/admin/v1/auth/unlock/passkey/begin", nil, token)
	assert.Equal(t, 40401, testsupport.Decode(t, rr).Code)

	row := &model.Passkey{
		UserID: admin.ID, CredentialID: "credential-admin", Name: "MacBook",
		Credential: `{"id":"Y3JlZGVudGlhbC1hZG1pbg=="}`,
	}
	require.NoError(t, app.Repos.Passkeys.Create(ctx, row))

	rr = app.Do(http.MethodPost, "/admin/v1/auth/unlock/passkey/begin", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	var begin passkeyOptionsPayload
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &begin))
	require.NotEmpty(t, begin.SessionID)
	// 解锁会话落入独立前缀，等待认证器响应。
	exists, err := app.Cache.Exists(ctx, "passkey:unlock:"+begin.SessionID)
	require.NoError(t, err)
	assert.True(t, exists)

	// 非法凭据：验证失败（40101）且一次性会话被消费。
	rr = app.Do(http.MethodPost, "/admin/v1/auth/unlock/passkey/finish", map[string]any{
		"session_id": begin.SessionID,
		"credential": map[string]any{
			"id": "dGVzdA", "rawId": "dGVzdA", "type": "public-key",
			"response": map[string]any{
				"clientDataJSON": "e30", "authenticatorData": "e30",
				"signature": "e30", "userHandle": "e30",
			},
		},
	}, token)
	assert.Equal(t, 40101, testsupport.Decode(t, rr).Code)
	exists, err = app.Cache.Exists(ctx, "passkey:unlock:"+begin.SessionID)
	require.NoError(t, err)
	assert.False(t, exists)

	// 他人会话不能给别的用户解锁：ceremony 绑定 admin，other 的 finish 被拒。
	app.CreateUser(t, "unlock-other", "Admin@123456", model.StatusEnabled)
	otherToken := app.Login(t, "unlock-other", "Admin@123456").AccessToken
	rr = app.Do(http.MethodPost, "/admin/v1/auth/unlock/passkey/begin", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &begin))
	rr = app.Do(http.MethodPost, "/admin/v1/auth/unlock/passkey/finish", map[string]any{
		"session_id": begin.SessionID,
		"credential": map[string]any{
			"id": "dGVzdA", "rawId": "dGVzdA", "type": "public-key",
			"response": map[string]any{
				"clientDataJSON": "e30", "authenticatorData": "e30",
				"signature": "e30", "userHandle": "e30",
			},
		},
	}, otherToken)
	assert.Equal(t, 40101, testsupport.Decode(t, rr).Code)
}
