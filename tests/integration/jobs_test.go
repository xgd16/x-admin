package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/tests/testsupport"
)

func TestJobRunHistoryAPI(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	pair := app.Login(t, "admin", "Admin@123456")

	rr := app.Do(http.MethodPost, "/admin/v1/jobs", map[string]any{
		"name": "记录测试", "cron": "0 1 * * *", "handler": "noop", "status": 1,
	}, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	var job model.Job
	require.NoError(t, json.Unmarshal(testsupport.Decode(t, rr).Data, &job))
	require.NotZero(t, job.ID)

	rr = app.Do(http.MethodPost, fmt.Sprintf("/admin/v1/jobs/%d/run", job.ID), nil, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())

	rr = app.Do(http.MethodGet, fmt.Sprintf("/admin/v1/jobs/%d/runs?page=1&page_size=10", job.ID), nil, pair.AccessToken)
	page := decodePage[model.JobRun](t, rr.Body.Bytes())
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.List, 1)
	assert.Equal(t, "manual", page.List[0].Trigger)
	assert.Equal(t, int8(model.StatusEnabled), page.List[0].Status)
	assert.Equal(t, job.Name, page.List[0].JobName)
	assert.GreaterOrEqual(t, page.List[0].DurationMS, int64(0))

	// 删除任务时执行记录一并清理。
	rr = app.Do(http.MethodDelete, fmt.Sprintf("/admin/v1/jobs/%d", job.ID), nil, pair.AccessToken)
	require.Equal(t, 0, testsupport.Decode(t, rr).Code, rr.Body.String())
	_, total, err := app.Repos.Jobs.ListRuns(t.Context(), job.ID, 1, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
}
