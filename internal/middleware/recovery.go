package middleware

import (
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/requestid"
	"x-admin/internal/pkg/response"
)

// Recovery 捕获处理链中的 panic，记录堆栈并返回统一的 500 响应。
// 如果响应已经写出，则只中止后续处理，避免重复写响应头。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"request_id", requestid.FromContext(c.Request.Context()),
					"method", c.Request.Method, "path", c.Request.URL.Path,
					"panic", rec, "stack", string(debug.Stack()))
				if !c.Writer.Written() {
					response.Error(c, apperr.New(apperr.CodeInternal, "服务器内部错误"))
					return
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

// AccessLog 在请求结束后输出一条结构化访问日志，包含请求 ID、状态码和耗时。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"request_id", requestid.FromContext(c.Request.Context()),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}
