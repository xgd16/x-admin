# 权限码清单

权限码格式 `模块:资源:动作`，来自 `sys_menus.permission`。后端 `middleware.Require`、前端 `can()`、菜单按钮节点三处必须一致。

> 下表的菜单 ID 来自种子数据 `migrations/000001_init.up.sql`。以数据库实际数据为准；新增模块时不要复用这些 ID。

## 1. 解析规则

```text
用户 → 启用角色 → 角色授权菜单（status=1）→ sys_menus.permission
```

- `super_admin` 跳过权限集合，所有 `Require` 直接放行。
- 目录节点通常没有权限码，只用于分组；按钮节点必须填权限码。
- 菜单节点（type=2）可以带权限码（如 `system:user:list`），进入页面需要它。
- 权限集合在每次请求时重新加载，改角色/菜单无需重启。

## 2. 工作台与账号

| 权限码 | 菜单 | ID | 说明 |
| --- | --- | --- | --- |
| `dashboard:view` | 总览 | 1 | 看板聚合接口 |
| —（登录即可） | 个人中心 | 3 | 个人资料、密码、会话、TOTP、Passkey |
| —（登录即可） | 站内通知 | — | `/notifications*` |

## 3. 用户

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:user:list` | 用户管理 | 10 | `GET /users`、`GET /users/:id` |
| `system:user:create` | 新建用户 | 20 | `POST /users` |
| `system:user:update` | 编辑用户 | 21 | `PUT /users/:id`、状态、角色分配 |
| `system:user:delete` | 删除用户 | 22 | `DELETE /users` |
| `system:user:reset` | 重置密码 | 23 | `PUT /users/:id/password` |
| `system:user:import` | 导入用户 | 24 | `POST /users/import`、`GET /users/template` |
| `system:user:export` | 导出用户 | 25 | 用户页「导出」按钮的显示权限；实际提交任务还要求 `system:export:submit` |

## 4. 角色

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:role:list` | 角色管理 | 11 | `GET /roles`、`/roles/all`、`/roles/:id/menus` |
| `system:role:create` | 新建角色 | 30 | `POST /roles` |
| `system:role:update` | 编辑角色 | 31 | `PUT /roles/:id` |
| `system:role:delete` | 删除角色 | 32 | `DELETE /roles/:id` |
| `system:role:assign` | 分配权限 | 33 | `PUT /roles/:id/menus` |

## 5. 菜单

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:menu:list` | 菜单管理 | 12 | `GET /menus/tree` |
| `system:menu:create` | 新建菜单 | 40 | `POST /menus` |
| `system:menu:update` | 编辑菜单 | 41 | `PUT /menus/:id` |
| `system:menu:delete` | 删除菜单 | 42 | `DELETE /menus/:id` |

## 6. 部门

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:dept:list` | 部门管理 | 13 | `GET /departments/tree` |
| `system:dept:create` | 新建部门 | 50 | `POST /departments` |
| `system:dept:update` | 编辑部门 | 51 | `PUT /departments/:id` |
| `system:dept:delete` | 删除部门 | 52 | `DELETE /departments/:id` |

## 7. 字典

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:dict:list` | 字典管理 | 14 | 类型/字典项查询 |
| `system:dict:create` | 新建字典 | 60 | 创建类型/字典项 |
| `system:dict:update` | 编辑字典 | 61 | 更新类型/字典项 |
| `system:dict:delete` | 删除字典 | 62 | 删除类型/字典项 |

## 8. 参数

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:param:list` | 参数配置 | 15 | `GET /parameters` |
| `system:param:create` | 新建参数 | 70 | `POST /parameters` |
| `system:param:update` | 编辑参数 | 71 | `PUT /parameters/:id` |
| `system:param:delete` | 删除参数 | 72 | `DELETE /parameters/:id` |

