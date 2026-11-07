package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"

	"x-admin/internal/middleware"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/security"
)

type fakeSessions struct {
	active bool
	err    error
}

func (f *fakeSessions) SessionActive(context.Context, uint64, string) (bool, error) {
	return f.active, f.err
}

func TestAuthMiddlewareSessionChecks(t *testing.T) {
	tokens := security.NewTokenManager("0123456789abcdef", "test", time.Hour)
	principal := &authn.Principal{UserID: 7, Username: "alice"}
	loader := &fakeLoader{principal: principal}

	var gotSID string
	newAuthed := func(sessions middleware.SessionChecker) (*gin.Engine, string) {
		r := newRouter()
		r.Use(middleware.Auth(tokens, loader, sessions))
		r.GET("/", func(c *gin.Context) {
			p, _ := authn.FromContext(c.Request.Context())
			gotSID = p.SessionID
			c.String(200, "ok")
		})
		token, _, err := tokens.Issue(7, "alice", 0, "sid-1")
		require.NoError(t, err)
		return r, token
	}

	do := func(r *gin.Engine, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr
	}

	// 加载到 nil 身份视同未认证。
	loaderNil := &fakeLoader{}
	r := newRouter()
	r.Use(middleware.Auth(tokens, loaderNil, nil))
	r.GET("/", func(c *gin.Context) { c.String(200, "ok") })
	blankToken, _, err := tokens.Issue(9, "bob", 0, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, do(r, blankToken).Code)

	// 会话校验开启后：空会话 ID、校验失败、会话不活跃均拒绝。
	rEmpty, _ := newAuthed(&fakeSessions{})
	// 空 SessionID 的令牌需要单独签发。
	noSID, _, err := tokens.Issue(7, "alice", 0, "")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, do(rEmpty, noSID).Code)

	rErr, tokenErr := newAuthed(&fakeSessions{err: errors.New("redis down")})
	assert.Equal(t, http.StatusInternalServerError, do(rErr, tokenErr).Code)

	rOff, tokenOff := newAuthed(&fakeSessions{active: false})
	assert.Equal(t, http.StatusUnauthorized, do(rOff, tokenOff).Code)

	rOn, tokenOn := newAuthed(&fakeSessions{active: true})
	rr := do(rOn, tokenOn)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "sid-1", gotSID)
}

type fakeFileChecker struct {
	sessionErr error
	checkErr   error
}

func (f *fakeFileChecker) SessionActive(context.Context, uint64, string) (bool, error) {
	return true, f.sessionErr
}

func (f *fakeFileChecker) CheckFileAccess(context.Context, uint64, int64, string) error {
	return f.checkErr
}

type fakeShares struct {
	path string
	err  error
}

func (f *fakeShares) ResolveShare(context.Context, string) (string, error) {
	return f.path, f.err
}

