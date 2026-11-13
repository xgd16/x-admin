# 代码生成：create

`go tool xadmin create` 用于生成**按 API 面和版本分层**的 handler 脚手架。它不是一次性把所有业务层都生成出来，而是先把目录、处理器和路由注册方式固定下来，业务代码再逐步接入 service。

> 也可以直接运行 `go tool xadmin`，在 TUI 中选择「创建 API 面」或「创建 Handler」填表生成，结果与本文命令一致。

## 1. 命令

```bash
# 新建一个 API 面（含版本目录、handler 入口、路由工厂）
go tool xadmin create api --name client --version v1 --auth --table client_users

# 在已生成的 API 面/版本下新建 handler
go tool xadmin create handler --api client --version v1 --name order --path /orders
```

`create api` 适合：

- 新增客户端、商户端或开放平台 API 面；
- 为不同业务域建立独立前缀与 handler 包。

> 框架默认只有 `admin`。client / merchant 也需要先运行 `create api` 生成，不会预置路由文件。Surface 名称是路由工厂的注册键，同一个名称只能注册一次。需要同一业务域的 v2 时，使用不同的 `--name`，例如 `partner-v2 --version v2 --prefix /partner/v2`。当前版本分层主要体现在 handler 目录、包名和 URL 前缀；一个 Surface 下挂多个版本的模型留待后续扩展。

`create handler` 适合：

- 在 `admin` 或已生成的自定义 API 面下增加资源；
- 每个资源一个 handler 文件，放在对应版本目录。

## 2. 生成的目录

```text
internal/
├── handler/
│   ├── admin/v1/...          # 后台（已有平铺文件，可逐步迁移）
│   ├── client/v1/order.go    # create handler 生成
│   └── partner/v1/
│       ├── handler.go        # create api 生成的入口
│       └── order.go          # create handler 生成
├── router/
│   ├── partner_v1.go         # create api：注册 Surface
│   └── partner_v1_order.go   # create handler：注册路由模块
└── migrations/
    └── 000002_client_users.up.sql / .down.sql   # create api --auth 生成
```

`create api --auth` 不生成独立的 model/repository/service 文件：账号读写由框架内置的 `SurfaceAuthService` 按表名处理，表结构由生成的 SQL 迁移创建。

包名规则：`<api><version>`，例如 `clientv1`、`partnerv1`。目录体现版本，包名避免和别的版本冲突。

## 3. `create api`

```bash
# 只生成路由壳
go tool xadmin create api --name partner --version v1 --prefix /partner/v1

# 生成完整 JWT 认证 + 独立用户表
go tool xadmin create api --name client --version v1 --auth --table client_users
```

| 参数 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- |
| `--name` | 是 | — | API 面名称，小写字母开头，允许 `a-z0-9-` |
| `--version` | 否 | `v1` | API 版本，格式 `v1`、`v2` |
| `--prefix` | 否 | `/<name>/<version>` | URL 前缀 |
| `--auth` | 否 | `false` | 生成完整 JWT 认证接口和用户表迁移 |
| `--table` | 否 | `<name>_users` | 认证用户表名，例如 `client_users` |
| `--migration` | 否 | 当前最大编号 + 1 | 用户表迁移编号 |
| `--dir` | 否 | `.` | 项目根目录 |
| `--force` | 否 | `false` | 覆盖已存在的生成文件 |
| `--json` | 否 | `false` | 输出机器可读 JSON |

生成内容：

1. `internal/handler/<name>/<version>/handler.go`
   - `Handler`、`NewHandler()`、`Register()`、`RegisterSecured()`；
   - `Register` 与 `RegisterSecured` 默认只有注释示例。
2. `internal/router/<name>_<version>.go`
   - `init()` 中调用 `router.RegisterSurface(name, factory)`；
   - 工厂从 `config.APISurfaceConfig` 读取 `enabled` / `prefix` / `table`；
   - 未开启 `--auth` 时，认证中间件取 `SurfaceAuth[name]`；
   - 开启 `--auth` 时自动使用该面的 `SurfaceTokens[name]` 和 `SurfaceAuthService`。
3. `--auth` 额外生成 `migrations/<number>_<table>.up.sql` / `.down.sql`
   - 按 `<name>_users` 或 `--table` 建表；
   - 编号默认取当前 `migrations/` 的最大编号 + 1，保证新迁移排在已有迁移之后。已有 `001000+` 历史编号时继续往上加，不会回填更小的号。

### 3.1 `--auth` 自动生成的接口

```text
POST /<prefix>/auth/register   注册并直接返回 token
POST /<prefix>/auth/login      登录
POST /<prefix>/auth/refresh    刷新 token（一次性轮换）
POST /<prefix>/auth/logout     退出登录（需要 access token）
GET  /<prefix>/auth/me         当前用户资料（需要 access token）
```

