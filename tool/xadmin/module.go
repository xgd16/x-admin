package xadmin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var moduleLineRe = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)

// ModulePath 读取项目 go.mod 的 module 路径。
func ModulePath(workDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		return "", err
	}
	m := moduleLineRe.FindStringSubmatch(string(data))
	if len(m) != 2 {
		return "", fmt.Errorf("go.mod 中没有找到 module 指令")
	}
	return m[1], nil
}

// FixModulePath 让工具适配用户自定义的 module 路径。
//
// 用户把 module x-admin 改成 example.com/myapp 后，go.mod 的 tool 指令和
// cmd/xadmin/main.go 的 import 仍然指向 x-admin/...，会导致 go tool xadmin 无法构建。
// 这里做一次机械替换，返回是否发生了修改。
func FixModulePath(workDir string) (bool, error) {
	module, err := ModulePath(workDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if module == "" || module == "x-admin" {
		return false, nil
	}
	changed := false

	goModPath := filepath.Join(workDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return false, err
	}
	oldTool := "tool x-admin/cmd/xadmin"
	newTool := "tool " + module + "/cmd/xadmin"
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == oldTool {
			lines[i] = newTool
			changed = true
		}
	}
	if changed {
		if err := os.WriteFile(goModPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return false, err
		}
	}

	mainPath := filepath.Join(workDir, "cmd", "xadmin", "main.go")
	if data, err := os.ReadFile(mainPath); err == nil {
		oldImport := `"x-admin/tool/xadmin"`
		newImport := `"` + module + `/tool/xadmin"`
		if strings.Contains(string(data), oldImport) {
			out := strings.ReplaceAll(string(data), oldImport, newImport)
			if err := os.WriteFile(mainPath, []byte(out), 0o644); err != nil {
				return changed, err
			}
			changed = true
		}
	} else if !os.IsNotExist(err) {
		return changed, err
	}
	return changed, nil
}

// CheckModulePath 检查工具是否已适配自定义 module 路径。doctor 使用。
func CheckModulePath(workDir string) (string, error) {
	module, err := ModulePath(workDir)
	if err != nil {
		return "", err
	}
	if module == "" || module == "x-admin" {
		return "", nil
	}
	data, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		return "", err
	}
	want := "tool " + module + "/cmd/xadmin"
	if !strings.Contains(string(data), want) {
		return fmt.Sprintf("module 是 %s，但 go.mod 的 tool 指令未适配；运行 go tool xadmin fix-module", module), nil
	}
	mainData, err := os.ReadFile(filepath.Join(workDir, "cmd", "xadmin", "main.go"))
	if err == nil && !strings.Contains(string(mainData), `"`+module+`/tool/xadmin"`) {
		return fmt.Sprintf("module 是 %s，但 cmd/xadmin/main.go 的 import 未适配；运行 go tool xadmin fix-module", module), nil
	}
	return "", nil
}
