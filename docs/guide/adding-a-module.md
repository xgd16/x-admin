# 新增一个业务模块（端到端清单）

这篇文档用「公告 Announcement」做例子，把从数据库到前端页面的完整流程走一遍。依赖注入的原理见 [wire 新手教程](../wire-tutorial.md)，这里只给可照抄的落地步骤。

> 命名约定：表 `sys_announcements`、模型 `Announcement`、仓储 `AnnouncementRepo`、服务 `AnnouncementService`、处理器 `AnnouncementHandler`、权限前缀 `system:announcement:*`、前端路由 `/system/announcements`。

## 0. 先确定契约

| 项目 | 示例 |
| --- | --- |
| 资源 | 公告：标题、正文、状态、发布时间 |
| 权限码 | `system:announcement:list/create/update/delete` |
| 接口前缀 | `/admin/v1/announcements` |
| 前端页面 | `/system/announcements` |
| 导出模块名 | `announcements`（可选） |

先把权限码和接口路径定下来，后面每一层都围绕它写。

## 1. 数据库迁移

在 `migrations/` 新增一对文件，编号接着现有最大值：

```sql
-- migrations/000017_announcements.up.sql
CREATE TABLE IF NOT EXISTS `sys_announcements` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `title`      VARCHAR(128) NOT NULL,
  `content`    TEXT NOT NULL,
  `status`     TINYINT NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_announcements_status` (`status`),
  KEY `idx_announcements_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

```sql
-- migrations/000017_announcements.down.sql
DROP TABLE IF EXISTS `sys_announcements`;
```

种子数据（菜单/按钮）通常放在同一个 `up.sql` 里，用 `INSERT IGNORE` 保证可重复执行：

```sql
INSERT IGNORE INTO `sys_menus`
(`id`,`parent_id`,`name`,`type`,`path`,`component`,`permission`,`icon`,`sort`,`visible`,`status`,`created_at`,`updated_at`)
VALUES
(300,2,'公告管理',2,'/system/announcements','system/announcements','system:announcement:list','bell',12,1,1,NOW(3),NOW(3)),
(301,300,'新建公告',3,'','','system:announcement:create','',1,1,1,NOW(3),NOW(3)),
(302,300,'编辑公告',3,'','','system:announcement:update','',2,1,1,NOW(3),NOW(3)),
(303,300,'删除公告',3,'','','system:announcement:delete','',3,1,1,NOW(3),NOW(3));
```

新建菜单 id 要避开已有范围；上线后**不要修改已发布的迁移**，结构变更请追加新版本。

## 2. 模型 `internal/model/announcement.go`

```go
package model

type Announcement struct {
    Base
    Title   string `gorm:"size:128;not null" json:"title"`
    Content string `gorm:"type:text;not null" json:"content"`
    Status  int8   `gorm:"not null;index" json:"status"`
}

func (Announcement) TableName() string { return "sys_announcements" }
```

模型只放数据结构与标签；查询条件放 `filter.go`：

```go
type AnnouncementFilter struct {
    pagination.Query
    Status *int8 `form:"status" json:"status"`
}
```

## 3. 仓储端口 `internal/service/ports.go`

service 只依赖接口，先定义能力：

```go
type AnnouncementRepository interface {
    Create(ctx context.Context, a *model.Announcement) error
    Update(ctx context.Context, a *model.Announcement) error
    Get(ctx context.Context, id uint64) (*model.Announcement, error)
    List(ctx context.Context, f model.AnnouncementFilter) ([]model.Announcement, int64, error)
    Delete(ctx context.Context, ids ...uint64) error
    Count(ctx context.Context, f model.AnnouncementFilter) (int64, error)
    ExportPage(ctx context.Context, f model.AnnouncementFilter, afterID uint64, limit int) ([]model.Announcement, error)
}
```

`ExportPage` 和 `Count` 只有做异步导出时才需要，见第 10 步。

## 4. 仓储实现 `internal/repository/announcement.go`

```go
type AnnouncementRepo struct {
    *Base[model.Announcement]
    db *gorm.DB
}

func NewAnnouncementRepo(db *gorm.DB) *AnnouncementRepo {
    return &AnnouncementRepo{Base: NewBase[model.Announcement](db), db: db}
}

func (r *AnnouncementRepo) Get(ctx context.Context, id uint64) (*model.Announcement, error) {
    return r.FindByID(ctx, id)
}

