package xadmin

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

const rootHelp = `xadmin - x-admin 框架工具链

用法:
  go tool xadmin                    打开交互式 TUI（终端中）
  go tool xadmin <command> [flags]  使用文本命令

命令:
  tui               打开交互式界面：升级、生成、体检、回滚
  upgrade           一键升级：自动检测 + 计划 + 确认 + 合并 + 校验
  upgrade check     只检查 GitHub 最新 Release，对比当前版本
  upgrade init      写入框架版本基准（新 git init / 复制源码的项目用）
  upgrade plan      只生成文件级升级计划（不改动工作区）
  upgrade apply     显式执行升级（与 upgrade 相同，保留给脚本）
  upgrade verify    对当前工作区运行构建校验
  upgrade rollback  恢复最近一次升级前的文件快照
  create            代码生成：新建 API 面或 handler 脚手架
  create api        新建 API 面 + 版本目录，自动注册路由工厂
  create handler    在指定 API 面/版本下新建 handler 与路由模块
  doctor            项目体检：版本、配置、迁移、前端工具链
  fix-module        把工具指令/import 适配到自定义 module 路径
  version           打印工具和运行环境版本

常用示例:
  go tool xadmin
  go tool xadmin upgrade
  go tool xadmin upgrade plan --list
  go tool xadmin upgrade --yes
  go tool xadmin create api --name client --version v1 --auth
  go tool xadmin create handler --api client --version v1 --name order
  go tool xadmin doctor --online

非交互环境（管道/CI）直接运行会输出本帮助；也可设置 XADMIN_NO_TUI=1 强制使用文本模式。
运行 "go tool xadmin <command> -h" 查看命令参数。
`

const upgradeHelp = `x-admin 框架升级

用法:
  go tool xadmin upgrade [flags]          一键升级（推荐）
  go tool xadmin upgrade <command> [flags] 高级命令

说明:
  upgrade 不带子命令时等价于 upgrade apply：
  自动检测最新版本 -> 生成文件级计划 -> 提示确认 -> 三方合并 -> 构建校验。
  已经是最新版本时直接退出，不做任何修改；有冲突时写入冲突标记并提示处理。

  升级基于三方合并：基准版本（上游旧版本）、本地改动、上游新版本。
  只自动应用用户没有改过的文件；双方都改过的文件会做文本合并，
  仍有冲突时写入冲突标记或 .upstream 文件，不会静默覆盖。

  工具假设你在自己的项目仓库里操作，不会向上游仓库推送任何内容。
  --pr 只会推送到你配置的 origin 并创建合并到你自己的 main 的 PR。

常用:
  go tool xadmin upgrade                  # 检测 + 升级（交互确认）
  go tool xadmin upgrade --yes            # 脚本/CI 中跳过确认
  go tool xadmin upgrade --pr             # 升级后推送并开 PR
  go tool xadmin upgrade plan --list      # 只看计划，不写入
  go tool xadmin upgrade --to v1.4.0      # 指定目标版本
  go tool xadmin upgrade init --from v1.3.0  # 新 .git 项目登记基准

安全机制:
  - 要求 git 工作区干净（--allow-dirty 可跳过）
  - 自动创建 chore/upgrade-xadmin-* 分支
  - 升级前把将被覆盖/删除的文件备份到 .git/x-admin/backups
  - 默认运行 go mod tidy（按需）、go build、go vet
  - 冲突未解决时不提交，退出码为 2
`

const createHelp = `x-admin 代码生成

用法:
  go tool xadmin create <api|handler> [flags]

说明:
  create 用于生成按 API 面和版本分层的 handler 脚手架：
    internal/handler/<api>/<version>/...
    internal/router/<api>_<version>_<name>.go

  生成的路由模块通过 init 自动注册，不需要手动修改 router.go。
  业务逻辑仍需要在 handler 中接入 service。

常用:
  go tool xadmin create api --name client --version v1 --auth
  go tool xadmin create handler --api client --version v1 --name order --path /orders

create api:
  --name <name>       新 API 面名称，例如 client/merchant/partner（必填）
  --version v1        API 版本，默认 v1
  --prefix /client/v1 路径前缀，默认 /<name>/<version>
  --auth              生成完整 JWT 认证（注册/登录/刷新/登出/me）
  --table <table>     认证用户表名，默认 <name>_users
  --migration <n>     认证表迁移编号，默认取当前最大编号 + 1
  --dir .             项目目录
  --force             覆盖已存在的生成文件
  --json              输出机器可读 JSON

create handler:
  --api <api>         API 面名称，例如 admin 或已生成的 client/merchant/partner（必填）
  --version v1        API 版本，默认 v1
  --name <name>       handler 资源名，例如 order（必填）
  --path /orders      路由路径，默认 /<name>
  --permission <p>    权限码前缀，例如 system:order；admin 默认 system:<name>
  --dir .             项目目录
  --force             覆盖已存在的生成文件
  --json              输出机器可读 JSON
`

