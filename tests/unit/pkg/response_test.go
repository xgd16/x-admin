package pkg_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/requestid"
	"x-admin/internal/pkg/response"
)

func newContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = c.Request.WithContext(requestid.With(c.Request.Context(), "req-1"))
	return c, rr
}

func TestOKAndPage(t *testing.T) {
	c, rr := newContext()
	response.OK(c, map[string]string{"hello": "world"})
	assert.Equal(t, http.StatusOK, rr.Code)
	var env response.Envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	assert.Equal(t, 0, env.Code)
	assert.Equal(t, "req-1", env.RequestID)

	c, rr = newContext()
	response.Page(c, []string{"a"}, 1, 2, 10)
	var page struct {
		Data response.PageData `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.Data.Total)
	assert.Equal(t, 2, page.Data.Page)
	assert.Equal(t, 10, page.Data.PageSize)
}

func TestCreatedAndNoContent(t *testing.T) {
	c, rr := newContext()
	response.Created(c, 1)
	assert.Equal(t, http.StatusCreated, rr.Code)

	c, rr = newContext()
	response.NoContent(c)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestError(t *testing.T) {
	c, rr := newContext()
	response.Error(c, apperr.New(apperr.CodeNotFound, "用户不存在"))
	assert.Equal(t, http.StatusNotFound, rr.Code)
	var env response.Envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	assert.Equal(t, int(apperr.CodeNotFound), env.Code)
	assert.Equal(t, "用户不存在", env.Message)
	assert.True(t, c.IsAborted())

	c, rr = newContext()
	response.Error(c, errors.New("secret database details"))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	assert.Equal(t, "服务器内部错误", env.Message)
	assert.NotContains(t, rr.Body.String(), "secret database")

	c, _ = newContext()
	response.Error(c, apperr.New(apperr.CodeOK, "zero"))
}
