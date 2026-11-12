package xadmin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Git 封装工具需要的 git 操作。所有命令都在仓库根目录执行。
type Git struct {
	Dir string
}

// NewGit 确认 dir 在 git 仓库内，并返回仓库根目录。
func NewGit(dir string) (*Git, error) {
	out, err := runGit(dir, 30*time.Second, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("当前目录不是 git 仓库（升级需要 git 保证可回滚）: %w", err)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return nil, fmt.Errorf("无法确定 git 仓库根目录")
	}
	return &Git{Dir: root}, nil
}

func runGit(dir string, timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return out, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (g *Git) run(args ...string) (string, error) {
	out, err := runGit(g.Dir, 60*time.Second, args...)
	return strings.TrimSpace(string(out)), err
}

// Head 返回当前 HEAD commit。
func (g *Git) Head() (string, error) {
	return g.run("rev-parse", "HEAD")
}

// GitDir 返回 .git 目录的绝对路径，兼容 worktree 的 gitdir 文件。
func (g *Git) GitDir() (string, error) {
	out, err := g.run("rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return out, nil
	}
	return filepath.Join(g.Dir, out), nil
}

// Branch 返回当前分支名；detached HEAD 时返回 HEAD。
func (g *Git) Branch() string {
	out, err := g.run("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || out == "" {
		return "HEAD"
	}
	return out
}

// Describe 返回带本地提交信息的版本描述，例如 v1.3.0-5-gd3f342a-dirty。
func (g *Git) Describe() string {
	out, err := g.run("describe", "--tags", "--match", "v[0-9]*", "--always", "--dirty")
	if err != nil {
		return ""
	}
	return out
}

// DescribeTag 返回最近的可达 release tag，例如 v1.3.0。
func (g *Git) DescribeTag() string {
	out, err := g.run("describe", "--tags", "--match", "v[0-9]*", "--abbrev=0")
	if err != nil {
		return ""
	}
	return out
}

// TagCommit 返回 tag 指向的 commit。
func (g *Git) TagCommit(tag string) (string, error) {
	out, err := g.run("rev-list", "-n", "1", tag)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Status 返回 git status --porcelain 的输出行。
func (g *Git) Status() ([]string, error) {
	out, err := g.run("status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// IsClean 判断工作区是否干净。
func (g *Git) IsClean() (bool, []string, error) {
	lines, err := g.Status()
	if err != nil {
		return false, nil, err
	}
	return len(lines) == 0, lines, nil
}

// CreateBranch 从当前 HEAD 创建并切换分支。
func (g *Git) CreateBranch(name string) error {
	_, err := g.run("switch", "-c", name)
	return err
}

// Switch 切换到已有分支。
func (g *Git) Switch(name string) error {
	_, err := g.run("switch", name)
	return err
}

// BranchExists 判断本地分支是否存在。
func (g *Git) BranchExists(name string) bool {
	_, err := g.run("rev-parse", "--verify", "refs/heads/"+name)
	return err == nil
}

// CommitCountBetween 返回 base..head 之间的提交数。
func (g *Git) CommitCountBetween(base, head string) (int, error) {
	out, err := g.run("rev-list", "--count", base+".."+head)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range strings.TrimSpace(out) {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("无法解析提交数: %q", out)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// CommitAll 暂存全部改动并提交。
func (g *Git) CommitAll(message string) error {
	if _, err := g.run("add", "-A"); err != nil {
		return err
	}
	_, err := g.run("commit", "-m", message)
	return err
}

// Push 推送分支到 origin。
func (g *Git) Push(branch string) error {
	_, err := g.run("push", "-u", "origin", branch)
	return err
}

// OriginURL 返回 origin 的 URL。
func (g *Git) OriginURL() string {
	out, err := g.run("remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return out
}

var githubRemoteRe = regexp.MustCompile(`github\.com[:/]+([^/]+)/(.+?)(?:\.git)?$`)

// RepoFromRemote 从 git remote URL 解析 owner/repo。
func RepoFromRemote(remote string) string {
	m := githubRemoteRe.FindStringSubmatch(strings.TrimSpace(remote))
	if len(m) != 3 {
		return ""
	}
	return m[1] + "/" + m[2]
}

// CompareURL 构造 origin 仓库的 compare 页面，用于没有 gh CLI 时提示手工开 PR。
func CompareURL(remote, base, branch string) string {
	repo := RepoFromRemote(remote)
	if repo == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/compare/%s...%s?expand=1", repo, base, branch)
}
