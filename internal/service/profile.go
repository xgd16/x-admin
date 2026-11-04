package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/security"
	"x-admin/internal/pkg/totp"
)

// ProfileInput 是个人资料更新输入。
type ProfileInput struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
}

// ProfileService 实现个人中心：资料、密码、双重验证。
type ProfileService struct {
	users       UserRepository
	hasher      security.PasswordHasher
	passwordMin int
	issuer      string // TOTP 发行者名称，显示在验证器 App 中
	notifier    Notifier
}

// NewProfileService 创建个人中心服务。
// passwordMin < 8 时按 8 处理；issuer 为空时使用 "x-admin"。
func NewProfileService(users UserRepository, hasher security.PasswordHasher, passwordMin int, issuer string, notifier Notifier) *ProfileService {
	if passwordMin < 8 {
		passwordMin = 8
	}
	if trim(issuer) == "" {
		issuer = "x-admin"
	}
	return &ProfileService{users: users, hasher: hasher, passwordMin: passwordMin, issuer: trim(issuer), notifier: notifier}
}

// Get 返回用户资料。
func (s *ProfileService) Get(ctx context.Context, userID uint64) (*model.User, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

// Update 更新当前用户的个人资料。
func (s *ProfileService) Update(ctx context.Context, userID uint64, in ProfileInput) (*model.User, error) {
	if _, err := s.users.Get(ctx, userID); err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := ValidateEmail(trim(in.Email)); err != nil {
		return nil, err
	}
	if err := ValidatePhone(trim(in.Phone)); err != nil {
		return nil, err
	}
	// 只更新用户可编辑字段，避免把整行（密码、TOTP、session_version）写回旧值。
	if err := s.users.UpdateFields(ctx, userID, map[string]any{
		"nickname": trim(in.Nickname),
		"email":    trim(in.Email),
		"phone":    trim(in.Phone),
		"avatar":   trim(in.Avatar),
	}); err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	return s.Get(ctx, userID)
}

// ChangePassword 校验原密码后修改密码，并发送安全通知。
func (s *ProfileService) ChangePassword(ctx context.Context, userID uint64, oldPassword, newPassword string) error {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if !s.hasher.Verify(user.Password, oldPassword) {
		return apperr.New(apperr.CodeInvalidArgument, "原密码不正确")
	}
	if err := ValidatePassword(newPassword, s.passwordMin); err != nil {
		return err
	}
	if oldPassword == newPassword {
		return apperr.New(apperr.CodeInvalidArgument, "新密码不能与原密码相同")
	}
	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return apperr.Wrap(err, apperr.CodeInternal, "密码加密失败")
	}
	// 与其他安全操作保持一致：改密会递增 session_version，
	// 撤销其他设备已签发的 access / refresh token。
	if err := s.users.SetPassword(ctx, userID, hash, true); err != nil {
		return err
	}
	notify(ctx, s.notifier, NotificationInput{
		UserID: userID, Link: "/profile",
		Title: "密码修改成功", Type: model.NotificationSuccess,
		Content: "你的登录密码已更新，如非本人操作请立即联系管理员。",
	})
	return nil
}

// TOTPStatus 是个人中心展示的双重验证状态。
type TOTPStatus struct {
	Enabled           bool `json:"enabled"`
	RecoveryCodesLeft int  `json:"recovery_codes_left"`
}

// TOTPSetup 是绑定双重验证时返回给客户端的一次性信息。
// 密钥在用户输入有效动态码之前一直处于“待确认”状态。
type TOTPSetup struct {
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauth_url"`
	QRCode     string `json:"qr_code"`
	Issuer     string `json:"issuer"`
	Account    string `json:"account"`
}

// TOTPStatus 返回是否已开启双重验证及剩余恢复码数量。
func (s *ProfileService) TOTPStatus(ctx context.Context, userID uint64) (*TOTPStatus, error) {
	user, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &TOTPStatus{
		Enabled:           user.TOTPEnabled,
		RecoveryCodesLeft: recoveryCodeCount(user.TOTPRecoveryCodes),
	}, nil
}

// SetupTOTP 生成新的 TOTP 密钥并返回二维码。
// 写入数据库时显式把 totp_enabled 置为 false，防止覆盖已开启状态。
func (s *ProfileService) SetupTOTP(ctx context.Context, userID uint64) (*TOTPSetup, error) {
	user, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.TOTPEnabled {
		return nil, apperr.New(apperr.CodeConflict, "双重验证已启用，请先关闭后再重新绑定")
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "生成密钥失败")
	}
	if err := s.users.UpdateFields(ctx, userID, map[string]any{
		"totp_secret":         secret,
		"totp_enabled":        false,
		"totp_recovery_codes": "",
	}); err != nil {
		return nil, err
	}
	provisioning := totp.ProvisioningURL(s.issuer, user.Username, secret)
	qr, err := totp.QRCodeDataURL(provisioning)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "生成二维码失败")
	}
	return &TOTPSetup{
		Secret:     secret,
		OtpauthURL: provisioning,
		QRCode:     qr,
		Issuer:     s.issuer,
		Account:    user.Username,
	}, nil
}

