package integration_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/tests/testsupport"
)

// TestMalformedRequests 覆盖各写接口的参数绑定失败分支，
// 确保非法 JSON / 非法路径 ID 都返回业务错误而不是 500。
func TestMalformedRequests(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	jsonEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/admin/v1/users"},
		{http.MethodPut, "/admin/v1/users/1"},
		{http.MethodPut, "/admin/v1/users/1/status"},
		{http.MethodPut, "/admin/v1/users/1/password"},
		{http.MethodPut, "/admin/v1/users/1/roles"},
		{http.MethodDelete, "/admin/v1/users"},
		{http.MethodPost, "/admin/v1/roles"},
		{http.MethodPut, "/admin/v1/roles/1"},
		{http.MethodPut, "/admin/v1/roles/1/menus"},
		{http.MethodPost, "/admin/v1/menus"},
		{http.MethodPut, "/admin/v1/menus/1"},
		{http.MethodPost, "/admin/v1/departments"},
		{http.MethodPut, "/admin/v1/departments/1"},
		{http.MethodPost, "/admin/v1/dict/types"},
		{http.MethodPut, "/admin/v1/dict/types/1"},
		{http.MethodPost, "/admin/v1/dict/types/1/items"},
		{http.MethodPut, "/admin/v1/dict/items/1"},
		{http.MethodPost, "/admin/v1/parameters"},
		{http.MethodPut, "/admin/v1/parameters/1"},
		{http.MethodPost, "/admin/v1/jobs"},
		{http.MethodPut, "/admin/v1/jobs/1"},
		{http.MethodPut, "/admin/v1/jobs/1/status"},
		{http.MethodPut, "/admin/v1/profile"},
		{http.MethodPut, "/admin/v1/profile/password"},
		{http.MethodDelete, "/admin/v1/profile/totp"},
		{http.MethodPost, "/admin/v1/profile/passkeys/finish"},
	}
	for _, endpoint := range jsonEndpoints {
		rr := app.Do(endpoint.method, endpoint.path, []byte("{not-json"), pair.AccessToken)
		env := testsupport.Decode(t, rr)
		assert.NotZero(t, env.Code, "%s %s should reject malformed JSON", endpoint.method, endpoint.path)
		assert.NotEqual(t, 500, rr.Code, "%s %s should not return 500", endpoint.method, endpoint.path)
	}

	idEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/v1/users/abc"},
		{http.MethodGet, "/admin/v1/roles/abc/menus"},
		{http.MethodGet, "/admin/v1/dict/types/abc/items"},
		{http.MethodGet, "/admin/v1/files/abc/download"},
		{http.MethodPost, "/admin/v1/files/abc/shares"},
		{http.MethodGet, "/admin/v1/files/abc/shares"},
		{http.MethodDelete, "/admin/v1/files/abc/shares/xyz"},
		{http.MethodGet, "/admin/v1/jobs/abc/runs"},
		{http.MethodDelete, "/admin/v1/departments/abc"},
		{http.MethodDelete, "/admin/v1/parameters/abc"},
		{http.MethodDelete, "/admin/v1/jobs/abc"},
		{http.MethodPut, "/admin/v1/notifications/abc/read"},
		{http.MethodPut, "/admin/v1/profile/passkeys/abc"},
		{http.MethodDelete, "/admin/v1/profile/passkeys/abc"},
	}
	for _, endpoint := range idEndpoints {
		rr := app.Do(endpoint.method, endpoint.path, nil, pair.AccessToken)
		env := testsupport.Decode(t, rr)
		assert.NotZero(t, env.Code, "%s %s should reject invalid ID", endpoint.method, endpoint.path)
		require.NotEqual(t, 500, rr.Code)
	}
}
