package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"x-admin/internal/config"
)

// CORS 根据配置的来源白名单写入跨域响应头，并直接响应 OPTIONS 预检请求。
// 白名单包含 "*" 时允许任意来源，但不会再回显具体 Origin。
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.Origins))
	wildcard := false
	for _, origin := range cfg.Origins {
		if origin == "*" {
			wildcard = true
		}
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok || wildcard {
				if wildcard {
					origin = "*"
				}
				c.Header("Access-Control-Allow-Origin", origin)
				// 响应随 Origin 变化，缓存需要按该头区分。
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
				c.Header("Access-Control-Expose-Headers", "X-Request-ID,Content-Disposition")
				c.Header("Access-Control-Max-Age", "600")
			}
		}
		if strings.EqualFold(c.Request.Method, http.MethodOptions) {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
