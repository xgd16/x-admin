# 基础库（internal/pkg）

`internal/pkg` 放无业务依赖、可被任何层复用的基础能力。这一页是速查手册，详细行为以源码注释为准。

## 1. apperr — 业务错误码

```go
return apperr.New(apperr.CodeConflict, "角色编码已存在")
return apperr.Newf(apperr.CodeLocked, "账号锁定 %d 分钟", 15)
return apperr.Wrap(err, apperr.CodeInternal, "文件保存失败")
```

| 常量 | 值 | HTTP | 含义 |
| --- | --- | --- | --- |
| `CodeOK` | 0 | 200/201 | 成功 |
| `CodeInvalidArgument` | 40001 | 400 | 参数错误 |
| `CodeUnauthorized` | 40101 | 401 | 未登录 |
| `CodeTokenExpired` | 40102 | 401 | 登录失效 |
| `CodeForbidden` | 40301 | 403 | 无权限 |
| `CodeNotFound` | 40401 | 404 | 不存在 |
| `CodeConflict` | 40901 | 409 | 冲突 |
| `CodeLocked` | 42301 | 423 | 账号锁定 |
| `CodeTooManyRequests` | 42901 | 429 | 限流 |
| `CodeInternal` | 50001 | 500 | 内部错误 |

- `apperr.From(err)` 提取业务错误；未知错误统一包装为 `50001`，不泄露内部信息。
- `apperr.Wrap` 的 `cause` 只用于服务端日志，不返回客户端。
- 预置 `ErrUnauthorized`、`ErrForbidden`、`ErrNotFound`。

## 2. response — 统一响应

```go
response.OK(c, data)          // 200
response.Created(c, data)     // 201
response.Page(c, list, total, page, pageSize)
response.NoContent(c)         // 204
response.Error(c, err)        // 按 apperr 映射状态码
```

信封：

```json
{ "code": 0, "message": "ok", "data": {}, "request_id": "…" }
```

分页 `data`：`{list,total,page,page_size}`。`response.Error` 会：

- 把 `50001` 的消息改为「服务器内部错误」。
- 把错误摘要写入 `c.Errors` 供审计。
- 5xx 写结构化错误日志（含 request_id、方法、路径、原始错误）。

## 3. pagination — 分页

```go
type Query struct {
    Page     int    `form:"page"`
    PageSize int    `form:"page_size"`
    Keyword  string `form:"keyword"`
}
```

- `Normalize()`：页码至少 1，页大小默认 20，上限 200。
- `Offset()`：`(page-1)*pageSize`。
- 列表过滤器内嵌 `pagination.Query`，handler 里调用 `f.Normalize()`，仓储里用 `paginate`。

## 4. authn — 请求身份

```go
type Principal struct {
    UserID, Username, Nickname, SessionVersion, SessionID
    Super       bool
    Permissions map[string]struct{}
}

p.Has("system:user:list") // super_admin 恒为 true
authn.With(ctx, p)
authn.FromContext(ctx)
```

handler 里用 `CurrentUserID(c)` / `CurrentPrincipal(c)` / `CurrentSessionID(c)`（`internal/handler/helpers.go`）。

## 5. cache — 缓存抽象

```go
type Store interface {
    Set(ctx, key, value string, ttl time.Duration) error
    Get(ctx, key string) (string, error)          // 不存在返回 ErrNotFound
    Take(ctx, key string) (string, error)         // 原子读+删
    Del(ctx, keys ...string) error
    Exists(ctx, key string) (bool, error)
    Keys(ctx, prefix string) ([]string, error)    // 只返回未过期键
    SetNX(ctx, key, value string, ttl time.Duration) (bool, error)
    Incr(ctx, key string, ttl time.Duration) (int64, error)
    Ping(ctx) error
    Close() error
}
```

- `cache.NewRedis(client)`：生产默认。
- `cache.NewMemory()`：并发安全，进程内；重启丢失 refresh/会话/验证码，仅开发或单副本。
- `Incr` 首次写入时设置 TTL，用于固定窗口限流。
- 应用内键前缀：`refresh:`、`session:`、`login:fail:`、`unlock:fail:`、`mfa:*`、`captcha:`、`job:lock:`、`passkey:*`。

## 6. captcha — 图形验证码

```go
gen := captcha.New(store, 5*time.Minute)
challenge, _ := gen.Generate(ctx)
ok := gen.Verify(ctx, challenge.ID, answer)
```

