package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/config"
	"x-admin/internal/middleware"
)

// adminSurface 是后台管理端 API 面。
// 它保持原有的 /admin/v1 前缀与后台 TokenManager/权限码体系。
func adminSurface(d Deps, cfg config.APISurfaceConfig) Surface {
	return Surface{
		Name:         "admin",
		Prefix:       cfg.Prefix,
		Enabled:      cfg.Enabled,
		Version:      "v1",
		RateLimitKey: "rl:global",
		Auth:         middleware.Auth(d.Tokens, d.Auth, d.Auth),
		Mount: func(api, secured *gin.RouterGroup) {
			registerAdminRoutes(d, api, secured)
		},
	}
}

// registerAdminRoutes 注册后台管理端全部路由。
// 受保护接口必须挂在 secured 组上，并显式声明权限码。
func registerAdminRoutes(d Deps, api, secured *gin.RouterGroup) {
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
	secured.POST("/auth/logout", d.Handlers.Auth.Logout)
	secured.GET("/auth/me", d.Handlers.Auth.Me)
	// 锁屏解锁：重新验证密码或当前用户的 Passkey，不签发新令牌。
	secured.POST("/auth/unlock", middleware.RateLimit(d.Cache, "rl:unlock", 10, time.Minute), d.Handlers.Auth.Unlock)
	secured.POST("/auth/unlock/passkey/begin", middleware.RateLimit(d.Cache, "rl:passkey", 20, time.Minute), d.Handlers.Auth.UnlockPasskeyBegin)
	secured.POST("/auth/unlock/passkey/finish", middleware.RateLimit(d.Cache, "rl:passkey", 20, time.Minute), d.Handlers.Auth.UnlockPasskeyFinish)

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

	// 审计日志：操作日志与登录日志（导出走 /export-tasks 异步任务）。
	secured.GET("/logs/operations", middleware.Require("system:log:operation"), d.Handlers.Log.Operations)
	secured.DELETE("/logs/operations", middleware.Require("system:log:operation"), d.Handlers.Log.DeleteOperations)
	secured.DELETE("/logs/operations/clear", middleware.Require("system:log:operation"), d.Handlers.Log.ClearOperations)
	secured.GET("/logs/logins", middleware.Require("system:log:login"), d.Handlers.Log.Logins)
	secured.DELETE("/logs/logins/clear", middleware.Require("system:log:login"), d.Handlers.Log.ClearLogins)

	// 在线会话：管理员查看全部登录设备并强制下线指定设备或用户。
	secured.GET("/online-sessions", middleware.Require("system:online:list"), d.Handlers.Online.List)
	secured.DELETE("/online-sessions/:userID", middleware.Require("system:online:kick"), d.Handlers.Online.Revoke)

	// 系统监控：主机、Go 运行时、数据库连接池与缓存探测。
	secured.GET("/monitor/overview", middleware.Require("system:monitor:view"), d.Handlers.Monitor.Overview)

	// 异步导出：提交任务、查看记录与进度、取消、删除与下载产物。
	secured.GET("/export-tasks", middleware.Require("system:export:list"), d.Handlers.Export.List)
	secured.POST("/export-tasks", middleware.Require("system:export:submit"), d.Handlers.Export.Submit)
	// 文件名模板路由是静态段，gin 会优先于 :id 参数路由匹配。
	secured.GET("/export-tasks/filename-template", middleware.Require("system:export:list"), d.Handlers.Export.FilenameTemplate)
	secured.PUT("/export-tasks/filename-template", middleware.Require("system:export:config"), d.Handlers.Export.UpdateFilenameTemplate)
	secured.POST("/export-tasks/:id/cancel", middleware.Require("system:export:cancel"), d.Handlers.Export.Cancel)
	secured.DELETE("/export-tasks/:id", middleware.Require("system:export:delete"), d.Handlers.Export.Delete)
	secured.GET("/export-tasks/:id/download", middleware.Require("system:export:download"), d.Handlers.Export.Download)

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
