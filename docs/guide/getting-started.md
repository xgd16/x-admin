# 本地开发与快速上手

这篇文档带你从零跑起 x-admin，并给出日常开发最常用的命令与目录定位。部署到服务器请看 [部署与发布](../ops/deploy.md)。

## 1. 环境要求

| 依赖 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.27+ | 后端与测试 |
| Node.js | 22+ | 前端构建与测试 |
| MySQL | 8.4 | 业务数据与审计日志 |
| Redis | 7.4 | 缓存、会话、限流、任务锁 |
| Docker | 可选 | 一条命令拉起 MySQL / Redis 或完整栈 |

> 只想跑后端接口、不装 MySQL/Redis 也可以：把 `redis.driver` 改为 `memory`，并使用本地已有的 MySQL；`memory` 仅适合开发，重启后 refresh token / 在线会话会丢失。

## 2. 配置

```bash
cp configs/config.example.yaml configs/config.yaml
cp .env.example .env        # 可选，用于 Docker Compose 或本地导出环境变量
```

配置优先级：**内置默认值 < `configs/config.yaml` < `X_ADMIN_*` 环境变量**。完整字段见 [配置参考](../ops/config.md)。

本地最少需要改 `database.dsn` 和 `jwt.secret`：

```yaml
database:
  dsn: xadmin:xadmin@tcp(127.0.0.1:3306)/xadmin?charset=utf8mb4&parseTime=True&loc=Local
jwt:
  secret: 至少16位的本地随机字符串
```

`database.auto_migrate: true` 时，启动会自动执行内嵌的版本化迁移并写入种子数据。

## 3. 三种启动方式

### 3.1 推荐：只起基础设施，本地跑前后端

```bash
# 1) 启动 MySQL 8.4 + Redis 7.4（端口 3306 / 6379）
make dev-infra

# 2) 启动后端（默认 :8080）
make run

# 3) 另开终端启动前端（默认 :5173）
cd frontend
npm ci
npm run dev
```

打开 http://localhost:5173 ，默认账号：

```text
用户名：admin
密码：Admin@123456
```

首次登录后请立即在「个人中心」修改密码。

### 3.2 完整 Docker Compose

```bash
cp .env.example .env
# 必须设置随机 JWT 密钥，否则 compose 拒绝启动
# JWT_SECRET=<32位以上随机值>

make compose-up
```

打开 http://localhost 。Compose 会构建后端二进制与前端静态资源，Nginx 托管 SPA 并反代 `/admin/v1`、`/uploads`、`/swagger`、`/healthz`。生成自定义 API 面后再补充对应反代。

### 3.3 只用环境变量跑后端

```bash
X_ADMIN_DATABASE_DSN='xadmin:xadmin@tcp(127.0.0.1:3306)/xadmin?charset=utf8mb4&parseTime=True&loc=Local' \
X_ADMIN_JWT_SECRET='local-dev-secret-please-change' \
go run ./cmd/server -config ""
```

`-config ""` 表示不读 YAML，完全使用默认值 + 环境变量。

## 4. 验证服务

| 检查项 | 地址 |
| --- | --- |
| 健康检查（DB + 缓存） | `GET http://127.0.0.1:8080/healthz` |
| Swagger UI | `http://127.0.0.1:8080/swagger/index.html` |
| OpenAPI 原文 | `http://127.0.0.1:8080/swagger/openapi.yaml` |
| 前端开发服务器 | `http://localhost:5173` |
| 上传文件 | `http://127.0.0.1:8080/uploads/...`（需登录或分享令牌） |

`/healthz` 会同时 Ping 数据库和缓存，任一不可用返回 `503`，适合给容器/K8s 做存活探针。

## 5. 常用命令

