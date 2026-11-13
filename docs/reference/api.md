# API 一览与错误码

默认后台前缀 `/admin/v1`。框架支持多个独立 API 面，但默认只挂载 admin；client、merchant、partner 等面由 `go tool xadmin create api` 生成。每个面有独立前缀、限流桶和 JWT issuer，令牌互不通用。路由与权限码的**唯一事实来源**是 `internal/router/`；Swagger 定义在 `internal/docs/openapi.yaml`，覆盖核心接口，但异步导出与日志导出以本文和路由为准。

## 0. API 面

| API 面 | 默认前缀 | JWT issuer | 状态 |
| --- | --- | --- | --- |
| `admin` | `/admin/v1` | `jwt.issuer` | 默认唯一内置面，后台接口已实现 |
| 自定义面 | `create api` 指定 | `jwt.issuer:<name>` | 例如 client、merchant、partner，由用户生成 |

所有面都提供公开的 `GET /ping`，返回 `{api,status}`。生成、启用或修改前缀见 [多 API 模式](../guide/multi-api.md) 与 [代码生成](../tool/create.md)。

`create api --auth` 生成的自定义面额外提供：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/auth/register` | 注册并返回 token |
| POST | `/auth/login` | 登录 |
| POST | `/auth/refresh` | 刷新 token |
| POST | `/auth/logout` | 退出登录（需要 access token） |
| GET | `/auth/me` | 当前用户资料（需要 access token） |

## 1. 通用约定

### 1.1 认证

```http
Authorization: Bearer <access_token>
```

- 除公开接口外都需要 Bearer。
- access token 默认 2 小时；过期用 refresh token 换新（轮换）。
- `/uploads` 另支持 HttpOnly Cookie 和 `?share=` 分享令牌。

### 1.2 响应信封

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "request_id": "a1b2c3"
}
```

分页 `data`：

```json
{ "list": [], "total": 0, "page": 1, "page_size": 20 }
```

### 1.3 分页与筛选

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `page` | 1 | 页码，最小 1 |
| `page_size` | 20 | 页大小，1–200 |
| `keyword` | — | 各模块按需支持模糊搜索 |
| `status` | — | 0 停用 / 1 启用 |
| `start` / `end` | — | 时间范围（日志） |

### 1.4 错误码

| code | HTTP | 含义 |
| --- | --- | --- |
| 0 | 200/201 | 成功 |
| 40001 | 400 | 参数错误 |
| 40101 | 401 | 未登录 / 凭证无效 |
| 40102 | 401 | 登录失效 / 已过期 |
| 40301 | 403 | 无权限 |
| 40401 | 404 | 不存在 |
| 40901 | 409 | 冲突 |
| 42301 | 423 | 账号锁定 |
| 42901 | 429 | 限流 |
| 50001 | 500 | 内部错误（消息统一为「服务器内部错误」） |

限流响应头：`X-RateLimit-Limit`、`X-RateLimit-Remaining`、`Retry-After`。

## 2. 公开接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/auth/captcha` | `{captcha_id,image,expires_in}`（SVG data URL） |
| POST | `/auth/login` | 用户名/密码/验证码/`remember_me` |
| POST | `/auth/login/mfa` | `mfa_token` + `otp_code`（动态码或恢复码） |
| POST | `/auth/passkey/login/begin` | 返回 `session_id` + WebAuthn 选项 |
| POST | `/auth/passkey/login/finish` | `session_id` + `credential` + `remember_me` |
| POST | `/auth/refresh` | `refresh_token` → 新 token 对 |
| GET | `/parameters/public` | `public=true` 的系统参数 |
| GET | `/healthz` | DB + 缓存健康检查，异常 503 |
| GET | `/swagger/index.html` | Swagger UI |
| GET | `/swagger/openapi.yaml` | OpenAPI 原文 |

登录响应：

```json
{
  "access_token": "…",
  "refresh_token": "…",
  "token_type": "Bearer",
  "expires_in": 7200
}
```

开启 TOTP 时第一步返回：

```json
{ "mfa_required": true, "mfa_token": "…", "mfa_expires_in": 300, "remember_me": false }
```

