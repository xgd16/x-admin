# 排错手册

按现象查。每条都给出「先看什么 → 常见原因 → 处理」。排查时优先拿响应里的 `request_id`，再去日志平台按 ID 搜。

## 1. 启动失败

| 现象 | 先看 | 常见原因 / 处理 |
| --- | --- | --- |
| `config: jwt secret must be at least 16 chars` | 配置 | `jwt.secret` 太短，换随机值 |
| `config: default jwt secret is not allowed in production` | 配置 | 生产用了示例密钥 |
| `database: ping` | MySQL | 地址/账号/网络不通；容器内用服务名 `mysql:3306` |
| `config: invalid trusted proxy` | 配置 | 代理网段写成域名或带协议，必须是 IP/CIDR |
| `schema version N is not covered` | `schema_migrations` | 数据库版本高于二进制内置迁移，按报错提示调整或升级 |
| `schema version N is dirty` | 迁移日志 | 迁移中途失败；修复 SQL 后清 dirty 或重建 |
| `redis: ...` | Redis | 地址/密码不对；只想本地跑可改 `redis.driver=memory` |
| `bind: address already in use` | 端口 | 8080 被占用，改 `server.port` 或停旧进程 |
| `storage: root is required` | 配置 | `storage.root` 为空 |

## 2. 登录与认证

| 现象 | 排查 |
| --- | --- |
| 登录一直提示验证码错误 | 验证码一次性、5 分钟过期；确认用的是最新 `captcha_id`，且请求没有重复提交 |
| `42301` 账号锁定 | 连续失败达到 `max_login_fails`；等待 `lock_duration`，或清缓存 `login:fail:<username>` |
| 登录成功但立刻 401 | `/auth/me` 失败：access 过期、用户被停用、session_version 变化、Redis 不共享 |
| 刷新接口 40102 | refresh token 已被使用/过期；多标签页同时刷新会触发一次性轮换 |
| 所有用户突然掉线 | 服务重启且用 memory 缓存；或 Redis 被清空/切换 db |
| MFA 验证码总是不对 | 服务器时间与手机偏差超过 30 秒；恢复码是否已用过 |
| Passkey 按钮不显示 | RP ID/Origin 不匹配、非 HTTPS/localhost，或 `passkey support disabled` 日志 |
| 锁屏后无法解锁 | 密码错误次数达到上限（独立计数 `unlock:fail:<uid>`），等待锁定时间 |
| 退出登录后仍能访问 | 用了旧 access token 且请求未走 Auth？确认接口在 `secured` 分组 |

## 3. 权限

| 现象 | 排查 |
| --- | --- |
| 接口 40301 | 角色是否授权对应菜单/按钮；菜单 `status=1`；权限码与 `Require` 一致；`super_admin` 是否启用了角色 |
| 菜单不显示 | `/auth/me` 返回的 `menus` 是否包含；`visible=1`、`status=1`、`type=2`；前端 `navItems` 是否注册该路径 |
| 按钮不显示 | 前端 `can('...')` 的权限码与后端菜单不一致 |
| 直接输 URL 显示 403 | `permissionForPath` 拦截，属预期；给角色授权或隐藏入口 |
| 改权限后没生效 | 权限每次请求重新加载；确认角色/菜单状态，刷新 `/auth/me` |
| 无法分配高权限角色 | 非超级管理员只能分配权限不超过自己的角色 |

## 4. 文件与上传

| 现象 | 排查 |
| --- | --- |
| 上传 413 | Nginx `client_max_body_size`；后端 64MB 硬上限 |
| 上传提示超过 20MB | `storage.max_upload_mb`；改配置后重启 |
| 图片 401 | `/uploads` 未反代到后端；文件 Cookie 过期；跨域时 Cookie 不发送 |
| 图片 403/404 | 文件记录存在但磁盘丢失；检查 `data/uploads` 卷 |
| 分享链接 401 | 令牌过期/被撤销；`share` 参数与文件路径不匹配 |
| 删除文件后磁盘还有 | 元数据先删、磁盘删除失败；用运维脚本清理孤儿文件 |
| 多副本图片一半 404 | 使用了本地磁盘且未共享；换共享卷/对象存储 |
| SVG/HTML 打开被拦截 | `CSP sandbox` 预期行为，防止存储型 XSS |

## 5. 异步导出

| 现象 | 排查 |
| --- | --- |
| 提交提示任务过多 | `export.queue_size` 满；等 worker 消费或调大队列 |
| 任务一直排队 | worker 数为 0 或进程未启动；检查 `export.workers` |
| 任务失败 | 看任务 `error` 字段与后端日志；常见是 SQL 超时/参数不合法 |
| 下载 404 | 产物已过期（`keep_days`）或被清理；重新导出 |
| 导出文件为空 | 筛选条件过窄；`Total=0` 时也会生成只有表头的文件 |
| 大量导出内存高 | 确认走的是 `StreamBook`/游标分页，而不是一次性 `Find` |

