package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/security"
	"x-admin/internal/pkg/totp"
)

// CaptchaVerifier 抽象验证码校验，便于测试替身。
type CaptchaVerifier interface {
	Verify(ctx context.Context, id, answer string) bool
}

// AuthOptions 是登录与令牌策略的可配置项，零值会被替换为默认值。
type AuthOptions struct {
	MaxLoginFails int           // 连续密码错误上限
	LockDuration  time.Duration // 账号锁定时长
	RefreshTTL    time.Duration // 勾选“记住我”时的 refresh token 有效期
	SessionTTL    time.Duration // 未勾选“记住我”时的 refresh token 有效期
	// MFAChallengeTTL 限制密码校验通过后到输入动态码之间的时间窗口。
	MFAChallengeTTL time.Duration
	// MaxMFAFails 限制单个 MFA 挑战允许的错误次数，超过后挑战作废。
	MaxMFAFails int
}

// LoginInput 是账号密码登录请求。
type LoginInput struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	CaptchaID   string `json:"captcha_id" binding:"required"`
	CaptchaCode string `json:"captcha_code" binding:"required"`
	RememberMe  bool   `json:"remember_me"` // true 使用 RefreshTTL，false 使用 SessionTTL
}

// LoginMFAInput 是二次验证请求，Code 可以是 TOTP 动态码或恢复码。
type LoginMFAInput struct {
	MFAToken   string `json:"mfa_token" binding:"required"`
	Code       string `json:"otp_code" binding:"required"`
	RememberMe bool   `json:"remember_me"`
}

// LoginMeta 是记录登录审计与通知所需的客户端信息。
type LoginMeta struct {
	IP        string
	UserAgent string
}

// TokenPair 是登录成功后返回的令牌对。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"` // access token 剩余秒数
}

// LoginResult 要么携带令牌，要么在账号开启双重验证时返回待完成的 MFA 挑战。
type LoginResult struct {
	MFARequired  bool   `json:"mfa_required,omitempty"`
	MFAToken     string `json:"mfa_token,omitempty"`
	MFAExpiresIn int64  `json:"mfa_expires_in,omitempty"`
	RememberMe   bool   `json:"remember_me,omitempty"`
	*TokenPair
}

// MeResult 是 /auth/me 的响应，前端据此渲染菜单和按钮权限。
type MeResult struct {
	User        *model.User `json:"user"`
	Permissions []string    `json:"permissions"`
	Menus       []*MenuNode `json:"menus"`
}

// AuthService 实现登录、令牌轮换、身份加载和权限计算。
type AuthService struct {
	users    UserRepository
	menus    MenuRepository
	logs     LogRepository
	store    cache.Store
	tokens   *security.TokenManager
	hasher   security.PasswordHasher
	captcha  CaptchaVerifier
	notifier Notifier
	opts     AuthOptions
}

// NewAuthService 创建认证服务，并为未配置的策略项填入默认值。
func NewAuthService(
	users UserRepository,
	menus MenuRepository,
	logs LogRepository,
	store cache.Store,
	tokens *security.TokenManager,
	hasher security.PasswordHasher,
	captcha CaptchaVerifier,
	notifier Notifier,
	opts AuthOptions,
) *AuthService {
	if opts.MaxLoginFails <= 0 {
		opts.MaxLoginFails = 5
	}
	if opts.LockDuration <= 0 {
		opts.LockDuration = 15 * time.Minute
	}
	if opts.RefreshTTL <= 0 {
		opts.RefreshTTL = 7 * 24 * time.Hour
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 12 * time.Hour
	}
	if opts.MFAChallengeTTL <= 0 {
		opts.MFAChallengeTTL = 5 * time.Minute
	}
	if opts.MaxMFAFails <= 0 {
		opts.MaxMFAFails = 5
	}
	return &AuthService{
		users: users, menus: menus, logs: logs, store: store,
		tokens: tokens, hasher: hasher, captcha: captcha, notifier: notifier, opts: opts,
	}
}

