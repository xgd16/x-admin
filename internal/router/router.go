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

type Handlers struct {
	Auth       *handler.AuthHandler
	Dashboard  *handler.DashboardHandler
	User       *handler.UserHandler
	Role       *handler.RoleHandler
	Menu       *handler.MenuHandler
	Department *handler.DepartmentHandler
	Dict       *handler.DictHandler
	Parameter  *handler.ParameterHandler
	File       *handler.FileHandler
	Job        *handler.JobHandler
	Log        *handler.LogHandler
	Profile    *handler.ProfileHandler
}

type Deps struct {
	Config   config.Config
	Tokens   *security.TokenManager
	Auth     *service.AuthService
	Captcha  *captcha.Generator
	Cache    cache.Store
	Recorder middleware.OperationRecorder
	Handlers Handlers
	Health   func(ctx context.Context) error
}

func New(d Deps) *gin.Engine {
	if d.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Recovery(), middleware.AccessLog(), middleware.CORS(d.Config.CORS))
	r.Use(middleware.Audit(d.Recorder))

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
	r.Static("/uploads", d.Config.Storage.Root)

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
	api.Use(middleware.RateLimit(d.Cache, "rl:global", 100, time.Minute))

	auth := api.Group("/auth")
	auth.GET("/captcha", d.Handlers.Auth.Captcha)
	auth.POST("/login", middleware.RateLimit(d.Cache, "rl:login", 10, time.Minute), d.Handlers.Auth.Login)
	auth.POST("/refresh", d.Handlers.Auth.Refresh)

	secured := api.Group("")
	secured.Use(middleware.Auth(d.Tokens, d.Auth))
	{
		secured.POST("/auth/logout", d.Handlers.Auth.Logout)
		secured.GET("/auth/me", d.Handlers.Auth.Me)

		secured.GET("/dashboard/summary", middleware.Require("dashboard:view"), d.Handlers.Dashboard.Summary)

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

		secured.GET("/roles", middleware.Require("system:role:list"), d.Handlers.Role.List)
		secured.GET("/roles/all", middleware.Require("system:role:list"), d.Handlers.Role.All)
		secured.POST("/roles", middleware.Require("system:role:create"), d.Handlers.Role.Create)
		secured.PUT("/roles/:id", middleware.Require("system:role:update"), d.Handlers.Role.Update)
		secured.DELETE("/roles/:id", middleware.Require("system:role:delete"), d.Handlers.Role.Delete)
		secured.GET("/roles/:id/menus", middleware.Require("system:role:list"), d.Handlers.Role.Menus)
		secured.PUT("/roles/:id/menus", middleware.Require("system:role:assign"), d.Handlers.Role.AssignMenus)

		secured.GET("/menus/tree", middleware.Require("system:menu:list"), d.Handlers.Menu.Tree)
		secured.POST("/menus", middleware.Require("system:menu:create"), d.Handlers.Menu.Create)
		secured.PUT("/menus/:id", middleware.Require("system:menu:update"), d.Handlers.Menu.Update)
		secured.DELETE("/menus/:id", middleware.Require("system:menu:delete"), d.Handlers.Menu.Delete)

		secured.GET("/departments/tree", middleware.Require("system:dept:list"), d.Handlers.Department.Tree)
		secured.POST("/departments", middleware.Require("system:dept:create"), d.Handlers.Department.Create)
		secured.PUT("/departments/:id", middleware.Require("system:dept:update"), d.Handlers.Department.Update)
		secured.DELETE("/departments/:id", middleware.Require("system:dept:delete"), d.Handlers.Department.Delete)

		secured.GET("/dict/types", middleware.Require("system:dict:list"), d.Handlers.Dict.ListTypes)
		secured.POST("/dict/types", middleware.Require("system:dict:create"), d.Handlers.Dict.CreateType)
		secured.PUT("/dict/types/:id", middleware.Require("system:dict:update"), d.Handlers.Dict.UpdateType)
		secured.DELETE("/dict/types/:id", middleware.Require("system:dict:delete"), d.Handlers.Dict.DeleteType)
		secured.GET("/dict/types/:id/items", middleware.Require("system:dict:list"), d.Handlers.Dict.Items)
		secured.POST("/dict/types/:id/items", middleware.Require("system:dict:create"), d.Handlers.Dict.CreateItem)
		secured.PUT("/dict/items/:id", middleware.Require("system:dict:update"), d.Handlers.Dict.UpdateItem)
		secured.DELETE("/dict/items/:id", middleware.Require("system:dict:delete"), d.Handlers.Dict.DeleteItem)

		secured.GET("/parameters", middleware.Require("system:param:list"), d.Handlers.Parameter.List)
		secured.POST("/parameters", middleware.Require("system:param:create"), d.Handlers.Parameter.Create)
		secured.PUT("/parameters/:id", middleware.Require("system:param:update"), d.Handlers.Parameter.Update)
		secured.DELETE("/parameters/:id", middleware.Require("system:param:delete"), d.Handlers.Parameter.Delete)

		secured.GET("/files", middleware.Require("system:file:list"), d.Handlers.File.List)
		secured.POST("/files", middleware.Require("system:file:upload"), d.Handlers.File.Upload)
		secured.GET("/files/:id/download", middleware.Require("system:file:download"), d.Handlers.File.Download)
		secured.DELETE("/files/:id", middleware.Require("system:file:delete"), d.Handlers.File.Delete)

		secured.GET("/jobs", middleware.Require("system:job:list"), d.Handlers.Job.List)
		secured.POST("/jobs", middleware.Require("system:job:create"), d.Handlers.Job.Create)
		secured.PUT("/jobs/:id", middleware.Require("system:job:update"), d.Handlers.Job.Update)
		secured.DELETE("/jobs/:id", middleware.Require("system:job:delete"), d.Handlers.Job.Delete)
		secured.PUT("/jobs/:id/status", middleware.Require("system:job:update"), d.Handlers.Job.SetStatus)
		secured.POST("/jobs/:id/run", middleware.Require("system:job:run"), d.Handlers.Job.Run)

		secured.GET("/logs/operations", middleware.Require("system:log:operation"), d.Handlers.Log.Operations)
		secured.DELETE("/logs/operations", middleware.Require("system:log:operation"), d.Handlers.Log.DeleteOperations)
		secured.DELETE("/logs/operations/clear", middleware.Require("system:log:operation"), d.Handlers.Log.ClearOperations)
		secured.GET("/logs/logins", middleware.Require("system:log:login"), d.Handlers.Log.Logins)
		secured.DELETE("/logs/logins/clear", middleware.Require("system:log:login"), d.Handlers.Log.ClearLogins)

		secured.GET("/profile", d.Handlers.Profile.Get)
		secured.PUT("/profile", d.Handlers.Profile.Update)
		secured.PUT("/profile/password", d.Handlers.Profile.ChangePassword)
	}

	slog.Info("routes registered", "prefix", "/admin/v1")
	return r
}
