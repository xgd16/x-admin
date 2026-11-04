# x-admin · 实现规格

> 通用 RBAC 管理后台。视觉采用 `design-preview/02-workbench.html` 的 **Workbench Liquid Glass**。
> 本文是实现契约：新增模块时先对齐这里，再写代码。

## 1. 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.27 · Gin · GORM · MySQL 8 · Redis 7 |
| 认证 | JWT access 2h + refresh 7d 轮换 |
| 迁移 | golang-migrate（embed 到二进制，启动时自动执行） |
| 文件 | 本地磁盘 `data/uploads`，`storage.Store` 接口可替换 |
| 文档 | Swagger / OpenAPI 3，`/swagger/index.html` |
| 前端 | React 19 · antd 6 · TypeScript 7 · Vite 8 · oxlint |
| 前端库 | React Router 7 · TanStack Query · Zustand · dayjs |
| 样式 | 原生 CSS 变量 + antd token，不引 Tailwind |
| 部署 | 前后端分离，Nginx 托管 SPA 并反代 `/admin/v1` |

## 2. 目录约定

```
cmd/server/               进程入口，只做装配
internal/
  bootstrap/              配置 → DB/Redis → repo → service → router
  config/                 YAML + 环境变量
  model/                  GORM 模型（仅数据结构）
  repository/             GORM 实现 + 通用 Base[T]
  service/                业务逻辑；repo 接口定义在 ports.go
  handler/                HTTP 绑定/校验/响应，不写业务
  middleware/             request-id / auth / permission / rate-limit / audit
  router/                 路由与依赖注入
  pkg/                    可复用、无业务依赖的基础模块
    apperr response pagination requestid security cache captcha storage excel tree
migrations/               版本化 SQL + seed
tests/                    所有 Go 测试（unit / integration）
frontend/                 React SPA
deploy/                   Docker Compose、Nginx
```

**硬规则**

- handler 不直接碰 GORM；service 不碰 `*gin.Context`。
- 可复用能力一律下沉到 `internal/pkg`，禁止复制粘贴分页、树、错误、存储逻辑。
- 每个 repository 只暴露 service 需要的方法；接口定义在 `service/ports.go`。
- 所有测试文件放在 `tests/`，不与被测代码同目录。
- 新增接口必须同时补：权限码、Swagger 注释、service 单测。

## 3. 数据表

| 表 | 说明 |
| --- | --- |
| `sys_users` | 账号、密码哈希、部门、状态、最后登录、TOTP 密钥与恢复码摘要 |
| `sys_roles` | 角色，`code` 唯一；`super_admin` 拥有全部权限 |
| `sys_menus` | 目录/菜单/按钮，`permission` 为权限码 |
| `sys_departments` | 部门树 |
| `sys_user_roles` | 用户-角色 |
| `sys_role_menus` | 角色-菜单 |
| `sys_dict_types` / `sys_dict_items` | 字典类型与字典项 |
| `sys_parameters` | 系统参数 |
| `sys_files` | 上传文件元数据 |
| `sys_file_shares` | 可撤销文件分享令牌摘要、有效期、撤销时间 |
| `sys_jobs` | 定时任务定义 |
| `sys_operation_logs` | 操作审计 |
| `sys_login_logs` | 登录审计 |
| `sys_notifications` | 站内通知，按 `user_id` 归属，`read_at` 判未读 |
| `sys_passkeys` | WebAuthn 通行密钥，一个用户可绑定多台设备，存公钥与签名计数器 |

所有业务表带 `id / created_at / updated_at / deleted_at`，软删除。

## 4. API

统一前缀 `/admin/v1`。响应：

```json
{ "code": 0, "message": "ok", "data": {}, "request_id": "..." }
```

分页 `data`：

```json
{ "list": [], "total": 0, "page": 1, "page_size": 20 }
```

错误码：`40001` 参数、`40101` 未登录、`40102` 登录失效、`40301` 无权限、
`40401` 不存在、`40901` 冲突、`42301` 锁定、`42901` 限流、`50001` 内部错误。

