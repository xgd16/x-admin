# 前台用户认证：独立身份体系实现演示

> x-admin 自带的后台认证（`/admin/v1/auth/*`）服务于管理端账号（`sys_users`）。如果要给前台（C 端）用户做一套登录——独立注册、独立账号体系、不能碰后台任何接口——本文给出一套完整可落地的实现：从迁移、模型一路到路由与 wire 装配，全部代码贴合项目现有约定。
>
> 框架可以通过 `go tool xadmin create api --name client --version v1 --auth --table client_users` 直接生成一套独立账号表 + JWT 注册/登录/刷新/登出接口（见 [多 API 模式](guide/multi-api.md)）。如果只需要标准账号密码体系，优先用生成器；本文适合需要独立字段、第三方登录或特殊会话策略的场景。本文的 `FrontAuthService` / `FrontAuth` 可以挂到 `SurfaceAuth["client"]`，并把 TokenManager 换成 `SurfaceTokens["client"]`。文中的 `/api/v1` 是另一种命名示例。

---

## 1. 隔离设计：为什么这样改

前台与后台是两套并行体系，隔离靠三道闸：

```
后台 /admin/v1                          前台 /api/v1
────────────────                        ────────────────
sys_users（管理账号）                    app_users（C 端账号）
TokenManager(issuer="x-admin")          TokenManager(issuer="x-admin-app")
  │                                        │
middleware.Auth                          middleware.FrontAuth
  │ 加载 Principal + 权限码                 │ 只加载身份，权限码恒为空
  │                                        │
refresh:{token}（Redis）                 front:refresh:{token}（Redis）
```

1. **issuer 隔离**：`security.TokenManager.Parse` 强制校验 issuer（`jwt.WithIssuer`，见 `internal/pkg/security/jwt.go:85`）。给前台再建一个不同 issuer 的实例，两套 access token 互相解析必然失败——`security` 包一行不用改，也不需要在 claims 里加"用户类型"字段再到处判断。
2. **缓存 key 前缀隔离**：refresh token 和登录失败计数都是不透明随机串/用户名拼 key。前台必须用 `front:` 前缀——否则前台的 refresh token 能换出后台会话，构成混淆攻击漏洞。
3. **fail-closed 权限**：`FrontAuth` 写入上下文的 `Principal` 权限为空、`Super=false`，`Require()` 对前台用户一律拒绝。即使某天前台路由误挂了后台权限中间件，也不会放行。

---

## 2. 第一步：迁移与模型

`migrations/000002_app_users.up.sql`：

```sql
CREATE TABLE app_users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    password VARCHAR(255) NOT NULL,
    nickname VARCHAR(64) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT 1,
    session_version BIGINT NOT NULL DEFAULT 0,
    last_login_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    UNIQUE KEY uk_app_users_username (username),
    KEY idx_app_users_status (status),
    KEY idx_app_users_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

`migrations/000002_app_users.down.sql`：

```sql
DROP TABLE IF EXISTS app_users;
```

`internal/model/app_user.go`：

```go
package model

import "time"

// AppUser 是前台（C 端）用户，与后台管理账号 sys_users 完全独立。
type AppUser struct {
	Base
	Username       string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Password       string     `gorm:"size:255;not null" json:"-"` // bcrypt 哈希，绝不下发
	Nickname       string     `gorm:"size:64;not null;default:''" json:"nickname"`
	Status         int8       `gorm:"not null;default:1;index" json:"status"`
	SessionVersion int64      `gorm:"not null;default:0" json:"-"` // 改密/封禁时递增，旧令牌立即失效
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
}

// TableName 指定前台用户表名。
func (AppUser) TableName() string { return "app_users" }
```

要点：`Password` 与 `SessionVersion` 都是 `json:"-"`，模型直接序列化返回也不会泄漏；`SessionVersion` 保留——它是"改密后旧 token 立即失效"的机制基础，前台同样需要。

---

## 3. 第二步：第二个 TokenManager

在 `internal/bootstrap/providers.go` 里加：

```go
// AppTokens 是前台用户的令牌管理器。
// wire 中同一个 *security.TokenManager 只能有一个 provider，
// 所以用命名类型包一层，让两套实例并存；issuer 不同保证令牌互不通用。
type AppTokens struct {
	*security.TokenManager
}

