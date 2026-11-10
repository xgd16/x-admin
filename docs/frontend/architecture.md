# 前端架构、路由与状态

前端是 React 19 + TypeScript + Vite 8 的 SPA，UI 基于 antd 6，状态用 Zustand，数据请求用 TanStack Query。

## 1. 启动链路

```text
main.tsx
  ├── QueryClientProvider     retry=1 / staleTime=15s / 关闭窗口聚焦重取
  ├── ThemeProvider           antd ConfigProvider + 主题/色板/材质/水印
  ├── BrowserRouter
  └── AppRoutes
        ├── /login            Login（公开）
        └── RequireAuth → AppLayout
              └── 页面路由 → KeepAliveOutlet
```

`RequireAuth` 首次挂载时调用 `useAuth.bootstrap()`：

- 没有 access token → `anon` → 跳转 `/login`。
- 有 token → 请求 `/auth/me`，成功进入 `ready`，失败清空令牌回登录页。
- 直接输入 URL 也会按 `permissionForPath` 校验页面权限，无权限显示 403 占位。

## 2. 路由表

| 路径 | 页面 | 权限码 |
| --- | --- | --- |
| `/login` | `Login` | 公开 |
| `/` | `HomeRedirect` | 跳到第一个可访问菜单 |
| `/dashboard` | `Dashboard` | `dashboard:view` |
| `/system/users` | `Users` | `system:user:list` |
| `/system/roles` | `Roles` | `system:role:list` |
| `/system/menus` | `Menus` | `system:menu:list` |
| `/system/departments` | `Departments` | `system:dept:list` |
| `/system/dicts` | `Dicts` | `system:dict:list` |
| `/system/parameters` | `Parameters` | `system:param:list` |
| `/system/files` | `Files` | `system:file:list` |
| `/system/jobs` | `Jobs` | `system:job:list` |
| `/system/online` | `Online` | `system:online:list` |
| `/system/monitor` | `Monitor` | `system:monitor:view` |
| `/system/logs/operations` | `Logs(mode=operations)` | `system:log:operation` |
| `/system/logs/logins` | `Logs(mode=logins)` | `system:log:login` |
| `/system/exports` | `Exports` | `system:export:list` |
| `/profile` | `Profile` | 登录即可 |
| `*` | `NotFound` | — |

**前端路由是静态注册的**：`App.tsx` 决定「路径 → 组件」，`config/nav.ts` 的 `navItems` 决定「路径 → 名称/图标/权限/分组」。后端菜单只决定展示哪些、顺序和显示名称，不会动态创建路由。

## 3. 导航生成

`useNav()` 把 `/auth/me` 的菜单树转换成导航：

1. `type=1` 目录 → 一级分组（图标轨 + 上下文栏）。
2. `type=2` 且 `visible=1`、`status=1`、有 `path` → 导航项。
3. 只保留 `navItems` 里静态注册过的路径（`navMeta`），避免点进去 404。
4. 再按 `can(permission)` 过滤；`super_admin` 直接通过。
5. 结果为空（`/auth/me` 还没回来或菜单异常）时，回退到静态 `navItems`，保证可用。

因此：

- 改菜单名称/图标/排序 → 后端改完刷新即可。
- 新增页面 → `App.tsx` + `navItems` + 后端菜单三处都要有。

## 4. 布局骨架

| 组件 | 作用 |
| --- | --- |
| `AppLayout` | 氛围光、图标轨、上下文栏、命令栏、页签栏、内容滚动区 |
| `Ambient` | 极光背景与漂浮光斑（纯色材质下关闭） |
| `Rail` | 一级模块图标轨，选中态用 motion 滑动 |
| `ContextBar` | 当前模块的二级导航；单页模块自动收起 |
| `CommandBar` | ⌘K 命令入口、主题切换、通知中心、用户菜单 |
| `TabBar` | 多标签页；右键刷新/关闭其他/关闭左侧/右侧/全部 |
| `KeepAliveOutlet` | 页签保活：隐藏而非卸载，保留筛选/表单/滚动位置 |
| `LockScreen` | 闲置锁屏，密码或 Passkey 解锁 |
| `NotificationCenter` | 未读数角标、全部/未读、单条/全部已读 |

### 4.1 页签与保活

- 打开页面时自动创建页签，`key` 是 pathname。
- `KeepAliveOutlet` 用 `Map<path, {node, context}>` 缓存页面，关闭页签时释放，避免内存持续增长。
- 每个页签有独立 `refreshKey`，右键「刷新」只重挂载当前页。
- 隐藏页通过冻结的 `UNSAFE_LocationContext` 保留激活时的路由快照，不会读到后来切换的地址。
- 登录页与 404 不参与页签体系。