## 9. 文件

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:file:list` | 文件管理 | 16 | `GET /files` |
| `system:file:upload` | 上传文件 | 80 | `POST /files` |
| `system:file:delete` | 删除文件 | 81 | `DELETE /files/:id` |
| `system:file:download` | 下载文件 | 82 | 下载、分享的查看/创建/撤销 |

## 10. 定时任务

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:job:list` | 定时任务 | 17 | `GET /jobs`、`/jobs/:id/runs` |
| `system:job:create` | 新建任务 | 90 | `POST /jobs` |
| `system:job:update` | 编辑任务 | 91 | `PUT /jobs/:id`、启停 |
| `system:job:delete` | 删除任务 | 92 | `DELETE /jobs/:id` |
| `system:job:run` | 执行任务 | 93 | `POST /jobs/:id/run` |

## 11. 审计

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:log:operation` | 操作日志 | 18 | 查询/删除/清空操作日志 |
| `system:log:login` | 登录日志 | 19 | 查询/清空登录日志 |

## 12. 在线用户与监控

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:online:list` | 在线用户 | 210 | `GET /online-sessions` |
| `system:online:kick` | 强制下线 | 212 | `DELETE /online-sessions/:userID` |
| `system:monitor:view` | 系统监控 | 211 | `GET /monitor/overview` |

## 13. 异步导出

| 权限码 | 菜单 | ID | 对应接口 |
| --- | --- | --- | --- |
| `system:export:list` | 导出记录 | 200 | `GET /export-tasks`、查看文件名模板 |
| `system:export:submit` | 提交导出 | 201 | `POST /export-tasks` |
| `system:export:cancel` | 取消导出 | 202 | `POST /export-tasks/:id/cancel` |
| `system:export:delete` | 删除记录 | 203 | `DELETE /export-tasks/:id` |
| `system:export:download` | 下载导出 | 204 | `GET /export-tasks/:id/download` |
| `system:export:config` | 文件名配置 | 205 | `PUT /export-tasks/filename-template` |

> 前端「导出」按钮的显示权限与接口权限是分开的：用户页按钮用 `system:user:export`，日志页按钮用对应的日志查看权限；而提交任务接口统一要求 `system:export:submit`，查看任务列表要 `system:export:list`，下载要 `system:export:download`。给角色授权时请把这些一起给，否则会出现「能点导出但提交 403」。

## 14. 种子角色

| 角色 | code | ID | 授权 |
| --- | --- | --- | --- |
| 超级管理员 | `super_admin` | 1 | 全部菜单（含后续新增） |
| 运营 | `operator` | 2 | 总览、系统/工作台/审计目录、操作日志、登录日志 |

`super_admin` 的菜单可见性与权限判断都有代码旁路：`AuthService.load` 会加载全部启用菜单，`Principal.Has` 直接放行，因此新增菜单无需给该角色补授权。其他角色（如 `operator`）需要在角色管理里勾选新菜单，或写 `sys_role_menus`。

## 15. 新增权限的检查表

- [ ] 迁移里新增 `sys_menus` 按钮节点，`permission` 符合 `模块:资源:动作`
- [ ] 后端路由加 `middleware.Require("...")`
- [ ] 前端按钮用 `can("...")` 包裹
- [ ] 给需要的角色授权（角色管理或 `sys_role_menus`）
- [ ] 前端 `config/nav.ts` 的页面权限码与菜单一致
- [ ] 测试覆盖有权限/无权限两条路径
- [ ] 更新本文与 `docs/reference/api.md`

## 16. 常见误区

| 误区 | 事实 |
| --- | --- |
| 隐藏按钮就等于没权限 | 不是，必须后端 `Require` |
| `super_admin` 是一个权限码 | 不是，是角色编码，代码里硬编码旁路 |
| 目录节点也要权限码 | 不需要；目录只分组，权限码在菜单/按钮上 |
| 改了菜单名称就改了权限 | 不会；权限看 `permission` 字段，不是 `name` |
| 权限改了要重新登录 | 不需要；每次请求重新加载 |
| 给角色授权就等于用户有权限 | 用户还要绑定该角色，且角色/菜单都是启用状态 |
