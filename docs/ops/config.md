# 配置参考

配置来源优先级：**内置默认值 < `configs/config.yaml` < `X_ADMIN_*` 环境变量**。

- 默认值定义在 `internal/config/config.go` 的 `Default()`。
- YAML 模板见 `configs/config.example.yaml`，复制为 `configs/config.yaml` 后修改（该文件已被 `.gitignore` 排除）。
- 环境变量覆盖只覆盖部分字段（见每节标注）；没有环境变量的字段请写 YAML。

## 1. 应用与 HTTP

```yaml
app:
  name: x-admin
  version: 1.0.0
  env: development        # development | production

server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 15s
  write_timeout: 30s
  shutdown_timeout: 10s
  trusted_proxies:
    - 127.0.0.1
    - "::1"
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
```

| 字段 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `app.name` | `x-admin` | — | 站点/应用名 |
| `app.version` | `1.0.0` | — | 监控页展示版本 |
| `app.env` | `development` | `X_ADMIN_APP_ENV` | `production` 时使用 JSON 日志、Gin release、Cookie Secure 逻辑 |
| `server.host/port` | `0.0.0.0:8080` | `X_ADMIN_SERVER_HOST` / `X_ADMIN_SERVER_PORT` | 监听地址 |
| `server.*_timeout` | 15s/30s/10s | — | 读写/优雅退出 |
| `trusted_proxies` | 本机 + 私有网段 | `X_ADMIN_TRUSTED_PROXIES` | 允许读取 `X-Forwarded-For`；显式 `none` 表示不信任任何代理 |

> 后端直接暴露公网时应收紧 `trusted_proxies`；放在 Nginx 后面时要包含 Nginx 的网段，否则 `ClientIP` 全是代理 IP，限流和审计失真。

## 1.1 多 API 面

```yaml
# 框架默认只有后台面；
# client / merchant / partner 等面由 create api 生成后写入。
api:
  admin:
    enabled: true
    prefix: /admin/v1
  # client:
  #   enabled: true
  #   prefix: /client/v1
  #   table: client_users
  #   jwt:
  #     secret: 至少16个字符的独立密钥
  #     access_ttl: 30m
  #     refresh_ttl: 720h
  #     session_ttl: 6h
```

| 字段 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `api.admin.enabled` | `true` | `X_ADMIN_API_ADMIN_ENABLED` | 是否挂载后台 API |
| `api.admin.prefix` | `/admin/v1` | `X_ADMIN_API_ADMIN_PREFIX` | 后台路径前缀 |
| `api.admin.issuer` | `jwt.issuer` | `X_ADMIN_API_ADMIN_ISSUER` | 后台令牌 issuer |
| `api.<name>.enabled` | `false` | — | 自定义面是否挂载 |
| `api.<name>.prefix` | — | — | 自定义面路径前缀 |
| `api.<name>.table` | — | — | `create api --auth` 生成的用户表名 |
| `api.<name>.issuer` | `jwt.issuer:<name>` | — | 自定义面令牌 issuer（旧字段，`jwt.issuer` 优先） |
| `api.<name>.jwt.secret` | `jwt.secret` | — | 面级 JWT 密钥，至少 16 字符 |
| `api.<name>.jwt.issuer` | `api.<name>.issuer` / `jwt.issuer:<name>` | — | 面级令牌 issuer |
| `api.<name>.jwt.access_ttl` | `jwt.access_ttl` | — | access token 有效期 |
| `api.<name>.jwt.refresh_ttl` | `jwt.refresh_ttl` | — | 勾选“记住我”时的 refresh 有效期 |
| `api.<name>.jwt.session_ttl` | `jwt.session_ttl` | — | 未勾选“记住我”时的 refresh 有效期 |

