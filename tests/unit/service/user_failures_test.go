package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/security"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

var errRepo = errors.New("repo failed")

type userGetFail struct{ service.UserRepository }

func (f userGetFail) Get(context.Context, uint64) (*model.User, error) { return nil, errRepo }

type userExistsFail struct{ service.UserRepository }

func (f userExistsFail) ExistsUsername(context.Context, string, uint64) (bool, error) {
	return false, errRepo
}

type userCreateFail struct{ service.UserRepository }

func (f userCreateFail) Create(context.Context, *model.User) error { return errRepo }

type userUpdateFieldsFail struct{ service.UserRepository }

func (f userUpdateFieldsFail) UpdateFields(context.Context, uint64, map[string]any) error {
	return errRepo
}

type userSetPasswordFail struct{ service.UserRepository }

func (f userSetPasswordFail) SetPassword(context.Context, uint64, string, bool) error {
	return errRepo
}

type userBumpFail struct{ service.UserRepository }

func (f userBumpFail) BumpSessionVersion(context.Context, uint64) error { return errRepo }

type userSetRolesFail struct{ service.UserRepository }

func (f userSetRolesFail) SetRoles(context.Context, uint64, []uint64) error { return errRepo }

type userDeleteFail struct{ service.UserRepository }

func (f userDeleteFail) Delete(context.Context, ...uint64) error { return errRepo }

type userListFail struct{ service.UserRepository }

func (f userListFail) List(context.Context, model.UserFilter) ([]model.User, int64, error) {
	return nil, 0, errRepo
}

type roleGetFail struct{ service.RoleRepository }

func (f roleGetFail) Get(context.Context, uint64) (*model.Role, error) { return nil, errRepo }

type menuForUserFail struct{ service.MenuRepository }

func (f menuForUserFail) ForUser(context.Context, uint64) ([]model.Menu, error) {
	return nil, errRepo
}

type deptGetFail struct{ service.DepartmentRepository }

func (f deptGetFail) Get(context.Context, uint64) (*model.Department, error) {
	return nil, errRepo
}

type failingHasher struct{}

func (failingHasher) Hash(string) (string, error) { return "", errRepo }
func (failingHasher) Verify(string, string) bool  { return false }

func newUserSvc(app *testsupport.App, users service.UserRepository, roles service.RoleRepository,
	menus service.MenuRepository, depts service.DepartmentRepository, hasher security.PasswordHasher) *service.UserService {
	return service.NewUserService(users, roles, menus, depts, hasher, 8, app.Services.Notifications)
}

func TestUserServiceRepositoryFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)
	target := app.CreateUser(t, "target_fail", "Passw0rd123", model.StatusEnabled)
	statusTarget := app.CreateUser(t, "status_fail", "Passw0rd123", model.StatusEnabled)
	role := app.EnsureRole(t, "fail_role", "角色")
	disabled := model.StatusDisabled

	base := newUserSvc(app, app.Repos.Users, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher)
	_, err := base.Get(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = newUserSvc(app, userGetFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Get(ctx, target.ID)
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, userExistsFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Create(ctx, service.CreateUserInput{Username: "exists_fail", Password: "Passw0rd123"})
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, userCreateFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Create(ctx, service.CreateUserInput{Username: "create_fail", Password: "Passw0rd123"})
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, app.Repos.Users, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, failingHasher{}).
		Create(ctx, service.CreateUserInput{Username: "hash_fail", Password: "Passw0rd123"})
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, userSetRolesFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Create(ctx, service.CreateUserInput{Username: "roles_fail", Password: "Passw0rd123", RoleIDs: []uint64{role.ID}})
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, app.Repos.Users, roleGetFail{app.Repos.Roles}, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Create(ctx, service.CreateUserInput{Username: "role_lookup_fail", Password: "Passw0rd123", RoleIDs: []uint64{role.ID}})
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, app.Repos.Users, app.Repos.Roles, app.Repos.Menus, deptGetFail{app.Repos.Depts}, app.Hasher).
		Create(ctx, service.CreateUserInput{Username: "dept_fail", Password: "Passw0rd123", DeptID: uint64Ptr(1)})
	assert.ErrorIs(t, err, errRepo)

	_, err = newUserSvc(app, userUpdateFieldsFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Update(ctx, target.ID, service.UpdateUserInput{Nickname: "x"})
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, userSetRolesFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Update(ctx, target.ID, service.UpdateUserInput{RoleIDs: &[]uint64{role.ID}})
	assert.ErrorIs(t, err, errRepo)
	_, err = newUserSvc(app, userBumpFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Update(ctx, target.ID, service.UpdateUserInput{Status: &disabled})
	assert.ErrorIs(t, err, errRepo)

	err = newUserSvc(app, userBumpFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		SetStatus(ctx, statusTarget.ID, model.StatusDisabled, 0)
	assert.ErrorIs(t, err, errRepo)
	err = newUserSvc(app, userUpdateFieldsFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		SetStatus(ctx, statusTarget.ID, model.StatusDisabled, 0)
	assert.ErrorIs(t, err, errRepo)

	err = newUserSvc(app, app.Repos.Users, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, failingHasher{}).
		ResetPassword(ctx, target.ID, "Passw0rd123")
	assert.ErrorIs(t, err, errRepo)
	err = newUserSvc(app, userSetPasswordFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		ResetPassword(ctx, target.ID, "Passw0rd123")
	assert.ErrorIs(t, err, errRepo)

	err = newUserSvc(app, userDeleteFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Delete(ctx, []uint64{target.ID}, 0)
	assert.ErrorIs(t, err, errRepo)

	err = newUserSvc(app, userSetRolesFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		AssignRoles(ctx, target.ID, []uint64{role.ID})
	assert.ErrorIs(t, err, errRepo)

	_, err = newUserSvc(app, userListFail{app.Repos.Users}, app.Repos.Roles, app.Repos.Menus, app.Repos.Depts, app.Hasher).
		Export(ctx, model.UserFilter{})
	assert.ErrorIs(t, err, errRepo)

	// 非超级管理员分配角色时菜单查询失败。
	managerRole := app.EnsureRole(t, "fail_manager", "主管")
	manager := app.CreateUser(t, "fail_manager", "Passw0rd123", model.StatusEnabled, managerRole.Code)
	err = newUserSvc(app, app.Repos.Users, app.Repos.Roles, menuForUserFail{app.Repos.Menus}, app.Repos.Depts, app.Hasher).
		AssignRoles(ctx, target.ID, []uint64{role.ID}, manager.ID)
	assert.ErrorIs(t, err, errRepo)
}
