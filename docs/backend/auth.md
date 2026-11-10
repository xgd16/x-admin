# 认证、会话与二次验证

认证相关代码集中在 `internal/service/auth.go`、`session.go`、`passkey.go`、`profile.go`，协议层在 `internal/handler/auth.go` 与 `profile.go`，令牌基础设施在 `internal/pkg/security`。

## 1. 身份模型

| 数据 | 位置 | 说明 |
| --- | --- | --- |
| 账号 | `sys_users` | 用户名唯一；密码只存 bcrypt 摘要；`session_version` 控制旧令牌失效 |
| 角色 | `sys_roles` / `sys_user_roles` | `super_admin` 是硬编码的超级管理员入口 |
| 权限 | `sys_menus.permission` | 角色授权菜单后得到权限码 |
| TOTP | `sys_users.totp_*` | 密钥与恢复码摘要只存服务端，`json:"-"` 永不下发 |
| Passkey | `sys_passkeys` | 公钥凭据与签名计数器，私钥在用户设备 |
| 会话 | 缓存 `session:<uid>:<sid>` | IP、UA、版本、记住我、首次/最近活动时间 |
| refresh | 缓存 `refresh:<token>` | 一次性白名单，值为 `uid:version:sid:remember` |

## 2. 登录流程

### 2.1 账号密码（未开启 TOTP）

```text
GET  /auth/captcha        → {captcha_id, image, expires_in}
POST /auth/login          → captcha 校验 → 锁定检查 → 用户查询 → 状态检查
                             → bcrypt 校验 → 清零失败计数 → 签发令牌
```

成功后：

1. 更新 `last_login_at` / `last_login_ip`。
2. 写一条登录日志（状态 200，消息「登录成功」）。
3. 发「新登录提醒」站内通知（IP + 设备）。
4. 生成随机 `session_id`，写入会话记录。
5. 签发 access token（HS256）与 refresh token（32 字节随机串）。
6. handler 额外下发 `/uploads` 专用的 HttpOnly 文件访问 Cookie。

请求体：

```json
{
  "username": "admin",
  "password": "Admin@123456",
  "captcha_id": "…",
  "captcha_code": "AB3D",
  "remember_me": false
}
```

响应：

```json
{
  "access_token": "…",
  "refresh_token": "…",
  "token_type": "Bearer",
  "expires_in": 7200
}
```

### 2.2 开启 TOTP 后

密码正确不会直接签发令牌，而是返回 5 分钟的一次性挑战：

```json
{ "mfa_required": true, "mfa_token": "…", "mfa_expires_in": 300, "remember_me": false }
```

前端继续调用 `POST /auth/login/mfa`：

```json
{ "mfa_token": "…", "otp_code": "123456", "remember_me": false }
```

`otp_code` 可以是 6 位动态码，也可以是一次性恢复码。单个挑战连续错误 5 次即作废，必须重新登录。

### 2.3 Passkey 无用户名登录

```text
POST /auth/passkey/login/begin   → 返回 session_id + WebAuthn 断言参数
浏览器 navigator.credentials.get(...)
POST /auth/passkey/login/finish  → {session_id, credential, remember_me}
```

服务端从凭据的用户句柄定位账号，校验签名与计数器，然后走与密码登录相同的收尾（登录日志、通知、会话、令牌）。Passkey 优先于 TOTP：绑定 Passkey 的账号可以免密码登录。

## 3. 令牌与刷新

### 3.1 access token

```json
{
  "uid": 1,
  "username": "admin",
  "sv": 0,
  "sid": "…",
  "iss": "x-admin",
  "exp": 1730000000,
  "jti": "…"
}
```

- 默认 2 小时（`jwt.access_ttl`）。
- 中间件每次请求都校验 `sv` 与 `sid`，所以改密、重置密码、启停账号、撤销设备后旧 access token 立即失效。

### 3.2 refresh token

- 32 字节随机串，数据库不落库，只存在缓存白名单。
- 值为 `uid:session_version:session_id:remember`。
- 刷新时用 `Take` **原子读取并删除**，旧 token 立刻作废，随后签发新 token 对（轮换）。
- `remember_me=true` 使用 `jwt.refresh_ttl`（默认 168h），否则使用 `jwt.session_ttl`（默认 12h）。

### 3.3 会话与设备

- 每次登录生成独立 `session_id`，缓存键 `session:<uid>:<sid>`。
- 个人中心 `/profile/sessions` 只展示当前 `session_version` 的会话；版本过期的记录会被顺手清理。
- 撤销单设备：`DELETE /profile/sessions/:id`。
- 撤销其他设备 / 全部设备：`POST /profile/session-actions/revoke-others|revoke-all`。
- 管理员强制下线：`DELETE /online-sessions/:userID`，请求体带 `session_id` 只踢单设备，不带则踢该用户全部会话。

详见 [在线会话与系统监控](monitor.md)。

## 4. 锁定、失败计数与解锁

| 策略 | 默认 | 配置 |
| --- | --- | --- |
| 连续密码错误 | 5 次 | `security.max_login_fails` |
| 锁定时长 | 15 分钟 | `security.lock_duration` |
| 验证码有效期 | 5 分钟，一次性 | `security.captcha_ttl` |
| 密码最小长度 | 8，且必须含字母和数字 | `security.password_min_len` |
| bcrypt cost | 10 | `security.bcrypt_cost` |

失败计数按**用户名**存在缓存（`login:fail:<username>`），达到上限后即使密码正确也先返回 `42301`。锁屏解锁使用独立的 `unlock:fail:<uid>`，上限与锁定时长复用同一组配置，避免锁屏尝试和登录锁定互相干扰。

