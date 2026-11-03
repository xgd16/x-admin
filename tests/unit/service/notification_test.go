package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func listTitles(t *testing.T, app *testsupport.App, userID uint64) []string {
	t.Helper()
	rows, _, err := app.Services.Notifications.List(context.Background(), userID, pagination.Query{Page: 1, PageSize: 50}, false)
	require.NoError(t, err)
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	return titles
}

func TestNotificationNotifyReadAndClean(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	owner := app.SeedAdmin(t)
	other := app.CreateUser(t, "reader", "Passw0rd123", model.StatusEnabled)

	require.NoError(t, app.Services.Notifications.Notify(ctx, service.NotificationInput{
		UserID: owner.ID, Title: "  标题  ", Content: strings.Repeat("内", 600),
		Type: "unexpected", Link: "https://evil.example",
	}))
	require.NoError(t, app.Services.Notifications.Notify(ctx, service.NotificationInput{
		UserID: owner.ID, Title: "第二条", Type: model.NotificationSuccess,
	}))

	count, err := app.Services.Notifications.UnreadCount(ctx, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	rows, total, err := app.Services.Notifications.List(ctx, owner.ID, pagination.Query{Page: 1, PageSize: 10}, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, rows, 2)
	assert.Equal(t, "标题", rows[1].Title)
	assert.Equal(t, 500, len([]rune(rows[1].Content)))
	assert.Equal(t, model.NotificationInfo, rows[1].Type)
	assert.Empty(t, rows[1].Link, "external links are dropped")

	unread, total, err := app.Services.Notifications.List(ctx, owner.ID, pagination.Query{}, true)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, unread, 2)

	require.NoError(t, app.Services.Notifications.MarkRead(ctx, owner.ID, rows[0].ID))
	require.NoError(t, app.Services.Notifications.MarkRead(ctx, owner.ID, rows[0].ID), "marking twice is idempotent")
	err = app.Services.Notifications.MarkRead(ctx, other.ID, rows[0].ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	err = app.Services.Notifications.MarkRead(ctx, owner.ID, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	count, err = app.Services.Notifications.UnreadCount(ctx, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	updated, err := app.Services.Notifications.MarkAllRead(ctx, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), updated)

	count, err = app.Services.Notifications.UnreadCount(ctx, owner.ID)
	require.NoError(t, err)
	assert.Zero(t, count)

	require.NoError(t, app.Services.Notifications.CleanBefore(ctx, -1))
}

func TestNotificationValidation(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	user := app.SeedAdmin(t)

	require.NoError(t, app.Services.Notifications.Notify(ctx, service.NotificationInput{Title: "no recipient"}))
	count, err := app.Services.Notifications.UnreadCount(ctx, user.ID)
	require.NoError(t, err)
	assert.Zero(t, count)

	err = app.Services.Notifications.Notify(ctx, service.NotificationInput{UserID: user.ID, Title: "   "})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
}

func TestAccountEventsCreateNotifications(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)

	target, err := app.Services.Users.Create(ctx, service.CreateUserInput{
		Username: "notified", Password: "Passw0rd123", Nickname: "通知用户",
	})
	require.NoError(t, err)
	assert.Contains(t, listTitles(t, app, target.ID), "欢迎使用 X-ADMIN")

	require.NoError(t, app.Services.Users.SetStatus(ctx, target.ID, model.StatusDisabled, admin.ID))
	assert.Contains(t, listTitles(t, app, target.ID), "账号已停用")
	require.NoError(t, app.Services.Users.SetStatus(ctx, target.ID, model.StatusEnabled, admin.ID))
	assert.Contains(t, listTitles(t, app, target.ID), "账号状态变更")

	require.NoError(t, app.Services.Users.ResetPassword(ctx, target.ID, "NewPassw0rd"))
	assert.Contains(t, listTitles(t, app, target.ID), "密码已重置")

	require.NoError(t, app.Services.Users.AssignRoles(ctx, target.ID, nil))
	assert.Contains(t, listTitles(t, app, target.ID), "角色权限已更新")

	require.NoError(t, app.Services.Profile.ChangePassword(ctx, target.ID, "NewPassw0rd", "AnotherPass1"))
	assert.Contains(t, listTitles(t, app, target.ID), "密码修改成功")
}