func (r *AnnouncementRepo) List(ctx context.Context, f model.AnnouncementFilter) ([]model.Announcement, int64, error) {
    var rows []model.Announcement
    q := r.db.WithContext(ctx).Model(&model.Announcement{})
    if f.Keyword != "" {
        q = q.Where("title LIKE ?", like(f.Keyword))
    }
    if f.Status != nil {
        q = q.Where("status = ?", *f.Status)
    }
    var total int64
    if err := q.Count(&total).Error; err != nil {
        return nil, 0, err
    }
    err := paginate(q.Order("id DESC"), f.Page, f.PageSize).Find(&rows).Error
    return rows, total, err
}
```

在 `internal/repository/wire.go` 注册：

```go
NewAnnouncementRepo,
wire.Bind(new(service.AnnouncementRepository), new(*AnnouncementRepo)),
```

## 5. 服务 `internal/service/announcement.go`

```go
type AnnouncementInput struct {
    Title   string `json:"title"`
    Content string `json:"content"`
    Status  int8   `json:"status"`
}

type AnnouncementService struct {
    announcements AnnouncementRepository
}

func NewAnnouncementService(repo AnnouncementRepository) *AnnouncementService {
    return &AnnouncementService{announcements: repo}
}

func (s *AnnouncementService) Create(ctx context.Context, in AnnouncementInput) (*model.Announcement, error) {
    if err := validateAnnouncement(in); err != nil {
        return nil, err
    }
    a := &model.Announcement{Title: trim(in.Title), Content: in.Content, Status: in.Status}
    if err := s.announcements.Create(ctx, a); err != nil {
        return nil, err
    }
    return a, nil
}

// validateAnnouncement 校验必填字段与状态；业务规则统一放 service。
func validateAnnouncement(in AnnouncementInput) error {
    if trim(in.Title) == "" {
        return apperr.New(apperr.CodeInvalidArgument, "公告标题不能为空")
    }
    if trim(in.Content) == "" {
        return apperr.New(apperr.CodeInvalidArgument, "公告正文不能为空")
    }
    return ValidateStatus(in.Status)
}
```

规则：

- 参数校验、唯一性、状态流转、跨聚合检查都放 service。
- 不存在统一转 `apperr.ErrNotFound`；冲突用 `apperr.CodeConflict`。
- 需要发站内通知时注入 `Notifier`，用 `notify(ctx, s.notifier, ...)`，失败不影响主流程。
- 不要 import Gin；不要直接拿 `*gorm.DB`。

在 `internal/service/wire.go` 的 `ProviderSet` 加入 `NewAnnouncementService`。

## 6. 处理器 `internal/handler/announcement.go`

```go
type AnnouncementHandler struct {
    announcements *service.AnnouncementService
}

func NewAnnouncementHandler(s *service.AnnouncementService) *AnnouncementHandler {
    return &AnnouncementHandler{announcements: s}
}

func (h *AnnouncementHandler) List(c *gin.Context) {
    var f model.AnnouncementFilter
    if err := c.ShouldBindQuery(&f); err != nil {
        response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
        return
    }
    f.Normalize()
    list, total, err := h.announcements.List(c.Request.Context(), f)
    if err != nil {
        response.Error(c, err)
        return
    }
    response.Page(c, list, total, f.Page, f.PageSize)
}

func (h *AnnouncementHandler) Create(c *gin.Context) {
    var in service.AnnouncementInput
    if !Bind(c, &in) {
        return
    }
    row, err := h.announcements.Create(c.Request.Context(), in)
    if err != nil {
        response.Error(c, err)
        return
    }
    response.Created(c, row)
}
```

在 `internal/handler/wire.go` 注册 `NewAnnouncementHandler`，并在 `internal/router/router.go` 的 `Handlers` 结构体加字段：

```go
type Handlers struct {
    // ...
    Announcement *handler.AnnouncementHandler
}
```

## 7. 路由与权限

```go
secured.GET("/announcements", middleware.Require("system:announcement:list"), d.Handlers.Announcement.List)
secured.POST("/announcements", middleware.Require("system:announcement:create"), d.Handlers.Announcement.Create)
secured.PUT("/announcements/:id", middleware.Require("system:announcement:update"), d.Handlers.Announcement.Update)
secured.DELETE("/announcements/:id", middleware.Require("system:announcement:delete"), d.Handlers.Announcement.Delete)
```

路由是权限的唯一事实来源；新增端点必须在这里写 `Require`，不要指望前端隐藏按钮。

## 8. 生成 wire 并验证

```bash
make wire
go build ./...
go test ./...
```

`make wire` 会重新生成 `internal/bootstrap/wire_gen.go`。如果报 "no provider found"，检查构造函数是否加进了对应的 `ProviderSet`。

## 9. 前端

### 9.1 类型与页面

在 `frontend/src/api/types.ts` 加 `Announcement` / `AnnouncementInput`；在 `pages/Announcements.tsx` 参照 `Parameters.tsx` 写列表 + 弹窗：

```tsx
const query = useQuery({
  queryKey: ['announcements', page, pageSize, keyword],
  queryFn: () => api.get<PageResult<Announcement>>('/announcements', { page, page_size: pageSize, keyword }),
})

