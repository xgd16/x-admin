# 异步导出框架：设计与使用

> 大数据量导出不该占着一个 HTTP 请求慢慢生成。x-admin 的做法：列表页点「导出」只提交一个任务，后台 worker 游标分批生成 xlsx，完成后站内通知，去「导出记录」页下载。本文讲清楚这套框架怎么用、怎么扩展新模块。

---

## 1. 它解决什么问题

同步导出（请求内直接返回文件流）有三个硬伤：

- 数据量大时请求超时、内存峰值高；
- 用户必须停在页面上等，关掉页面导出就断了；
- 无法统一限流——几个用户同时点导出就能把服务打满。

异步导出把「提交」和「生成」拆开：

```
列表页「导出」按钮 → POST /export-tasks（按当前筛选条件）
        ↓ 立即返回任务记录（pending）
进程内有界队列 → worker（默认 2 个）游标分批取数 → 流式写 xlsx → 存储落盘
        ↓
导出记录页轮询进度 → success 后下载；失败/取消可见原因
        ↓
站内通知（成功/失败都推，点击跳导出记录页）
```

已接入的模块：用户列表（`users`）、操作日志（`operation_logs`）、登录日志（`login_logs`）。

---

## 2. 使用方视角

### 2.1 列表页提交

「用户管理」「操作日志」「登录日志」页的「导出」按钮会把**当前筛选条件**原样带给后端：

```ts
// frontend/src/pages/Users.tsx
api.post('/export-tasks', {
  module: 'users',
  params: { keyword, dept_id: deptID, status },
})
```

提交成功提示「已加入导出队列」，之后可以离开页面干别的。

### 2.2 导出记录页（/system/exports）

- **进度**：存在排队/生成中任务时每 2 秒轮询，全部结束自动停轮询；
- **取消**：排队中、生成中均可取消，已结束的任务不可取消；
- **下载**：仅「已完成」的任务可下载，文件名按全局模板生成；
- **删除**：删除记录并清理产物文件；未结束的任务会先转为「请求取消」；
- **文件名配置**：弹层里可视化拼装文件名模板（见第 4 节），所见即所得。

---

## 3. API 一览

路由注册在 `internal/router/router.go`（前缀 `/admin/v1`），全部走登录态：

| 方法 | 路径 | 权限码 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/export-tasks` | `system:export:submit` | 提交任务，body 为 `{"module": "users", "params": {…筛选条件}}`，返回任务记录 |
| `GET` | `/export-tasks` | `system:export:list` | 分页查**当前用户**的任务，可按 `status` 过滤（0 排队 1 生成 2 完成 3 失败 4 已取消） |
| `POST` | `/export-tasks/:id/cancel` | `system:export:cancel` | 请求取消 |
| `DELETE` | `/export-tasks/:id` | `system:export:delete` | 删除已结束的记录与产物文件 |
| `GET` | `/export-tasks/:id/download` | `system:export:download` | 下载 xlsx 产物（流式） |
| `GET` | `/export-tasks/filename-template` | `system:export:list` | 读当前文件名模板 |
| `PUT` | `/export-tasks/filename-template` | `system:export:config` | 保存文件名模板 |

任务记录关键字段：`module` / `title` / `status` / `total`（预估总行数）/ `processed`（已写入行数）/ `file_name` / `file_size` / `error` / `finished_at`。注意 `file_key`（存储相对路径）不会下发前端，下载一律走导出接口，避免越权拼接路径。

归属校验：任务只属于提交人，查别人的任务一律 404（不暴露存在性）。

菜单与权限码见迁移 `migrations/000014_export_tasks.up.sql` 与 `000015_filename_template.up.sql`（「人员与权限」→ 新增「导出记录」菜单，按钮码 200–205）。

---

## 4. 任务生命周期与设计要点

代码主体在 `internal/service/export.go`。

### 4.1 状态机

```
pending(0) ── worker 出队 ──→ running(1) ──┬─→ success(2)   （写盘成功，记录 file_name/size）
    │                     │                ├─→ failed(3)    （error 记录原因，512 字截断）
    │ 取消(canceled=true) ─┘                └─→ canceled(4)
```

- 取消是**协作式**的：`Cancel` 只置 `canceled` 标志；worker 每批取数前、以及收尾写盘前都会复查标志，命中即停止并收敛为 canceled。收尾前那次复查是必须的——否则取消请求会丢在最后一批取数之后。
- 服务**重启**时，`Boot()` 把上次遗留的 pending/running 任务统一标记失败（「服务重启导致任务中断」），任务不会悬挂。
- `generate` 全程 recover 兜底，业务 builder 的 panic 会被记为任务失败而不是带崩 worker。

### 4.2 游标分批取数

业务侧提供两个函数（`ExportJob`）：

- `Total(ctx)`：预估总行数，只用于进度条，可为 nil（进度就没有分母）；
- `Fetch(ctx, afterID, limit)`：返回本批 `rows [][]any` 和下一批游标 `nextCursor`；`next == 0` 或空批表示取完。

框架侧的防护：

- `written >= MaxRows` 时任务失败，提示缩小筛选范围（默认 200000 行）；
- `next <= cursor`（游标不前进）立即失败，避免业务实现错误变成死循环；
- 每批写完更新 `processed`，前端进度条随批推进。

### 4.3 有界队列与限流

- 队列长度 `QueueSize`（默认 16），满了直接拒绝提交（429「导出任务过多」），不会无界堆积；
- 每用户未结束任务数 ≥ `QueueSize` 时同样拒绝；
- 与并发提交竞争队尾时，撤回已建的记录再拒绝，不留孤儿记录。

### 4.4 文件名模板

- 存在 `sys_parameters`，键 `export.filename_template`，值为片段数组 JSON；由导出记录页可视化配置，参数管理页只读展示；
- 片段类型：`title`（任务标题）、`module`（模块标识）、`time`（时间，格式白名单见 `FilenameTimeFormats`，杜绝 Windows 非法字符）、`text`（自定义文本，≤32 字）；最多 8 段，title/module/time 各最多一个；
- 默认模板 = `module + time(20060102150405)`；任一片段拼空则跳过，整体为空回退「模块-时间」；
- 后台生成与前端预览共用 `ComposeFilename`，所见即所得；配置变更对下一次导出立即生效。

### 4.5 产物与保留期

- 产物写入 `storage.Store`（与上传文件同一存储抽象，本地盘或对象存储）；
- 定时任务 handler `clean_export_tasks` 按 `keep_days`（默认 7 天）删除过期记录并清理文件，参数可传 `{"days": N}` 覆盖。

---

## 5. 配置

`configs/config.yaml`：

```yaml
export:
  workers: 2          # 并发执行的导出任务数
  queue_size: 16      # 等待队列长度，超出后提交返回"导出任务过多"
  batch_size: 2000    # 游标分页每批取数行数
  max_rows: 200000    # 单个任务允许导出的最大行数
  keep_days: 7        # 导出任务与文件的保留天数（定时任务 clean_export_tasks）
