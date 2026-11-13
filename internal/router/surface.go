package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/config"
	"x-admin/internal/middleware"
)

// Surface 描述一个独立 API 面。
//
// 每个面拥有自己的路径前缀和限流桶；Auth 非空时会额外创建 secured 子组，
// 由业务路由把需要登录的接口挂到 secured 上。admin 与 create 生成的自定义面
// 共用这套挂载逻辑，新增 API 面时只需要再实现一个 Surface。
type Surface struct {
	Name    string
	Prefix  string
	Enabled bool
	// Version 是该面的 API 版本（例如 v1）。非空时会自动挂载
	// go tool xadmin create handler 生成的路由模块。
	Version string
	// RateLimitKey 是全局限流桶前缀；为空时使用 rl:<name>:global。
	RateLimitKey string
	// Auth 是该 API 面的认证中间件；为空表示只注册公开接口。
	Auth gin.HandlerFunc
	// Mount 注册该面的路由。secured 在 Auth 为空时为 nil。
	// /ping 由 mount 统一注册，Mount 内不要重复注册。
	Mount func(api, secured *gin.RouterGroup)
}

// SurfaceFactory 根据配置创建 API 面，供内置面和生成代码注册。
type SurfaceFactory func(d Deps, cfg config.APISurfaceConfig) Surface

var surfaceFactories = make(map[string]SurfaceFactory)

// RegisterSurface 注册 API 面工厂。
// go tool xadmin create api 生成的文件会在 init 中调用它。
func RegisterSurface(name string, factory SurfaceFactory) {
	if name == "" || factory == nil {
		return
	}
	if _, exists := surfaceFactories[name]; exists {
		panic("router: duplicate api surface " + name)
	}
	surfaceFactories[name] = factory
}

// Module 是一个生成的路由模块，按 API 面和版本挂载。
// go tool xadmin create handler 生成的文件会在 init 中调用 RegisterModule。
type Module struct {
	API      string
	Version  string
	Register func(d Deps, api, secured *gin.RouterGroup)
}

var modules []Module

// RegisterModule 注册生成的路由模块。
func RegisterModule(m Module) {
	if m.API == "" || m.Version == "" || m.Register == nil {
		return
	}
	modules = append(modules, m)
}

// mountModules 挂载匹配 API 面和版本的生成模块。
func mountModules(d Deps, apiName, version string, api, secured *gin.RouterGroup) {
	for _, m := range modules {
		if m.API == apiName && m.Version == version {
			m.Register(d, api, secured)
		}
	}
}

// 内置 API 面只有 admin；client/merchant/partner 等自定义面由
// go tool xadmin create api 生成，并通过 RegisterSurface 追加。
func init() {
	RegisterSurface("admin", adminSurface)
}

// mount 把 API 面挂到 Gin 引擎上。
func (s Surface) mount(r *gin.Engine, d Deps) {
	if !s.Enabled {
		return
	}
	if s.Prefix == "" {
		slog.Warn("api surface skipped: empty prefix", "surface", s.Name)
		return
	}

	api := r.Group(s.Prefix)
	key := s.RateLimitKey
	if key == "" {
		key = "rl:" + s.Name + ":global"
	}
	api.Use(middleware.RateLimit(d.Cache, key, 100, time.Minute))

	// 所有 API 面提供统一的存活检查，便于网关和探针识别当前面。
	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"api": s.Name, "status": "ok"})
	})

	var secured *gin.RouterGroup
	if s.Auth != nil {
		secured = api.Group("")
		secured.Use(s.Auth)
	}
	if s.Mount != nil {
		s.Mount(api, secured)
	}
	// 生成器创建的模块按 API 面 + 版本自动挂载，不需要手改路由文件。
	if s.Version != "" {
		mountModules(d, s.Name, s.Version, api, secured)
	}
	slog.Info("routes registered", "surface", s.Name, "prefix", s.Prefix, "version", s.Version)
}
