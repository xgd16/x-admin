# 权限体系（RBAC）

x-admin 的权限模型是「用户 → 角色 → 菜单/按钮」。菜单既是导航数据，也是权限码的载体；超级管理员角色 `super_admin` 是唯一硬编码的旁路。

## 1. 数据模型

```text
sys_users ──< sys_user_roles >── sys_roles ──< sys_role_menus >── sys_menus
    │                              │                              │
    └ dept_id ──> sys_departments  └ code / status                ├ type: 1 目录 / 2 菜单 / 3 按钮
                                                                  ├ path / component / icon
                                                                  └ permission: system:user:list
```

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `sys_users` | `username` 唯一、`status`、`dept_id`、`session_version` | 账号与登录态 |
| `sys_roles` | `code` 唯一、`status` | `super_admin` 内置 |
| `sys_menus` | `parent_id`、`type`、`path`、`permission`、`visible`、`status` | 目录/菜单/按钮统一树 |
| `sys_user_roles` | 联合主键 | 用户与角色多对多 |
| `sys_role_menus` | 联合主键 | 角色与菜单多对多 |
| `sys_departments` | `parent_id`、`sort`、`status` | 部门树 |

## 2. 权限码

格式：`模块:资源:动作`，由 `ValidatePermission` 校验：

```text
^[a-z][a-z0-9_]*(:[a-z][a-z0-9_]*){1,3}$
```

例：

```text
dashboard:view
system:user:list
system:user:create
system:role:assign
system:export:submit
```

规则：

- 只有 `type=3` 的按钮节点需要权限码；目录可以不填。
- `type=2` 的菜单必须有 `path`（前端路由），权限码可选（个人中心就是登录即可访问）。
- 角色被授予某个菜单节点后，该节点的 `permission` 进入用户权限集合。
- `super_admin` 跳过权限集合判断，直接放行所有 `Require`。
- 权限码同时用于：后端 `middleware.Require`、前端 `can()`、菜单管理里的按钮节点。三处必须一致。

完整清单见 [权限码清单](../reference/permissions.md)。

## 3. 登录后如何得到权限

`GET /auth/me` 返回：

```json
{
  "user": { "id": 1, "username": "admin", "roles": [...] },
  "permissions": ["dashboard:view", "system:user:list", "..."],
  "menus": [{ "id": 1, "name": "总览", "type": 2, "path": "/dashboard", "children": [] }]
}
```

- `permissions` 来自启用角色关联的启用菜单，去重并排序。
- `menus` 是导航用的树，已排除按钮节点；`visible=0` 的节点仍会返回（例如个人中心），由前端决定是否展示。
- 前端 `config/nav.ts` 只认静态注册过的路径，后端菜单负责决定「显示哪些、叫什么、什么顺序」。
- 角色/菜单改动后，下一次 `/auth/me` 或下一次请求就会生效；不需要清缓存或重启。

## 4. 越权保护（重点）

这些规则都在 service 层，不能只靠前端：

### 4.1 授予角色

- 只有超级管理员可以授予 `super_admin` 角色。
- 非超级管理员只能分配「权限不超过自己」的角色：他会把自己的全部权限码与目标角色的菜单权限逐一比对，发现目标角色有自己没有的权限码就返回 `40301`。
- 内置 `admin` 账号必须保留 `super_admin` 角色；不能停用、不能删除。
- 不能停用/删除当前登录账号。

### 4.2 授予菜单

- 非超级管理员只能把自己已经拥有的菜单权限授予角色。
- 空权限节点（目录、个人中心等）允许授权。
- `super_admin` 角色的编码不可修改、不可停用、不可删除，只有超级管理员能修改它。

### 4.3 菜单与部门树

- 菜单/部门不能把上级设为自己或自己的子级（防环）。
- 删除菜单前必须没有子节点。
- 删除部门前必须没有子部门、没有下属用户。
- 菜单删除会同时清理 `sys_role_menus` 关联。

## 5. 用户管理

### 5.1 创建

```json
{
  "username": "zhangsan",
  "password": "abc12345",
  "nickname": "张三",
  "email": "zhangsan@example.com",
  "phone": "13800000000",
  "avatar": "/uploads/…",
  "dept_id": 2,
  "status": 1,
  "role_ids": [2],
  "remark": ""
}
```

校验：

| 字段 | 规则 |
| --- | --- |
| `username` | 3–32 位字母、数字、下划线，唯一 |
| `password` | 至少 `security.password_min_len` 位，必须同时含字母和数字 |
| `email` | 标准邮箱格式，可为空 |
| `phone` | `0-9 + - 空格`，6–20 位，可为空 |
| `dept_id` | 必须存在 |
| `role_ids` | 角色必须存在，且满足越权保护 |

创建成功会发欢迎通知。角色绑定失败会回滚刚创建的用户，避免半成品账号。

### 5.2 更新

`UpdateUserInput` 的 `status` 和 `role_ids` 都是指针：

- `role_ids` 不传 → 不修改角色。
- `role_ids: []` → 清空角色。
- 状态或角色真正变化时才发通知，避免保存资料时刷「角色已更新」。
- 状态变化会递增 `session_version`，旧会话立即失效。
- 使用 `UpdateFields` 只更新可编辑字段，不会覆盖并发修改的密码/TOTP/session_version。

### 5.3 删除

`DELETE /users` 请求体 `{"ids":[1,2]}`，批量删除。逐个体检：

