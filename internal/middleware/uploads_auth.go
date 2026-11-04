package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/response"
	"x-admin/internal/pkg/security"
)

// FileAccessChecker 校验文件访问凭证对应的账号状态与会话有效性。
type FileAccessChecker interface {
	SessionChecker
	CheckFileAccess(ctx context.Context, userID uint64, sessionVersion int64, sessionID string) error
}

// ShareResolver 校验可撤销分享令牌并返回其对应的文件路径。
type ShareResolver interface {
	ResolveShare(ctx context.Context, token string) (string, error)
}

// UploadAuth 保护 /uploads 静态文件。
//
// 浏览器通过登录时下发的 HttpOnly Cookie 访问，<img> 等标签无需额外改造；
// 非浏览器客户端可以继续使用 Authorization: Bearer <access token>。
func UploadAuth(tokens *security.TokenManager, checker FileAccessChecker, shares ShareResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path != "/uploads" && !strings.HasPrefix(c.Request.URL.Path, "/uploads/") {
			c.Next()
			return
		}
		if checker == nil {
			response.Error(c, apperr.New(apperr.CodeUnauthorized, "文件访问未配置鉴权"))
			return
		}

		// 分享链接使用数据库令牌，撤销后立即失效；带 share 参数时不再回退登录态。
		if token := c.Query("share"); token != "" {
			if shares == nil {
				response.Error(c, apperr.New(apperr.CodeUnauthorized, "分享链接无效"))
				return
			}
			path, err := shares.ResolveShare(c.Request.Context(), token)
			if err != nil {
				response.Error(c, err)
				return
			}
			if path != c.Request.URL.Path {
				response.Error(c, apperr.New(apperr.CodeUnauthorized, "分享链接与文件不匹配"))
				return
			}
			// 分享链接可能随时被撤销，禁止浏览器/代理缓存响应。
			c.Header("Cache-Control", "private, no-store")
			c.Next()
			return
		}

		// API 客户端：直接使用 Bearer token。
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if parts := strings.Fields(header); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			claims, err := tokens.Parse(parts[1])
			if err != nil {
				c.Header("Cache-Control", "no-store")
				response.Error(c, apperr.New(apperr.CodeUnauthorized, "登录凭证无效"))
				return
			}
			if err := checker.CheckFileAccess(c.Request.Context(), claims.UserID, claims.SessionVersion, claims.SessionID); err != nil {
				c.Header("Cache-Control", "no-store")
				response.Error(c, err)
				return
			}
			c.Next()
			return
		}

		// 浏览器：使用登录时下发的 HttpOnly 文件访问 Cookie。
		raw, err := c.Cookie(security.FileAccessCookie)
		if err != nil || raw == "" {
			c.Header("Cache-Control", "no-store")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": apperr.CodeUnauthorized, "message": "请先登录"})
			return
		}
		userID, version, sessionID, err := tokens.ParseFileAccess(raw)
		if err != nil {
			c.Header("Cache-Control", "no-store")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": apperr.CodeTokenExpired, "message": "文件访问凭证已过期"})
			return
		}
		if err := checker.CheckFileAccess(c.Request.Context(), userID, version, sessionID); err != nil {
			c.Header("Cache-Control", "no-store")
			response.Error(c, err)
			return
		}
		c.Next()
	}
}
