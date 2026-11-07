package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/repository"
	"x-admin/tests/testsupport"
)

// TestRepoNotFoundAndEdgeBranches 覆盖仓储层的"行不存在"与边界分支。
func TestRepoNotFoundAndEdgeBranches(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	// UpdateFields：目标行不存在时返回 ErrNotFound。
	assert.ErrorIs(t, app.Repos.Params.UpdateFields(ctx, 9999, map[string]any{"name": "x"}), repository.ErrNotFound)

	// 未传 ID 的批量删除是空操作。
	assert.NoError(t, app.Repos.Menus.Delete(ctx))

	// Base.List：不分页（pageSize=0）、页码小于 1 自动归位。
	jobs, _, err := app.Repos.Jobs.List(ctx, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, jobs)
	_, _, err = app.Repos.Jobs.List(ctx, 0, 10)
	require.NoError(t, err)

	// 凭据 ID 不存在返回 ErrNotFound。
	_, err = app.Repos.Passkeys.GetByCredentialID(ctx, "missing-credential")
	assert.ErrorIs(t, err, repository.ErrNotFound)

	// 字典类型：编码不存在、关键字/状态过滤、删除不存在的类型。
	_, err = app.Repos.Dicts.FindTypeByCode(ctx, "no-such-code")
	assert.ErrorIs(t, err, repository.ErrNotFound)

	status := int8(1)
	_, _, err = app.Repos.Dicts.ListTypes(ctx, pagination.Query{Keyword: "sys"}, &status)
	require.NoError(t, err)
	_, _, err = app.Repos.Dicts.ListTypes(ctx, pagination.Query{Page: 1, PageSize: 10}, nil)
	require.NoError(t, err)
	assert.Error(t, app.Repos.Dicts.DeleteType(ctx, 404))
}

func TestFileRepoFilterCombinations(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	kinds := "image,video,audio,document,archive,other,all,,junk"
	_, _, err := app.Repos.Files.List(ctx, model.FileFilter{
		Query: pagination.Query{Page: 1, PageSize: 10, Keyword: "photo"},
		MIME:  "image/", Kind: kinds,
	})
	require.NoError(t, err)
	_, _, err = app.Repos.Files.List(ctx, model.FileFilter{
		Query: pagination.Query{Page: 1, PageSize: 10}, Kind: "other",
	})
	require.NoError(t, err)
	total, err := app.Repos.Files.TotalSize(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(0))
}

func TestLogRepoFilterCombinations(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)
	opStatus := 201    // 与操作日志种子一致
	loginStatus := 200 // 与登录日志种子一致

	require.NoError(t, app.Repos.Logs.CreateOperation(ctx, &model.OperationLog{
		Base: model.Base{CreatedAt: time.Now()}, UserID: admin.ID, Username: admin.Username,
		Method: "POST", Path: "/admin/v1/users", Status: 201, Body: `{"a":1}`,
	}))
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{
		Base: model.Base{CreatedAt: time.Now()}, Username: admin.Username, Status: 200, IP: "127.0.0.1",
	}))

	ops, total, err := app.Repos.Logs.ListOperations(ctx, model.OperationLogFilter{
		Query:    pagination.Query{Page: 1, PageSize: 10, Keyword: "users"},
		Username: admin.Username, Status: &opStatus, Start: &start, End: &end,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, ops, 1)

	logins, _, err := app.Repos.Logs.ListLogins(ctx, model.LoginLogFilter{
		Query:    pagination.Query{Page: 1, PageSize: 10, Keyword: "127"},
		Username: admin.Username, Status: &loginStatus, Start: &start, End: &end,
	})
	require.NoError(t, err)
	require.Len(t, logins, 1)

	// days<=0 回退为默认 7 天；仓库层只返回有登录数据的日期，缺口由 service 层补齐。
	points, err := app.Repos.Logs.LoginTrend(ctx, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, points)
	assert.LessOrEqual(t, len(points), 7)
}

// TestTombstoneTruncatesLongUniqueValues 覆盖墓碑值在唯一字段接近上限时的截断分支。
func TestTombstoneTruncatesLongUniqueValues(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	longName := strings.Repeat("u", 64) // 用户名列上限
	user := app.CreateUser(t, longName, "Passw0rd123", model.StatusEnabled)
	require.NoError(t, app.Repos.Users.Delete(ctx, user.ID))
	// 墓碑值保留后缀，唯一索引释放后可以重建同名账号。
	require.NoError(t, app.Repos.Users.Create(ctx, &model.User{Username: longName, Password: "x", Status: model.StatusEnabled}))

	longCode := strings.Repeat("r", 64)
	role := &model.Role{Name: "长编码角色", Code: longCode, Status: model.StatusEnabled, Sort: 1}
	require.NoError(t, app.Repos.Roles.Create(ctx, role))
	require.NoError(t, app.Repos.Roles.Delete(ctx, role.ID))
	require.NoError(t, app.Repos.Roles.Create(ctx, &model.Role{Name: "重建角色", Code: longCode, Status: model.StatusEnabled, Sort: 1}))

	longKey := strings.Repeat("k", 128)
	param := &model.Parameter{Key: longKey, Name: "长键参数", Value: "v"}
	require.NoError(t, app.Repos.Params.Create(ctx, param))
	require.NoError(t, app.Repos.Params.Delete(ctx, param.ID))
	require.NoError(t, app.Repos.Params.Delete(ctx, 404)) // 删除不存在的 ID 是空操作
	assert.NoError(t, app.Repos.Params.Delete(ctx))
}
