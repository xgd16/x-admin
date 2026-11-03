package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/security"
)

const (
	maxPasskeysPerUser = 10 // 单个用户最多绑定的设备数
	passkeyNameLimit   = 64 // 设备名称最大字符数
)

// 注册与登录挑战在缓存中的键前缀。
const (
	passkeyRegisterPrefix = "passkey:register:"
	passkeyLoginPrefix    = "passkey:login:"
)

// PasskeyRegistrationOptions 是用户开始添加设备时返回的注册参数。
type PasskeyRegistrationOptions struct {
	SessionID string                       `json:"session_id"`
	Options   *protocol.CredentialCreation `json:"options"`
}

// PasskeyLoginOptions 是登录页发起无用户名（discoverable）Passkey 流程时返回的参数。
type PasskeyLoginOptions struct {
	SessionID string                        `json:"session_id"`
	Options   *protocol.CredentialAssertion `json:"options"`
}

// passkeyCeremony 是存放在缓存中的一次 WebAuthn 流程状态。
// 注册流程会带上 UserID，登录流程为 0（由认证器返回的用户句柄定位用户）。
type passkeyCeremony struct {
	UserID  uint64               `json:"user_id,omitempty"`
	Session webauthn.SessionData `json:"session"`
}

// PasskeyService 负责 WebAuthn 的注册、登录和设备管理。
// wa 为 nil 时表示 Passkey 功能被禁用，其余功能不受影响。
type PasskeyService struct {
	users    UserRepository
	passkeys PasskeyRepository
	hasher   security.PasswordHasher
	store    cache.Store
	wa       *webauthn.WebAuthn
	ttl      time.Duration
	notifier Notifier
}

// NewPasskeyService 创建 Passkey 服务。ttl <= 0 时挑战有效期默认 5 分钟。
func NewPasskeyService(
	users UserRepository,
	passkeys PasskeyRepository,
	hasher security.PasswordHasher,
	store cache.Store,
	wa *webauthn.WebAuthn,
	ttl time.Duration,
	notifier Notifier,
) *PasskeyService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &PasskeyService{
		users: users, passkeys: passkeys, hasher: hasher, store: store,
		wa: wa, ttl: ttl, notifier: notifier,
	}
}

// List 返回用户已绑定的设备列表；没有设备时返回空数组而不是 null。
func (s *PasskeyService) List(ctx context.Context, userID uint64) ([]model.Passkey, error) {
	rows, err := s.passkeys.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.Passkey{}
	}
	return rows, nil
}

// BeginRegistration 校验当前密码后生成 Passkey 注册参数。
// 要求认证器支持 resident key（可发现凭据）和用户验证，
// 并把已绑定的凭据加入排除列表，避免同一设备重复注册。
func (s *PasskeyService) BeginRegistration(ctx context.Context, userID uint64, password string) (*PasskeyRegistrationOptions, error) {
	if s.wa == nil {
		return nil, apperr.New(apperr.CodeForbidden, "通行密钥未启用，请联系管理员")
	}
	user, waUser, err := s.loadWAUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !s.hasher.Verify(user.Password, password) {
		return nil, apperr.New(apperr.CodeInvalidArgument, "密码不正确")
	}
	if count, err := s.passkeys.CountByUser(ctx, userID); err != nil {
		return nil, err
	} else if count >= maxPasskeysPerUser {
		return nil, apperr.Newf(apperr.CodeConflict, "最多绑定 %d 个通行密钥，请先删除不用的设备", maxPasskeysPerUser)
	}

	exclusions := make([]protocol.CredentialDescriptor, 0, len(waUser.credentials))
	for i := range waUser.credentials {
		exclusions = append(exclusions, waUser.credentials[i].Descriptor())
	}
	creation, session, err := s.wa.BeginRegistration(waUser,
		webauthn.WithExclusions(exclusions),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return nil, passkeyFailure("生成绑定参数失败", err, apperr.CodeInvalidArgument)
	}
	sessionID, err := s.saveCeremony(ctx, passkeyRegisterPrefix, userID, session)
	if err != nil {
		return nil, err
	}
	return &PasskeyRegistrationOptions{SessionID: sessionID, Options: creation}, nil
}

