package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// ProfileHandler 处理个人中心：资料、密码、双重验证与通行密钥。
type ProfileHandler struct {
	profile  *service.ProfileService
	passkeys *service.PasskeyService
	auth     *service.AuthService
}

// NewProfileHandler 创建个人中心处理器。
func NewProfileHandler(profile *service.ProfileService, passkeys *service.PasskeyService, auth *service.AuthService) *ProfileHandler {
	return &ProfileHandler{profile: profile, passkeys: passkeys, auth: auth}
}

// Get 返回当前用户资料。
func (h *ProfileHandler) Get(c *gin.Context) {
	user, err := h.profile.Get(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// Update 更新当前用户的昵称、邮箱、手机号和头像。
func (h *ProfileHandler) Update(c *gin.Context) {
	var in service.ProfileInput
	if !Bind(c, &in) {
		return
	}
	user, err := h.profile.Update(c.Request.Context(), CurrentUserID(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// ChangePassword 校验原密码后修改当前用户密码。
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	var in struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.profile.ChangePassword(c.Request.Context(), CurrentUserID(c), in.OldPassword, in.NewPassword); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// TOTPStatus 返回双重验证是否开启以及剩余恢复码数量。
func (h *ProfileHandler) TOTPStatus(c *gin.Context) {
	status, err := h.profile.TOTPStatus(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, status)
}

// SetupTOTP 生成待绑定的 TOTP 密钥、otpauth 链接和二维码。
// 此时只是写入待确认密钥，尚未开启双重验证。
func (h *ProfileHandler) SetupTOTP(c *gin.Context) {
	setup, err := h.profile.SetupTOTP(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, setup)
}

// EnableTOTP 校验密码和动态验证码后开启双重验证，并返回一次性恢复码。
func (h *ProfileHandler) EnableTOTP(c *gin.Context) {
	var in struct {
		Password     string `json:"password" binding:"required"`
		Code         string `json:"otp_code" binding:"required"`
		RefreshToken string `json:"refresh_token"`
	}
	if !Bind(c, &in) {
		return
	}
	codes, err := h.profile.EnableTOTP(c.Request.Context(), CurrentUserID(c), in.Password, in.Code)
	if err != nil {
		response.Error(c, err)
		return
	}
	// 开启双重验证会递增 session_version，导致其他设备的 refresh token 失效。
	// 这里把当前设备的 token 重新指向新版本，避免操作者本人被登出。
	h.auth.RefreshSession(c.Request.Context(), CurrentUserID(c), in.RefreshToken)
	response.OK(c, map[string]any{"recovery_codes": codes})
}

// DisableTOTP 校验密码和动态码/恢复码后关闭双重验证。
func (h *ProfileHandler) DisableTOTP(c *gin.Context) {
	var in struct {
		Password string `json:"password" binding:"required"`
		Code     string `json:"otp_code" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.profile.DisableTOTP(c.Request.Context(), CurrentUserID(c), in.Password, in.Code); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// ListPasskeys 返回当前用户已绑定的 Passkey 设备。
func (h *ProfileHandler) ListPasskeys(c *gin.Context) {
	rows, err := h.passkeys.List(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, rows)
}

// BeginPasskey 校验当前密码后生成注册参数。
func (h *ProfileHandler) BeginPasskey(c *gin.Context) {
	var in struct {
		Password string `json:"password" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	options, err := h.passkeys.BeginRegistration(c.Request.Context(), CurrentUserID(c), in.Password)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, options)
}

// FinishPasskey 校验认证器 attestation 并保存设备公钥。
func (h *ProfileHandler) FinishPasskey(c *gin.Context) {
	var in struct {
		SessionID  string          `json:"session_id" binding:"required"`
		Name       string          `json:"name"`
		Credential json.RawMessage `json:"credential" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	row, err := h.passkeys.FinishRegistration(c.Request.Context(), CurrentUserID(c), in.SessionID, in.Name, in.Credential)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, row)
}

// RenamePasskey 修改当前用户某台设备的名称。
func (h *ProfileHandler) RenamePasskey(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.passkeys.Rename(c.Request.Context(), CurrentUserID(c), id, in.Name); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// DeletePasskey 解绑当前用户的一台设备。
func (h *ProfileHandler) DeletePasskey(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.passkeys.Delete(c.Request.Context(), CurrentUserID(c), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