func TestUploadAuthFullMatrix(t *testing.T) {
	tokens := security.NewTokenManager("0123456789abcdef", "test", time.Hour)
	access := tokens.IssueFileAccess(7, 1, "sid-1", time.Hour)
	token, _, err := tokens.Issue(7, "alice", 0, "sid-1")
	require.NoError(t, err)

	newRouterWith := func(checker middleware.FileAccessChecker, shares middleware.ShareResolver) *gin.Engine {
		r := newRouter()
		r.Use(middleware.UploadAuth(tokens, checker, shares))
		r.GET("/uploads/*filepath", func(c *gin.Context) { c.String(200, "ok") })
		r.GET("/api", func(c *gin.Context) { c.String(200, "ok") })
		return r
	}
	get := func(r *gin.Engine, path, cookie string, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: security.FileAccessCookie, Value: cookie})
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr
	}

	// 非 uploads 路径直接放行。
	ok := &fakeFileChecker{}
	assert.Equal(t, http.StatusOK, get(newRouterWith(ok, nil), "/api", "", "").Code)

	// 未配置鉴权。
	req := httptest.NewRequest(http.MethodGet, "/uploads/a.png", nil)
	rr := httptest.NewRecorder()
	newRouterWith(nil, nil).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// 分享链接：解析器缺失、解析失败、路径不匹配、匹配成功。
	noShares := newRouterWith(ok, nil)
	req = httptest.NewRequest(http.MethodGet, "/uploads/a.png?share=tk", nil)
	rr = httptest.NewRecorder()
	noShares.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	badShares := newRouterWith(ok, &fakeShares{err: errors.New("revoked")})
	req = httptest.NewRequest(http.MethodGet, "/uploads/a.png?share=tk", nil)
	rr = httptest.NewRecorder()
	badShares.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	mismatch := newRouterWith(ok, &fakeShares{path: "/uploads/b.png"})
	req = httptest.NewRequest(http.MethodGet, "/uploads/a.png?share=tk", nil)
	rr = httptest.NewRecorder()
	mismatch.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	match := newRouterWith(ok, &fakeShares{path: "/uploads/a.png"})
	req = httptest.NewRequest(http.MethodGet, "/uploads/a.png?share=tk", nil)
	rr = httptest.NewRecorder()
	match.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))

	// Bearer 路径：凭证无效 / 校验失败 / 成功。
	badToken := newRouterWith(ok, nil)
	assert.Equal(t, http.StatusUnauthorized, get(badToken, "/uploads/a.png", "", "bad").Code)

	checkFail := newRouterWith(&fakeFileChecker{checkErr: errors.New("session revoked")}, nil)
	rr = get(checkFail, "/uploads/a.png", "", token)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))

	assert.Equal(t, http.StatusOK, get(newRouterWith(ok, nil), "/uploads/a.png", "", token).Code)

	// Cookie 路径：缺失 / 无效 / 校验失败 / 成功。
	rr = get(newRouterWith(ok, nil), "/uploads/a.png", "", "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "请先登录")

	rr = get(newRouterWith(ok, nil), "/uploads/a.png", "garbage", "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "已过期")

	rr = get(newRouterWith(&fakeFileChecker{checkErr: errors.New("stale")}, nil), "/uploads/a.png", access, "")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	assert.Equal(t, http.StatusOK, get(newRouterWith(ok, nil), "/uploads/a.png", access, "").Code)
}

func TestAuditTruncation(t *testing.T) {
	rec := &captureRecorder{}
	r := newRouter()
	r.Use(middleware.RequestID(), middleware.Audit(rec))
	r.POST("/admin/v1/users", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		c.Error(errors.New(strings.Repeat("x", 600)))
		c.Status(http.StatusInternalServerError)
	})

	// 请求体 20KB（合法 JSON，落库前截断到 4KB）；UA 用多字节字符触发 UTF-8 边界回退。
	body := `{"items":[{"password":"a"},{"password":"b"}],"pad":"` + strings.Repeat("a", 20*1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "a"+strings.Repeat("浏", 200)) // 601 字节，255 处跨字符
	req.Header.Set("Authorization", "Bearer t")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Len(t, rec.logs, 1)
	log := rec.logs[0]
	assert.Len(t, log.Body, 4096)
	assert.Len(t, log.UserAgent, 253) // 回退到 "a"+84 个汉字的合法边界
	assert.LessOrEqual(t, len(log.Error), 500)
	assert.Contains(t, log.Body, `"pad":"aaa`)
	assert.Equal(t, http.StatusInternalServerError, log.Status)
}

func TestAuditBodyCaptureSeal(t *testing.T) {
	rec := &captureRecorder{}
	r := newRouter()
	r.Use(middleware.Audit(rec))
	r.POST("/admin/v1/users", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		c.Status(http.StatusOK)
	})

	// 超过 64KB 的请求体触发捕获封顶：缓冲区停止增长，剩余内容只透传不记录。
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users", strings.NewReader(strings.Repeat("x", 128*1024)))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Len(t, rec.logs, 1)
	assert.Equal(t, "[non-json body]", rec.logs[0].Body)
	assert.Equal(t, http.StatusOK, rec.logs[0].Status)
}
