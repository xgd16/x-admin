package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/tests/testsupport"
)

func TestLogRepoCleanBefore(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	old := time.Now().AddDate(0, 0, -120)
	require.NoError(t, app.Repos.Logs.CreateOperation(ctx, &model.OperationLog{
		Username: "old", Method: "POST", Path: "/old", CreatedAt: old,
	}))
	require.NoError(t, app.Repos.Logs.CreateOperation(ctx, &model.OperationLog{
		Username: "new", Method: "POST", Path: "/new",
	}))
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{
		Username: "old", Status: 200, CreatedAt: old,
	}))
	require.NoError(t, app.Repos.Logs.CreateLogin(ctx, &model.LoginLog{
		Username: "new", Status: 200,
	}))

	require.NoError(t, app.Repos.Logs.CleanBefore(ctx, 90))

	_, operations, err := app.Repos.Logs.ListOperations(ctx, model.OperationLogFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), operations)
	_, logins, err := app.Repos.Logs.ListLogins(ctx, model.LoginLogFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), logins)

	require.NoError(t, app.Repos.Logs.CleanBefore(ctx, 0))
}