```

---

## 6. 扩展：给新模块接入导出

以「给字典管理加导出」为例，共四步。

### 第一步：写 Builder

在业务 service 上加 `ExportBuilders()`（参考 `internal/service/user.go` 的 `users` builder）：

```go
// ExportBuilders 返回字典模块的异步导出构建器。
func (s *DictService) ExportBuilders() map[string]ExportBuilder {
	return map[string]ExportBuilder{
		"dicts": func(ctx context.Context, userID uint64, params json.RawMessage) (ExportJob, error) {
			var f model.DictFilter
			if len(params) > 0 {
				if err := json.Unmarshal(params, &f); err != nil {
					return ExportJob{}, apperr.New(apperr.CodeInvalidArgument, "导出参数不合法")
				}
			}
			f.Page, f.PageSize = 1, 0 // 关键：清掉分页，让仓库层走游标导出查询
			return ExportJob{
				Module: "dicts",
				Title:  "字典列表", // 同时作为工作表名
				Headers: []string{"ID", "名称", "键", "值", "状态", "时间"},
				Total: func(ctx context.Context) (int64, error) {
					return s.repo.Count(ctx, f)
				},
				Fetch: func(ctx context.Context, afterID uint64, limit int) ([][]any, uint64, error) {
					rows, err := s.repo.ExportPage(ctx, f, afterID, limit) // WHERE id > afterID ORDER BY id LIMIT limit
					if err != nil {
						return nil, 0, err
					}
					out := make([][]any, 0, len(rows))
					for _, r := range rows {
						out = append(out, []any{r.ID, r.Name, r.Key, r.Value, r.Status,
							r.CreatedAt.Format("2006-01-02 15:04:05")})
					}
					var next uint64
					if len(rows) > 0 {
						next = rows[len(rows)-1].ID // 游标 = 本批最后一行业务主键
					}
					return out, next, nil
				},
			}, nil
		},
	}
}
```

要点：

- **Fetch 必须用游标分页**（`WHERE id > ? ORDER BY id LIMIT ?`），不要用 offset——深分页会越来越慢且数据抖动时会重/漏行；
- **游标必须严格前进**，否则框架判定异常直接失败；
- builder 里解析 `params` 时自己负责校验（它就是前端列表页的筛选条件 JSON）；
- 数据能一次性拿到（或由上游组装成 `[]map[string]any`）时，可以不写游标逻辑，直接用快捷封装 `NewMapsExportJob(module, title, spec, rows)`，列序与表头用 `MapExportSpec{Order, Names}` 控制，单元格值转换（时间格式化、切片/嵌套 map 转 JSON 等）由 `MapCellValue` 统一处理。

### 第二步：注册 Builder

在 `internal/bootstrap/providers.go` 的 `provideExportService` 里挂上：

```go
for module, b := range dicts.ExportBuilders() {
    svc.Register(module, b)
}
```

`module` 名与前端提交的 `module` 字段一致，重复注册后者覆盖前者。改了 provider 记得 `make wire`。

### 第三步：仓库层游标查询

给 repo 加 `ExportPage(ctx, f, afterID, limit)`。注意 `Base.Count` / 分页查询的签名复用问题：导出查询不走 `pagination.Query`，直接收 `afterID` 与 `limit` 两个参数即可。

### 第四步：前端入口

在列表页加导出按钮（权限显隐用 `useAuth().can('system:export:submit')`）：

```tsx
api.post('/export-tasks', { module: 'dicts', params: filterParams })
```

不需要动导出记录页——它按 module 无关的方式展示所有任务。

### 接入自查清单

- [ ] builder 里清掉了 `f.Page / f.PageSize`（否则仓库层可能仍按页截断）
- [ ] Fetch 游标严格前进、空批/`next=0` 正确表示结束
- [ ] 行数据统一过 `MapCellValue`（时间格式化、nil → 空串）
- [ ] `make wire` 重新生成装配
- [ ] 前端按钮挂了权限码，提交后 toast 提示 + 失效 `['export-tasks']` 查询缓存

### 已踩过的坑

- **excelize 默认工作表名**：新建工作簿自带 `Sheet1`，`excel.NewStreamBook` 已处理改名/去重，业务不要直接操作 excelize 实例；
- **取消丢失**：只在批间查 `canceled` 不够，收尾写盘前必须再查一次（框架已实现，自己改流程时别删）；
- **Count 签名冲突**：导出的 `Total` 走仓库层裸 `Count(ctx, filter)`，别和列表页带分页参数的 Count 混用。
