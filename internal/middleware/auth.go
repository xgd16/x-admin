// Package middleware 提供 HTTP 中间件：请求 ID、恢复、访问日志、
// 跨域、鉴权、权限校验、限流与操作审计。
package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/response"
	"x-admin/internal/pkg/security"
)

// PrincipalLoader 根据用户 ID 加载当前身份与权限，
// 由 AuthService 实现，避免中间件直接依赖业务服务。
type PrincipalLoader interface {
	Authenticate(ctx context.Context, userID uint64) (*authn.Principal, error)
}

// Auth 校验 Authorization: Bearer <token>，解析 JWT 并把 Principal 写入请求上下文。
// 令牌缺失、无效或过期时分别返回对应的业务错误码。
func Auth(tokens *security.TokenManager, loader PrincipalLoader) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, apperr.ErrUnauthorized)
			return
		}
		claims, err := tokens.Parse(parts[1])
		if err != nil {
			if errors.Is(err, security.ErrExpiredToken) {
				response.Error(c, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录"))
				return
			}
			response.Error(c, apperr.New(apperr.CodeUnauthorized, "登录凭证无效"))
			return
		}
		// 每次请求都重新加载身份，确保用户停用、角色变更立即生效。
		principal, err := loader.Authenticate(c.Request.Context(), claims.UserID)
		if err != nil {
			response.Error(c, err)
			return
		}
		c.Request = c.Request.WithContext(authn.With(c.Request.Context(), principal))
		c.Next()
	}
}

// Require 要求当前用户拥有指定权限码；super_admin 在 Principal.Has 中直接放行。
func Require(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := authn.FromContext(c.Request.Context())
		if !ok {
			response.Error(c, apperr.ErrUnauthorized)
			return
		}
		if !p.Has(permission) {
			response.Error(c, apperr.ErrForbidden)
			return
		}
		c.Next()
	}
}
