package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/handler"
	"x-admin/internal/pkg/authn"
	"x-admin/tests/testsupport"
)

func newFailingRouter(app *testsupport.App) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(authn.With(c.Request.Context(), &authn.Principal{
			UserID: 1, Username: "admin", Super: true,
		}))
		c.Next()
	})

	users := handler.NewUserHandler(app.Services.Users)
	roles := handler.NewRoleHandler(app.Services.Roles)
	menus := handler.NewMenuHandler(app.Services.Menus)
	depts := handler.NewDepartmentHandler(app.Services.Depts)
	dicts := handler.NewDictHandler(app.Services.Dicts)
	params := handler.NewParameterHandler(app.Services.Params)
	files := handler.NewFileHandler(app.Services.Files)
	jobs := handler.NewJobHandler(app.Services.Jobs)
	logs := handler.NewLogHandler(app.Services.Logs)
	profile := handler.NewProfileHandler(app.Services.Profile, app.Services.Passkeys, app.Services.Auth)
	notifications := handler.NewNotificationHandler(app.Services.Notifications)
	exports := handler.NewExportHandler(app.Services.Exports)
	dashboard := handler.NewDashboardHandler(app.Services.Dashboard)
	auth := handler.NewAuthHandler(app.Services.Auth, app.Captcha, app.Services.Passkeys, app.Tokens, false, time.Hour)

	r.GET("/auth/captcha", auth.Captcha)
	r.POST("/auth/login", auth.Login)
	r.POST("/auth/login/mfa", auth.LoginMFA)
	r.POST("/auth/passkey/login/begin", auth.PasskeyLoginBegin)
	r.POST("/auth/passkey/login/finish", auth.PasskeyLoginFinish)
	r.POST("/auth/refresh", auth.Refresh)
	r.POST("/auth/logout", auth.Logout)
	r.GET("/auth/me", auth.Me)

	r.GET("/users", users.List)
	r.GET("/users/:id", users.Get)
	r.POST("/users", users.Create)
	r.PUT("/users/:id", users.Update)
	r.DELETE("/users", users.Delete)
	r.PUT("/users/:id/status", users.SetStatus)
	r.PUT("/users/:id/password", users.ResetPassword)
	r.PUT("/users/:id/roles", users.AssignRoles)

	r.GET("/roles", roles.List)
	r.GET("/roles/all", roles.All)
	r.POST("/roles", roles.Create)
	r.PUT("/roles/:id", roles.Update)
	r.DELETE("/roles/:id", roles.Delete)
	r.GET("/roles/:id/menus", roles.Menus)
	r.PUT("/roles/:id/menus", roles.AssignMenus)

	r.GET("/menus/tree", menus.Tree)
	r.POST("/menus", menus.Create)
	r.PUT("/menus/:id", menus.Update)
	r.DELETE("/menus/:id", menus.Delete)
	r.GET("/departments/tree", depts.Tree)
	r.POST("/departments", depts.Create)
	r.PUT("/departments/:id", depts.Update)
	r.DELETE("/departments/:id", depts.Delete)

	r.GET("/dict/types", dicts.ListTypes)
	r.POST("/dict/types", dicts.CreateType)
	r.PUT("/dict/types/:id", dicts.UpdateType)
	r.DELETE("/dict/types/:id", dicts.DeleteType)
	r.GET("/dict/types/:id/items", dicts.Items)
	r.POST("/dict/types/:id/items", dicts.CreateItem)
	r.PUT("/dict/items/:id", dicts.UpdateItem)
	r.DELETE("/dict/items/:id", dicts.DeleteItem)

	r.GET("/parameters", params.List)
	r.POST("/parameters", params.Create)
	r.PUT("/parameters/:id", params.Update)
	r.DELETE("/parameters/:id", params.Delete)

	r.GET("/files", files.List)
	r.GET("/files/:id/download", files.Download)
	r.GET("/files/:id/share", files.CreateShare)
	r.GET("/files/:id/shares", files.ListShares)
	r.DELETE("/files/:id/shares/:shareID", files.RevokeShare)
	r.DELETE("/files/:id", files.Delete)

	r.GET("/jobs", jobs.List)
	r.POST("/jobs", jobs.Create)
	r.PUT("/jobs/:id", jobs.Update)
	r.PUT("/jobs/:id/status", jobs.SetStatus)
	r.POST("/jobs/:id/run", jobs.Run)
	r.GET("/jobs/:id/runs", jobs.Runs)
	r.DELETE("/jobs/:id", jobs.Delete)

	r.GET("/logs/operations", logs.Operations)
	r.DELETE("/logs/operations", logs.DeleteOperations)
	r.DELETE("/logs/operations/clear", logs.ClearOperations)
	r.GET("/logs/logins", logs.Logins)
	r.DELETE("/logs/logins/clear", logs.ClearLogins)

	r.GET("/export-tasks", exports.List)
	r.POST("/export-tasks", exports.Submit)
	r.GET("/export-tasks/filename-template", exports.FilenameTemplate)
	r.PUT("/export-tasks/filename-template", exports.UpdateFilenameTemplate)
	r.POST("/export-tasks/:id/cancel", exports.Cancel)
	r.DELETE("/export-tasks/:id", exports.Delete)
	r.GET("/export-tasks/:id/download", exports.Download)

	r.GET("/profile", profile.Get)
	r.PUT("/profile", profile.Update)
	r.PUT("/profile/password", profile.ChangePassword)
	r.GET("/profile/totp", profile.TOTPStatus)
	r.POST("/profile/totp/setup", profile.SetupTOTP)
	r.GET("/profile/passkeys", profile.ListPasskeys)
	r.GET("/profile/sessions", profile.Sessions)

	r.GET("/notifications", notifications.List)
	r.GET("/notifications/unread-count", notifications.UnreadCount)
	r.PUT("/notifications/read-all", notifications.MarkAllRead)
	r.PUT("/notifications/:id/read", notifications.MarkRead)

	r.GET("/dashboard", dashboard.Summary)
	return r
}

