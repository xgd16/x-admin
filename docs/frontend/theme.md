# 主题、样式与响应式

视觉系统由三部分组成：`ThemeProvider`（antd token + html 属性）、`styles/base/_tokens.scss`（CSS 变量）、`config/themes.ts` + `config/appearance.ts`（可选项配置）。改主题时三处要一起考虑。

## 1. 主题模式

- `light` / `dark`：写入 `<html data-theme="dark">`，CSS 用 `:root[data-theme='dark']` 切换。
- antd 通过 `theme.darkAlgorithm` / `defaultAlgorithm` 同步组件默认样式。
- 主题色板（palette）与明暗独立，默认珊瑚橙。
- 材质（surface）三档：`solid`（默认）/ `glass` / `liquid`，写入 `<html data-surface>`。

```text
<html data-theme="dark" data-palette="ocean" data-surface="glass">
```

`ThemeProvider` 还负责：

- 把 `--accent` 等色板值同步给 antd `colorPrimary` 与图表。
- 更新 `<meta name="theme-color">`。
- 水印：`Watermark` 内容为「x-admin · 昵称 + 日期」。
- 液态玻璃：只在支持 CSS backdrop refraction 的 Chromium 挂载折射脚本，其他浏览器降级。
- 减少动态效果：`MotionConfig reducedMotion="user"`。

## 2. 色板

定义在 `frontend/src/config/themes.ts`：

| id | 名称 | accent |
| --- | --- | --- |
| `coral` | 珊瑚橙 | `#ff5f33`（默认） |
| `ocean` | 海洋蓝 | `#2f7fe0` |
| `forest` | 森野绿 | `#1f9d6b` |
| `violet` | 暮光紫 | `#7c5ce6` |
| `rose` | 玫瑰粉 | `#e0508f` |
| `graphite` | 曜石灰 | `#4a6076` |

每个色板包含 `accent / accent2 / accent3 / ink`，图表可覆盖 `chartFrom / chartArea`。

CSS 变量在 `styles/base/_tokens.scss` 按 `:root[data-palette='…']` 定义：

```css
--accent: #ff5f33;
--accent-2: #ff8a5c;
--accent-3: #ff4d8d;
--accent-soft: #ffb38f;
--accent-ink: #8f2a0d;
```

### 新增色板

1. 在 `themes.ts` 的 `PALETTES` 加一项。
2. 在 `_tokens.scss` 加 `:root[data-palette='<id>']` 块，补齐 `--accent*`。
3. 如需出现在偏好设置，`ThemePalettePicker` 会遍历 `PALETTES`，无需额外改。
4. 检查图表、状态标签、KPI 光晕在浅色/深色下的对比度。

**规则**：品牌色一律用 `var(--accent)` / `var(--accent-2)` / `var(--accent-3)`，半透明用 `color-mix(in srgb, var(--accent) N%, transparent)`，不要写死十六进制。语义色（成功/警告/错误）和文件类型色不受色板影响。

## 3. 材质

`config/appearance.ts`：

| id | 名称 | 说明 |
| --- | --- | --- |
| `solid` | 纯色 | 默认，关闭背景模糊与动画，适合低性能设备 |
| `glass` | 毛玻璃 | 半透明磨砂 + 极光背景 + 高光 |
| `liquid` | 液态玻璃 | 玻璃边缘折射 + 镜面高光，Chromium 完整支持 |

- 纯色规则集中在 `styles/overrides/_solid.scss`。
- 液态玻璃规则在 `styles/components/_liquid-glass.scss`，同时保留 `data-liquid-glass` 兼容开关。
- 新增玻璃表面时必须确认三档都有合理降级：纯色可读、毛玻璃可读、液态不遮挡内容。

## 4. 样式组织

入口 `styles/index.scss`，按「基础 → 布局 → 组件 → 页面 → 响应式」加载：

