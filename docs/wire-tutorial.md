# wire 新手教程：加一个新功能到底要动哪几个文件

> 写给还没用过 wire 的你。看完这篇，你加任何新模块都是同一套动作，不用再碰那 100 行装配代码。

---

## 一、先搞懂 wire 是干嘛的（2 分钟）

改造前，`internal/bootstrap/app.go` 里有一大段手写的装配代码：

```go
userRepo := repository.NewUserRepo(db)
roleRepo := repository.NewRoleRepo(db)
// …… 11 行 repo
authSvc := service.NewAuthService(userRepo, menuRepo, logRepo, store, tokens, hasher, captchaGen, notifications, opts)
// …… 13 行 service
handlers := router.Handlers{ Auth: handler.NewAuthHandler(...), ... }
// …… 13 行 handler
```

它的作用只有一个：**把所有零件按依赖关系 new 出来，一个传给下一个**。

这段代码很无聊——每个零件怎么 new，构造函数签名里写得清清楚楚，凭什么要人手抄一遍？所以现在交给 wire：

- **你**只负责两件事：① 把零件造出来（写构造函数）；② 把零件名字写进「清单」（ProviderSet）。
- **wire** 读清单和构造函数签名，自动生成 `wire_gen.go`——内容就是上面那段手写代码，一个字不差。

所以：**wire = 帮你自动写 newApp 的机器人**。它不干别的。

### 一张图记住零件怎么串

```
HTTP 请求 → handler（服务员，接请求）
              ↓ 调用
           service（厨师，业务逻辑）
              ↓ 调用
           repo（仓库管理员，读写数据库）
```

加一个新功能 = 造三个新零件（repo/service/handler）+ 在三个清单上登记 + 跑一次 `make wire`。

---

## 二、跟着做一遍：新增「公告」功能

假设要加一个公告模块。下面每一步都写明：**打开哪个文件、加哪几行、加在哪**。照抄即可。

### 第 1 步：造 repo 零件

新建 `internal/repository/announcement.go`：

```go
package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
)

// AnnouncementRepo 公告数据仓储。
type AnnouncementRepo struct{ db *gorm.DB }

func NewAnnouncementRepo(db *gorm.DB) *AnnouncementRepo {
	return &AnnouncementRepo{db: db}
}

func (r *AnnouncementRepo) List(ctx context.Context, q pagination.Query) ([]model.Announcement, int64, error) {
	// 具体查询逻辑，随便你怎么写
	...
}
```

### 第 2 步：给 service 定接口（为什么？往下看）

这个项目的规矩：**service 不认识具体 repo，只认接口**（方便测试时换假实现）。接口统一放在 `internal/service/ports.go`，往里加一段：

```go
type AnnouncementRepository interface {
	List(ctx context.Context, q pagination.Query) ([]model.Announcement, int64, error)
}
```

### 第 3 步：造 service 零件

新建 `internal/service/announcement.go`：

```go
package service

// AnnouncementService 公告业务逻辑。
type AnnouncementService struct {
	announcements AnnouncementRepository // 注意：是接口类型
}

// 参数声明为接口，wire 之后会自动把 AnnouncementRepo 塞进来
func NewAnnouncementService(announcements AnnouncementRepository) *AnnouncementService {
	return &AnnouncementService{announcements: announcements}
}

func (s *AnnouncementService) List(ctx context.Context, q pagination.Query) ([]model.Announcement, int64, error) {
	return s.announcements.List(ctx, q)
}
```

### 第 4 步：造 handler 零件

新建 `internal/handler/announcement.go`：

```go
package handler

type AnnouncementHandler struct {
	announcements *service.AnnouncementService
}

func NewAnnouncementHandler(announcements *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{announcements: announcements}
}

func (h *AnnouncementHandler) List(c *gin.Context) {
	// 解析参数、调 service、返回 JSON
	...
}
```

---

**零件造完了。下面开始「登记」——把三个零件名字写进三个清单。**

### 第 5 步：登记 repo（两行，别漏）

打开 `internal/repository/wire.go`，在 `ProviderSet` 里加：

```go
var ProviderSet = wire.NewSet(
	NewUserRepo,
	...
	NewAnnouncementRepo,                                        // ← 加这行：登记构造函数
	wire.Bind(new(service.AnnouncementRepository), new(*AnnouncementRepo)),  // ← 加这行：介绍认识
)
```

**为什么要第二行？** service 只认接口 `AnnouncementRepository`，不认识 `*AnnouncementRepo` 这个具体类型。`wire.Bind` 就是给 wire 递名片：「这个 repo 可以当那个接口用」。少写它，第 7 步会报错（错误对照见第三节）。

### 第 6 步：登记 service 和 handler（各一行）

`internal/service/wire.go`：

```go
var ProviderSet = wire.NewSet(
	NewAuthService,
	...
	NewAnnouncementService,   // ← 加这行
)
```

`internal/handler/wire.go`：

```go
var ProviderSet = wire.NewSet(
	NewDashboardHandler,
	...
	NewAnnouncementHandler,   // ← 加这行
)
```

### 第 7 步：跑 wire，让它生成装配代码

```bash
make wire
```

它会重新生成 `internal/bootstrap/wire_gen.go`。点开看看，你会发现自己一行都没写的装配代码出现了：

