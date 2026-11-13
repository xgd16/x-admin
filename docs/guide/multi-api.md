# 多 API 模式：后台与自定义 API 面

x-admin 默认只暴露后台 `/admin/v1`。路由层支持**多个并行 API 面**：每个面拥有自己的前缀、限流桶和 JWT issuer，令牌互不通用。

框架不预置 client / merchant 路由文件。需要客户端、商户端或开放平台时，用 `go tool xadmin create api` 按需生成：

```bash
# 带完整 JWT 认证（推荐）
go tool xadmin create api --name client --version v1 --auth --table client_users

# 只生成路由壳，认证后续自己接
go tool xadmin create api --name partner --version v1
```

多 API 不是把同一组接口换个前缀暴露出去，而是让每个面独立选择身份体系、权限模型和限流策略。

## 1. 默认结构

```text
internal/router/
├── router.go       # 全局中间件、健康检查、Swagger、按配置挂载 API 面
├── surface.go      # Surface 注册表、Module 注册表、公共挂载逻辑
└── admin.go        # 默认唯一内置面：后台管理端
```

启动后默认只有：

| API 面 | 默认前缀 | JWT issuer | 认证 |
| --- | --- | --- | --- |
| `admin` | `/admin/v1` | `jwt.issuer` | JWT + `sys_users` + 权限码 |

```bash
curl http://localhost:8080/admin/v1/ping
```

```json
{ "api": "admin", "status": "ok" }
```

`create api` 生成的自定义面同样带 `/ping`：

```bash
curl http://localhost:8080/client/v1/ping
```

## 2. 创建自定义 API 面

```bash
# 只生成路由壳
go tool xadmin create api --name client --version v1

# 生成完整 JWT 认证 + 独立用户表
go tool xadmin create api --name client --version v1 --auth --table client_users
```

不带 `--auth` 时生成：

```text
internal/handler/client/v1/handler.go
internal/router/client_v1.go
```

带 `--auth` 时额外生成：

```text
migrations/000002_client_users.up.sql
migrations/000002_client_users.down.sql
```

并自动注册：

```text
POST /client/v1/auth/register
POST /client/v1/auth/login
POST /client/v1/auth/refresh
POST /client/v1/auth/logout
GET  /client/v1/auth/me
```

注册/登录/刷新返回 `access_token`、`refresh_token` 和当前用户；用户表由迁移创建，表名来自 `--table` 或默认 `<name>_users`。迁移编号默认取当前最大编号 + 1（例如只有 `000001_init` 时生成 `000002_client_users`），保证新迁移排在已有迁移之后。

同时自动写入 `configs/config.yaml` 的 `api` 段，并按 API 面名称排序：

```yaml
api:
  admin:
    enabled: true
    prefix: /admin/v1
  client:
    enabled: true
    prefix: /client/v1
    table: client_users
```

写入后 `/client/v1` 面才会挂载；重复执行只更新同一个面。排序只移动 API 面的原文块，面内注释和空行原样保留。还没有 `configs/config.yaml` 时工具会退回到打印片段；也可以加 `--no-config` 让它只打印不改文件。生成的路由工厂通过 `init()` 自动注册，不需要改 `router.go`。

继续添加业务 handler：

```bash
go tool xadmin create handler --api client --version v1 --name order --path /orders
```

生成：

```text
internal/handler/client/v1/order.go
internal/router/client_v1_order.go
```

handler 按 `<api>/<version>` 分层；路由模块通过 `init()` 注册，按 API 面 + 版本自动挂载。`--auth` 生成面的 `secured` 路由会自动带上 JWT 校验。详见 [代码生成（create）](../tool/create.md)。

## 3. 配置

默认只有后台面：

```yaml
api:
  admin:
    enabled: true
    prefix: /admin/v1
```

自定义面直接写 `api.<name>`：

```yaml
api:
  admin:
    enabled: true
    prefix: /admin/v1
  client:
    enabled: true
    prefix: /client/v1
    table: client_users       # create api --auth 生成的用户表
  merchant:
    enabled: true
    prefix: /merchant/v1
    table: merchant_users
```

