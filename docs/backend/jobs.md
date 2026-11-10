# 定时任务

定时任务由三部分组成：`jobs.Cron`（调度）、`JobService`（业务编排与执行记录）、`provideJobExecutor`（handler 名称 → 具体逻辑）。

## 1. 数据模型

```go
type Job struct {
    Base
    Name       string
    Cron       string // 标准 5 段：分 时 日 月 周
    Handler    string // clean_logs | clean_notifications | clean_export_tasks | noop
    Params     string // JSON，例如 {"days":90}
    Status     int8   // 0 停用 / 1 启用
    LastRunAt  *time.Time
    LastResult string
    Remark     string
}

type JobRun struct {
    Base
    JobID, JobName, Trigger, Status, Result, StartedAt, FinishedAt, DurationMS
}
```

- 任务定义存 `sys_jobs`。
- 每次手动/Cron 执行写一条 `sys_job_runs`，**默认只保留 30 天**（执行时自动清理更早记录）。
- `last_result` 保存最近一次结果摘要（成功为「成功」，失败为错误信息，截断到 255 字节且不切断多字节字符）。

## 2. 调度器

`internal/jobs/cron.go` 对 `robfig/cron` 做了一层轻量封装：

- `cron.ParseStandard` 校验 5 段表达式（分 时 日 月 周）。
- `Add(name, spec, fn)` 同名任务先移除旧 entry，保证一个业务任务只有一个调度。
- `cron.WithChain(cron.SkipIfStillRunning())`：上一次还没结束就跳过本次，清理类任务不会重叠。
- 服务层用 `job:<id>` 作为调度器内的唯一名称。

应用启动时 `provideJobService` 调用 `Boot()`，把所有 `status=1` 的任务注册进调度器。

## 3. 内置 handler

| Handler | 默认参数 | 逻辑 |
| --- | --- | --- |
| `clean_logs` | `{"days":90}` | 物理删除超过保留期的操作/登录日志 |
| `clean_notifications` | `{"days":90}` | 物理删除超过保留期的站内通知 |
| `clean_export_tasks` | `{"days":7}`（缺省取 `export.keep_days`） | 删除过期导出任务记录与产物文件 |
| `noop` | 无 | 空任务，用于验证调度链路 |

参数解析失败时使用默认保留天数，不会让任务报错停机。

## 4. 一次执行的流程

```text
手动 POST /jobs/:id/run          cron 触发
        │                           │
        └────────── Execute(id, trigger) ──────────┘
                           │
                ① 读取任务定义（不存在 → 404）
                ② 多副本缓存锁 job:lock:<id>（30 分钟兜底）
                   抢不到：手动返回 409，Cron 静默跳过
                ③ 调用 executor（按 Handler 分发）
                ④ 更新 last_run_at / last_result
                ⑤ 写 sys_job_runs（trigger=manual|cron）
                ⑥ 清理 30 天前的执行记录
```

- 锁是 `cache.Store.SetNX`。Redis 故障时会记录警告并**退化为进程内** `SkipIfStillRunning`，不让所有任务停摆。
- 手动执行失败会返回 `50001`（消息「任务执行失败」）；Cron 回调忽略错误，避免同一周期重试风暴。
- 执行上下文使用 `context.Background()`，不随 HTTP 请求结束而取消。

## 5. 接口

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET/POST | `/jobs` | `system:job:list` / `system:job:create` | 分页 / 新建 |
| PUT/DELETE | `/jobs/:id` | `system:job:update` / `system:job:delete` | 更新 / 删除（同时删执行记录） |
| PUT | `/jobs/:id/status` | `system:job:update` | 启停，同步调度器 |
| POST | `/jobs/:id/run` | `system:job:run` | 立即执行一次，不改变计划 |
| GET | `/jobs/:id/runs` | `system:job:list` | 执行历史分页 |

创建/更新请求体：

```json
{
  "name": "清理登录日志",
  "cron": "0 3 * * *",
  "handler": "clean_logs",
  "params": "{\"days\":90}",
  "status": 1,
  "remark": "每天 03:00"
}
```

`status` 为 `null` 时：创建默认启用，更新保持原状态。

## 6. 新增一个 handler

以「清理过期文件分享」为例：

### 6.1 写执行逻辑

在合适的 repository/service 上提供方法，例如：

```go
func (r *FileRepo) PruneShares(ctx context.Context, before time.Time) error {
    return r.db.WithContext(ctx).Where("expires_at < ? OR revoked_at < ?", before, before).
        Delete(&model.FileShare{}).Error
}
```

### 6.2 注册到 executor

`internal/bootstrap/providers.go`：

```go
func provideJobExecutor(...) func(ctx context.Context, job *model.Job) error {
    return func(ctx context.Context, job *model.Job) error {
        switch job.Handler {
        case "clean_logs":
            return logs.CleanBefore(ctx, daysFrom(job.Params))
        case "clean_notifications":
            return notifications.CleanBefore(ctx, daysFrom(job.Params))
        case "clean_export_tasks":
            before := time.Now().AddDate(0, 0, -keepDaysFrom(job.Params, cfg.Export.KeepDays))
            return exports.CleanBefore(ctx, before)
        case "clean_file_shares":
            return files.PruneSharesBefore(ctx, time.Now().AddDate(0, 0, -daysFrom(job.Params)))
        case "noop":
            return nil
        default:
            return fmt.Errorf("unknown job handler %q", job.Handler)
        }
    }
}
```

- executor 的依赖通过 wire 注入，不要在这里 `New` 仓储。
- 返回值会被写入 `last_result` 和执行记录，尽量返回可读错误。
- 参数统一 JSON，解析失败给默认值，保持任务幂等。

### 6.3 建任务

在「定时任务」页面新建，`handler` 填 `clean_file_shares`，`params` 填 `{"days":30}`，`cron` 例如 `30 4 * * *`。也可以在迁移里写 `INSERT IGNORE INTO sys_jobs ...`。

## 7. 生产注意事项

- **单副本**：进程内调度即可，`SkipIfStillRunning` 保证不重叠。
- **多副本**：必须使用 Redis（`redis.driver=redis`），依赖 `job:lock:<id>` 保证同一时刻只有一个实例执行。内存缓存无法跨进程，只适合单副本。
- **时钟**：Cron 使用服务器本地时区；容器里请设置 `TZ=Asia/Shanghai`（Compose 已设置）。
- **长任务**：执行锁兜底 30 分钟；超过 30 分钟的任务可能被其他副本重复执行，长任务建议拆小或调大锁 TTL。
- **历史表**：`sys_job_runs` 高频任务增长快，已内置 30 天清理；如需更长保留期，调整 `Execute` 里的清理窗口。
- **可观测性**：任务结果同时写 `last_result` 和 `sys_job_runs`，排查时先看执行记录，再看应用日志中的 `job_id`。

## 8. 常见问题

| 现象 | 原因 |
| --- | --- |
| 新建任务后不执行 | `status=0`，或 Cron 表达式不是 5 段 |
| 手动执行返回 409 | 任务正在执行（缓存锁未释放），等待或检查是否有卡死任务 |
| 多副本重复执行 | 使用了 memory 缓存，或 Redis 不可用导致锁退化 |
| 执行记录为空 | 任务从未执行；或 `sys_job_runs` 已被 30 天清理 |
| 任务报 unknown handler | handler 名称与 `provideJobExecutor` 的 switch 不一致 |
| 修改 Cron 不生效 | 更新接口会重建调度；确认返回成功且 `status=1` |