// Run 是工具入口，返回进程退出码。
func Run(args []string) int {
	noTUI := false
	if len(args) > 0 && args[0] == "--no-tui" {
		noTUI = true
		args = args[1:]
	}
	if len(args) == 0 {
		if !noTUI && shouldUseTUI() {
			return runTUI()
		}
		fmt.Print(rootHelp)
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Print(rootHelp)
		return 0
	case "tui":
		if noTUI {
			fmt.Print(rootHelp)
			return 0
		}
		if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
			fmt.Fprintln(os.Stderr, "tui 需要交互式终端；请在终端中直接运行 go tool xadmin")
			return 1
		}
		return runTUI()
	case "version", "-v", "--version":
		printVersion()
		return 0
	case "doctor":
		return runDoctor(args[1:])
	case "create":
		return runCreate(args[1:])
	case "fix-module":
		return runFixModule(args[1:])
	case "upgrade":
		return runUpgrade(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n\n%s", args[0], rootHelp)
		return 1
	}
}

func printVersion() {
	printVersionTo(os.Stdout)
}

func printVersionTo(w io.Writer) {
	fmt.Fprintf(w, "xadmin tool %s\n", ToolVersion)
	fmt.Fprintf(w, "框架仓库   %s\n", DefaultRepo)
	fmt.Fprintf(w, "运行环境   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(w, "Go 版本    %s\n", runtime.Version())
}

func runUpgrade(args []string) int {
	if len(args) == 0 {
		return runUpgradeAuto(nil)
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(upgradeHelp)
		return 0
	case "check":
		return runUpgradeCheck(args[1:])
	case "init":
		return runUpgradeInit(args[1:])
	case "plan":
		return runUpgradePlan(args[1:])
	case "apply":
		return runUpgradeApply(args[1:])
	case "verify":
		return runUpgradeVerify(args[1:])
	case "rollback":
		return runUpgradeRollback(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return runUpgradeAuto(args)
		}
		fmt.Fprintf(os.Stderr, "未知 upgrade 子命令 %q\n\n%s", args[0], upgradeHelp)
		return 1
	}
}

type sourceFlags struct {
	opts  SourceOptions
	json  bool
	notes bool
	list  bool
}

func addSourceFlags(fs *flag.FlagSet, withJSON bool) *sourceFlags {
	sf := &sourceFlags{}
	fs.StringVar(&sf.opts.Repo, "repo", DefaultRepo, "上游仓库 owner/name")
	fs.StringVar(&sf.opts.Token, "token", "", "GitHub token（默认读 GITHUB_TOKEN / GH_TOKEN）")
	fs.StringVar(&sf.opts.From, "from", "", "基准版本 tag/commit（默认自动：状态文件 -> FRAMEWORK_VERSION -> tag -> 指纹）")
	fs.StringVar(&sf.opts.To, "to", "", "目标版本 tag/commit（默认最新 Release）")
	fs.StringVar(&sf.opts.Dir, "dir", ".", "项目目录")
	fs.StringVar(&sf.opts.FromDir, "from-dir", "", "离线基准源码目录（调试/测试）")
	fs.StringVar(&sf.opts.ToDir, "to-dir", "", "离线目标源码目录（调试/测试）")
	fs.DurationVar(&sf.opts.Timeout, "timeout", 10*time.Minute, "下载与网络操作超时")
	fs.BoolVar(&sf.opts.Verbose, "verbose", false, "输出回退原因等调试信息")
	fs.BoolVar(&sf.opts.NoFingerprint, "no-fingerprint", false, "不使用内容指纹推断基准版本")
	if withJSON {
		fs.BoolVar(&sf.json, "json", false, "输出机器可读 JSON")
	}
	return sf
}

func runUpgradeCheck(args []string) int {
	fs := flag.NewFlagSet("upgrade check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sf := addSourceFlags(fs, true)
	fs.BoolVar(&sf.notes, "notes", false, "输出完整 Release 说明")
	failOutdated := fs.Bool("fail-on-outdated", false, "有新版本时以退出码 1 结束（适合 CI 提醒）")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), sf.opts.Timeout)
	defer cancel()
	result, err := CheckUpgrade(ctx, sf.opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "检查失败: %v\n", err)
		return 1
	}
	if sf.json {
		if err := PrintJSON(os.Stdout, result); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON: %v\n", err)
			return 1
		}
	} else {
		PrintCheck(os.Stdout, result, sf.notes)
	}
	if *failOutdated && result.Comparable && !result.UpToDate {
		return 1
	}
	return 0
}