| 字段 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `api.admin.enabled` | `true` | `X_ADMIN_API_ADMIN_ENABLED` | 是否挂载后台 API |
| `api.admin.prefix` | `/admin/v1` | `X_ADMIN_API_ADMIN_PREFIX` | 后台路径前缀 |
| `api.admin.issuer` | `jwt.issuer` | `X_ADMIN_API_ADMIN_ISSUER` | 后台令牌 issuer |
| `api.<name>.enabled` | `false` | — | 自定义面是否挂载 |
| `api.<name>.prefix` | — | — | 自定义面路径前缀 |
| `api.<name>.table` | — | — | `--auth` 生成面的用户表名 |
| `api.<name>.issuer` | `jwt.issuer:<name>` | — | 自定义面令牌 issuer |

启动校验会拒绝：

- 两个启用中的 API 面使用相同前缀；
- 两个启用中的 API 面使用相同 issuer；
- 前缀不是以 `/` 开头、以 `/` 结尾或包含 `//`；
- 前缀占用 `/healthz`、`/uploads`、`/swagger` 及其子路径。

修改前缀后，Nginx、Vite 代理和前端 `VITE_API_BASE` 要同步调整。

## 4. 令牌隔离

`internal/bootstrap/providers.go` 会为每个启用的面创建独立 `TokenManager`：

```go
surfaceTokens := map[string]*security.TokenManager{
    "admin":  adminTokens, // issuer = jwt.issuer
    "client": security.NewTokenManager(secret, issuer+":client", ttl),
}
```

`security.TokenManager.Parse` 会校验 issuer，因此：

- 后台 token 调 `/client/v1` → `40101`；
- 客户端 token 调 `/admin/v1` → `40101`；
- 两端即使用同一个 `jwt.secret` 也不会串用。

`router.Deps.SurfaceTokens` 暴露这些实例，接入认证时直接取对应面：

```go
Auth: middleware.Auth(d.SurfaceTokens["client"], clientAuth, clientAuth)
```

不要把后台的 `d.Tokens` 传给自定义面，否则令牌隔离就失效了。

## 5. 客户端认证（进阶）

最简单的做法是 `create api --name client --version v1 --auth --table client_users`，它会生成完整 JWT 接口和用户表迁移，账号由框架内置的 `SurfaceAuthService` 管理。

如果要把客户端账号接到已有系统、第三方身份或特殊字段，再按下面的步骤自定义。注意：`--auth` 已经自动配置好 `SurfaceAuth`；下面的手工步骤适用于 `create api` 不带 `--auth` 的场景。

### 5.1 建前台身份体系

客户端账号通常与后台 `sys_users` 分离。可以按 [前台用户认证](../front-auth.md) 建 `app_users`、`AppUserRepository`、`FrontAuthService` 和 `FrontAuth` 中间件。

关键点：

- 独立账号表，不复用后台管理账号；
- refresh token、失败锁定的缓存 key 加 `client:` 前缀；
- 每次请求加载用户状态和 `session_version`；
- `Principal` 不携带后台权限码，`Require("system:...")` 对客户端身份一律拒绝。

### 5.2 注入 SurfaceAuth

使用 `--auth` 时本节跳过（生成的 Surface 已经内置 `middleware.Auth`）。自定义身份体系时，在 `internal/bootstrap/providers.go` 的 `provideRouter` 中构造认证中间件：

```go
surfaceTokens := buildSurfaceTokens(cfg, tokens)
return router.New(router.Deps{
    // ...
    SurfaceTokens: surfaceTokens,
    SurfaceAuth: map[string]gin.HandlerFunc{
        "client": middleware.Auth(
            surfaceTokens["client"], // issuer = jwt.issuer:client
            clientAuth,              // 实现 PrincipalLoader
            clientAuth,              // 实现 SessionChecker
        ),
    },
})
```

### 5.3 注册业务路由

`create api` 生成的 `client_v1.go` 已经注册了 Surface。公开登录接口可以写在它的 `Mount` 中，或者继续用 `create handler` 生成资源 handler：

```go
Mount: func(api, secured *gin.RouterGroup) {
    // 公开接口
    api.POST("/auth/login", clientAuthHandler.Login)
    api.POST("/auth/refresh", clientAuthHandler.Refresh)
    api.GET("/config", clientConfigHandler.Get)

    // 登录后接口；secured 只有在 SurfaceAuth["client"] 非空时才存在
    if secured != nil {
        secured.GET("/me", clientAuthHandler.Me)
        secured.GET("/orders", clientOrderHandler.List)
    }
},
```

