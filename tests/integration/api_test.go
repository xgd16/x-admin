package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/excel"
	"x-admin/tests/testsupport"
)

type dataEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func decodeData(t *testing.T, rr *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var env dataEnvelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env), rr.Body.String())
	require.Equal(t, 0, env.Code, rr.Body.String())
	return env.Data
}

func decodeID(t *testing.T, rr *httptest.ResponseRecorder) uint64 {
	t.Helper()
	var row struct {
		ID uint64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(decodeData(t, rr), &row))
	require.NotZero(t, row.ID)
	return row.ID
}

func TestAPIEndToEnd(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	loginPair := app.Login(t, "admin", "Admin@123456")
	token := loginPair.AccessToken

	// health + me + dashboard
	rr := httptest.NewRecorder()
	app.Router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = app.Do(http.MethodGet, "/swagger/index.html", nil, "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "SwaggerUIBundle")
	rr = app.Do(http.MethodGet, "/swagger/openapi.yaml", nil, "")
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "openapi: 3.0.3")

	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, token)
	me := decodeData(t, rr)
	assert.Contains(t, string(me), "admin")

	rr = app.Do(http.MethodPost, "/admin/v1/auth/refresh", map[string]any{"refresh_token": loginPair.RefreshToken}, "")
	assert.Contains(t, string(decodeData(t, rr)), "access_token")
	rr = app.Do(http.MethodPost, "/admin/v1/auth/refresh", map[string]any{"refresh_token": "expired"}, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	rr = app.Do(http.MethodGet, "/admin/v1/dashboard/summary", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "login_trend")

	// departments
	rr = app.Do(http.MethodPost, "/admin/v1/departments", map[string]any{"name": "技术部", "status": 1, "sort": 1}, token)
	deptID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/departments/tree", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "技术部")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/departments/%d", deptID), map[string]any{"name": "研发部", "status": 1}, token)
	assert.Contains(t, string(decodeData(t, rr)), "研发部")

	// menus
	rr = app.Do(http.MethodPost, "/admin/v1/menus", map[string]any{
		"name": "系统管理", "type": 1, "path": "/system", "status": 1, "visible": true, "sort": 1,
	}, token)
	menuID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/menus/tree", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "系统管理")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/menus/%d", menuID), map[string]any{
		"name": "系统", "type": 1, "path": "/system", "status": 1, "visible": true,
	}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)

	// roles
	rr = app.Do(http.MethodPost, "/admin/v1/roles", map[string]any{
		"name": "运营", "code": "operator", "status": 1, "menu_ids": []uint64{menuID},
	}, token)
	roleID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/roles?page=1&page_size=10", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "operator")
	rr = app.Do(http.MethodGet, "/admin/v1/roles/all", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "operator")
	rr = app.Do(http.MethodGet, fmt.Sprintf("/admin/v1/roles/%d/menus", roleID), nil, token)
	assert.Contains(t, string(decodeData(t, rr)), fmt.Sprint(menuID))
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/roles/%d/menus", roleID), map[string]any{"menu_ids": []uint64{menuID}}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/roles/%d", roleID), map[string]any{
		"name": "运营 2", "code": "operator", "status": 1,
	}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)

	// users
	rr = app.Do(http.MethodPost, "/admin/v1/users", map[string]any{
		"username": "alice", "password": "Passw0rd", "nickname": "Alice",
		"email": "alice@example.com", "phone": "13800000000", "dept_id": deptID,
		"status": 1, "role_ids": []uint64{roleID},
	}, token)
	userID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/users?page=1&page_size=10&keyword=alice", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "alice")
	rr = app.Do(http.MethodGet, fmt.Sprintf("/admin/v1/users/%d", userID), nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "Alice")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/users/%d", userID), map[string]any{
		"nickname": "Alice2", "email": "a2@example.com", "dept_id": deptID, "status": 1,
		"role_ids": []uint64{roleID},
	}, token)
	assert.Contains(t, string(decodeData(t, rr)), "Alice2")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/users/%d/status", userID), map[string]any{"status": 0}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/users/%d/password", userID), map[string]any{"password": "NewPass123"}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/users/%d/roles", userID), map[string]any{"role_ids": []uint64{roleID}}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodGet, "/admin/v1/users/export?keyword=alice", nil, token)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "users.xlsx")
	assert.NotEmpty(t, rr.Body.Bytes())

	rr = app.Do(http.MethodGet, "/admin/v1/users/template", nil, token)
	assert.Equal(t, http.StatusOK, rr.Code)

	importBody, contentType := userImportFile(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/users/import", importBody)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	app.Router.ServeHTTP(rr, req)
	assert.Contains(t, string(decodeData(t, rr)), "success")

	// dictionaries
	rr = app.Do(http.MethodPost, "/admin/v1/dict/types", map[string]any{"name": "状态", "code": "status", "status": 1}, token)
	dictID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/dict/types?page=1&page_size=10", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "status")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/dict/types/%d", dictID), map[string]any{"name": "状态2", "code": "status", "status": 1}, token)
	assert.Contains(t, string(decodeData(t, rr)), "状态2")
	rr = app.Do(http.MethodPost, fmt.Sprintf("/admin/v1/dict/types/%d/items", dictID), map[string]any{
		"dict_type_id": dictID, "label": "正常", "value": "1", "status": 1,
	}, token)
	itemID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, fmt.Sprintf("/admin/v1/dict/types/%d/items", dictID), nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "正常")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/dict/items/%d", itemID), map[string]any{
		"dict_type_id": dictID, "label": "启用", "value": "1", "status": 1,
	}, token)
	assert.Contains(t, string(decodeData(t, rr)), "启用")

	// parameters
	rr = app.Do(http.MethodPost, "/admin/v1/parameters", map[string]any{
		"key": "site.name", "value": "x-admin", "name": "站点名称", "public": true,
	}, token)
	paramID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/parameters?page=1&page_size=10", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "site.name")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/parameters/%d", paramID), map[string]any{
		"key": "site.name", "value": "x", "name": "站点", "public": true,
	}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)

	// files
	fileBody, fileType := uploadFile(t, "hello.txt", "hello")
	req = httptest.NewRequest(http.MethodPost, "/admin/v1/files", fileBody)
	req.Header.Set("Content-Type", fileType)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	app.Router.ServeHTTP(rr, req)
	fileID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/files?page=1&page_size=10", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "hello.txt")
	rr = app.Do(http.MethodGet, "/admin/v1/files?page=1&page_size=10&kind=document", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "hello.txt")
	rr = app.Do(http.MethodGet, "/admin/v1/files?page=1&page_size=10&kind=image", nil, token)
	assert.NotContains(t, string(decodeData(t, rr)), "hello.txt")
	rr = app.Do(http.MethodGet, fmt.Sprintf("/admin/v1/files/%d/download", fileID), nil, token)
	assert.Equal(t, "hello", rr.Body.String())

	// jobs
	rr = app.Do(http.MethodPost, "/admin/v1/jobs", map[string]any{
		"name": "清理", "cron": "0 3 * * *", "handler": "clean_logs", "params": `{"days":90}`, "status": 1,
	}, token)
	jobID := decodeID(t, rr)
	rr = app.Do(http.MethodGet, "/admin/v1/jobs?page=1&page_size=10", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "clean_logs")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/jobs/%d", jobID), map[string]any{
		"name": "清理 2", "cron": "0 4 * * *", "handler": "clean_logs", "params": `{"days":30}`, "status": 1,
	}, token)
	assert.Contains(t, string(decodeData(t, rr)), "清理 2")
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/jobs/%d/status", jobID), map[string]any{"status": 0}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodPost, fmt.Sprintf("/admin/v1/jobs/%d/run", jobID), nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)

	// logs (login logs already exist)
	rr = app.Do(http.MethodGet, "/admin/v1/logs/logins?page=1&page_size=10", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "登录成功")

	// profile
	rr = app.Do(http.MethodGet, "/admin/v1/profile", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "admin")
	rr = app.Do(http.MethodPut, "/admin/v1/profile", map[string]any{"nickname": "管理员", "email": "admin@x-admin.cn"}, token)
	assert.Contains(t, string(decodeData(t, rr)), "管理员")

	// deletes
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/files/%d", fileID), nil, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/jobs/%d", jobID), nil, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/parameters/%d", paramID), nil, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/dict/items/%d", itemID), nil, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/dict/types/%d", dictID), nil, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, "/admin/v1/users", map[string]any{"ids": []uint64{userID}}, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/roles/%d", roleID), nil, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/menus/%d", menuID), nil, token)).Code)
	require.Equal(t, 0, testsupport.Decode(t, app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/departments/%d", deptID), nil, token)).Code)

	// flush the audit queue and verify at least one operation log was written.
	app.Services.Logs.Close()
	logs, total, err := app.Repos.Logs.ListOperations(context.Background(), model.OperationLogFilter{})
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
	assert.NotEmpty(t, logs)

	rr = app.Do(http.MethodGet, "/admin/v1/logs/operations?page=1&page_size=10", nil, token)
	assert.Contains(t, string(decodeData(t, rr)), "username")
	var operationPage struct {
		List []struct {
			ID uint64 `json:"id"`
		} `json:"list"`
	}
	require.NoError(t, json.Unmarshal(decodeData(t, rr), &operationPage))
	if len(operationPage.List) > 0 {
		rr = app.Do(http.MethodDelete, "/admin/v1/logs/operations", map[string]any{"ids": []uint64{operationPage.List[0].ID}}, token)
		require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	}
	rr = app.Do(http.MethodGet, "/admin/v1/logs/logins?page=1&page_size=10", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodDelete, "/admin/v1/logs/operations/clear", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodDelete, "/admin/v1/logs/logins/clear", nil, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)

	rr = app.Do(http.MethodPut, "/admin/v1/profile/password", map[string]any{
		"old_password": "Admin@123456", "new_password": "NewAdmin123",
	}, token)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
	app.Login(t, "admin", "NewAdmin123")

	_ = admin
}

func TestAPIPermissionAndValidation(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	adminToken := app.Login(t, "admin", "Admin@123456").AccessToken

	app.EnsureRole(t, "viewer", "访客")
	app.CreateUser(t, "viewer", "Admin@123456", model.StatusEnabled, "viewer")
	viewerToken := app.Login(t, "viewer", "Admin@123456").AccessToken

	rr := app.Do(http.MethodGet, "/admin/v1/users", nil, viewerToken)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, viewerToken)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = app.Do(http.MethodGet, "/admin/v1/users/abc", nil, adminToken)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	rr = app.Do(http.MethodPost, "/admin/v1/users", "{", adminToken)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	rr = app.Do(http.MethodGet, "/admin/v1/users", nil, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	rr = app.Do(http.MethodGet, "/admin/v1/nope", nil, adminToken)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	rr = app.Do(http.MethodPost, "/admin/v1/auth/logout", map[string]any{}, adminToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code)
}

func uploadFile(t *testing.T, name, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func userImportFile(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	data, err := excel.Write("用户", []string{"用户名", "昵称", "邮箱", "手机号", "部门ID", "部门名称", "密码", "状态"}, [][]string{
		{"imported", "导入用户", "import@example.com", "13800000001", "", "", "Passw0rd", "1"},
	})
	require.NoError(t, err)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "users.xlsx")
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}
