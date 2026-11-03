// Package apperr 定义稳定的业务错误码与错误包装方式。
//
// 业务错误码与 HTTP 状态码解耦：客户端应以 code 判断错误类型，
// HTTP 状态码只用于网关和监控的粗略分类。
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code 是返回给客户端的稳定业务错误码。
type Code int

// 业务错误码取值，与前端约定的错误码保持一致。
const (
	CodeOK              Code = 0
	CodeInvalidArgument Code = 40001 // 参数错误
	CodeUnauthorized    Code = 40101 // 未登录
	CodeTokenExpired    Code = 40102 // 登录失效
	CodeForbidden       Code = 40301 // 无权限
	CodeNotFound        Code = 40401 // 不存在
	CodeConflict        Code = 40901 // 冲突
	CodeLocked          Code = 42301 // 已锁定
	CodeTooManyRequests Code = 42901 // 限流
	CodeInternal        Code = 50001 // 内部错误
)

// Error 携带业务码、可安全展示给用户的消息以及可选的底层原因。
type Error struct {
	Code    Code
	Message string
	cause   error
}

// Error 实现 error 接口；cause 只用于服务端日志，不直接返回给客户端。
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%d: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

// Unwrap 支持 errors.Is / errors.As 穿透到原始错误。
func (e *Error) Unwrap() error { return e.cause }

// New 创建不带底层原因的业务错误。
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf 创建格式化消息的业务错误。
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap 包装底层错误并附加业务码与安全消息。
func Wrap(cause error, code Code, message string) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

// From 提取业务错误；未知错误统一包装为内部错误，避免泄露实现细节。
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

// HTTPStatus 把业务码映射为 HTTP 状态码。
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

// 预置的通用业务错误，避免在调用处重复构造。
var (
	ErrUnauthorized = New(CodeUnauthorized, "请先登录")
	ErrForbidden    = New(CodeForbidden, "没有操作权限")
	ErrNotFound     = New(CodeNotFound, "数据不存在")
)
