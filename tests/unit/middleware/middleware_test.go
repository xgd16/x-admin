package middleware_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/config"
	"x-admin/internal/middleware"
	"x-admin/internal/model"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/requestid"
	"x-admin/internal/pkg/security"
)

func newRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestRequestIDMiddleware(t *testing.T) {
	r := newRouter()
	r.Use(middleware.RequestID())
	r.GET("/", func(c *gin.Context) {
		c.String(200, requestid.FromContext(c.Request.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestid.Header, "incoming")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, "incoming", rr.Body.String())
	assert.Equal(t, "incoming", rr.Header().Get(requestid.Header))

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestid.Header, strings.Repeat("x", 200))
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Len(t, rr.Body.String(), 32)
}

func TestRecoveryMiddleware(t *testing.T) {
	r := newRouter()
	r.Use(middleware.RequestID(), middleware.Recovery())
	r.GET("/panic", func(*gin.Context) { panic("boom") })
	r.GET("/ok", func(c *gin.Context) { c.String(200, "ok") })

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/panic", nil))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "50001")

	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ok", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRecoveryAfterWrite(t *testing.T) {
	r := newRouter()
	r.Use(middleware.Recovery())
	r.GET("/panic", func(c *gin.Context) {
		c.String(200, "partial")
		panic("late")
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/panic", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "partial", rr.Body.String())
}

func TestCORSMiddleware(t *testing.T) {
	cfg := configCORS([]string{"http://localhost:5173"})
	r := newRouter()
	r.Use(middleware.CORS(cfg))
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, "http://localhost:5173", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rr.Header().Get("Access-Control-Allow-Headers"), "Authorization")

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.test")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))

	req = httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestCORSWildcard(t *testing.T) {
	r := newRouter()
	r.Use(middleware.CORS(configCORS([]string{"*"})))
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://any.test")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestAuthMiddleware(t *testing.T) {
	tokens := security.NewTokenManager("0123456789abcdef", "test", time.Hour)
	token, _, err := tokens.Issue(7, "alice")
	require.NoError(t, err)
	loader := &fakeLoader{principal: &authn.Principal{UserID: 7, Username: "alice", Super: true}}

	r := newRouter()
	r.Use(middleware.Auth(tokens, loader))
	r.GET("/me", func(c *gin.Context) {
		p, _ := authn.FromContext(c.Request.Context())
		c.JSON(200, gin.H{"username": p.Username})
	})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/me", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	for _, header := range []string{"Basic abc", "Bearer"} {
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", header)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "alice")
}

func TestAuthMiddlewareExpiredAndLoaderError(t *testing.T) {
	tokens := security.NewTokenManager("0123456789abcdef", "test", time.Hour)
	expiredTokens := security.NewTokenManager("0123456789abcdef", "test", -time.Second)
	token, _, err := expiredTokens.Issue(1, "a")
	require.NoError(t, err)

	r := newRouter()
	r.Use(middleware.Auth(tokens, &fakeLoader{err: errors.New("db down")}))
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "40102")

	valid, _, err := tokens.Issue(1, "a")
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+valid)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestRequirePermission(t *testing.T) {
	r := newRouter()
	r.Use(func(c *gin.Context) {
		if raw := c.GetHeader("X-Principal"); raw != "" {
			p := &authn.Principal{Super: raw == "super", Permissions: map[string]struct{}{"a:b:c": {}}}
			c.Request = c.Request.WithContext(authn.With(c.Request.Context(), p))
		}
	})
	r.GET("/", middleware.Require("a:b:c"), func(c *gin.Context) { c.String(200, "ok") })

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Principal", "user")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Principal", "super")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	r2 := newRouter()
	r2.Use(func(c *gin.Context) {
		p := &authn.Principal{}
		c.Request = c.Request.WithContext(authn.With(c.Request.Context(), p))
	})
	r2.GET("/", middleware.Require("a:b:c"), func(c *gin.Context) { c.String(200, "ok") })
	rr = httptest.NewRecorder()
	r2.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRateLimit(t *testing.T) {
	store := cache.NewMemory()
	r := newRouter()
	r.Use(middleware.RateLimit(store, "test", 2, time.Minute))
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, rr.Code)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))

	disabled := newRouter()
	disabled.Use(middleware.RateLimit(store, "x", 0, time.Minute))
	disabled.GET("/", func(c *gin.Context) { c.String(200, "ok") })
	rr = httptest.NewRecorder()
	disabled.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimitStoreFailureFailsOpen(t *testing.T) {
	r := newRouter()
	r.Use(middleware.RateLimit(failingStore{}, "x", 1, time.Minute))
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}

