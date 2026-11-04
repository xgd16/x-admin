// Package router 负责注册中间件与全部 HTTP 路由。
//
// 路由在这里集中声明，便于一眼确认每个接口使用的权限码。
package router

import (
	"context"
	"log/slog"
	"net/http"
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

	api := r.Group("/admin/v1")
	// 全局兜底限流，登录等敏感接口会在路由上叠加更严格的限制。
	api.Use(middleware.RateLimit(d.Cache, "rl:global", 100, time.Minute))

	// 无需登录的认证接口；验证码、登录、二次验证和 Passkey 分别限流。
	auth := api.Group("/auth")
	auth.GET("/captcha", d.Handlers.Auth.Captcha)
	auth.POST("/login", middleware.RateLimit(d.Cache, "rl:login", 10, time.Minute), d.Handlers.Auth.Login)
	auth.POST("/login/mfa", middleware.RateLimit(d.Cache, "rl:mfa", 20, time.Minute), d.Handlers.Auth.LoginMFA)
	auth.POST("/passkey/login/begin", middleware.RateLimit(d.Cache, "rl:passkey", 20, time.Minute), d.Handlers.Auth.PasskeyLoginBegin)
	auth.POST("/passkey/login/finish", middleware.RateLimit(d.Cache, "rl:passkey", 20, time.Minute), d.Handlers.Auth.PasskeyLoginFinish)
	auth.POST("/refresh", d.Handlers.Auth.Refresh)

	// 公开参数：登录页、品牌配置等未登录场景可读取 site.name / site.logo。
	api.GET("/parameters/public", d.Handlers.Parameter.Public)

	// 以下接口都要求 Bearer token，并通过权限码做细粒度授权。
	secured := api.Group("")
	secured.Use(middleware.Auth(d.Tokens, d.Auth, d.Auth))
	{
		secured.POST("/auth/logout", d.Handlers.Auth.Logout)
		secured.GET("/auth/me", d.Handlers.Auth.Me)

		secured.GET("/dashboard/summary", middleware.Require("dashboard:view"), d.Handlers.Dashboard.Summary)

		// 用户管理：查询、增删改、状态、密码重置、角色分配、导入导出。
		secured.GET("/users", middleware.Require("system:user:list"), d.Handlers.User.List)
		secured.GET("/users/:id", middleware.Require("system:user:list"), d.Handlers.User.Get)
		secured.POST("/users", middleware.Require("system:user:create"), d.Handlers.User.Create)
		secured.PUT("/users/:id", middleware.Require("system:user:update"), d.Handlers.User.Update)
		secured.DELETE("/users", middleware.Require("system:user:delete"), d.Handlers.User.Delete)
		secured.PUT("/users/:id/status", middleware.Require("system:user:update"), d.Handlers.User.SetStatus)
		secured.PUT("/users/:id/password", middleware.Require("system:user:reset"), d.Handlers.User.ResetPassword)
		secured.PUT("/users/:id/roles", middleware.Require("system:user:update"), d.Handlers.User.AssignRoles)
		secured.POST("/users/import", middleware.Require("system:user:import"), d.Handlers.User.Import)
		secured.GET("/users/export", middleware.Require("system:user:export"), d.Handlers.User.Export)
		secured.GET("/users/template", middleware.Require("system:user:import"), d.Handlers.User.Template)

		// 角色管理：CRUD 与菜单授权。
		secured.GET("/roles", middleware.Require("system:role:list"), d.Handlers.Role.List)
		secured.GET("/roles/all", middleware.Require("system:role:list"), d.Handlers.Role.All)
		secured.POST("/roles", middleware.Require("system:role:create"), d.Handlers.Role.Create)
		secured.PUT("/roles/:id", middleware.Require("system:role:update"), d.Handlers.Role.Update)
		secured.DELETE("/roles/:id", middleware.Require("system:role:delete"), d.Handlers.Role.Delete)
		secured.GET("/roles/:id/menus", middleware.Require("system:role:list"), d.Handlers.Role.Menus)
		secured.PUT("/roles/:id/menus", middleware.Require("system:role:assign"), d.Handlers.Role.AssignMenus)

		// 菜单管理：目录/菜单/按钮统一以树结构维护。
		secured.GET("/menus/tree", middleware.Require("system:menu:list"), d.Handlers.Menu.Tree)
		secured.POST("/menus", middleware.Require("system:menu:create"), d.Handlers.Menu.Create)
		secured.PUT("/menus/:id", middleware.Require("system:menu:update"), d.Handlers.Menu.Update)
		secured.DELETE("/menus/:id", middleware.Require("system:menu:delete"), d.Handlers.Menu.Delete)

		// 部门管理：树结构，删除前会检查子部门与用户。
		secured.GET("/departments/tree", middleware.Require("system:dept:list"), d.Handlers.Department.Tree)
		secured.POST("/departments", middleware.Require("system:dept:create"), d.Handlers.Department.Create)
		secured.PUT("/departments/:id", middleware.Require("system:dept:update"), d.Handlers.Department.Update)
		secured.DELETE("/departments/:id", middleware.Require("system:dept:delete"), d.Handlers.Department.Delete)

		// 字典管理：类型与字典项分开维护。
		secured.GET("/dict/types", middleware.Require("system:dict:list"), d.Handlers.Dict.ListTypes)
		secured.POST("/dict/types", middleware.Require("system:dict:create"), d.Handlers.Dict.CreateType)
		secured.PUT("/dict/types/:id", middleware.Require("system:dict:update"), d.Handlers.Dict.UpdateType)
		secured.DELETE("/dict/types/:id", middleware.Require("system:dict:delete"), d.Handlers.Dict.DeleteType)
		secured.GET("/dict/types/:id/items", middleware.Require("system:dict:list"), d.Handlers.Dict.Items)
		secured.POST("/dict/types/:id/items", middleware.Require("system:dict:create"), d.Handlers.Dict.CreateItem)
		secured.PUT("/dict/items/:id", middleware.Require("system:dict:update"), d.Handlers.Dict.UpdateItem)
		secured.DELETE("/dict/items/:id", middleware.Require("system:dict:delete"), d.Handlers.Dict.DeleteItem)

		// 系统参数：键值对形式的运行期配置。
		secured.GET("/parameters", middleware.Require("system:param:list"), d.Handlers.Parameter.List)
		secured.POST("/parameters", middleware.Require("system:param:create"), d.Handlers.Parameter.Create)
		secured.PUT("/parameters/:id", middleware.Require("system:param:update"), d.Handlers.Parameter.Update)
		secured.DELETE("/parameters/:id", middleware.Require("system:param:delete"), d.Handlers.Parameter.Delete)

		// 文件管理：上传、列表、下载、删除。
		secured.GET("/files", middleware.Require("system:file:list"), d.Handlers.File.List)
		secured.POST("/files", middleware.Require("system:file:upload"), d.Handlers.File.Upload)
		secured.GET("/files/:id/download", middleware.Require("system:file:download"), d.Handlers.File.Download)
		secured.GET("/files/:id/shares", middleware.Require("system:file:download"), d.Handlers.File.ListShares)
		secured.POST("/files/:id/shares", middleware.Require("system:file:download"), d.Handlers.File.CreateShare)
		secured.DELETE("/files/:id/shares/:shareID", middleware.Require("system:file:download"), d.Handlers.File.RevokeShare)
		secured.DELETE("/files/:id", middleware.Require("system:file:delete"), d.Handlers.File.Delete)

		// 定时任务：CRUD、启停与立即执行。
		secured.GET("/jobs", middleware.Require("system:job:list"), d.Handlers.Job.List)
		secured.POST("/jobs", middleware.Require("system:job:create"), d.Handlers.Job.Create)
		secured.PUT("/jobs/:id", middleware.Require("system:job:update"), d.Handlers.Job.Update)
		secured.DELETE("/jobs/:id", middleware.Require("system:job:delete"), d.Handlers.Job.Delete)
		secured.PUT("/jobs/:id/status", middleware.Require("system:job:update"), d.Handlers.Job.SetStatus)
		secured.POST("/jobs/:id/run", middleware.Require("system:job:run"), d.Handlers.Job.Run)
		secured.GET("/jobs/:id/runs", middleware.Require("system:job:list"), d.Handlers.Job.Runs)

		// 审计日志：操作日志与登录日志。
		secured.GET("/logs/operations", middleware.Require("system:log:operation"), d.Handlers.Log.Operations)
		secured.GET("/logs/operations/export", middleware.Require("system:log:operation"), d.Handlers.Log.ExportOperations)
		secured.DELETE("/logs/operations", middleware.Require("system:log:operation"), d.Handlers.Log.DeleteOperations)
		secured.DELETE("/logs/operations/clear", middleware.Require("system:log:operation"), d.Handlers.Log.ClearOperations)
		secured.GET("/logs/logins", middleware.Require("system:log:login"), d.Handlers.Log.Logins)
		secured.GET("/logs/logins/export", middleware.Require("system:log:login"), d.Handlers.Log.ExportLogins)
		secured.DELETE("/logs/logins/clear", middleware.Require("system:log:login"), d.Handlers.Log.ClearLogins)

		// 个人中心：登录即可访问，不需要额外权限码。
		secured.GET("/profile", d.Handlers.Profile.Get)
		secured.PUT("/profile", d.Handlers.Profile.Update)
		secured.PUT("/profile/password", d.Handlers.Profile.ChangePassword)
		secured.GET("/profile/sessions", d.Handlers.Profile.Sessions)
		secured.DELETE("/profile/sessions/:id", d.Handlers.Profile.RevokeSession)
		secured.POST("/profile/session-actions/revoke-others", d.Handlers.Profile.RevokeOtherSessions)
		secured.POST("/profile/session-actions/revoke-all", d.Handlers.Profile.RevokeAllSessions)
		secured.GET("/profile/totp", d.Handlers.Profile.TOTPStatus)
		secured.POST("/profile/totp/setup", d.Handlers.Profile.SetupTOTP)
		secured.POST("/profile/totp/enable", d.Handlers.Profile.EnableTOTP)
		secured.DELETE("/profile/totp", d.Handlers.Profile.DisableTOTP)

		secured.GET("/profile/passkeys", d.Handlers.Profile.ListPasskeys)
		secured.POST("/profile/passkeys/begin", d.Handlers.Profile.BeginPasskey)
		secured.POST("/profile/passkeys/finish", d.Handlers.Profile.FinishPasskey)
		secured.PUT("/profile/passkeys/:id", d.Handlers.Profile.RenamePasskey)
		secured.DELETE("/profile/passkeys/:id", d.Handlers.Profile.DeletePasskey)

		// 站内通知：按当前登录用户隔离。
		secured.GET("/notifications", d.Handlers.Notification.List)
		secured.GET("/notifications/unread-count", d.Handlers.Notification.UnreadCount)
		secured.PUT("/notifications/read-all", d.Handlers.Notification.MarkAllRead)
		secured.PUT("/notifications/:id/read", d.Handlers.Notification.MarkRead)
	}

	slog.Info("routes registered", "prefix", "/admin/v1")
	return r
}
