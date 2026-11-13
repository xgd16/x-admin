package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/config"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/router"
)

func init() {
	// 模拟 go tool xadmin create api 生成的自定义 API 面。
	// 框架默认只有 admin，因此测试里显式注册一个 testapi。
	router.RegisterSurface("testapi", func(d router.Deps, cfg config.APISurfaceConfig) router.Surface {
		return router.Surface{
			Name:    "testapi",
			Prefix:  cfg.Prefix,
			Enabled: cfg.Enabled,
			Version: "v1",
		}
	})
}

func newTestRouter(t *testing.T, mutate func(cfg *config.Config)) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.Default()
	// 测试只关心 API 面挂载，关闭 admin 避免构造整套后台 handler。
	cfg.API.Admin.Enabled = false
	cfg.API.Extra = map[string]config.APISurfaceConfig{
		"testapi": {Enabled: true, Prefix: "/testapi/v1"},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return router.New(router.Deps{Config: cfg, Cache: cache.NewMemory()})
}

func doRequest(r http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(rec, req)
	return rec
}

func TestAPISurfacesMounted(t *testing.T) {
	r := newTestRouter(t, nil)

	rec := doRequest(r, "/testapi/v1/ping")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"api":"testapi"`)

	// 默认只有 admin；测试里显式关闭后不应存在。
	assert.Equal(t, http.StatusNotFound, doRequest(r, "/admin/v1/ping").Code)
}

func TestAPISurfaceCanBeDisabled(t *testing.T) {
	r := newTestRouter(t, func(cfg *config.Config) {
		cfg.API.Extra["testapi"] = config.APISurfaceConfig{Enabled: false, Prefix: "/testapi/v1"}
	})

	assert.Equal(t, http.StatusNotFound, doRequest(r, "/testapi/v1/ping").Code)
}

func TestAPISurfacePrefixCanBeChanged(t *testing.T) {
	r := newTestRouter(t, func(cfg *config.Config) {
		cfg.API.Extra["testapi"] = config.APISurfaceConfig{Enabled: true, Prefix: "/partner/v1"}
	})

	assert.Equal(t, http.StatusOK, doRequest(r, "/partner/v1/ping").Code)
	assert.Equal(t, http.StatusNotFound, doRequest(r, "/testapi/v1/ping").Code)
}