// provideAppTokens 创建前台令牌管理器：与后台共用签名密钥（HS256 单机部署足够），
// 但 issuer 加 -app 后缀——Parse 会校验 issuer，两套令牌因此互不通用。
func provideAppTokens(cfg config.Config) AppTokens {
	return AppTokens{security.NewTokenManager(cfg.JWT.Secret, cfg.JWT.Issuer+"-app", cfg.JWT.AccessTTL)}
}
```

> **已踩过的坑**：直接再写一个 `provideAppTokens(cfg) *security.TokenManager` 会触发 wire 报错
> `multiple providers for *security.TokenManager`。命名类型包裹是标准解法。

---

## 4. 第三步：端口接口与仓储

`internal/service/ports.go` 里加端口（service 层只依赖接口，符合项目分层约定）：

```go
// AppUserRepository 定义前台用户数据的读写能力。
type AppUserRepository interface {
	Get(ctx context.Context, id uint64) (*model.AppUser, error)
	FindByUsername(ctx context.Context, username string) (*model.AppUser, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	BumpSessionVersion(ctx context.Context, id uint64) error
}
```

`internal/repository/app_user.go`：

```go
package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// AppUserRepo 提供前台用户的数据访问。
type AppUserRepo struct {
	*Base[model.AppUser]
	db *gorm.DB
}

// NewAppUserRepo 创建前台用户仓储。
func NewAppUserRepo(db *gorm.DB) *AppUserRepo {
	return &AppUserRepo{Base: NewBase[model.AppUser](db), db: db}
}

// Get 按主键查询前台用户。Base[T] 提供的是 FindByID，
// 这里显式实现 Get 与端口接口、后台 UserRepository 保持同名同职责。
func (r *AppUserRepo) Get(ctx context.Context, id uint64) (*model.AppUser, error) {
	var user model.AppUser
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsername 按用户名查询前台用户。
func (r *AppUserRepo) FindByUsername(ctx context.Context, username string) (*model.AppUser, error) {
	var user model.AppUser
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// BumpSessionVersion 递增会话版本，撤销该用户当前所有 access / refresh token。
func (r *AppUserRepo) BumpSessionVersion(ctx context.Context, id uint64) error {
	res := r.db.WithContext(ctx).Model(&model.AppUser{}).Where("id = ?", id).
		UpdateColumn("session_version", gorm.Expr("session_version + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
```

（`UpdateFields` 由 `Base[T]` 泛型仓储直接提供；`Get` 需要显式实现——`Base[T]` 提供的方法名是 `FindByID`，端口统一叫 `Get`，与后台 `UserRepository` 一致。）

`internal/repository/wire.go` 的 `ProviderSet` 加两行：

```go
NewAppUserRepo,
wire.Bind(new(service.AppUserRepository), new(*AppUserRepo)),
```

---

## 5. 第四步：FrontAuthService

新建 `internal/service/front_auth.go`。流程与 `AuthService.Login`（`internal/service/auth.go:134`）同构，但按第 1 节的隔离设计换掉 key 前缀、去掉验证码（C 端 API 客户端没有图形界面），锁定与限流保留：

```go
package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/security"
)

// FrontLoginInput 是前台登录入参；C 端默认不带图形验证码，防暴力破解交给锁定 + 限流。
type FrontLoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// FrontAuthService 提供前台（C 端）用户的登录、刷新、登出与身份加载。
// 与后台 AuthService 的关键隔离：
//   - 注入的 TokenManager issuer 不同，两套 access token 互不通用；
//   - refresh token、失败锁定缓存 key 都带 front: 前缀，杜绝跨体系混淆。
type FrontAuthService struct {
	users  AppUserRepository
	store  cache.Store
	tokens *security.TokenManager
	hasher security.PasswordHasher
	opts   AuthOptions
}

// NewFrontAuthService 创建前台认证服务，并为未配置的策略项填入默认值。
func NewFrontAuthService(
	users AppUserRepository,
	store cache.Store,
	tokens *security.TokenManager,
	hasher security.PasswordHasher,
	opts AuthOptions,
) *FrontAuthService {
	if opts.MaxLoginFails <= 0 {
		opts.MaxLoginFails = 5
	}
	if opts.LockDuration <= 0 {
		opts.LockDuration = 15 * time.Minute
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 12 * time.Hour
	}
	return &FrontAuthService{users: users, store: store, tokens: tokens, hasher: hasher, opts: opts}
}

// Login 校验前台用户名密码并签发令牌对。
func (s *FrontAuthService) Login(ctx context.Context, in FrontLoginInput, meta LoginMeta) (*TokenPair, error) {
	in.Username = strings.TrimSpace(in.Username)
	if in.Username == "" || in.Password == "" {
		return nil, apperr.New(apperr.CodeInvalidArgument, "请输入用户名和密码")
	}
	// 锁定状态先于密码校验判断，避免继续消耗密码尝试次数。
	if n, _ := s.failures(ctx, in.Username); n >= int64(s.opts.MaxLoginFails) {
		return nil, apperr.Newf(apperr.CodeLocked, "账号已锁定，请 %d 分钟后再试", int(s.opts.LockDuration.Minutes()))
	}
	user, err := s.users.FindByUsername(ctx, in.Username)
	if err != nil {
		if IsNotFound(err) {
			s.bumpFailure(ctx, in.Username)
			return nil, apperr.New(apperr.CodeUnauthorized, "用户名或密码错误")
		}
		return nil, err
	}
	if user.Status != model.StatusEnabled {
		return nil, apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	if !s.hasher.Verify(user.Password, in.Password) {
		s.bumpFailure(ctx, in.Username)
		return nil, apperr.New(apperr.CodeUnauthorized, "用户名或密码错误")
	}
	_ = s.store.Del(ctx, frontLoginFailKey(in.Username))
	return s.issue(ctx, user, s.opts.SessionTTL)
}

// Refresh 使用 refresh token 换发新的令牌对。
// Take 原子读取并删除——一次性轮换，旧 token 用过即废；
// 会话版本比对保证改密/封禁后遗留的 refresh token 也失效。
func (s *FrontAuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	raw, err := s.store.Take(ctx, frontRefreshKey(refreshToken))
	if err != nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "登录已失效，请重新登录")
	}
	uid, sv, ok := strings.Cut(raw, ":")
	if !ok {
		return nil, apperr.New(apperr.CodeUnauthorized, "登录已失效，请重新登录")
	}
	id, err := strconv.ParseUint(uid, 10, 64)
	if err != nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "登录已失效，请重新登录")
	}
	version, err := strconv.ParseInt(sv, 10, 64)
	if err != nil {
		return nil, apperr.New(apperr.CodeUnauthorized, "登录已失效，请重新登录")
	}
	user, err := s.users.Get(ctx, id)
	if err != nil || user.Status != model.StatusEnabled || user.SessionVersion != version {
		return nil, apperr.New(apperr.CodeTokenExpired, "登录状态已更新，请重新登录")
	}
	return s.issue(ctx, user, s.opts.SessionTTL)
}

// Logout 删除服务端的 refresh token，使其无法再次使用。
func (s *FrontAuthService) Logout(ctx context.Context, refreshToken string) {
	if refreshToken != "" {
		_ = s.store.Del(ctx, frontRefreshKey(refreshToken))
	}
}

// Authenticate 供 FrontAuth 中间件加载当前身份；
// 返回的 Principal 权限为空、Super=false——前台用户不参与后台权限码体系，
// 后台的 Require() 对其一律拒绝（fail-closed）。
func (s *FrontAuthService) Authenticate(ctx context.Context, userID uint64) (*authn.Principal, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
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

// Me 返回当前登录的前台用户资料。
func (s *FrontAuthService) Me(ctx context.Context, userID uint64) (*model.AppUser, error) {
	return s.users.Get(ctx, userID)
}

// issue 签发 access token 与 refresh token。
// refresh token 是 32 字节随机串，缓存值为 "uid:session_version"。
func (s *FrontAuthService) issue(ctx context.Context, user *model.AppUser, ttl time.Duration) (*TokenPair, error) {
	sessionID, err := security.RandomToken()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	access, _, err := s.tokens.Issue(user.ID, user.Username, user.SessionVersion, sessionID)
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	refresh, err := security.RandomToken()
	if err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "签发令牌失败")
	}
	value := strconv.FormatUint(user.ID, 10) + ":" + strconv.FormatInt(user.SessionVersion, 10)
	if err := s.store.Set(ctx, frontRefreshKey(refresh), value, ttl); err != nil {
		return nil, apperr.Wrap(err, apperr.CodeInternal, "保存登录状态失败")
	}
	_ = s.users.UpdateFields(ctx, user.ID, map[string]any{"last_login_at": time.Now()})
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.tokens.TTL() / time.Second),
	}, nil
}

