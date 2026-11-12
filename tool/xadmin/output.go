package xadmin

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func printSection(w io.Writer, title string) {
	fmt.Fprintf(w, "\n%s\n", title)
	fmt.Fprintln(w, strings.Repeat("─", 52))
}

// PrintCheck 输出 upgrade check 结果。
func PrintCheck(w io.Writer, r *CheckResult, fullNotes bool) {
	printSection(w, "x-admin 升级检查")
	fmt.Fprintf(w, "  仓库      %s\n", r.Repo)
	current := r.Current
	if current == "" {
		current = "未知"
	}
	fmt.Fprintf(w, "  当前基准  %s", current)
	if r.CurrentSource != "" {
		fmt.Fprintf(w, "（来源：%s）", r.CurrentSource)
	}
	if r.CurrentDetail != "" && r.CurrentDetail != r.Current && r.CurrentSource != "内容指纹" {
		fmt.Fprintf(w, "（git describe: %s）", r.CurrentDetail)
	}
	fmt.Fprintln(w)
	latest := r.Latest
	if r.LatestCommit != "" {
		latest += " (" + ShortSHA(r.LatestCommit) + ")"
	}
	fmt.Fprintf(w, "  最新版本  %s\n", latest)
	if !r.PublishedAt.IsZero() {
		fmt.Fprintf(w, "  发布时间  %s\n", r.PublishedAt.Local().Format("2006-01-02 15:04"))
	}
	if r.ReleaseURL != "" {
		fmt.Fprintf(w, "  发布页    %s\n", r.ReleaseURL)
	}
	switch {
	case r.Current == "":
		fmt.Fprintln(w, "\n⚠ 无法确定当前版本，请用 --from 指定基准，或创建 "+VersionPath)
		if r.DetectError != "" {
			fmt.Fprintf(w, "  内容指纹识别失败：%s\n", r.DetectError)
			fmt.Fprintln(w, "  可以设置 GITHUB_TOKEN 后重试，或运行 go tool xadmin upgrade init --from <版本>")
		}
	case !r.Comparable:
		fmt.Fprintln(w, "\n? 当前版本不是 release tag，无法直接比较；请运行 go tool xadmin upgrade plan 查看差异")
	case r.UpToDate:
		fmt.Fprintln(w, "\n✔ 已经是最新版本")
	default:
		fmt.Fprintln(w, "\n↑ 有新版本可用")
		fmt.Fprintln(w, "  下一步：go tool xadmin upgrade plan")
	}
	if strings.TrimSpace(r.Notes) != "" {
		fmt.Fprintln(w, "\n发布说明：")
		lines := strings.Split(strings.TrimSpace(r.Notes), "\n")
		maxLines := 18
		if fullNotes {
			maxLines = len(lines)
		}
		for i, line := range lines {
			if i >= maxLines {
				fmt.Fprintf(w, "  ...（还有 %d 行，使用 --notes 查看完整内容）\n", len(lines)-maxLines)
				break
			}
			fmt.Fprintln(w, "  "+line)
		}
	}
}

