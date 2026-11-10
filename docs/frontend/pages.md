# 前端页面与功能地图

这一页按页面列出「能做什么、调用哪些接口、用了哪些组件」，方便接手时快速定位。路由与权限的机制见 [前端架构](architecture.md)。

## 1. 登录页 `/login`

- 账号密码 + 图形验证码；支持「记住我」。
- 开启 TOTP 后切换到动态码/恢复码输入。
- Passkey 免密码登录。
- 读取公开参数展示站点名称/Logo。
- 登录成功按 `state.from` 回到原页面，否则进第一个可访问菜单。

接口：`GET /auth/captcha`、`POST /auth/login`、`POST /auth/login/mfa`、`POST /auth/passkey/login/*`、`GET /parameters/public`。

## 2. 工作台 `/dashboard`

- KPI：用户/启用用户/角色/菜单/文件/文件总量/任务数。
- 最近 7 天登录趋势图（懒加载 ECharts）。
- 最近操作、系统信号、快捷入口。
- 快捷入口按权限过滤。

接口：`GET /dashboard/summary`。组件：`PageHeader`、`LoginTrendChart`、`MobileRecordList`。

## 3. 用户管理 `/system/users`

- 关键字/部门/状态/角色筛选，服务端分页。
- 新建/编辑用户，右侧检查器（手机为抽屉）。
- 分配角色、启停、重置密码、删除（批量）。
- Excel 导入（逐行失败明细）、下载模板。
- 用户导出走异步任务（模块 `users`）。
- 头像使用 `FileSelect types="image"`。
- 所有操作按 `system:user:*` 控制。

接口：`/users*`、`/departments/tree`、`/roles/all`、`/export-tasks`。组件：`DataTable`、`FormModal`/`Drawer`、`FileSelect`、`StatusTag`。

## 4. 角色管理 `/system/roles`

- 角色列表、创建/编辑/删除。
- 菜单权限树勾选，整体替换 `menu_ids`。
- 内置 `super_admin` 的编码/状态/删除受后端保护。
- 非超级管理员只能授予自己拥有的权限。

接口：`/roles*`、`/menus/tree`。组件：`DataTable`、`FormModal`、antd `Tree`。

## 5. 菜单管理 `/system/menus`

- 目录/菜单/按钮三级树维护。
- 桌面端支持拖拽调整层级与顺序；手机端用表单选择上级。
- 菜单必填路径；按钮必填权限码。
- 删除前检查子节点。
- 图标与前端 `Icon` 名称对应。

接口：`/menus/tree`、`/menus`、`/menus/:id`。组件：树形列表、`FormModal`。

## 6. 部门管理 `/system/departments`

- 部门树 CRUD，负责人/电话/邮箱/排序/状态。
- 禁止把上级设为自己或子级。
- 删除前检查子部门与下属用户。

接口：`/departments/tree`、`/departments`、`/departments/:id`。

## 7. 字典管理 `/system/dicts`

- 左侧字典类型、右侧字典项的分栏维护。
- 类型编码唯一；字典项 label/value/排序/状态/标签色。
- 删除类型会级联删除字典项。

接口：`/dict/types*`、`/dict/types/:id/items`、`/dict/items/:id`。

## 8. 参数配置 `/system/parameters`

- 参数键/值/名称/备注/是否公开。
- 新建默认非公开，避免敏感配置被未登录页面读取。
- 公开参数供登录页品牌配置使用。

接口：`/parameters*`、`/parameters/public`。组件：`DataTable`、`FormModal`。

## 9. 文件管理 `/system/files`

- 上传（受 `system:file:upload` 控制）、类型/关键字筛选、分页。
- 下载、删除、图片/文件图标展示。
- 「分享」抽屉：创建限时链接、查看历史、逐条撤销。
- 文件选择器复用 `FilePicker` / `FileSelect`。

接口：`/files*`、`/files/:id/shares*`、`/export-tasks`（导出文件列表）。组件：`DataTable`、`Upload`、`Drawer`。