## 6. 定时任务

| 现象 | 排查 |
| --- | --- |
| 任务不执行 | `status=0`；Cron 不是 5 段；应用未启动 |
| 手动执行 409 | 任务正在执行，缓存锁未释放；等锁过期或检查卡死任务 |
| 多副本重复执行 | `redis.driver=memory`，或 Redis 故障导致锁 fail-open |
| 执行记录为空 | 任务从未执行；`sys_job_runs` 30 天清理 |
| unknown job handler | handler 名称与 `provideJobExecutor` 的 switch 不一致 |
| 时区不对 | 容器未设 `TZ=Asia/Shanghai` |

## 7. 监控与在线会话

| 现象 | 排查 |
| --- | --- |
| 在线列表为空 | memory 缓存多副本；会话已过期；Redis 被清 |
| 列表有幽灵会话 | 旧数据，重新读取会清理；仍不消失手动删 `session:*` |
| 踢人无效 | 请求打到了另一个副本且缓存不共享；确认 Redis 统一 |
| CPU/内存是容器值 | 预期；宿主机监控用 node_exporter |
| 监控接口慢 | 主机磁盘/网络采集慢；精简 `sysinfo.Collect` |
| `/healthz` 503 | 数据库或缓存 Ping 失败；分别检查 |

## 8. 前端

| 现象 | 排查 |
| --- | --- |
| 页面白屏 | 看 Console；`root element missing`、构建产物未部署、JS 404 |
| 刷新 404 | Nginx 缺 SPA fallback |
| 请求跨域 | `VITE_API_BASE` / `cors.origins` / 代理配置 |
| 401 循环 | refresh 失败后仍跳登录；检查 `noRefreshPaths` 与后端 refresh |
| 主题色不生效 | 写死颜色；检查 `data-theme/data-palette` 与 CSS 变量 |
| 手机布局错乱 | 用了 `window.innerWidth`；改用 `useIsCompactViewport` |
| 页签切换丢状态 | 页面不在 `KeepAliveOutlet` 下，或被 `key` 强制重挂载 |
| antd 弹窗样式异常 | 没包 `ThemeProvider` / `AntdApp` 上下文 |

## 9. 文档站

| 现象 | 排查 |
| --- | --- |
| Actions 构建失败 | 看 `mkdocs build --strict` 的 WARNING：多为相对链接指向不存在的页面 |
| 页面 404 | `mkdocs.yml` 的 `nav` 路径与文件名不一致 |
| 搜索没结果 | 中文搜索依赖 `jieba`；确认 `docs-requirements.txt` 已安装 |
| 站点样式旧 | GitHub Pages 缓存；强刷或等部署完成 |
| 图片不显示 | 图片放在 `docs/` 下且路径大小写一致 |

## 10. 性能问题

| 现象 | 排查方向 |
| --- | --- |
| 列表慢 | 缺索引（`status`、`dept_id`、`created_at`）；`keyword` 前后模糊无法用索引；分页过深 |
| 登录慢 | bcrypt cost 过高；数据库/Redis 延迟；锁竞争 |
| 导出慢 | 批次太小、SQL 未走索引、单任务行数过大 |
| 内存高 | 导出一次性加载、图片/文件未流式、日志队列积压 |
| 429 多 | 全局 100/min/IP 太小，或代理导致 IP 聚合；调大或按用户维度限流 |
| 监控接口慢 | gopsutil 采集磁盘/网络；减少采集项或异步缓存 |

## 11. 常用诊断命令

```bash
# 健康检查
curl -i http://127.0.0.1:8080/healthz

# 带 token 调接口
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/admin/v1/auth/me

# 查迁移版本
mysql -e "SELECT * FROM xadmin.schema_migrations"

# 查 Redis 会话
redis-cli --scan --pattern 'session:*' | head

# 查任务锁
redis-cli --scan --pattern 'job:lock:*'

# 查后端日志（Compose）
docker compose -f deploy/docker-compose.yml logs -f --tail=200 backend

# 查 Nginx 错误
tail -f /var/log/nginx/error.log
```

## 12. 提 issue 时请附上

- 版本/提交号（`git rev-parse --short HEAD`）与部署方式。
- `request_id`、接口路径、HTTP 状态与响应体。
- 相关日志（去掉密码/token）。
- 数据库版本（`schema_migrations`）与是否多副本。
- 是否使用 Redis、共享存储、反向代理。
