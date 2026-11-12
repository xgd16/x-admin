# 部署与发布

x-admin 是「前端静态资源 + 后端单二进制 + MySQL + Redis」的分离部署。推荐 Docker Compose；也可以手动部署到已有 Nginx。

## 1. 部署拓扑

```text
              ┌──────────────┐
   :80/:443   │    Nginx     │
  ───────────►│  SPA + 反代  │
              └──────┬───────┘
                     │ /admin/v1 /uploads /swagger /healthz
              ┌──────▼───────┐        ┌──────────┐
              │   backend    │───────►│  MySQL   │
              │  Go 单二进制  │───────►│  Redis   │
              └──────┬───────┘        └──────────┘
                     │
              data/uploads（卷/对象存储）
```

## 2. Docker Compose（推荐）

```bash
cp .env.example .env
# 必填：JWT_SECRET=<32位以上随机值>
# 生产：WEBAUTHN_RP_ID / WEBAUTHN_RP_ORIGINS / CORS 改成真实域名
make compose-up
```

Compose 服务：

| 服务 | 镜像/构建 | 端口 | 说明 |
| --- | --- | --- | --- |
| `mysql` | `mysql:8.4` | 内部 3306 | utf8mb4，数据卷 `mysql-data` |
| `redis` | `redis:7.4-alpine` | 内部 6379 | 数据卷 `redis-data` |
| `backend` | `deploy/Dockerfile.backend` | 8080 | 非 root 用户，健康检查 `/healthz` |
| `frontend` | `deploy/Dockerfile.frontend` | 80 | Nginx 托管 SPA 并反代 |

卷：

- `mysql-data`：数据库
- `redis-data`：缓存（可丢，但会丢失会话）
- `uploads`：上传文件（**必须持久化/备份**）

常用命令：

```bash
make compose-logs        # 跟踪日志
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml restart backend
make compose-down
```

## 3. 手动部署

### 3.1 构建后端

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o x-admin ./cmd/server
```

把 `x-admin`、`configs/config.yaml`（或环境变量）、`data/uploads` 放到服务器：

```bash
./x-admin -config /etc/x-admin/config.yaml
```

建议用 systemd 管理：

```ini
[Unit]
Description=x-admin
After=network.target mysql.service redis.service

[Service]
User=xadmin
WorkingDirectory=/opt/x-admin
ExecStart=/opt/x-admin/x-admin -config /opt/x-admin/config.yaml
Restart=always
Environment=TZ=Asia/Shanghai
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

### 3.2 构建前端

```bash
cd frontend
npm ci
npm run build
# 产物在 frontend/dist/
```

如果前端与后端不同源，构建时指定：

```bash
VITE_API_BASE=https://api.example.com/admin/v1 npm run build
```

同时把后端 `cors.origins` 配成前端域名。

### 3.3 Nginx

`deploy/nginx.conf` 是现成模板，核心规则：

```nginx
client_max_body_size 64m;

location /assets/ {
  expires 1y;
  add_header Cache-Control "public, immutable";
  try_files $uri =404;
}

location /admin/v1/ {
  proxy_pass http://127.0.0.1:8080;
  proxy_set_header Host $host;
  proxy_set_header X-Real-IP $remote_addr;
  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  proxy_set_header X-Forwarded-Proto $scheme;
  proxy_read_timeout 60s;
}

location /uploads/ {
  proxy_pass http://127.0.0.1:8080;
  # 同样透传 Host / X-Real-IP / X-Forwarded-For
}

location /swagger { proxy_pass http://127.0.0.1:8080; }
location = /healthz { proxy_pass http://127.0.0.1:8080; }

location / {
  try_files $uri $uri/ /index.html;
}
```

注意：

- `client_max_body_size` 要 ≥ 后端上传上限（默认 64MB 硬上限）。
- `/uploads` 必须反代到后端，不能直接 `alias` 到磁盘目录，否则绕过鉴权与安全头。
- SPA 刷新 404 必须靠 `try_files ... /index.html`。
- 生产必须有 HTTPS；Passkey 与 Secure Cookie 都依赖 HTTPS。

## 4. HTTPS 与域名

以 Certbot 为例：

```bash
certbot --nginx -d example.com
```

然后更新：

```yaml
app:
  env: production
cors:
  origins: ["https://example.com"]
webauthn:
  rp_id: example.com
  rp_origins: ["https://example.com"]
```

`app.env=production` 时：

- Gin 关闭 debug。
- 日志改为 JSON。
- 文件访问 Cookie 在 HTTPS 下加 `Secure`。
- 启动拒绝示例 JWT 密钥。

## 5. 生产检查清单

