# 在线会话与系统监控

这两个模块都面向运维：在线会话看「谁在登录」，系统监控看「服务本身是否健康」。

## 1. 在线会话

### 1.1 数据来源

在线会话不落库，完全来自缓存中的 `session:<userID>:<sessionID>` 记录。每条记录包含：

```json
{
  "ip": "127.0.0.1",
  "ua": "Mozilla/5.0 …",
  "created_at": "2026-11-10T09:00:00+08:00",
  "last_seen_at": "2026-11-10T10:30:00+08:00",
  "version": 3,
  "remember": true
}
```

- 登录时写入；刷新令牌时更新 `last_seen_at`（保留首次登录时间与 IP/UA）。
- `version` 必须等于用户当前的 `session_version`，否则视为过期会话。
- 用户被删除/停用、版本变化、记录损坏时，列表读取会顺手删除该键，避免幽灵会话。

### 1.2 管理员接口

`GET /online-sessions`（`system:online:list`）：

| 参数 | 说明 |
| --- | --- |
| `keyword` | 用户名、昵称、IP、设备名 |
| `user_id` | 只看某个用户 |
| `page` / `page_size` | 分页 |

响应在标准分页之外多一个 `users`（去重后的在线用户数）：

```json
{
  "list": [{
    "id": "session-id",
    "user_id": 1,
    "username": "admin",
    "nickname": "管理员",
    "ip": "127.0.0.1",
    "user_agent": "…",
    "device": "macOS · Chrome",
    "remember": true,
    "current": false,
    "created_at": "…",
    "last_seen_at": "…"
  }],
  "total": 1,
  "page": 1,
  "page_size": 20,
  "users": 1
}
```

强制下线 `DELETE /online-sessions/:userID`（`system:online:kick`）：

```json
{ "session_id": "…" }
```

- 带 `session_id`：只撤销该设备。
- 不带：撤销该用户全部会话。
- 撤销后对应 access token、refresh token、文件访问 Cookie 立即失效。
- 踢人动作本身不额外写业务日志，但该 `DELETE` 请求会被审计中间件记录一条操作日志。

### 1.3 个人中心

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/profile/sessions` | 当前用户设备列表，当前设备排最前 |
| DELETE | `/profile/sessions/:id` | 退出指定设备 |
| POST | `/profile/session-actions/revoke-others` | 退出除当前设备外的全部会话 |
| POST | `/profile/session-actions/revoke-all` | 退出全部会话（含当前） |

设备名由 User-Agent 粗略识别（iPhone/iPad/Android/macOS/Windows/Linux + Edge/Chrome/Firefox/Safari），识别不出显示「未知设备」。

### 1.4 运维注意

- 在线列表是**缓存全量扫描**（`Keys("session:")`），会话量大时会有性能压力；当前实现适合中小规模。规模上来后建议改为 Redis Set/Sorted Set 索引，或在网关层统计。
- 使用 `memory` 缓存时，在线会话仅在本进程有效，多副本互不可见。
- 管理员踢人只删缓存，不写业务审计；踢人请求本身会被审计中间件记录。

## 2. 系统监控

`GET /monitor/overview`（`system:monitor:view`）返回一次快照：

```json
{
  "app": {
    "name": "x-admin", "env": "production", "version": "1.3.0",
    "pid": 1234, "started_at": "…", "uptime_seconds": 3600
  },
  "system": { "host": {...}, "cpu": {...}, "memory": {...}, "swap": {...}, "load": {...}, "disks": [...], "network": {...}, "process": {...} },
  "runtime": { "go_version": "go1.27", "goroutines": 42, "gomaxprocs": 8, "alloc": 0, "heap_alloc": 0, "num_gc": 0, "gc_cpu_fraction": 0, "last_gc": "…" },
  "database": { "status": "up", "open": 3, "in_use": 1, "idle": 2, "max_open": 50, "wait_count": 0, "wait_duration_ms": 0, "latency_ms": 0.4 },
  "cache": { "driver": "redis", "status": "up", "latency_ms": 0.2 }
}
```

### 2.1 数据来源

| 部分 | 来源 | 说明 |
| --- | --- | --- |
| `app` | 进程内 | 名称/环境/版本、PID、启动时间、运行时长 |
| `system` | `internal/pkg/sysinfo` + gopsutil | 主机、CPU、内存、Swap、负载、磁盘、网络、进程 |
| `runtime` | `runtime.ReadMemStats` | Goroutine、GOMAXPROCS、堆/栈、GC 次数与暂停 |
| `database` | `sql.DB.Stats()` + Ping | 连接池使用、等待次数/时长、探测延迟 |
| `cache` | `cache.Store.Ping` | 驱动类型、状态、延迟 |

### 2.2 设计要点

- 数据库或缓存探测失败**不会**让整个接口 500，而是返回 `status: "down"`，监控页仍能展示其他指标。
- 先探测缓存/数据库，再采集可能受慢磁盘影响的主机指标，避免把正常依赖误报为 down。
- 主机指标依赖 gopsutil，在容器里读到的是**容器内**的 CPU/内存；需要宿主机指标请用 node_exporter 等外部方案。
- 版本号优先取配置 `app.version`，为空时读 Go 构建信息，再回退 `dev`。
- 监控接口不缓存，每次实时采集；前端可手动刷新。

### 2.3 健康检查

`GET /healthz` 是给容器/K8s 用的轻量探针：

- 数据库 `Ping` + 缓存 `Ping`，任一失败返回 `503`。
- 成功返回：

```json
{ "status": "ok" }
```

- Compose 的 backend healthcheck 就是 `wget http://127.0.0.1:8080/healthz`。
- 与 `/monitor/overview` 的区别：healthz 只做依赖连通性探测，不采集主机指标，开销小。

## 3. 前端页面

| 页面 | 路由 | 权限 | 能力 |
| --- | --- | --- | --- |
| 在线用户 | `/system/online` | `system:online:list` | 设备列表、强制下线、搜索 |
| 系统监控 | `/system/monitor` | `system:monitor:view` | 指标卡、趋势、依赖状态、磁盘/网络明细 |

监控页按 12 列网格布局：顶部四张指标卡，中部实时趋势为主视觉，右侧依赖状态，底部主机/运行时事实与磁盘/网络明细。数据刷新频率由页面控制，不轮询。

## 4. 常见问题

| 现象 | 排查 |
| --- | --- |
| 在线列表为空但用户已登录 | 用了 memory 缓存且请求打到了其他副本；或会话已过期 |
| 列表里有幽灵会话 | 旧版本数据；重新读取会自动清理，仍不消失可手动删 `session:*` |
| 踢人后对方还能访问 | access token 未走鉴权缓存？确认 `SessionChecker` 已启用、请求确实命中同一缓存 |
| 监控 CPU/内存是容器值 | 预期行为，宿主机监控请用外部采集 |
| 数据库 `status=down` | 连接池耗尽或网络不通，看 `/healthz` 与 DB 日志 |
| 监控接口很慢 | 主机磁盘/网络采集慢，或 gopsutil 在容器内读取受限；可精简 `sysinfo.Collect` |

## 5. 扩展

- 新增指标：在 `MonitorOverview` 加字段，填充 `MonitorService.Overview`，前端类型同步。
- 增加历史趋势：当前只返回瞬时快照；需要趋势应落时序库（Prometheus/InfluxDB），不要在 MySQL 里高频写。
- 多副本：每个副本的监控只反映自己；生产建议接入 Prometheus + Grafana，`/healthz` 留给探针。