func runUpgradeInit(args []string) int {
	fs := flag.NewFlagSet("upgrade init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".", "项目目录")
	repo := fs.String("repo", DefaultRepo, "上游仓库 owner/name")
	token := fs.String("token", "", "GitHub token")
	from := fs.String("from", "", "当前项目所基于的框架版本 tag/commit（必填）")
	timeout := fs.Duration("timeout", 2*time.Minute, "解析版本超时")
	jsonOut := fs.Bool("json", false, "输出机器可读 JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if strings.TrimSpace(*from) == "" {
		fmt.Fprintln(os.Stderr, "需要 --from，例如：go tool xadmin upgrade init --from v1.3.0")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	state, _, err := InitBaseline(ctx, *repo, *token, *from, *dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if *jsonOut {
		_ = PrintJSON(os.Stdout, state)
		return 0
	}
	fmt.Printf("✔ 已记录框架基准：%s (%s)\n", state.Ref, ShortSHA(state.Commit))
	fmt.Printf("  %s（本地缓存，已被 .gitignore 忽略）\n", StatePath)
	fmt.Printf("  %s（需要提交）\n", VersionPath)
	fmt.Println("请提交 FRAMEWORK_VERSION；之后 upgrade 会以它为基准。")
	return 0
}

func runUpgradePlan(args []string) int {
	fs := flag.NewFlagSet("upgrade plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sf := addSourceFlags(fs, true)
	failOnConflict := fs.Bool("fail-on-conflict", false, "存在冲突时以退出码 2 结束（适合 CI）")
	fs.BoolVar(&sf.list, "list", false, "列出全部文件变更")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), sf.opts.Timeout)
	defer cancel()
	_, plan, warning, err := PrepareUpgrade(ctx, sf.opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成计划失败: %v\n", err)
		return 1
	}
	if warning != "" {
		fmt.Fprintf(os.Stderr, "[warn] %s\n", warning)
	}
	if sf.json {
		if err := PrintPlanJSON(os.Stdout, plan); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON: %v\n", err)
			return 1
		}
	} else {
		PrintPlan(os.Stdout, plan, sf.list, true)
	}
	if *failOnConflict && (plan.HasConflicts() || plan.HasMigrationErrors()) {
		return 2
	}
	return 0
}

func runUpgradeApply(args []string) int { return runUpgradeFlow("upgrade apply", args) }

func runUpgradeAuto(args []string) int { return runUpgradeFlow("upgrade", args) }

func runUpgradeFlow(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sf := addSourceFlags(fs, false)
	var opts ApplyOptions
	fs.BoolVar(&opts.Yes, "yes", false, "跳过交互确认")
	fs.BoolVar(&opts.AllowDirty, "allow-dirty", false, "允许工作区有未提交改动")
	fs.BoolVar(&opts.InPlace, "in-place", false, "不新建分支，直接在当前分支升级")
	fs.BoolVar(&opts.PR, "pr", false, "推送分支并创建 PR（需要 gh 或手动打开链接）")
	fs.StringVar(&opts.PRBase, "pr-base", "main", "PR 目标分支")
	fs.BoolVar(&opts.NoVerify, "no-verify", false, "跳过 go build/vet 校验")
	fs.BoolVar(&opts.WithTests, "with-tests", false, "额外运行 go test ./...")
	fs.BoolVar(&opts.WithFrontend, "with-frontend", false, "额外运行前端 npm ci / npm run build")
	fs.BoolVar(&opts.NoTidy, "no-tidy", false, "不自动运行 go mod tidy")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "只打印计划，不写入工作区")
	fs.BoolVar(&opts.NoCommit, "no-commit", false, "写入和校验后不自动提交")
	fs.BoolVar(&opts.AllowMigrationConflict, "allow-migration-conflict", false, "迁移编号冲突时仍然继续（不推荐）")
	fs.StringVar(&opts.Branch, "branch", "", "升级分支名（默认 chore/upgrade-xadmin-<版本>）")
	fs.BoolVar(&sf.list, "list", false, "列出全部文件变更")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if opts.PR && opts.InPlace {
		fmt.Fprintln(os.Stderr, "--pr 与 --in-place 不能同时使用")
		return 1
	}
	if opts.PR && opts.NoCommit {
		fmt.Fprintln(os.Stderr, "--pr 需要提交，不能与 --no-commit 同时使用")
		return 1
	}
	prepCtx, cancel := context.WithTimeout(context.Background(), sf.opts.Timeout)
	res, plan, warning, err := PrepareUpgrade(prepCtx, sf.opts)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成计划失败: %v\n", err)
		return 1
	}
	if warning != "" {
		fmt.Fprintf(os.Stderr, "[warn] %s\n", warning)
	}
	if !plan.HasWork() {
		if name == "upgrade" {
			fmt.Fprintf(os.Stdout, "✔ 已是最新：%s -> %s，无需升级\n",
				labelWithCommit(plan.FromRef, plan.FromCommit), labelWithCommit(plan.ToRef, plan.ToCommit))
		} else {
			PrintPlan(os.Stdout, plan, sf.list, false)
		}
		return 0
	}
	if opts.DryRun {
		PrintPlan(os.Stdout, plan, sf.list, false)
		fmt.Fprintln(os.Stdout, "\n--dry-run：未写入任何文件")
		return 0
	}
	if res.Git == nil {
		fmt.Fprintln(os.Stderr, "升级需要 git 仓库，请先 git init 或克隆完整仓库")
		return 1
	}
	if !opts.AllowDirty {
		if clean, lines, err := res.Git.IsClean(); err == nil && !clean {
			fmt.Fprintf(os.Stderr, "检测到可升级：%s -> %s，但工作区有未提交改动。请先提交/暂存，或加 --allow-dirty：\n",
				labelWithCommit(plan.FromRef, plan.FromCommit), labelWithCommit(plan.ToRef, plan.ToCommit))
			printDirtyWorktree(lines)
			return 1
		}
	}
	if plan.HasMigrationErrors() && !opts.AllowMigrationConflict {
		PrintPlan(os.Stdout, plan, sf.list, false)
		fmt.Fprintln(os.Stderr, "\n存在迁移编号冲突，升级已停止；请先按 docs/tool/upgrade.md 处理，或明确加 --allow-migration-conflict。")
		return 1
	}
	PrintPlan(os.Stdout, plan, sf.list, false)
	if !opts.Yes {
		if !isTerminal(os.Stdin) {
			fmt.Fprintln(os.Stderr, "非交互环境请加 --yes 确认升级")
			return 1
		}
		prompt := "确认执行升级？"
		if plan.HasConflicts() {
			prompt = fmt.Sprintf("仍有 %d 个冲突，将写入冲突标记/.upstream 文件，继续？", plan.ComputeSummary().Conflict)
		}
		if !confirm(prompt) {
			fmt.Fprintln(os.Stdout, "已取消")
			return 0
		}
	}
	if err := ExecuteUpgrade(context.Background(), res, plan, opts, os.Stdout); err != nil {
		if errors.Is(err, ErrConflicts) {
			return 2
		}
		fmt.Fprintf(os.Stderr, "\n升级失败: %v\n", err)
		return 1
	}
	if opts.NoCommit {
		fmt.Fprintln(os.Stdout, "\n✔ 升级文件已写入工作区但未提交")
		fmt.Fprintln(os.Stdout, "下一步：git diff 审查后手动 git add / git commit")
		return 0
	}
	fmt.Fprintln(os.Stdout, "\n✔ 升级完成")
	fmt.Fprintln(os.Stdout, "下一步：")
	fmt.Fprintln(os.Stdout, "  1. 审查 git show --stat HEAD")
	fmt.Fprintln(os.Stdout, "  2. 备份数据库后再启动新版（迁移会在启动时自动执行）")
	fmt.Fprintln(os.Stdout, "  3. 验证登录、文件、导出与定时任务")
	return 0
}