## 3. 当前用户与个人中心

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/auth/logout` | 登录 | 注销 refresh + 当前会话 |
| GET | `/auth/me` | 登录 | 用户 + 权限码 + 菜单树，续期文件 Cookie |
| POST | `/auth/unlock` | 登录 | 密码解锁（10/min/IP） |
| POST | `/auth/unlock/passkey/begin|finish` | 登录 | Passkey 解锁 |
| GET/PUT | `/profile` | 登录 | 个人资料 |
| PUT | `/profile/password` | 登录 | 改密，撤销其他设备 |
| GET | `/profile/sessions` | 登录 | 在线设备 |
| DELETE | `/profile/sessions/:id` | 登录 | 退出指定设备 |
| POST | `/profile/session-actions/revoke-others` | 登录 | 退出其他设备 |
| POST | `/profile/session-actions/revoke-all` | 登录 | 退出全部设备 |
| GET | `/profile/totp` | 登录 | 状态 + 剩余恢复码 |
| POST | `/profile/totp/setup` | 登录 | 生成密钥/二维码 |
| POST | `/profile/totp/enable` | 登录 | 密码 + 动态码 + `refresh_token` |
| DELETE | `/profile/totp` | 登录 | 密码 + 动态码/恢复码 |
| GET | `/profile/passkeys` | 登录 | Passkey 设备 |
| POST | `/profile/passkeys/begin|finish` | 登录 | 绑定设备（begin 需密码） |
| PUT/DELETE | `/profile/passkeys/:id` | 登录 | 重命名 / 删除 |

## 4. 看板

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/dashboard/summary` | `dashboard:view` | 用户/角色/菜单/文件/任务统计 + 7 日登录趋势 + 最近操作 |

## 5. 用户、角色、菜单、部门

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET | `/users` | `system:user:list` |
| GET | `/users/:id` | `system:user:list` |
| POST | `/users` | `system:user:create` |
| PUT | `/users/:id` | `system:user:update` |
| DELETE | `/users` | `system:user:delete` |
| PUT | `/users/:id/status` | `system:user:update` |
| PUT | `/users/:id/password` | `system:user:reset` |
| PUT | `/users/:id/roles` | `system:user:update` |
| POST | `/users/import` | `system:user:import` |
| GET | `/users/template` | `system:user:import` |
| GET | `/roles` | `system:role:list` |
| GET | `/roles/all` | `system:role:list` |
| POST | `/roles` | `system:role:create` |
| PUT | `/roles/:id` | `system:role:update` |
| DELETE | `/roles/:id` | `system:role:delete` |
| GET | `/roles/:id/menus` | `system:role:list` |
| PUT | `/roles/:id/menus` | `system:role:assign` |
| GET | `/menus/tree` | `system:menu:list` |
| POST | `/menus` | `system:menu:create` |
| PUT | `/menus/:id` | `system:menu:update` |
| DELETE | `/menus/:id` | `system:menu:delete` |
| GET | `/departments/tree` | `system:dept:list` |
| POST | `/departments` | `system:dept:create` |
| PUT | `/departments/:id` | `system:dept:update` |
| DELETE | `/departments/:id` | `system:dept:delete` |

用户列表筛选：`keyword`、`dept_id`、`status`、`role_id`。
删除用户：`{"ids":[1,2]}`；分配角色：`{"role_ids":[2]}`；角色授权：`{"menu_ids":[1,2]}`。
详细规则见 [权限体系](../backend/rbac.md)。

## 6. 字典、参数

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET/POST | `/dict/types` | `system:dict:list` / `system:dict:create` |
| PUT/DELETE | `/dict/types/:id` | `system:dict:update` / `system:dict:delete` |
| GET/POST | `/dict/types/:id/items` | `system:dict:list` / `system:dict:create` |
| PUT/DELETE | `/dict/items/:id` | `system:dict:update` / `system:dict:delete` |
| GET/POST | `/parameters` | `system:param:list` / `system:param:create` |
| PUT/DELETE | `/parameters/:id` | `system:param:update` / `system:param:delete` |

参数支持 `keyword`；字典类型支持 `status`。详见 [字典、参数、通知与看板](../backend/system.md)。

