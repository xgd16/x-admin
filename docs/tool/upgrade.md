# 框架升级工具

`go tool xadmin` 是 x-admin 自带的安全升级工具。它解决一个具体问题：

> 你基于 x-admin 的源码创建了自己的项目（通常是复制源码后 `git init`，和上游没有共享历史），上游发布了新版本。怎样在不覆盖你本地改动的前提下，把框架更新合进来？

x-admin 是**应用型脚手架**，不是可以 `go get -u` 的库。升级的本质是三方合并：

| 参与方 | 来源 |
| --- | --- |
| base（基准） | 你当前代码对应的上游版本 |
| local（本地） | 你自己的业务改动 |
| incoming（上游） | GitHub 新 Release 的源码归档 |

工具只自动应用**你没有改过**的文件；双方都改过的文件会尝试文本三方合并，仍有冲突时写入冲突标记或 `.upstream` 文件，绝不静默覆盖。

> 在终端直接运行 `go tool xadmin` 可以打开交互式界面，升级流程与本文命令一致，见 [交互式界面（TUI）](tui.md)。

## 你的项目是一个独立仓库

这个工具的默认假设是：你基于 x-admin 的源码创建了**自己的项目仓库**，有自己的 `.git`、自己的远程仓库和自己的发布节奏。工具不会向 `xgd16/x-admin` 推送任何东西，也**不要求你的 `.git` 与上游共享历史**。

- 所有文件修改、分支、提交都发生在你的项目里；
- `.x-admin/` 是本地升级缓存，默认忽略；`FRAMEWORK_VERSION` 是需要提交的版本标记；
- `--pr` 只会 push 到你配置的 `origin`，并向**你自己的** `main` 开 PR；
- 上游仓库只作为"新版本源码"的下载源；工具不会 fetch 上游 git 历史，也不会修改你的 remote；
- 即使你的 `.git` 是刚 `git init` 的、没有任何上游 tag 和历史，也可以升级（见下文"基准版本如何确定"）。

## 快速开始

工具通过 `go.mod` 的 `tool` 指令注册，克隆/复制项目后即可使用，不需要全局安装：

```bash
# 0. 第一次使用、且知道当前基于哪个版本时，先登记基准
go tool xadmin upgrade init --from v1.3.0

# 1. 一键升级：自动检测、生成计划、提示确认、合并、构建校验
go tool xadmin upgrade

# 2. 只想先看计划，不写入工作区
go tool xadmin upgrade plan --list

# 3. 脚本/CI 中跳过确认
go tool xadmin upgrade --yes

# 4. 升级后推送分支并创建 PR
go tool xadmin upgrade --yes --pr
```

`upgrade` 不带子命令时就是"检测 + 升级"的完整流程；已经是最新版本会直接退出。需要更细的控制时再用 `upgrade check` / `plan` / `apply`。不知道当前基于哪个版本也没关系：直接运行 `upgrade` 或 `upgrade plan`，工具会按文件内容指纹推断，并给出 `upgrade init --from ...` 的建议命令。

写入前会检查工作区是否干净、迁移编号是否冲突；有问题会直接停止，不会创建分支或改动文件。

升级完成后，工具会：

1. 在新分支 `chore/upgrade-xadmin-<版本>` 上写入合并结果；
2. 把被覆盖/删除的文件备份到 `.git/x-admin/backups/<时间戳>/`；
3. 按需运行 `go mod tidy`、`go build ./...`、`go vet ./...`；
4. 更新 `FRAMEWORK_VERSION` 并提交（`.x-admin/framework.json` 作为本地缓存同步更新，已忽略）；
5. 可选地推送分支、创建 PR。

前端构建较慢，默认不执行；需要时加 `--with-frontend`，或交给 PR 上的 CI。

## 已有项目如何接入

工具依赖仓库里的 `cmd/xadmin`、`tool/xadmin` 和 `FRAMEWORK_VERSION`，并已在 `go.mod` 注册：

```text
tool x-admin/cmd/xadmin
```

在这个工具之前创建的项目没有这些文件，有两种接入方式。**新 `git init` 的项目同样适用**。

**方式一：从上游 checkout 直接运行（推荐，零侵入）**

```bash
git clone https://github.com/xgd16/x-admin /tmp/x-admin-tool
cd /tmp/x-admin-tool
go build -o /tmp/xadmin ./cmd/xadmin

/tmp/xadmin upgrade check --dir /path/to/your/project
/tmp/xadmin upgrade plan  --dir /path/to/your/project
/tmp/xadmin upgrade --dir /path/to/your/project --yes
```

