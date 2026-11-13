package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/security"
)

// SurfaceScope 标识一个自定义 API 面及其用户表。
// 表名来自 create api --auth --table，不同 API 面不能共用同一张表。
type SurfaceScope struct {
	Name  string
	Table string
}

// SurfaceRegisterInput 是自定义 API 面的注册入参。
type SurfaceRegisterInput struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Nickname   string `json:"nickname"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	RememberMe bool   `json:"remember_me"`
}

// SurfaceLoginInput 是自定义 API 面的登录入参。
type SurfaceLoginInput struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"remember_me"`
}

// SurfaceAuthResult 是注册/登录/刷新成功的响应。
type SurfaceAuthResult struct {
	*TokenPair
	User *model.APIUser `json:"user"`
}

// SurfaceAuthService 为自定义 API 面提供独立的 JWT 注册、登录、刷新和身份加载。
//
// 它不复用后台 sys_users，也不参与后台 RBAC；账号存放在每个 API 面自己的表里。
type SurfaceAuthService struct {
	users          APIUserRepository
	store          cache.Store
	hasher         security.PasswordHasher
	passwordMinLen int
	opts           AuthOptions
}

// NewSurfaceAuthService 创建自定义 API 面认证服务。
func NewSurfaceAuthService(
	users APIUserRepository,
	store cache.Store,
	hasher security.PasswordHasher,
	passwordMinLen int,
	opts AuthOptions,
) *SurfaceAuthService {
	if passwordMinLen <= 0 {
		passwordMinLen = 8
	}
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
	return &SurfaceAuthService{
		users: users, store: store, hasher: hasher,
		passwordMinLen: passwordMinLen, opts: opts,
	}
}

// Register 注册一个自定义 API 面账号并直接签发令牌。
func (s *SurfaceAuthService) Register(
	ctx context.Context,
	scope SurfaceScope,
	in SurfaceRegisterInput,
	tokens *security.TokenManager,
	meta LoginMeta,
) (*SurfaceAuthResult, error) {
	in.Username = trim(in.Username)
	if err := ValidateUsername(in.Username); err != nil {
		return nil, err
	}
	if err := ValidatePassword(in.Password, s.passwordMinLen); err != nil {
		return nil, err
	}
	if err := ValidateEmail(trim(in.Email)); err != nil {
		return nil, err
	}
	if err := ValidatePhone(trim(in.Phone)); err != nil {
		return nil, err
	}
	if tokens == nil {
		return nil, apperr.New(apperr.CodeInternal, "API 面令牌未配置")
	}
	if _, err := s.users.FindByUsername(ctx, scope.Table, in.Username); err == nil {
		return nil, apperr.New(apperr.CodeConflict, "用户名已存在")
	} else if !IsNotFound(err) {
		return nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "密码处理失败")
	}
	user := &model.APIUser{
		Username: in.Username,
		Password: hash,
		Nickname: trim(in.Nickname),
		Email:    trim(in.Email),
		Phone:    trim(in.Phone),
		Status:   model.StatusEnabled,
	}
	if err := s.users.Create(ctx, scope.Table, user); err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "创建账号失败")
	}
	return s.issue(ctx, scope, tokens, user, "", meta, in.RememberMe)
}