管理员重置密码、用户改密、启停账号都会递增 `session_version`，旧会话立即失效。

## 5. 双重验证（TOTP）

流程：

1. `POST /profile/totp/setup`：生成新密钥，返回 `secret`、`otpauth_url`、二维码 data URL；此时 `totp_enabled=false`。
2. 用户用 Google Authenticator 等扫码。
3. `POST /profile/totp/enable`：提交当前密码 + 6 位动态码 + 当前 refresh token。
4. 成功后返回 **10 个一次性恢复码**（只返回这一次，数据库只存 SHA-256 摘要），递增 `session_version`，撤销其他设备，当前设备通过 `RefreshSession` 保持登录。
5. 关闭：`DELETE /profile/totp`，需密码 + 动态码或恢复码。

TOTP 参数固定为 RFC 6238：SHA1、6 位、30 秒、±1 步容差。恢复码消费后会从摘要列表中移除；`GET /profile/totp` 返回剩余数量。

## 6. Passkey（WebAuthn）

| 规则 | 值 |
| --- | --- |
| 单用户设备上限 | 10 |
| 设备名长度 | ≤ 64 字符 |
| 挑战有效期 | 5 分钟，一次性 |
| 注册要求 | 当前密码 |
| 登录方式 | discoverable credential（无需先输用户名） |
| 解锁 | 用户绑定的 Passkey 断言 |

流程：

```text
POST /profile/passkeys/begin      密码校验 → 注册参数
POST /profile/passkeys/finish     {session_id, name, credential}
PUT  /profile/passkeys/:id        重命名
DELETE /profile/passkeys/:id      删除设备
```

配置（`webauthn.*`）：

```yaml
webauthn:
  rp_display_name: x-admin
  rp_id: example.com            # 有效域名，不带协议/端口
  rp_origins:
    - https://example.com       # 生产必须 HTTPS
```

`rp_id` 与 `rp_origins` 不匹配时应用仍会启动，但 Passkey 功能会被禁用（启动日志有 `passkey support disabled`）。

## 7. 锁屏解锁

- `POST /auth/unlock`：密码解锁，不签发新令牌。
- `POST /auth/unlock/passkey/begin|finish`：用当前用户已绑定的 Passkey 解锁。
- 解锁失败计数独立，连续失败达到上限后返回 `42301`。

前端 `useIdleLock` 根据 `ui.lockIdleMinutes` 在闲置后覆盖锁屏层；解锁只恢复界面，不改变登录态。

## 8. 文件访问 Cookie

`/uploads` 不是公开静态目录。登录、刷新、`/auth/me` 时服务端会下发：

```text
Set-Cookie: x_admin_file_access=…; Path=/uploads; HttpOnly; SameSite=Lax
```

- 浏览器 `<img>` 等标签自动携带，无需改业务代码。
- API 客户端也可以直接用 `Authorization: Bearer <access>` 访问 `/uploads`。
- Cookie 内含 `uid / sv / sid`，撤销会话或递增版本后立即失效。
- 生产环境且请求为 HTTPS 时才加 `Secure`；本地 HTTP 不加，避免 Cookie 存不下来。
- 分享链接使用 `?share=<token>`，与登录态互斥，撤销后立即失效。

## 9. 接口一览

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/auth/captcha` | 公开 | 验证码 |
| POST | `/auth/login` | 公开 | 密码登录（10/min/IP） |
| POST | `/auth/login/mfa` | 公开 | 二次验证（20/min/IP） |
| POST | `/auth/passkey/login/begin|finish` | 公开 | Passkey 登录 |
| POST | `/auth/refresh` | 公开 | 轮换 refresh token |
| POST | `/auth/logout` | 登录 | 注销当前会话 |
| GET | `/auth/me` | 登录 | 用户 + 权限 + 菜单树，并续期文件 Cookie |
| POST | `/auth/unlock` | 登录 | 密码解锁（10/min/IP） |
| GET/PUT | `/profile` | 登录 | 个人资料 |
| PUT | `/profile/password` | 登录 | 修改密码 |
| GET/DELETE | `/profile/sessions`、`/profile/sessions/:id` | 登录 | 设备列表 / 踢设备 |
| GET/POST/DELETE | `/profile/totp*` | 登录 | TOTP 绑定、开启、关闭 |
| GET/POST/PUT/DELETE | `/profile/passkeys*` | 登录 | Passkey 设备管理 |

## 10. 扩展新认证体系

需要给 C 端用户做一套完全隔离的登录时，不要复用 `sys_users`。推荐模式：

- 独立用户表与独立 `TokenManager`（不同 issuer）。
- 缓存键加前缀（例如 `front:refresh:`），避免与后台会话串号。
- 独立中间件与 `Principal` 加载逻辑，权限码可以为空并 fail-closed。

完整演示见 [前台用户认证](../front-auth.md)。

## 11. 常见问题

| 现象 | 原因 |
| --- | --- |
| 登录返回 `42301` | 该用户名连续失败达到上限，等待锁定时长或清缓存 `login:fail:<username>` |
| 刷新返回 `40102` | refresh token 已被使用/过期，或 `session_version` 已变化 |
| 请求突然全部 `40102` | 账号被停用、密码被重置、TOTP 被开启/关闭、其他设备执行了安全操作 |
| 图片全部 401 | 文件访问 Cookie 缺失或过期，重新调用 `/auth/me` |
| Passkey 按钮不可用 | RP ID / Origin 配置不匹配，或当前环境不是 HTTPS/localhost |
| MFA 一直失败 | 服务器时间偏差超过 30 秒，或恢复码已被使用 |
