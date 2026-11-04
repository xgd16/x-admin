package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"x-admin/tests/testsupport"
)

// TestNotFoundBranches 覆盖各 handler 在服务层返回 404 时的错误分支。
func TestNotFoundBranches(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	cases := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/admin/v1/users/999", nil},
		{http.MethodPut, "/admin/v1/users/999", map[string]any{"nickname": "x", "status": 1}},
		{http.MethodPut, "/admin/v1/users/999/status", map[string]any{"status": 1}},
		{http.MethodPut, "/admin/v1/users/999/password", map[string]any{"password": "Passw0rd123"}},
		{http.MethodPut, "/admin/v1/users/999/roles", map[string]any{"role_ids": []uint64{}}},
		{http.MethodGet, "/admin/v1/roles/999/menus", nil},
		{http.MethodPut, "/admin/v1/roles/999", map[string]any{"name": "x", "code": "x_role", "status": 1}},
		{http.MethodPut, "/admin/v1/roles/999/menus", map[string]any{"menu_ids": []uint64{}}},
		{http.MethodDelete, "/admin/v1/roles/999", nil},
		{http.MethodPut, "/admin/v1/menus/999", map[string]any{"name": "x", "type": 2, "status": 1}},
		{http.MethodDelete, "/admin/v1/menus/999", nil},
		{http.MethodPut, "/admin/v1/departments/999", map[string]any{"name": "x", "status": 1}},
		{http.MethodDelete, "/admin/v1/departments/999", nil},
		{http.MethodPut, "/admin/v1/dict/types/999", map[string]any{"name": "x", "code": "x_dict", "status": 1}},
		{http.MethodDelete, "/admin/v1/dict/types/999", nil},
		{http.MethodGet, "/admin/v1/dict/types/999/items", nil},
		{http.MethodPut, "/admin/v1/dict/items/999", map[string]any{"label": "x", "value": "1"}},
		{http.MethodDelete, "/admin/v1/dict/items/999", nil},
		{http.MethodPut, "/admin/v1/parameters/999", map[string]any{"key": "x.key", "name": "x", "value": "1"}},
		{http.MethodDelete, "/admin/v1/parameters/999", nil},
		{http.MethodDelete, "/admin/v1/files/999", nil},
		{http.MethodPut, "/admin/v1/jobs/999", map[string]any{"name": "x", "cron": "0 1 * * *", "handler": "noop"}},
		{http.MethodDelete, "/admin/v1/jobs/999", nil},
		{http.MethodPost, "/admin/v1/jobs/999/run", nil},
		{http.MethodGet, "/admin/v1/jobs/999/runs", nil},
		{http.MethodPut, "/admin/v1/profile/passkeys/999", map[string]any{"name": "x"}},
		{http.MethodDelete, "/admin/v1/profile/passkeys/999", nil},
	}
	for _, tc := range cases {
		rr := app.Do(tc.method, tc.path, tc.body, pair.AccessToken)
		env := testsupport.Decode(t, rr)
		assert.NotZero(t, env.Code, "%s %s", tc.method, tc.path)
		assert.NotEqual(t, 500, rr.Code, "%s %s", tc.method, tc.path)
	}
}