// Login 校验用户名密码并签发令牌。
func (s *SurfaceAuthService) Login(
	ctx context.Context,
	scope SurfaceScope,
	in SurfaceLoginInput,
	tokens *security.TokenManager,
	meta LoginMeta,
) (*SurfaceAuthResult, error) {
	in.Username = trim(in.Username)
	if in.Username == "" || in.Password == "" {
		return nil, apperr.New(apperr.CodeInvalidArgument, "请输入用户名和密码")
	}
	if tokens == nil {
		return nil, apperr.New(apperr.CodeInternal, "API 面令牌未配置")
	}
	if n, _ := s.failures(ctx, scope.Name, in.Username); n >= int64(s.opts.MaxLoginFails) {
		return nil, apperr.Newf(apperr.CodeLocked, "账号已锁定，请 %d 分钟后再试", int(s.opts.LockDuration.Minutes()))
	}

	user, err := s.users.FindByUsername(ctx, scope.Table, in.Username)
	if err != nil {
		if IsNotFound(err) {
			s.bumpFailure(ctx, scope.Name, in.Username)
			return nil, apperr.New(apperr.CodeUnauthorized, "用户名或密码错误")
		}
		return nil, err
	}
	if user.Status != model.StatusEnabled {
		return nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	if !s.hasher.Verify(user.Password, in.Password) {
		s.bumpFailure(ctx, scope.Name, in.Username)
		return nil, apperr.New(apperr.CodeUnauthorized, "用户名或密码错误")
	}
	_ = s.store.Del(ctx, surfaceLoginFailKey(scope.Name, in.Username))
	return s.issue(ctx, scope, tokens, user, "", meta, in.RememberMe)
}

// Refresh 轮换 refresh token，并校验会话与 session_version。
func (s *SurfaceAuthService) Refresh(
	ctx context.Context,
	scope SurfaceScope,
	refreshToken string,
	tokens *security.TokenManager,
	meta LoginMeta,
) (*SurfaceAuthResult, error) {
	if refreshToken == "" || tokens == nil {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
	}
	raw, err := s.store.Take(ctx, surfaceRefreshKey(scope.Name, refreshToken))
	if err != nil {
		if err == cache.ErrNotFound {
			return nil, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
		}
		return nil, err
	}
	userID, version, sessionID, remember, err := parseSurfaceRefresh(raw)
	if err != nil || userID == 0 || sessionID == "" {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录状态已更新，请重新登录")
	}
	active, err := s.SessionActive(ctx, scope, userID, sessionID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
	}
	user, err := s.users.Get(ctx, scope.Table, userID)
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
	return s.issue(ctx, scope, tokens, user, sessionID, meta, remember)
}

// Logout 撤销 refresh token 与当前会话。
func (s *SurfaceAuthService) Logout(ctx context.Context, scope SurfaceScope, userID uint64, refreshToken, sessionID string) error {
	if refreshToken != "" {
		_ = s.store.Del(ctx, surfaceRefreshKey(scope.Name, refreshToken))
	}
	if userID != 0 && sessionID != "" {
		_ = s.store.Del(ctx, surfaceSessionKey(scope.Name, userID, sessionID))
	}
	return nil
}

// Authenticate 加载当前自定义 API 面用户，供 middleware.Auth 使用。
// 返回的 Principal 权限为空、Super=false，后台 Require 对其一律拒绝。
func (s *SurfaceAuthService) Authenticate(ctx context.Context, scope SurfaceScope, userID uint64) (*authn.Principal, error) {
	user, err := s.users.Get(ctx, scope.Table, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrUnauthorized
		}
		return nil, err
	}
	if user.Status != model.StatusEnabled {
		return nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	return &authn.Principal{
		UserID:         user.ID,
		Username:       user.Username,
		Nickname:       user.Nickname,
		SessionVersion: user.SessionVersion,
	}, nil
}

// SessionActive 校验 access token 所属会话是否仍然存在。
func (s *SurfaceAuthService) SessionActive(ctx context.Context, scope SurfaceScope, userID uint64, sessionID string) (bool, error) {
	if userID == 0 || sessionID == "" {
		return false, nil
	}
	raw, err := s.store.Get(ctx, surfaceSessionKey(scope.Name, userID, sessionID))
	if err != nil {
		if err == cache.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return raw == "1", nil
}

// Me 返回当前自定义 API 面用户资料。
func (s *SurfaceAuthService) Me(ctx context.Context, scope SurfaceScope, userID uint64) (*model.APIUser, error) {
	return s.users.Get(ctx, scope.Table, userID)
}

// issue 签发 access / refresh token，并记录登录会话。
func (s *SurfaceAuthService) issue(
	ctx context.Context,
	scope SurfaceScope,
	tokens *security.TokenManager,
	user *model.APIUser,
	sessionID string,
	meta LoginMeta,
	remember bool,
) (*SurfaceAuthResult, error) {
	if sessionID == "" {
		var err error
		sessionID, err = security.RandomToken()
		if err != nil {
			return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
		}
	}
	access, _, err := tokens.Issue(user.ID, user.Username, user.SessionVersion, sessionID)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	refresh, err := security.RandomToken()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	ttl := s.refreshTTL(remember)
	if err := s.store.Set(ctx, surfaceRefreshKey(scope.Name, refresh), surfaceRefreshValue(user, sessionID, remember), ttl); err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "保存登录状态失败")
	}
	if err := s.store.Set(ctx, surfaceSessionKey(scope.Name, user.ID, sessionID), "1", ttl); err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "保存登录会话失败")
	}
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = meta.IP
	_ = s.users.UpdateFields(ctx, scope.Table, user.ID, map[string]any{
		"last_login_at": now,
		"last_login_ip": meta.IP,
	})
	return &SurfaceAuthResult{
		TokenPair: &TokenPair{
			AccessToken:  access,
			RefreshToken: refresh,
			TokenType:    "Bearer",
			ExpiresIn:    int64(tokens.TTL() / time.Second),
		},
		User: user,
	}, nil
}

func (s *SurfaceAuthService) refreshTTL(remember bool) time.Duration {
	if remember {
		return s.opts.RefreshTTL
	}
	return s.opts.SessionTTL
}

func (s *SurfaceAuthService) failures(ctx context.Context, surface, username string) (int64, error) {
	raw, err := s.store.Get(ctx, surfaceLoginFailKey(surface, username))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}

func (s *SurfaceAuthService) bumpFailure(ctx context.Context, surface, username string) int64 {
	n, _ := s.store.Incr(ctx, surfaceLoginFailKey(surface, username), s.opts.LockDuration)
	return n
}

func surfaceRefreshKey(surface, token string) string {
	return "auth:" + surface + ":refresh:" + token
}

func surfaceLoginFailKey(surface, username string) string {
	return "auth:" + surface + ":login:fail:" + username
}

func surfaceSessionKey(surface string, userID uint64, sessionID string) string {
	return fmt.Sprintf("auth:%s:session:%d:%s", surface, userID, sessionID)
}

func surfaceRefreshValue(user *model.APIUser, sessionID string, remember bool) string {
	return fmt.Sprintf("%d:%d:%s:%t", user.ID, user.SessionVersion, sessionID, remember)
}

func parseSurfaceRefresh(raw string) (uint64, int64, string, bool, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 4 {
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
	remember, err := strconv.ParseBool(parts[3])
	if err != nil {
		return 0, 0, "", false, err
	}
	return userID, version, parts[2], remember, nil
}