## 7. 文件

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/files` | `system:file:list` | `keyword`、`mime`、`kind` |
| POST | `/files` | `system:file:upload` | multipart `file` |
| GET | `/files/:id/download` | `system:file:download` | 流式下载 |
| GET/POST | `/files/:id/shares` | `system:file:download` | 分享记录 / 创建（`expires` 秒） |
| DELETE | `/files/:id/shares/:shareID` | `system:file:download` | 撤销分享 |
| DELETE | `/files/:id` | `system:file:delete` | 删除元数据 + 物理文件 |

分享 URL：`/uploads/<key>?share=<token>`。详见 [文件、存储与分享](../backend/files.md)。

## 8. 定时任务

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET/POST | `/jobs` | `system:job:list` / `system:job:create` |
| PUT/DELETE | `/jobs/:id` | `system:job:update` / `system:job:delete` |
| PUT | `/jobs/:id/status` | `system:job:update` |
| POST | `/jobs/:id/run` | `system:job:run` |
| GET | `/jobs/:id/runs` | `system:job:list` |

详见 [定时任务](../backend/jobs.md)。

## 9. 审计日志

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET | `/logs/operations` | `system:log:operation` |
| DELETE | `/logs/operations` | `system:log:operation`（`{"ids":[...]}`） |
| DELETE | `/logs/operations/clear` | `system:log:operation` |
| GET | `/logs/logins` | `system:log:login` |
| DELETE | `/logs/logins/clear` | `system:log:login` |

筛选：`keyword`、`username`、`status`、`start`、`end`。日志导出走异步任务（模块 `operation_logs` / `login_logs`）。详见 [日志与审计](../backend/audit.md)。

## 10. 在线会话与监控

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/online-sessions` | `system:online:list` | `keyword`、`user_id`；额外返回 `users` |
| DELETE | `/online-sessions/:userID` | `system:online:kick` | 带 `session_id` 踢单设备，否则踢全部 |
| GET | `/monitor/overview` | `system:monitor:view` | 主机/运行时/DB/缓存快照 |

详见 [在线会话与系统监控](../backend/monitor.md)。

## 11. 异步导出

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/export-tasks` | `system:export:list` | `status` 可选；只返回当前用户任务 |
| POST | `/export-tasks` | `system:export:submit` | `{"module":"users","params":{...}}` |
| POST | `/export-tasks/:id/cancel` | `system:export:cancel` | 取消排队/执行中任务 |
| DELETE | `/export-tasks/:id` | `system:export:delete` | 删除已结束任务与产物 |
| GET | `/export-tasks/:id/download` | `system:export:download` | 下载 xlsx |
| GET/PUT | `/export-tasks/filename-template` | `system:export:list` / `system:export:config` | 文件名模板 |

已注册模块：

| module | 内容 | params |
| --- | --- | --- |
| `users` | 用户列表 | `keyword/dept_id/status/role_id` |
| `operation_logs` | 操作日志 | `username/status/start/end/keyword` |
| `login_logs` | 登录日志 | 同上 |

任务状态：0 排队 / 1 生成中 / 2 完成 / 3 失败 / 4 已取消。详见 [异步导出](../export.md)。

## 12. 站内通知

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/notifications` | `unread=1` 只看未读；分页 |
| GET | `/notifications/unread-count` | 未读数 |
| PUT | `/notifications/:id/read` | 单条已读 |
| PUT | `/notifications/read-all` | 全部已读 |

## 13. 请求示例

### 13.1 登录

```bash
CAPTCHA=$(curl -s http://127.0.0.1:8080/admin/v1/auth/captcha)
# 从响应取 captcha_id 与图片中的字符（开发环境可从缓存取答案）

curl -s -X POST http://127.0.0.1:8080/admin/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{
    "username":"admin",
    "password":"Admin@123456",
    "captcha_id":"…",
    "captcha_code":"AB3D",
    "remember_me":true
  }'
```

### 13.2 创建用户

```bash
curl -s -X POST http://127.0.0.1:8080/admin/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "username":"zhangsan",
    "password":"abc12345",
    "nickname":"张三",
    "dept_id":2,
    "status":1,
    "role_ids":[2]
  }'
```

### 13.3 提交导出

```bash
curl -s -X POST http://127.0.0.1:8080/admin/v1/export-tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"module":"users","params":{"status":1,"keyword":"zhang"}}'
```

## 14. 兼容性与版本

- 路径前缀带版本 `/v1`，破坏性变更应新增 `/v2`，不要在原路径改语义。
- 新增响应字段是兼容的；删除/改名需要前端同步。
- `openapi.yaml` 更新后 Swagger UI 自动生效；新增端点时记得同步，避免文档落后。
