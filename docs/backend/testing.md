# 测试与质量

测试分两层：Go 的单元/HTTP 集成测试，以及前端的 Vitest 组件测试。CI 会同时跑后端与前端。

## 1. 命令

```bash
# 后端
make test                 # go test ./...
make coverage             # 生成 coverage.out 并打印总覆盖率
make coverage-html        # 生成 coverage.html
go test ./tests/integration -run TestUsers -v
go test -race ./tests/unit/service/...

# 前端
cd frontend
npm test                  # vitest run
npm run test:watch
npm run test:coverage
npm run lint              # oxlint
npm run build             # tsc -b && vite build
```

CI 对应 `.github/workflows/ci.yml`：

- Backend：`go vet` → `go test ./tests/... -coverpkg=./internal/...` → 上传覆盖率。
- Frontend：`npm ci` → `npm run lint` → `npm test` → `npm run build`。

## 2. 目录

```text
tests/
  testsupport/app.go         完整应用测试夹具（SQLite + 内存缓存 + 路由）
  unit/
    pkg/                     基础库单测
    service/                 业务规则单测
    repository/              仓储单测（软删除、错误分支）
    handler/ middleware/     HTTP 层单测
  integration/               走真实路由 + 鉴权的接口测试
frontend/src/**/*.test.tsx   组件与 store 测试
```

单元测试和集成测试都大量复用 `testsupport`，区别是：unit 直接调用 service/repository，integration 通过 `app.Do` 发 HTTP 请求。

## 3. Go 测试夹具

```go
app := testsupport.New(t)
```

它做了：

- 用当前测试名创建独立的内存 SQLite 数据库，`SetMaxOpenConns(1)`。
- `database.AutoMigrate` 建全部表（含新模型，记得同步）。
- 内存缓存、临时目录存储、低成本 bcrypt（cost=4）。
- 组装全部 repo、service、handler、router，与生产的装配方式一致。
- 启动导出 worker，注册 users / logs 导出模块。
- `t.Cleanup` 关闭连接、日志队列、导出服务。

常用辅助：

```go
admin := app.SeedAdmin(t)                         // admin / Admin@123456 + super_admin
pair := app.Login(t, admin.Username, "Admin@123456") // 真实验证码 + 登录流程
role := app.EnsureRole(t, "operator", "运营")
user := app.CreateUser(t, "alice", "abc12345", 1, "operator")
rr := app.Do(http.MethodGet, "/admin/v1/users", nil, pair.AccessToken)
env := testsupport.Decode(t, rr)                  // {code,message,data}
```

`Login` 会先请求 `/auth/captcha`，再从缓存读取答案，模拟真实登录；因此它同时覆盖验证码、锁定、令牌签发。

## 4. 写一个 service 单测

```go
func TestRoleValidation(t *testing.T) {
    ctx := context.Background()
    app := testsupport.New(t)

    _, err := app.Services.Roles.Create(ctx, service.RoleInput{
        Name: "", Code: "operator", Status: 1,
    })
    require.Error(t, err)
    assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
}
```

建议覆盖：

- 正常路径（创建/更新/删除/查询）。
- 参数校验（空值、格式、状态）。
- 唯一冲突（用户名/角色编码/参数键）。
- 404 与软删除后的行为。
- 越权（非 super 授予 super、分配高权限角色）。
- 安全变更后的 session_version。

## 5. 写一个 HTTP 集成测试

```go
func TestAnnouncementHTTP(t *testing.T) {
    app := testsupport.New(t)
    admin := app.SeedAdmin(t)
    pair := app.Login(t, admin.Username, "Admin@123456")

    rr := app.Do(http.MethodPost, "/admin/v1/announcements", map[string]any{
        "title": "维护通知", "content": "今晚升级", "status": 1,
    }, pair.AccessToken)
    require.Equal(t, http.StatusCreated, rr.Code)

    env := testsupport.Decode(t, rr)
    require.Equal(t, 0, env.Code)
}
```

集成测试重点：

- `Require` 权限码是否生效（普通用户 403、super 200）。
- 统一响应信封与 HTTP 状态码。
- 分页参数边界（page=0、page_size=999）。
- 文件上传/下载、分享令牌、Cookie 鉴权。
- 会话撤销、改密、停用后旧 token 失效。
- 导出任务的提交、轮询、下载。

## 6. 前端测试

Vitest 配置在 `frontend/vite.config.ts`：

```ts
test: {
  environment: 'jsdom',
  include: ['src/**/*.test.{ts,tsx}'],
  setupFiles: ['./src/test/setup.ts'],
}
```

`src/test/setup.ts` 补齐了 jsdom 缺失的 `matchMedia`、`scrollIntoView`、`ResizeObserver` 与 `getComputedStyle`。

典型组件测试（`FileSelect.test.tsx`）：

```tsx
vi.mock('../stores/auth', () => ({
  useAuth: (selector) => selector({ can: () => false }),
}))

function renderWithQuery(ui: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}
```

建议：

- 用 `@testing-library/react` 的 `screen.getByRole` 查询，避免依赖 class。
- 不 mock `api/client` 时，用 MSW 或在测试内 `vi.mock`；现有页面测试多用 `vi.spyOn(api, 'get')`。
- antd 弹窗/通知需要包 `App` 或 `AntdApp` 上下文。
- 每个测试后 `cleanup()` + `vi.clearAllMocks()`。
- 组件里的权限判断通过 mock `useAuth` 控制，不依赖真实登录。

## 7. 覆盖率

- 后端覆盖率命令：`go test ./tests/... -coverpkg=./internal/... -coverprofile=coverage.out`。
- 覆盖率只统计被测试触达的代码；`cmd/` 与生成代码不计入。
- 不要为了数字写无断言测试；优先覆盖业务分支、错误路径、越权路径。
- 前端用 `@vitest/coverage-v8`，关注核心组件与工具函数，不追求 100%。

## 8. 测试约定

- 测试文件命名 `*_test.go` / `*.test.ts(x)`。
- 表驱动测试用 `t.Run(name, ...)`；失败信息包含输入。
- 单元测试不连真实 MySQL/Redis；需要时用 `testsupport`（SQLite + memory）。
- 时间相关逻辑通过 `WithClock` 或可替换的 `now` 注入，避免 sleep。
- 不依赖测试执行顺序；每个测试独立 app。
- 测试中的密码用低成本 bcrypt，生产配置不要跟着改。
- 新增模型后同步 `database.AutoMigrate`，否则集成测试建表失败。

## 9. 常见问题

| 现象 | 原因 |
| --- | --- |
| `no such table` | 新模型没加到 `AutoMigrate` |
| 集成测试 401 | 没走 `app.Login`，或访问的是 `/uploads` 需要 Cookie |
| 验证码错误 | 直接构造登录请求，没先取 `/auth/captcha` |
| SQLite 锁库 | 多个连接并发写；夹具已设 `MaxOpenConns(1)`，测试内不要另开连接 |
| 前端 `matchMedia is not a function` | 没加载 `src/test/setup.ts` |
| antd 报 `App.useApp` 上下文缺失 | 组件外层没包 `AntdApp` / `App` |
| 测试互相污染 | 共用了同一个 `testsupport.New(t)` 实例或全局 store |