## 10. 定时任务 `/system/jobs`

- 任务列表、Cron 表达式、handler、JSON 参数、启停。
- 立即执行、查看执行历史（触发方式、状态、耗时、错误）。
- 启用/停用同步调度器；多副本依赖 Redis 锁。

接口：`/jobs*`、`/jobs/:id/runs`。组件：`DataTable`、`Drawer`、`FormModal`。

## 11. 在线用户 `/system/online`

- 管理员视角的全部登录设备，按最近活动排序。
- 关键字（用户名/昵称/IP/设备）、按用户过滤。
- 强制下线单设备或整用户。
- 当前设备有标记。

接口：`GET /online-sessions`、`DELETE /online-sessions/:userID`。

## 12. 系统监控 `/system/monitor`

- 应用信息、主机 CPU/内存/磁盘/网络、Go runtime、DB 连接池、缓存状态。
- 12 列网格：指标卡 + 趋势主视觉 + 依赖状态 + 明细。
- 手动刷新，不轮询。

接口：`GET /monitor/overview`。

## 13. 审计日志 `/system/logs/operations|logins`

- 操作日志：方法/路径/操作人/IP/状态/耗时/请求体/错误/request_id。
- 登录日志：账号/IP/UA/状态/说明。
- 关键字、状态、时间范围筛选；详情抽屉。
- 操作日志支持勾选删除与清空；登录日志支持清空。
- 导出走异步任务（`operation_logs` / `login_logs`）。

接口：`/logs/operations*`、`/logs/logins*`、`/export-tasks`。组件：`DataTable`、`Drawer`。

## 14. 导出记录 `/system/exports`

- 当前用户的导出任务列表，按状态筛选。
- 有排队/生成中任务时每 2 秒轮询，全部结束停止。
- 取消、下载、删除；文件名模板配置（`system:export:config`）。
- 下载走 `api.download` + `saveBlob`。

接口：`/export-tasks*`、`/export-tasks/filename-template`。组件：`DataTable`、`Progress`、`FilenameTemplateDrawer`。

## 15. 个人中心 `/profile`

- 资料编辑（昵称/邮箱/手机/头像）。
- 修改密码（撤销其他设备）。
- 在线设备：当前设备、退出指定设备/其他设备/全部。
- 双重验证：二维码绑定、恢复码、关闭。
- Passkey：绑定、重命名、删除；注册需当前密码。

接口：`/profile*`、`/profile/sessions*`、`/profile/totp*`、`/profile/passkeys*`。组件：`FileSelect`、`FormModal`、`Modal`。

## 16. 404 `/404`

- 后台框架内展示，提供回工作台/上一页入口。
- 不创建页签，不参与导航。

## 17. 跨页面约定

| 约定 | 说明 |
| --- | --- |
| 权限 | 页面级由 `RequireAuth` 拦，按钮级用 `can()` |
| 列表 | 一律 `DataTable`，不要自己写桌面/手机两套 |
| 弹窗 | 新增/编辑用 `FormModal` 或 `Drawer`，提交前 `form.validateFields` |
| 反馈 | `useToast`，长请求用 `toast.promise` |
| 空状态 | `DataTable.emptyText` 或 antd `Empty` |
| 加载 | `Loading` 系列；首屏骨架、刷新遮罩 |
| 时间 | dayjs 格式化，接口 RFC3339 |
| 文件 | 表单用 `FileSelect`，选文件用 `FilePicker` |
| 移动端 | `useIsCompactViewport`，不要读 `window.innerWidth` |

## 18. 新增页面的落点

1. `pages/Xxx.tsx` 写页面。
2. `App.tsx` 注册路由。
3. `config/nav.ts` 注册 `navItems`（权限码、图标、分组）。
4. 后端菜单/按钮入库。
5. 需要导出时在 service 注册 `ExportBuilders`。
6. 加组件测试与权限路径测试。
