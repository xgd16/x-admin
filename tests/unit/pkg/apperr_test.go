package pkg_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/apperr"
)

func TestErrorBasics(t *testing.T) {
	cause := errors.New("boom")
	err := apperr.Wrap(cause, apperr.CodeConflict, "冲突")
	assert.Contains(t, err.Error(), "冲突")
	assert.Contains(t, err.Error(), "boom")
	assert.Equal(t, cause, errors.Unwrap(err))
	assert.Equal(t, apperr.CodeConflict, err.Code)

	plain := apperr.New(apperr.CodeNotFound, "找不到")
	assert.Equal(t, "40401: 找不到", plain.Error())
	assert.Nil(t, plain.Unwrap())

	formatted := apperr.Newf(apperr.CodeInvalidArgument, "第 %d 行", 3)
	assert.Equal(t, "第 3 行", formatted.Message)
}

func TestFrom(t *testing.T) {
	assert.Nil(t, apperr.From(nil))

	direct := apperr.New(apperr.CodeForbidden, "无权限")
	assert.Same(t, direct, apperr.From(direct))

	wrapped := errors.Join(errors.New("other"), direct)
	assert.Same(t, direct, apperr.From(wrapped))

	unknown := apperr.From(errors.New("db down"))
	assert.Equal(t, apperr.CodeInternal, unknown.Code)
	assert.Equal(t, "服务器内部错误", unknown.Message)
}

func TestHTTPStatus(t *testing.T) {
	cases := map[apperr.Code]int{
		apperr.CodeOK:              http.StatusInternalServerError,
		apperr.CodeInvalidArgument: http.StatusBadRequest,
		apperr.CodeUnauthorized:    http.StatusUnauthorized,
		apperr.CodeTokenExpired:    http.StatusUnauthorized,
		apperr.CodeForbidden:       http.StatusForbidden,
		apperr.CodeNotFound:        http.StatusNotFound,
		apperr.CodeConflict:        http.StatusConflict,
		apperr.CodeLocked:          http.StatusLocked,
		apperr.CodeTooManyRequests: http.StatusTooManyRequests,
		apperr.CodeInternal:        http.StatusInternalServerError,
		apperr.Code(999):           http.StatusInternalServerError,
	}
	for code, want := range cases {
		assert.Equalf(t, want, apperr.HTTPStatus(code), "code %d", code)
	}
}

func TestSentinels(t *testing.T) {
	require.Equal(t, apperr.CodeUnauthorized, apperr.ErrUnauthorized.Code)
	require.Equal(t, apperr.CodeForbidden, apperr.ErrForbidden.Code)
	require.Equal(t, apperr.CodeNotFound, apperr.ErrNotFound.Code)
}
