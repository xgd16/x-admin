package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/security"
)

type CaptchaVerifier interface {
	Verify(ctx context.Context, id, answer string) bool
}

type AuthOptions struct {
	MaxLoginFails int
	LockDuration  time.Duration
	RefreshTTL    time.Duration
}

type LoginInput struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	CaptchaID   string `json:"captcha_id" binding:"required"`
	CaptchaCode string `json:"captcha_code" binding:"required"`
}

type LoginMeta struct {
	IP        string
	UserAgent string
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type MeResult struct {
	User        *model.User `json:"user"`
	Permissions []string    `json:"permissions"`
	Menus       []*MenuNode `json:"menus"`
}

type AuthService struct {
	users   UserRepository
	menus   MenuRepository
	logs    LogRepository
	store   cache.Store
	tokens  *security.TokenManager
	hasher  security.PasswordHasher
	captcha CaptchaVerifier
	opts    AuthOptions
}

func NewAuthService(
	users UserRepository,
	menus MenuRepository,
	logs LogRepository,
	store cache.Store,
	tokens *security.TokenManager,
	hasher security.PasswordHasher,
	captcha CaptchaVerifier,
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
	return &AuthService{
		users: users, menus: menus, logs: logs, store: store,
		tokens: tokens, hasher: hasher, captcha: captcha, opts: opts,
	}
}

func (s *AuthService) Login(ctx context.Context, in LoginInput, meta LoginMeta) (*TokenPair, error) {
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		return nil, apperr.New(apperr.CodeInvalidArgument, "请输入用户名和密码")
	}
	if !s.captcha.Verify(ctx, in.CaptchaID, in.CaptchaCode) {
		return nil, apperr.New(apperr.CodeInvalidArgument, "验证码错误或已过期")
	}

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
			return nil, apperr.Newf(apperr.CodeLocked, "密码错误次数过多，账号锁定 %d 分钟", int(s.opts.LockDuration.Minutes()))
		}
		return nil, apperr.New(apperr.CodeUnauthorized, "用户名或密码错误")
	}

	_ = s.store.Del(ctx, loginFailKey(in.Username))
	now := time.Now()
	_ = s.users.UpdateFields(ctx, user.ID, map[string]any{
		"last_login_at": now,
		"last_login_ip": meta.IP,
	})
	s.recordLogin(ctx, user.ID, user.Username, 200, "登录成功", meta)
	return s.issue(ctx, user)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
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
	userID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || userID == 0 {
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
	return s.issue(ctx, user)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.store.Del(ctx, refreshKey(refreshToken))
}

// Authenticate loads the principal for every authenticated request.
func (s *AuthService) Authenticate(ctx context.Context, userID uint64) (*authn.Principal, error) {
	user, _, super, perms, err := s.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &authn.Principal{
		UserID:      user.ID,
		Username:    user.Username,
		Nickname:    user.Nickname,
		Super:       super,
		Permissions: perms,
	}, nil
}

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

func (s *AuthService) issue(ctx context.Context, user *model.User) (*TokenPair, error) {
	access, _, err := s.tokens.Issue(user.ID, user.Username)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	refresh, err := security.RandomToken()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	if err := s.store.Set(ctx, refreshKey(refresh), strconv.FormatUint(user.ID, 10), s.opts.RefreshTTL); err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "保存登录状态失败")
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.tokens.TTL() / time.Second),
	}, nil
}

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

func (s *AuthService) bumpFailure(ctx context.Context, username string) int64 {
	n, err := s.store.Incr(ctx, loginFailKey(username), s.opts.LockDuration)
	if err != nil {
		return 0
	}
	return n
}

func (s *AuthService) recordLogin(ctx context.Context, userID uint64, username string, status int, message string, meta LoginMeta) {
	_ = s.logs.CreateLogin(ctx, &model.LoginLog{
		UserID: userID, Username: username, IP: meta.IP, UserAgent: meta.UserAgent,
		Status: status, Message: message,
	})
}

func loginFailKey(username string) string { return "login:fail:" + strings.ToLower(username) }
func refreshKey(token string) string      { return "refresh:" + token }
