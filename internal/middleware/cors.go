package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"x-admin/internal/config"
)

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
