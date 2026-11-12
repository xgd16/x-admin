package xadmin

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignoredDirNames 是升级时永远不参与合并的目录名。
// 这些目录要么是构建产物，要么是运行时数据。
var ignoredDirNames = map[string]bool{
	".git":         true,
	".idea":        true,
	".vscode":      true,
	".cache":       true,
	".zcode":       true,
	"node_modules": true,
	"vendor":       true,
	"bin":          true,
	"dist":         true,
	"site":         true,
	"coverage":     true,
	"tmp":          true,
}

// ignoredFileNames 是永远不参与合并的文件名。
var ignoredFileNames = map[string]bool{
	".DS_Store":            true,
	"coverage.out":         true,
	"coverage.html":        true,
	"coverage.json":        true,
	"tsconfig.tsbuildinfo": true,
}

// isUserConfigPath 判断是否属于用户本地配置，升级时不应触碰。
func isUserConfigPath(rel string) bool {
	switch rel {
	case "configs/config.yaml", "configs/local.yaml", ".env":
		return true
	}
	if strings.HasPrefix(rel, ".env.") && rel != ".env.example" {
		return true
	}
	return false
}

func shouldIgnoreDir(rel string) bool {
	if rel == "data/uploads" || strings.HasPrefix(rel, "data/uploads/") {
		return true
	}
	if strings.HasPrefix(rel, ".zcode/") {
		return true
	}
	return ignoredDirNames[path.Base(rel)]
}

func shouldIgnoreFile(rel string) bool {
	if isUserConfigPath(rel) {
		return true
	}
	if ignoredFileNames[path.Base(rel)] {
		return true
	}
	switch path.Ext(rel) {
	case ".log", ".prof":
		return true
	}
	return false
}

// isIgnoredPath 判断归档中的完整文件路径是否位于忽略目录下。
// LoadTree 会直接跳过目录，但 tar 归档需要逐文件判断。
func isIgnoredPath(rel string) bool {
	if shouldIgnoreFile(rel) {
		return true
	}
	if rel == "data/uploads" || strings.HasPrefix(rel, "data/uploads/") {
		return true
	}
	parts := strings.Split(rel, "/")
	for i := 0; i < len(parts)-1; i++ {
		if ignoredDirNames[parts[i]] {
			return true
		}
	}
	return false
}

// LoadTree 把一个目录加载成 Tree。
// 只读取普通文件，跳过符号链接、构建产物、运行时数据和用户本地配置。
func LoadTree(root string) (Tree, error) {
	tree := Tree{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if shouldIgnoreDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldIgnoreFile(rel) {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		tree[rel] = File{Data: data, Mode: info.Mode().Perm()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// writeWorkFile 写入一个工作区文件，保留已有文件的权限位。
func writeWorkFile(workDir, rel string, data []byte, mode fs.FileMode) error {
	target, err := safeJoin(workDir, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("拒绝写入符号链接: %s", rel)
	}
	if mode == 0 {
		mode = 0o644
	}
	return os.WriteFile(target, data, mode)
}

func removeWorkFile(workDir, rel string) error {
	target, err := safeJoin(workDir, rel)
	if err != nil {
		return err
	}
	err = os.Remove(target)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// safeJoin 把相对路径拼到 root 下，并确保结果没有逃出 root。
// 这是对 tar 归档和备份清单的最后一道防线。
func safeJoin(root, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("空路径")
	}
	native := filepath.FromSlash(rel)
	if filepath.IsAbs(native) || filepath.VolumeName(native) != "" {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(rootAbs, native))
	if err != nil {
		return "", err
	}
	if target != rootAbs && !strings.HasPrefix(target, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("路径越界: %s", rel)
	}
	return target, nil
}
