package xadmin

import (
	"context"
	"fmt"
	"strings"
)

// InitBaseline 解析 from 对应的上游 commit，并写入框架版本基准。
//
// .x-admin/framework.json 是本地缓存（已忽略），FRAMEWORK_VERSION 需要提交。
func InitBaseline(ctx context.Context, repo, token, from, dir string) (State, string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return State{}, "", fmt.Errorf("需要 from，例如 v1.3.0")
	}
	if strings.TrimSpace(repo) == "" {
		repo = DefaultRepo
	}
	workDir, err := absProjectDir(dir)
	if err != nil {
		return State{}, "", err
	}
	client := NewGitHubClient(repo, tokenOrDefault(token))
	commit, err := client.ResolveCommit(ctx, from)
	if err != nil {
		return State{}, "", fmt.Errorf("解析版本 %q 失败: %w", from, err)
	}
	state := State{Repo: repo, Ref: from, Commit: commit}
	if err := WriteState(workDir, state); err != nil {
		return State{}, "", fmt.Errorf("写入 %s: %w", StatePath, err)
	}
	versionRef := state.Ref
	if _, ok := ParseVersion(versionRef); !ok {
		versionRef = commit
	}
	if err := WriteFrameworkVersion(workDir, versionRef); err != nil {
		return State{}, "", fmt.Errorf("写入 %s: %w", VersionPath, err)
	}
	return state, versionRef, nil
}
