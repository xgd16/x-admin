package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/requestid"
)

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
