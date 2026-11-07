package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/security"
	"x-admin/internal/pkg/totp"
	"x-admin/internal/repository"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestUserGuardBranches(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	users := app.Services.Users

	// 部门不存在。
	badDept := uint64(404)
	_, err := users.Create(ctx, service.CreateUserInput{
		Username: "user_dept", Password: "Passw0rd123", DeptID: &badDept,
	})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "部门不存在")

	// 角色 ID 为零。
	_, err = users.Create(ctx, service.CreateUserInput{
		Username: "user_zero", Password: "Passw0rd123", RoleIDs: []uint64{0},
	})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "角色 ID 不合法")

	// 角色不存在（重复 ID 会去重后仍校验失败）。
	_, err = users.Create(ctx, service.CreateUserInput{
		Username: "user_missing", Password: "Passw0rd123", RoleIDs: []uint64{999999, 999999},
	})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "角色不存在")

	// 非法状态值。
	badStatus := int8(5)
	_, err = users.Create(ctx, service.CreateUserInput{
		Username: "user_status", Password: "Passw0rd123", Status: &badStatus,
	})
	require.Error(t, err)

	// 详情不存在。
	_, err = users.Get(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	// 非超级管理员不能授予超级管理员角色。
	super := app.EnsureRole(t, "super_admin", "超级管理员")
	normal := app.CreateUser(t, "plain_actor", "Passw0rd123", model.StatusEnabled)
	_, err = users.Create(ctx, service.CreateUserInput{
		Username: "user_super", Password: "Passw0rd123", RoleIDs: []uint64{super.ID},
	}, normal.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "只有超级管理员")

	// 非超级管理员不能分配权限超出自身的角色。
	menu := &model.Menu{Name: "敏感菜单", Type: model.MenuMenu, Path: "/x", Permission: "x:y:z", Status: 1, Visible: true}
	require.NoError(t, app.Repos.Menus.Create(ctx, menu))
	privileged := &model.Role{Name: "特权角色", Code: "privileged", Status: model.StatusEnabled, Sort: 1}
	require.NoError(t, app.Repos.Roles.Create(ctx, privileged))
	require.NoError(t, app.Repos.Roles.SetMenus(ctx, privileged.ID, []uint64{menu.ID}))
	_, err = users.Create(ctx, service.CreateUserInput{
		Username: "user_priv", Password: "Passw0rd123", RoleIDs: []uint64{privileged.ID},
	}, normal.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "不能分配权限超过自己")

	// 内置 admin 不能被摘除超级管理员角色。
	_, err = users.Update(ctx, admin.ID, service.UpdateUserInput{RoleIDs: &[]uint64{privileged.ID}}, admin.ID)
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "内置管理员必须保留")
}

func TestRoleSuperAdminGuards(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	// super_admin 尚不存在：占用编码被守护分支直接拒绝。
	role := &model.Role{Name: "普通角色", Code: "plain", Status: model.StatusEnabled, Sort: 1}
	require.NoError(t, app.Repos.Roles.Create(ctx, role))
	_, err := app.Services.Roles.Update(ctx, role.ID, service.RoleInput{Name: "普通角色", Code: "super_admin", Status: 1})
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
	_, err = app.Services.Roles.Create(ctx, service.RoleInput{Name: "山寨超管", Code: "super_admin", Status: 1})
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)

	// super_admin 存在后：编码不可修改、不可停用。
	super := app.EnsureRole(t, "super_admin", "超级管理员")

	// super_admin 编码不可修改、不可停用。
	_, err = app.Services.Roles.Update(ctx, super.ID, service.RoleInput{Name: "超级管理员", Code: "hacked", Status: 1})
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	_, err = app.Services.Roles.Update(ctx, super.ID, service.RoleInput{Name: "超级管理员", Code: "super_admin", Status: model.StatusDisabled})
	assert.Equal(t, apperr.CodeForbidden, apperr.From(err).Code)
}

func TestDictGuardBranches(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	dicts := app.Services.Dicts

	_, err := dicts.UpdateType(ctx, 404, service.DictTypeInput{Name: "x", Code: "x"})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(dicts.DeleteItem(ctx, 404)).Code)
	_, err = dicts.UpdateItem(ctx, 404, service.DictItemInput{Label: "x", Value: "x"})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	// 字典项所属类型不存在。
	_, err = dicts.CreateItem(ctx, service.DictItemInput{DictTypeID: 404, Label: "x", Value: "x"})
	require.Error(t, err)
}