type captureRecorder struct{ logs []*model.OperationLog }

func (c *captureRecorder) RecordOperation(log *model.OperationLog) { c.logs = append(c.logs, log) }

func TestAuditMiddleware(t *testing.T) {
	rec := &captureRecorder{}
	r := newRouter()
	r.Use(middleware.RequestID(), middleware.Audit(rec))
	r.POST("/admin/v1/users", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		c.Status(http.StatusCreated)
	})
	r.GET("/admin/v1/users", func(c *gin.Context) { c.String(200, "ok") })
	r.POST("/admin/v1/auth/login", func(c *gin.Context) { c.String(200, "ok") })

	body := `{"username":"alice","password":"secret","nested":{"refresh_token":"abc","ok":1}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer x")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Len(t, rec.logs, 1)
	log := rec.logs[0]
	assert.Equal(t, "POST", log.Method)
	assert.Equal(t, "/admin/v1/users", log.Path)
	assert.Equal(t, http.StatusCreated, log.Status)
	assert.NotContains(t, log.Body, "secret")
	assert.NotContains(t, log.Body, "abc")
	assert.Contains(t, log.Body, "***")
	assert.NotEmpty(t, log.RequestID)

	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/v1/users", nil))
	assert.Len(t, rec.logs, 1)

	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/admin/v1/auth/login", strings.NewReader("{}")))
	assert.Len(t, rec.logs, 1)
}

func TestAuditNonJSONAndInvalid(t *testing.T) {
	rec := &captureRecorder{}
	r := newRouter()
	r.Use(middleware.Audit(rec))
	r.POST("/x", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		c.Status(200)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("plain"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Len(t, rec.logs, 1)
	assert.Equal(t, "[non-json body]", rec.logs[0].Body)

	req = httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("plain"))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Len(t, rec.logs, 2)
	assert.Equal(t, "[non-json body]", rec.logs[1].Body)
}

func TestAuditNilRecorder(t *testing.T) {
	r := newRouter()
	r.Use(middleware.Audit(nil))
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAccessLog(t *testing.T) {
	r := newRouter()
	r.Use(middleware.AccessLog())
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}

type fakeLoader struct {
	principal *authn.Principal
	err       error
}

func (f *fakeLoader) Authenticate(context.Context, uint64) (*authn.Principal, error) {
	return f.principal, f.err
}

type failingStore struct{}

func (failingStore) Set(context.Context, string, string, time.Duration) error { return nil }
func (failingStore) Get(context.Context, string) (string, error)              { return "", cache.ErrNotFound }
func (failingStore) Take(context.Context, string) (string, error)             { return "", cache.ErrNotFound }
func (failingStore) Del(context.Context, ...string) error                     { return nil }
func (failingStore) Exists(context.Context, string) (bool, error)             { return false, nil }
func (failingStore) Incr(context.Context, string, time.Duration) (int64, error) {
	return 0, errors.New("down")
}
func (failingStore) Ping(context.Context) error { return nil }
func (failingStore) Close() error               { return nil }

func configCORS(origins []string) config.CORSConfig {
	return config.CORSConfig{Origins: origins}
}
