package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/tests/testsupport"
)

// TestAPIHandlerErrors drives the validation branches of every handler with
// malformed IDs, bodies and queries. Super admin bypasses permissions, so the
// expected failures come from binding/service validation rather than RBAC.
func TestAPIHandlerErrors(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		status int
	}{
		{"user list bad query", http.MethodGet, "/admin/v1/users?page=abc", nil, http.StatusBadRequest},
		{"user get bad id", http.MethodGet, "/admin/v1/users/abc", nil, http.StatusBadRequest},
		{"user get missing", http.MethodGet, "/admin/v1/users/999999", nil, http.StatusNotFound},
		{"user create bad json", http.MethodPost, "/admin/v1/users", "{", http.StatusBadRequest},
		{"user update bad id", http.MethodPut, "/admin/v1/users/abc", map[string]any{}, http.StatusBadRequest},
		{"user update bad json", http.MethodPut, "/admin/v1/users/1", "{", http.StatusBadRequest},
		{"user update missing", http.MethodPut, "/admin/v1/users/999999", map[string]any{"nickname": "x"}, http.StatusNotFound},
		{"user delete empty", http.MethodDelete, "/admin/v1/users", map[string]any{}, http.StatusBadRequest},
		{"user status bad body", http.MethodPut, "/admin/v1/users/1/status", map[string]any{"status": 9}, http.StatusBadRequest},
		{"user status bad id", http.MethodPut, "/admin/v1/users/abc/status", map[string]any{"status": 1}, http.StatusBadRequest},
		{"user password bad body", http.MethodPut, "/admin/v1/users/1/password", map[string]any{"password": "x"}, http.StatusBadRequest},
		{"user roles bad id", http.MethodPut, "/admin/v1/users/abc/roles", map[string]any{"role_ids": []int{}}, http.StatusBadRequest},
		{"user import missing file", http.MethodPost, "/admin/v1/users/import", nil, http.StatusBadRequest},
		{"user export bad query", http.MethodGet, "/admin/v1/users/export?page=abc", nil, http.StatusBadRequest},

		{"role list bad query", http.MethodGet, "/admin/v1/roles?page=abc", nil, http.StatusBadRequest},
		{"role create bad body", http.MethodPost, "/admin/v1/roles", map[string]any{"name": "", "code": "Bad", "status": 9}, http.StatusBadRequest},
		{"role update bad id", http.MethodPut, "/admin/v1/roles/abc", map[string]any{}, http.StatusBadRequest},
		{"role delete missing", http.MethodDelete, "/admin/v1/roles/999999", nil, http.StatusNotFound},
		{"role menus bad id", http.MethodGet, "/admin/v1/roles/abc/menus", nil, http.StatusBadRequest},
		{"role assign bad body", http.MethodPut, "/admin/v1/roles/1/menus", "{", http.StatusBadRequest},

		{"menu create bad body", http.MethodPost, "/admin/v1/menus", map[string]any{"name": "", "type": 0}, http.StatusBadRequest},
		{"menu update bad id", http.MethodPut, "/admin/v1/menus/abc", map[string]any{}, http.StatusBadRequest},
		{"menu delete missing", http.MethodDelete, "/admin/v1/menus/999999", nil, http.StatusNotFound},

		{"dept create bad body", http.MethodPost, "/admin/v1/departments", map[string]any{"name": "", "status": 9}, http.StatusBadRequest},
		{"dept update bad id", http.MethodPut, "/admin/v1/departments/abc", map[string]any{}, http.StatusBadRequest},
		{"dept delete missing", http.MethodDelete, "/admin/v1/departments/999999", nil, http.StatusNotFound},

		{"dict types bad query", http.MethodGet, "/admin/v1/dict/types?page=abc", nil, http.StatusBadRequest},
		{"dict type create bad body", http.MethodPost, "/admin/v1/dict/types", map[string]any{"name": "", "code": "Bad"}, http.StatusBadRequest},
		{"dict type update bad id", http.MethodPut, "/admin/v1/dict/types/abc", map[string]any{}, http.StatusBadRequest},
		{"dict items bad id", http.MethodGet, "/admin/v1/dict/types/abc/items", nil, http.StatusBadRequest},
		{"dict item create bad body", http.MethodPost, "/admin/v1/dict/types/1/items", map[string]any{"label": ""}, http.StatusBadRequest},
		{"dict item update bad id", http.MethodPut, "/admin/v1/dict/items/abc", map[string]any{}, http.StatusBadRequest},
		{"dict item delete missing", http.MethodDelete, "/admin/v1/dict/items/999999", nil, http.StatusNotFound},

		{"param list bad query", http.MethodGet, "/admin/v1/parameters?page=abc", nil, http.StatusBadRequest},
		{"param create bad body", http.MethodPost, "/admin/v1/parameters", map[string]any{"key": "1bad", "name": ""}, http.StatusBadRequest},
		{"param update bad id", http.MethodPut, "/admin/v1/parameters/abc", map[string]any{}, http.StatusBadRequest},
		{"param delete missing", http.MethodDelete, "/admin/v1/parameters/999999", nil, http.StatusNotFound},

		{"file list bad query", http.MethodGet, "/admin/v1/files?page=abc", nil, http.StatusBadRequest},
		{"file upload missing", http.MethodPost, "/admin/v1/files", nil, http.StatusBadRequest},
		{"file download bad id", http.MethodGet, "/admin/v1/files/abc/download", nil, http.StatusBadRequest},
		{"file download missing", http.MethodGet, "/admin/v1/files/999999/download", nil, http.StatusNotFound},
		{"file delete missing", http.MethodDelete, "/admin/v1/files/999999", nil, http.StatusNotFound},

		{"job list bad query", http.MethodGet, "/admin/v1/jobs?page=abc", nil, http.StatusBadRequest},
		{"job create bad body", http.MethodPost, "/admin/v1/jobs", map[string]any{"name": ""}, http.StatusBadRequest},
		{"job update bad id", http.MethodPut, "/admin/v1/jobs/abc", map[string]any{}, http.StatusBadRequest},
		{"job delete missing", http.MethodDelete, "/admin/v1/jobs/999999", nil, http.StatusNotFound},
		{"job status bad body", http.MethodPut, "/admin/v1/jobs/999999/status", map[string]any{"status": 9}, http.StatusBadRequest},
		{"job run bad id", http.MethodPost, "/admin/v1/jobs/abc/run", nil, http.StatusBadRequest},

		{"operation logs bad query", http.MethodGet, "/admin/v1/logs/operations?page=abc", nil, http.StatusBadRequest},
		{"operation logs bad delete", http.MethodDelete, "/admin/v1/logs/operations", map[string]any{}, http.StatusBadRequest},
		{"login logs bad query", http.MethodGet, "/admin/v1/logs/logins?page=abc", nil, http.StatusBadRequest},

		{"profile bad update", http.MethodPut, "/admin/v1/profile", map[string]any{"email": "bad"}, http.StatusBadRequest},
		{"profile password bad body", http.MethodPut, "/admin/v1/profile/password", map[string]any{}, http.StatusBadRequest},
		{"refresh bad body", http.MethodPost, "/admin/v1/auth/refresh", nil, http.StatusBadRequest},
		{"logout bad body", http.MethodPost, "/admin/v1/auth/logout", "{", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := app.Do(tc.method, tc.path, tc.body, token)
			assert.Equalf(t, tc.status, rr.Code, "%s %s -> %s", tc.method, tc.path, rr.Body.String())
		})
	}

	// Missing token on a protected route.
	rr := app.Do(http.MethodGet, "/admin/v1/profile", nil, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	// Unknown route under the API.
	rr = app.Do(http.MethodGet, "/admin/v1/nope", nil, token)
	require.Equal(t, http.StatusNotFound, rr.Code)
}
