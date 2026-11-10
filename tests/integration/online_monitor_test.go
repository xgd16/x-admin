package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

type onlinePage struct {
	List  []service.OnlineSessionInfo `json:"list"`
	Total int64                       `json:"total"`
	Users int                         `json:"users"`
}

func decodeOnlinePage(t *testing.T, body []byte) onlinePage {
	t.Helper()
	var env struct {
		Code int        `json:"code"`
		Data onlinePage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	require.Zero(t, env.Code, string(body))
	return env.Data
}

func TestAdminOnlineSessionsListAndForceLogout(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	first := app.Login(t, "admin", "Admin@123456")
	second := app.Login(t, "admin", "Admin@123456")

	rr := app.Do(http.MethodGet, "/admin/v1/online-sessions", nil, first.AccessToken)
	page := decodeOnlinePage(t, rr.Body.Bytes())
	require.Len(t, page.List, 2)
	assert.Equal(t, int64(2), page.Total)
	assert.Equal(t, 1, page.Users)

	var otherID string
	for _, row := range page.List {
		if !row.Current {
			otherID = row.ID
		}
	}
	require.NotEmpty(t, otherID)

	// 强制下线另一台设备后，它的 access token 立即失效。
	rr = app.Do(http.MethodDelete, "/admin/v1/online-sessions/1", map[string]string{"session_id": otherID}, first.AccessToken)
	require.Zero(t, testsupport.Decode(t, rr).Code, rr.Body.String())
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, second.AccessToken)
	assert.Equal(t, 40102, testsupport.Decode(t, rr).Code)

	// 当前设备仍可用，列表只剩一条会话。
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, first.AccessToken)
	assert.Zero(t, testsupport.Decode(t, rr).Code)
	rr = app.Do(http.MethodGet, "/admin/v1/online-sessions", nil, first.AccessToken)
	assert.Equal(t, int64(1), decodeOnlinePage(t, rr.Body.Bytes()).Total)

	// 不带 session_id 时撤销该用户全部会话。
	rr = app.Do(http.MethodDelete, "/admin/v1/online-sessions/1", nil, first.AccessToken)
	require.Zero(t, testsupport.Decode(t, rr).Code, rr.Body.String())
	rr = app.Do(http.MethodGet, "/admin/v1/auth/me", nil, first.AccessToken)
	assert.Equal(t, 40102, testsupport.Decode(t, rr).Code)
}

func TestMonitorOverview(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken

	rr := app.Do(http.MethodGet, "/admin/v1/monitor/overview", nil, token)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	data := decodeData(t, rr)

	var overview struct {
		App struct {
			Name      string `json:"name"`
			StartedAt string `json:"started_at"`
		} `json:"app"`
		System  map[string]json.RawMessage `json:"system"`
		Runtime struct {
			GoVersion  string `json:"go_version"`
			Goroutines int    `json:"goroutines"`
		} `json:"runtime"`
		Database struct {
			Status string `json:"status"`
		} `json:"database"`
		Cache struct {
			Status string `json:"status"`
		} `json:"cache"`
	}
	require.NoError(t, json.Unmarshal(data, &overview))
	assert.NotEmpty(t, overview.App.Name)
	assert.NotEmpty(t, overview.App.StartedAt)
	assert.NotEmpty(t, overview.Runtime.GoVersion)
	assert.GreaterOrEqual(t, overview.Runtime.Goroutines, 1)
	assert.Equal(t, "up", overview.Database.Status)
	assert.Equal(t, "up", overview.Cache.Status)
	for _, key := range []string{"host", "cpu", "memory", "swap", "load", "disks", "network", "process"} {
		assert.Contains(t, overview.System, key)
	}
}

func TestMonitorRequiresPermission(t *testing.T) {
	app := testsupport.New(t)
	app.CreateUser(t, "viewer", "Passw0rd123", model.StatusEnabled)
	token := app.Login(t, "viewer", "Passw0rd123").AccessToken

	rr := app.Do(http.MethodGet, "/admin/v1/monitor/overview", nil, token)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Equal(t, 40301, testsupport.Decode(t, rr).Code)
}