### 4.2 滚动静止与材质

- 向下滚动时页签栏自动收起，回滚/回到顶部展开。
- 内容滚到命令栏下方才开启实时磨砂，减少无意义的背景采样。
- 液态玻璃只在 Chromium 系浏览器挂载折射脚本，其他浏览器自动降级。

### 4.3 锁屏

- `useIdleLock` 按 `ui.lockIdleMinutes` 计时（0 关闭），监听 `pointermove/pointerdown/keydown/wheel/scroll`。
- 合成事件（`isTrusted=false`）不重置计时，避免程序化滚动永久续命。
- 多标签页锁定状态通过 `bindLockAcrossTabs` 联动。
- 解锁只恢复界面，不改变登录态；换人登录会重置锁屏状态。

## 5. 状态管理

| Store | 持久化 | 内容 |
| --- | --- | --- |
| `useAuth` | `localStorage` `x-admin.auth` | `user / permissions / menus / status`，登录、MFA、Passkey、登出、`can()` |
| `useTabs` | `sessionStorage` `x-admin.tabs` | 页签列表、当前页签、刷新计数 |
| `useUi` | `localStorage` `x-admin.ui` | 明暗主题、色板、材质、表格密度、水印、列设置、锁屏时长、二级导航偏好 |
| `useLock` | `localStorage` `x-admin.lock` | 锁屏状态 |

Token 单独存 `localStorage`：

```text
x-admin.access
x-admin.refresh
```

> 这是前后端分离 SPA 的常见方案；若对 XSS 有更高要求，可改为 HttpOnly Cookie + CSRF。

`useAuth` 只持久化展示所需字段（`user/permissions/menus`），令牌由 `tokenStore` 管理；刷新页面时 `bootstrap()` 用令牌换最新用户信息。

## 6. 权限协作

```tsx
const can = useAuth((state) => state.can)

// 按钮
{can('system:user:create') && <Button>新建用户</Button>}

// 路由守卫
<RequireAuth><AppLayout /></RequireAuth>
```

- `can()` 只影响 UI；后端 `Require` 才是最终授权。
- 页面级权限来自 `permissionForPath`，与后端菜单无关；即使菜单被隐藏，直接输 URL 也会被拦。
- `super_admin` 不参与权限码判断，前后端都是直接放行。

## 7. 请求与错误

- 所有请求走 `api/client.ts`，自动拼接 `VITE_API_BASE ?? '/admin/v1'`、注入 Bearer、401 单飞刷新。
- 查询用 TanStack Query，`queryKey` 约定 `['resource', page, pageSize, filters]`。
- 写操作后用 `queryClient.invalidateQueries({ queryKey: ['resource'] })` 刷新。
- 错误统一 `toast.fail(error)`，展示后端 `message`。
- 详见 [API 客户端与状态](api-client.md)。

## 8. 新增页面

1. `pages/Announcements.tsx`：列表 + 弹窗/抽屉，参考 `Parameters.tsx`。
2. `App.tsx` 加路由。
3. `config/nav.ts` 加 `navItems`（路径、权限码、图标、分组）。
4. 后端菜单/按钮权限入库（见 [权限体系](../backend/rbac.md)）。
5. 按需接入 `DataTable` / `FormModal` / `FileSelect`。
6. 加组件测试。

各页面现有能力见 [页面与功能地图](pages.md)。

## 9. 移动端

- `useIsCompactViewport()` 判定：`max-width: 900px`，或触屏设备 `≤1024px`。
- `DataTable` 自动切换为 `MobileRecordList`（首列作标题，其余列转标签值，操作列固定底部）。
- 手机端筛选器默认收起，通过工具栏展开；输入框保持 16px 防 iOS 缩放。
- 抽屉/弹窗宽度不超过视口，过高时内部滚动。
- 多页签改为「当前页 + 数量」入口，底部弹出全部页签。
- 通知中心改为底部面板。

## 10. 常见问题

| 现象 | 排查 |
| --- | --- |
| 新页面菜单不出现 | `navItems` 没加、后端菜单路径不一致、角色未授权、`visible/status` 为 0 |
| 刷新后 404 | Nginx 没配 `try_files ... /index.html` |
| 登录后回到登录页 | `/auth/me` 失败（令牌过期/后端不可用），看 Network |
| 页签切换后筛选丢失 | 页面没在 `KeepAliveOutlet` 下，或用了 `key` 强制重挂载 |
| 手机布局不生效 | 断点判断用了 `window.innerWidth` 而非 `useIsCompactViewport` |
| 主题切换后部分颜色不变 | 写死了十六进制，没使用 `var(--accent)` 等 CSS 变量 |
