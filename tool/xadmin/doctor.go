package xadmin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// CheckStatus 是体检项的状态。
type CheckStatus string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

// Check 是一条体检结果。
type Check struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Detail string      `json:"detail,omitempty"`
}

// DoctorOptions 是 doctor 的参数。
type DoctorOptions struct {
	Dir    string
	Repo   string
	Token  string
	Online bool
}

var goDirectiveRe = regexp.MustCompile(`(?m)^go\s+([0-9]+(?:\.[0-9]+){0,2})\s*$`)

// RunDoctor 对项目做一组只读检查。
func RunDoctor(ctx context.Context, opts DoctorOptions) ([]Check, error) {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	workDir, err := absProjectDir(dir)
	if err != nil {
		return nil, err
	}
	repo := opts.Repo
	if repo == "" {
		repo = DefaultRepo
	}
	var checks []Check
	add := func(name string, status CheckStatus, detail string) {
		checks = append(checks, Check{Name: name, Status: status, Detail: detail})
	}

	git, gitErr := NewGit(workDir)
	if gitErr != nil {
		add("git 仓库", CheckFail, gitErr.Error())
	} else {
		add("git 仓库", CheckOK, git.Dir)
		if clean, lines, err := git.IsClean(); err != nil {
			add("工作区", CheckFail, err.Error())
		} else if clean {
			add("工作区", CheckOK, "干净")
		} else {
			add("工作区", CheckWarn, fmt.Sprintf("有 %d 项未提交改动", len(lines)))
		}
	}

	state, stateErr := ReadState(workDir)
	if stateErr != nil {
		add("框架状态", CheckFail, stateErr.Error())
	} else if state == nil {
		add("框架状态", CheckWarn, "未找到 "+StatePath+"；将回退到 "+VersionPath+" 或内容指纹")
	} else {
		detail := state.Ref
		if state.Commit != "" {
			detail += " (" + ShortSHA(state.Commit) + ")"
		}
		add("框架状态", CheckOK, detail)
		if state.Repo != "" && state.Repo != repo {
			add("上游仓库", CheckWarn, fmt.Sprintf("%s 与 --repo %s 不一致", state.Repo, repo))
		}
	}

	version, versionErr := ReadFrameworkVersion(workDir)
	switch {
	case versionErr != nil:
		add("版本标记", CheckFail, versionErr.Error())
	case version == "":
		add("版本标记", CheckWarn, "未找到 "+VersionPath+"；复制源码时建议保留该文件")
	default:
		add("版本标记", CheckOK, version)
		if state != nil && state.Ref != "" {
			consistent := state.Ref == version || (state.Commit != "" && state.Commit == version)
			if !consistent {
				add("版本一致性", CheckWarn, fmt.Sprintf("%s=%s，%s=%s", StatePath, state.Ref, VersionPath, version))
			}
		}
	}
	if git != nil {
		current := ""
		if state != nil {
			current = state.Ref
		}
		if current == "" {
			current = version
		}
		if current == "" {
			current = git.DescribeTag()
		}
		detail := git.Describe()
		if detail == "" {
			detail = "无 release tag"
		}
		add("当前版本", CheckOK, fmt.Sprintf("%s（git describe: %s）", current, detail))
	} else {
		add("当前版本", CheckWarn, "没有 git，无法从 tag 推断版本")
	}

	// go.mod 的 go 指令与运行时版本。
	goModPath := filepath.Join(workDir, "go.mod")
	if data, err := os.ReadFile(goModPath); err != nil {
		add("Go 版本", CheckFail, "读取 go.mod 失败: "+err.Error())
	} else if m := goDirectiveRe.FindStringSubmatch(string(data)); len(m) == 2 {
		required := m[1]
		current := strings.TrimPrefix(runtime.Version(), "go")
		if compareGoVersion(current, required) < 0 {
			add("Go 版本", CheckFail, fmt.Sprintf("运行时 go%s 低于 go.mod 要求的 %s", current, required))
		} else {
			add("Go 版本", CheckOK, fmt.Sprintf("go%s >= %s", current, required))
		}
	} else {
		add("Go 版本", CheckWarn, "go.mod 中没有找到 go 指令")
	}

	// 自定义 module 路径时，go tool 指令和 cmd/xadmin 的 import 需要同步。
	if module, err := ModulePath(workDir); err == nil && module != "" && module != "x-admin" {
		if detail, _ := CheckModulePath(workDir); detail != "" {
			add("module 适配", CheckWarn, detail)
		} else {
			add("module 适配", CheckOK, module)
		}
	}

	// 配置差异。
	examplePath := filepath.Join(workDir, "configs", "config.example.yaml")
	userPath := filepath.Join(workDir, "configs", "config.yaml")
	example, exampleErr := os.ReadFile(examplePath)
	user, userErr := os.ReadFile(userPath)
	switch {
	case exampleErr != nil:
		add("配置模板", CheckFail, "读取 configs/config.example.yaml 失败")
	case userErr != nil:
		add("本地配置", CheckWarn, "未找到 configs/config.yaml（部署时再复制模板）")
	default:
		exampleFlat, err1 := FlattenYAML(example)
		userFlat, err2 := FlattenYAML(user)
		if err1 != nil || err2 != nil {
			add("配置差异", CheckFail, "解析 YAML 失败")
		} else {
			var missing []string
			for key := range exampleFlat {
				if _, ok := userFlat[key]; !ok {
					missing = append(missing, key)
				}
			}
			sort.Strings(missing)
			if len(missing) == 0 {
				add("配置差异", CheckOK, "本地配置覆盖了模板全部键")
			} else {
				detail := fmt.Sprintf("本地缺少 %d 个模板键：%s", len(missing), strings.Join(limitStrings(missing, 8), ", "))
				if len(missing) > 8 {
					detail += " ..."
				}
				add("配置差异", CheckWarn, detail)
			}
			removed, _ := ConfigRemovedKeys(example, user)
			removed = filterGeneratedSurfaceKeys(removed)
			if len(removed) > 0 {
				add("废弃配置", CheckWarn, fmt.Sprintf("本地有 %d 个模板已不存在的键：%s", len(removed), strings.Join(limitStrings(configKeys(removed), 8), ", ")))
			}
		}
	}

	// 迁移文件。
	if tree, err := LoadTree(workDir); err != nil {
		add("迁移文件", CheckFail, err.Error())
	} else {
		files := parseMigrationFiles(tree)
		if len(files) == 0 {
			add("迁移文件", CheckWarn, "migrations/ 下没有迁移文件")
		} else {
			problems := MigrationPairs(tree)
			byNumber := make(map[int]map[string]bool)
			for _, mf := range files {
				if mf.Dir != "up" {
					continue
				}
				if byNumber[mf.Number] == nil {
					byNumber[mf.Number] = make(map[string]bool)
				}
				byNumber[mf.Number][mf.Name] = true
			}
			var duplicates []string
			for num, names := range byNumber {
				if len(names) > 1 {
					duplicates = append(duplicates, fmt.Sprintf("%06d", num))
				}
			}
			sort.Strings(duplicates)
			switch {
			case len(duplicates) > 0:
				add("迁移文件", CheckFail, "重复编号: "+strings.Join(duplicates, ", "))
			case len(problems) > 0:
				add("迁移文件", CheckWarn, strings.Join(limitStrings(problems, 5), "；"))
			default:
				add("迁移文件", CheckOK, fmt.Sprintf("%d 个文件，up/down 成对，最新编号 %06d", len(files), files[len(files)-1].Number))
			}
		}
	}

	// 前端工具链。
	frontendPkg := filepath.Join(workDir, "frontend", "package.json")
	if _, err := os.Stat(frontendPkg); err != nil {
		add("前端", CheckWarn, "未找到 frontend/package.json")
	} else {
		var missing []string
		if _, err := exec.LookPath("node"); err != nil {
			missing = append(missing, "node")
		}
		if _, err := exec.LookPath("npm"); err != nil {
			missing = append(missing, "npm")
		}
		if len(missing) > 0 {
			add("前端", CheckWarn, "缺少命令: "+strings.Join(missing, ", "))
		} else {
			add("前端", CheckOK, "node/npm 可用")
		}
	}

	if opts.Online {
		client := NewGitHubClient(repo, tokenOrDefault(opts.Token))
		release, err := client.LatestRelease(ctx)
		if err != nil {
			add("上游版本", CheckWarn, err.Error())
		} else {
			current := ""
			if state != nil {
				current = state.Ref
			}
			if current == "" {
				current = version
			}
			if current == "" && git != nil {
				current = git.DescribeTag()
			}
			switch {
			case current == "":
				add("上游版本", CheckWarn, "无法确定本地版本；最新为 "+release.TagName)
			case IsUpToDate(current, release.TagName):
				add("上游版本", CheckOK, "已是最新 "+release.TagName)
			default:
				add("上游版本", CheckWarn, fmt.Sprintf("可升级：%s -> %s", current, release.TagName))
			}
		}
	}

	return checks, nil
}

func tokenOrDefault(token string) string {
	if strings.TrimSpace(token) != "" {
		return strings.TrimSpace(token)
	}
	return TokenFromEnv()
}

func compareGoVersion(current, required string) int {
	parse := func(s string) []int {
		parts := strings.Split(s, ".")
		out := make([]int, len(parts))
		for i, p := range parts {
			n, _ := strconv.Atoi(p)
			out[i] = n
		}
		return out
	}
	a, b := parse(current), parse(required)
	for i := 0; i < len(a) || i < len(b); i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func limitStrings(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func configKeys(in []ConfigChange) []string {
	out := make([]string, 0, len(in))
	for _, c := range in {
		out = append(out, c.Key)
	}
	return out
}
