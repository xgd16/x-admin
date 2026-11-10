# 数据库、模型与迁移

x-admin 使用 MySQL 8.4 + GORM，迁移脚本通过 `embed` 打进二进制，启动时由 golang-migrate 执行。这一页覆盖建表约定、迁移流程、软删除与常用表。

## 1. 迁移机制

```text
migrations/*.sql  ──embed.FS──> 二进制
                                   │
database.Migrate ── golang-migrate ──> schema_migrations
```

关键实现（`internal/database/database.go`）：

- `NormalizeDSN` 自动补 `multiStatements=true`，因为单个迁移文件可能包含多条语句。
- `auto_migrate: true` 时启动执行 `m.Up()`；`ErrNoChange` 视为正常。
- **版本守卫**：如果数据库的 `schema_migrations.version` 大于当前二进制内置的最新版本（例如历史迁移被压缩、旧库版本更高），启动会失败并提示：
  - v1.3.0（schema 16）的库：备份后执行 `UPDATE schema_migrations SET version = <latest>, dirty = 0`；
  - 更老的版本：先升级到 v1.3.0，或重建数据库。
- **dirty 状态**：迁移中途失败会标记 dirty，启动直接报错；需要修复 SQL 后手动 `UPDATE schema_migrations SET dirty = 0` 或重建。
- 生产升级前**先备份**；不要依赖自动迁移做破坏性变更。

### 1.1 写一个迁移

```text
migrations/000017_announcements.up.sql
migrations/000017_announcements.down.sql
```

约定：

- 6 位递增编号，`_` 后是 snake_case 描述。
- 每个 up 必须有对应 down；down 要能安全回滚（先删外键/关联，再删表）。
- 已发布的迁移**永不修改**，结构变更追加新版本。
- 种子数据用 `INSERT IGNORE`，保证重复执行安全。
- 字符集统一 `utf8mb4`，时间字段用 `DATETIME(3)`，与 GORM 的 `time.Time` 对应。
- 索引命名 `idx_<table>_<columns>`，唯一索引 `uk_<table>_<columns>`。

### 1.2 不要用 AutoMigrate 上生产

`database.AutoMigrate` 只给测试和本地实验（SQLite）用，覆盖全部模型；生产的表结构以 `migrations/*.sql` 为准。两者不一致时，以迁移为准。

## 2. 模型约定

```go
type Announcement struct {
    Base
    Title  string `gorm:"size:128;not null" json:"title"`
    Status int8   `gorm:"not null;index" json:"status"`
}

func (Announcement) TableName() string { return "sys_announcements" }
```

- 所有业务模型内嵌 `model.Base`：`ID / CreatedAt / UpdatedAt / DeletedAt`。
- 表名统一 `sys_` 前缀，通过 `TableName()` 显式指定。
- 敏感字段（密码、TOTP 密钥、恢复码、`session_version`、存储 key）用 `json:"-"`，永不返回前端。
- 状态常量：`model.StatusDisabled=0`、`model.StatusEnabled=1`。
- 菜单类型：`MenuDirectory=1`、`MenuMenu=2`、`MenuButton=3`。
- 模型包不写逻辑；查询条件放 `model/filter.go`，内嵌 `pagination.Query`。
- 只在需要时建关联：用户预加载 `Dept`、`Roles`，列表计数用 `DISTINCT` 避免 JOIN 多行。
- 例外：**通行密钥删除、日志清理、任务执行记录清理**使用 `Unscoped()` 物理删除；其余业务删除走软删除。

## 3. 软删除与唯一索引

GORM 默认软删除（`deleted_at`）。这带来一个经典问题：软删除后 `username/code/key` 仍占着唯一索引，无法重建同名数据。

解决方案（`repository/tombstone.go`）：

```go
// 删除时把唯一字段改成 "#deleted#<id>" 墓碑值
UPDATE sys_users SET username = CONCAT(username, '#deleted#', id), deleted_at = NOW()
```

实际实现会保留原值前缀并截断到列宽，后缀始终保留。适用表：

- `sys_users.username`
- `sys_roles.code`
- `sys_dict_types.code`
- `sys_parameters.key`

删除逻辑在仓储事务里：先改唯一字段 → 清理关联（`sys_user_roles` / `sys_role_menus`）→ 软删除主记录。

## 4. 仓储模式

```go
type Base[T any] struct{ db *gorm.DB }

NewBase[T](db)
Create / Update / UpdateFields / Delete / FindByID / Count / List
```

- `Update` 是 `Save`，会写全部字段（含零值）；日常更新优先 `UpdateFields`，避免覆盖并发修改的密码/TOTP/version。
- `UpdateFields` 在 `RowsAffected=0` 时返回 `ErrNotFound`。
- `List(page,pageSize,...)`：`pageSize<=0` 表示不分页，导出场景使用。
- `paginate` 统一 offset/limit；`like` 生成 `%keyword%`。