// Login 执行账号密码登录。
//
// 流程：图形验证码 → 锁定检查 → 用户查询 → 密码校验 → 双重验证判断。
// 连续失败达到上限后锁定账号并发送安全提醒。
func (s *AuthService) Login(ctx context.Context, in LoginInput, meta LoginMeta) (*LoginResult, error) {
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		return nil, apperr.New(apperr.CodeInvalidArgument, "请输入用户名和密码")
	}
	if !s.captcha.Verify(ctx, in.CaptchaID, in.CaptchaCode) {
		return nil, apperr.New(apperr.CodeInvalidArgument, "验证码错误或已过期")
	}

	// 锁定状态先于密码校验判断，避免继续消耗密码尝试次数。
	if n, _ := s.failures(ctx, in.Username); n >= int64(s.opts.MaxLoginFails) {
		return nil, apperr.Newf(apperr.CodeLocked, "账号已锁定，请 %d 分钟后再试", int(s.opts.LockDuration.Minutes()))
	}

	user, err := s.users.FindByUsername(ctx, in.Username)
	if err != nil {
		if IsNotFound(err) {
			s.bumpFailure(ctx, in.Username)
			s.recordLogin(ctx, 0, in.Username, 401, "用户名或密码错误", meta)
			return nil, apperr.New(apperr.CodeUnauthorized, "用户名或密码错误")
		}
		return nil, err
	}
	if user.Status != model.StatusEnabled {
		s.recordLogin(ctx, user.ID, user.Username, 403, "账号已停用", meta)
		return nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	if !s.hasher.Verify(user.Password, in.Password) {
		n := s.bumpFailure(ctx, in.Username)
		s.recordLogin(ctx, user.ID, user.Username, 401, "用户名或密码错误", meta)
		if n >= int64(s.opts.MaxLoginFails) {
			notify(ctx, s.notifier, NotificationInput{
				UserID:  user.ID,
				Title:   "账号安全提醒",
				Content: fmt.Sprintf("连续 %d 次密码错误，账号已锁定 %d 分钟。如非本人操作，请联系管理员。", n, int(s.opts.LockDuration.Minutes())),
				Type:    model.NotificationError,
				Link:    "/profile",
			})
			return nil, apperr.Newf(apperr.CodeLocked, "密码错误次数过多，账号锁定 %d 分钟", int(s.opts.LockDuration.Minutes()))
		}
		return nil, apperr.New(apperr.CodeUnauthorized, "用户名或密码错误")
	}

	// 密码正确即清零失败计数。
	_ = s.store.Del(ctx, loginFailKey(in.Username))
	if user.TOTPEnabled {
		if user.TOTPSecret == "" {
			s.recordLogin(ctx, user.ID, user.Username, 403, "双重验证配置异常", meta)
			return nil, apperr.New(apperr.CodeForbidden, "双重验证配置异常，请联系管理员重置")
		}
		// 不直接签发令牌，而是生成一次性挑战，等待 LoginMFA 完成。
		challenge, err := security.RandomToken()
		if err != nil {
			return nil, apperr.Wrap(err, apperr.CodeInternal, "创建验证会话失败")
		}
		if err := s.store.Set(ctx, mfaChallengeKey(challenge), strconv.FormatUint(user.ID, 10), s.opts.MFAChallengeTTL); err != nil {
			return nil, apperr.Wrap(err, apperr.CodeInternal, "创建验证会话失败")
		}
		return &LoginResult{
			MFARequired:  true,
			MFAToken:     challenge,
			MFAExpiresIn: int64(s.opts.MFAChallengeTTL / time.Second),
			RememberMe:   in.RememberMe,
		}, nil
	}
	return s.completeLogin(ctx, user, meta, in.RememberMe)
}

