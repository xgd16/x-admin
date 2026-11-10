# 日志与审计

审计分两类：**操作日志**记录写接口，**登录日志**记录认证事件。两者都带 `request_id`，可与应用日志串联。

## 1. 操作日志

### 1.1 采集链路

```text
请求进入
  → middleware.Audit（只对 POST/PUT/PATCH/DELETE，且排除 /admin/v1/auth/*）
  → 用 bodyCapture 在 handler 读取请求体的同时复制最多 64KB
  → 请求结束，取状态码、耗时、ClientIP、RequestID、c.Errors
  → sanitizeBody 脱敏并截断到 4KB
  → LogService.RecordOperation（有界队列，默认 2048）
  → 后台 worker 单条写库，每条独立 5 秒超时
```

关键设计：

- **不阻塞请求**：队列满时直接丢弃并继续服务，宁可少日志也不拖慢接口。
- **不二次读取请求体**：`bodyCapture` 包装 `io.ReadCloser`，handler 读多少就复制多少。
- **只记录可展示的错误**：`response.Error` 写入 `c.Errors` 的是安全消息，不会把 SQL/文件错误带进审计表。
- **脱敏**：字段名包含 `password`、`token`、`secret`、`captcha`，或等于 `otp_code` 时，值替换为 `***`；非 JSON 请求体只记 `[non-json body]`。
- **UTF-8 安全截断**：按字节上限截断时回退到合法字符边界，避免 MySQL 写入报错。

### 1.2 字段

| 字段 | 说明 |
| --- | --- |
| `user_id` / `username` | 当前登录身份；匿名写请求为 0/空 |
| `method` / `path` | HTTP 方法与路径（不含 query） |
| `ip` / `user_agent` | `ClientIP`（受 trusted_proxies 影响）与 UA |
| `status` / `latency_ms` | HTTP 状态与处理耗时 |
| `request_id` | 与访问日志、错误日志关联 |
| `body` | 脱敏后的 JSON，≤4KB |
| `error` | 处理过程中写入 `c.Errors` 的消息，≤500 字节 |

### 1.3 查询

`GET /logs/operations`（`system:log:operation`）：

| 参数 | 说明 |
| --- | --- |
| `keyword` | 用户名/路径等模糊匹配（仓储实现） |
| `username` | 精确/模糊按仓储实现 |
| `status` | HTTP 状态码 |
| `start` / `end` | 时间范围 |
| `page` / `page_size` | 分页 |

### 1.4 删除与清空

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| DELETE | `/logs/operations` | 请求体 `{"ids":[1,2]}`，批量删除 |
| DELETE | `/logs/operations/clear` | 清空全部操作日志 |

> 删除日志本身也是 `DELETE`，会被审计中间件记录一条操作日志。

### 1.5 导出

没有同步导出接口。前端「导出」会提交异步任务：

```json
{ "module": "operation_logs", "params": { "username": "admin", "start": "2026-11-01T00:00:00+08:00" } }
```

模块名 `operation_logs`，导出列：ID、用户ID、操作人、方法、路径、IP、状态、耗时、RequestID、请求体、错误、时间。机制见 [异步导出](../export.md)。

## 2. 登录日志

覆盖密码登录、MFA、Passkey 登录的成功与失败：

| 字段 | 说明 |
| --- | --- |
| `user_id` / `username` | 登录账号；用户不存在时 `user_id=0` |
| `ip` / `user_agent` | 客户端信息 |
| `status` | HTTP 语义状态码（200 成功 / 401 密码错误 / 403 停用 / 423 锁定） |
| `message` | 可读说明，如「登录成功」「动态验证码错误」「账号已停用」 |

记录时机（`AuthService.recordLogin`）：

- 用户不存在、密码错误、账号停用、锁定、MFA 失败/成功、Passkey 成功。
- 成功登录还会同时写「新登录提醒」通知。
- 登录接口本身**不写操作日志**（`shouldAudit` 排除 `/auth/`），避免记录密码与验证码。

查询：

`GET /logs/logins`（`system:log:login`），参数同操作日志；`DELETE /logs/logins/clear` 清空。

导出模块名 `login_logs`，列：ID、用户ID、账号、IP、User-Agent、状态、说明、时间。

## 3. 保留与清理

`LogService` 只负责写入和查询，清理由定时任务完成：

| 任务 | 默认参数 | 行为 |
| --- | --- | --- |
| `clean_logs` | `{"days":90}` | 物理删除 90 天前的操作日志与登录日志 |
| `clean_notifications` | `{"days":90}` | 清理站内通知 |

种子数据里已经有一条「清理登录日志」任务（`0 3 * * *`）。日志清理是物理删除，**不可恢复**；合规要求更高时可改为归档到对象存储再删除。

## 4. 与访问日志的区别

| 日志 | 输出 | 用途 |
| --- | --- | --- |
| 访问日志 `middleware.AccessLog` | stdout（JSON/文本） | 全量请求、耗时、状态，给日志平台 |
| 操作日志 `sys_operation_logs` | MySQL | 审计写操作、追责、导出 |
| 登录日志 `sys_login_logs` | MySQL | 登录安全审计 |
| 错误日志 `response.Error` | stdout | 5xx 原始错误 + request_id |

排查线上问题时，先在响应头/响应体拿 `request_id`，再到日志平台搜同 ID；需要业务上下文再查操作日志表。

## 5. 权限与安全

| 动作 | 权限 |
| --- | --- |
| 查看操作日志 / 删除 / 清空 | `system:log:operation` |
| 查看登录日志 / 清空 | `system:log:login` |
| 导出（异步任务） | `system:export:submit` |

- 日志接口只对管理员开放；没有「普通用户查看自己操作日志」的接口。
- 操作日志请求体已脱敏，但**不要**把密钥放进 query 参数（query 不入库，但会进访问日志）。
- 清空日志是高危操作，建议生产环境只授予极少数角色，并配合数据库备份。

## 6. 扩展

- **新增审计字段**：在 `model.OperationLog` 加列 + 迁移 + `Audit` 中间件赋值；注意同步导出列。
- **新增登录事件**：在认证流程调用 `recordLogin`，保持状态码语义。
- **异步队列容量**：`provideLogService` 固定 2048，可按吞吐调整；队列满会丢日志，需监控丢弃率（可加指标）。
- **写入失败**：当前只忽略错误（`_ = s.repo.CreateOperation`），生产建议加错误日志/指标。

## 7. 常见问题

| 现象 | 原因 |
| --- | --- |
| 操作日志没有某次写请求 | 路径以 `/admin/v1/auth/` 开头；或队列满被丢弃；或方法不是写方法 |
| 请求体是 `***` | 字段名命中敏感词，属于预期脱敏 |
| 请求体是 `[non-json body]` | 上传等非 JSON 请求不记录内容 |
| 登录日志没有失败记录 | 只有走认证 service 的失败才会记录；网关层拒绝不会入库 |
| 日志时间与本地差 8 小时 | 容器时区未设置，检查 `TZ=Asia/Shanghai` |
| 导出没有数据 | 导出参数时间格式不对，或筛选条件过窄；导出走异步任务，需到「导出记录」下载 |