// FinishRegistration 校验认证器的 attestation 并保存设备公钥。
// 会话一次性使用，且必须与开始注册的用户一致。
func (s *PasskeyService) FinishRegistration(ctx context.Context, userID uint64, sessionID, name string, credentialJSON json.RawMessage) (*model.Passkey, error) {
	if s.wa == nil {
		return nil, apperr.New(apperr.CodeForbidden, "通行密钥未启用，请联系管理员")
	}
	ceremony, err := s.takeCeremony(ctx, passkeyRegisterPrefix, sessionID)
	if err != nil {
		return nil, err
	}
	if ceremony.UserID != userID {
		return nil, apperr.New(apperr.CodeInvalidArgument, "绑定会话不匹配，请重新开始")
	}
	_, waUser, err := s.loadWAUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	registered, err := s.wa.FinishRegistration(waUser, ceremony.Session, credentialRequest(credentialJSON))
	if err != nil {
		return nil, passkeyFailure("通行密钥绑定失败，请重试", err, apperr.CodeInvalidArgument)
	}
	encodedID := base64.RawURLEncoding.EncodeToString(registered.ID)
	// 注册参数中的排除列表只是最佳努力，这里再做一次兜底唯一性检查。
	if _, err := s.passkeys.GetByCredentialID(ctx, encodedID); err == nil {
		return nil, apperr.New(apperr.CodeConflict, "该设备已经绑定过了")
	}
	raw, err := json.Marshal(registered)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "保存通行密钥失败")
	}
	row := &model.Passkey{
		UserID:       userID,
		CredentialID: encodedID,
		Name:         normalizePasskeyName(name),
		Credential:   string(raw),
	}
	if err := s.passkeys.Create(ctx, row); err != nil {
		return nil, err
	}
	notify(ctx, s.notifier, NotificationInput{
		UserID: userID, Link: "/profile",
		Title: "通行密钥已添加", Type: model.NotificationSuccess,
		Content: fmt.Sprintf("设备「%s」已绑定通行密钥，可用于免密码登录。", row.Name),
	})
	return row, nil
}

// BeginLogin 生成无用户名（discoverable credential）登录参数。
// 全站没有任何 Passkey 时直接返回 404，避免浏览器弹出无意义的系统提示。
func (s *PasskeyService) BeginLogin(ctx context.Context) (*PasskeyLoginOptions, error) {
	if s.wa == nil {
		return nil, apperr.New(apperr.CodeForbidden, "通行密钥未启用，请联系管理员")
	}
	count, err := s.passkeys.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, apperr.New(apperr.CodeNotFound, "还没有绑定通行密钥")
	}
	assertion, session, err := s.wa.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, passkeyFailure("生成登录参数失败", err, apperr.CodeInvalidArgument)
	}
	sessionID, err := s.saveCeremony(ctx, passkeyLoginPrefix, 0, session)
	if err != nil {
		return nil, err
	}
	return &PasskeyLoginOptions{SessionID: sessionID, Options: assertion}, nil
}

