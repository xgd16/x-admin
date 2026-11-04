package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestUserRoleScopeGuards(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	super, err := app.Repos.Roles.FindByCode(ctx, "super_admin")
	require.NoError(t, err)
	target := app.CreateUser(t, "scope-target", "Passw0rd123", model.StatusEnabled)
	wide := app.EnsureRole(t, "wide-scope", "宽权限")

	// 操作者不存在。
	err = app.Services.Users.AssignRoles(ctx, target.ID, []uint64{super.ID}, 999)
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)

	// 超级管理员可以分配任意角色。
	require.NoError(t, app.Services.Users.AssignRoles(ctx, target.ID, []uint64{wide.ID}, admin.ID))

	// 内置 admin 可以保留超级管理员角色。
	require.NoError(t, app.Services.Users.AssignRoles(ctx, admin.ID, []uint64{super.ID}, admin.ID))

	// 创建用户时操作者不存在，也不能绕过范围校验。
	_, err = app.Services.Users.Create(ctx, service.CreateUserInput{
		Username: "created_by_ghost", Password: "Passw0rd123", Nickname: "ghost",
		RoleIDs: []uint64{wide.ID},
	}, 999)
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
}

func TestAdminWithoutSuperRoleCanBeAssigned(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	// 不创建超级管理员角色，模拟种子数据缺失时的兜底路径。
	admin := app.CreateUser(t, "admin", "Passw0rd123", model.StatusEnabled)
	role := app.EnsureRole(t, "plain", "普通角色")
	require.NoError(t, app.Services.Users.AssignRoles(ctx, admin.ID, []uint64{role.ID}))
	roles, err := app.Repos.Users.Get(ctx, admin.ID)
	require.NoError(t, err)
	require.Len(t, roles.Roles, 1)
	assert.Equal(t, "plain", roles.Roles[0].Code)
}

func TestUserSelfStatusUpdateGuard(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	self := app.CreateUser(t, "self_status", "Passw0rd123", model.StatusEnabled)
	other := app.CreateUser(t, "other_status", "Passw0rd123", model.StatusEnabled)
	disabled := model.StatusDisabled

	// 通过 Update 停用自己会被拒绝。
	_, err := app.Services.Users.Update(ctx, self.ID, service.UpdateUserInput{Status: &disabled}, self.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	// 允许停用其他用户，并递增会话版本。
	_, err = app.Services.Users.Update(ctx, other.ID, service.UpdateUserInput{Status: &disabled}, admin.ID)
	require.NoError(t, err)
	got, err := app.Repos.Users.Get(ctx, other.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusDisabled, got.Status)
	assert.Equal(t, int64(1), got.SessionVersion)
}

func TestUserCreateValidationEdges(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	role := app.EnsureRole(t, "edge", "边界")

	cases := []service.CreateUserInput{
		{Username: "bad name", Password: "Passw0rd123"},
		{Username: "zero-role", Password: "Passw0rd123", RoleIDs: []uint64{0}},
		{Username: "missing-dept", Password: "Passw0rd123", DeptID: uint64Ptr(999)},
	}
	for _, in := range cases {
		_, err := app.Services.Users.Create(ctx, in)
		assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code, in.Username)
	}

	// 重复角色 ID 会被去重，不影响创建。
	user, err := app.Services.Users.Create(ctx, service.CreateUserInput{
		Username: "dup_role", Password: "Passw0rd123", RoleIDs: []uint64{role.ID, role.ID},
	})
	require.NoError(t, err)
	assert.NotZero(t, user.ID)
}
