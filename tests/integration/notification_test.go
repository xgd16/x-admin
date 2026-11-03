package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/tests/testsupport"
)

func decodeInto(t *testing.T, rr *httptest.ResponseRecorder, dst any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(decodeData(t, rr), dst), rr.Body.String())
}

func TestNotificationAPI(t *testing.T) {
	app := testsupport.New(t)
	app.SeedAdmin(t)
	token := app.Login(t, "admin", "Admin@123456").AccessToken

	// A successful login leaves a real security notice for the user.
	rr := app.Do(http.MethodGet, "/admin/v1/notifications/unread-count", nil, token)
	var unread struct {
		Count int64 `json:"count"`
	}
	decodeInto(t, rr, &unread)
	require.GreaterOrEqual(t, unread.Count, int64(1))

	rr = app.Do(http.MethodGet, "/admin/v1/notifications?page=1&page_size=10", nil, token)
	var page struct {
		List  []model.Notification `json:"list"`
		Total int64                `json:"total"`
	}
	decodeInto(t, rr, &page)
	require.NotEmpty(t, page.List)
	assert.Equal(t, "新登录提醒", page.List[0].Title)

	rr = app.Do(http.MethodGet, "/admin/v1/notifications?unread=1", nil, token)
	var onlyUnread struct {
		List []model.Notification `json:"list"`
	}
	decodeInto(t, rr, &onlyUnread)
	for _, row := range onlyUnread.List {
		assert.Nil(t, row.ReadAt)
	}

	single := page.List[0].ID
	rr = app.Do(http.MethodPut, fmt.Sprintf("/admin/v1/notifications/%d/read", single), nil, token)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = app.Do(http.MethodPut, "/admin/v1/notifications/read-all", nil, token)
	var marked struct {
		Updated int64 `json:"updated"`
	}
	decodeInto(t, rr, &marked)
	assert.GreaterOrEqual(t, marked.Updated, int64(0))

	rr = app.Do(http.MethodGet, "/admin/v1/notifications/unread-count", nil, token)
	decodeInto(t, rr, &unread)
	assert.Zero(t, unread.Count)

	rr = app.Do(http.MethodPut, "/admin/v1/notifications/999999/read", nil, token)
	assert.Equal(t, http.StatusNotFound, rr.Code)

	rr = app.Do(http.MethodGet, "/admin/v1/notifications", nil, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