const save = useMutation({
  mutationFn: (values: AnnouncementInput) =>
    editing ? api.put(`/announcements/${editing.id}`, values) : api.post('/announcements', values),
  onSuccess: () => {
    toast.success(editing ? '公告已更新' : '公告已创建')
    void queryClient.invalidateQueries({ queryKey: ['announcements'] })
  },
  onError: (error) => toast.fail(error),
})
```

### 9.2 路由与导航元数据

`frontend/src/App.tsx` 加：

```tsx
<Route path="system/announcements" element={<Announcements />} />
```

`frontend/src/config/nav.ts` 的 `navItems` 加：

```ts
{ key: 'announcements', label: '公告管理', path: '/system/announcements',
  icon: 'bell', permission: 'system:announcement:list', group: '系统' },
```

前端路由是静态注册的（`navMeta` 只认识 `navItems` 里的路径）；只加后端菜单而不加这里，导航不会出现。

### 9.3 按钮权限

```tsx
const can = useAuth((state) => state.can)
{can('system:announcement:create') && <Button>新建公告</Button>}
```

## 10. 接入异步导出（可选）

在 `AnnouncementService` 实现：

```go
func (s *AnnouncementService) ExportBuilders() map[string]ExportBuilder {
    return map[string]ExportBuilder{
        "announcements": func(ctx context.Context, userID uint64, params json.RawMessage) (ExportJob, error) {
            var f model.AnnouncementFilter
            if len(params) > 0 {
                if err := json.Unmarshal(params, &f); err != nil {
                    return ExportJob{}, apperr.New(apperr.CodeInvalidArgument, "导出参数不合法")
                }
            }
            return ExportJob{
                Module:  "announcements",
                Title:   "公告列表",
                Headers: []string{"ID", "标题", "状态", "创建时间"},
                Total:   func(ctx context.Context) (int64, error) { return s.announcements.Count(ctx, f) },
                Fetch: func(ctx context.Context, afterID uint64, limit int) ([][]any, uint64, error) {
                    rows, err := s.announcements.ExportPage(ctx, f, afterID, limit)
                    if err != nil {
                        return nil, 0, err
                    }
                    out := make([][]any, 0, len(rows))
                    var next uint64
                    for _, row := range rows {
                        out = append(out, []any{
                            row.ID, row.Title, row.Status,
                            row.CreatedAt.Format("2006-01-02 15:04:05"),
                        })
                        next = row.ID
                    }
                    return out, next, nil
                },
            }, nil
        },
    }
}
```

在 `internal/bootstrap/providers.go` 的 `provideExportService` 里遍历注册。前端只需在列表页调用 `POST /export-tasks`，module 传 `announcements`。机制详见 [异步导出](../export.md)。

## 11. 测试

至少补三类：

1. **service 单测**：`tests/unit/service/announcement_test.go`，用 `testsupport.New(t)` 拿真实 SQLite + 内存缓存，覆盖校验、404、冲突。
2. **HTTP 集成**：`tests/integration/announcement_test.go`，用 `SeedAdmin` + `Login` 走真实鉴权，验证权限码和响应结构。
3. **前端组件测试**：`pages/Announcements.test.tsx` 或复用 `DataTable` 的既有测试模式。

```go
app := testsupport.New(t)
admin := app.SeedAdmin(t)
pair := app.Login(t, admin.Username, "Admin@123456")
rr := app.Do(http.MethodPost, "/admin/v1/announcements", map[string]any{
    "title": "维护通知", "content": "今晚 23:00 升级", "status": 1,
}, pair.AccessToken)
require.Equal(t, 201, rr.Code)
```

测试细节见 [测试与质量](../backend/testing.md)。

## 12. 完成检查表

- [ ] 迁移有 up/down，编号不冲突，种子用 `INSERT IGNORE`
- [ ] model 只含数据结构，表名 `sys_` 前缀
- [ ] 仓储实现端口，事务内的关联清理完整
- [ ] service 做校验与越权保护，错误码语义正确
- [ ] handler 不直接访问 DB，统一 `response.OK/Created/Page/Error`
- [ ] 所有端点在 router 里声明 `Require`
- [ ] wire provider 已注册并重新生成
- [ ] 菜单/按钮权限已入库，`super_admin` 可见
- [ ] 前端路由、navItems、按钮 `can()` 三处一致
- [ ] Go 单测 + HTTP 集成 + 前端测试通过
- [ ] `make lint`、`make wire`、`make test-all` 无报错
