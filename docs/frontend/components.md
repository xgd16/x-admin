# 通用组件与交互约定

业务页面尽量复用 `frontend/src/components`，不要重复实现表格、弹窗、文件选择与提示。这一页列出手头可用的组件和推荐用法。

## 1. 组件总览

| 组件 | 作用 | 典型场景 |
| --- | --- | --- |
| `PageHeader` | 页面标题 + 副标题 + 右侧操作 | 每个页面顶部 |
| `DataTable` | 列表表格：列设置、密度、刷新、分页、移动端卡片 | 所有列表页 |
| `MobileRecordList` | 手机卡片列表 | 由 `DataTable` 自动调用 |
| `FormModal` | 弹窗 + antd Form | 新增/编辑 |
| `FilePicker` | 文件库选择弹窗 | 选文件、上传 |
| `FileSelect` | 表单内文件字段 | 头像、附件、图片 |
| `StatusTag` / `MethodTag` | 状态/方法标签 | 用户状态、日志方法 |
| `Icon` | 统一图标 | 导航、按钮 |
| `Loading` 系列 | 页面/表格/骨架屏 | 加载态 |
| `RequireAuth` | 登录/权限路由守卫 | 路由层 |
| `useToast` | 统一 message/notification | 所有操作反馈 |

布局组件（`components/layout`）：`AppLayout`、`Rail`、`ContextBar`、`CommandBar`、`TabBar`、`KeepAliveOutlet`、`LockScreen`、`NotificationCenter`。

## 2. DataTable

```tsx
<DataTable<User>
  storageKey="users"
  rowKey="id"
  data={query.data?.list ?? []}
  loading={query.isFetching}
  total={query.data?.total ?? 0}
  page={page}
  pageSize={pageSize}
  onPageChange={(p, size) => { setPage(p); setPageSize(size) }}
  onRefresh={() => void query.refetch()}
  columns={columns}
  filters={<Input placeholder="搜索用户名" />}
  toolbar={<Button type="primary">新建用户</Button>}
  selection={rowSelection}
  scroll={{ x: 'max-content' }}
  expandable={{ ... }}
/>
```

Props：

| Prop | 说明 |
| --- | --- |
| `columns` | antd `TableColumnsType`，同一份定义同时驱动桌面/手机 |
| `rowKey` | 字符串或函数 |
| `storageKey` | 列显隐偏好的持久化 key；同一页面保持稳定 |
| `loading` | 首屏无数据时显示骨架，已有数据时只显示轻量遮罩 |
| `total/page/pageSize/onPageChange` | 服务端分页 |
| `toolbar` / `filters` | 左侧业务按钮 / 筛选区；手机端筛选默认收起 |
| `selection/expandable/onRow` | 透传 antd Table |
| `mobile` | 覆盖手机端主标题列、隐藏列、操作列 |

行为：

- 列设置、密度、刷新按钮固定在工具栏右侧。
- 列偏好写入 `useUi.columns[storageKey]`，刷新后保留；列被删除时自动清理脏 key。
- 紧凑视口下自动换成 `MobileRecordList`：首列作标题，其余列转标签/值，`fixed: 'right'` 或 `key: 'actions'` 固定底部。
- 空数据文案用 `emptyText`，首屏加载用骨架屏。

**约定**：页面不要自己判断 `window.innerWidth`，交给 `DataTable`。

## 3. MobileRecordList

一般不需要直接用，`DataTable` 会调用。需要自定义手机列表时：

```tsx
<MobileRecordList
  columns={columns}
  data={rows}
  rowKey="id"
  loading={loading}
  pagination={{ current, pageSize, total, onChange }}
  mobile={{ titleKey: 'name', hiddenKeys: ['id'], actionKeys: ['actions'] }}
/>
```

- 支持选择、树形展开、加载态、空状态、分页。
- 操作列固定在卡片底部；宽列自动换行。
- 与桌面表格共享 `utils/table.ts` 的列 key/渲染逻辑。

## 4. FormModal

```tsx
const [form] = Form.useForm<FormValues>()

<FormModal
  open={open}
  title={editing ? '编辑参数' : '新建参数'}
  form={form}
  onCancel={() => setOpen(false)}
  onSubmit={(values) => save.mutate(values)}
  confirmLoading={save.isPending}
>
  <Form.Item name="name" label="名称" rules={[{ required: true }]}>
    <Input />
  </Form.Item>
</FormModal>
```

- 确定按钮触发 `form.submit()`，再由 `onSubmit` 提交。
- `destroyOnHidden` 会卸载表单，避免残留状态；打开前用 `form.setFieldsValue` 回填。
- 默认宽度 520，复杂表单传 `width`。

## 5. PageHeader

```tsx
<PageHeader
  title="参数配置"
  sub={`${total} 个参数 · 公共参数可被未登录页面读取`}
  actions={can('system:param:create') && <Button>新建参数</Button>}
/>
```