func TestHandlersReturnInternalErrorWhenDatabaseFails(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	router := newFailingRouter(app)
	challenge, err := app.Captcha.Generate(context.Background())
	require.NoError(t, err)
	captchaCode, err := app.Cache.Get(context.Background(), "captcha:"+challenge.ID)
	require.NoError(t, err)

	sqlDB, err := app.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/users", nil},
		{http.MethodGet, "/users/1", nil},
		{http.MethodPost, "/users", map[string]any{"username": "new_user", "password": "Passw0rd123"}},
		{http.MethodPut, "/users/1", map[string]any{"nickname": "x", "status": 1}},
		{http.MethodDelete, "/users", map[string]any{"ids": []uint64{1}}},
		{http.MethodPut, "/users/1/status", map[string]any{"status": 1}},
		{http.MethodPut, "/users/1/password", map[string]any{"password": "Passw0rd123"}},
		{http.MethodPut, "/users/1/roles", map[string]any{"role_ids": []uint64{1}}},
		{http.MethodGet, "/export-tasks", nil},
		{http.MethodPost, "/export-tasks", map[string]any{"module": "users"}},
		{http.MethodGet, "/export-tasks/filename-template", nil},
		{http.MethodPut, "/export-tasks/filename-template", map[string]any{"segments": []any{map[string]any{"type": "module"}}}},
		{http.MethodGet, "/roles", nil},
		{http.MethodGet, "/roles/all", nil},
		{http.MethodPost, "/roles", map[string]any{"name": "x", "code": "x_role", "status": 1}},
		{http.MethodPut, "/roles/1", map[string]any{"name": "x", "code": "x_role", "status": 1}},
		{http.MethodDelete, "/roles/1", nil},
		{http.MethodGet, "/roles/1/menus", nil},
		{http.MethodPut, "/roles/1/menus", map[string]any{"menu_ids": []uint64{1}}},
		{http.MethodGet, "/menus/tree", nil},
		{http.MethodPost, "/menus", map[string]any{"name": "x", "type": 2, "path": "/x", "component": "x", "permission": "x:list", "status": 1}},
		{http.MethodPut, "/menus/1", map[string]any{"name": "x", "type": 2, "status": 1}},
		{http.MethodDelete, "/menus/1", nil},
		{http.MethodGet, "/departments/tree", nil},
		{http.MethodPost, "/departments", map[string]any{"name": "x", "status": 1}},
		{http.MethodPut, "/departments/1", map[string]any{"name": "x", "status": 1}},
		{http.MethodDelete, "/departments/1", nil},
		{http.MethodGet, "/dict/types", nil},
		{http.MethodPost, "/dict/types", map[string]any{"name": "x", "code": "x_dict", "status": 1}},
		{http.MethodPut, "/dict/types/1", map[string]any{"name": "x", "code": "x_dict", "status": 1}},
		{http.MethodDelete, "/dict/types/1", nil},
		{http.MethodGet, "/dict/types/1/items", nil},
		{http.MethodPost, "/dict/types/1/items", map[string]any{"dict_type_id": 1, "label": "x", "value": "1"}},
		{http.MethodPut, "/dict/items/1", map[string]any{"label": "x", "value": "1"}},
		{http.MethodDelete, "/dict/items/1", nil},
		{http.MethodGet, "/parameters", nil},
		{http.MethodPost, "/parameters", map[string]any{"key": "x.key", "name": "x", "value": "1"}},
		{http.MethodPut, "/parameters/1", map[string]any{"key": "x.key", "name": "x", "value": "1"}},
		{http.MethodDelete, "/parameters/1", nil},
		{http.MethodGet, "/files", nil},
		{http.MethodGet, "/files/1/download", nil},
		{http.MethodGet, "/files/1/share", nil},
		{http.MethodGet, "/files/1/shares", nil},
		{http.MethodDelete, "/files/1/shares/1", nil},
		{http.MethodDelete, "/files/1", nil},
		{http.MethodGet, "/jobs", nil},
		{http.MethodPost, "/jobs", map[string]any{"name": "x", "cron": "0 1 * * *", "handler": "noop"}},
		{http.MethodPut, "/jobs/1", map[string]any{"name": "x", "cron": "0 1 * * *", "handler": "noop"}},
		{http.MethodPut, "/jobs/1/status", map[string]any{"status": 1}},
		{http.MethodPost, "/jobs/1/run", nil},
		{http.MethodGet, "/jobs/1/runs", nil},
		{http.MethodDelete, "/jobs/1", nil},
		{http.MethodGet, "/logs/operations", nil},
		{http.MethodDelete, "/logs/operations", map[string]any{"ids": []uint64{1}}},
		{http.MethodDelete, "/logs/operations/clear", nil},
		{http.MethodGet, "/logs/logins", nil},
		{http.MethodDelete, "/logs/logins/clear", nil},
		{http.MethodPost, "/export-tasks/1/cancel", nil},
		{http.MethodDelete, "/export-tasks/1", nil},
		{http.MethodGet, "/export-tasks/1/download", nil},
		{http.MethodGet, "/profile", nil},
		{http.MethodPut, "/profile", map[string]any{"nickname": "x"}},
		{http.MethodPut, "/profile/password", map[string]any{"old_password": "x", "new_password": "Passw0rd123"}},
		{http.MethodGet, "/profile/totp", nil},
		{http.MethodPost, "/profile/totp/setup", nil},
		{http.MethodGet, "/profile/passkeys", nil},
		{http.MethodGet, "/profile/sessions", nil},
		{http.MethodGet, "/notifications", nil},
		{http.MethodGet, "/notifications/unread-count", nil},
		{http.MethodPut, "/notifications/read-all", nil},
		{http.MethodPut, "/notifications/1/read", nil},
		{http.MethodGet, "/dashboard", nil},
		{http.MethodGet, "/auth/me", nil},
		{http.MethodPost, "/auth/login", map[string]any{
			"username": "admin", "password": "Admin@123456",
			"captcha_id": challenge.ID, "captcha_code": captchaCode,
		}},
		{http.MethodPost, "/auth/login/mfa", map[string]any{"mfa_token": "mfa-db-fail", "otp_code": "123456"}},
		{http.MethodPost, "/auth/passkey/login/begin", nil},
	}
	require.NoError(t, app.Cache.Set(context.Background(), "mfa:challenge:mfa-db-fail", "1", time.Minute))
	for _, tc := range cases {
		var reader *bytes.Reader
		if tc.body == nil {
			reader = bytes.NewReader(nil)
		} else {
			raw, _ := json.Marshal(tc.body)
			reader = bytes.NewReader(raw)
		}
		req := httptest.NewRequest(tc.method, tc.path, reader)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code, "%s %s: %s", tc.method, tc.path, rec.Body.String())
	}
}