// failures 读取用户名当前的连续失败次数。
func (s *FrontAuthService) failures(ctx context.Context, username string) (int64, error) {
	raw, err := s.store.Get(ctx, frontLoginFailKey(username))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(raw, 10, 64)
}

// bumpFailure 递增失败计数（TTL 即锁定时长），返回递增后的值。
func (s *FrontAuthService) bumpFailure(ctx context.Context, username string) int64 {
	n, _ := s.store.Incr(ctx, frontLoginFailKey(username), s.opts.LockDuration)
	return n
}

// 前台专用缓存 key：必须带 front: 前缀，与后台的 refresh:{token}、login:fail:* 隔离。
func frontRefreshKey(token string) string { return "front:refresh:" + token }
func frontLoginFailKey(username string) string { return "front:login:fail:" + username }
```

复用说明：`TokenPair` / `LoginMeta` / `AuthOptions` / `IsNotFound` 都是 `service` 包里的现成定义，直接用；bcrypt 哈希仍走统一的 `security.PasswordHasher`，不要自己引 `bcrypt`。

---

## 6. 第五步：FrontAuth 中间件

新建 `internal/middleware/front_auth.go`。比后台 `Auth` 简单——没有权限码和菜单——但三道检查一个不少：

```go
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

// FrontUserLoader 加载前台用户身份，由 FrontAuthService 实现，
// 避免中间件直接依赖业务服务（与 Auth 的 PrincipalLoader 同一模式）。
type FrontUserLoader interface {
	Authenticate(ctx context.Context, userID uint64) (*authn.Principal, error)
}

