# 文件、存储与分享

文件模块分为三层：`sys_files` 元数据、`storage.Store` 物理存储、`/uploads` 访问鉴权。业务表只存 URL，不直接持有文件内容。

## 1. 存储布局与限制

```text
data/uploads/
  2026/
    11/
      8f3c2a1b9d4e5f60718293a4b5c6d7e8.png
```

- 文件名随机（16 字节 hex），扩展名清洗后保留；原始文件名只存在 `sys_files.name`。
- 路径由 `storage.Local.resolve` 做穿越校验，拒绝 `..` 等非法 key。
- 单文件大小：`storage.max_upload_mb`（默认 20MB）；HTTP 请求体硬上限 64MB（在 handler 里用 `MaxBytesReader`）。
- 上传失败或落库失败会删除已写入的物理文件，避免产生孤儿文件。
- 多副本部署必须使用共享卷，或替换 `storage.Store` 为对象存储（见第 7 节）。

## 2. 上传

`POST /files`（`system:file:upload`），multipart 字段 `file`：

```bash
curl -X POST http://127.0.0.1:8080/admin/v1/files \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@demo.png"
```

响应是文件元数据：

```json
{
  "id": 12,
  "name": "demo.png",
  "key": "2026/11/8f3c….png",
  "url": "/uploads/2026/11/8f3c….png",
  "mime": "image/png",
  "ext": "png",
  "size": 24567,
  "uploader_id": 1,
  "created_at": "2026-11-10T10:00:00+08:00"
}
```

`key` 是存储层相对路径，不参与前端展示；业务表存 `url` 即可。

## 3. 列表与筛选

`GET /files`（`system:file:list`）：

| 参数 | 说明 |
| --- | --- |
| `keyword` | 文件名模糊搜索 |
| `mime` | MIME 前缀，如 `image/` |
| `kind` | 大类，可逗号组合：`image`、`video`、`audio`、`document`、`archive`、`other` |
| `page` / `page_size` | 分页 |

前端 `FileSelect` / `FilePicker` 的 `types` 参数就是这组大类；两者共享同一份 `utils/file.ts` 判断逻辑。

## 4. 下载与访问鉴权

### 4.1 API 下载

`GET /files/:id/download`（`system:file:download`）以流式返回，并同时设置 ASCII 与 UTF-8 文件名：

```text
Content-Disposition: attachment; filename="demo.png"; filename*=UTF-8''demo.png
```

### 4.2 `/uploads` 静态访问

`/uploads` **不是公开目录**，`middleware.UploadAuth` 支持三种凭证：

1. **浏览器 Cookie**：登录/刷新/`/auth/me` 下发的 `HttpOnly` 文件访问 Cookie，路径限定 `/uploads`。`<img>` 自动携带。
2. **Bearer**：API 客户端直接带 `Authorization: Bearer <access>`。
3. **分享令牌**：`/uploads/...?share=<token>`，无需登录，撤销后立即失效；带 `share` 时不会回退到登录态。

鉴权失败时返回 401，并设置 `Cache-Control: no-store`；响应头由 `UploadSecurityHeaders` 统一加 `CSP sandbox`、`X-Content-Type-Options: nosniff`，防止上传内容被当作主动页面执行。

## 5. 可撤销分享

`POST /files/:id/shares`（`system:file:download`）：

```json
{ "expires": 3600 }
```

- `expires` 是秒数，默认 1 小时，最小 1 分钟、最大 7 天。
- 服务端生成 32 字节随机令牌，**数据库只存 SHA-256 摘要**，明文只在创建响应里返回一次：

```json
{
  "id": 5,
  "url": "/uploads/2026/11/8f3c….png?share=…",
  "expires_at": "2026-11-10T11:00:00+08:00"
}
```

- `GET /files/:id/shares` 返回历史记录（不含明文），`DELETE /files/:id/shares/:shareID` 立即撤销。
- 创建分享时会顺手清理 30 天前已过期/已撤销的记录，避免表膨胀。
- 分享令牌与文件路径必须匹配，否则返回 401。
- 删除文件时同步删除该文件的全部分享记录。

## 6. 删除

`DELETE /files/:id`（`system:file:delete`）：

1. 先删 `sys_files` 元数据。
2. 再删物理文件；物理文件已不存在视为成功。
3. 清理该文件的分享记录。

由于元数据删除在前，即使磁盘删除失败也不会留下「可下载但无记录」的文件；反过来会产生少量孤儿文件，建议用定时任务/运维脚本定期清理 `data/uploads` 中不在 `sys_files.key` 里的文件。

## 7. 替换为对象存储

实现 `internal/pkg/storage.Store`：

```go
type Store interface {
    Save(ctx context.Context, filename string, r io.Reader) (Info, error)
    Open(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    URL(key string) string
}
```

然后在 `internal/bootstrap/providers.go` 的 `provideStorage` 返回你的实现（S3/OSS/MinIO 等）。约定：

- `Key` 用对象 key，不要带 bucket。
- `URL` 返回可直接访问的地址；如果对象存储需要签名，建议仍走 `/uploads` 代理或在 `URL()` 里生成短期签名。
- `Save` 必须返回实际写入的 `Size`，`FileService` 依赖它判断超限。
- 对象存储更适合多副本；本地存储只适合单机/共享卷。

## 8. 接口一览

| 方法 | 路径 | 权限 |
| --- | --- | --- |
| GET | `/files` | `system:file:list` |
| POST | `/files` | `system:file:upload` |
| GET | `/files/:id/download` | `system:file:download` |
| GET/POST | `/files/:id/shares` | `system:file:download` |
| DELETE | `/files/:id/shares/:shareID` | `system:file:download` |
| DELETE | `/files/:id` | `system:file:delete` |

## 9. 前端组件

```tsx
// 表单字段：value/onChange 是 URL 或 URL[]
<Form.Item name="avatar" label="头像">
  <FileSelect types="image" />
</Form.Item>

// 多选，最多 8 个图片/视频
<FileSelect types={['image', 'video']} multiple limit={8} />

// 只要选择弹窗，拿完整 FileItem[]
<FilePicker
  open={open}
  types="image"
  multiple={false}
  onCancel={() => setOpen(false)}
  onConfirm={(files) => save(files[0])}
/>
```

- `FileSelect` 只回传 URL，保持 Form 值可序列化。
- `FilePicker` 内置上传入口（受 `system:file:upload` 权限控制），上传后立即选中。
- 选择弹窗支持搜索、分页、跨页保留已选、按类型过滤。

## 10. 安全清单

- [ ] 生产使用 HTTPS，文件 Cookie 才会带 `Secure`
- [ ] `storage.root` 不在 Web 服务器可直接列目录的位置
- [ ] 反向代理也限制 `/uploads` 的请求体与超时
- [ ] 定期清理孤儿文件与过期分享记录
- [ ] 不要把 `sys_files.key` 直接暴露给前端
- [ ] 上传目录不要挂载为可执行脚本目录