// LoginMFA 完成 Login 发起的二次验证。
// 挑战一次性使用，单个挑战的错误次数有上限。
func (s *AuthService) LoginMFA(ctx context.Context, in LoginMFAInput, meta LoginMeta) (*LoginResult, error) {
	in.MFAToken = strings.TrimSpace(in.MFAToken)
	in.Code = strings.TrimSpace(in.Code)
	if in.MFAToken == "" || in.Code == "" {
		return nil, apperr.New(apperr.CodeInvalidArgument, "请输入动态验证码")
	}
	raw, err := s.store.Get(ctx, mfaChallengeKey(in.MFAToken))
	if err != nil {
		if err == cache.ErrNotFound {
			return nil, apperr.New(apperr.CodeTokenExpired, "验证已过期，请重新登录")
		}
		return nil, err
	}
	userID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || userID == 0 {
		_ = s.store.Del(ctx, mfaChallengeKey(in.MFAToken))
		return nil, apperr.New(apperr.CodeTokenExpired, "验证已过期，请重新登录")
	}
	if n, _ := s.mfaFailures(ctx, in.MFAToken); n >= int64(s.opts.MaxMFAFails) {
		s.clearMFA(ctx, in.MFAToken)
		return nil, apperr.New(apperr.CodeTooManyRequests, "验证次数过多，请重新登录")
	}
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		s.clearMFA(ctx, in.MFAToken)
		if IsNotFound(err) {
			return nil, apperr.New(apperr.CodeTokenExpired, "验证已过期，请重新登录")
		}
		return nil, err
	}
	// 挑战签发后账号可能被停用或关闭双重验证，需要重新确认状态。
	if user.Status != model.StatusEnabled || !user.TOTPEnabled || user.TOTPSecret == "" {
		s.clearMFA(ctx, in.MFAToken)
		return nil, apperr.New(apperr.CodeForbidden, "账号不可用")
	}
	valid := totp.Validate(user.TOTPSecret, in.Code, time.Now())
	if !valid {
		// 动态码不匹配时再尝试一次性恢复码。
		used, err := consumeRecoveryCode(ctx, s.users, user, in.Code)
		if err != nil {
			return nil, err
		}
		valid = used
	}
	if !valid {
		n := s.bumpMFAFailure(ctx, in.MFAToken)
		s.recordLogin(ctx, user.ID, user.Username, 401, "动态验证码错误", meta)
		if n >= int64(s.opts.MaxMFAFails) {
			s.clearMFA(ctx, in.MFAToken)
			return nil, apperr.New(apperr.CodeTooManyRequests, "验证码错误次数过多，请重新登录")
		}
		return nil, apperr.New(apperr.CodeUnauthorized, "验证码或恢复码不正确")
	}
	s.clearMFA(ctx, in.MFAToken)
	return s.completeLogin(ctx, user, meta, in.RememberMe)
}

// completeLogin 统一普通登录与二次验证成功后的收尾。
func (s *AuthService) completeLogin(ctx context.Context, user *model.User, meta LoginMeta, remember bool) (*LoginResult, error) {
	message := "登录成功"
	if user.TOTPEnabled {
		message = "登录成功（双重验证）"
	}
	return s.finishLogin(ctx, user, meta, message, remember)
}

// LoginPasskey 完成免密码的 WebAuthn 登录。
func (s *AuthService) LoginPasskey(ctx context.Context, user *model.User, meta LoginMeta, remember bool) (*LoginResult, error) {
	if user.Status != model.StatusEnabled {
		return nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	return s.finishLogin(ctx, user, meta, "登录成功（Passkey）", remember)
}

// finishLogin 记录登录信息、发送新登录提醒并签发令牌。
func (s *AuthService) finishLogin(ctx context.Context, user *model.User, meta LoginMeta, message string, remember bool) (*LoginResult, error) {
	now := time.Now()
	_ = s.users.UpdateFields(ctx, user.ID, map[string]any{
		"last_login_at": now,
		"last_login_ip": meta.IP,
	})
	s.recordLogin(ctx, user.ID, user.Username, 200, message, meta)
	notify(ctx, s.notifier, NotificationInput{
		UserID: user.ID,
		Title:  "新登录提醒",
		Content: fmt.Sprintf("你的账号于 %s 在 %s%s 登录。如非本人操作，请立即修改密码。",
			now.Format("01-02 15:04"), meta.IP, deviceLabel(meta.UserAgent)),
		Type: model.NotificationInfo,
		Link: "/profile",
	})
	sessionID, err := security.RandomToken()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "创建登录会话失败")
	}
	pair, err := s.issue(ctx, user, sessionID, meta, s.refreshTTL(remember), remember)
	if err != nil {
		return nil, err
	}
	return &LoginResult{TokenPair: pair}, nil
}

