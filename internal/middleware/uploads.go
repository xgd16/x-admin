package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// UploadSecurityHeaders 为 /uploads 静态文件补上安全响应头。
//
// 上传目录可能包含 HTML / SVG 等主动内容，直接以同源文档打开时存在存储型
// XSS 风险。CSP sandbox 会阻止脚本执行，nosniff 阻止浏览器按内容猜测类型。
func UploadSecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/uploads" || strings.HasPrefix(c.Request.URL.Path, "/uploads/") {
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
			c.Header("Cross-Origin-Resource-Policy", "same-origin")
		}
		c.Next()
	}
}
