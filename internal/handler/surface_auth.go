package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/response"
	"x-admin/internal/pkg/security"
	"x-admin/internal/service"
)

// SurfaceAuthHandler 是自定义 API 面的 JWT 认证处理器。
//
// 它同时实现 middleware.PrincipalLoader 与 middleware.SessionChecker：
// 生成的路由面把它传给 middleware.Auth 即可完成鉴权。
type SurfaceAuthHandler struct {
	auth   *service.SurfaceAuthService
	tokens *security.TokenManager
	scope  service.SurfaceScope
}

// NewSurfaceAuthHandler 创建自定义 API 面认证处理器。
func NewSurfaceAuthHandler(
	auth *service.SurfaceAuthService,
	tokens *security.TokenManager,
	scope service.SurfaceScope,
) *SurfaceAuthHandler {
	return &SurfaceAuthHandler{auth: auth, tokens: tokens, scope: scope}
}

// Register 注册账号并返回令牌。
func (h *SurfaceAuthHandler) Register(c *gin.Context) {
	var in service.SurfaceRegisterInput
	if !Bind(c, &in) {
		return
	}
	result, err := h.auth.Register(c.Request.Context(), h.scope, in, h.tokens, loginMeta(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, result)
}

// Login 校验账号密码并返回令牌。
func (h *SurfaceAuthHandler) Login(c *gin.Context) {
	var in service.SurfaceLoginInput
	if !Bind(c, &in) {
		return
	}
	result, err := h.auth.Login(c.Request.Context(), h.scope, in, h.tokens, loginMeta(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Refresh 轮换 refresh token。
func (h *SurfaceAuthHandler) Refresh(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	result, err := h.auth.Refresh(c.Request.Context(), h.scope, in.RefreshToken, h.tokens, loginMeta(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Logout 撤销当前 refresh token 与登录会话。
func (h *SurfaceAuthHandler) Logout(c *gin.Context) {
	p, ok := authn.FromContext(c.Request.Context())
	if !ok {
		response.Error(c, apperr.ErrUnauthorized)
		return
	}
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.auth.Logout(c.Request.Context(), h.scope, p.UserID, in.RefreshToken, p.SessionID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Me 返回当前登录用户资料。
func (h *SurfaceAuthHandler) Me(c *gin.Context) {
	p, ok := authn.FromContext(c.Request.Context())
	if !ok {
		response.Error(c, apperr.ErrUnauthorized)
		return
	}
	user, err := h.auth.Me(c.Request.Context(), h.scope, p.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// Authenticate 实现 middleware.PrincipalLoader。
func (h *SurfaceAuthHandler) Authenticate(ctx context.Context, userID uint64) (*authn.Principal, error) {
	return h.auth.Authenticate(ctx, h.scope, userID)
}

// SessionActive 实现 middleware.SessionChecker。
func (h *SurfaceAuthHandler) SessionActive(ctx context.Context, userID uint64, sessionID string) (bool, error) {
	return h.auth.SessionActive(ctx, h.scope, userID, sessionID)
}

func loginMeta(c *gin.Context) service.LoginMeta {
	return service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}