客户端接口不需要 `middleware.Require("system:...")`，因为客户端 Principal 不参与 RBAC。需要区分普通用户和管理员时，在 service 层校验资源归属，不要复用后台权限码。

`go tool xadmin create handler` 生成的 CRUD 默认都放在 `RegisterSecured`；在 `SurfaceAuth["client"]` 接入之前，除 `/ping` 外的业务路由不会挂载，这是 fail-closed 设计。

### 5.4 限流

自定义 API 面默认使用独立桶：

```go
RateLimitKey: "rl:client:global"
```

登录、验证码等敏感接口再叠加更严格的桶，例如：

```go
api.POST("/auth/login", middleware.RateLimit(d.Cache, "rl:client-login", 10, time.Minute), handler.Login)
```

## 6. 接入商户端 API

```bash
go tool xadmin create api --name merchant --version v1
```

商户端认证通常不是用户名密码，而是 `app_id + sign`、API Key 或 mTLS。如果先用简化账号体系，可以直接：

```bash
go tool xadmin create api --name merchant --version v1 --auth --table merchant_users
```

如果不是用户名密码，生成的 `merchant_v1.go` 不限定认证方式，只需要提供一个 `gin.HandlerFunc`：

```go
// merchantAuth 校验请求头中的 X-Merchant-Key / X-Signature，
// 校验通过后把商户身份写入 authn.With(ctx, principal)。
SurfaceAuth: map[string]gin.HandlerFunc{
    "merchant": merchantAuth,
},
```

商户身份可以复用 `authn.Principal`（把商户 ID 放进 `UserID`，或在 service 层单独定义 `MerchantID`），也可以自己定义上下文类型。建议：

- 公开回调（支付、退款通知）放 `api`，但必须做签名校验和重放保护；
- 查询/下单等接口放 `secured`；
- 商户端限流键按商户 ID + IP 组合，而不是只按 IP；
- 审计日志要记录商户 ID，不要只记录 `Principal.UserID`。

## 7. 全局中间件与 API 面的关系

所有 API 面共享：

- `RequestID`、`Recovery`、`AccessLog`、`CORS`、`Audit`；
- `/uploads` 的安全响应头与鉴权；
- `/healthz`、`/swagger`。

每个面单独拥有：

- 路径前缀；
- 全局限流桶；
- 认证中间件；
- `secured` 子组。

审计中间件会跳过路径中的 `/auth/`，避免把客户端/商户端登录请求中的密码、验证码、签名写进审计表。

## 8. 部署

Nginx 默认只反代后台：

```nginx
location /admin/v1/ { proxy_pass http://backend:8080; }
```

生成 client/merchant 等面后，再补充对应 location：

```nginx
location /client/v1/   { proxy_pass http://backend:8080; }
location /merchant/v1/ { proxy_pass http://backend:8080; }
```

Vite 开发代理同理。客户端或商户端如果独立域名/独立前端，建议直接反代到对应前缀，不要把后台入口暴露给外部客户端。

## 9. 测试

最低覆盖：

1. **配置**：默认只有 admin、自定义面前缀、重复前缀/issuer 校验；
2. **路由**：启用的面返回 `/ping`，关闭的面返回 404，改前缀后旧路径 404；
3. **令牌**：自定义面 token 不能通过后台 `TokenManager.Parse`，反之亦然；
4. **业务**：登录、刷新、越权、资源归属校验；
5. **前端/客户端**：确认请求基地址、401 刷新逻辑和后台 SPA 不混用。

示例见 `tests/unit/router/surface_test.go` 与 `tests/unit/pkg/api_config_test.go`。

## 10. 安全清单

- [ ] 每个启用面使用不同 JWT issuer；没有把 `d.Tokens` 传给其他面；
- [ ] `--auth` 生成面的 `api.<name>.table` 唯一，迁移已执行；
- [ ] 自定义面有独立账号表或独立密钥体系，不复用后台管理账号；
- [ ] refresh token、失败锁定、验证码的缓存 key 带面名前缀；
- [ ] `secured` 路由全部挂在对应面的 `SurfaceAuth` 之后；
- [ ] 客户端 Principal 不携带后台权限码；
- [ ] 商户回调校验签名、时间戳和随机串，防重放；
- [ ] 每个面有独立限流桶，敏感接口再叠加更严格的限流；
- [ ] Nginx 只暴露需要对外的那几个前缀；
- [ ] 新增接口同步 OpenAPI 和本页的 API 面表格。