// FrontAuth 校验前台用户的 Bearer token 并把 Principal 写入请求上下文。
// 与 Auth 的区别：使用前台 TokenManager（issuer 不同，后台令牌在此直接被拒）；
// 不加载权限码——Principal.Permissions 为空，后台 Require() 对前台用户一律拒绝。
func FrontAuth(tokens *security.TokenManager, loader FrontUserLoader) gin.HandlerFunc {
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
		// 每次请求都重新加载身份，确保封停、资料变更立即生效。
		principal, err := loader.Authenticate(c.Request.Context(), claims.UserID)
		if err != nil || principal == nil {
			response.Error(c, apperr.ErrUnauthorized)
			return
		}
		// session_version 变化（改密、封禁等）后旧 access token 立即失效。
		if claims.SessionVersion != principal.SessionVersion {
			response.Error(c, apperr.New(apperr.CodeTokenExpired, "登录状态已更新，请重新登录"))
			return
		}
		c.Request = c.Request.WithContext(authn.With(c.Request.Context(), principal))
		c.Next()
	}
}
```

---

## 7. 第六步：Handler

新建 `internal/handler/front_auth.go`：

```go
package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// FrontAuthHandler 处理前台用户的登录、刷新、登出与资料查询。
type FrontAuthHandler struct {
	auth *service.FrontAuthService
}

// NewFrontAuthHandler 创建前台认证处理器。
func NewFrontAuthHandler(auth *service.FrontAuthService) *FrontAuthHandler {
	return &FrontAuthHandler{auth: auth}
}