```text
styles/
├── index.scss
├── abstracts/      _variables.scss（断点/层级）、_mixins.scss（below/truncate）
├── base/           _tokens.scss、_reset.scss、_ambient.scss
├── layout/         shell / rail / context-bar / command-bar / tabs / notifications
├── components/     glass / liquid-glass / table / mobile-list / toast / file-* …
├── pages/          dashboard / users / menus / login / monitor / profile/*
├── overrides/      canvas-glass / solid
└── responsive/     断点规则
```

约定（详见 `frontend/src/styles/README.md`）：

- 页面专属样式放 `pages/`，跨页面复用放 `components/`，框架放 `layout/`。
- 新增 partial 要在 `index.scss` 对应位置登记，保持加载顺序。
- 编译期变量只放 `abstracts/`，不要在 partial 重复定义。
- 单行截断用 `@include truncate`。
- 暗色规则与对应组件放在同一 partial，用 `:root[data-theme='dark'] .selector`。
- 优先 CSS 变量（`var(--ink)`、`var(--line)`），SCSS 变量只用于断点/计算。
- 操作反馈统一 `useToast()`，加载态统一 `Loading` 系列。

## 5. 响应式

断点（`abstracts/_variables.scss`）：

| 键 | 宽度 |
| --- | --- |
| `xl` | 1200 |
| `lg` | 1024 |
| `md` | 900 |
| `sm` | 820 |
| `xs` | 700 |
| `xxs` | 640 |

```scss
@use '../abstracts/mixins' as *;

.my-block {
  @include below('md') { display: none; }
}
```

JS 侧统一用 `useIsCompactViewport()`：

```text
(max-width: 900px), (pointer: coarse) and (max-width: 1024px)
```

不要在页面里散落 `window.innerWidth`。手机端约定：

- 表格 → 卡片列表（`DataTable` 自动切换）。
- 筛选器默认收起，工具栏滑杆展开。
- 输入框 16px，避免 iOS 聚焦缩放。
- 抽屉/弹窗不超过视口，过高内部滚动。
- 页签改为「当前页 + 数量」底部面板。
- 通知中心改底部面板。
- 滚动条隐藏但保留滚动，`scrollbar-gutter: auto`。

## 6. 动效

- 统一用 `motion/react`；`MotionConfig reducedMotion="user"` 遵循系统设置。
- 位移/缩放动画只用于状态切换，不做无限循环装饰（极光背景除外）。
- `prefers-reduced-motion: reduce` 时关闭位移，保留透明度变化。
- 表格刷新用轻量遮罩/骨架，不要整页闪白。

## 7. 图表

- ECharts 封装在 `charts/echarts.ts` / `charts/theme.ts`。
- 颜色从当前色板取（`paletteOf`），不要硬编码。
- 趋势图 `LoginTrendChart` 支持悬停吸附与 tooltip；大数据量注意降采样。
- 图表容器需要固定高度，避免 ResizeObserver 抖动。

## 8. 水印

`useUi.watermark` 控制，`ThemeProvider` 用 antd `Watermark` 包裹应用：

```text
x-admin · 昵称 2026-11-10
```

水印是前端展示层，不防截图/篡改；敏感页面仍需后端权限控制。

## 9. 主题排错

| 现象 | 原因 |
| --- | --- |
| 切换色板后按钮变色但图表没变 | 图表用了写死颜色，没取 `paletteOf` |
| 暗色下文字看不清 | 只改了亮色 token，没写 `:root[data-theme='dark']` 覆盖 |
| 液态玻璃在 Safari 失效 | 预期降级为普通玻璃；折射仅 Chromium |
| 页面闪烁 | 主题属性在 React 挂载后才写入；可在 `index.html` 内联脚本提前设置 |
| 移动端横向滚动 | 固定宽度/大表格未用 `DataTable` 的卡片模式 |
| 样式改了不生效 | partial 没在 `index.scss` 登记，或被后面的 override 覆盖 |
