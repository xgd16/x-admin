package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestLogServiceQueueAndQueries(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	svc := service.NewLogService(app.Repos.Logs, 8)

	svc.RecordOperation(nil)
	svc.RecordOperation(&model.OperationLog{UserID: 1, Username: "admin", Method: "POST", Path: "/a", Status: 200})
	svc.RecordOperation(&model.OperationLog{UserID: 2, Username: "bob", Method: "DELETE", Path: "/b", Status: 500})
	svc.Close()
	svc.Close()

	list, total, err := svc.Operations(ctx, model.OperationLogFilter{Query: paginationQuery()})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)

	list, total, err = svc.Operations(ctx, func() model.OperationLogFilter {
		f := model.OperationLogFilter{Query: paginationQuery()}
		f.Keyword = "bob"
		f.Status = intPtr(500)
		return f
	}())
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "bob", list[0].Username)

	start := time.Now().Add(-time.Hour)
	_, total, err = svc.Operations(ctx, model.OperationLogFilter{Query: paginationQuery(), Start: &start})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	require.NoError(t, svc.DeleteOperations(ctx, list[0].ID))
	require.NoError(t, svc.DeleteOperations(ctx))
	_, total, err = svc.Operations(ctx, model.OperationLogFilter{Query: paginationQuery()})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.NoError(t, svc.ClearOperations(ctx))
}

func TestLogServiceLoginLogs(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	svc := service.NewLogService(app.Repos.Logs, 4)
	defer svc.Close()

	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{Username: "admin", IP: "127.0.0.1", Status: 200, Message: "ok"}))
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{Username: "bob", IP: "10.0.0.1", Status: 401, Message: "bad"}))

	list, total, err := svc.Logins(ctx, model.LoginLogFilter{Query: paginationQuery()})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)

	_, total, err = svc.Logins(ctx, func() model.LoginLogFilter {
		f := model.LoginLogFilter{Query: paginationQuery()}
		f.Keyword = "bob"
		f.Status = intPtr(401)
		return f
	}())
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	require.NoError(t, svc.ClearLogins(ctx))
	_, total, err = svc.Logins(ctx, model.LoginLogFilter{Query: paginationQuery()})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestLogServiceDropsWhenQueueFull(t *testing.T) {
	app := testsupport.New(t)
	svc := service.NewLogService(app.Repos.Logs, 1)
	for i := 0; i < 50; i++ {
		svc.RecordOperation(&model.OperationLog{Username: "x"})
	}
	svc.Close()
	_, total, err := svc.Operations(context.Background(), model.OperationLogFilter{Query: paginationQuery()})
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
	assert.LessOrEqual(t, total, int64(50))
}

func intPtr(v int) *int { return &v }