统一标题、副标题与右侧操作区，不要页面自己写 `h1`。

## 6. 文件选择

### 6.1 FileSelect（表单字段）

```tsx
<Form.Item name="avatar" label="头像">
  <FileSelect types="image" />
</Form.Item>

<Form.Item name="attachments" label="附件">
  <FileSelect types={['document', 'archive']} multiple limit={5} />
</Form.Item>
```

Props：

| Prop | 说明 |
| --- | --- |
| `value/onChange` | URL 或 URL[]，可直接接 Form.Item |
| `types` | `image/video/audio/document/archive/other`，可单值或数组 |
| `multiple` | 是否多选 |
| `limit` | 多选上限 |
| `disabled` | 只读 |
| `placeholder/title` | 文案 |
| `upload` | 是否允许在弹窗内上传（默认受权限控制） |

只回传 URL，保持表单值可序列化；已有 URL 会被包装成文件卡片展示，可单独移除。

### 6.2 FilePicker（弹窗）

```tsx
<FilePicker
  open={open}
  types="image"
  multiple={false}
  value={selected}
  onCancel={() => setOpen(false)}
  onConfirm={(files) => { save(files[0]); setOpen(false) }}
/>
```

- 支持类型过滤、搜索、分页、跨页保留已选、清除单项。
- 内置上传入口（受 `system:file:upload` 权限控制），上传后立即选中。
- `upload={false}` 可强制隐藏上传按钮。

## 7. 状态与反馈

### 7.1 StatusTag / MethodTag

```tsx
<StatusTag status={user.status} on="正常" off="停用" />
<MethodTag method={log.method} />
```

### 7.2 useToast

```tsx
const toast = useToast()

toast.success('保存成功')
toast.fail(error)                       // 自动提取错误消息
await toast.promise(task, {
  loading: '正在导出…',
  success: '导出完成',
  error: '导出失败',
})
toast.notify({ type: 'success', title: '导入完成', description: '成功 3 条，失败 1 条' })
```

- 成功/失败/警告用 `success/warning/error`；长请求用 `promise`；多行结果用 `notify`。
- 位置、时长、最大条数由 `ThemeProvider` 的 `AntdApp` 统一配置，页面不要单独设置 `top/duration`。
- 错误优先展示后端 `message`，不要自己拼「操作失败」。

## 8. 布局与导航组件

| 组件 | 说明 |
| --- | --- |
| `Rail` | 一级分组图标轨，选中态动画 |
| `ContextBar` | 二级导航；单页模块自动隐藏 |
| `CommandBar` | ⌘K 命令面板、主题切换、通知、用户菜单 |
| `TabBar` | 多页签，右键菜单支持刷新/关闭其他/左/右/全部 |
| `KeepAliveOutlet` | 页签保活，关闭页签释放页面 |
| `NotificationCenter` | 未读角标、全部/未读、单条/全部已读 |
| `LockScreen` | 闲置锁屏，密码/Passkey 解锁 |
| `Ambient` | 极光背景（纯色材质关闭） |

这些组件依赖 `AppLayout` 的上下文，页面不要单独渲染。

## 9. 图标

```tsx
<Icon name="users" size={15} />
```

- `name` 是内部图标表的 key（`users`、`shield`、`tree`、`folder`、`clock`、`activity`…）。
- 菜单的 `icon` 字段与 `Icon` 的 key 对应；新增图标在 `components/Icon.tsx` 注册。
- 品牌标记用 `BrandMark`，不要用普通 Icon 代替。

## 10. 组件编写约定

- 组件只关心展示与交互，数据请求放页面或 hook。
- Props 用显式 interface，避免 `any`；受控组件同时支持 `value/onChange`。
- 样式用 class + 现有 partial，不写行内大段样式。
- 加载态复用 `Loading` 系列；空状态复用 antd `Empty` 或 `DataTable.emptyText`。
- 无障碍：按钮有可读名称，图标按钮加 `aria-label`，弹窗标题明确。
- 动效遵循 `prefers-reduced-motion`，统一用 motion 的 `MotionConfig reducedMotion="user"`。
- 组件测试放在同目录 `*.test.tsx`，用 `@testing-library/react`。

## 11. 常见问题

| 现象 | 原因 |
| --- | --- |
| 手机端表格错位 | 列定义用了自定义 `render` 但没给 `key/dataIndex`，`tableColumnKey` 无法稳定识别 |
| 列设置刷新后丢失 | `storageKey` 变了，或列 key 不稳定 |
| `FileSelect` 表单值为空 | 没有接 `Form.Item` 的 `value/onChange`，或用了非受控 `defaultValue` |
| Toast 被导航遮挡 | 页面自己设置了 `top`，应使用全局 `useToast` |
| 弹窗关闭后表单还有值 | 没设 `destroyOnHidden` 或没 `form.resetFields()` |
| 移动端筛选看不到 | 筛选器默认收起，点工具栏滑杆按钮展开 |