// PrintPlan 输出文件级升级计划。
func PrintPlan(w io.Writer, plan *Plan, list bool, showNext bool) {
	s := plan.ComputeSummary()
	printSection(w, "x-admin 升级计划")
	fmt.Fprintf(w, "  仓库      %s\n", plan.Repo)
	fmt.Fprintf(w, "  工作目录  %s\n", plan.WorkDir)
	fmt.Fprintf(w, "  基准      %s", labelWithCommit(plan.FromRef, plan.FromCommit))
	if plan.FromSource != "" {
		fmt.Fprintf(w, "  [%s]", plan.FromSource)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  目标      %s\n", labelWithCommit(plan.ToRef, plan.ToCommit))
	if vFrom, okFrom := ParseVersion(plan.FromRef); okFrom {
		if vTo, okTo := ParseVersion(plan.ToRef); okTo && CompareVersion(plan.ToRef, plan.FromRef) < 0 {
			fmt.Fprintf(w, "  ⚠ 目标版本 %s 低于基准 %s，这是降级操作，请确认。\n", FormatVersion(vTo), FormatVersion(vFrom))
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  自动更新  %d\n", s.Update)
	fmt.Fprintf(w, "  新增      %d\n", s.Add)
	fmt.Fprintf(w, "  上游删除  %d\n", s.Delete)
	fmt.Fprintf(w, "  本地保留  %d\n", s.Keep)
	fmt.Fprintf(w, "  冲突      %d\n", s.Conflict)

	if !plan.HasWork() {
		fmt.Fprintln(w, "\n✔ 没有需要应用的变更")
		return
	}

	var conflicts, others []Change
	for _, ch := range plan.Changes {
		switch ch.Kind {
		case ChangeConflict:
			conflicts = append(conflicts, ch)
		case ChangeUpdate, ChangeAdd, ChangeDelete:
			others = append(others, ch)
		}
	}
	if len(conflicts) > 0 {
		printSection(w, "需要手工处理")
		for _, ch := range conflicts {
			extra := ch.Path
			if ch.UpstreamOnly {
				extra = UpstreamPath(ch.Path)
			}
			fmt.Fprintf(w, "  ! %-8s %s\n", ch.Kind, extra)
			if ch.Reason != "" {
				fmt.Fprintf(w, "           %s\n", ch.Reason)
			}
		}
	}
	if len(plan.Migrations) > 0 {
		printSection(w, "迁移编号")
		hasMigrationError := false
		for _, issue := range plan.Migrations {
			mark := "!"
			if issue.Severity == "error" {
				mark = "✖"
				hasMigrationError = true
			}
			fmt.Fprintf(w, "  %s %s\n", mark, issue.Message)
			if issue.Local != "" || issue.Upstream != "" {
				fmt.Fprintf(w, "           本地: %s\n           上游: %s\n", issue.Local, issue.Upstream)
			}
		}
		if hasMigrationError {
			fmt.Fprintln(w, "\n  ⚠ apply 会拒绝执行；请先处理迁移编号，或明确使用 --allow-migration-conflict。")
		}
	}
	if len(plan.ConfigAdded) > 0 {
		printSection(w, "本地配置缺少的新键")
		for _, c := range plan.ConfigAdded {
			fmt.Fprintf(w, "  + %s = %s\n", c.Key, c.Value)
		}
		fmt.Fprintln(w, "  → 新键已包含在 configs/config.example.yaml，请按需同步到 configs/config.yaml")
	}
	if list && len(others) > 0 {
		printSection(w, "文件变更")
		for _, ch := range others {
			fmt.Fprintf(w, "  %-8s %s\n", ch.Kind, ch.Path)
		}
	} else if len(others) > 0 {
		fmt.Fprintln(w, "\n  使用 --list 查看全部文件变更")
	}

	if !showNext {
		return
	}
	switch {
	case plan.HasMigrationErrors():
		fmt.Fprintln(w, "\n下一步：先处理迁移编号冲突，再运行 go tool xadmin upgrade apply --yes")
		fmt.Fprintln(w, "  如确认要强制继续，加 --allow-migration-conflict。")
	case plan.HasConflicts():
		fmt.Fprintln(w, "\n下一步：go tool xadmin upgrade apply --yes")
		fmt.Fprintln(w, "  冲突文件会写入冲突标记或 .upstream 文件，不会静默覆盖。")
	default:
		fmt.Fprintln(w, "\n下一步：go tool xadmin upgrade apply --yes")
	}
}

func labelWithCommit(ref, commit string) string {
	if ref == "" {
		if commit == "" {
			return "未知"
		}
		return ShortSHA(commit)
	}
	if commit == "" {
		return ref
	}
	if IsSHA(ref) {
		return ShortSHA(ref)
	}
	return ref + " (" + ShortSHA(commit) + ")"
}

// PrintDoctor 输出体检结果。
func PrintDoctor(w io.Writer, checks []Check) {
	printSection(w, "x-admin 项目体检")
	for _, c := range checks {
		mark := "✔"
		switch c.Status {
		case CheckWarn:
			mark = "⚠"
		case CheckFail:
			mark = "✖"
		}
		fmt.Fprintf(w, "  %s %-10s %s\n", mark, c.Name, c.Detail)
	}
}

// PrintPlanJSON 输出机器可读的计划。
func PrintPlanJSON(w io.Writer, plan *Plan) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}

// PrintJSON 输出任意 JSON 结构。
func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