func runUpgradeVerify(args []string) int {
	fs := flag.NewFlagSet("upgrade verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".", "项目目录")
	withTests := fs.Bool("with-tests", false, "运行 go test ./...")
	withFrontend := fs.Bool("with-frontend", false, "运行前端 npm ci / npm run build")
	tidy := fs.Bool("tidy", false, "先运行 go mod tidy")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	workDir, err := absProjectDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "目录无效: %v\n", err)
		return 1
	}
	if err := VerifyProject(context.Background(), workDir, VerifyOptions{
		Tidy:     *tidy,
		Tests:    *withTests,
		Frontend: *withFrontend,
	}, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "\n校验失败: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "\n✔ 校验通过")
	return 0
}

func runUpgradeRollback(args []string) int {
	fs := flag.NewFlagSet("upgrade rollback", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".", "项目目录")
	id := fs.String("id", "", "备份目录名（默认最近一次）")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	workDir, err := absProjectDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "目录无效: %v\n", err)
		return 1
	}
	git, err := NewGit(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "回滚失败: %v\n", err)
		return 1
	}
	var backupDir string
	if *id == "" {
		backupDir, err = LatestBackup(git)
	} else if filepath.IsAbs(*id) {
		backupDir = *id
	} else {
		gitDir, e := git.GitDir()
		if e != nil {
			fmt.Fprintf(os.Stderr, "回滚失败: %v\n", e)
			return 1
		}
		backupDir = filepath.Join(gitDir, "x-admin", "backups", *id)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "回滚失败: %v\n", err)
		return 1
	}
	n, manifest, err := Rollback(git, workDir, backupDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "回滚失败: %v\n", err)
		return 1
	}
	fmt.Printf("✔ 已恢复 %d 个文件（备份：%s）\n", n, backupDir)
	if manifest.Branch != "" {
		fmt.Printf("  升级分支：%s", manifest.Branch)
		if manifest.OriginalBranch != "" {
			fmt.Printf("，原分支：%s", manifest.OriginalBranch)
		}
		fmt.Println()
	}
	if manifest.Branch != "" && manifest.OriginalBranch != "" &&
		manifest.Branch != manifest.OriginalBranch && git.Branch() == manifest.Branch &&
		git.BranchExists(manifest.OriginalBranch) {
		if n, err := git.CommitCountBetween(manifest.OriginalBranch, "HEAD"); err == nil && n == 0 {
			if err := git.Switch(manifest.OriginalBranch); err == nil {
				fmt.Printf("✔ 已切回原分支 %s\n", manifest.OriginalBranch)
				fmt.Println("工作区已恢复到升级前状态。")
				return 0
			}
		}
	}
	fmt.Println("请运行 git status 检查工作区。")
	fmt.Println("如果升级已经产生 commit，执行 git reset HEAD^ 回到升级前；需要时再切回原分支。")
	return 0
}