// Refresh 轮换 refresh token。
// Take 原子地读取并删除旧 token，保证同一 token 只能使用一次；
// 同时校验 session_version，安全变更后旧 token 立即失效。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string, meta LoginMeta) (*TokenPair, error) {
	if refreshToken == "" {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
	}
	raw, err := s.store.Take(ctx, refreshKey(refreshToken))
	if err != nil {
		if err == cache.ErrNotFound {
			return nil, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
		}
		return nil, err
	}
	userID, version, sessionID, remember, err := parseRefreshValue(raw)
	if err != nil || userID == 0 || sessionID == "" {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录状态已更新，请重新登录")
	}
	// 会话被单独撤销后，残留的 refresh token 也不能再使用。
	active, err := s.SessionActive(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
	}
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
		}
		return nil, err
	}
	if user.Status != model.StatusEnabled {
		return nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	if user.SessionVersion != version {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录状态已更新，请重新登录")
	}
	return s.issue(ctx, user, sessionID, meta, s.refreshTTL(remember), remember)
}

// RefreshSession 把已有 refresh token 重新指向用户当前的 session_version。
// 用于开启双重验证等会使所有旧令牌失效的安全操作之后，
// 让执行该操作的设备保持登录。
func (s *AuthService) RefreshSession(ctx context.Context, userID uint64, refreshToken string) {
	if refreshToken == "" || userID == 0 {
		return
	}
	raw, err := s.store.Get(ctx, refreshKey(refreshToken))
	if err != nil {
		return
	}
	id, _, sessionID, remember, err := parseRefreshValue(raw)
	if err != nil || id != userID || sessionID == "" {
		return
	}
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return
	}
	active, err := s.SessionActive(ctx, userID, sessionID)
	if err != nil || !active {
		return
	}
	// 同步会话记录的版本号，设备列表才不会把当前设备当成过期会话。
	ttl := s.refreshTTL(remember)
	s.touchSession(ctx, userID, sessionID, user.SessionVersion, LoginMeta{}, ttl, remember)
	_ = s.store.Set(ctx, refreshKey(refreshToken), refreshValue(user, sessionID, remember), ttl)
}

// Logout 删除 refresh token 与登录会话，使其无法再换取新令牌。
// currentSessionID 用于额外撤销当前 access token 所属会话。
func (s *AuthService) Logout(ctx context.Context, userID uint64, refreshToken, currentSessionID string) error {
	if refreshToken != "" {
		if raw, err := s.store.Get(ctx, refreshKey(refreshToken)); err == nil {
			id, _, sessionID, _, parseErr := parseRefreshValue(raw)
			// 只允许注销自己的令牌，避免拿他人 refresh token 做破坏。
			if parseErr == nil && id == userID {
				_ = s.store.Del(ctx, refreshKey(refreshToken))
				if sessionID != "" {
					_ = s.store.Del(ctx, sessionKey(userID, sessionID))
				}
			}
		}
	}
	if currentSessionID != "" {
		_ = s.store.Del(ctx, sessionKey(userID, currentSessionID))
	}
	return nil
}

// Unlock 校验锁屏解锁密码。
//
// 失败计数独立于登录（键 unlock:fail:<uid>），避免锁屏尝试与账号登录锁定互相干扰；
// 上限与锁定时长复用 AuthOptions，连续失败达到上限后在窗口内直接拒绝校验。
func (s *AuthService) Unlock(ctx context.Context, userID uint64, password string) error {
	if strings.TrimSpace(password) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "请输入密码")
	}
	// 锁定状态先于密码校验判断，避免继续消耗尝试次数。
	if s.unlockFailures(ctx, userID) >= int64(s.opts.MaxLoginFails) {
		return apperr.Newf(apperr.CodeLocked, "解锁失败次数过多，请 %d 分钟后再试", int(s.opts.LockDuration.Minutes()))
	}
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return err
	}
	if user.Status != model.StatusEnabled {
		return apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	if !s.hasher.Verify(user.Password, password) {
		n, _ := s.store.Incr(ctx, unlockFailKey(userID), s.opts.LockDuration)
		if n >= int64(s.opts.MaxLoginFails) {
			return apperr.Newf(apperr.CodeLocked, "密码错误次数过多，请 %d 分钟后再试", int(s.opts.LockDuration.Minutes()))
		}
		return apperr.New(apperr.CodeUnauthorized, "密码错误")
	}
	// 密码正确即清零失败计数。
	_ = s.store.Del(ctx, unlockFailKey(userID))
	return nil
}

