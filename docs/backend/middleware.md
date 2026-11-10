# 中间件与请求治理

中间件全部在 `internal/middleware`，由 `internal/router/router.go` 按固定顺序注册。**顺序本身就是契约**，调整前先确认依赖关系。

## 1. 注册顺序

```go
r.Use(RequestID(), Recovery(), AccessLog(), CORS(cfg.CORS))
r.Use(Audit(recorder))
r.Use(UploadSecurityHeaders())
r.Use(UploadAuth(tokens, auth, shares))

api := r.Group("/admin/v1")
api.Use(RateLimit(store, "rl:global", 100, time.Minute))

secured := api.Group("")
secured.Use(Auth(tokens, auth, auth))
// 每个端点再挂 Require("system:xxx:yyy")
```

| 顺序 | 中间件 | 作用 | 失败行为 |
| --- | --- | --- | --- |
| 1 | `RequestID` | 复用/生成 `X-Request-ID`，写入响应头与 context | — |
| 2 | `Recovery` | 捕获 panic，记录堆栈 | 500，已写响应则只 abort |
| 3 | `AccessLog` | 请求结束输出方法、路径、状态、耗时、IP | — |
| 4 | `CORS` | 白名单来源、预检直返 204 | 非白名单不加头 |
| 5 | `Audit` | 复制写请求体，异步落审计表 | 不阻塞请求 |
| 6 | `UploadSecurityHeaders` | `/uploads` 加 `nosniff`、CSP sandbox、CORP | — |
| 7 | `UploadAuth` | `/uploads` 的 Cookie / Bearer / 分享令牌鉴权 | 401 |
| 8 | `RateLimit` | 固定窗口按 IP 限流 | 429，fail-open |
| 9 | `Auth` | JWT + 用户加载 + session_version + 会话存在性 | 401 |
| 10 | `Require` | 权限码判断 | 403 |

> `RequestID` 必须最先，`Recovery` 紧随其后，否则日志拿不到 ID、panic 也兜不住。

## 2. 各中间件说明

### 2.1 RequestID

- 读取请求头 `X-Request-ID`（`requestid.Header`），为空或超过 64 字符时生成新 ID。
- 写入响应头 + `request.Context()`。
- 响应体 `request_id`、访问日志、错误日志、操作日志都从这里取。

### 2.2 Recovery + AccessLog

- panic 时记录 `request_id / method / path / panic / stack`，返回统一 500；如果响应已经写出，只 `Abort`，避免重复写头。
- AccessLog 在 `c.Next()` 之后读取最终状态码与耗时，因此放在 Recovery 之后才能记录到 panic 产生的 500。

### 2.3 CORS

- 白名单来自 `cors.origins`；命中后回显具体 Origin 并加 `Vary: Origin`。
- 配置 `"*"` 时回显 `*`，不携带凭证假设；生产建议写明确来源。
- 允许头：`Authorization, Content-Type, X-Request-ID`；暴露头：`X-Request-ID, Content-Disposition`。
- 所有 `OPTIONS` 请求直接 204，不再进入业务链路。

### 2.4 Audit

- 只审计 `POST/PUT/PATCH/DELETE`，排除 `/admin/v1/auth/*`。
- 请求体最多复制 64KB，落库前脱敏并截断到 4KB。
- 敏感字段：名字包含 `password/token/secret/captcha` 或等于 `otp_code`。
- 非 JSON 请求体记录 `[non-json body]`。
- 通过 `OperationRecorder` 异步投递；队列满丢弃，不阻塞。

详见 [日志与审计](../backend/audit.md)。

### 2.5 UploadSecurityHeaders + UploadAuth

- 只作用于 `/uploads` 及其子路径。
- 安全头：`X-Content-Type-Options: nosniff`、`Content-Security-Policy: default-src 'none'; sandbox`、`Cross-Origin-Resource-Policy: same-origin`。
- 鉴权优先级：分享令牌（`?share=`，命中后不再回退登录态）→ `Authorization: Bearer` → HttpOnly Cookie `x_admin_files`。
- 所有失败响应加 `Cache-Control: no-store`，撤销立即生效。

详见 [文件、存储与分享](../backend/files.md)。

### 2.6 RateLimit

- 固定窗口计数器：`Incr(prefix:ip, window)`，首次写入设置 TTL。
- 响应头：`X-RateLimit-Limit`、`X-RateLimit-Remaining`；超限加 `Retry-After`。
- 缓存故障时 **fail-open**（记录警告后放行），避免 Redis 故障拖垮整个 API。
- 当前限流：

| 前缀 | 额度 | 范围 |
| --- | --- | --- |
| `rl:global` | 100/min/IP | 整个 `/admin/v1` |
| `rl:login` | 10/min/IP | 登录 |
| `rl:mfa` | 20/min/IP | 二次验证 |
| `rl:passkey` | 20/min/IP | Passkey 登录/解锁 |
| `rl:unlock` | 10/min/IP | 锁屏解锁 |

### 2.7 Auth

- 解析 `Authorization: Bearer <token>`，要求恰好两段。
- 区分「未登录」`40101`、「凭证无效」`40101`、「已过期」`40102`。
- 每次请求重新加载用户（停用/删除立即生效）。
- 校验 `session_version` 与会话存在性，撤销设备后 access token 立即失效。
- 成功后把 `Principal` 写入 context，并补上 `SessionID`。

### 2.8 Require

- 读 context 里的 `Principal`，调用 `Has(permission)`。
- `super_admin` 恒 true；无权限返回 `40301`。
- 只在 `secured` 分组内使用；公开端点不要挂。

## 3. 新增中间件

1. 在 `internal/middleware` 新建 `xxx.go`，返回 `gin.HandlerFunc`。
2. 想清楚放在全局还是路由组：全局中间件会影响 `/healthz`、`/swagger`、`/uploads`。
3. 在 `router.New` 或对应 group 注册，保持顺序合理。
4. 失败必须用 `response.Error(c, ...)` 并 `return`，不要直接 `c.JSON` 绕过统一信封。
5. 需要依赖时通过 `router.Deps` 注入，不要在中间件里 new 仓储。
6. 加测试：`tests/unit/middleware` 已有直接构造 `gin.Context` 的模式。

## 4. 安全边界

- **限流是 IP 级**，不是用户级；登录接口同时有账号锁定（按用户名）。
- **CORS 不是鉴权**；即使同源，也必须走 Auth/Require。
- **`trusted_proxies` 必须配置正确**，否则 Nginx 后所有请求的 `ClientIP` 都是容器 IP，限流与审计会失真。
- **审计日志不能替代访问日志**：前者只覆盖写操作且会脱敏，后者是全量请求。
- **上传目录的安全头不能省**，否则同源打开 HTML/SVG 可能造成存储型 XSS。

## 5. 常见问题

| 现象 | 排查 |
| --- | --- |
| 所有请求 IP 都一样 | `server.trusted_proxies` 未包含 Nginx 网段 |
| 预检 404 | 请求没走 CORS 中间件，或路径不在 Gin 路由内 |
| 限流不生效 | `redis.driver=memory` 且多副本；或缓存异常导致 fail-open |
| `request_id` 为空 | 响应来自 Nginx/网关而非应用；检查代理是否透传 |
| 上传 401 但接口正常 | 文件 Cookie 过期/路径不对；确认 `Path=/uploads` 且重新 `/auth/me` |
| 审计表没有记录 | 方法不是写方法、路径是 `/auth/*`、或队列满被丢弃 |