func runCreate(args []string) int {
	if len(args) == 0 {
		fmt.Print(createHelp)
		return 0
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Print(createHelp)
		return 0
	case "api":
		return runCreateAPI(args[1:])
	case "handler":
		return runCreateHandler(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "未知 create 子命令 %q\n\n%s", args[0], createHelp)
		return 1
	}
}

func runCreateAPI(args []string) int {
	fs := flag.NewFlagSet("create api", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts CreateAPIOptions
	var jsonOut bool
	fs.StringVar(&opts.Dir, "dir", ".", "项目目录")
	fs.StringVar(&opts.Name, "name", "", "新 API 面名称，例如 partner")
	fs.StringVar(&opts.Version, "version", "v1", "API 版本，例如 v1")
	fs.StringVar(&opts.Prefix, "prefix", "", "路径前缀，默认 /<name>/<version>")
	fs.BoolVar(&opts.Auth, "auth", false, "生成完整 JWT 认证：注册、登录、刷新、登出、me")
	fs.StringVar(&opts.Table, "table", "", "认证用户表名，默认 <name>_users")
	fs.StringVar(&opts.Migration, "migration", "", "认证表的迁移编号，默认取当前最大编号 + 1")
	fs.BoolVar(&opts.Force, "force", false, "覆盖已存在的生成文件")
	fs.BoolVar(&jsonOut, "json", false, "输出机器可读 JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	result, err := CreateAPI(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 API 面失败: %v\n", err)
		return 1
	}
	if jsonOut {
		if err := PrintJSON(os.Stdout, result); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printCreateResult(result)
	return 0
}

func runCreateHandler(args []string) int {
	fs := flag.NewFlagSet("create handler", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts CreateHandlerOptions
	var jsonOut bool
	fs.StringVar(&opts.Dir, "dir", ".", "项目目录")
	fs.StringVar(&opts.API, "api", "", "API 面名称，例如 admin 或已生成的 client/merchant")
	fs.StringVar(&opts.Version, "version", "v1", "API 版本，例如 v1")
	fs.StringVar(&opts.Name, "name", "", "handler 资源名，例如 order")
	fs.StringVar(&opts.Path, "path", "", "路由路径，默认 /<name>")
	fs.StringVar(&opts.Permission, "permission", "", "权限码前缀，例如 system:order；admin 默认 system:<name>，none 表示不生成")
	fs.BoolVar(&opts.Force, "force", false, "覆盖已存在的生成文件")
	fs.BoolVar(&jsonOut, "json", false, "输出机器可读 JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	result, err := CreateHandler(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 handler 失败: %v\n", err)
		return 1
	}
	if jsonOut {
		if err := PrintJSON(os.Stdout, result); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printCreateResult(result)
	return 0
}

func printCreateResult(result *CreateResult) {
	printCreateResultTo(os.Stdout, result)
}

// printCreateResultTo 把生成结果写进任意 writer，CLI 与 TUI 共用。
func printCreateResultTo(w io.Writer, result *CreateResult) {
	if result == nil {
		return
	}
	switch result.Kind {
	case "api":
		fmt.Fprintf(w, "✔ 已创建 API 面 %s %s\n", result.Name, result.Version)
	case "handler":
		fmt.Fprintf(w, "✔ 已创建 handler %s/%s/%s\n", result.API, result.Version, result.Name)
	default:
		fmt.Fprintf(w, "✔ 已生成 %s\n", result.Kind)
	}
	for _, f := range result.Files {
		fmt.Fprintf(w, "  %s\n", f)
	}
	if result.Auth {
		fmt.Fprintf(w, "\n认证：JWT（表 %s，迁移 %s）\n", result.Table, result.Migration)
	}
	if result.Config != "" {
		fmt.Fprintln(w, "\n配置（加入 configs/config.yaml）：")
		fmt.Fprint(w, result.Config)
	}
	if len(result.NextSteps) > 0 {
		fmt.Fprintln(w, "\n下一步：")
		for i, step := range result.NextSteps {
			fmt.Fprintf(w, "  %d. %s\n", i+1, step)
		}
	}
}

func runFixModule(args []string) int {
	fs := flag.NewFlagSet("fix-module", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", ".", "项目目录")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	workDir, err := absProjectDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "目录无效: %v\n", err)
		return 1
	}
	module, err := ModulePath(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取 go.mod 失败: %v\n", err)
		return 1
	}
	if module == "x-admin" {
		fmt.Println("module 是 x-admin，无需适配。")
		return 0
	}
	changed, err := FixModulePath(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "适配失败: %v\n", err)
		return 1
	}
	if changed {
		fmt.Printf("✔ 已把工具指令与 import 适配到 module %s\n", module)
	} else {
		fmt.Printf("✔ module %s 已经适配，无需修改\n", module)
	}
	return 0
}

func runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts DoctorOptions
	var jsonOut bool
	fs.StringVar(&opts.Dir, "dir", ".", "项目目录")
	fs.StringVar(&opts.Repo, "repo", DefaultRepo, "上游仓库 owner/name")
	fs.StringVar(&opts.Token, "token", "", "GitHub token")
	fs.BoolVar(&opts.Online, "online", false, "检查 GitHub 最新版本")
	fs.BoolVar(&jsonOut, "json", false, "输出机器可读 JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ctx := context.Background()
	if opts.Online {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}
	checks, err := RunDoctor(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "体检失败: %v\n", err)
		return 1
	}
	if jsonOut {
		if err := PrintJSON(os.Stdout, checks); err != nil {
			fmt.Fprintf(os.Stderr, "输出 JSON: %v\n", err)
			return 1
		}
	} else {
		PrintDoctor(os.Stdout, checks)
	}
	for _, c := range checks {
		if c.Status == CheckFail {
			return 1
		}
	}
	return 0
}

func printDirtyWorktree(lines []string) {
	for i, line := range lines {
		if i >= 8 {
			fmt.Fprintln(os.Stderr, "  ...")
			break
		}
		fmt.Fprintln(os.Stderr, "  "+line)
	}
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	var answer string
	if _, err := fmt.Fscanln(os.Stdin, &answer); err != nil {
		return false
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes" || answer == "是"
}