// Authenticate 为每个已认证请求加载身份与权限，供中间件调用。
func (s *AuthService) Authenticate(ctx context.Context, userID uint64) (*authn.Principal, error) {	user, _, super, perms, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &authn.Principal{
		UserID:         user.ID,
		Username:       user.Username,
		Nickname:       user.Nickname,
		SessionVersion: user.SessionVersion,
		Super:          super,
		Permissions:    perms,
	}, nil
}

// Me 返回当前用户资料、排序后的权限码和导航菜单树（不含按钮）。
func (s *AuthService) Me(ctx context.Context, userID uint64) (*MeResult, error) {
	user, menus, _, perms, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(perms))
	for code := range perms {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	nav := make([]model.Menu, 0, len(menus))
	for _, m := range menus {
		if m.Type != model.MenuButton {
			nav = append(nav, m)
		}
	}
	return &MeResult{User: user, Permissions: codes, Menus: buildMenuTree(nav)}, nil
}

// load 加载用户、菜单与权限集合。
// super_admin 拥有全部启用菜单；其他用户只加载角色授权且启用的菜单。
// 权限码来自菜单的 Permission 字段。
func (s *AuthService) load(ctx context.Context, userID uint64) (*model.User, []model.Menu, bool, map[string]struct{}, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil, false, nil, apperr.ErrUnauthorized
		}
		return nil, nil, false, nil, err
	}
	if user.Status != model.StatusEnabled {
		return nil, nil, false, nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	super := false
	for _, role := range user.Roles {
		if role.Code == "super_admin" && role.Status == model.StatusEnabled {
			super = true
			break
		}
	}
	var menus []model.Menu
	if super {
		menus, err = s.menus.All(ctx)
		if err == nil {
			// 就地过滤停用菜单，避免额外分配。
			filtered := menus[:0]
			for _, m := range menus {
				if m.Status == model.StatusEnabled {
					filtered = append(filtered, m)
				}
			}
			menus = filtered
		}
	} else {
		menus, err = s.menus.ForUser(ctx, user.ID)
	}
	if err != nil {
		return nil, nil, false, nil, err
	}
	perms := make(map[string]struct{})
	for _, m := range menus {
		if m.Permission != "" {
			perms[m.Permission] = struct{}{}
		}
	}
	return user, menus, super, perms, nil
}

// issue 签发 access token 与 refresh token。
// refresh token 是 32 字节随机串，缓存值为 "uid:session_version:session_id"。
func (s *AuthService) issue(ctx context.Context, user *model.User, sessionID string, meta LoginMeta, ttl time.Duration, remember bool) (*TokenPair, error) {
	access, _, err := s.tokens.Issue(user.ID, user.Username, user.SessionVersion, sessionID)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	refresh, err := security.RandomToken()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	if err := s.store.Set(ctx, refreshKey(refresh), refreshValue(user, sessionID, remember), ttl); err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "保存登录状态失败")
	}
	s.touchSession(ctx, user.ID, sessionID, user.SessionVersion, meta, ttl, remember)
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.tokens.TTL() / time.Second),
	}, nil
}