- 默认 4 位字符，去掉 `I O 0 1` 等易混淆字符。
- 返回 SVG data URL，无需图片服务。
- 答案存 `captcha:<id>`，**校验一次即删除**，失败也会消费，防重放。
- 可通过 `WithReader` / `WithRand` / `WithClock` / `WithIDFunc` / `WithLength` 注入测试替身。

## 7. security — JWT 与密码

### 7.1 TokenManager

```go
tokens := security.NewTokenManager(secret, issuer, accessTTL)
access, expiresAt, _ := tokens.Issue(userID, username, sessionVersion, sessionID)
claims, err := tokens.Parse(raw)
```

- HS256，解析时校验签名、issuer、exp。
- `Claims`：`uid`、`username`、`sv`、`sid`、`iss`、`exp`、`jti`。
- 另有 `IssueFileAccess` / `ParseFileAccess` 用于 `/uploads` Cookie；Cookie 名 `x_admin_files`。

### 7.2 随机串与哈希

```go
token, _ := security.RandomToken()   // 32 字节 URL-safe
hash := security.HashToken(token)    // SHA-256 hex
```

refresh token、分享令牌、会话 ID 都用这套。

### 7.3 密码

```go
hasher := security.NewBcryptHasher(cost)
hash, _ := hasher.Hash(plain)
ok := hasher.Verify(hash, plain)
```

`PasswordHasher` 是接口，测试可用低成本实现。

## 8. storage — 文件存储

```go
type Store interface {
    Save(ctx, filename string, r io.Reader) (Info, error)
    Open(ctx, key string) (io.ReadCloser, error)
    Delete(ctx, key string) error
    URL(key string) string
}
```

- `NewLocal(root, urlPrefix)`：布局 `YYYY/MM/<random>.<ext>`，扩展名清洗，路径穿越校验。
- 测试可用 `WithClock` / `WithIDFunc` 固定时间与文件名。
- 替换对象存储时实现该接口并在 `provideStorage` 注入。

## 9. excel — Excel 读写

### 9.1 小数据量

```go
data, err := excel.Write("用户", headers, rows) // [][]string
rows, err := excel.Read(reader)                 // [][]string，跳过空行
```

### 9.2 流式导出

```go
book, _ := excel.NewStreamBook("用户列表", headers)
defer book.Close()
for rows.Next() {
    _ = book.WriteRow([]any{"张三", 18, time.Now()})
}
_ = book.SaveTo(writer)
```

- 基于 excelize `StreamWriter`，内存占用与总行数无关。
- 单表写满自动追加新工作表，表头重复。
- `Rows()` 返回累计数据行数。

## 10. sysinfo — 主机与运行时

```go
snap := sysinfo.Collect(ctx)
```

包含 Host / CPU / Memory / Swap / Load / Disks / Network / Process，以及 `DatabaseStats` 类型（连接池指标）。监控服务在此之上补充 Go runtime 与缓存探测。容器内读到的是容器视角。

## 11. totp — 双重验证

```go
secret, _ := totp.GenerateSecret()
url := totp.ProvisioningURL(issuer, account, secret)
qr, _ := totp.QRCodeDataURL(url)
ok := totp.Validate(secret, code, time.Now())
ok = totp.ValidateSkew(secret, code, time.Now(), 1)
codes, _ := totp.GenerateRecoveryCodes()
hash := totp.HashRecoveryCode(code)
```

固定 SHA1 / 6 位 / 30 秒；`Validate` 允许 ±1 步偏差。恢复码只存 SHA-256 摘要。

## 12. tree — 扁平列表转树

```go
nodes := tree.Build(items, tree.Options[*Node]{
    ID:          func(n *Node) uint64 { return n.ID },
    ParentID:    func(n *Node) uint64 { return n.ParentID },
    SetChildren: func(n *Node, kids []*Node) { n.Children = kids },
})
```

- 保持输入顺序。
- 孤儿、自引用、环都会被提升为根节点，保证数据不丢失。
- 菜单树、部门树都用它。

## 13. requestid — 请求 ID

```go
requestid.New()
requestid.With(ctx, id)
requestid.FromContext(ctx)
```

- 中间件优先复用请求头 `X-Request-ID`，否则生成。
- 响应体 `request_id` 与访问日志、错误日志、操作日志共用同一个值。

## 14. 使用约定

- `pkg` 不 import `internal/service` / `internal/model` / Gin（`response` 除外，它本身就是 HTTP 响应封装）。
- 需要配置的能力通过参数传入，不要在 pkg 里读全局配置。
- 需要测试替换的依赖（时钟、随机源、ID）用 `Option` 注入。
- 新增 pkg 时先问：它是否与业务无关？会被两个以上模块复用？否则放到使用它的 service 里。
