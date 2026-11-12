package xadmin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReadState 读取 .x-admin/framework.json。文件不存在时返回 nil, nil。
func ReadState(workDir string) (*State, error) {
	p := filepath.Join(workDir, filepath.FromSlash(StatePath))
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", StatePath, err)
	}
	return &st, nil
}

// WriteState 写入 .x-admin/framework.json。
func WriteState(workDir string, st State) error {
	if st.Repo == "" {
		st.Repo = DefaultRepo
	}
	st.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	p := filepath.Join(workDir, filepath.FromSlash(StatePath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// ReadFrameworkVersion 读取根目录的 FRAMEWORK_VERSION。
// 文件不存在时返回空字符串。
func ReadFrameworkVersion(workDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(workDir, VersionPath))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line, nil
}

// WriteFrameworkVersion 写入根目录的 FRAMEWORK_VERSION。
func WriteFrameworkVersion(workDir, version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(workDir, VersionPath), []byte(version+"\n"), 0o644)
}