// FinishLogin 校验 discoverable assertion 并返回已认证用户。
// 认证器返回的签名计数器会被持久化，用于检测克隆的认证器。
func (s *PasskeyService) FinishLogin(ctx context.Context, sessionID string, credentialJSON json.RawMessage) (*model.User, error) {
	if s.wa == nil {
		return nil, apperr.New(apperr.CodeForbidden, "通行密钥未启用，请联系管理员")
	}
	ceremony, err := s.takeCeremony(ctx, passkeyLoginPrefix, sessionID)
	if err != nil {
		return nil, err
	}
	// handler 通过凭据 ID 找到记录，再加载对应用户；用户句柄必须存在。
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		if len(userHandle) == 0 {
			return nil, errors.New("missing user handle")
		}
		row, err := s.passkeys.GetByCredentialID(ctx, base64.RawURLEncoding.EncodeToString(rawID))
		if err != nil {
			return nil, err
		}
		_, waUser, err := s.loadWAUser(ctx, row.UserID)
		if err != nil {
			return nil, err
		}
		return waUser, nil
	}
	authenticatedUser, updated, err := s.wa.FinishPasskeyLogin(handler, ceremony.Session, credentialRequest(credentialJSON))
	if err != nil {
		return nil, passkeyFailure("通行密钥验证失败，请重试", err, apperr.CodeUnauthorized)
	}
	owner, ok := authenticatedUser.(*waUser)
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "通行密钥验证失败，请重试")
	}
	// 再次确认凭据确实属于认证器声明的用户，防止越权登录。
	row, err := s.passkeys.GetByCredentialID(ctx, base64.RawURLEncoding.EncodeToString(updated.ID))
	if err != nil || row.UserID != owner.userID {
		return nil, apperr.New(apperr.CodeUnauthorized, "通行密钥验证失败，请重试")
	}
	raw, err := json.Marshal(updated)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "保存通行密钥失败")
	}
	now := time.Now()
	if err := s.passkeys.UpdateCredential(ctx, row.ID, string(raw), now); err != nil {
		return nil, err
	}
	return s.users.Get(ctx, owner.userID)
}

// Rename 修改设备名称。
func (s *PasskeyService) Rename(ctx context.Context, userID, id uint64, name string) error {
	name = normalizePasskeyName(name)
	if name == "" {
		return apperr.New(apperr.CodeInvalidArgument, "请输入设备名称")
	}
	if err := s.passkeys.RenameForUser(ctx, userID, id, name); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return nil
}

// Delete 解绑设备并发送安全通知。
func (s *PasskeyService) Delete(ctx context.Context, userID, id uint64) error {
	row, err := s.passkeys.GetForUser(ctx, userID, id)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if err := s.passkeys.DeleteForUser(ctx, userID, id); err != nil {
		return err
	}
	notify(ctx, s.notifier, NotificationInput{
		UserID: userID, Link: "/profile",
		Title: "通行密钥已移除", Type: model.NotificationWarning,
		Content: fmt.Sprintf("设备「%s」的通行密钥已删除，将无法再用该设备免密码登录。", row.Name),
	})
	return nil
}

// loadWAUser 把用户与其凭据转换成 go-webauthn 需要的形式。
// 单条凭据损坏时跳过并记录日志，不影响其他设备登录。
func (s *PasskeyService) loadWAUser(ctx context.Context, userID uint64) (*model.User, *waUser, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil, apperr.ErrNotFound
		}
		return nil, nil, err
	}
	if user.Status != model.StatusEnabled {
		return nil, nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	rows, err := s.passkeys.ListByUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	credentials := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		var credential webauthn.Credential
		if err := json.Unmarshal([]byte(row.Credential), &credential); err != nil {
			slog.Warn("skip corrupt passkey credential", "id", row.ID, "error", err)
			continue
		}
		credentials = append(credentials, credential)
	}
	return user, newWAUser(user, credentials), nil
}

// saveCeremony 生成一次性会话 ID，把 WebAuthn 流程状态写入缓存并返回 ID。
func (s *PasskeyService) saveCeremony(ctx context.Context, prefix string, userID uint64, session *webauthn.SessionData) (string, error) {
	id, err := security.RandomToken()
	if err != nil {
		return "", apperr.Wrap(err, apperr.CodeInternal, "创建验证会话失败")
	}
	payload, err := json.Marshal(passkeyCeremony{UserID: userID, Session: *session})
	if err != nil {
		return "", apperr.Wrap(err, apperr.CodeInternal, "创建验证会话失败")
	}
	if err := s.store.Set(ctx, prefix+id, string(payload), s.ttl); err != nil {
		return "", apperr.Wrap(err, apperr.CodeInternal, "创建验证会话失败")
	}
	return id, nil
}