// EnableTOTP 用密码和动态码确认绑定，正式开启双重验证并生成恢复码。
// 开启后会递增 session_version，使其他设备的 refresh token 失效。
func (s *ProfileService) EnableTOTP(ctx context.Context, userID uint64, password, code string) ([]string, error) {
	user, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.TOTPEnabled {
		return nil, apperr.New(apperr.CodeConflict, "双重验证已启用")
	}
	if !s.hasher.Verify(user.Password, password) {
		return nil, apperr.New(apperr.CodeInvalidArgument, "密码不正确")
	}
	if user.TOTPSecret == "" {
		return nil, apperr.New(apperr.CodeInvalidArgument, "请先生成二维码并扫码")
	}
	if !totp.Validate(user.TOTPSecret, code, time.Now()) {
		return nil, apperr.New(apperr.CodeInvalidArgument, "验证码不正确，请确认手机时间准确后重试")
	}
	codes, err := totp.GenerateRecoveryCodes()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "生成恢复码失败")
	}
	// 只保存恢复码的 SHA-256 摘要，明文仅在本次响应中返回一次。
	hashes := make([]string, len(codes))
	for i, recovery := range codes {
		hashes[i] = totp.HashRecoveryCode(recovery)
	}
	if err := s.users.UpdateFields(ctx, userID, map[string]any{
		"totp_enabled":        true,
		"totp_recovery_codes": encodeRecoveryHashes(hashes),
		"session_version":     user.SessionVersion + 1,
	}); err != nil {
		return nil, err
	}
	notify(ctx, s.notifier, NotificationInput{
		UserID: userID, Link: "/profile",
		Title: "双重验证已开启", Type: model.NotificationSuccess,
		Content: "Google Authenticator 验证已绑定，下次登录需要输入动态验证码。请妥善保存恢复码。",
	})
	return codes, nil
}

// DisableTOTP 用密码和动态码/恢复码关闭双重验证，并清空所有 TOTP 数据。
func (s *ProfileService) DisableTOTP(ctx context.Context, userID uint64, password, code string) error {
	user, err := s.Get(ctx, userID)
	if err != nil {
		return err
	}
	if !user.TOTPEnabled {
		return apperr.New(apperr.CodeConflict, "双重验证未启用")
	}
	if !s.hasher.Verify(user.Password, password) {
		return apperr.New(apperr.CodeInvalidArgument, "密码不正确")
	}
	valid := totp.Validate(user.TOTPSecret, code, time.Now())
	if !valid {
		used, err := consumeRecoveryCode(ctx, s.users, user, code)
		if err != nil {
			return err
		}
		valid = used
	}
	if !valid {
		return apperr.New(apperr.CodeInvalidArgument, "验证码或恢复码不正确")
	}
	if err := s.users.UpdateFields(ctx, userID, map[string]any{
		"totp_secret":         "",
		"totp_enabled":        false,
		"totp_recovery_codes": "",
	}); err != nil {
		return err
	}
	// 关闭双重验证属于降低安全等级的操作，撤销其他设备的旧令牌。
	if err := s.users.BumpSessionVersion(ctx, userID); err != nil {
		return err
	}
	notify(ctx, s.notifier, NotificationInput{
		UserID: userID, Link: "/profile",
		Title: "双重验证已关闭", Type: model.NotificationWarning,
		Content: "账号已恢复为仅密码登录。如非本人操作，请立即修改密码并重新开启双重验证。",
	})
	return nil
}

// recoveryCodeCount 返回剩余恢复码数量。
func recoveryCodeCount(stored string) int {
	return len(decodeRecoveryHashes(stored))
}

// decodeRecoveryHashes 解析 JSON 格式的恢复码摘要；损坏时按空处理。
func decodeRecoveryHashes(stored string) []string {
	if stored == "" {
		return nil
	}
	var hashes []string
	if err := json.Unmarshal([]byte(stored), &hashes); err != nil {
		return nil
	}
	return hashes
}

// encodeRecoveryHashes 把摘要列表序列化为 JSON；序列化失败时返回空数组。
func encodeRecoveryHashes(hashes []string) string {
	raw, err := json.Marshal(hashes)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// consumeRecoveryCode 校验并消费一个一次性恢复码。
// 使用常量时间比较避免时序侧信道，匹配后立即从存储中删除该码。
func consumeRecoveryCode(ctx context.Context, users UserRepository, user *model.User, code string) (bool, error) {
	normalized := totp.NormalizeRecoveryCode(code)
	if normalized == "" {
		return false, nil
	}
	hashes := decodeRecoveryHashes(user.TOTPRecoveryCodes)
	if len(hashes) == 0 {
		return false, nil
	}
	input := totp.HashRecoveryCode(normalized)
	matched := -1
	for i, hash := range hashes {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(input)) == 1 {
			matched = i
			break
		}
	}
	if matched < 0 {
		return false, nil
	}
	remaining := append(append([]string{}, hashes[:matched]...), hashes[matched+1:]...)
	if err := users.UpdateFields(ctx, user.ID, map[string]any{
		"totp_recovery_codes": encodeRecoveryHashes(remaining),
	}); err != nil {
		return false, err
	}
	// 同步内存中的用户对象，调用方后续判断能拿到最新状态。
	user.TOTPRecoveryCodes = encodeRecoveryHashes(remaining)
	return true, nil
}
