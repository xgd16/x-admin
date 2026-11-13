---
title: 首页
hide:
  - navigation
  - toc
---

<div class="docs-hero" markdown>

# x-admin 框架文档

<p class="docs-hero__lead">通用 RBAC 管理后台 · Go + React 前后端分离。从本地启动、分层架构到每个业务模块的实现约定，写给要在这套脚手架上继续加功能的人。</p>

<p class="docs-hero__actions">
<a class="md-button md-button--primary" href="guide/getting-started.md">本地启动</a>
<a class="md-button" href="#document-map">浏览全部文档</a>
</p>

</div>

## 文档地图 {#document-map}

<div class="grid cards" markdown>

-   :material-rocket-launch-outline:{ .lg .middle } __本地开发与快速上手__

    ---

    环境要求、三种启动方式、默认账号、常用命令与目录速查。

    [:octicons-arrow-right-24: 开始](guide/getting-started.md)

-   :material-sitemap-outline:{ .lg .middle } __架构与请求生命周期__

    ---

    分层边界、wire 装配、中间件顺序、一次请求的完整链路与错误处理。

    [:octicons-arrow-right-24: 阅读](guide/architecture.md)

-   :material-api:{ .lg .middle } __多 API 模式__

    ---

    后台默认 API + 用 create 生成带 JWT 的客户端、商户端等并行 API 面，独立前缀、限流与 issuer。

    [:octicons-arrow-right-24: 接入](guide/multi-api.md)

-   :material-puzzle-outline:{ .lg .middle } __新增一个业务模块__

    ---

    从迁移、模型、仓储、服务到路由、菜单权限与前端页面的端到端清单。

    [:octicons-arrow-right-24: 照抄流程](guide/adding-a-module.md)

-   :material-shield-account:{ .lg .middle } __权限体系（RBAC）__

    ---

    用户/角色/菜单/部门，权限码计算、越权保护与内置账号规则。

    [:octicons-arrow-right-24: 阅读](backend/rbac.md)

-   :material-lock-outline:{ .lg .middle } __认证、会话与二次验证__

    ---

    登录、JWT、refresh 轮换、会话撤销、TOTP、Passkey 与文件访问 Cookie。

    [:octicons-arrow-right-24: 阅读](backend/auth.md)

-   :material-monitor-dashboard:{ .lg .middle } __前端架构、路由与状态__

    ---

    启动链路、静态路由、后端菜单生成导航、页签保活与 Zustand 状态。

    [:octicons-arrow-right-24: 阅读](frontend/architecture.md)

-   :material-api:{ .lg .middle } __API 一览与错误码__

    ---

    全部端点、权限码、分页/响应约定、错误码与请求示例。

    [:octicons-arrow-right-24: 查阅](reference/api.md)

-   :material-hammer-wrench:{ .lg .middle } __框架升级与工具链__

    ---

    `go tool xadmin`：交互式 TUI + 三方合并升级、冲突处理、迁移撞号检查与构建校验。

    [:octicons-arrow-right-24: 阅读](tool/tui.md)

-   :material-server-network:{ .lg .middle } __部署与排错__

    ---

    Compose / 手动部署、Nginx、生产检查清单，以及按现象排查的手册。

    [:octicons-arrow-right-24: 部署](ops/deploy.md) · [:octicons-arrow-right-24: 排错](ops/troubleshooting.md)

</div>

## 按角色阅读

| 你是 | 推荐顺序 |
| --- | --- |
| 刚接手项目 | [本地开发](guide/getting-started.md) → [架构](guide/architecture.md) → [新增模块](guide/adding-a-module.md) |
| 后端开发 | [认证](backend/auth.md) → [RBAC](backend/rbac.md) → [基础库](backend/packages.md) → [数据库](backend/database.md) → [测试](backend/testing.md) |
| 前端开发 | [前端架构](frontend/architecture.md) → [API 客户端](frontend/api-client.md) → [通用组件](frontend/components.md) → [主题样式](frontend/theme.md) |
| 运维 / 上线 | [配置参考](ops/config.md) → [部署发布](ops/deploy.md) → [排错手册](ops/troubleshooting.md) |
| 想深入某个机制 | [wire 教程](wire-tutorial.md) · [异步导出](export.md) · [前台用户认证](front-auth.md) |

## 项目一览

<div class="shot-grid">
  <figure>
    <img src="screenshots/dashboard.jpg" alt="工作台看板" loading="lazy">
    <figcaption>工作台看板</figcaption>
  </figure>
  <figure>
    <img src="screenshots/users.jpg" alt="用户管理" loading="lazy">
    <figcaption>用户管理</figcaption>
  </figure>
  <figure>
    <img src="screenshots/login.jpg" alt="登录页" loading="lazy">
    <figcaption>登录页</figcaption>
  </figure>
  <figure>
    <img src="screenshots/profile-dark.jpg" alt="个人中心（深色）" loading="lazy">
    <figcaption>个人中心 · 深色模式</figcaption>
  </figure>
</div>

## 文档覆盖范围

**开发指南**：环境与命令、分层与请求链路、端到端新增模块。

**后端模块**：认证与会话、RBAC（用户/角色/菜单/部门）、字典/参数/通知/看板、文件与分享、定时任务、审计日志、在线会话与监控、异步导出。

**基础设施**：基础库（错误码/响应/缓存/JWT/存储/Excel/TOTP/树）、中间件、数据库迁移与软删除、测试夹具与 CI。

**前端**：路由与权限、导航生成、页签保活、API 客户端与 401 刷新、状态管理、通用组件、主题色板/材质/响应式。

**运维**：全部配置项与环境变量、Docker/手动部署、Nginx、多副本、备份恢复、排错。

**参考**：API 全量端点、权限码清单、数据表、错误码。

## 技术栈与官方文档

| 层 | 选型 | 官方文档 |
| --- | --- | --- |
| 后端语言 | Go 1.27 | [go.dev/doc](https://go.dev/doc/) |
| Web 框架 | Gin | [gin-gonic.com](https://gin-gonic.com/zh-cn/docs/) |
| ORM | GORM | [gorm.io](https://gorm.io/zh_CN/docs/) |
| 依赖注入 | Google wire | [wire 指南](https://github.com/google/wire/blob/main/docs/guide.md) |
| 数据库 | MySQL 8.4 | [dev.mysql.com/doc](https://dev.mysql.com/doc/) |
| 缓存 | Redis 7.4 | [redis.io/docs](https://redis.io/docs/latest/) |
| 前端框架 | React 19 | [zh-hans.react.dev](https://zh-hans.react.dev/) |
| 组件库 | Ant Design 6 | [ant.design](https://ant.design/components/overview-cn) |
| 构建 | Vite 8 | [cn.vite.dev](https://cn.vite.dev/) |

实现契约见 [系统实现规格](spec.md)；框架如何落地到代码，见 [架构与请求生命周期](guide/architecture.md)。

## 本地预览本站

```bash
pip install -r docs-requirements.txt
mkdocs serve
# 打开 http://127.0.0.1:8000/
```

推送到 `main` 且改动命中 `docs/**`、`SPEC.md`、`mkdocs.yml` 时，GitHub Actions 会自动构建并发布到 GitHub Pages。