// takeCeremony 原子读取并删除会话，保证同一会话只能完成一次流程。
func (s *PasskeyService) takeCeremony(ctx context.Context, prefix, sessionID string) (*passkeyCeremony, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, apperr.New(apperr.CodeInvalidArgument, "缺少验证会话")
	}
	raw, err := s.store.Take(ctx, prefix+sessionID)
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			return nil, apperr.New(apperr.CodeInvalidArgument, "操作已超时，请重新开始")
		}
		return nil, err
	}
	var ceremony passkeyCeremony
	if err := json.Unmarshal([]byte(raw), &ceremony); err != nil {
		return nil, apperr.New(apperr.CodeInvalidArgument, "验证会话无效，请重新开始")
	}
	return &ceremony, nil
}

// passkeyFailure 把 WebAuthn 协议错误转换为对用户友好的提示。
// 常见问题是访问地址与 RP ID / Origins 配置不一致，这里会给出明确指引。
func passkeyFailure(message string, err error, code apperr.Code) error {
	slog.Warn("passkey ceremony failed", "error", err)
	var protocolErr *protocol.Error
	if errors.As(err, &protocolErr) {
		if protocolErr.DevInfo != "" {
			slog.Warn("passkey protocol detail", "detail", protocolErr.DevInfo)
		}
		detail := strings.ToLower(protocolErr.Details + " " + protocolErr.DevInfo)
		switch {
		case strings.Contains(detail, "origin"):
			message = "当前访问地址与 Passkey 配置不匹配，请使用 localhost:5173 或将当前来源加入 RP Origins"
		case strings.Contains(detail, "rp id") || strings.Contains(detail, "relying party id"):
			message = "Passkey 域名配置不匹配，请检查 RP ID 与当前访问域名"
		}
	}
	return apperr.New(code, message)
}

// normalizePasskeyName 去除空白、按字符截断并补默认名称。
func normalizePasskeyName(name string) string {
	name = strings.TrimSpace(name)
	runes := []rune(name)
	if len(runes) > passkeyNameLimit {
		name = string(runes[:passkeyNameLimit])
	}
	if name == "" {
		name = "未命名设备"
	}
	return name
}

// credentialRequest 把前端提交的原始 JSON 包装成 go-webauthn 需要的 *http.Request。
func credentialRequest(raw json.RawMessage) *http.Request {
	body := []byte(raw)
	return &http.Request{
		Method: http.MethodPost,
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(bytes.NewReader(body)),
	}
}

// waUser 把数据库用户适配为 go-webauthn 的 User 接口。
// 用户句柄使用带命名空间前缀的 SHA-256 摘要，避免把自增 ID 暴露给认证器。
type waUser struct {
	userID      uint64
	name        string
	displayName string
	credentials []webauthn.Credential
}

// newWAUser 用昵称（为空时用用户名）作为展示名构造适配对象。
func newWAUser(user *model.User, credentials []webauthn.Credential) *waUser {
	display := strings.TrimSpace(user.Nickname)
	if display == "" {
		display = user.Username
	}
	return &waUser{
		userID:      user.ID,
		name:        user.Username,
		displayName: display,
		credentials: credentials,
	}
}

// WebAuthnID 返回稳定且不可逆的用户句柄。
func (u *waUser) WebAuthnID() []byte {
	sum := sha256.Sum256([]byte(fmt.Sprintf("x-admin:passkey-user:%d", u.userID)))
	return sum[:]
}

// WebAuthnName 返回用户名，供认证器展示。
func (u *waUser) WebAuthnName() string { return u.name }

// WebAuthnDisplayName 返回昵称或用户名。
func (u *waUser) WebAuthnDisplayName() string { return u.displayName }

// WebAuthnCredentials 返回用户已绑定的全部凭据。
func (u *waUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }
