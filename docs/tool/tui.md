# 交互式界面（TUI）

在终端里直接运行 `go tool xadmin` 会打开交互式界面，不需要记命令和参数：

```bash
go tool xadmin
# 或显式打开
go tool xadmin tui
```

TUI 与文本命令调用的是同一套核心函数（`PrepareUpgrade`、`ExecuteUpgrade`、`CreateAPI`、`CreateHandler`、`RunDoctor` 等），不会产生两套行为。

## 1. 入口与回退

| 场景 | 行为 |
| --- | --- |
| 终端中运行 `go tool xadmin` | 打开 TUI |
| 管道、CI、重定向 | 输出文本帮助 |
| `XADMIN_NO_TUI=1 go tool xadmin` | 强制文本帮助 |
| `go tool xadmin --no-tui` | 强制文本模式（无子命令时输出帮助） |
| `go tool xadmin tui` | 在非交互终端中报错退出 |
| `go tool xadmin upgrade ...` | 始终使用文本命令 |

`CI=true` 也会自动回退到文本模式。

## 2. 主菜单

| 功能 | 说明 |
| --- | --- |
| 升级框架 | 检测最新版本，生成文件级计划，确认后执行三方合并与构建校验 |
| 检查更新 | 只查询 GitHub 最新 Release，不修改工作区 |
| 创建 API 面 | 生成 handler、路由和可选的 JWT 认证与用户表迁移 |
| 创建 Handler | 在指定 API 面/版本下生成资源路由 |
| 项目体检 | 检查版本、配置、迁移、前端工具链 |
| 构建校验 | 运行 `go build ./...` 与 `go vet ./...` |
| 回滚升级 | 恢复最近一次升级前的文件快照 |
| 初始化基准 | 为复制源码/新 `git init` 项目写入 `FRAMEWORK_VERSION` |
| 版本信息 | 查看工具、仓库和 Go 版本 |

顶部会显示当前框架版本、git 分支和工作区是否干净。升级或回滚完成后会自动刷新。

## 3. 按键

| 页面 | 按键 | 动作 |
| --- | --- | --- |
| 菜单 | `↑` / `↓` / `k` / `j` | 移动选择 |
| 菜单 | `enter` | 执行 |
| 菜单 | `q` | 退出 |
| 表单 | `tab` / `↓` | 下一个字段 |
| 表单 | `shift+tab` / `↑` | 上一个字段 |
| 表单 | `ctrl+t` | 切换“生成 JWT 认证”（创建 API 面时） |
| 表单 | `enter` | 提交 |
| 表单 | `esc` | 返回菜单 |
| 计划/输出 | `↑` / `↓` / `pgup` / `pgdown` | 纵向滚动 |
| 计划/输出 | `←` / `→` | 横向滚动 |
| 计划/输出 | `esc` | 返回 |
| 确认 | `enter` / `y` | 确认 |
| 确认 | `esc` / `n` | 取消 |
| 任意页面 | `ctrl+c` | 退出 |

## 4. 升级流程

选择「升级框架」后，TUI 会：

1. 自动解析当前基准和目标版本；
2. 下载上游源码并生成文件级计划；
3. 展示更新/新增/删除/冲突统计和迁移问题；
4. 等待确认；
5. 执行三方合并、构建校验、提交；
6. 在输出页展示完整过程和结果。

存在迁移编号 **error** 时不会执行升级；存在文件冲突时会明确提示，确认后才会写入冲突标记或 `.upstream` 文件。

TUI 默认使用 `--yes` 语义（确认已在界面完成），不会在子进程中再次提示。需要 `--pr`、`--in-place`、`--with-tests` 等高级参数时，仍使用文本命令：

```bash
go tool xadmin upgrade --pr
go tool xadmin upgrade --in-place --with-tests
```

## 5. 代码生成

创建 API 面表单包含：

- API 面名称
- 版本（默认 `v1`）
- 路径前缀（默认 `/<name>/<version>`）
- 认证用户表（默认 `<name>_users`）
- 迁移编号（默认当前最大编号 + 1）
- 是否生成 JWT 认证

提交后会在输出页展示生成的文件、配置写入结果和下一步操作。生成的迁移编号、路由注册方式与命令行完全一致；`configs/config.yaml` 存在时会自动写入 `api.<name>`，并按名称排序。

## 6. 环境变量

| 变量 | 说明 |
| --- | --- |
| `XADMIN_NO_TUI=1` | 禁用 TUI，输出文本帮助 |
| `CI=true` | 自动禁用 TUI |
| `X_ADMIN_GITHUB_TOKEN` / `GITHUB_TOKEN` / `GH_TOKEN` | 升级检查使用的 GitHub token |
| 终端尺寸 | 监听 `WindowSizeMsg`，窗口变化时自动调整 |