- 面级 `api.<name>.jwt` 只覆盖写出的字段，其余继续沿用全局 `jwt`；`admin` 也支持覆盖。
- 不同面的 issuer 不能重复；面级 `secret` 与全局密钥一样必须至少 16 字符，生产环境禁止使用默认值。
- 独立 JWT 的 `refresh_ttl` / `session_ttl` 决定该面 refresh token 和登录会话的缓存有效期，`access_ttl` 决定 access token 过期时间。
- 框架不预置 client / merchant 路由；先运行 `go tool xadmin create api --name client --version v1`。
- 不同 API 面的前缀和 issuer 都不能重复；启动校验会直接拒绝冲突配置。
- 前缀不能是 `/`，不能以 `/` 结尾，不能包含 `//`，也不能占用 `/healthz`、`/uploads`、`/swagger` 及其子路径。
- 自定义面生成后先只有 `/ping`，业务路由与认证按 [多 API 模式](../guide/multi-api.md) 接入。
- `go tool xadmin create api` 会自动把对应的 `api.<name>` 写入 `configs/config.yaml`，并按名称排序；没有配置文件时只打印片段，见 [代码生成](../tool/create.md)。
- 修改前缀后要同步 Nginx 反代、Vite 代理和前端 `VITE_API_BASE`。

## 2. 数据库

```yaml
database:
  dsn: root:change-me@tcp(127.0.0.1:3306)/xadmin?charset=utf8mb4&parseTime=True&loc=Local
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: 1h
  auto_migrate: true
```

| 字段 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `dsn` | 本地 xadmin | `X_ADMIN_DATABASE_DSN` | DSN 必须含 `parseTime=True&loc=Local`；启动会自动补 `multiStatements=true` |
| `max_open_conns` | 50 | — | 连接池上限 |
| `max_idle_conns` | 10 | — | 空闲连接 |
| `conn_max_lifetime` | 1h | — | 连接复用上限 |
| `auto_migrate` | true | — | 启动执行迁移；生产升级前先备份 |

## 3. 缓存

```yaml
redis:
  driver: redis        # redis | memory
  addr: 127.0.0.1:6379
  password: ""
  db: 0
```

| 字段 | 默认 | 环境变量 |
| --- | --- | --- |
| `driver` | `redis` | `X_ADMIN_REDIS_DRIVER` |
| `addr` | `127.0.0.1:6379` | `X_ADMIN_REDIS_ADDR` |
| `password` | 空 | `X_ADMIN_REDIS_PASSWORD` |
| `db` | 0 | `X_ADMIN_REDIS_DB` |

`memory` 只适合开发/单副本：refresh token、在线会话、验证码、任务锁都只在进程内，重启丢失，多副本互不可见。

## 4. JWT

```yaml
jwt:
  secret: x-admin-dev-secret-change-me
  issuer: x-admin
  access_ttl: 2h
  refresh_ttl: 168h
  session_ttl: 12h
```

| 字段 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `secret` | 开发值 | `X_ADMIN_JWT_SECRET` | 至少 16 字符；生产禁止默认值 |
| `issuer` | `x-admin` | `X_ADMIN_JWT_ISSUER` | 解析时校验 |
| `access_ttl` | `2h` | `X_ADMIN_JWT_ACCESS_TTL` | access token 有效期 |
| `refresh_ttl` | `168h` | `X_ADMIN_JWT_REFRESH_TTL` | 勾选「记住我」时的 refresh 有效期 |
| `session_ttl` | `12h` | `X_ADMIN_JWT_SESSION_TTL` | 未勾选时的 refresh 有效期 |

生产启动会拒绝两个公开示例密钥：`Default().JWT.Secret` 与 `.env.example` 中的 Compose 示例值。请生成随机值：

```bash
openssl rand -base64 48
```

## 5. 安全策略

```yaml
security:
  bcrypt_cost: 10
  max_login_fails: 5
  lock_duration: 15m
  captcha_ttl: 5m
  password_min_len: 8
  permission_cache: 5m   # 预留字段，当前未使用
```

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `bcrypt_cost` | 10 | 越高越安全但登录越慢；测试用 4 |
| `max_login_fails` | 5 | 连续密码错误锁定阈值 |
| `lock_duration` | 15m | 锁定时长（登录与锁屏解锁共用） |
| `captcha_ttl` | 5m | 验证码有效期 |
| `password_min_len` | 8 | 小于 8 时服务按 8 处理 |
| `permission_cache` | 5m | 预留，未参与权限计算 |

这些字段没有环境变量覆盖，只能写 YAML。

## 6. 文件存储

```yaml
storage:
  root: data/uploads
  url_prefix: /uploads
  max_upload_mb: 20
```

| 字段 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `root` | `data/uploads` | `X_ADMIN_STORAGE_ROOT` | 物理根目录 |
| `url_prefix` | `/uploads` | `X_ADMIN_STORAGE_URL_PREFIX` | 对外访问前缀 |
| `max_upload_mb` | 20 | `X_ADMIN_STORAGE_MAX_MB` | 业务上限；HTTP 硬上限 64MB |