```go
announcementRepo := repository.NewAnnouncementRepo(db)
announcementService := service.NewAnnouncementService(announcementRepo)
announcementHandler := handler.NewAnnouncementHandler(announcementService)
```

wire 怎么知道顺序？看构造函数的**参数类型**：handler 要 `*AnnouncementService` → 谁提供它？`NewAnnouncementService` → 它要 `AnnouncementRepository` → Bind 名片说了 repo 能当 → 调 `NewAnnouncementRepo(db)`。全按类型自动连线。

### 第 8 步：把 handler 挂到路由上（这步是手动的，wire 不管）

打开 `internal/router/router.go` 添加 `Handlers` 字段，然后到 `internal/router/admin.go` 的 `registerAdminRoutes` 注册路由：

```go
// ① Handlers 结构体加字段（wire 会自动填这个字段，你不用在任何地方 new 它）
type Handlers struct {
	...
	Announcement *handler.AnnouncementHandler
}

// ② 在 admin.go 合适的位置注册路由（照抄别的模块的写法）
secured.GET("/announcements", middleware.Require("system:announcement:list"), d.Handlers.Announcement.List)
```

> 为什么字段不用手动赋值？注入器里有 `wire.Struct(new(router.Handlers), "*")`——把 Handlers 的**所有字段**都用对应类型的提供者填满。这也是清单的一部分，永远不用改。

### 第 9 步：验证

```bash
make wire && go build ./... && go test ./tests/unit/bootstrap/...
```

全绿就完事了。前端、model、migration 该怎么写还怎么写，跟 wire 无关。

---

## 三、报错急救对照表

`make wire` 的报错很啰嗦但很精确，都是「缺哪个零件」的问题：

| 报错（节选） | 原因 | 对策 |
|---|---|---|
| `provider for *service.AnnouncementService not found` | service 构造器没登记 | 第 6 步的 service/wire.go |
| `provider for service.AnnouncementRepository not found` | 忘了 Bind 名片，或 repo 构造器没登记 | 第 5 步，两行都要 |
| `provider for *handler.AnnouncementHandler not found` | handler 构造器没登记 | 第 6 步的 handler/wire.go |
| `provider ... is not used` | 登记了但没人消费（比如 handler 写了却没加 Handlers 字段） | 检查第 8 步 |
| `not enough return values` 之类 | 注入器签名和生成代码不匹配 | 基本不会遇到；改了注入器签名就重跑 `make wire` |

`go build` 报的错（如 `undefined: xxx`）是普通代码错误，跟 wire 无关，正常修。

**两条纪律：**
1. `wire_gen.go` 是生成的，**永远别手改**。想改装配 → 改清单/构造函数 → 重跑 `make wire`。
2. `wire_gen.go` 要**跟着代码一起提交**，CI 不需要装 wire。

---

## 四、两种特殊情况（10% 的场景）

### 情况 A：构造函数要用 config 里的数值

比如 `NewAnnouncementService(repo, maxPerDay int)`，`maxPerDay` 来自 `cfg`。

**不要**把它登记进 `service/wire.go`（两个构造函数都要 `int` 会撞车），而是在 `internal/bootstrap/providers.go` 写个包装：

```go
func provideAnnouncementService(
	announcements service.AnnouncementRepository,
	cfg config.Config,   // 原语从 cfg 现取，不单独作为注入类型
) *service.AnnouncementService {
	return service.NewAnnouncementService(announcements, cfg.Announcement.MaxPerDay)
}
```

然后把这行加进 `internal/bootstrap/wire.go` 的 `wire.Build(...)` 列表。

**怎么一眼识别？** 看现有代码：`provideUserService`、`provideFileService`、`provideProfileService` 都是这种包装，抄最近的那个。

### 情况 B：零件要启动/关闭动作

比如像 `JobService` 那样要 `Boot()`、要 `Close()`。也是去 `providers.go`，返回值多带清理函数，抄 `provideJobService` / `provideLogService`：

```go
func provideAnnouncementService(...) (*service.AnnouncementService, func(), error) {
	svc := service.NewAnnouncementService(...)
	if err := svc.Warmup(ctx); err != nil {
		return nil, nil, err     // 出错时 wire 自动回收前面所有零件
	}
	return svc, svc.Close, nil  // 正常关闭时 App.Close 会调它
}
```

---

## 五、速记卡（贴墙版）

```
加新功能 = 造 3 个零件 + 3 个清单登记 + 1 条命令

1. repository/xxx.go   →  NewXxxRepo(db)
2. service/ports.go    →  接口     service/xxx.go → NewXxxService(接口)
3. handler/xxx.go      →  NewXxxHandler(*service.XxxService)

登记：
   repository/wire.go  → 构造器 + wire.Bind（两行！）
   service/wire.go     → 构造器（一行）
   handler/wire.go     → 构造器（一行）
   router/router.go    → Handlers 字段（手动，wire 不管）
   router/admin.go     → 路由注册（手动）

收尾：make wire && go build ./...

特殊：要 config 数值 / 要 Boot+Close → 去 bootstrap/providers.go 抄包装
```

**wire 按类型连线。构造函数写对了、清单登记了，剩下的它全包；漏了它编译期就骂你，骂得很具体。**
