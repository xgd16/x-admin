package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// AuthHandler 处理登录、令牌刷新、登出以及 Passkey 登录入口。
type AuthHandler struct {
	auth     *service.AuthService
	captcha  *captcha.Generator
	passkeys *service.PasskeyService
}

// NewAuthHandler 创建认证处理器。
func NewAuthHandler(auth *service.AuthService, captcha *captcha.Generator, passkeys *service.PasskeyService) *AuthHandler {
	return &AuthHandler{auth: auth, captcha: captcha, passkeys: passkeys}
}

// Captcha 生成图形验证码，返回 captcha_id 与 SVG data URL。
func (h *AuthHandler) Captcha(c *gin.Context) {
	challenge, err := h.captcha.Generate(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, challenge)
}

// Login 校验用户名、密码和验证码；开启双重验证时返回 MFA 挑战。
func (h *AuthHandler) Login(c *gin.Context) {
	var in service.LoginInput
	if !Bind(c, &in) {
		return
	}
	result, err := h.auth.Login(c.Request.Context(), in, service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// LoginMFA 使用 TOTP 动态码或一次性恢复码完成二次验证。
func (h *AuthHandler) LoginMFA(c *gin.Context) {
	var in service.LoginMFAInput
	if !Bind(c, &in) {
		return
	}
	result, err := h.auth.LoginMFA(c.Request.Context(), in, service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// PasskeyLoginBegin 生成无用户名（discoverable）Passkey 登录参数。
func (h *AuthHandler) PasskeyLoginBegin(c *gin.Context) {
	options, err := h.passkeys.BeginLogin(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, options)
}

// PasskeyLoginFinish 校验 Passkey 断言并签发令牌。
func (h *AuthHandler) PasskeyLoginFinish(c *gin.Context) {
	var in struct {
		SessionID  string          `json:"session_id" binding:"required"`
		Credential json.RawMessage `json:"credential" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	user, err := h.passkeys.FinishLogin(c.Request.Context(), in.SessionID, in.Credential)
	if err != nil {
		response.Error(c, err)
		return
	}
	result, err := h.auth.LoginPasskey(c.Request.Context(), user, service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// Refresh 用 refresh token 换取新的令牌对；旧 token 会被立即作废。
func (h *AuthHandler) Refresh(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	pair, err := h.auth.Refresh(c.Request.Context(), in.RefreshToken)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, pair)
}

// Logout 删除服务端的 refresh token，使其无法再次使用。
func (h *AuthHandler) Logout(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.auth.Logout(c.Request.Context(), in.RefreshToken); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Me 返回当前用户资料、权限码和菜单树，前端启动时调用。
func (h *AuthHandler) Me(c *gin.Context) {
	me, err := h.auth.Me(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, me)
}