// failures 读取用户名当前的连续失败次数。
func (s *AuthService) failures(ctx context.Context, username string) (int64, error) {
	raw, err := s.store.Get(ctx, loginFailKey(username))
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// bumpFailure 累加失败次数并刷新锁定窗口，返回累计值。
func (s *AuthService) bumpFailure(ctx context.Context, username string) int64 {
	n, err := s.store.Incr(ctx, loginFailKey(username), s.opts.LockDuration)
	if err != nil {
		return 0
	}
	return n
}

// mfaFailures 读取某个 MFA 挑战的错误次数。
func (s *AuthService) mfaFailures(ctx context.Context, token string) (int64, error) {
	raw, err := s.store.Get(ctx, mfaFailKey(token))
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// bumpMFAFailure 累加 MFA 挑战的错误次数。
func (s *AuthService) bumpMFAFailure(ctx context.Context, token string) int64 {
	n, err := s.store.Incr(ctx, mfaFailKey(token), s.opts.MFAChallengeTTL)
	if err != nil {
		return 0
	}
	return n
}

// clearMFA 删除挑战及其错误计数。
func (s *AuthService) clearMFA(ctx context.Context, token string) {
	_ = s.store.Del(ctx, mfaChallengeKey(token), mfaFailKey(token))
}

// recordLogin 写入登录审计；审计失败不影响主流程。
func (s *AuthService) recordLogin(ctx context.Context, userID uint64, username string, status int, message string, meta LoginMeta) {
	_ = s.logs.CreateLogin(ctx, &model.LoginLog{
		UserID: userID, Username: username, IP: meta.IP, UserAgent: meta.UserAgent,
		Status: status, Message: message,
	})
}

// loginFailKey 返回账号失败计数的缓存键；用户名统一转小写避免绕过。
func loginFailKey(username string) string { return "login:fail:" + strings.ToLower(username) }

// unlockFailKey 返回锁屏解锁失败计数的缓存键（按用户隔离）。
func unlockFailKey(userID uint64) string { return "unlock:fail:" + strconv.FormatUint(userID, 10) }

// unlockFailures 读取解锁失败计数，键不存在按 0 处理。
func (s *AuthService) unlockFailures(ctx context.Context, userID uint64) int64 {
	raw, err := s.store.Get(ctx, unlockFailKey(userID))
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(raw, 10, 64)
	return n
}

// refreshKey 返回 refresh token 的缓存键。
func refreshKey(token string) string { return "refresh:" + token }

// mfaChallengeKey 返回二次验证挑战的缓存键。
func mfaChallengeKey(token string) string { return "mfa:challenge:" + token }

// mfaFailKey 返回二次验证错误计数的缓存键。
func mfaFailKey(token string) string { return "mfa:fail:" + token }

// refreshValue 生成 refresh token 的缓存值：
// 用户 ID + 会话版本 + 会话 ID + 是否“记住我”。
func refreshValue(user *model.User, sessionID string, remember bool) string {
	flag := "0"
	if remember {
		flag = "1"
	}
	return strconv.FormatUint(user.ID, 10) + ":" + strconv.FormatInt(user.SessionVersion, 10) + ":" + sessionID + ":" + flag
}

// parseRefreshValue 解析 "uid:session_version:session_id[:remember]"。
// 缺少会话 ID 的旧值会被调用方视为过期，强制重新登录。
func parseRefreshValue(raw string) (uint64, int64, string, bool, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ":", 4)
	if len(parts) < 2 {
		return 0, 0, "", false, fmt.Errorf("invalid refresh value")
	}
	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, 0, "", false, err
	}
	version, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", false, err
	}
	var sessionID string
	if len(parts) >= 3 {
		sessionID = strings.TrimSpace(parts[2])
	}
	remember := len(parts) == 4 && parts[3] == "1"
	return userID, version, sessionID, remember, nil
}

// refreshTTL 根据是否“记住我”返回 refresh token 有效期。
func (s *AuthService) refreshTTL(remember bool) time.Duration {
	if remember {
		return s.opts.RefreshTTL
	}
	return s.opts.SessionTTL
}

// deviceLabel 根据 User-Agent 给出粗略的设备提示，便于用户判断登录提醒是否可信。
// 无法识别的客户端不追加后缀。
func deviceLabel(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "android"):
		return "（移动端）"
	case strings.Contains(ua, "mac os"), strings.Contains(ua, "macintosh"):
		return "（macOS）"
	case strings.Contains(ua, "windows"):
		return "（Windows）"
	case strings.Contains(ua, "linux"):
		return "（Linux）"
	}
	return ""
}
