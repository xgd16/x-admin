# 架构与请求生命周期

x-admin 是标准的 Go 分层单体 + React SPA。理解这一页，后面的模块文档都只是这套骨架的具体填充。

## 1. 分层与依赖方向

```text
cmd/server
   └── bootstrap（wire 装配，唯一允许同时依赖各层实现）
         ├── router ── middleware
         ├── handler ── service（定义 repo 端口）
         │                 └── repository ── model
         ├── jobs（cron 调度）
         └── pkg（错误码、响应、缓存、JWT、存储、Excel…）
```

| 层 | 目录 | 职责 | 禁止 |
| --- | --- | --- | --- |
| 入口 | `cmd/server` | 加载配置、启动 HTTP、优雅退出 | 写业务 |
| 装配 | `internal/bootstrap` | 创建基础设施、repo、service、handler、router | 写业务 |
| 路由 | `internal/router` | 声明全部端点与所需权限码 | 写业务 |
| 中间件 | `internal/middleware` | 请求 ID、恢复、日志、CORS、审计、鉴权、限流 | 访问数据库 |
| 协议 | `internal/handler` | 参数绑定、校验入口、统一响应 | 直接访问 DB、写业务规则 |
| 业务 | `internal/service` | 业务规则、事务编排、权限约束、通知 | 依赖 Gin |
| 端口 | `internal/service/ports.go` | 定义仓储接口，供测试替换 | 依赖具体实现 |
| 数据 | `internal/repository` | GORM 查询、分页、软删除、游标 | 写业务判断 |
| 模型 | `internal/model` | 结构体、GORM 标签、TableName、过滤器 | 写逻辑 |
| 基础库 | `internal/pkg` | 无业务依赖的通用能力 | 反向依赖 service |

依赖方向永远是 **外层 → 内层**：handler 依赖 service，service 依赖 `ports.go` 里的接口，repository 实现接口。service 不 import `gin`，repository 不知道 HTTP。

## 2. 依赖注入（wire）

- 各层用 `wire.go` 里的 `ProviderSet` 声明构造函数（`repository.NewUserRepo`、`service.NewUserService`…）。
- `internal/bootstrap/providers.go` 负责把配置里的原语（端口、TTL、上限）转换成具名依赖，并承接带生命周期的构造（缓存关闭、任务 Boot、导出 worker）。
- `wire_gen.go` 是生成物，**不要手改**；改了构造参数后运行 `make wire`。
- handler 依赖由 `bootstrap` 汇总成 `router.Handlers`，路由层只拿一个结构体。

新增依赖时的顺序：构造函数 → 加进对应 `ProviderSet` → `make wire` → 编译。具体示例见 [新增业务模块](adding-a-module.md)。

## 3. 启动与关闭顺序

**启动：**

1. `config.Load`：默认值 → YAML → `X_ADMIN_*` 环境变量 → 校验（端口、JWT 密钥、DSN、代理网段）。
2. `database.Open`：建立 GORM 连接、设置连接池、Ping。
3. `database.Migrate`（`auto_migrate: true`）：执行内嵌的 golang-migrate 脚本与种子数据。
4. wire 装配：缓存（Redis / memory）→ 本地存储 → 各仓储 → 各服务 → handler → router。
5. 生命周期动作：审计队列启动 worker、导出服务注册模块并 `Boot()`、定时任务 `Boot()` 把启用任务注册进 cron。
6. `scheduler.Start()`，HTTP 开始监听。

**关闭（SIGINT/SIGTERM）：**

1. `http.Server.Shutdown`：停止接收新请求，等待存量请求，超时由 `server.shutdown_timeout` 控制。
2. `scheduler.Stop()`：停止 cron 循环，等待在途任务。
3. wire 注入的 cleanup：关闭导出队列、审计队列、缓存。
4. 关闭数据库连接池。

依赖顺序保证不会出现「任务还在跑、数据库已关」的情况。

## 4. 一次请求的完整链路

以 `PUT /admin/v1/users/12` 为例：

```text
client
  │  Authorization: Bearer <access>
  ▼
RequestID        生成/透传 X-Request-ID，写入 context
Recovery         panic → 500，记录堆栈
AccessLog        记录方法、路径、状态、耗时
CORS             按 cors.origins 放行预检
Audit            写操作复制请求体（敏感字段脱敏），异步落库
UploadSecurity   给 /uploads 加 CSP sandbox / nosniff
UploadAuth       /uploads 专用鉴权（Cookie / Bearer / 分享令牌）
RateLimit(/admin/v1)  全局 100 次/分/IP
Auth             解析 JWT → 加载用户 → 校验 session_version 与会话
Require("system:user:update")  权限码判断，super_admin 直接放行
handler.Update   绑定 JSON → PathID → service.Update
service.Update   校验字段、部门/角色、越权与内置账号保护 → repo
repository       事务写 sys_users / sys_user_roles
handler          response.OK → {code:0,message:"ok",data,request_id}
```