### 认证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/auth/captcha` | 返回 `captcha_id` + SVG data URL |
| POST | `/auth/login` | 用户名/密码/验证码，10 次/分/IP；开启双重验证时返回 `mfa_token` |
| POST | `/auth/login/mfa` | 动态验证码或恢复码换取 token，20 次/分/IP |
| POST | `/auth/passkey/login/begin` | 生成 discoverable credential 登录参数 |
| POST | `/auth/passkey/login/finish` | 验证通行密钥签名并签发 token |
| POST | `/auth/refresh` | refresh 轮换 |
| POST | `/auth/logout` | 删除 refresh 白名单 |
| GET | `/auth/me` | 当前用户 + 权限码 + 菜单树 |

### 个人中心安全设置

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/profile/totp` | 绑定状态、剩余恢复码数量 |
| POST | `/profile/totp/setup` | 生成 TOTP 密钥、otpauth 链接和二维码 |
| POST | `/profile/totp/enable` | 当前密码 + 6 位验证码，返回 10 个一次性恢复码 |
| DELETE | `/profile/totp` | 当前密码 + 动态验证码/恢复码，关闭双重验证 |
| GET | `/profile/passkeys` | 当前用户已绑定的设备列表 |
| POST | `/profile/passkeys/begin` | 当前密码校验通过后生成绑定参数 |
| POST | `/profile/passkeys/finish` | 校验 attestation 并保存设备公钥 |
| PUT | `/profile/passkeys/:id` | 重命名设备 |
| DELETE | `/profile/passkeys/:id` | 删除设备（物理删除） |

### 业务

| 模块 | 路径 | 权限码前缀 |
| --- | --- | --- |
| 看板 | `GET /dashboard/summary` | `dashboard:view` |
| 用户 | `/users`、`/users/:id`、`/users/:id/roles`、`/users/:id/password`、`/users/:id/status`、`/users/import`、`/users/export`、`/users/template` | `system:user:*` |
| 角色 | `/roles`、`/roles/:id`、`/roles/:id/menus` | `system:role:*` |
| 菜单 | `/menus/tree`、`/menus`、`/menus/:id` | `system:menu:*` |
| 部门 | `/departments/tree`、`/departments`、`/departments/:id` | `system:dept:*` |
| 字典 | `/dict/types`、`/dict/types/:id/items`、`/dict/items/:id` | `system:dict:*` |
| 参数 | `/parameters`、`/parameters/:id` | `system:param:*` |
| 文件 | `POST /files`、`GET /files`、`GET /files/:id/download`、`DELETE /files/:id` | `system:file:*` |
| 任务 | `/jobs`、`/jobs/:id`、`/jobs/:id/status`、`/jobs/:id/run` | `system:job:*` |
| 操作日志 | `/logs/operations`、`DELETE /logs/operations`、`DELETE /logs/operations/:id` | `system:log:operation` |
| 登录日志 | `/logs/logins`、`DELETE /logs/logins` | `system:log:login` |
| 个人中心 | `GET/PUT /profile`、`PUT /profile/password` | 登录即可 |
| 站内通知 | `/notifications`、`/notifications/unread-count`、`/notifications/read-all`、`/notifications/:id/read` | 登录即可 |

列表统一支持 `page`、`page_size`、`keyword`，按模块追加 `dept_id`、`status`、`role_id` 等。文件列表额外支持 `mime` 前缀和 `kind`（`image` / `video` / `audio` / `document` / `archive` / `other`，可逗号组合）。

## 5. 权限码

菜单/按钮权限统一 `模块:资源:动作`：

```
dashboard:view
system:user:list create update delete reset import export
system:role:list create update delete assign
system:menu:list create update delete
system:dept:list create update delete
system:dict:list create update delete
system:param:list create update delete
system:file:list upload delete download
system:job:list create update delete run
system:log:operation  system:log:login
```

`super_admin` 角色跳过权限判断；按钮级权限由前端 `hasPermission()` 控制显隐。

## 6. 认证与安全

- bcrypt cost 10；密码最少 8 位，必须含字母和数字。
- access token HS256，2h；claims 含 `uid`、`username`。
- refresh token 为 32 字节随机串，Redis key `refresh:{token}` → `uid:session_version:session_id:remember`；记住我 7d、普通会话 12h；每次刷新删除旧 token。
- 验证码 5 分钟，Redis `captcha:{id}`，校验后立即删除。
- 登录失败 `login:fail:{username}` 计数，5 次锁定 15 分钟；成功清零。
- 双重验证：TOTP（RFC 6238，SHA1 / 6 位 / 30 秒，±1 步容差），密钥仅服务端可见；恢复码为 10 个一次性随机码，只存 SHA-256 摘要。
- 两步登录：密码通过后签发 5 分钟 `mfa:challenge:{token}`，验证成功后立即删除；单个挑战错误 5 次即作废。开启 / 关闭双重验证都需要当前密码。
- refresh token 值携带 `uid:session_version:session_id:remember`；开启双重验证时递增版本，其他设备 refresh 立即失效，当前设备的 token 同步更新版本。
- 通行密钥：WebAuthn / Passkey（go-webauthn），注册与登录挑战存 Redis 5 分钟且一次性使用；注册需当前密码，最多 10 台设备；只保存公钥、attestation 和签名计数器，私钥留在设备。
- RP ID / Origin 由 `X_ADMIN_WEBAUTHN_RP_ID`、`X_ADMIN_WEBAUTHN_RP_ORIGINS` 配置；Passkey 仅在 localhost 或 HTTPS 域名下可用，不支持 IP 地址。
- 限流：全局 100/min/IP，登录 10/min/IP，二次验证 20/min/IP，Passkey 20/min/IP，Redis 计数。
- 审计：写操作自动记录操作人、路径、IP、状态、耗时、请求体（密码字段脱敏）。

## 7. 前端

- 布局：悬浮玻璃图标轨（随主题切换浅色/深色）+ 浅色玻璃上下文栏 + 玻璃内容面板。
- 页面：登录、看板、用户、角色、菜单、部门、字典、参数、文件、任务、操作日志、登录日志、个人中心。
- 个人中心：资料、密码、Google Authenticator 双重验证、通行密钥（Passkey）设备列表 / 添加 / 重命名 / 删除；登录页支持密码 + 验证码或 Passkey 免密码进入。
- 交互：多标签页、暗色模式、表格列设置/密度、全局水印。
- 文件复用：`FilePicker`（选择弹窗，返回 `FileItem[]`）、`FileSelect`（antd Form 字段，返回 URL 或 URL[]），支持 `types` 限定类型、`multiple` / `limit` 控制多选。
- 主题 token：`#edf3f9` 背景、`#0b1c2c` ink、`#ff5f33` 主色、`#8b7cf6` violet、`#2fb8d9` cyan、`#2fb98a` mint。
- 字体：Avenir Next / SF Pro Display / PingFang SC；数字 `tabular-nums`。
- 不做 i18n。

## 8. 测试

- 全部在 `tests/`：`unit/pkg`、`unit/service`、`unit/handler`、`unit/repository`、`integration`。
- service 通过 `ports.go` 接口注入 fake；repository 用 SQLite 内存库。
- 目标：`internal/pkg` 与 `internal/service` 覆盖率 100%，整体不低于 90%。
- 命令：`make test`、`make coverage`。

## 9. 种子数据

- 账号 `admin` / `Admin@123456`（bcrypt）。
- 角色 `super_admin`（超级管理员）、`operator`（运营）。
- 部门：总公司 → 技术部 / 运营部 / 财务部。
- 菜单：上述全部模块 + 按钮权限。
- 字典：`user_status`、`job_status`、`menu_type`。
- 参数：`site.name`、`site.logo`、`upload.max_size`。
