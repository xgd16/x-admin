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

func TestProfileFlow(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	profile := app.Services.Profile

	user, err := profile.Get(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Username)

	updated, err := profile.Update(ctx, admin.ID, service.ProfileInput{
		Nickname: "新昵称", Email: "new@example.com", Phone: "13900000000", Avatar: "/a.png",
	})
	require.NoError(t, err)
	assert.Equal(t, "新昵称", updated.Nickname)
	assert.Equal(t, "new@example.com", updated.Email)

	_, err = profile.Update(ctx, admin.ID, service.ProfileInput{Email: "bad"})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = profile.Update(ctx, admin.ID, service.ProfileInput{Phone: "x"})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	_, err = profile.Update(ctx, 404, service.ProfileInput{})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = profile.Get(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestProfileChangePassword(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	profile := app.Services.Profile

	err := profile.ChangePassword(ctx, admin.ID, "wrong", "NewPass123")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	err = profile.ChangePassword(ctx, admin.ID, "Admin@123456", "short")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	err = profile.ChangePassword(ctx, admin.ID, "Admin@123456", "Admin@123456")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	require.NoError(t, profile.ChangePassword(ctx, admin.ID, "Admin@123456", "NewPass123"))
	after, err := app.Repos.Users.Get(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), after.SessionVersion)

	auth := newAuth(app, true)
	_, err = login(app, auth, "admin", "NewPass123")
	require.NoError(t, err)

	err = profile.ChangePassword(ctx, 404, "x", "NewPass123")
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestDashboardSummary(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	app.CreateUser(t, "second", "Admin@123456", model.StatusDisabled)
	app.EnsureRole(t, "operator", "运营")
	require.NoError(t, app.Repos.Menus.Create(ctx, &model.Menu{
		Name: "总览", Type: model.MenuMenu, Path: "/dashboard", Permission: "dashboard:view", Status: 1, Visible: true,
	}))
	require.NoError(t, app.Repos.Files.Create(ctx, &model.File{Name: "a.txt", Key: "a", URL: "/a", Size: 12}))
	require.NoError(t, app.Repos.Jobs.Create(ctx, &model.Job{Name: "noop", Cron: "0 1 * * *", Handler: "noop", Status: 1}))
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{UserID: admin.ID, Username: "admin", Status: 200, Message: "ok"}))
	// 失败登录不应计入登录趋势。
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{UserID: admin.ID, Username: "admin", Status: 401, Message: "bad"}))
	require.NoError(t, app.Repos.Logs.CreateOperation(ctx, &model.OperationLog{UserID: admin.ID, Username: "admin", Method: "POST", Path: "/x", Status: 200}))

	summary, err := app.Services.Dashboard.Summary(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.Users)
	assert.Equal(t, int64(1), summary.Active)
	assert.Equal(t, int64(2), summary.Roles)
	assert.Equal(t, int64(1), summary.Menus)
	assert.Equal(t, int64(1), summary.Files)
	assert.Equal(t, int64(12), summary.FileSize)
	assert.Equal(t, int64(1), summary.Jobs)
	assert.Len(t, summary.LoginTrend, 7)
	assert.Len(t, summary.Recent, 1)
	assert.Equal(t, int64(1), summary.LoginTrend[len(summary.LoginTrend)-1].Count)
}