工具操作的是 `--dir` 指向的项目，不需要先把工具复制进去。

**方式二：把工具文件合入项目**

从上游复制 `cmd/xadmin/`、`tool/xadmin/` 和 `FRAMEWORK_VERSION`，然后在 `go.mod` 末尾加一行：

```text
tool x-admin/cmd/xadmin
```

之后即可在项目根目录使用 `go tool xadmin`。

如果项目里没有状态文件（例如源码是从旧版本复制来的），先登记一次基准：

```bash
go tool xadmin upgrade init --from v1.3.0
```

> 由于模块路径是 `x-admin` 而不是 `github.com/xgd16/x-admin`，不能直接 `go install github.com/xgd16/x-admin/cmd/xadmin@vX.Y.Z`。请使用上面的本地构建方式。

**改了 module 路径怎么办**

如果你把 `go.mod` 的 `module x-admin` 改成自己的路径（例如 `example.com/myapp`），`tool` 指令和 `cmd/xadmin/main.go` 的 import 需要同步。运行：

```bash
go tool xadmin fix-module
```

升级工具在 `apply` 成功后也会自动做这一步；`doctor` 会检查是否遗漏。如果改完 module 后 `go tool xadmin` 已经无法构建，先用「已有项目如何接入」里的方式从上游 checkout 构建工具，再运行：

```bash
/tmp/xadmin fix-module --dir /path/to/your/project
```

## 基准版本如何确定

工具按以下顺序确定 base，越靠前越精确：

1. **`--from`**：显式指定 tag 或 commit。
2. **`.x-admin/framework.json`**：本地缓存的结构化状态（默认忽略），包含版本和 commit。
3. **`FRAMEWORK_VERSION`**：根目录的可见版本标记，需要提交到仓库；复制源码时也不会丢。
4. **上游同名 git tag**：只有本地 tag 与上游同名 tag 指向同一个 commit 时才采用，避免把用户自己的版本 tag 误当成框架版本。
5. **内容指纹**：前面都不可用时，读取上游最近的 tag 文件树，用本地文件内容做比对，推断最接近的版本。

目标版本默认取 GitHub 最新 Release，也可以用 `--to v1.4.0` 指定。

`upgrade check` 在无法从状态文件/版本标记/tag 确定当前版本时，也会尝试内容指纹识别。`upgrade plan` 会显示基准版本和来源，例如：

```text
基准      v1.3.0 (9434cd61)  [状态文件]
```

### 新 git init / 复制源码的项目

这是最常见的下游场景：你把 x-admin 源码复制到自己的目录，`git init` 后提交，`.git` 里没有上游历史，也没有 tag。

**知道当前版本**：

```bash
go tool xadmin upgrade init --from v1.3.0
git add FRAMEWORK_VERSION
git commit -m "chore: record x-admin base version"
```

**不知道当前版本**：

```bash
go tool xadmin upgrade plan
```

工具会按内容指纹给出最接近的候选版本，例如：

```text
[warn] 未找到共享 git 历史或版本状态，正在按文件内容识别基准版本...
[warn] 按内容指纹推断基准为 v1.3.0（匹配 92%，287/312 个文件，置信度 high）
```

如果置信度低，工具不会猜，而是列出候选并提示你显式指定：

```text
文件指纹最接近：v1.3.0（41%）、v1.2.0（37%）
建议显式指定基准：
  go tool xadmin upgrade init --from v1.3.0
```

指纹识别需要调用 GitHub 的 tree API（每个候选 tag 2 个请求）。匿名请求有速率限制，建议设置 `GITHUB_TOKEN`。

> 如果你只复制了部分目录（例如只拿走 `internal/` 和 `frontend/`），文件指纹的匹配率会偏低，工具不会猜；请直接用 `--from v1.3.0` 指定。


### 维护者：发布时更新版本标记

`FRAMEWORK_VERSION` 是需要提交的版本标记，`.x-admin/framework.json` 是本地缓存（已忽略）。发布新版本前，建议更新 `FRAMEWORK_VERSION`：

```text
v1.4.0
```

不更新也不会导致错误：旧一点只会让三方合并的基准更早、diff 更大。但保持它准确可以让复制源码创建的项目获得更好的默认体验。

下游项目不需要手动维护：`upgrade init` / `upgrade` 会自动更新 `FRAMEWORK_VERSION` 和本地缓存。

## 三方合并规则

