package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestOnlineServiceListFiltersAndRevoke(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	viewer := app.CreateUser(t, "viewer", "Passw0rd123", model.StatusEnabled)
	auth := newAuth(app, true)

	for i := 0; i < 2; i++ {
		_, err := login(app, auth, "admin", "Admin@123456")
		require.NoError(t, err)
	}
	_, err := login(app, auth, "viewer", "Passw0rd123")
	require.NoError(t, err)

	online := service.NewOnlineService(app.Repos.Users, app.Cache)
	page, err := online.List(ctx, service.OnlineQuery{Query: pagination.Query{Page: 1, PageSize: 20}}, "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.Total)
	assert.Equal(t, 2, page.Users)
	assert.Len(t, page.List, 3)

	// 按用户筛选与关键字搜索。
	page, err = online.List(ctx, service.OnlineQuery{Query: pagination.Query{Page: 1, PageSize: 20}, UserID: &viewer.ID}, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	assert.Equal(t, 1, page.Users)
	assert.Equal(t, "viewer", page.List[0].Username)

	page, err = online.List(ctx, service.OnlineQuery{Query: pagination.Query{Page: 1, PageSize: 20, Keyword: "admin"}}, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.Total)
	assert.Equal(t, 1, page.Users)

	// 分页只影响返回列表，不影响总数与在线用户数。
	page, err = online.List(ctx, service.OnlineQuery{Query: pagination.Query{Page: 2, PageSize: 1}}, "")
	require.NoError(t, err)
	assert.Len(t, page.List, 1)
	assert.Equal(t, int64(3), page.Total)
	assert.Equal(t, 2, page.Users)

	// 损坏记录与旧版本会话会在读取时清理。
	viewerPrefix := fmt.Sprintf("session:%d:", viewer.ID)
	require.NoError(t, app.Cache.Set(ctx, viewerPrefix+"corrupt", "not-json", time.Hour))
	stale, err := json.Marshal(map[string]any{"version": 99, "created_at": time.Now(), "last_seen_at": time.Now()})
	require.NoError(t, err)
	require.NoError(t, app.Cache.Set(ctx, viewerPrefix+"stale", string(stale), time.Hour))

	page, err = online.List(ctx, service.OnlineQuery{Query: pagination.Query{Page: 1, PageSize: 20}}, "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.Total)
	for _, gone := range []string{viewerPrefix + "corrupt", viewerPrefix + "stale"} {
		exists, err := app.Cache.Exists(ctx, gone)
		require.NoError(t, err)
		assert.False(t, exists)
	}

	// 撤销指定设备。
	keys, err := app.Cache.Keys(ctx, viewerPrefix)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	sessionID := strings.TrimPrefix(keys[0], viewerPrefix)
	require.NoError(t, online.Revoke(ctx, viewer.ID, sessionID))
	page, err = online.List(ctx, service.OnlineQuery{Query: pagination.Query{Page: 1, PageSize: 20}, UserID: &viewer.ID}, "")
	require.NoError(t, err)
	assert.Zero(t, page.Total)

	// 参数与不存在会话的错误分支。
	err = online.Revoke(ctx, 0, "x")
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	err = online.Revoke(ctx, viewer.ID, "missing")
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	// 撤销用户全部会话。
	revoked, err := online.RevokeUser(ctx, admin.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, revoked)
	revoked, err = online.RevokeUser(ctx, admin.ID)
	require.NoError(t, err)
	assert.Zero(t, revoked)
	revoked, err = online.RevokeUser(ctx, 0)
	require.Error(t, err)
	assert.Zero(t, revoked)
}
