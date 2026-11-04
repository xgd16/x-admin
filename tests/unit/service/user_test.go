package service_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/excel"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func seedDept(t *testing.T, app *testsupport.App) *model.Department {
	t.Helper()
	dept := &model.Department{Name: "技术部", Status: model.StatusEnabled}
	require.NoError(t, app.Repos.Depts.Create(context.Background(), dept))
	return dept
}

func enabledStatus() *int8 {
	v := model.StatusEnabled
	return &v
}

func TestUserCreate(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	dept := seedDept(t, app)
	role := app.EnsureRole(t, "operator", "运营")

	user, err := app.Services.Users.Create(ctx, service.CreateUserInput{
		Username: "alice", Password: "Passw0rd", Nickname: "Alice", Email: "alice@example.com",
		Phone: "13800000000", DeptID: &dept.ID, Status: enabledStatus(), RoleIDs: []uint64{role.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.NotEqual(t, "Passw0rd", user.Password)
	assert.Equal(t, dept.ID, *user.DeptID)
	require.Len(t, user.Roles, 1)
	assert.Equal(t, "operator", user.Roles[0].Code)
}

func TestUserCreateValidation(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	dept := seedDept(t, app)
	role := app.EnsureRole(t, "operator", "运营")
	base := service.CreateUserInput{Username: "valid", Password: "Passw0rd"}

	cases := []struct {
		name  string
		input service.CreateUserInput
		code  apperr.Code
	}{
		{"bad username", service.CreateUserInput{Username: "a", Password: "Passw0rd"}, apperr.CodeInvalidArgument},
		{"bad password", service.CreateUserInput{Username: "valid", Password: "short"}, apperr.CodeInvalidArgument},
		{"bad email", service.CreateUserInput{Username: "valid", Password: "Passw0rd", Email: "x"}, apperr.CodeInvalidArgument},
		{"bad phone", service.CreateUserInput{Username: "valid", Password: "Passw0rd", Phone: "abc"}, apperr.CodeInvalidArgument},
		{"missing dept", service.CreateUserInput{Username: "valid", Password: "Passw0rd", DeptID: uint64Ptr(999)}, apperr.CodeInvalidArgument},
		{"missing role", service.CreateUserInput{Username: "valid", Password: "Passw0rd", RoleIDs: []uint64{999}}, apperr.CodeInvalidArgument},
		{"bad status", service.CreateUserInput{Username: "valid", Password: "Passw0rd", Status: int8Ptr(2)}, apperr.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := app.Services.Users.Create(ctx, tc.input)
			require.Error(t, err)
			assert.Equal(t, tc.code, apperr.From(err).Code)
		})
	}

	_, err := app.Services.Users.Create(ctx, base)
	require.NoError(t, err)
	_, err = app.Services.Users.Create(ctx, service.CreateUserInput{Username: "valid", Password: "Passw0rd"})
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	deptUser, err := app.Services.Users.Create(ctx, service.CreateUserInput{
		Username: "bob", Password: "Passw0rd", DeptID: &dept.ID, RoleIDs: []uint64{role.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, dept.ID, *deptUser.DeptID)
}

func TestUserUpdate(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	dept := seedDept(t, app)
	role := app.EnsureRole(t, "operator", "运营")
	user := app.CreateUser(t, "alice", "Passw0rd", model.StatusEnabled)

	roleIDs := []uint64{role.ID}
	updated, err := app.Services.Users.Update(ctx, user.ID, service.UpdateUserInput{
		Nickname: "Alice", Email: "alice@example.com", Phone: "13900000000",
		DeptID: &dept.ID, Status: enabledStatus(), RoleIDs: &roleIDs, Remark: "hi",
	})
	require.NoError(t, err)
	assert.Equal(t, "Alice", updated.Nickname)
	assert.Equal(t, dept.ID, *updated.DeptID)
	assert.Len(t, updated.Roles, 1)

	_, err = app.Services.Users.Update(ctx, user.ID, service.UpdateUserInput{Email: "bad"})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Users.Update(ctx, user.ID, service.UpdateUserInput{Phone: "bad"})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Users.Update(ctx, user.ID, service.UpdateUserInput{DeptID: uint64Ptr(999)})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Users.Update(ctx, user.ID, service.UpdateUserInput{RoleIDs: &[]uint64{999}})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Users.Update(ctx, user.ID, service.UpdateUserInput{Status: int8Ptr(7)})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Users.Update(ctx, 404, service.UpdateUserInput{})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	_, err = app.Services.Users.Update(ctx, user.ID, service.UpdateUserInput{Status: int8Ptr(0)}, user.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	admin := app.SeedAdmin(t)
	_, err = app.Services.Users.Update(ctx, admin.ID, service.UpdateUserInput{Status: int8Ptr(0)})
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	err = app.Services.Users.AssignRoles(ctx, admin.ID, nil, admin.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
}

func TestUserStatusAndPassword(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	user := app.CreateUser(t, "alice", "Passw0rd", model.StatusEnabled)

	err := app.Services.Users.SetStatus(ctx, user.ID, 2, 1)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	err = app.Services.Users.SetStatus(ctx, 404, 0, 1)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	require.NoError(t, app.Services.Users.SetStatus(ctx, user.ID, 0, 999))
	afterDisable, err := app.Repos.Users.Get(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), afterDisable.SessionVersion)
	err = app.Services.Users.SetStatus(ctx, user.ID, 0, user.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	admin := app.SeedAdmin(t)
	err = app.Services.Users.SetStatus(ctx, admin.ID, 0, user.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)

	err = app.Services.Users.ResetPassword(ctx, user.ID, "weak")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	err = app.Services.Users.ResetPassword(ctx, 404, "Passw0rd")
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	require.NoError(t, app.Services.Users.ResetPassword(ctx, user.ID, "NewPass123"))
	afterReset, err := app.Repos.Users.Get(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), afterReset.SessionVersion)
}

func TestUserDeleteAndAssignRoles(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	role := app.EnsureRole(t, "operator", "运营")
	user := app.CreateUser(t, "alice", "Passw0rd", model.StatusEnabled)

	err := app.Services.Users.Delete(ctx, nil, 0)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	err = app.Services.Users.Delete(ctx, []uint64{404}, 0)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	admin := app.SeedAdmin(t)
	err = app.Services.Users.Delete(ctx, []uint64{admin.ID}, user.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	err = app.Services.Users.Delete(ctx, []uint64{user.ID}, user.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	super, err := app.Repos.Roles.FindByCode(ctx, "super_admin")
	require.NoError(t, err)
	actor := app.CreateUser(t, "actor", "Passw0rd", model.StatusEnabled, "operator")
	err = app.Services.Users.AssignRoles(ctx, user.ID, []uint64{super.ID}, actor.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)

	err = app.Services.Users.AssignRoles(ctx, 404, nil)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	err = app.Services.Users.AssignRoles(ctx, user.ID, []uint64{999})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	require.NoError(t, app.Services.Users.AssignRoles(ctx, user.ID, []uint64{role.ID}))
	require.NoError(t, app.Services.Users.Delete(ctx, []uint64{user.ID}, admin.ID))
}

func TestUserListExportTemplate(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	dept := seedDept(t, app)
	app.SeedAdmin(t)
	role := app.EnsureRole(t, "operator", "运营")
	_, err := app.Services.Users.Create(ctx, service.CreateUserInput{
		Username: "alice", Password: "Passw0rd", Nickname: "Alice", DeptID: &dept.ID, RoleIDs: []uint64{role.ID},
	})
	require.NoError(t, err)

	list, total, err := app.Services.Users.List(ctx, model.UserFilter{Query: paginationQuery()})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)

	enabled := model.StatusEnabled
	filter := model.UserFilter{Query: paginationQuery()}
	filter.Keyword = "alice"
	filter.Status = &enabled
	list, total, err = app.Services.Users.List(ctx, filter)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "alice", list[0].Username)

	data, err := app.Services.Users.Export(ctx, model.UserFilter{})
	require.NoError(t, err)
	rows, err := excel.Read(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, "用户名", rows[0][0])
	assert.Len(t, rows, 3)

	template, err := app.Services.Users.Template()
	require.NoError(t, err)
	rows, err = excel.Read(bytes.NewReader(template))
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	_, err = app.Services.Users.Get(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestUserImport(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	dept := seedDept(t, app)

	_, err := app.Services.Users.Import(ctx, nil)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	tooMany := make([]service.UserImportRow, 501)
	_, err = app.Services.Users.Import(ctx, tooMany)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	rows := []service.UserImportRow{
		{Username: "ok1", Password: "Passw0rd", DeptID: fmt.Sprint(dept.ID), Status: "1"},
		{Username: "ok2", Password: "Passw0rd", Status: "0", DeptID: "bad"},
		{Username: "ok1", Password: "Passw0rd"},
		{Username: "bad", Password: "short"},
	}
	result, err := app.Services.Users.Import(ctx, rows)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Success)
	assert.Len(t, result.Failed, 3)
	assert.Equal(t, 3, result.Failed[0].Row)
	assert.Equal(t, "部门 ID 不合法", result.Failed[0].Message)
}

func TestUserImportRowFromCells(t *testing.T) {
	row := service.ImportRowFromCells([]string{"alice", "Alice", "a@b.com", "13800000000", "2", "技术部", "Passw0rd", "1"})
	assert.Equal(t, "alice", row.Username)
	assert.Equal(t, "Passw0rd", row.Password)
	assert.Equal(t, "1", row.Status)
	short := service.ImportRowFromCells([]string{"bob"})
	assert.Equal(t, "bob", short.Username)
	assert.Equal(t, "", short.Password)
}

func uint64Ptr(v uint64) *uint64 { return &v }
func int8Ptr(v int8) *int8       { return &v }

func paginationQuery() pagination.Query {
	q := pagination.Query{Page: 1, PageSize: 20}
	q.Normalize()
	return q
}
