package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/response"
)

// RateLimit uses a fixed window counter. Store failures fail open so a Redis
// outage degrades protection instead of taking the API down.
func RateLimit(store cache.Store, prefix string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		key := prefix + ":" + c.ClientIP()
		n, err := store.Incr(c.Request.Context(), key, window)
		if err != nil {
			slog.Warn("rate limit store error", "error", err, "key", key)
			c.Next()
			return
		}
		remaining := int64(limit) - n
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		if n > int64(limit) {
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			response.Error(c, apperr.New(apperr.CodeTooManyRequests, "请求过于频繁，请稍后再试"))
			return
		}
		c.Next()
	}
}
