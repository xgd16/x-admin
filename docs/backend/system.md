# 字典、参数、通知与看板

这四个模块共同构成「系统配置与运营信息」层：字典管枚举展示，参数管运行期配置，通知管站内消息，看板做首页聚合。

## 1. 字典管理

### 1.1 模型

```text
sys_dict_types (id, code 唯一, name, status, remark)
        │ 1:N
sys_dict_items (id, dict_type_id, label, value, sort, status, tag_color)
```

- `code` 是稳定标识（如 `user_status`），前端/dict 组件按 code 取值。
- `label` 展示，`value` 存储，`tag_color` 给前端标签配色。
- 删除字典类型会级联删除其全部字典项（仓储事务内完成）。
- 字典编码规则：小写字母开头，2–64 位小写字母/数字/下划线。

内置字典（种子数据）：

| code | 用途 | 项 |
| --- | --- | --- |
| `user_status` | 用户状态 | 正常 1 / 停用 0 |
| `job_status` | 任务状态 | 启用 1 / 停用 0 |
| `menu_type` | 菜单类型 | 目录 1 / 菜单 2 / 按钮 3 |

接口：

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET/POST | `/dict/types` | `system:dict:list` / `system:dict:create` |
| PUT/DELETE | `/dict/types/:id` | `system:dict:update` / `system:dict:delete` |
| GET/POST | `/dict/types/:id/items` | `system:dict:list` / `system:dict:create` |
| PUT/DELETE | `/dict/items/:id` | `system:dict:update` / `system:dict:delete` |

### 1.2 使用建议

- 业务表只存 `value`，展示时前端通过字典接口映射为 `label`/`tag_color`。
- 不要用字典存业务逻辑分支；状态机、权限等稳定枚举直接写常量。
- 删除类型前确认没有页面依赖该 code。

## 2. 系统参数

### 2.1 模型与规则

```go
type Parameter struct {
    Base
    Key    string // 唯一，字母开头，可含 . _ -
    Value  string // text
    Name   string
    Public bool   // 是否允许未登录读取
    Remark string
}
```

- `Key` 规则：`^[a-zA-Z][a-zA-Z0-9._-]{1,127}$`。
- `Public=true` 的参数会出现在 `GET /parameters/public`，适合站点名称、Logo 等品牌配置；**不要把密钥、开关策略设为公开**。
- `GetByKey` 对非公开参数返回 404，避免通过键名探测内部配置。

内置参数：

| Key | 默认值 | 公开 | 说明 |
| --- | --- | --- | --- |
| `site.name` | `x-admin` | 是 | 站点名称 |
| `site.logo` | 空 | 是 | 站点 Logo URL |
| `upload.max_size` | `20` | 否 | 上传大小上限（MB，提示用；实际上限仍以配置为准） |

接口：

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET | `/parameters/public` | 公开 |
| GET/POST | `/parameters` | `system:param:list` / `system:param:create` |
| PUT/DELETE | `/parameters/:id` | `system:param:update` / `system:param:delete` |

### 2.2 前端读取

登录页通过 `useSiteConfig()` 拉取公开参数，渲染站点名称/Logo；后台页面可直接读 `useUi` 或再请求公开参数。新增品牌参数时，前端只需要按 key 取值，无需改后端。

## 3. 站内通知

### 3.1 模型

```go
type Notification struct {
    Base
    UserID  uint64     // 收件人
    Title   string     // ≤128
    Content string     // ≤500
    Type    string     // info | success | warning | error
    Link    string     // 只允许站内相对路径
    ActorID uint64     // 触发者，0 表示系统
    ReadAt  *time.Time // 为空表示未读
}
```

规则（`NotificationService.Notify`）：

- `UserID=0` 直接忽略。
- 标题为空返回参数错误；标题/正文按字符截断，避免切断中文。
- 未知 `type` 回退为 `info`。
- `Link` 必须以 `/` 开头，否则置空，防止通知变成任意外链跳转。
- 通知是业务附属动作，发送失败**不影响**主流程（调用方使用 `notify` 辅助函数吞错）。

### 3.2 哪些动作会发通知

| 触发点 | 收件人 | 类型 |
| --- | --- | --- |
| 创建用户 | 新用户 | `success` 欢迎 |
| 登录成功 | 本人 | `info` 新登录提醒 |
| 连续密码错误锁定 | 本人 | `error` 安全提醒 |
| 管理员启停账号 | 被操作用户 | `success` / `warning` |
| 角色变更 | 被操作用户 | `info` |
| 管理员重置密码 | 被操作用户 | `warning` |
| 开启/关闭 TOTP | 本人 | `success` / `warning` |
| 异步导出完成 | 提交人 | `success` / `error` |

通知写入是同步的（单条 insert），由业务 service 在事务外调用；高并发场景可改为队列，接口不变。

### 3.3 查询与已读

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/notifications?unread=1&page=&page_size=` | 当前用户通知分页 |
| GET | `/notifications/unread-count` | 未读数（前端每 60s 轮询） |
| PUT | `/notifications/:id/read` | 标记单条，非本人消息返回 404 |
| PUT | `/notifications/read-all` | 全部已读，返回影响条数 |

通知按 `user_id` 隔离，管理员没有「查看所有人通知」的接口；需要审计请查操作日志。

### 3.4 清理

定时任务 `clean_notifications` 按 `{"days":90}` 物理删除过期通知。见 [定时任务](jobs.md)。

## 4. 看板

`GET /dashboard/summary`（`dashboard:view`）一次返回：

```json
{
  "users": 128,
  "active_users": 120,
  "roles": 6,
  "menus": 54,
  "files": 302,
  "file_size": 104857600,
  "jobs": 4,
  "login_trend": [{ "date": "2026-11-04", "count": 12 }],
  "recent_operations": [{ "username": "admin", "path": "/admin/v1/users", "status": 200 }]
}
```

- `login_trend` 固定返回最近 7 天，缺失日期补 0。
- `recent_operations` 返回最近 6 条操作日志。
- 数据来自多个仓储的顺序查询，任一失败整体返回错误；看板不缓存，高并发时可加只读缓存。
- 前端 `Dashboard` 页面还展示系统信号、快捷入口等，均由该聚合接口驱动。

## 5. 接口与权限小结

| 模块 | 列表/读 | 写 |
| --- | --- | --- |
| 字典 | `system:dict:list` | `system:dict:create/update/delete` |
| 参数 | `system:param:list` | `system:param:create/update/delete` |
| 通知 | 登录即可 | 登录即可（仅本人） |
| 看板 | `dashboard:view` | — |

## 6. 扩展建议

- 新增业务枚举：优先用字典，避免前端硬编码中文。
- 新增运行期开关：用参数，但涉及安全/容量的配置仍应放 `configs/config.yaml`（需要重启，可审计）。
- 新增通知场景：在 service 里调用 `notify`，保持「通知失败不影响主流程」。
- 新增看板指标：在 `DashboardSummary` 加字段，前端按需渲染；注意不要引入慢查询。