- [ ] `JWT_SECRET` 是随机值（≥32 字符），不是示例值
- [ ] `app.env=production`
- [ ] MySQL / Redis 使用强密码，且不暴露公网端口
- [ ] `server.trusted_proxies` 只包含实际代理网段
- [ ] HTTPS 已启用，HTTP 自动跳转
- [ ] `cors.origins` 是明确域名，不是 `*`
- [ ] `webauthn.rp_id` / `rp_origins` 与域名一致
- [ ] `data/uploads` 使用持久卷并纳入备份
- [ ] 数据库迁移已备份；`auto_migrate` 策略明确
- [ ] `/healthz` 已接入探针/监控
- [ ] 日志接入采集（JSON stdout）
- [ ] 限流依赖 Redis，未使用 memory
- [ ] 定时任务使用 Redis 锁（多副本）
- [ ] 默认 `admin` 密码已修改，或已删除演示数据

## 6. 多副本与扩容

- **无状态**：backend 不存本地状态，会话在 Redis，文件在共享存储。
- **文件**：多副本必须共享卷（NFS/云盘）或换对象存储；本地磁盘会读不到别的副本写的文件。
- **定时任务**：使用 Redis 后 `job:lock:<id>` 保证同一任务只执行一次；memory 缓存只能单副本。
- **在线会话/监控**：每个副本只统计自己的会话/指标；全局视图需要外部采集。
- **导出**：worker 在各自进程内，产物写共享存储；任务表在 MySQL，天然共享。
- **Nginx**：可以水平扩展，但需保证 sticky 不必要（JWT 无状态）。

## 7. 备份与恢复

```bash
# 数据库
mysqldump --single-transaction --routines --triggers xadmin > xadmin.sql

# 上传文件
tar -czf uploads.tar.gz data/uploads

# 恢复
mysql xadmin < xadmin.sql
tar -xzf uploads.tar.gz
```

- 升级前备份数据库 + 上传目录。
- Redis 不是权威数据源：丢了只需重新登录，不需要恢复。
- 定期验证恢复流程，不要只备份不演练。

## 8. 升级流程

1. 备份数据库与上传目录。
2. 拉取新版本，构建后端/前端。
3. 停旧进程（或滚动更新）。
4. 启动新二进制，观察迁移日志与 `/healthz`。
5. 验证登录、文件、导出、定时任务。
6. 出问题回滚二进制 + 数据库备份（迁移一般不可逆，回滚前先看 down 脚本）。

## 9. CI / 发布工作流

| 工作流 | 触发 | 内容 |
| --- | --- | --- |
| `.github/workflows/ci.yml` | push main / PR / 手动 | 后端 vet + test + coverage，前端 lint + test + build |
| `.github/workflows/release.yml` | Release published / 手动 | 交叉编译 linux/darwin/windows，打包配置与 README，构建前端 zip，附到 Release |
| `.github/workflows/docs.yml` | docs / mkdocs 变更 | 构建 MkDocs 并发布 GitHub Pages |

发布：

```bash
git tag v1.4.0
git push origin v1.4.0
# 在 GitHub 创建 Release，release.yml 自动上传各平台产物与 checksums.txt
```

发布前建议把根目录 `FRAMEWORK_VERSION` 更新到本次 tag。它不参与二进制运行，只用于下游源码项目的升级基准；复制源码后新 `git init`、没有上游 git 历史的项目会优先读取它。`.x-admin/framework.json` 是下游本地缓存，不需要维护者处理。

## 10. 文档站

本站由 MkDocs Material 构建，发布在 GitHub Pages：

- 源文件在 `docs/`，导航在 `mkdocs.yml`。
- 推送到 `main` 且改动 `docs/**`、`SPEC.md`、`mkdocs.yml` 时自动构建。
- 本地预览：

```bash
pip install -r docs-requirements.txt
mkdocs serve
```

## 11. 常见部署问题

| 现象 | 排查 |
| --- | --- |
| 前端刷新 404 | Nginx 缺 `try_files $uri $uri/ /index.html` |
| 接口 502 | 后端未启动/端口不对；看 Nginx error log 与 backend 日志 |
| 上传 413 | Nginx `client_max_body_size` 太小 |
| 图片 401 | `/uploads` 未反代到后端，或 Cookie 域/路径不对 |
| 登录后立刻退出 | `JWT_SECRET` 多副本不一致；或 Redis 不共享 |
| Passkey 注册失败 | RP ID/Origin 与访问地址不一致 |
| CORS 报错 | `cors.origins` 没包含前端来源；或代理丢了 `Origin` |
| 时间差 8 小时 | 容器 `TZ` 未设置，DSN `loc` 不一致 |
| 迁移启动失败 | 数据库版本高于内置迁移，或处于 dirty；按报错处理 |
