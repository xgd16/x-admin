# 样式组织

样式入口为 `index.scss`，由 `src/main.tsx` 引入。原先单文件 `global.css` 已按“基础层 → 布局 → 通用组件 → 页面 → 响应式”拆分为多个 partial。

## 目录结构

```text
styles/
├── index.scss                  # 入口，维护全局加载顺序
├── abstracts/                  # 仅编译期使用的 SCSS 变量与 mixin
│   ├── _variables.scss         # 断点、z-index
│   └── _mixins.scss            # below() / truncate / reduced-motion
├── base/
│   ├── _tokens.scss            # 浅色/深色 CSS 变量
│   ├── _reset.scss             # 基础元素重置
│   └── _ambient.scss           # 极光背景与漂浮光斑
├── layout/
│   ├── _shell.scss             # 应用外框、canvas、滚动容器
│   ├── _rail.scss              # 左侧图标栏
│   ├── _context-bar.scss       # 二级上下文导航
│   ├── _command-bar.scss       # 顶部命令栏
│   ├── _notifications.scss     # 通知中心
│   └── _tabs.scss              # 页面标签栏与右键菜单
├── components/
│   ├── _glass.scss             # 玻璃拟态表面
│   ├── _icon-button.scss       # 圆形图标按钮
│   ├── _page-header.scss       # 页面标题区
│   ├── _buttons.scss           # 胶囊按钮
│   ├── _panels.scss            # 通用面板
│   ├── _table.scss             # 数据表格、筛选与工具栏
│   ├── _mobile-list.scss       # 手机卡片列表（DataTable / MobileRecordList）
│   ├── _antd.scss              # Ant Design 主题微调
│   ├── _toast.scss             # Toast / Notification 玻璃表面与移动端宽度
│   ├── _loading.scss           # 启动加载、环形指示器与骨架屏
│   ├── _utilities.scss         # 小工具类与空状态
│   ├── _settings.scss          # 命令面板、设置菜单
│   ├── _file-icon.scss         # 文件图标与类型色
│   ├── _file-picker.scss       # 文件选择弹窗
│   └── _file-select.scss       # 行内文件选择
├── pages/
│   ├── _dashboard.scss         # 仪表盘：KPI、图表、信号、快捷入口
│   ├── _users.scss             # 用户页：列表 + 检查器
│   ├── _menus.scss             # 菜单页：可拖拽树形列表
│   ├── _login.scss             # 登录页
│   └── profile/
│       ├── _profile.scss       # 个人资料卡片
│       ├── _totp.scss          # 双因素认证
│       └── _passkey.scss       # Passkey 管理
├── overrides/
│   └── _canvas-glass.scss      # 依赖加载顺序的最终覆盖
└── responsive/
    └── _index.scss             # 按断点分组的响应式规则
```

## 约定

1. **按业务归属放置样式**：页面专属样式放 `pages/`，跨页面复用放 `components/`，整体框架放 `layout/`。
2. **在 `index.scss` 中登记并保持顺序**：部分选择器优先级相同，后加载的规则会覆盖先加载的规则。新增 partial 时请放在对应层级附近，不要随意打乱顺序。
3. **只在 `abstracts/` 写编译期变量/mixin**，不要在 partial 中重复定义 `$breakpoints`。
4. **响应式统一用 mixin**：

   ```scss
   @use '../abstracts/mixins' as *;

   .my-block {
     @include below('lg') {
       display: none;
     }
   }
   ```

   可用断点：`xl` 1200、`lg` 1024、`md` 900、`sm` 820、`xs` 700、`xxs` 640。

5. **单行截断统一用 `@include truncate`**，不要重复写 `overflow / text-overflow / white-space` 三件套。
6. **暗色主题**继续使用 `:root[data-theme='dark'] .selector` 选择器，并与对应组件放在同一个 partial 中，便于一起维护。
7. **设计变量优先用 CSS 变量**（`var(--ink)`、`var(--line)` 等），它们会随主题切换；SCSS 变量只用于编译期计算与断点。
8. **加载态**统一复用 `src/components/Loading.tsx`（antd Spin / Skeleton），样式集中在 `_loading.scss`；React 挂载前的启动占位写在 `index.html` 内联样式中，调整品牌色时与 `_loading.scss` 同步。
9. **操作反馈**统一使用 `useToast()`（`src/hooks/useToast.ts`），内部直接调用 antd `message` / `notification`：
   - 普通成功、失败、警告用 `toast.success / warning / error`，异常用 `toast.fail(error, fallback)`；
   - 下载、导入等长请求用 `toast.promise(task, { loading, success, error })`；
   - 多行结果或需要展开说明的信息用 `toast.notify()`，不要再自建 toast 容器；
   - 位置、时长、最大条数与悬停暂停由 `ThemeProvider` 的 `AntdApp` 统一配置，页面不要单独设置 `top` 或 `duration`。

## 移动端约定

- 紧凑视口的 JS 判定统一用 `useIsCompactViewport()`（`src/hooks/useMediaQuery.ts`），不要把 `window.innerWidth` 散落在页面里。
- 表格与密集数据优先复用 `DataTable` / `MobileRecordList`：桌面渲染 antd Table，手机把同一份列定义渲染成卡片；不要在页面里再写一套移动端列表。
- 手机卡片样式集中在 `components/_mobile-list.scss`，页面只负责提供列定义与操作。
- 手机输入框保持 16px，避免 iOS 聚焦缩放；抽屉/弹窗宽度不超过视口，内容过高时在内部滚动。
- 手机端隐藏滚动条但保留滚动能力，并把 `scrollbar-gutter` 改为 `auto`，不要为滚动条预留宽度。
- 手机端页签栏使用“当前页 + 数量入口 + 底部切换面板”，不要在窄屏继续横向铺开多个页签胶囊。
- 通知中心在手机上使用底部面板（`NotificationCenter` 内按紧凑视口切换），不要用固定在铃铛右侧的 320px 弹层，否则窄屏会被推到视口外。
- 触屏点击目标建议不小于 36–40px；筛选器在手机端默认收起，通过表格工具栏展开。