| base | 本地 | 上游 | 处理 |
| --- | --- | --- | --- |
| 有 | 未改 | 改了 | 自动更新 |
| 有 | 改了 | 未改 | 保留本地 |
| 有 | 改了 | 改了，内容相同 | 无需处理 |
| 有 | 改了 | 改了，可合并 | 三方合并后写入 |
| 有 | 改了 | 改了，冲突 | 写入冲突标记，退出码 2 |
| 有 | 未改 | 删除 | 删除 |
| 有 | 改了 | 删除/重命名 | 保留本地，报告冲突 |
| 有 | 删除 | 未改 | 保持删除（无需操作） |
| 有 | 删除 | 改了 | 本地保持删除，上游版本写入 `.upstream` |
| 无 | 无 | 新增 | 新增 |
| 无 | 有，内容相同 | 新增 | 无需处理 |
| 无 | 有，内容不同 | 新增 | 本地保留，上游版本写入 `.upstream` |
| 无 | 有 | 无 | 用户自己的文件，完全不碰 |

以下文件永远不参与合并：

- `configs/config.yaml`、`configs/local.yaml`
- `.env`、`.env.*`（`.env.example` 除外）
- `data/uploads/`、`node_modules/`、`frontend/dist/`、`bin/`、`site/`、`.git/`

`go.mod` / `go.sum` 按文本合并；需要时工具会在校验前运行 `go mod tidy`。

## 处理冲突

`apply` 遇到冲突时会停止在提交之前：

- **文本冲突**：文件里写入 `<<<<<<< local` / `||||||| base` / `>>>>>>> upstream` 标记。
- **二进制冲突**、**上游新增同名文件**、**本地删除但上游修改**：本地文件保持原样，上游版本写入 `<path>.upstream`。
- **上游删除但本地修改**：保留本地文件，不写 `.upstream`。

退出码为 `2`，方便 CI 识别。处理方式：

```bash
# 查看冲突
git status
git diff

# 解决后校验
go tool xadmin upgrade verify

# 如果自定义过 module 路径
go tool xadmin fix-module

# 手动提交（apply 不会替你提交冲突中的代码）
git add -A
git commit -m "chore(upgrade): resolve framework upgrade conflicts"
```

如果决定放弃：

```bash
go tool xadmin upgrade rollback
```

`rollback` 会恢复最近一次 `apply` 前的文件快照。如果升级没有产生 commit，且当前还在升级分支上，它会自动切回原分支；如果已经提交，按提示执行 `git reset HEAD^` 回到升级前的提交。

### 依赖文件冲突

- `go.mod` / `go.sum` 按文本合并；解决冲突后运行 `go tool xadmin upgrade verify --tidy`，让 Go 重新整理依赖。
- `frontend/package-lock.json` 冲突时不要手工拼 JSON，删除冲突标记后运行 `cd frontend && npm install` 重新生成锁文件。
- 二进制文件（图片、字体等）冲突时，工具不会自动合并，会保留本地并写入 `.upstream`。

## 迁移编号：始终使用最大编号 + 1

这是升级中最容易踩的坑。

`golang-migrate` 只执行版本号**大于** `schema_migrations` 当前值的迁移。假设：

1. 你在本地新增了 `000005_announcements` 并已在数据库执行，当前 schema version = 5；
2. 上游 v1.4.0 新增了 `000002_notifications`；
3. 新版启动时，这个上游迁移的版本号不大于 5，会被**直接跳过**。

因此约定：

- 新迁移的编号始终取当前 `migrations/` 下的**最大编号 + 1**；
- `go tool xadmin create api --auth` 已按这个规则自动取号；
- 不要预留区间、不要跳号、不要回填比当前最大值更小的编号；
- 不要修改已发布的迁移文件。

`upgrade plan` 会检测两类问题：

- 同编号冲突（error）：本地 `000002_x` 与上游 `000002_y`；
- 上游新增编号低于本地最大编号（warning）：存在被跳过的风险。

出现 **error** 时，`upgrade` 默认拒绝执行，必须先处理编号；确认要强制继续时加 `--allow-migration-conflict`（不推荐）。出现 warning 时，先备份数据库，再决定处理方式。如果本地已经有执行过的迁移，正确做法是把**上游新增的迁移**重编号到本地最大编号之后（up/down 成对改），再启动应用；不要调低本地已执行的编号，也不要直接改 `schema_migrations` 来“补跑”低编号迁移。

工具不会自动改迁移编号，也不会自动执行 SQL。

## 配置升级

上游新增的配置键会随 `configs/config.example.yaml` 一起合并。`plan` 会对比本地 `configs/config.yaml`，列出缺失的键和默认值：

```
本地配置缺少的新键
  + security.unlock_max_fails = 5
  → 新键已包含在 configs/config.example.yaml，请按需同步到 configs/config.yaml
```

工具不会读取或回写你的密钥，也不会自动修改 `configs/config.yaml`。

## 安全机制

| 机制 | 说明 |
| --- | --- |
| git 工作区检查 | 默认要求干净；`--allow-dirty` 可跳过 |
| 无历史基准识别 | 状态文件 → `FRAMEWORK_VERSION` → git tag → 内容指纹；新 `git init` 也能升级 |
| 独立分支 | 默认创建 `chore/upgrade-xadmin-<版本>`；`--in-place` 可直接在当前分支升级 |
| 文件备份 | 升级前把将被覆盖/删除的文件保存到 `.git/x-admin/backups/<时间戳>/` |
| 冲突不提交 | 有冲突时退出码 2，不自动提交 |
| 构建校验 | 默认 `go build ./...` + `go vet ./...`；`--with-tests` 追加测试 |
| 失败可回滚 | `upgrade rollback` 恢复最近一次快照 |
| PR 工作流 | `--pr` 推送分支并创建 PR，让 GitHub Actions 再跑一遍完整 CI |

数据库迁移不在工具的控制范围内。**升级生产环境前先备份数据库和上传目录**，这是工具无法替代的步骤。

## 在 CI 中使用

`plan` 支持 `--json` 和 `--fail-on-conflict`（文件冲突或迁移编号错误都会让退出码为 2）：

```bash
go tool xadmin upgrade plan --json > plan.json
go tool xadmin upgrade plan --fail-on-conflict
```

退出码：

| 退出码 | 含义 |
| --- | --- |
| 0 | 成功 / 已是最新 / 无冲突 |
| 1 | 参数、网络、git 或构建校验失败；或 `check --fail-on-outdated` 发现新版本 |
| 2 | `plan --fail-on-conflict` 发现文件冲突/迁移编号错误，或 `apply` 存在未解决冲突 |

`apply --json` 不支持；请先用 `plan --json` 获取机器可读计划。

### GitHub Actions 定时检查

可以用一个定时工作流每周检查上游版本，把计划作为 artifact 保留，或接 issue / 通知：

```yaml
name: x-admin upgrade check
on:
  schedule:
    - cron: "0 1 * * 1"
  workflow_dispatch:

jobs:
  check:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 1   # 只需要工作区，不依赖上游 git 历史
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Check upstream
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: go tool xadmin upgrade check --fail-on-outdated --json > upgrade-check.json
      - name: Plan
        if: always()
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: go tool xadmin upgrade plan --json > upgrade-plan.json
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: x-admin-upgrade-plan
          path: |
            upgrade-check.json
            upgrade-plan.json
```

`--fail-on-outdated` 让工作流在发现新版本时变红；如果不想用失败作为提醒，也可以去掉它，改为解析 JSON 后创建 issue。

### 手动触发升级 PR

把升级动作也放进 Actions，需要时点一下 `Run workflow`：

```yaml
name: x-admin upgrade
on:
  workflow_dispatch:

permissions:
  contents: write
  pull-requests: write

jobs:
  upgrade:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 1
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Configure git
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
      - name: Upgrade and open PR
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: go tool xadmin upgrade --yes --pr
```

CI 会再跑一遍完整的 backend/frontend 检查；确认无误后合并 PR 即可。这个 PR 是在**你自己的仓库**里，把升级分支合并到你的 `main`，不会提交到 `xgd16/x-admin`。数据库迁移仍然建议在合并后、启动新版前手动备份。

## 私有仓库与离线使用

你的项目仓库是私有的不影响升级；token 只用于访问**上游框架源码**。如果上游使用了私有镜像，再用 `--repo` 指向镜像：

```bash
# 上游是公开仓库：项目私有也可以正常升级
GITHUB_TOKEN=ghp_xxx go tool xadmin upgrade check

# 上游是私有镜像时
go tool xadmin upgrade plan --repo your-org/x-admin-mirror --token ghp_xxx

# 指定版本
go tool xadmin upgrade plan --to v1.4.0
go tool xadmin upgrade --from v1.3.0 --to v1.4.0

# 完全离线（调试/测试）：直接对比两个源码目录
go tool xadmin upgrade plan --from-dir /path/to/old --to-dir /path/to/new
```

