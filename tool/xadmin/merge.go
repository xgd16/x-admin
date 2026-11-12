package xadmin

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// Merger 负责三方合并。
type Merger interface {
	// Merge 返回合并结果和是否存在冲突。
	Merge(path string, local, base, upstream []byte) ([]byte, bool, error)
}

// GitMerger 使用 git merge-file 做文本三方合并，行为与 git 冲突标记一致。
type GitMerger struct{}

// Merge 实现 Merger。
func (GitMerger) Merge(_ string, local, base, upstream []byte) ([]byte, bool, error) {
	if isBinary(local) || isBinary(base) || isBinary(upstream) {
		return nil, true, nil
	}
	localPath, cleanLocal, err := writeTempMergeFile(local)
	if err != nil {
		return nil, false, err
	}
	defer cleanLocal()
	basePath, cleanBase, err := writeTempMergeFile(base)
	if err != nil {
		return nil, false, err
	}
	defer cleanBase()
	upstreamPath, cleanUpstream, err := writeTempMergeFile(upstream)
	if err != nil {
		return nil, false, err
	}
	defer cleanUpstream()

	cmd := exec.Command("git", "merge-file", "-p", "--diff3",
		"-L", "local", "-L", "base", "-L", "upstream",
		localPath, basePath, upstreamPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return stdout.Bytes(), false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return stdout.Bytes(), true, nil
	}
	return nil, false, fmt.Errorf("git merge-file: %w: %s", err, strings.TrimSpace(stderr.String()))
}

// FallbackMerger 在没有 git 时使用：双方都改动直接判冲突。
type FallbackMerger struct{}

// Merge 实现 Merger。
func (FallbackMerger) Merge(_ string, local, base, upstream []byte) ([]byte, bool, error) {
	switch {
	case bytes.Equal(local, base):
		return upstream, false, nil
	case bytes.Equal(upstream, base):
		return local, false, nil
	case bytes.Equal(local, upstream):
		return local, false, nil
	default:
		return nil, true, nil
	}
}

// NewMerger 优先使用 git merge-file，找不到 git 时退化为保守策略。
func NewMerger() (Merger, string) {
	if _, err := exec.LookPath("git"); err != nil {
		return FallbackMerger{}, "未找到 git，双方都修改的文件会一律标记为冲突"
	}
	return GitMerger{}, ""
}

func writeTempMergeFile(data []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "xadmin-merge-*")
	if err != nil {
		return "", nil, err
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return name, cleanup, nil
}

func isBinary(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}

// ComputePlan 对比 base、target、local 三棵树，生成文件级升级计划。
//
// 规则：
//   - 本地未改、上游改了        -> update
//   - 本地改了、上游没改        -> keep
//   - 两边改得一样              -> same
//   - 两边都改、文本可合并      -> update（合并结果）
//   - 两边都改、无法合并        -> conflict（写冲突标记或 .upstream）
//   - 上游删除、本地未改        -> delete
//   - 上游删除、本地改过        -> conflict（保留本地）
//   - 上游新增、本地已有同名文件 -> conflict（本地保留，上游写 .upstream）
func ComputePlan(repo, workDir, fromRef, fromCommit, toRef, toCommit string, base, target, local Tree, merger Merger) *Plan {
	plan := &Plan{
		Repo:       repo,
		WorkDir:    workDir,
		FromRef:    fromRef,
		FromCommit: fromCommit,
		ToRef:      toRef,
		ToCommit:   toCommit,
	}
	if merger == nil {
		merger = FallbackMerger{}
	}
	paths := make(map[string]struct{}, len(base)+len(target)+len(local))
	for p := range base {
		paths[p] = struct{}{}
	}
	for p := range target {
		paths[p] = struct{}{}
	}
	for p := range local {
		paths[p] = struct{}{}
	}
	for _, rel := range slices.Sorted(maps.Keys(paths)) {
		b, hasB := base[rel]
		t, hasT := target[rel]
		l, hasL := local[rel]
		if ch, ok := planFile(rel, b, hasB, t, hasT, l, hasL, merger); ok {
			plan.Changes = append(plan.Changes, ch)
		}
	}
	plan.Stats = plan.ComputeSummary()
	return plan
}

