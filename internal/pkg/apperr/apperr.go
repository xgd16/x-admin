package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is the stable business error code returned to clients.
type Code int

const (
	CodeOK              Code = 0
	CodeInvalidArgument Code = 40001
	CodeUnauthorized    Code = 40101
	CodeTokenExpired    Code = 40102
	CodeForbidden       Code = 40301
	CodeNotFound        Code = 40401
	CodeConflict        Code = 40901
	CodeLocked          Code = 42301
	CodeTooManyRequests Code = 42901
	CodeInternal        Code = 50001
)

// Error carries a business code, a safe message and an optional cause.
type Error struct {
	Code    Code
	Message string
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%d: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func Wrap(cause error, code Code, message string) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

// From extracts a business error or wraps an unknown error as internal.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var target *Error
	if errors.As(err, &target) {
		return target
	}
	return Wrap(err, CodeInternal, "服务器内部错误")
}

// HTTPStatus maps a business code to an HTTP status code.
func HTTPStatus(code Code) int {
	switch code {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeUnauthorized, CodeTokenExpired:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeLocked:
		return http.StatusLocked
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

var (
	ErrUnauthorized = New(CodeUnauthorized, "请先登录")
	ErrForbidden    = New(CodeForbidden, "没有操作权限")
	ErrNotFound     = New(CodeNotFound, "数据不存在")
)