注册/登录响应：

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "Bearer",
  "expires_in": 7200,
  "user": {
    "id": 1,
    "username": "customer",
    "nickname": ""
  }
}
```

认证用户与后台 `sys_users` 完全独立：

- 每个 API 面一张表，表名由 `--table` 指定；
- 不参与后台 RBAC，`Require("system:...")` 对其一律拒绝；
- access token 使用该面的独立 issuer，不能跨面使用；
- refresh token、登录失败计数、会话缓存 key 都带 `auth:<surface>:` 前缀。

生成后需要把配置片段加入 `configs/config.yaml`：

```yaml
api:
  client:
    enabled: true
    prefix: /client/v1
    table: client_users
```

`APIConfig` 支持任意 `api.<name>`；框架内置面只有 `admin`，client / merchant 由 `create api` 生成后写入配置。

## 4. `create handler`

```bash
go tool xadmin create handler --api client --version v1 --name order --path /orders
```

| 参数 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- |
| `--api` | 是 | — | API 面名称，例如 `admin`，或已生成的 `client`、`merchant`、`partner` |
| `--version` | 否 | `v1` | API 版本 |
| `--name` | 是 | — | 资源名，例如 `order`、`order_item` |
| `--path` | 否 | `/<name>` | 路由路径，建议用复数 `/orders` |
| `--permission` | 否 | admin 为 `system:<name>` | 权限码前缀；`none` 表示不生成权限判断 |
| `--dir` | 否 | `.` | 项目根目录 |
| `--force` | 否 | `false` | 覆盖已存在的生成文件 |
| `--json` | 否 | `false` | 输出机器可读 JSON |

生成的 handler 默认按 fail-closed 组织：公开接口 `Register` 留空，CRUD 全部放在 `RegisterSecured`：

```go
func (h *OrderHandler) Register(api *gin.RouterGroup) {
    // 确认无需登录的接口再放进来
}

func (h *OrderHandler) RegisterSecured(secured *gin.RouterGroup) {
    secured.GET("/orders", middleware.Require("system:order:list"), h.List)
    secured.GET("/orders/:id", middleware.Require("system:order:list"), h.Get)
    secured.POST("/orders", middleware.Require("system:order:create"), h.Create)
    secured.PUT("/orders/:id", middleware.Require("system:order:update"), h.Update)
    secured.DELETE("/orders/:id", middleware.Require("system:order:delete"), h.Delete)
}
```

- `admin` 面默认生成 `system:<name>:*` 权限码；用 `--permission none` 可以关闭；
- 自定义面默认不生成权限码，只依赖该面的认证；`create api --auth` 生成的面已经自动配置好 JWT；
- 未开启 `--auth` 的自定义面，只有在配置了 `SurfaceAuth` 时 `secured` 才非空；生成后未接入认证时，除 `/ping` 外不会有业务路由，这是故意的 fail-closed；
- 方法体是 `response.OK(c, nil)` + TODO，下一步接入 service。

## 5. 自动注册机制

生成的路由文件不需要手动加到 `router.go`：

```go
func init() {
    RegisterModule(Module{
        API:     "client",
        Version: "v1",
        Register: func(d Deps, api, secured *gin.RouterGroup) {
            h := clientv1.NewOrderHandler()
            h.Register(api)
            if secured != nil {
                h.RegisterSecured(secured)
            }
        },
    })
}
```

`Surface.mount` 在挂载时按 `API` + `Version` 找到这些模块。`create api` 生成的面也会设置 `Version`，所以后续 handler 会自动挂载。

注意：

- 同一个 API 面/版本不要注册重复路径，Gin 会在启动时 panic；
- 生成文件使用 `init()`，删除 handler 时要同时删除对应的路由文件；
- 覆盖生成文件用 `--force`，会丢失手工修改。

## 6. 与多 API 面的关系

- `admin`：默认唯一内置面，已有 `internal/router/admin.go`；生成模块会挂在 `admin` v1 下。
- `client`：先运行 `create api --name client --version v1`，再接入 `SurfaceAuth["client"]`；之后生成的 handler 才会挂载 `secured` 路由。
- `merchant`：先运行 `create api --name merchant --version v1`；适合 API Key / 签名校验。
- 其他自定义面：同样由 `create api` 生成，配置 `api.<name>` 后挂载。

API 面本身的启用、前缀、令牌隔离见 [多 API 模式](../guide/multi-api.md)。

## 7. 当前边界

生成器目前负责：

- handler 目录与版本分层；
- 路由工厂/模块注册；
- 基础 CRUD 方法骨架；
- `--auth` 模式的用户表迁移、JWT 注册/登录/刷新/登出/me 接口。

`--auth` 的账号模型、仓储、服务和 JWT 逻辑复用框架内置的 `SurfaceAuthService`，不会为每个 API 面复制一套 model/repository/service 文件；表名由配置中的 `api.<name>.table` 决定。账号表字段目前固定为 `APIUser`，需要扩展字段时同时修改 `internal/model/api_user.go` 和生成的迁移。

尚未自动生成：

- 业务 CRUD 的 service / repository / model；
- OpenAPI 定义；
- 前端页面与类型；
- wire provider。

这些仍按 [新增一个业务模块](../guide/adding-a-module.md) 的清单补。后续可以在此基础上扩展 `create module` 或 `create crud`。
