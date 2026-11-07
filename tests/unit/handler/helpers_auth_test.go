package handler_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/handler"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/security"
	"x-admin/tests/testsupport"
)

// failingCaptchaStore 让验证码生成在写入挑战时失败。
type failingCaptchaStore struct{ cache.Store }

func (failingCaptchaStore) Set(context.Context, string, string, time.Duration) error {
	return errors.New("store down")
}

func TestPathIDHelpers(t *testing.T) {
	r := gin.New()
	r.GET("/t/:id", func(c *gin.Context) {
		id, ok := handler.PathID(c)
		if !ok {
			return
		}
		c.String(200, "%d", id)
	})
	r.GET("/u/:uid", func(c *gin.Context) {
		id, ok := handler.PathParamID(c, "uid")
		if !ok {
			return
		}
		c.String(200, "%d", id)
	})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/t/5", nil))
	assert.Equal(t, "5", rr.Body.String())

	for _, bad := range []string{"/t/abc", "/t/0"} {
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, bad, nil))
		assert.Equal(t, http.StatusBadRequest, rr.Code, bad)
		assert.Contains(t, rr.Body.String(), "ID 不合法")
	}

	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/u/abc", nil))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPrincipalHelpers(t *testing.T) {
	p := &authn.Principal{UserID: 42, Username: "alice", SessionID: "sid-9"}

	r := gin.New()
	r.GET("/with", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"id":  handler.CurrentUserID(c),
			"sid": handler.CurrentSessionID(c),
			"ok":  handler.CurrentPrincipal(c) != nil,
		})
	})
	r.GET("/without", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"id":  handler.CurrentUserID(c),
			"sid": handler.CurrentSessionID(c),
			"ok":  handler.CurrentPrincipal(c) != nil,
		})
	})

	inject := func(c *gin.Context) {
		c.Request = c.Request.WithContext(authn.With(c.Request.Context(), p))
		c.Next()
	}
	r2 := gin.New()
	r2.Use(inject)
	r2.GET("/with", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"id":  handler.CurrentUserID(c),
			"sid": handler.CurrentSessionID(c),
			"ok":  handler.CurrentPrincipal(c) != nil,
		})
	})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/without", nil))
	assert.Contains(t, rr.Body.String(), `"id":0`)
	assert.Contains(t, rr.Body.String(), `"ok":false`)

	rr = httptest.NewRecorder()
	r2.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/with", nil))
	assert.Contains(t, rr.Body.String(), `"id":42`)
	assert.Contains(t, rr.Body.String(), `"sid":"sid-9"`)
	assert.Contains(t, rr.Body.String(), `"ok":true`)
}

func TestBindAndAttachment(t *testing.T) {
	r := gin.New()
	r.POST("/bind", func(c *gin.Context) {
		var in struct {
			Name string `json:"name" binding:"required"`
		}
		if !handler.Bind(c, &in) {
			return
		}
		c.String(200, "ok:"+in.Name)
	})
	r.GET("/file", func(c *gin.Context) {
		handler.Attachment(c, "报表.xlsx", []byte("xlsx-bytes"))
	})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(`{"name":"x"}`)))
	assert.Equal(t, "ok:x", rr.Body.String())

	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(`{}`)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/file", nil))
	assert.Equal(t, "xlsx-bytes", rr.Body.String())
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "报表.xlsx")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment")
}

