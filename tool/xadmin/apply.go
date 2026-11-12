package xadmin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ErrConflicts 表示升级写入了冲突标记，需要人工处理后才能提交。
var ErrConflicts = errors.New("存在需要手工处理的冲突")

// ApplyOptions 是 upgrade apply 的行为开关。
type ApplyOptions struct {
	Yes          bool
	AllowDirty   bool
	InPlace      bool
	PR           bool
	PRBase       string
	NoVerify     bool
	WithTests    bool
	WithFrontend bool
	NoTidy       bool
	DryRun       bool
	NoCommit     bool
	Branch       string
	// AllowMigrationConflict 允许在迁移编号冲突未解决时继续。
	AllowMigrationConflict bool
}

// VerifyOptions 是构建校验参数。
type VerifyOptions struct {
	Tidy     bool
	Tests    bool
	Frontend bool
}

// PrepareUpgrade 解析版本、下载源码并生成升级计划。
func PrepareUpgrade(ctx context.Context, opts SourceOptions) (*Resolved, *Plan, string, error) {
	res, err := ResolveSources(ctx, opts)
	if err != nil {
		return nil, nil, "", err
	}
	merger, warning := NewMerger()
	plan := ComputePlan(res.Client.Repo, res.WorkDir, res.FromRef, res.FromCommit, res.ToRef, res.ToCommit, res.Base, res.Target, res.Local, merger)
	plan.FromSource = res.FromSource
	EnrichPlan(plan, res.Base, res.Target, res.Local, res.WorkDir)
	return res, plan, warning, nil
}

// ExecuteUpgrade 把计划写入工作区，完成校验、提交和可选的 PR。
func ExecuteUpgrade(ctx context.Context, res *Resolved, plan *Plan, opts ApplyOptions, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	if res.Git == nil {
		return fmt.Errorf("升级需要 git 仓库，请先 git init 或克隆完整仓库")
	}
	if opts.PR && opts.InPlace {
		return fmt.Errorf("--pr 与 --in-place 不能同时使用")
	}
	if opts.PR && opts.NoCommit {
		return fmt.Errorf("--pr 需要提交，不能与 --no-commit 同时使用")
	}
	if !opts.AllowMigrationConflict && plan.HasMigrationErrors() {
		return fmt.Errorf("存在迁移编号冲突，请先按 docs/tool/upgrade.md 处理；确认要强制继续时加 --allow-migration-conflict")
	}
	clean, lines, err := res.Git.IsClean()
	if err != nil {
		return err
	}
	if !clean && !opts.AllowDirty {
		var b strings.Builder
		b.WriteString("工作区有未提交的改动，升级会混入这些改动。请先提交/暂存，或加 --allow-dirty：\n")
		for i, line := range lines {
			if i >= 12 {
				b.WriteString("  ...\n")
				break
			}
			b.WriteString("  " + line + "\n")
		}
		return errors.New(b.String())
	}
	if !clean {
		fmt.Fprintln(log, "[warn] 工作区不干净，将继续执行；建议升级前先提交当前改动。")
	}

	originalBranch := res.Git.Branch()
	branch := opts.Branch
	if opts.InPlace {
		branch = originalBranch
	} else if branch == "" {
		branch = defaultBranchName(res.ToRef)
	}
	if !opts.InPlace {
		if err := res.Git.CreateBranch(branch); err != nil {
			return fmt.Errorf("创建分支 %s 失败（分支可能已存在）。如果上次 apply 留下了冲突，请先解决并 verify，或执行 rollback 清理后重试；也可用 --branch 指定新名字: %w", branch, err)
		}
		fmt.Fprintf(log, "[ok] 已创建分支 %s\n", branch)
	}

	backupDir, err := CreateBackup(res.Git, plan, res.WorkDir)
	if err != nil {
		return fmt.Errorf("创建备份: %w", err)
	}
	_ = SetBackupBranch(backupDir, branch, originalBranch)
	fmt.Fprintf(log, "[ok] 升级前备份已写入 %s\n", backupDir)

	if err := ApplyPlan(plan, res.WorkDir); err != nil {
		fmt.Fprintf(log, "\n[error] 写入升级文件失败，可执行 go tool xadmin upgrade rollback 恢复\n")
		return fmt.Errorf("写入升级文件: %w", err)
	}

	if plan.HasConflicts() {
		fmt.Fprintln(log, "\n冲突文件：")
		for _, ch := range plan.Changes {
			if ch.Kind != ChangeConflict {
				continue
			}
			if ch.UpstreamOnly {
				fmt.Fprintf(log, "  ! %s  → 上游版本写入 %s\n", ch.Path, UpstreamPath(ch.Path))
			} else {
				fmt.Fprintf(log, "  ! %s  → %s\n", ch.Path, ch.Reason)
			}
		}
		fmt.Fprintf(log, "\n请解决冲突后运行：\n  go tool xadmin upgrade verify\n然后手动提交。回滚命令：\n  go tool xadmin upgrade rollback\n")
		return ErrConflicts
	}

	if changed, err := FixModulePath(res.WorkDir); err != nil {
		fmt.Fprintf(log, "[warn] 检查 module 路径失败: %v\n", err)
	} else if changed {
		fmt.Fprintf(log, "[ok] 已按自定义 module 路径适配工具指令\n")
	}

	if opts.NoVerify {
		fmt.Fprintln(log, "[warn] 已跳过构建校验（--no-verify）")
	} else {
		verifyOpts := VerifyOptions{
			Tidy:     !opts.NoTidy && planTouches(plan, "go.mod", "go.sum"),
			Tests:    opts.WithTests,
			Frontend: opts.WithFrontend,
		}
		if err := VerifyProject(ctx, res.WorkDir, verifyOpts, log); err != nil {
			fmt.Fprintf(log, "\n[error] 校验失败，改动已保留在工作区。\n")
			fmt.Fprintf(log, "  查看改动: git diff\n")
			fmt.Fprintf(log, "  回滚:     go tool xadmin upgrade rollback\n")
			return err
		}
	}

	if err := writeUpgradeState(res); err != nil {
		return fmt.Errorf("写入 %s: %w", StatePath, err)
	}

	if !opts.NoCommit {
		msg := buildCommitMessage(res, plan)
		if err := res.Git.CommitAll(msg); err != nil {
			fmt.Fprintf(log, "\n[error] 提交失败，改动保留在工作区；可运行 go tool xadmin upgrade rollback 恢复\n")
			return fmt.Errorf("提交升级: %w", err)
		}
		fmt.Fprintf(log, "[ok] 已提交升级\n")
	} else {
		fmt.Fprintln(log, "[warn] 已跳过提交（--no-commit）")
	}

	if opts.PR {
		if err := res.Git.Push(branch); err != nil {
			return fmt.Errorf("推送分支 %s: %w", branch, err)
		}
		fmt.Fprintf(log, "[ok] 已推送 %s\n", branch)
		createPR(ctx, res, plan, branch, opts.PRBase, VerifyOptions{
			Tidy:     !opts.NoTidy && planTouches(plan, "go.mod", "go.sum"),
			Tests:    opts.WithTests,
			Frontend: opts.WithFrontend,
		}, opts.NoVerify, log)
	}

	return nil
}

