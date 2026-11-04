package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/security"
	"x-admin/tests/testsupport"
)

func TestFileShareCanBeRevoked(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	info, err := app.Storage.Save(context.Background(), "share.txt", strings.NewReader("shared"))
	require.NoError(t, err)
	file := &model.File{Name: "share.txt", Key: info.Key, URL: info.URL, MIME: info.MIME, Size: info.Size}
	require.NoError(t, app.Repos.Files.Create(context.Background(), file))
	fileID := strconv.FormatUint(file.ID, 10)

	rr := app.Do(http.MethodPost, "/admin/v1/files/"+fileID+"/shares", map[string]any{"expires": 3600}, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	var created struct {
		ID        uint64    `json:"id"`
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &created))
	assert.Contains(t, created.URL, "/uploads/")
	assert.Contains(t, created.URL, "share=")
	assert.True(t, created.ExpiresAt.After(time.Now()))

	// 分享链接无需登录即可访问。
	req := httptest.NewRequest(http.MethodGet, created.URL, nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "shared", rec.Body.String())
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")

	// 分享记录可查询。
	rr = app.Do(http.MethodGet, "/admin/v1/files/"+fileID+"/shares", nil, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	var rows []model.FileShare
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &rows))
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].RevokedAt)

	// 撤销后链接立即失效。
	rr = app.Do(http.MethodDelete, "/admin/v1/files/"+fileID+"/shares/"+strconv.FormatUint(created.ID, 10), nil, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	req = httptest.NewRequest(http.MethodGet, created.URL, nil)
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 记录保留 revoked_at，便于审计。
	rr = app.Do(http.MethodGet, "/admin/v1/files/"+fileID+"/shares", nil, pair.AccessToken)
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &rows))
	require.Len(t, rows, 1)
	assert.NotNil(t, rows[0].RevokedAt)
}

func TestFileShareRejectsBadAndExpiredTokens(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	info, err := app.Storage.Save(context.Background(), "share2.txt", strings.NewReader("shared"))
	require.NoError(t, err)
	file := &model.File{Name: "share2.txt", Key: info.Key, URL: info.URL, MIME: info.MIME, Size: info.Size}
	require.NoError(t, app.Repos.Files.Create(context.Background(), file))
	path := info.URL

	// 伪造令牌。
	req := httptest.NewRequest(http.MethodGet, path+"?share=not-a-real-token", nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 已过期令牌。
	expiredToken := "expired-token"
	require.NoError(t, app.Repos.Files.CreateShare(context.Background(), &model.FileShare{
		FileID: file.ID, TokenHash: security.HashToken(expiredToken),
		ExpiresAt: time.Now().Add(-time.Minute),
	}))
	req = httptest.NewRequest(http.MethodGet, path+"?share="+expiredToken, nil)
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 已登录用户打开失效链接也不能回退到 Cookie。
	rr := app.Do(http.MethodGet, "/admin/v1/auth/me", nil, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	var fileCookie *http.Cookie
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == security.FileAccessCookie {
			fileCookie = cookie
		}
	}
	require.NotNil(t, fileCookie)
	req = httptest.NewRequest(http.MethodGet, path+"?share="+expiredToken, nil)
	req.AddCookie(fileCookie)
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUploadsRejectInvalidCredentialsAndMismatchedShare(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	first, err := app.Storage.Save(context.Background(), "first.txt", strings.NewReader("first"))
	require.NoError(t, err)
	second, err := app.Storage.Save(context.Background(), "second.txt", strings.NewReader("second"))
	require.NoError(t, err)
	fileOne := &model.File{Name: "first.txt", Key: first.Key, URL: first.URL, MIME: first.MIME, Size: first.Size}
	require.NoError(t, app.Repos.Files.Create(context.Background(), fileOne))
	_, token, err := app.Services.Files.CreateShare(context.Background(), fileOne.ID, 1, time.Hour)
	require.NoError(t, err)

	// 分享令牌只能访问签发时的文件。
	req := httptest.NewRequest(http.MethodGet, second.URL+"?share="+token, nil)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 非法 Bearer。
	req = httptest.NewRequest(http.MethodGet, first.URL, nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 损坏的文件访问 Cookie。
	req = httptest.NewRequest(http.MethodGet, first.URL, nil)
	req.AddCookie(&http.Cookie{Name: security.FileAccessCookie, Value: "broken"})
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 无效分享令牌。
	req = httptest.NewRequest(http.MethodGet, first.URL+"?share=bad", nil)
	rec = httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	_ = pair
}