func planFile(rel string, b File, hasB bool, t File, hasT bool, l File, hasL bool, merger Merger) (Change, bool) {
	switch {
	case hasT && hasB:
		if !hasL {
			// 本地删除。
			if bytes.Equal(t.Data, b.Data) {
				return Change{Path: rel, Kind: ChangeKeep, Reason: "本地已删除，上游未修改"}, true
			}
			return Change{
				Path: rel, Kind: ChangeConflict, UpstreamOnly: true, Data: t.Data, Mode: t.Mode,
				Reason: "本地已删除，但上游修改了该文件；上游版本已另存为 .upstream",
			}, true
		}
		switch {
		case bytes.Equal(l.Data, b.Data):
			if bytes.Equal(t.Data, b.Data) {
				return Change{Path: rel, Kind: ChangeSame}, true
			}
			return Change{Path: rel, Kind: ChangeUpdate, Data: t.Data, Mode: modeOr(t.Mode, l.Mode), Reason: "上游修改，本地未改"}, true
		case bytes.Equal(l.Data, t.Data):
			return Change{Path: rel, Kind: ChangeSame, Reason: "本地与上游修改一致"}, true
		case bytes.Equal(t.Data, b.Data):
			return Change{Path: rel, Kind: ChangeKeep, Reason: "本地修改，上游未改"}, true
		default:
			if isBinary(l.Data) || isBinary(b.Data) || isBinary(t.Data) {
				return Change{
					Path: rel, Kind: ChangeConflict, UpstreamOnly: true, Data: t.Data, Mode: t.Mode,
					Reason: "二进制文件双方都修改；本地保留，上游版本另存为 .upstream",
				}, true
			}
			merged, conflict, err := merger.Merge(rel, l.Data, b.Data, t.Data)
			if err != nil {
				return Change{
					Path: rel, Kind: ChangeConflict, Data: l.Data, Mode: l.Mode,
					Reason: "三方合并失败：" + err.Error(),
				}, true
			}
			if conflict {
				return Change{
					Path: rel, Kind: ChangeConflict, Data: merged, Mode: l.Mode,
					Reason: "双方都修改，三方合并存在冲突，已写入冲突标记",
				}, true
			}
			return Change{
				Path: rel, Kind: ChangeUpdate, Data: merged, Mode: modeOr(l.Mode, t.Mode),
				Reason: "双方都修改，三方合并成功",
			}, true
		}
	case hasT && !hasB:
		if !hasL {
			return Change{Path: rel, Kind: ChangeAdd, Data: t.Data, Mode: t.Mode, Reason: "上游新增文件"}, true
		}
		if bytes.Equal(l.Data, t.Data) {
			return Change{Path: rel, Kind: ChangeSame, Reason: "本地已存在相同文件"}, true
		}
		return Change{
			Path: rel, Kind: ChangeConflict, UpstreamOnly: true, Data: t.Data, Mode: t.Mode,
			Reason: "上游新增文件与本地同名；本地保留，上游版本另存为 .upstream",
		}, true
	case !hasT && hasB:
		if !hasL {
			return Change{Path: rel, Kind: ChangeSame, Reason: "本地与上游均无此文件"}, true
		}
		if bytes.Equal(l.Data, b.Data) {
			return Change{Path: rel, Kind: ChangeDelete, Reason: "上游删除，本地未改"}, true
		}
		return Change{
			Path: rel, Kind: ChangeConflict, Data: l.Data, Mode: l.Mode,
			Reason: "上游删除或重命名了该文件，但本地有修改；已保留本地文件",
		}, true
	default:
		// 只存在于本地：用户自己的文件，不参与升级。
		return Change{}, false
	}
}

func modeOr(modes ...os.FileMode) os.FileMode {
	for _, m := range modes {
		if m != 0 {
			return m
		}
	}
	return 0o644
}

// EnrichPlan 补充迁移编号冲突和配置新增项。
func EnrichPlan(plan *Plan, base, target, local Tree, workDir string) {
	plan.Migrations = detectMigrationIssues(base, target, local)
	plan.ConfigAdded = detectConfigAdditions(target, workDir)
}

// UpstreamPath 返回另存上游版本的文件名。
func UpstreamPath(rel string) string {
	return rel + ".upstream"
}
