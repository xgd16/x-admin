package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/response"
	"x-admin/internal/pkg/security"
	"x-admin/internal/service"
)

// AuthHandler 处理登录、令牌刷新、登出以及 Passkey 登录入口。
type AuthHandler struct {
	auth         *service.AuthService
	captcha      *captcha.Generator
	passkeys     *service.PasskeyService
	tokens       *security.TokenManager
	secureCookie bool
	fileTTL      time.Duration
}

// NewAuthHandler 创建认证处理器。
// secureCookie 在生产环境应为 true；fileTTL 控制 /uploads 访问凭证的有效期。
func NewAuthHandler(auth *service.AuthService, captcha *captcha.Generator, passkeys *service.PasskeyService,
	tokens *security.TokenManager, secureCookie bool, fileTTL time.Duration) *AuthHandler {
	if fileTTL <= 0 {
		fileTTL = 24 * time.Hour
	}
	return &AuthHandler{
		auth: auth, captcha: captcha, passkeys: passkeys,
		tokens: tokens, secureCookie: secureCookie, fileTTL: fileTTL,
	}
}

// setFileCookie 从 access token 中读取用户与会话版本，写入 /uploads 访问 Cookie。
func (h *AuthHandler) setFileCookie(c *gin.Context, accessToken string) {
	if h.tokens == nil || accessToken == "" {
		return
	}
	claims, err := h.tokens.Parse(accessToken)
	if err != nil {
		return
	}
	h.writeFileCookie(c, claims.UserID, claims.SessionVersion, claims.SessionID)
}

// writeFileCookie 写入 HttpOnly 文件访问 Cookie。
func (h *AuthHandler) writeFileCookie(c *gin.Context, userID uint64, sessionVersion int64, sessionID string) {
	if h.tokens == nil || userID == 0 || sessionID == "" {
		return
	}
	value := h.tokens.IssueFileAccess(userID, sessionVersion, sessionID, h.fileTTL)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(security.FileAccessCookie, value, int(h.fileTTL.Seconds()), "/uploads", "", h.cookieSecure(c), true)
}

// clearFileCookie 清除文件访问 Cookie。
func (h *AuthHandler) clearFileCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(security.FileAccessCookie, "", -1, "/uploads", "", h.cookieSecure(c), true)
}

// cookieSecure 只在生产环境且请求确实走 HTTPS 时设置 Secure，
// 避免默认的 HTTP 部署（http://localhost）无法保存 Cookie。
func (h *AuthHandler) cookieSecure(c *gin.Context) bool {
	if !h.secureCookie {
		return false
	}
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
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
	if result.TokenPair != nil && result.AccessToken != "" {
		h.setFileCookie(c, result.AccessToken)
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
	if result.TokenPair != nil && result.AccessToken != "" {
		h.setFileCookie(c, result.AccessToken)
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
		RememberMe bool            `json:"remember_me"`
	}
	if !Bind(c, &in) {
		return
	}
	user, err := h.passkeys.FinishLogin(c.Request.Context(), in.SessionID, in.Credential)
	if err != nil {
		response.Error(c, err)
		return
	}
	result, err := h.auth.LoginPasskey(c.Request.Context(), user,
		service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}, in.RememberMe)
	if err != nil {
		response.Error(c, err)
		return
	}
	if result.TokenPair != nil && result.AccessToken != "" {
		h.setFileCookie(c, result.AccessToken)
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
	pair, err := h.auth.Refresh(c.Request.Context(), in.RefreshToken,
		service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		response.Error(c, err)
		return
	}
	h.setFileCookie(c, pair.AccessToken)
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
	if err := h.auth.Logout(c.Request.Context(), CurrentUserID(c), in.RefreshToken, CurrentSessionID(c)); err != nil {
		response.Error(c, err)
		return
	}
	h.clearFileCookie(c)
	response.OK(c, nil)
}

// Me 返回当前用户资料、权限码和菜单树，前端启动时调用。
func (h *AuthHandler) Me(c *gin.Context) {
	me, err := h.auth.Me(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	// 页面刷新后重新下发文件访问 Cookie，避免旧 Cookie 到期导致图片全部 401。
	h.writeFileCookie(c, me.User.ID, me.User.SessionVersion, CurrentSessionID(c))
	response.OK(c, me)
}

// Unlock 校验锁屏解锁密码。
func (h *AuthHandler) Unlock(c *gin.Context) {
	var in struct {
		Password string `json:"password" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.auth.Unlock(c.Request.Context(), CurrentUserID(c), in.Password); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// UnlockPasskeyBegin 生成绑定当前用户的锁屏解锁参数。
func (h *AuthHandler) UnlockPasskeyBegin(c *gin.Context) {
	options, err := h.passkeys.BeginUnlock(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, options)
}

// UnlockPasskeyFinish 校验锁屏解锁断言。
func (h *AuthHandler) UnlockPasskeyFinish(c *gin.Context) {
	var in struct {
		SessionID  string          `json:"session_id" binding:"required"`
		Credential json.RawMessage `json:"credential" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.passkeys.FinishUnlock(c.Request.Context(), CurrentUserID(c), in.SessionID, in.Credential); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