func writeUpgradeState(res *Resolved) error {
	if strings.HasPrefix(res.ToRef, "local:") {
		return nil
	}
	if err := WriteState(res.WorkDir, State{
		Repo:   res.Client.Repo,
		Ref:    res.ToRef,
		Commit: res.ToCommit,
	}); err != nil {
		return err
	}
	// FRAMEWORK_VERSION 要写稳定的引用：tag 优先，分支目标写 commit，
	// 否则下次在没有状态文件的项目里会把分支解析到更新的提交。
	versionRef := res.ToRef
	if _, ok := ParseVersion(versionRef); !ok && res.ToCommit != "" {
		versionRef = res.ToCommit
	}
	return WriteFrameworkVersion(res.WorkDir, versionRef)
}

func buildCommitMessage(res *Resolved, plan *Plan) string {
	s := plan.ComputeSummary()
	var b strings.Builder
	fmt.Fprintf(&b, "chore(upgrade): x-admin %s -> %s\n\n", res.FromRef, res.ToRef)
	fmt.Fprintf(&b, "- update: %d\n- add: %d\n- delete: %d\n- keep: %d\n- conflict: %d\n",
		s.Update, s.Add, s.Delete, s.Keep, s.Conflict)
	if res.Release != nil && res.Release.HTMLURL != "" {
		fmt.Fprintf(&b, "\nRelease: %s\n", res.Release.HTMLURL)
	}
	return b.String()
}

func defaultBranchName(toRef string) string {
	s := strings.TrimPrefix(toRef, "local:")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" {
		name = "local"
	}
	return "chore/upgrade-xadmin-" + name
}

func planTouches(plan *Plan, paths ...string) bool {
	want := make(map[string]bool, len(paths))
	for _, p := range paths {
		want[p] = true
	}
	for _, ch := range plan.Changes {
		if ch.Kind == ChangeSame || ch.Kind == ChangeKeep {
			continue
		}
		if want[ch.Path] || want[UpstreamPath(ch.Path)] {
			return true
		}
	}
	return false
}

// ApplyPlan 把计划中的写入/删除动作落到工作区。
func ApplyPlan(plan *Plan, workDir string) error {
	for _, ch := range plan.Changes {
		rel, write, remove := writeTarget(ch)
		switch {
		case write:
			if ch.Data == nil {
				return fmt.Errorf("内部错误: %s 没有可写入的内容", rel)
			}
			if err := writeWorkFile(workDir, rel, ch.Data, ch.Mode); err != nil {
				return err
			}
		case remove:
			if err := removeWorkFile(workDir, rel); err != nil {
				return err
			}
		}
	}
	return nil
}

