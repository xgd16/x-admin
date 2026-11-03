package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestRoleCreateAndMenus(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	menu := &model.Menu{Name: "用户管理", Type: model.MenuMenu, Path: "/users", Permission: "system:user:list", Status: 1, Visible: true}
	require.NoError(t, app.Repos.Menus.Create(ctx, menu))

	role, err := app.Services.Roles.Create(ctx, service.RoleInput{
		Name: "运营", Code: "operator", Status: 1, Sort: 2, Remark: "ops", MenuIDs: []uint64{menu.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, "operator", role.Code)

	ids, err := app.Services.Roles.Menus(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint64{menu.ID}, ids)

	roles, total, err := app.Services.Roles.List(ctx, model.RoleFilter{Query: pagination.Query{Page: 1, PageSize: 10}})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, roles, 1)

	all, err := app.Services.Roles.All(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestRoleValidation(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	_, err := app.Services.Roles.Create(ctx, service.RoleInput{Name: "运营", Code: "operator", Status: 1})
	require.NoError(t, err)

	cases := []service.RoleInput{
		{Name: "", Code: "valid", Status: 1},
		{Name: "X", Code: "Bad", Status: 1},
		{Name: "X", Code: "valid", Status: 2},
	}
	for _, in := range cases {
		_, err := app.Services.Roles.Create(ctx, in)
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	}

	_, err = app.Services.Roles.Create(ctx, service.RoleInput{Name: "X", Code: "operator", Status: 1})
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

func TestRoleUpdateDelete(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	role, err := app.Services.Roles.Create(ctx, service.RoleInput{Name: "运营", Code: "operator", Status: 1})
	require.NoError(t, err)

	updated, err := app.Services.Roles.Update(ctx, role.ID, service.RoleInput{Name: "运营 2", Code: "operator", Status: 0, Sort: 9})
	require.NoError(t, err)
	assert.Equal(t, "运营 2", updated.Name)
	assert.Equal(t, int8(0), updated.Status)

	_, err = app.Services.Roles.Update(ctx, 404, service.RoleInput{Name: "X", Code: "x", Status: 1})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	super := app.EnsureRole(t, "super_admin", "超级管理员")
	_, err = app.Services.Roles.Update(ctx, super.ID, service.RoleInput{Name: "超级管理员", Code: "changed", Status: 1})
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	err = app.Services.Roles.Delete(ctx, super.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	err = app.Services.Roles.Delete(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	user := app.CreateUser(t, "alice", "Passw0rd", model.StatusEnabled, "operator")
	require.NoError(t, app.Repos.Users.SetRoles(ctx, user.ID, []uint64{role.ID}))
	err = app.Services.Roles.Delete(ctx, role.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	require.NoError(t, app.Repos.Users.SetRoles(ctx, user.ID, nil))
	require.NoError(t, app.Services.Roles.Delete(ctx, role.ID))
}

func TestRoleAssignMenus(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	role := app.EnsureRole(t, "operator", "运营")
	menu := &model.Menu{Name: "菜单", Type: model.MenuMenu, Path: "/m", Permission: "system:menu:list", Status: 1, Visible: true}
	require.NoError(t, app.Repos.Menus.Create(ctx, menu))

	err := app.Services.Roles.AssignMenus(ctx, 404, []uint64{menu.ID})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	require.NoError(t, app.Services.Roles.AssignMenus(ctx, role.ID, []uint64{menu.ID}))
	ids, err := app.Services.Roles.Menus(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint64{menu.ID}, ids)

	_, err = app.Services.Roles.Menus(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}