// Login 校验用户名密码，返回令牌对。
func (h *FrontAuthHandler) Login(c *gin.Context) {
	var in service.FrontLoginInput
	if !Bind(c, &in) {
		return
	}
	pair, err := h.auth.Login(c.Request.Context(), in,
		service.LoginMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, pair)
}

// Refresh 使用 refresh token 轮换新的令牌对。
func (h *FrontAuthHandler) Refresh(c *gin.Context) {
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

// Logout 删除服务端 refresh token，使其立即作废。
func (h *FrontAuthHandler) Logout(c *gin.Context) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !Bind(c, &in) {
		return
	}
	h.auth.Logout(c.Request.Context(), in.RefreshToken)
	response.OK(c, nil)
}

// Me 返回当前登录的前台用户资料。
func (h *FrontAuthHandler) Me(c *gin.Context) {
	p, ok := authn.FromContext(c.Request.Context())
	if !ok {
		response.Error(c, apperr.ErrUnauthorized)
		return
	}
	user, err := h.auth.Me(c.Request.Context(), p.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}
```

`internal/handler/wire.go` 的 `ProviderSet` 加一行：`NewFrontAuthHandler,`。

---

## 8. 第七步：路由与 wire 装配

### 路由（推荐接入 create api 生成的 `client` 面）

先用 `go tool xadmin create api --name client --version v1` 生成 `/client/v1` API 面。接入本方案时，不需要再手工创建顶层 `/api/v1`：在 `provideRouter` 中把 `FrontAuth` 放进 `SurfaceAuth["client"]`，然后在生成的 `internal/router/client_v1.go` 的 `Mount` 里注册登录、刷新和业务路由，TokenManager 使用 `SurfaceTokens["client"]`（issuer 为 `jwt.issuer:client`）。

如果仍想使用独立的 `/api/v1` 前缀，可以按下面的方式手工注册：

`Handlers` 结构体加一个字段：

```go
FrontAuth *handler.FrontAuthHandler
```

`Deps` 加一个字段（前台 TokenManager 实例）：

```go
AppTokens *security.TokenManager
```

在 `/admin/v1` 组之后注册前台路由组：

```go
// /api/v1 是前台用户（C 端）路由组：与 /admin/v1 平行，使用独立 issuer 的
// TokenManager 与 FrontAuth 中间件，两套令牌互不通用。限流 key 同样独立。
front := r.Group("/api/v1")
front.Use(middleware.RateLimit(d.Cache, "rl:front-global", 100, time.Minute))
front.POST("/auth/login", middleware.RateLimit(d.Cache, "rl:front-login", 10, time.Minute), d.Handlers.FrontAuth.Login)
front.POST("/auth/refresh", d.Handlers.FrontAuth.Refresh)

frontSecured := front.Group("", middleware.FrontAuth(d.AppTokens, d.Handlers.FrontAuth))
frontSecured.GET("/auth/me", d.Handlers.FrontAuth.Me)
frontSecured.POST("/auth/logout", d.Handlers.FrontAuth.Logout)
```

### wire 装配（`internal/bootstrap/`）

**providers.go** 加服务适配器（`FrontAuthService` 依赖 `AppTokens` 这个 bootstrap 私有类型，所以和 `provideUserService` 一样走适配器，不进 `service.ProviderSet`）：

```go
// provideFrontAuthService 装配前台认证服务，注入前台令牌管理器。
func provideFrontAuthService(
	users service.AppUserRepository,
	store cache.Store,
	appTokens AppTokens,
	hasher security.PasswordHasher,
	opts service.AuthOptions,
) *service.FrontAuthService {
	return service.NewFrontAuthService(users, store, appTokens.TokenManager, hasher, opts)
}
```

**wire.go** 的 `wire.Build` 列表加两行：

```go
provideAppTokens,
provideFrontAuthService,
```

**provideRouter** 加 `appTokens AppTokens` 参数，并传进 `router.Deps`：

```go
func provideRouter(
	cfg config.Config,
	db *gorm.DB,
	tokens *security.TokenManager,
	appTokens AppTokens, // ← 新增
	auth *service.AuthService,
	gen *captcha.Generator,
	store cache.Store,
	recorder middleware.OperationRecorder,
	handlers router.Handlers,
) *gin.Engine {
	return router.New(router.Deps{
		// ...原有字段...
		AppTokens: appTokens.TokenManager, // ← 新增
		// ...
	})
}
```

最后运行 `make wire` 重新生成 `wire_gen.go`（不要手工编辑生成文件）。

---

## 9. 端到端验证

造一个测试账号（密码哈希用项目统一的 bcrypt 成本，随便找个地方跑一次）：

```go
h, _ := bcrypt.GenerateFromPassword([]byte("Front@123456"), 10)
// INSERT INTO app_users (username, password, nickname, status) VALUES ('frontdemo', <h>, '演示用户', 1);
```

四个接口走一遍：

```bash
# ① 登录
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"frontdemo","password":"Front@123456"}'
# → {"code":0,"data":{"access_token":"...","refresh_token":"...","token_type":"Bearer","expires_in":7200}}