| 命令 | 作用 |
| --- | --- |
| `make run` | 用 `configs/config.yaml` 启动后端 |
| `make build` | 编译到 `bin/x-admin` |
| `make test` | 运行全部 Go 测试 |
| `make coverage` | 生成 `coverage.out` 并打印总覆盖率 |
| `make coverage-html` | 生成并打开 HTML 覆盖率报告 |
| `make lint` | `go vet` + 前端 oxlint |
| `make wire` | 重新生成 `internal/bootstrap/wire_gen.go` |
| `make frontend` | 启动前端开发服务器 |
| `make frontend-build` | 生产构建前端 |
| `make test-all` | Go 测试 + 前端 Vitest |
| `make dev-infra` / `dev-infra-down` | 启停开发用 MySQL / Redis |
| `make compose-up` / `compose-down` / `compose-logs` | 完整栈启停与日志 |

工具入口：在终端直接运行 `go tool xadmin` 会打开交互式 TUI，也可以继续使用下面的文本命令。非交互环境或 `XADMIN_NO_TUI=1` 会输出帮助。说明见 [交互式界面（TUI）](../tool/tui.md)。

代码生成：

```bash
go tool xadmin create api --name client --version v1 --auth --table client_users
go tool xadmin create handler --api client --version v1 --name order --path /orders
```

说明见 [代码生成（create）](../tool/create.md)。

## 6. 目录速查

```text
cmd/server/           进程入口：配置 → 装配 → 监听 → 优雅退出
internal/
  bootstrap/          依赖注入（wire 生成），唯一同时依赖各层实现的地方
  config/             YAML + X_ADMIN_* 环境变量
  database/           MySQL 连接池、迁移执行、健康检查
  router/             全部 HTTP 路由与权限码
  middleware/         请求 ID、恢复、访问日志、CORS、审计、鉴权、限流
  handler/            HTTP 参数绑定与响应，不写业务
  service/            业务规则、事务编排、端口接口（ports.go）
  repository/         GORM 实现、通用 Base[T]、软删除墓碑
  model/              仅数据结构与 GORM 标签
  jobs/               robfig/cron 调度封装
  pkg/                可复用基础库（错误码、响应、缓存、JWT、存储…）
migrations/           版本化 SQL，embed 进二进制
tests/                Go 单元测试与 HTTP 集成测试
frontend/             React 19 + antd 6 + Vite 8
deploy/               Dockerfile、Compose、Nginx
docs/                 本站源文件
```

## 7. 第一个改动放哪里

| 你想做的事 | 入口 |
| --- | --- |
| 加一个接口 | `internal/router/admin.go`（或 create api 生成的 API 面）→ handler → service → repository |
| 加一个权限点 | 菜单管理新增按钮节点，或迁移里写 `sys_menus` |
| 加一个定时任务 handler | `internal/bootstrap/providers.go` 的 `provideJobExecutor` |
| 加一个导出模块 | 业务 Service 实现 `ExportBuilders()`，在 `provideExportService` 注册 |
| 加一个前端页面 | `frontend/src/pages` + `App.tsx` 路由 + `config/nav.ts` 元数据 |
| 改配置项 | `internal/config/config.go` + `configs/config.example.yaml` + `.env.example` |
| 改数据库结构 | 新增 `migrations/0000xx_xxx.up.sql` / `.down.sql` |

完整的「新增一个业务模块」流程见 [新增业务模块](adding-a-module.md)。

## 8. 调试建议

- **接口 401/403**：先看响应里的 `code`（`40102` 登录失效 / `40301` 无权限），再检查 `Authorization` 与账号权限。
- **验证码总是不对**：验证码一次性且 5 分钟过期，刷新页面后要使用最新 `captcha_id`。
- **上传图片 401**：`/uploads` 需要登录后由 `/auth/me` 下发的 HttpOnly Cookie，或显式 `Authorization: Bearer`。
- **Passkey 不工作**：RP ID 必须是有效域名（本地 `localhost`），且页面来源必须命中 `webauthn.rp_origins`。
- **改了 wire 依赖但编译报缺参数**：运行 `make wire` 重新生成。

排错清单见 [排错手册](../ops/troubleshooting.md)。
