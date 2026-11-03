package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/requestid"
)

// RequestID 复用上游传入的请求 ID，缺失或过长时生成新的 ID。
// ID 同时写入响应头和请求上下文，方便日志与审计关联。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(requestid.Header))
		if id == "" || len(id) > 64 {
			id = requestid.New()
		}
		c.Header(requestid.Header, id)
		c.Request = c.Request.WithContext(requestid.With(c.Request.Context(), id))
		c.Next()
	}
}