- 内置 `admin` 不可删。
- 当前登录账号不可删。
- 删除采用软删除；用户名会被改写成 `#deleted#<id>` 墓碑值，允许之后重建同名用户（见 [数据库与迁移](database.md)）。

### 5.4 导入

`POST /users/import`（`system:user:import`），multipart 字段 `file`，硬上限 10MB，最多 500 行。

模板列顺序：

```text
用户名 | 昵称 | 邮箱 | 手机号 | 部门ID | 部门名称 | 密码 | 状态(1正常/0停用)
```

- 部门名称列不参与导入，实际使用部门 ID。
- 单行失败不会中断整体导入，响应返回成功条数与失败明细：

```json
{
  "success": 3,
  "failed": [{ "row": 5, "username": "lisi", "message": "用户名已存在" }]
}
```

- `row` 是 Excel 实际行号（表头为第 1 行）。

用户导出统一走 [异步导出](../export.md)，模块名 `users`，参数与列表筛选一致。

## 6. 角色管理

```json
{
  "name": "运营",
  "code": "operator",
  "status": 1,
  "sort": 2,
  "remark": "",
  "menu_ids": [1, 2, 10, 20]
}
```

- `code`：小写字母开头，2–32 位小写字母/数字/下划线，唯一。
- 创建时 `menu_ids` 可同时授权；更新时 `menu_ids` 为 `null` 表示不动授权，空数组表示清空。
- 删除前检查是否仍有用户绑定；有则返回 `40901`。
- `super_admin` 的编码、状态、删除都受保护。

分配菜单接口 `PUT /roles/:id/menus` 是**整体替换**：

```json
{ "menu_ids": [1, 2, 10, 11, 20] }
```

## 7. 菜单管理

`GET /menus/tree` 返回完整树（含按钮），节点字段：

| 字段 | 说明 |
| --- | --- |
| `parent_id` | 0 为根 |
| `name` | 显示名称 |
| `type` | 1 目录 / 2 菜单 / 3 按钮 |
| `path` | 前端路由，菜单必填 |
| `component` | 预留的组件标识（当前前端用静态路由表） |
| `permission` | 按钮必填，菜单可选 |
| `icon` | `Icon` 组件名称，如 `users`、`shield` |
| `sort` | 同级排序 |
| `visible` | 是否出现在导航 |
| `status` | 0 停用 / 1 启用，停用后不参与权限计算 |

新增一个页面时，典型做法是：目录节点 + 菜单节点 + 若干按钮节点。只加菜单不加按钮，页面能进但按钮会全部隐藏（`can()` 返回 false）。

## 8. 部门管理

- 树结构，`parent_id=0` 为根。
- 字段：名称、负责人、电话、邮箱、排序、状态。
- 更新时禁止挂到自己的子级。
- 删除前检查子部门与用户。
- 部门只用于组织和用户筛选，不直接参与权限判断。

## 9. 接口一览

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET | `/users` | `system:user:list` |
| GET | `/users/:id` | `system:user:list` |
| POST | `/users` | `system:user:create` |
| PUT | `/users/:id` | `system:user:update` |
| DELETE | `/users` | `system:user:delete` |
| PUT | `/users/:id/status` | `system:user:update` |
| PUT | `/users/:id/password` | `system:user:reset` |
| PUT | `/users/:id/roles` | `system:user:update` |
| POST/GET | `/users/import`、`/users/template` | `system:user:import` |
| GET | `/roles`、`/roles/all`、`/roles/:id/menus` | `system:role:list` |
| POST/PUT/DELETE | `/roles`、`/roles/:id` | `system:role:create/update/delete` |
| PUT | `/roles/:id/menus` | `system:role:assign` |
| GET | `/menus/tree` | `system:menu:list` |
| POST/PUT/DELETE | `/menus`、`/menus/:id` | `system:menu:create/update/delete` |
| GET | `/departments/tree` | `system:dept:list` |
| POST/PUT/DELETE | `/departments`、`/departments/:id` | `system:dept:create/update/delete` |

用户列表筛选：`keyword`（用户名/昵称/邮箱/手机号）、`dept_id`、`status`、`role_id`、`page`、`page_size`。

## 10. 前端配合

```tsx
const can = useAuth((state) => state.can)

{can('system:user:create') && <Button>新建用户</Button>}
```

- `can()` 对 `super_admin` 一律返回 true。
- 路由守卫 `RequireAuth` 会按 `config/nav.ts` 的 `permissionForPath` 再拦一次，直接输入 URL 也会得到 403 页面。
- 导航由 `/auth/me` 的菜单树生成；菜单名称、图标、排序改后端即可生效，无需重新构建前端。

## 11. 常见问题

| 现象 | 排查 |
| --- | --- |
| 新按钮不显示 | 菜单里是否建了按钮节点？角色是否授权？前端 `can()` 权限码是否一致？ |
| 接口 403 但菜单可见 | 菜单 `permission` 与路由 `Require` 不一致，或角色未授权该按钮节点 |
| 改了角色权限没生效 | 确认角色/菜单 `status=1`；权限在每次请求时重新加载，正常无需重登 |
| 删除角色失败 | 仍有用户绑定该角色，先改派角色 |
| 删除部门失败 | 仍有子部门或下属用户 |
| 无法授予超级管理员 | 只有 `super_admin` 账号可以授予 `super_admin` 角色 |
