package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/tests/testsupport"
)

// pageData 只解析测试需要的分页字段。
type pageData[T any] struct {
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

func decodePage[T any](t *testing.T, body []byte) pageData[T] {
	t.Helper()
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	require.Equal(t, 0, env.Code)
	var page pageData[T]
	require.NoError(t, json.Unmarshal(env.Data, &page))
	return page
}

func TestUserListFiltersBindQuery(t *testing.T) {
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	deptA := &model.Department{Name: "A", Status: model.StatusEnabled}
	deptB := &model.Department{Name: "B", Status: model.StatusEnabled}
	require.NoError(t, app.Repos.Depts.Create(context.Background(), deptA))
	require.NoError(t, app.Repos.Depts.Create(context.Background(), deptB))
	disabled := model.StatusDisabled
	alice := &model.User{Username: "alice", Password: "x", DeptID: &deptA.ID, Status: model.StatusEnabled}
	bob := &model.User{Username: "bob", Password: "x", DeptID: &deptB.ID, Status: disabled}
	require.NoError(t, app.Repos.Users.Create(context.Background(), alice))
	require.NoError(t, app.Repos.Users.Create(context.Background(), bob))

	rr := app.Do(http.MethodGet, "/admin/v1/users?dept_id="+fmt.Sprint(deptA.ID)+"&page_size=100", nil, pair.AccessToken)
	page := decodePage[model.User](t, rr.Body.Bytes())
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.List, 1)
	assert.Equal(t, "alice", page.List[0].Username)
	assert.NotEqual(t, admin.Username, page.List[0].Username)

	rr = app.Do(http.MethodGet, "/admin/v1/users?status=0&page_size=100", nil, pair.AccessToken)
	page = decodePage[model.User](t, rr.Body.Bytes())
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.List, 1)
	assert.Equal(t, "bob", page.List[0].Username)
}

func TestLogListFiltersBindQuery(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")
	ctx := context.Background()

	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now()
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{
		Base: model.Base{CreatedAt: old}, Username: "old", Status: 401, Message: "bad",
	}))
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{
		Base: model.Base{CreatedAt: recent}, Username: "recent", Status: 401, Message: "bad",
	}))
	require.NoError(t, app.Repos.Logs.CreateOperation(ctx, &model.OperationLog{
		Base: model.Base{CreatedAt: recent}, Username: "ops", Method: "POST", Path: "/x", Status: 500,
	}))
	require.NoError(t, app.Repos.Logs.CreateOperation(ctx, &model.OperationLog{
		Base: model.Base{CreatedAt: recent}, Username: "ops", Method: "POST", Path: "/y", Status: 200,
	}))

	rr := app.Do(http.MethodGet, "/admin/v1/logs/logins?status=401&page_size=100", nil, pair.AccessToken)
	page := decodePage[model.LoginLog](t, rr.Body.Bytes())
	assert.Equal(t, int64(2), page.Total)

	start := url.QueryEscape(recent.Add(-time.Hour).Format(time.RFC3339))
	rr = app.Do(http.MethodGet, "/admin/v1/logs/logins?status=401&start="+start+"&page_size=100", nil, pair.AccessToken)
	page = decodePage[model.LoginLog](t, rr.Body.Bytes())
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.List, 1)
	assert.Equal(t, "recent", page.List[0].Username)

	rr = app.Do(http.MethodGet, "/admin/v1/logs/operations?status=500&page_size=100", nil, pair.AccessToken)
	ops := decodePage[model.OperationLog](t, rr.Body.Bytes())
	assert.Equal(t, int64(1), ops.Total)
	require.Len(t, ops.List, 1)
	assert.Equal(t, "/x", ops.List[0].Path)

	// 异步导出：提交任务，等待完成后通过下载接口校验 xlsx 内容。
	ctx2 := context.Background()
	admin, err := app.Repos.Users.FindByUsername(ctx2, "admin")
	require.NoError(t, err)
	submitExport := func(module, params string) *model.ExportTask {
		t.Helper()
		task, err := app.Services.Exports.Submit(ctx2, admin.ID, module, json.RawMessage(params))
		require.NoError(t, err)
		deadline := time.Now().Add(5 * time.Second)
		for {
			fresh, err := app.Repos.Exports.Get(ctx2, task.ID)
			require.NoError(t, err)
			if fresh.Status != model.ExportPending && fresh.Status != model.ExportRunning {
				require.Equal(t, model.ExportSuccess, fresh.Status, fresh.Error)
				return fresh
			}
			if time.Now().After(deadline) {
				t.Fatal("export task not finished in time")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	for _, tc := range []struct {
		module, params, path string
	}{{"operation_logs", `{"status":500}`, "/admin/v1/export-tasks/%d/download"},
		{"login_logs", `{"status":401}`, "/admin/v1/export-tasks/%d/download"}} {
		task := submitExport(tc.module, tc.params)
		rr = app.Do(http.MethodGet, fmt.Sprintf(tc.path, task.ID), nil, pair.AccessToken)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "PK", rr.Body.String()[:2])
	}
}

func TestRoleListStatusFilterBindQuery(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")
	ctx := context.Background()
	require.NoError(t, app.Repos.Roles.Create(ctx, &model.Role{Name: "enabled", Code: "enabled_role", Status: model.StatusEnabled}))
	require.NoError(t, app.Repos.Roles.Create(ctx, &model.Role{Name: "disabled", Code: "disabled_role", Status: model.StatusDisabled}))

	rr := app.Do(http.MethodGet, "/admin/v1/roles?status=0&page_size=100", nil, pair.AccessToken)
	page := decodePage[model.Role](t, rr.Body.Bytes())
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.List, 1)
	assert.Equal(t, "disabled_role", page.List[0].Code)
}