## 7. 异步导出

```yaml
export:
  workers: 2
  queue_size: 16
  batch_size: 2000
  max_rows: 200000
  keep_days: 7
```

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `workers` | 2 | 并发执行任务数 |
| `queue_size` | 16 | 等待队列；满后提交返回「导出任务过多」 |
| `batch_size` | 2000 | 游标每批取数 |
| `max_rows` | 200000 | 单任务最大行数 |
| `keep_days` | 7 | 产物保留天数，`clean_export_tasks` 使用 |

无环境变量覆盖。

## 8. Passkey / WebAuthn

```yaml
webauthn:
  rp_display_name: x-admin
  rp_id: localhost
  rp_origins:
    - http://localhost
    - http://localhost:5173
```

| 字段 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `rp_display_name` | `x-admin` | `X_ADMIN_WEBAUTHN_RP_DISPLAY_NAME` | 认证器显示名 |
| `rp_id` | `localhost` | `X_ADMIN_WEBAUTHN_RP_ID` | 有效域名，不带协议/端口 |
| `rp_origins` | localhost | `X_ADMIN_WEBAUTHN_RP_ORIGINS` | 完整来源，逗号分隔 |

生产必须是 HTTPS 且 `rp_id` 是 `rp_origins` 的父域（例如 `example.com` 对应 `https://example.com`）。配置非法只禁用 Passkey，不阻止启动。

## 9. CORS

```yaml
cors:
  origins:
    - http://localhost:5173
```

环境变量 `X_ADMIN_CORS_ORIGINS`（逗号分隔）。`"*"` 允许任意来源但不回显具体 Origin；生产建议写明确域名。

## 10. 环境变量速查

```bash
X_ADMIN_APP_ENV=production
X_ADMIN_SERVER_HOST=0.0.0.0
X_ADMIN_SERVER_PORT=8080
X_ADMIN_TRUSTED_PROXIES=127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
X_ADMIN_DATABASE_DSN='xadmin:pass@tcp(mysql:3306)/xadmin?charset=utf8mb4&parseTime=True&loc=Local'
X_ADMIN_REDIS_DRIVER=redis
X_ADMIN_REDIS_ADDR=redis:6379
X_ADMIN_REDIS_PASSWORD=
X_ADMIN_REDIS_DB=0
X_ADMIN_JWT_SECRET=<random>
X_ADMIN_JWT_ISSUER=x-admin
X_ADMIN_JWT_ACCESS_TTL=2h
X_ADMIN_JWT_REFRESH_TTL=168h
X_ADMIN_JWT_SESSION_TTL=12h
X_ADMIN_STORAGE_ROOT=/app/data/uploads
X_ADMIN_STORAGE_URL_PREFIX=/uploads
X_ADMIN_STORAGE_MAX_MB=20
X_ADMIN_WEBAUTHN_RP_ID=example.com
X_ADMIN_WEBAUTHN_RP_DISPLAY_NAME=x-admin
X_ADMIN_WEBAUTHN_RP_ORIGINS=https://example.com
X_ADMIN_CORS_ORIGINS=https://example.com
```

## 11. 启动前校验

`config.validate` 会拒绝：

- 端口不在 1–65535。
- JWT 密钥少于 16 字符，或生产环境使用公开示例值。
- DSN 为空。
- `storage.root` 为空。
- `trusted_proxies` 里既不是 IP 也不是 CIDR。

配置错误会直接退出并打印原因，不会带着明显错误的配置启动。

## 12. 常见问题

| 现象 | 原因 |
| --- | --- |
| 改了环境变量不生效 | 字段没有对应的 `X_ADMIN_*`，需要写 YAML |
| 生产启动报 default jwt secret | 使用了示例密钥，换随机值 |
| 所有请求 IP 相同 | `trusted_proxies` 没包含代理网段 |
| 迁移没执行 | `auto_migrate: false`，或数据库版本守卫拦截 |
| 上传超过 20MB 失败 | `storage.max_upload_mb` 或 Nginx `client_max_body_size` |
| Passkey 不可用 | `rp_id`/`rp_origins` 与访问地址不匹配，或非 HTTPS |
