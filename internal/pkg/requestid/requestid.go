// Package requestid 生成并传递请求 ID，用于串联访问日志、审计记录和错误响应。
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// Header 是请求 ID 使用的 HTTP 头。
const Header = "X-Request-ID"

// contextKey 是私有上下文键，避免冲突。
type contextKey struct{}

// New 返回 16 字节的随机请求 ID；随机源故障时返回全零 ID。
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}

// With 把请求 ID 写入上下文。
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext 读取请求 ID；不存在时返回空串。
func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}
