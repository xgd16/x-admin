package xadmin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// BackupManifest 记录一次升级前的文件快照，供 rollback 使用。
type BackupManifest struct {
	Version        int           `json:"version"`
	CreatedAt      time.Time     `json:"created_at"`
	FromRef        string        `json:"from_ref"`
	ToRef          string        `json:"to_ref"`
	Branch         string        `json:"branch,omitempty"`
	OriginalBranch string        `json:"original_branch,omitempty"`
	Entries        []BackupEntry `json:"entries"`
}

// BackupEntry 是单个文件的备份记录。
type BackupEntry struct {
	Path    string `json:"path"`
	Existed bool   `json:"existed"`
	Backup  string `json:"backup,omitempty"`
}

func writeTarget(ch Change) (rel string, write bool, remove bool) {
	switch ch.Kind {
	case ChangeUpdate, ChangeAdd:
		rel = ch.Path
		if ch.UpstreamOnly {
			rel = UpstreamPath(rel)
		}
		return rel, true, false
	case ChangeDelete:
		return ch.Path, false, true
	case ChangeConflict:
		if ch.Data == nil {
			return "", false, false
		}
		rel = ch.Path
		if ch.UpstreamOnly {
			rel = UpstreamPath(rel)
		}
		return rel, true, false
	default:
		return "", false, false
	}
}

// CreateBackup 在 .git/x-admin/backups 下保存将被覆盖或删除的文件。
func CreateBackup(git *Git, plan *Plan, workDir string) (string, error) {
	gitDir, err := git.GitDir()
	if err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	root := filepath.Join(gitDir, "x-admin", "backups")
	dir := filepath.Join(root, stamp)
	if _, err := os.Stat(dir); err == nil {
		dir = filepath.Join(root, stamp+"-"+fmt.Sprintf("%03d", time.Now().Nanosecond()/1e6))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	seen := make(map[string]bool)
	var entries []BackupEntry
	backupOne := func(rel string) error {
		if rel == "" || seen[rel] {
			return nil
		}
		seen[rel] = true
		src, err := safeJoin(workDir, rel)
		if err != nil {
			return err
		}
		info, statErr := os.Stat(src)
		existed := statErr == nil
		entry := BackupEntry{Path: rel, Existed: existed}
		if existed {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("无法备份非普通文件: %s", rel)
			}
			backupRel := filepath.ToSlash(filepath.Join("files", filepath.FromSlash(rel)))
			dst, err := safeJoin(dir, backupRel)
			if err != nil {
				return err
			}
			if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
				return err
			}
			entry.Backup = backupRel
		}
		entries = append(entries, entry)
		return nil
	}
	for _, ch := range plan.Changes {
		rel, write, remove := writeTarget(ch)
		if (!write && !remove) || rel == "" {
			continue
		}
		if err := backupOne(rel); err != nil {
			return "", err
		}
	}
	// 升级成功后会更新状态文件；回滚时也要把它恢复到升级前。
	if err := backupOne(StatePath); err != nil {
		return "", err
	}
	if err := backupOne(VersionPath); err != nil {
		return "", err
	}

	manifest := BackupManifest{
		Version:   1,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		FromRef:   plan.FromRef,
		ToRef:     plan.ToRef,
		Entries:   entries,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		return "", err
	}
	return dir, nil
}

// SetBackupBranch 在写入 manifest 后补充升级分支与原分支。
func SetBackupBranch(dir, branch, originalBranch string) error {
	p := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	var manifest BackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	manifest.Branch = branch
	manifest.OriginalBranch = originalBranch
	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(p, out, 0o644)
}

// LatestBackup 返回最近一次备份目录。
func LatestBackup(git *Git) (string, error) {
	gitDir, err := git.GitDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(gitDir, "x-admin", "backups")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("没有可回滚的升级记录")
		}
		return "", err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("没有可回滚的升级记录")
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return filepath.Join(root, names[0]), nil
}

// Rollback 恢复指定备份目录中的文件。
func Rollback(git *Git, workDir, backupDir string) (int, BackupManifest, error) {
	data, err := os.ReadFile(filepath.Join(backupDir, "manifest.json"))
	if err != nil {
		return 0, BackupManifest{}, fmt.Errorf("读取备份清单: %w", err)
	}
	var manifest BackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return 0, BackupManifest{}, fmt.Errorf("解析备份清单: %w", err)
	}
	restored := 0
	for _, e := range manifest.Entries {
		dst, err := safeJoin(workDir, e.Path)
		if err != nil {
			return restored, manifest, err
		}
		if !e.Existed {
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				return restored, manifest, err
			}
			restored++
			continue
		}
		src, err := safeJoin(backupDir, e.Backup)
		if err != nil {
			return restored, manifest, err
		}
		info, err := os.Stat(src)
		if err != nil {
			return restored, manifest, fmt.Errorf("备份文件缺失 %s: %w", e.Backup, err)
		}
		if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
			return restored, manifest, err
		}
		restored++
	}
	return restored, manifest, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("拒绝读取非普通文件: %s", src)
	}
	if dstInfo, err := os.Lstat(dst); err == nil && dstInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("拒绝写入符号链接: %s", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
