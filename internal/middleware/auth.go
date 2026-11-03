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

type PrincipalLoader interface {
	Authenticate(ctx context.Context, userID uint64) (*authn.Principal, error)
}

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
		principal, err := loader.Authenticate(c.Request.Context(), claims.UserID)
		if err != nil {
			response.Error(c, err)
			return
		}
		c.Request = c.Request.WithContext(authn.With(c.Request.Context(), principal))
		c.Next()
	}
}

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