`--from-dir` / `--to-dir` 会跳过源码下载，但仍然读取本地工作区，并需要 git 才能执行 `apply`。

## 命令参考

```text
go tool xadmin upgrade           一键升级（等价于 upgrade apply）
  --yes                          跳过交互确认（非交互环境必需）
  --to v1.4.0                    指定目标版本
  --allow-dirty                  允许工作区有未提交改动
  --in-place                     不新建分支，直接在当前分支升级
  --pr                           推送并创建 PR
  --pr-base main                 PR 目标分支
  --with-tests                   追加 go test ./...
  --with-frontend                追加 npm ci / npm run build
  --no-verify                    跳过构建校验
  --no-tidy                      不运行 go mod tidy
  --no-commit                    写入和校验后不提交
  --allow-migration-conflict     迁移编号冲突时仍继续（不推荐）
  --branch <name>                自定义分支名
  --list                         列出全部文件变更
  --dry-run                      只打印计划

go tool xadmin upgrade check     只检查最新 Release
  --notes                        输出完整 Release 说明
  --fail-on-outdated             有新版本时退出码 1
  --json                         机器可读输出

go tool xadmin upgrade init      登记当前项目所基于的框架版本
  --from v1.3.0                  必填，tag 或 commit
  --json                         机器可读输出

go tool xadmin upgrade plan      只生成文件级计划
  --list                         列出全部文件变更
  --fail-on-conflict             有文件冲突或迁移编号错误时退出码 2
  --json                         机器可读输出

go tool xadmin upgrade apply     显式执行升级（与 upgrade 相同）
  ...                            参数同上

go tool xadmin upgrade verify    校验当前工作区
  --with-tests / --with-frontend / --tidy

go tool xadmin upgrade rollback  恢复最近一次升级前的文件快照
  --id <备份目录名>

go tool xadmin doctor            项目体检
  --online                       检查上游最新版本
  --json                         机器可读输出

go tool xadmin fix-module        适配自定义 module 路径
  --dir <目录>
```

通用参数：`--repo`、`--token`、`--from`、`--to`、`--dir`、`--timeout`、`--verbose`、`--no-fingerprint`。

## 局限

- 不做**语义级**合并。Go 代码按文本合并，接口变更、重命名、行为变化仍需要人审查。
- 不自动处理 `go.mod` 的依赖版本冲突；冲突会保留为文本冲突。
- 不自动执行或重排数据库迁移，只报告编号风险。
- 不合并二进制文件的内容。
- 不处理 git submodule 或 Git LFS 指针；如项目使用，需要人工检查。
- 归档中的符号链接会被跳过；框架当前没有符号链接。
- 内容指纹是启发式推断；置信度不足时工具不会猜，会要求用 `--from` 指定。
- 指纹识别会调用 GitHub tree API；匿名请求有速率限制，建议设置 `GITHUB_TOKEN`。
- 不 fetch 上游 git 历史；`--pr` 只操作你自己的 `origin`，不会向上游仓库推送。
- 前端构建默认关闭，完整验证建议交给 PR 上的 CI。

## 后续规划

工具已经为代码生成预留了位置。计划中的 `generate` 子命令会读取一份资源描述（表名、字段、权限前缀），生成：

- 迁移 up/down（按上述编号约定取号）；
- model、filter、repository、service、handler；
- 路由与 wire 装配片段；
- OpenAPI 片段；
- 前端列表页、表单、API 客户端与菜单种子数据。

资源描述草案：

```yaml
# x-admin/announcement.yaml
resource: announcement
table: sys_announcements
permission: system:announcement
route: /system/announcements
fields:
  - name: title
    type: string
    size: 128
    required: true
    query: like
  - name: content
    type: text
    required: true
  - name: status
    type: int8
    index: true
    default: 1
```

生成器的安全规则会和升级工具保持一致：

- 已存在的文件默认不覆盖；需要 `--force` 或逐个确认；
- 迁移编号自动取当前最大值，并避开框架保留区间；
- 路由、wire、菜单等"插入式"修改优先使用带标记的代码块，无法安全插入时输出补丁片段而不是猜；
- 生成后运行 `go build ./...` / `go vet ./...` 自检。

这样"新增业务模块"和"升级框架"可以共用同一套版本感知的工具链。

## 代码生成

`go tool xadmin` 还提供按 API 面/版本分层的脚手架生成：

```bash
go tool xadmin create api --name client --version v1 --auth --table client_users
go tool xadmin create handler --api client --version v1 --name order --path /orders
```

详见 [代码生成（create）](create.md)。
