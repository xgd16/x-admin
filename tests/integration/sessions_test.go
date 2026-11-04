package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/security"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func decodeSessions(t *testing.T, body []byte) []service.SessionInfo {
	t.Helper()
	var env struct {
		Code int                   `json:"code"`
		Data []service.SessionInfo `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	require.Equal(t, 0, env.Code)
	return env.Data
}

func TestProfileSessionsListAndRevoke(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	first := app.Login(t, "admin", "Admin@123456")
	second := app.Login(t, "admin", "Admin@123456")

	rr := app.Do(http.MethodGet, "/admin/v1/profile/sessions", nil, first.AccessToken)
	rows := decodeSessions(t, rr.Body.Bytes())
	require.Len(t, rows, 2)
	current := 0
	for _, row := range rows {
		if row.Current {
			current++
		}
	}
	assert.Equal(t, 1, current)

	var otherID string
	for _, row := range rows {
		if !row.Current {
			otherID = row.ID
		}
	}
	require.NotEmpty(t, otherID)

	rr = app.Do(http.MethodDelete, "/admin/v1/profile/sessions/"+otherID, nil, first.AccessToken)
	assert.Equal(t, 0, testsupport.Decode(t, rr).Code)

	// 被撤销设备的 access token 立即失效。
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, second.AccessToken)
	assert.Equal(t, 40102, testsupport.Decode(t, rr).Code)

	// 当前设备仍可用。
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, first.AccessToken)
	assert.Equal(t, 0, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodGet, "/admin/v1/profile/sessions", nil, first.AccessToken)
	assert.Len(t, decodeSessions(t, rr.Body.Bytes()), 1)
}

func TestProfileRevokeOtherAndAllSessions(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	first := app.Login(t, "admin", "Admin@123456")
	second := app.Login(t, "admin", "Admin@123456")
	require.NotEqual(t, first.RefreshToken, second.RefreshToken)

	rr := app.Do(http.MethodPost, "/admin/v1/profile/session-actions/revoke-others", nil, first.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, second.AccessToken)
	assert.Equal(t, 40102, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodPost, "/admin/v1/profile/session-actions/revoke-all", nil, first.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, first.AccessToken)
	assert.Equal(t, 40102, testsupport.Decode(t, rr).Code)
}

func TestUploadsRequireAuthenticatedCookie(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	// /auth/me 会下发文件访问 Cookie。
	rr := app.Do(http.MethodGet, "/admin/v1/auth/me", nil, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	var fileCookie *http.Cookie
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == security.FileAccessCookie {
			fileCookie = cookie
		}
	}
	require.NotNil(t, fileCookie)
	assert.True(t, fileCookie.HttpOnly)

	info, err := app.Storage.Save(context.Background(), "hello.txt", strings.NewReader("hello"))
	require.NoError(t, err)

	// 无凭证访问被拒绝。
	req := httptest.NewRequest(http.MethodGet, info.URL, nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 携带文件访问 Cookie 可以读取。
	req = httptest.NewRequest(http.MethodGet, info.URL, nil)
	req.AddCookie(fileCookie)
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())

	// Bearer token 同样可以访问，方便 API 客户端。
	req = httptest.NewRequest(http.MethodGet, info.URL, nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// 会话被撤销后，即使 Cookie 仍在有效期内也无法继续读取。
	rows := decodeSessions(t, app.Do(http.MethodGet, "/admin/v1/profile/sessions", nil, pair.AccessToken).Body.Bytes())
	require.Len(t, rows, 1)
	rr = app.Do(http.MethodDelete, "/admin/v1/profile/sessions/"+rows[0].ID, nil, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)

	req = httptest.NewRequest(http.MethodGet, info.URL, nil)
	req.AddCookie(fileCookie)
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
