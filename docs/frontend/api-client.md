# API 客户端、状态与数据请求

前端不直接 `fetch`，所有 HTTP 都通过 `frontend/src/api/client.ts`，以保证令牌注入、401 刷新、错误封装和文件上传下载行为一致。

## 1. 客户端结构

```text
api/
  client.ts    fetch 封装：BASE、Bearer、401 单飞刷新、ApiError、上传/下载
  tokens.ts    localStorage 令牌读写（x-admin.access / x-admin.refresh）
  public.ts    useSiteConfig：公开参数（站点名/Logo）
  types.ts     后端 DTO 的 TypeScript 类型
```

```ts
const BASE = import.meta.env.VITE_API_BASE ?? '/admin/v1'
```

开发环境由 Vite 把 `/admin/v1` 代理到 `http://127.0.0.1:8080`，生产由 Nginx 反代。

## 2. 请求方法

```ts
api.get<T>(path, params?)
api.post<T>(path, body?)
api.put<T>(path, body?)
api.del<T>(path, body?)
api.upload<T>(path, file)       // multipart，字段名 file
api.download(path, params?)     // 返回 Blob
saveBlob(blob, filename)        // 触发浏览器下载
```

特性：

- 自动拼接 `BASE`；`queryString` 忽略 `undefined / null / ''`。
- 自动注入 `Authorization: Bearer <access>`。
- JSON body 自动 `JSON.stringify`；`FormData` 不设 `Content-Type`，交给浏览器带 boundary。
- 统一解析 `{code,message,data}`；HTTP 非 2xx 或 `code!==0` 抛 `ApiError`。
- `ApiError` 带 `code` 与 `message`，页面直接 `toast.fail(error)`。

示例：

```ts
const query = useQuery({
  queryKey: ['users', page, pageSize, keyword],
  queryFn: () => api.get<PageResult<User>>('/users', { page, page_size: pageSize, keyword }),
})

const save = useMutation({
  mutationFn: (values: UserInput) =>
    editing ? api.put<User>(`/users/${editing.id}`, values) : api.post<User>('/users', values),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
})
```

## 3. 401 单飞刷新

```text
请求 A 401 ─┐
请求 B 401 ─┼─> 只发起一次 /auth/refresh ─> 成功：用新 token 重放 A/B
请求 C 401 ─┘                           └> 失败：清 token，跳 /login
```

- `refreshing` 是模块级 Promise，并发 401 复用同一次刷新，避免刷新风暴。
- 认证接口本身不参与自动刷新（`noRefreshPaths`），防止 401 循环：
  `/auth/login`、`/auth/login/mfa`、`/auth/passkey/login/*`、`/auth/refresh`。
- 刷新失败会 `tokenStore.clear()` 并 `window.location.assign('/login')`，同时抛出 `ApiError(40102)`。

后端 refresh token 是**一次性轮换**：刷新成功旧 token 立即作废。不要在多个标签页手动共享/重放旧 refresh token。

## 4. 令牌存储

```ts
tokenStore.access   // 内存 + localStorage 'x-admin.access'
tokenStore.refresh  // 内存 + localStorage 'x-admin.refresh'
tokenStore.set(access, refresh)
tokenStore.clear()
```

- 键名保持稳定，升级后不会强制退出。
- 令牌与用户信息分开：`useAuth` 的 persist 只存 `user/permissions/menus`。
- 页面刷新后 `RequireAuth.bootstrap()` 用 access token 请求 `/auth/me` 恢复会话；失败则回登录页。

## 5. 认证 Store

```ts
const login = useAuth((s) => s.login)
await login(username, password, captchaId, code, rememberMe)
```

- `login` 若返回 `mfa_required`，不落地令牌，调用方继续 `verifyMFA(mfaToken, code, rememberMe)`。
- `passkeyLogin(rememberMe)` 内部完成 begin → 浏览器签名 → finish。
- 成功后 `establish()` 保存令牌并请求 `/auth/me`，写入 `user/permissions/menus`。
- `logout()` 调后端注销、清令牌、重置锁屏；失败也继续前端清理。
- `can(permission)`：空权限视为公开；`super_admin` 恒 true。

## 6. TanStack Query 约定

全局配置（`main.tsx`）：

```ts
queries: {
  retry: 1,
  refetchOnWindowFocus: false,
  staleTime: 15_000,
}
```

约定：

- `queryKey` 用数组：`['resource', ...参数]`，参数变化自动重新请求。
- 列表筛选：`['users', page, pageSize, keyword, deptId, status, roleId]`。
- 写操作成功用 `invalidateQueries({ queryKey: ['resource'] })`，不要手动改缓存，除非有明确的乐观更新需求。
- 通知未读数每 60 秒轮询一次；导出任务在「进行中」时才轮询。
- 页面卸载后不要保留定时器；轮询放在 `useQuery` 的 `refetchInterval` 里，离开页面自动停止。

## 7. 公开参数

```ts
const { name, logo } = useSiteConfig()
```

- 请求 `GET /parameters/public`，失败回退 `x-admin`。
- `staleTime` 5 分钟，登录页与后台布局共用。
- 同时把 `document.title` 设置为站点名称。
- 只读取 `public=true` 的参数；不要在这里放敏感配置。

## 8. 上传与下载

### 8.1 上传

```ts
const file = await api.upload<FileItem>('/files', selectedFile)
```

- 字段名固定 `file`。
- 后端硬上限 64MB，业务上限 `storage.max_upload_mb`。
- `FilePicker` 已经封装了上传 + 选中流程，业务页面一般不需要直接调。

### 8.2 下载

```ts
const blob = await api.download(`/files/${id}/download`)
saveBlob(blob, 'report.xlsx')
```

- 错误响应仍按 JSON 解析，抛出 `ApiError`。
- `saveBlob` 创建临时 Object URL，点击后立即释放。
- 导出任务下载用 `/export-tasks/:id/download`，同样走 `api.download`。

## 9. 错误处理

```ts
try {
  await save.mutateAsync(values)
} catch (error) {
  toast.fail(error) // 读取 ApiError.message
}
```

- 业务错误优先展示后端 `message`；网络错误展示通用文案。
- `useToast().fail` 会自动从 `Error`/对象/字符串中提取消息。
- 401 由客户端统一处理，页面通常不需要单独判断。
- 403 页面由 `RequireAuth` 兜底，按钮级 403 由 `toast.fail` 展示。

## 10. 类型约定

- `api/types.ts` 与后端 DTO 保持一致；字段名用 snake_case（后端直出）。
- 分页统一 `PageResult<T>`。
- 时间字段是 RFC3339 字符串，展示时用 dayjs 格式化。
- 可空字段用 `| null`，不要用 `?` 混用。
- 新增接口时先加类型，再写页面，避免 `any`。

## 11. 常见问题

| 现象 | 原因 |
| --- | --- |
| 请求 404 | `VITE_API_BASE` 配错，或 Vite/Nginx 代理未生效 |
| 刷新后立刻跳登录 | access 过期且 refresh 也失效；检查 `/auth/refresh` 响应 |
| 多标签页同时退出 | refresh token 一次性轮换，另一个标签页用了旧 token；属预期，重新登录即可 |
| 上传 413 | Nginx `client_max_body_size` 或后端 64MB 上限 |
| 下载文件名乱码 | 后端同时设置 ASCII 与 UTF-8 `Content-Disposition`，不要用 `encodeURI` 二次处理 |
| `useQuery` 一直 loading | `queryFn` 抛错且 `retry` 掩盖，检查 Network 与 `ApiError` |
