// Package router 负责注册中间件与全部 HTTP 路由。
//
// 路由按 API 面拆分：admin.go 是内置后台面，其他面由 go tool xadmin create api
// 生成；公共挂载逻辑与 Surface/Module 注册表在 surface.go。
// 每个 API 面拥有独立前缀、限流桶和认证体系，后台权限码只在 admin 面使用。
package router

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/config"
	"x-admin/internal/docs"
	"x-admin/internal/handler"
	"x-admin/internal/middleware"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/security"
	"x-admin/internal/service"
)

// Handlers 汇总所有 HTTP 处理器，避免 New 的参数列表过长。
type Handlers struct {
	Auth         *handler.AuthHandler
	Dashboard    *handler.DashboardHandler
	User         *handler.UserHandler
	Role         *handler.RoleHandler
	Menu         *handler.MenuHandler
	Department   *handler.DepartmentHandler
	Dict         *handler.DictHandler
	Parameter    *handler.ParameterHandler
	File         *handler.FileHandler
	Job          *handler.JobHandler
	Log          *handler.LogHandler
	Profile      *handler.ProfileHandler
	Notification *handler.NotificationHandler
	Export       *handler.ExportHandler
	Online       *handler.OnlineHandler
	Monitor      *handler.MonitorHandler
}

// Deps 是构建路由所需的依赖集合。
type Deps struct {
	Config   config.Config
	Tokens   *security.TokenManager
	Auth     *service.AuthService
	Captcha  *captcha.Generator
	Cache    cache.Store
	Recorder middleware.OperationRecorder
	Handlers Handlers
	Shares   middleware.ShareResolver        // 可撤销文件分享令牌解析
	Health   func(ctx context.Context) error // 健康检查依赖探测

	// SurfaceTokens 是各 API 面的 JWT 管理器，键为面名称。
	// admin 与 Tokens 指向同一个实例；自定义面使用独立 issuer。
	SurfaceTokens map[string]*security.TokenManager
	// SurfaceAuth 按 API 面提供认证中间件。未提供时该面只注册公开路由。
	SurfaceAuth map[string]gin.HandlerFunc
	// SurfaceAuthService 为 create api --auth 生成的自定义面提供账号注册/登录/刷新。
	SurfaceAuthService *service.SurfaceAuthService
}

// New 构建 Gin 引擎并注册全部路由。
func New(d Deps) *gin.Engine {
	if d.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// 只有来自受信代理的 X-Forwarded-For / X-Real-IP 才会被用于 ClientIP。
	// 否则 Nginx 反代后所有请求都会显示为同一个容器 IP，限流与审计都会失真。
	if err := r.SetTrustedProxies(d.Config.Server.TrustedProxies); err != nil {
		slog.Warn("invalid trusted proxies", "error", err)
	}
	// 全局中间件顺序很关键：请求 ID 最先写入，恢复器随后兜底，
	// 访问日志和审计需要拿到最终的响应状态。
	r.Use(middleware.RequestID(), middleware.Recovery(), middleware.AccessLog(), middleware.CORS(d.Config.CORS))
	r.Use(middleware.Audit(d.Recorder))
	// 上传目录的安全响应头必须在静态文件路由之前注册。
	r.Use(middleware.UploadSecurityHeaders())
	// /uploads 不再是公开静态目录：浏览器用 HttpOnly Cookie，API 客户端可用 Bearer。
	r.Use(middleware.UploadAuth(d.Tokens, d.Auth, d.Shares))

	// 健康检查同时探测数据库和缓存，任一不可用都返回 503。
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if d.Health != nil {
			if err := d.Health(ctx); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// 上传目录直接由 Gin 提供静态访问，生产环境通常交给 Nginx。
	r.Static("/uploads", d.Config.Storage.Root)

	// 内嵌的 Swagger UI 与 OpenAPI 文档。
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(302, "/swagger/index.html")
	})
	r.GET("/swagger/index.html", func(c *gin.Context) {
		c.Data(200, "text/html; charset=utf-8", []byte(docs.IndexHTML))
	})
	r.GET("/swagger/openapi.yaml", func(c *gin.Context) {
		c.Data(200, "application/yaml; charset=utf-8", docs.OpenAPI)
	})

	// 多 API 面：按配置挂载全部启用的面。内置 admin 通过
	// RegisterSurface 注册；go tool xadmin create api 生成的面也会自动加入。
	surfaces := d.Config.API.Surfaces()
	names := make([]string, 0, len(surfaces))
	for name := range surfaces {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		surfaceCfg := surfaces[name]
		if !surfaceCfg.Enabled {
			continue
		}
		factory, ok := surfaceFactories[name]
		if !ok {
			slog.Warn("api surface enabled but no route module registered", "surface", name)
			continue
		}
		factory(d, surfaceCfg).mount(r, d)
	}

	return r
}