# ② 带前台 token 查资料
curl -s http://localhost:8080/api/v1/auth/me -H 'Authorization: Bearer <access_token>'

# ③ 用后台 token 调前台接口 → 必须被拒（issuer 不匹配）
curl -s http://localhost:8080/api/v1/auth/me -H "Authorization: Bearer <后台access_token>"
# → {"code":40101,"message":"登录凭证无效"}

# ④ 用前台 token 调后台接口 → 同样被拒
curl -s http://localhost:8080/admin/v1/auth/me -H "Authorization: Bearer <前台access_token>"
# → {"code":40101,"message":"登录凭证无效"}

# ⑤ 刷新与登出
curl -s -X POST http://localhost:8080/api/v1/auth/refresh -d '{"refresh_token":"<refresh_token>"}'
curl -s -X POST http://localhost:8080/api/v1/auth/logout -H 'Authorization: Bearer <access_token>' -d '{"refresh_token":"<refresh_token>"}'
```

③④ 是本方案的核心验收点：**两套令牌互不通用**。

---

## 10. 安全清单与常见坑

自查清单：

- [ ] 前后台 TokenManager issuer 不同（两套 access token 互不通用，用 9 节的 ③④ 验证）
- [ ] refresh / 失败锁定的缓存 key 都带 `front:` 前缀
- [ ] `Refresh` 校验了缓存值里的 `session_version` 与数据库当前值一致
- [ ] FrontAuth 每请求查库校验 status + session_version，不信任 token 快照
- [ ] Principal 权限为空、Super=false（后台 `Require` 天然拒绝前台身份）
- [ ] 密码哈希走统一的 `security.PasswordHasher`，没有自己引 bcrypt
- [ ] 改密/封禁的实现里调用了 `AppUserRepo.BumpSessionVersion`
- [ ] `make wire` 已重新生成

已踩过/容易踩的坑：

- **wire 同类型多实例冲突**：`*security.TokenManager` 只能有一个 provider，第二个实例必须用命名类型（`AppTokens`）包裹，见第 3 节；
- **缓存 key 混淆攻击**：refresh token 是不透明随机串，如果前台后台共用 `refresh:` 前缀，前台的 refresh token 就能换出后台会话——前缀隔离不是风格问题，是安全边界；
- **`/uploads` 仍只认后台凭证**：`middleware.UploadAuth(d.Tokens, d.Auth, ...)` 挂的是后台 TokenManager，前台 token 访问不了上传文件。前台需要文件能力时，要给 `UploadAuth` 扩展第二套凭证解析，或为前台单独开文件路由组；
- **不要给前台路由挂 `Require("system:...")`**：前台 Principal 无权限码，挂了等于全拒绝；前台接口的"权限"就是登录态本身；
- **OpenAPI 同步**：按项目开发约定，新增接口记得同步 `internal/docs/openapi.yaml`；
- **想加"记住我"**：照抄后台 `AuthService.issue` 的 remember 分支，`RefreshTTL` / `SessionTTL` 二选一即可，缓存 value 里带上 remember 标记。
