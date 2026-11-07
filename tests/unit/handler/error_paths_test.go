package handler_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"x-admin/internal/pkg/excel"
	"x-admin/tests/testsupport"
)

// TestHandlerServiceErrorBranches 通过不存在 ID / 非法参数触发各处理器的
// "service 报错 → response.Error" 分支。
func TestHandlerServiceErrorBranches(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")
	token := pair.AccessToken

	expectCode := func(method, path string, body any) int {
		return app.Do(method, path, body, token).Code
	}

	// 字典：更新/删除不存在的类型与字典项、非法请求体。
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodPut, "/admin/v1/dict/types/1", "not-json"))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/dict/types/404",
		map[string]any{"name": "x", "code": "x", "status": 1}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodDelete, "/admin/v1/dict/types/404", nil))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/dict/items/404",
		map[string]any{"label": "x", "value": "x"}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodDelete, "/admin/v1/dict/items/404", nil))

	// 角色、菜单、参数、通知、任务。
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/roles/404",
		map[string]any{"name": "x", "code": "x", "status": 1}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodDelete, "/admin/v1/roles/404", nil))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/menus/404",
		map[string]any{"name": "x", "type": 1, "status": 1}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/parameters/404",
		map[string]any{"key": "x", "name": "x", "value": "x"}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodDelete, "/admin/v1/parameters/404", nil))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/notifications/404/read", nil))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPost, "/admin/v1/jobs/404/run", nil))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/jobs/404",
		map[string]any{"name": "x", "cron": "* * * * *", "handler": "noop"}))

	// 文件：上传缺文件、分享/撤销/删除不存在的文件。
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodPost, "/admin/v1/files", nil))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPost, "/admin/v1/files/404/shares",
		map[string]any{"expires": 3600}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodDelete, "/admin/v1/files/404/shares/404", nil))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodDelete, "/admin/v1/files/404", nil))
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodGet, "/admin/v1/files/abc/download", nil))

	// 日志导出与查询参数非法。
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodGet, "/admin/v1/logs/operations?page=abc", nil))
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodGet, "/admin/v1/logs/logins?page=abc", nil))
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodGet, "/admin/v1/users/export?page=abc", nil))
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodGet, "/admin/v1/files?page=abc", nil))

	// 个人中心：非法邮箱、错误旧密码、不存在的会话与 Passkey、TOTP 错误密码。
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodPut, "/admin/v1/profile",
		map[string]any{"email": "bad"}))
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodPut, "/admin/v1/profile/password",
		map[string]any{"old_password": "wrong", "new_password": "NewPass@123"}))
	// 撤销不存在的会话是幂等成功。
	assert.Equal(t, http.StatusOK, expectCode(http.MethodDelete, "/admin/v1/profile/sessions/404", nil))
	assert.Equal(t, http.StatusBadRequest, expectCode(http.MethodPost, "/admin/v1/profile/passkeys/begin",
		map[string]any{"password": "wrong"}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodPut, "/admin/v1/profile/passkeys/404",
		map[string]any{"name": "新名字"}))
	assert.Equal(t, http.StatusNotFound, expectCode(http.MethodDelete, "/admin/v1/profile/passkeys/404", nil))

	// TOTP 未开启时查询/停用直接冲突或错误。
	_ = expectCode(http.MethodGet, "/admin/v1/profile/totp", nil)
	_ = expectCode(http.MethodDelete, "/admin/v1/profile/totp",
		map[string]any{"password": "Admin@123456", "code": "000000"})

	// 用户导入：缺少文件、内容不是 Excel、只有表头。
	assert.Equal(t, http.StatusBadRequest, importUsers(t, app, token, nil, "nofile"))
	assert.Equal(t, http.StatusBadRequest, importUsers(t, app, token, []byte("plain text"), "plain.xlsx"))
	headerOnly, err := excel.Write("", []string{"用户名"}, nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, importUsers(t, app, token, headerOnly, "empty.xlsx"))
}

// importUsers 构造 multipart 导入请求。
func importUsers(t *testing.T, app *testsupport.App, token string, content []byte, filename string) int {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if filename != "" {
		fw, err := w.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	w.Close()

	req, _ := http.NewRequest(http.MethodPost, "/admin/v1/users/import", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rr := app.DoRaw(req)
	return rr.Code
}