func TestAuthHandlerFileCookie(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	type cookieResult struct {
		maxAge int
		secure bool
		found  bool
	}
	fileCookie := func(rr *httptest.ResponseRecorder) cookieResult {
		for _, ck := range rr.Result().Cookies() {
			if ck.Name == security.FileAccessCookie {
				return cookieResult{maxAge: ck.MaxAge, secure: ck.Secure, found: true}
			}
		}
		return cookieResult{}
	}

	newAuthRouter := func(secureCookie bool, fileTTL time.Duration, tokens *security.TokenManager) *gin.Engine {
		authH := handler.NewAuthHandler(app.Services.Auth, app.Captcha, app.Services.Passkeys, tokens, secureCookie, fileTTL)
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(authn.With(c.Request.Context(), &authn.Principal{
				UserID: admin.ID, Username: admin.Username, SessionVersion: admin.SessionVersion, SessionID: "sid-1",
			}))
			c.Next()
		})
		r.POST("/auth/refresh", authH.Refresh)
		r.POST("/auth/logout", authH.Logout)
		r.GET("/auth/me", authH.Me)
		return r
	}

	do := func(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
		req := encodeJSONBody(method, path, body)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr
	}

	// fileTTL 为 0 时回退到 24 小时。
	ck := fileCookie(do(newAuthRouter(false, 0, app.Tokens), http.MethodPost, "/auth/refresh",
		map[string]string{"refresh_token": pair.RefreshToken}))
	require.True(t, ck.found)
	assert.Equal(t, 24*3600, ck.maxAge)
	assert.False(t, ck.secure)

	// 生产环境 + HTTPS 请求（模拟 TLS）→ Secure Cookie。
	rTLS := newAuthRouter(true, time.Hour, app.Tokens)
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.TLS = &tls.ConnectionState{}
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rr := httptest.NewRecorder()
	rTLS.ServeHTTP(rr, req)
	ck = fileCookie(rr)
	require.True(t, ck.found)
	assert.True(t, ck.secure)

	// 生产环境 + X-Forwarded-Proto: https → Secure Cookie。
	req = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr = httptest.NewRecorder()
	rTLS.ServeHTTP(rr, req)
	assert.True(t, fileCookie(rr).secure)

	// 生产环境 + 普通 HTTP → 不加 Secure，避免本地 HTTP 部署丢 Cookie。
	req = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rr = httptest.NewRecorder()
	rTLS.ServeHTTP(rr, req)
	ck = fileCookie(rr)
	require.True(t, ck.found)
	assert.False(t, ck.secure)

	// 无 TokenManager 时不写 Cookie。
	ck = fileCookie(do(newAuthRouter(false, time.Hour, nil), http.MethodPost, "/auth/refresh",
		map[string]string{"refresh_token": pair.RefreshToken}))
	assert.False(t, ck.found)
}

func TestAuthHandlerCaptchaAndPasskeyErrors(t *testing.T) {
	app := testsupport.New(t)

	// 缓存写入失败时验证码接口报错。
	broken := handler.NewAuthHandler(app.Services.Auth, captcha.New(failingCaptchaStore{}, time.Minute),
		app.Services.Passkeys, app.Tokens, false, time.Hour)
	r := gin.New()
	r.GET("/auth/captcha", broken.Captcha)
	r.POST("/auth/passkey/login/finish", broken.PasskeyLoginFinish)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/captcha", nil))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	// Passkey 完成接口：参数缺失走 Bind 失败分支。
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/auth/passkey/login/finish", strings.NewReader(`{}`)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// 未知会话走服务层错误分支。
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/auth/passkey/login/finish",
		strings.NewReader(`{"session_id":"missing","credential":{}}`)))
	assert.True(t, rr.Code >= http.StatusBadRequest, rr.Body.String())
}

// encodeJSONBody 构造带 Content-Type 的 JSON 请求。
func encodeJSONBody(method, path string, body any) *http.Request {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestMeWithoutSessionIDSkipsFileCookie(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	// 主体没有会话 ID（如接口调试场景）时不应写文件 Cookie。
	authH := handler.NewAuthHandler(app.Services.Auth, app.Captcha, app.Services.Passkeys, app.Tokens, false, time.Hour)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authn.With(c.Request.Context(), &authn.Principal{
			UserID: admin.ID, Username: admin.Username, SessionVersion: admin.SessionVersion,
		}))
		c.Next()
	})
	r.GET("/auth/me", authH.Me)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	for _, ck := range rr.Result().Cookies() {
		if ck.Name == security.FileAccessCookie {
			t.Fatal("session-less principal must not receive file cookie")
		}
	}
}
