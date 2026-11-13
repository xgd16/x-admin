package xadmin

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var migrationNameRe = regexp.MustCompile(`^migrations/(\d+)_(.+)\.(up|down)\.sql$`)

type migrationFile struct {
	Path   string
	Number int
	Name   string
	Dir    string // up | down
}

func parseMigrationFiles(tree Tree) []migrationFile {
	var files []migrationFile
	for p := range tree {
		m := migrationNameRe.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		files = append(files, migrationFile{Path: p, Number: n, Name: m[2], Dir: m[3]})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Number == files[j].Number {
			return files[i].Path < files[j].Path
		}
		return files[i].Number < files[j].Number
	})
	return files
}

// detectMigrationIssues 检测上游新增迁移与本地迁移的编号关系。
//
// 为什么重要：golang-migrate 只执行版本号大于 schema_migrations 当前值的迁移。
// 如果本地迁移已经执行到较大编号，上游后补的较小号会被直接跳过。
func detectMigrationIssues(base, target, local Tree) []MigrationIssue {
	basePaths := make(map[string]bool, len(base))
	for p := range base {
		basePaths[p] = true
	}
	upstreamNew := make(map[string]migrationFile)
	for _, mf := range parseMigrationFiles(target) {
		if !basePaths[mf.Path] {
			upstreamNew[mf.Path] = mf
		}
	}
	localNew := make(map[string]migrationFile)
	localMax := 0
	for _, mf := range parseMigrationFiles(local) {
		if basePaths[mf.Path] {
			continue
		}
		localNew[mf.Path] = mf
		if mf.Number > localMax {
			localMax = mf.Number
		}
	}
	if len(upstreamNew) == 0 || len(localNew) == 0 {
		return nil
	}

	// 同一编号的 up 文件是最明确的冲突。
	localByNumber := make(map[int][]migrationFile)
	for _, mf := range localNew {
		if mf.Dir != "up" {
			continue
		}
		localByNumber[mf.Number] = append(localByNumber[mf.Number], mf)
	}
	var issues []MigrationIssue
	upstreamNumbers := make(map[int]bool)
	minUpstream := 0
	for _, mf := range upstreamNew {
		if mf.Dir != "up" {
			continue
		}
		upstreamNumbers[mf.Number] = true
		if minUpstream == 0 || mf.Number < minUpstream {
			minUpstream = mf.Number
		}
		for _, lm := range localByNumber[mf.Number] {
			issues = append(issues, MigrationIssue{
				Severity: "error",
				Number:   mf.Number,
				Local:    lm.Path,
				Upstream: mf.Path,
				Message:  fmt.Sprintf("本地迁移与上游新增迁移编号重复（%06d）", mf.Number),
			})
		}
	}

	// 没有精确重复，但上游新增编号低于本地最大编号，存在被跳过的风险。
	if minUpstream > 0 && minUpstream < localMax {
		var lower []string
		for n := range upstreamNumbers {
			if n < localMax {
				lower = append(lower, fmt.Sprintf("%06d", n))
			}
		}
		sort.Strings(lower)
		if len(lower) > 0 {
			issues = append(issues, MigrationIssue{
				Severity: "warning",
				Number:   minUpstream,
				Message: fmt.Sprintf(
					"上游新增迁移 %s 的编号低于本地最大编号 %06d；如果本地迁移已在数据库执行，golang-migrate 会跳过这些上游迁移。建议把上游新增迁移按原顺序重编号为 %06d 及之后（up/down 成对修改），并先备份数据库。详见 docs/tool/upgrade.md。",
					strings.Join(lower, ", "), localMax, localMax+1,
				),
			})
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Number != issues[j].Number {
			return issues[i].Number < issues[j].Number
		}
		return issues[i].Severity < issues[j].Severity
	})
	return issues
}

// MigrationPairs 检查 up/down 是否成对，返回问题描述。doctor 使用。
func MigrationPairs(tree Tree) []string {
	type pair struct {
		up   bool
		down bool
	}
	pairs := make(map[string]*pair)
	for _, mf := range parseMigrationFiles(tree) {
		key := fmt.Sprintf("%06d_%s", mf.Number, mf.Name)
		p := pairs[key]
		if p == nil {
			p = &pair{}
			pairs[key] = p
		}
		if mf.Dir == "up" {
			p.up = true
		} else {
			p.down = true
		}
	}
	var problems []string
	for key, p := range pairs {
		if !p.up {
			problems = append(problems, key+" 缺少 .up.sql")
		}
		if !p.down {
			problems = append(problems, key+" 缺少 .down.sql")
		}
	}
	sort.Strings(problems)
	return problems
}

// NextMigrationNumber 返回当前迁移目录的下一个编号，供代码生成使用。
func NextMigrationNumber(tree Tree) int {
	max := 0
	for _, mf := range parseMigrationFiles(tree) {
		if mf.Number > max {
			max = mf.Number
		}
	}
	return max + 1
}