### 4.1 鉴权不是只验 JWT

`middleware.Auth` 每次请求都会：

1. 校验 HS256 签名、issuer、过期时间。
2. 用 `uid` 重新从数据库加载用户（停用/删除立即生效）。
3. 比对 token 里的 `session_version` 与用户当前值（改密、重置密码、启停账号后旧 token 立即失效）。
4. 检查 `session:<uid>:<sid>` 是否仍存在（单设备撤销后 access token 立即失效）。
5. 把 `Principal{UserID, Username, SessionVersion, SessionID, Super, Permissions}` 写入 context。

因此角色/菜单改动**无需重新登录**即可生效，而安全变更会强制旧会话失效。

### 4.2 权限码

- 路由集中声明在 `internal/router/router.go`，每个受保护路由都带 `middleware.Require("system:xxx:yyy")`。
- `super_admin` 在 `Principal.Has` 中直接返回 true，不走权限集合。
- 普通用户的权限码来自「启用角色 → 授权菜单 → `sys_menus.permission`」。
- 前端 `can()` 只决定按钮显隐，最终授权永远以后端 `Require` 为准。

详见 [权限体系（RBAC）](../backend/rbac.md)。

## 5. 错误处理

业务错误统一用 `internal/pkg/apperr`：

```go
return apperr.New(apperr.CodeConflict, "角色下仍有用户，无法删除")
return apperr.Wrap(err, apperr.CodeInternal, "文件保存失败")
```

- `response.Error` 把业务码映射为 HTTP 状态，并输出 `{code,message,request_id}`。
- 未知错误统一转 `50001`，**不把数据库/文件系统细节返回给客户端**。
- 5xx 会写结构化错误日志（含 `request_id`、方法、路径、原始错误），便于按请求 ID 追日志。
- 审计中间件读取 `c.Errors`，只记录可安全展示的错误摘要。

错误码表见 [API 与错误码](../reference/api.md)。

## 6. 异步与后台任务

| 机制 | 实现 | 关键点 |
| --- | --- | --- |
| 操作审计 | `LogService` 有界队列（2048）+ 后台 worker | 队列满时**丢弃**而不是阻塞请求；关闭时等待剩余日志写完 |
| 异步导出 | `ExportService` worker 池 + 队列 | 游标分批生成 xlsx，完成后站内通知；详见 [异步导出](../export.md) |
| 定时任务 | `jobs.Cron` + `JobService` | 5 段 Cron、同名替换、`SkipIfStillRunning`、多副本缓存锁 |
| 站内通知 | 同步写入 `sys_notifications` | 通知失败不影响主流程（`notify` 辅助函数吞掉错误） |
| 会话/限流/验证码 | `cache.Store`（Redis / memory） | 键设计见各模块文档 |

## 7. 前后端边界

- 后端只返回 JSON，统一前缀 `/admin/v1`；文件通过 `/uploads` 提供，但需要鉴权。
- 前端 `api/client.ts` 负责令牌注入、401 单飞刷新、错误封装、上传与下载。
- 登录后 `/auth/me` 返回用户、权限码、菜单树；前端据此生成导航，但页面路由是静态注册的（未知菜单路径不会渲染页面）。
- 前后端权限码必须一致：新增按钮权限时，后端 `Require`、种子菜单、前端 `can()` 三处都要改。

## 8. 扩展点速查

| 需求 | 扩展点 |
| --- | --- |
| 新增业务模块 | 参考 [新增业务模块](adding-a-module.md) |
| 新增后台任务 handler | `provideJobExecutor` 的 switch + [定时任务](../backend/jobs.md) |
| 新增导出模块 | `ExportBuilders()` + `provideExportService` |
| 替换文件存储为对象存储 | 实现 `storage.Store`，替换 `provideStorage` |
| 新增缓存实现 | 实现 `cache.Store`，替换 `newCache` |
| 新增认证体系（C 端） | 参考 [前台用户认证](../front-auth.md) |
| 新增前端页面 | `App.tsx` 路由 + `config/nav.ts` + `pages/` |