复杂列表（用户、日志、导出）在各自仓储里写查询，不强行套 `Base.List`。

### 4.1 游标分页（导出）

导出不能一次性 `Find` 全表，使用主键游标：

```go
func (r *UserRepo) ExportPage(ctx, f, afterID uint64, limit int) ([]model.User, error) {
    q := applyUserFilter(...)
    if afterID > 0 {
        q = q.Where("sys_users.id > ?", afterID)
    }
    return q.Order("id ASC").Limit(limit).Find(&users)
}
```

每批返回最后一行 ID 作为下一批的 `afterID`，内存占用与总行数无关。

## 5. 表清单

| 表 | 用途 | 关键索引 / 保留策略 |
| --- | --- | --- |
| `sys_users` | 账号、部门、状态、TOTP、session_version | `username` 唯一；`dept_id`、`status`、`email`、`phone` |
| `sys_roles` | 角色 | `code` 唯一；`status` |
| `sys_menus` | 目录/菜单/按钮 | `parent_id`、`type`、`permission`、`status` |
| `sys_departments` | 部门树 | `parent_id`、`status` |
| `sys_user_roles` | 用户-角色 | 联合主键 |
| `sys_role_menus` | 角色-菜单 | 联合主键 |
| `sys_dict_types` | 字典类型 | `code` 唯一 |
| `sys_dict_items` | 字典项 | `dict_type_id`、`status` |
| `sys_parameters` | 系统参数 | `key` 唯一 |
| `sys_files` | 文件元数据 | `key` 唯一；`uploader_id` |
| `sys_file_shares` | 分享令牌摘要 | `token_hash` 唯一；`file_id`、`expires_at`；30 天清理 |
| `sys_jobs` | 定时任务 | `status` |
| `sys_job_runs` | 执行历史 | `job_id`、`status`；保留 30 天 |
| `sys_export_tasks` | 异步导出任务 | `user_id` + `status`；保留 `export.keep_days` |
| `sys_operation_logs` | 操作审计 | `user_id`、`username`、`request_id`；默认 90 天 |
| `sys_login_logs` | 登录审计 | `user_id`、`username`；默认 90 天 |
| `sys_notifications` | 站内通知 | `(user_id, read_at)` 复合；默认 90 天 |
| `sys_passkeys` | WebAuthn 凭据 | `credential_id` 唯一；`user_id` |

## 6. 时间与时区

- GORM `NowFunc` 使用 `time.Now().Local()`，容器请设 `TZ=Asia/Shanghai`。
- DSN 带 `parseTime=True&loc=Local`。
- 迁移里的时间列用 `DATETIME(3)`，默认/更新由 SQL 或 GORM 维护。
- 前端用 dayjs 展示，接口时间是 RFC3339（带时区）。

## 7. 备份与恢复

```bash
# 备份
mysqldump -h127.0.0.1 -uxadmin -p \
  --single-transaction --routines --triggers --databases xadmin \
  > xadmin-$(date +%F).sql

# 恢复（先停应用，避免写入）
mysql -h127.0.0.1 -uxadmin -p xadmin < xadmin-2026-11-10.sql
```

- 备份包含 `schema_migrations`，恢复后版本与迁移记录一致。
- 升级前同时备份 `data/uploads`（或对象存储桶）。
- 迁移压缩/版本调整后，按启动报错里的提示处理 `schema_migrations.version`。

## 8. 测试中的数据库

`tests/testsupport.New` 使用内存 SQLite + `database.AutoMigrate`，每个测试独立库名，结束自动关闭。它不执行 MySQL 迁移，所以：

- 新模型必须同时加到 `database.AutoMigrate` 的列表，否则测试建表失败。
- SQLite 与 MySQL 的差异（大小写、`ON UPDATE`、JSON 函数）要在测试中避开，或放到集成环境验证。
- 关闭连接池前不要复用 `*gorm.DB`；每个测试用新实例。

## 9. 常见问题

| 现象 | 原因 / 处理 |
| --- | --- |
| 启动报 `schema version N is not covered` | 数据库版本高于二进制内置迁移；按报错提示调整 `schema_migrations.version` 或升级二进制 |
| 启动报 `schema version N is dirty` | 迁移中途失败；修复 SQL 后清 dirty 或重建库 |
| 软删除后无法重建同名用户 | 删除逻辑未写墓碑值；检查仓储事务 |
| 列表 total 比实际多 | JOIN 导致重复计数；用 `DISTINCT` 主表 ID |
| 时间差 8 小时 | 容器 TZ / DSN loc 配置不一致 |
| 测试建表缺表 | 新模型没加进 `AutoMigrate` |