// VerifyProject 运行 go build/vet/test 以及可选的前端构建。
func VerifyProject(ctx context.Context, workDir string, opts VerifyOptions, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	if leftovers := upstreamLeftovers(workDir); len(leftovers) > 0 {
		fmt.Fprintf(log, "[warn] 发现 %d 个 .upstream 文件，确认已处理并删除后再提交：\n", len(leftovers))
		for i, p := range leftovers {
			if i >= 10 {
				fmt.Fprintf(log, "  ...\n")
				break
			}
			fmt.Fprintf(log, "  %s\n", p)
		}
	}
	if opts.Tidy {
		if err := runCommand(ctx, workDir, log, "go", "mod", "tidy"); err != nil {
			return err
		}
	}
	if err := runCommand(ctx, workDir, log, "go", "build", "./..."); err != nil {
		return err
	}
	if err := runCommand(ctx, workDir, log, "go", "vet", "./..."); err != nil {
		return err
	}
	if opts.Tests {
		if err := runCommand(ctx, workDir, log, "go", "test", "./..."); err != nil {
			return err
		}
	}
	if opts.Frontend {
		frontend := filepath.Join(workDir, "frontend")
		if _, err := os.Stat(filepath.Join(frontend, "package.json")); err != nil {
			return fmt.Errorf("未找到 frontend/package.json，无法执行前端构建")
		}
		if err := runCommand(ctx, frontend, log, "npm", "ci"); err != nil {
			return err
		}
		if err := runCommand(ctx, frontend, log, "npm", "run", "build"); err != nil {
			return err
		}
	}
	return nil
}

func upstreamLeftovers(workDir string) []string {
	tree, err := LoadTree(workDir)
	if err != nil {
		return nil
	}
	var out []string
	for p := range tree {
		if strings.HasSuffix(p, ".upstream") {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func runCommand(ctx context.Context, dir string, log io.Writer, name string, args ...string) error {
	fmt.Fprintf(log, "\n▶ %s %s\n", name, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s 失败: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func createPR(ctx context.Context, res *Resolved, plan *Plan, branch, base string, verify VerifyOptions, noVerify bool, log io.Writer) {
	if base == "" {
		base = "main"
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" {
		fmt.Fprintln(log, "[warn] 当前不在命名分支上，无法自动创建 PR")
		return
	}
	title := fmt.Sprintf("chore(upgrade): x-admin %s -> %s", res.FromRef, res.ToRef)
	body := buildPRBody(res, plan, verify, noVerify)
	if _, err := exec.LookPath("gh"); err != nil {
		printManualPR(res, branch, base, log)
		return
	}
	cmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--base", base,
		"--head", branch,
	)
	cmd.Dir = res.WorkDir
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(log, "[warn] gh pr create 失败: %v\n%s\n", err, strings.TrimSpace(string(out)))
		printManualPR(res, branch, base, log)
		return
	}
	fmt.Fprintf(log, "[ok] PR 已创建: %s\n", strings.TrimSpace(string(out)))
}

func printManualPR(res *Resolved, branch, base string, log io.Writer) {
	url := CompareURL(res.Git.OriginURL(), base, branch)
	fmt.Fprintf(log, "[info] 分支 %s 已推送。\n", branch)
	if url != "" {
		fmt.Fprintf(log, "       打开 PR: %s\n", url)
	} else {
		fmt.Fprintf(log, "       请在 GitHub 上向 %s 发起 PR。\n", base)
	}
}

func buildPRBody(res *Resolved, plan *Plan, verify VerifyOptions, noVerify bool) string {
	s := plan.ComputeSummary()
	var b strings.Builder
	b.WriteString("## x-admin 框架升级\n\n")
	fmt.Fprintf(&b, "- 版本：`%s` → `%s`\n", res.FromRef, res.ToRef)
	fmt.Fprintf(&b, "- 自动更新：%d，新增：%d，删除：%d，本地保留：%d，冲突：%d\n\n", s.Update, s.Add, s.Delete, s.Keep, s.Conflict)
	if res.Release != nil && res.Release.HTMLURL != "" {
		fmt.Fprintf(&b, "Release: %s\n\n", res.Release.HTMLURL)
	}
	b.WriteString("### 验证\n\n")
	if noVerify {
		b.WriteString("> 本次升级跳过了本地构建校验，请以 CI 结果为准。\n")
	} else {
		b.WriteString("- [x] `go build ./...`\n")
		b.WriteString("- [x] `go vet ./...`\n")
		if verify.Tests {
			b.WriteString("- [x] `go test ./...`\n")
		}
		if verify.Frontend {
			b.WriteString("- [x] `npm ci && npm run build`\n")
		}
	}
	b.WriteString("\n> 合并前请确认数据库备份与新迁移的执行顺序。\n")
	return b.String()
}