func TestFileShareResolveErrors(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	files := app.Services.Files
	admin := app.SeedAdmin(t)

	file := &model.File{
		Key: "2026/11/x.png", Name: "x.png", URL: "/uploads/2026/11/x.png",
		MIME: "image/png", Size: 3, UploaderID: admin.ID,
	}
	require.NoError(t, app.Repos.Files.Create(ctx, file))

	// 空令牌与未知令牌。
	_, err := files.ResolveShare(ctx, "")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
	_, err = files.ResolveShare(ctx, "unknown-token")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	// 过期与撤销的分享。
	expired := &model.FileShare{
		FileID: file.ID, TokenHash: security.HashToken("expired-token"),
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, app.Repos.Files.CreateShare(ctx, expired))
	_, err = files.ResolveShare(ctx, "expired-token")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	revokedAt := time.Now().Add(-time.Minute)
	revoked := &model.FileShare{
		FileID: file.ID, TokenHash: security.HashToken("revoked-token"),
		ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &revokedAt,
	}
	require.NoError(t, app.Repos.Files.CreateShare(ctx, revoked))
	_, err = files.ResolveShare(ctx, "revoked-token")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	// 指向已删除文件的分享同样视为失效。
	ghost := &model.FileShare{
		FileID: 404, TokenHash: security.HashToken("ghost-token"),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, app.Repos.Files.CreateShare(ctx, ghost))
	_, err = files.ResolveShare(ctx, "ghost-token")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)

	// 有效分享返回文件 URL。
	share, token, err := files.CreateShare(ctx, file.ID, admin.ID, time.Hour)
	require.NoError(t, err)
	path, err := files.ResolveShare(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, file.URL, path)
	require.NotZero(t, share.ID)
}

func TestDashboardSummaryErrorPaths(t *testing.T) {
	// 关闭的数据库让每个聚合查询都返回错误，覆盖 Summary 的失败分支。
	db, err := gorm.Open(sqlite.Open("file:dashboard-broken?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	svc := service.NewDashboardService(
		repositoryUser(db), repositoryRole(db), repositoryMenu(db),
		repositoryFile(db), repositoryJob(db), repositoryLog(db),
	)
	_, err = svc.Summary(context.Background())
	require.Error(t, err)
}

func TestAuthMFARecoveryCodeAndRefreshDisabled(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	auth := app.Services.Auth
	profile := app.Services.Profile

	// 开启 TOTP 并保存一个恢复码用于登录。
	setup, err := profile.SetupTOTP(ctx, admin.ID)
	require.NoError(t, err)
	code, err := totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	recoveryCodes, err := profile.EnableTOTP(ctx, admin.ID, "Admin@123456", code)
	require.NoError(t, err)

	// 恢复码可以代替动态码完成登录。
	require.NoError(t, app.Cache.Set(ctx, "mfa:challenge:recovery", fmt.Sprint(admin.ID), time.Minute))
	result, err := auth.LoginMFA(ctx, service.LoginMFAInput{MFAToken: "recovery", Code: recoveryCodes[0]}, service.LoginMeta{})
	require.NoError(t, err)
	require.NotNil(t, result.TokenPair)

	// 账号停用后 refresh token 立即失效。
	require.NoError(t, app.Repos.Users.UpdateFields(ctx, admin.ID, map[string]any{"status": model.StatusDisabled}))
	_, err = auth.Refresh(ctx, result.TokenPair.RefreshToken, service.LoginMeta{})
	require.Error(t, err)
}

func repositoryUser(db *gorm.DB) *repository.UserRepo { return repository.NewUserRepo(db) }
func repositoryRole(db *gorm.DB) *repository.RoleRepo { return repository.NewRoleRepo(db) }
func repositoryMenu(db *gorm.DB) *repository.MenuRepo { return repository.NewMenuRepo(db) }
func repositoryFile(db *gorm.DB) *repository.FileRepo { return repository.NewFileRepo(db) }
func repositoryJob(db *gorm.DB) *repository.JobRepo   { return repository.NewJobRepo(db) }
func repositoryLog(db *gorm.DB) *repository.LogRepo   { return repository.NewLogRepo(db) }
