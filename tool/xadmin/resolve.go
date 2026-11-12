package xadmin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SourceOptions 是 check/plan/apply 共用的版本与目录参数。
type SourceOptions struct {
	Repo    string
	Token   string
	From    string
	To      string
	Dir     string
	FromDir string
	ToDir   string
	Timeout time.Duration
	Verbose bool
	// NoFingerprint 禁用内容指纹推断，避免额外的 GitHub tree API 请求。
	NoFingerprint bool
}

func (o SourceOptions) token() string {
	if strings.TrimSpace(o.Token) != "" {
		return strings.TrimSpace(o.Token)
	}
	return TokenFromEnv()
}

func (o SourceOptions) workDir() (string, error) {
	dir := o.Dir
	if dir == "" {
		dir = "."
	}
	return absProjectDir(dir)
}

// absProjectDir 校验目录存在，并向上解析到最近的项目根目录。
func absProjectDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("目录不存在: %s", abs)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("不是目录: %s", abs)
	}
	return findProjectRoot(abs), nil
}

// findProjectRoot 从 start 向上寻找最近的 go.mod 所在目录。
// 如果 start 在 git 仓库内，最多找到 git 根目录，避免误用父目录的无关项目。
// 找不到时返回 start。
func findProjectRoot(start string) string {
	gitRoot := ""
	if git, err := NewGit(start); err == nil {
		gitRoot = git.Dir
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if gitRoot != "" && dir == gitRoot {
			return gitRoot
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

// Resolved 是解析完成后的升级输入。
type Resolved struct {
	Git        *Git
	Client     *GitHubClient
	WorkDir    string
	FromRef    string
	FromCommit string
	FromSource string
	ToRef      string
	ToCommit   string
	Base       Tree
	Target     Tree
	Local      Tree
	Release    *Release
}

// ResolveSources 解析基准/目标版本并加载三棵树。
func ResolveSources(ctx context.Context, opts SourceOptions) (*Resolved, error) {
	workDir, err := opts.workDir()
	if err != nil {
		return nil, err
	}
	repo := opts.Repo
	if repo == "" {
		repo = DefaultRepo
	}
	if !strings.Contains(repo, "/") {
		return nil, fmt.Errorf("--repo 需要 owner/name 格式: %q", repo)
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	client := NewGitHubClient(repo, opts.token())

	git, gitErr := NewGit(workDir)
	if gitErr != nil && opts.FromDir == "" {
		// 没有 git 时，plan 仍然可以借助状态文件/显式 --from 运行；
		// apply 会在真正写入前再次要求 git。
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "[warn] %v\n", gitErr)
		}
	}

	res := &Resolved{
		Git:     git,
		Client:  client,
		WorkDir: workDir,
	}

	// 先加载本地工作区：指纹识别基准版本时需要它。
	res.Local, err = LoadTree(workDir)
	if err != nil {
		return nil, fmt.Errorf("扫描工作区: %w", err)
	}

	// 1. 目标版本。
	if opts.ToDir != "" {
		targetDir, err := filepath.Abs(opts.ToDir)
		if err != nil {
			return nil, err
		}
		res.Target, err = LoadTree(targetDir)
		if err != nil {
			return nil, fmt.Errorf("加载目标目录 %s: %w", targetDir, err)
		}
		res.ToRef = opts.To
		if res.ToRef == "" {
			res.ToRef = "local:" + filepath.Base(targetDir)
		}
	} else {
		toRef := opts.To
		if toRef == "" {
			release, err := client.LatestRelease(ctx)
			if err != nil {
				return nil, fmt.Errorf("查询最新版本: %w", err)
			}
			res.Release = release
			toRef = release.TagName
		} else if !IsSHA(toRef) {
			// Release notes 是锦上添花，查不到不阻塞升级。
			if release, err := client.ReleaseByTag(ctx, toRef); err == nil {
				res.Release = release
			}
		}
		res.ToRef = toRef
		res.ToCommit, err = client.ResolveCommit(ctx, toRef)
		if err != nil {
			return nil, err
		}
		res.Target, err = client.DownloadTree(ctx, res.ToCommit)
		if err != nil {
			return nil, fmt.Errorf("下载 %s 源码: %w", toRef, err)
		}
	}

	// 2. 基准版本。
	if opts.FromDir != "" {
		baseDir, err := filepath.Abs(opts.FromDir)
		if err != nil {
			return nil, err
		}
		res.Base, err = LoadTree(baseDir)
		if err != nil {
			return nil, fmt.Errorf("加载基准目录 %s: %w", baseDir, err)
		}
		res.FromRef = opts.From
		res.FromSource = "目录"
		if res.FromRef == "" {
			res.FromRef = "local:" + filepath.Base(baseDir)
		}
	} else if opts.From != "" {
		res.FromRef = opts.From
		res.FromSource = "参数 --from"
		res.FromCommit, err = client.ResolveCommit(ctx, opts.From)
		if err != nil {
			return nil, err
		}
		if res.FromCommit == res.ToCommit && res.Target != nil {
			res.Base = res.Target
		} else {
			res.Base, err = client.DownloadTree(ctx, res.FromCommit)
			if err != nil {
				return nil, fmt.Errorf("下载基准 %s 源码: %w", opts.From, err)
			}
		}
	} else {
		if err := resolveAutoBase(ctx, res, opts); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// resolveAutoBase 按 状态文件 -> FRAMEWORK_VERSION -> 上游同名 git tag -> 内容指纹 的顺序确定基准。
// 全程只读上游公开 API，不 fetch 上游 git 历史，也不修改用户的远程仓库。
func resolveAutoBase(ctx context.Context, res *Resolved, opts SourceOptions) error {
	state, stateErr := ReadState(res.WorkDir)
	if stateErr != nil && opts.Verbose {
		fmt.Fprintf(os.Stderr, "[warn] 读取 %s: %v\n", StatePath, stateErr)
	}
	if state != nil && strings.TrimSpace(state.Ref) != "" {
		commit := strings.TrimSpace(state.Commit)
		if commit == "" {
			var err error
			commit, err = res.Client.ResolveCommit(ctx, state.Ref)
			if err != nil {
				return fmt.Errorf("解析状态文件版本 %q: %w", state.Ref, err)
			}
		}
		return setBaseFromCommit(ctx, res, state.Ref, "状态文件", commit)
	}

	version, versionErr := ReadFrameworkVersion(res.WorkDir)
	if versionErr != nil && opts.Verbose {
		fmt.Fprintf(os.Stderr, "[warn] 读取 %s: %v\n", VersionPath, versionErr)
	}
	if version != "" {
		commit, err := res.Client.ResolveCommit(ctx, version)
		if err != nil {
			return fmt.Errorf("解析 %s 中的版本 %q: %w", VersionPath, version, err)
		}
		return setBaseFromCommit(ctx, res, version, VersionPath, commit)
	}

	if res.Git != nil {
		if tag, commit := verifiedUpstreamTag(ctx, res.Client, res.Git); tag != "" {
			return setBaseFromCommit(ctx, res, tag, "git tag", commit)
		}
	}

	// 最后一层：新 git init / 复制源码没有状态文件时，按文件内容指纹识别。
	if !opts.NoFingerprint {
		fmt.Fprintln(os.Stderr, "[warn] 未找到版本状态文件或上游同名 tag，正在按文件内容识别基准版本...")
		fingerprint, fpErr := DetectBaseFingerprint(ctx, res.Client, res.Local, res.ToRef, 8)
		if fpErr == nil && fingerprint != nil && fingerprint.Detected {
			fmt.Fprintf(os.Stderr, "[warn] 按内容指纹推断基准为 %s（匹配 %.0f%%，%d/%d 个文件，置信度 %s）\n",
				fingerprint.Tag, fingerprint.Score*100, fingerprint.Matched, fingerprint.Total, fingerprint.Confidence)
			return setBaseFromCommit(ctx, res, fingerprint.Tag, "内容指纹", fingerprint.Commit)
		}

		var b strings.Builder
		b.WriteString("无法确定当前框架版本（没有状态文件、FRAMEWORK_VERSION 或可用的上游 tag）。\n")
		if fingerprint != nil && len(fingerprint.Candidates) > 0 {
			b.WriteString("文件指纹最接近：")
			for i, c := range fingerprint.Candidates {
				if i >= 5 {
					break
				}
				if i > 0 {
					b.WriteString("、")
				}
				fmt.Fprintf(&b, "%s（%.0f%%）", c.Tag, c.Score*100)
			}
			b.WriteString("\n")
		}
		if fpErr != nil {
			fmt.Fprintf(&b, "指纹识别失败：%v\n", fpErr)
		}
		if fingerprint != nil && fingerprint.Tag != "" {
			fmt.Fprintf(&b, "建议显式指定基准：\n  go tool xadmin upgrade init --from %s\n或：\n  go tool xadmin upgrade plan --from %s\n", fingerprint.Tag, fingerprint.Tag)
		} else {
			fmt.Fprintf(&b, "请在仓库根目录创建 %s，或使用 --from 指定基准版本：\n{\n  \"repo\": \"%s\",\n  \"ref\": \"v1.3.0\"\n}\n", StatePath, res.Client.Repo)
		}
		return errors.New(b.String())
	}

	return fmt.Errorf("无法确定当前框架版本。请使用 --from 指定基准，或运行 go tool xadmin upgrade init --from <版本>")
}

func setBaseFromCommit(ctx context.Context, res *Resolved, ref, source, commit string) error {
	res.FromRef = ref
	res.FromSource = source
	res.FromCommit = commit
	if commit == res.ToCommit && res.Target != nil {
		res.Base = res.Target
		return nil
	}
	base, err := res.Client.DownloadTree(ctx, commit)
	if err != nil {
		return fmt.Errorf("下载基准 %s 源码: %w", ref, err)
	}
	res.Base = base
	return nil
}

// verifiedUpstreamTag 只接受"本地 tag 与上游同名 tag 指向同一个 commit"的情况。
// 下游项目可能给自己的版本打 v1.0.0 之类的 tag，不能把它误当成框架版本。
func verifiedUpstreamTag(ctx context.Context, client *GitHubClient, git *Git) (string, string) {
	if git == nil || client == nil {
		return "", ""
	}
	tag := git.DescribeTag()
	if tag == "" {
		return "", ""
	}
	localCommit, err := git.TagCommit(tag)
	if err != nil {
		return "", ""
	}
	upstreamCommit, err := client.ResolveCommit(ctx, tag)
	if err != nil || !strings.EqualFold(localCommit, upstreamCommit) {
		return "", ""
	}
	return tag, upstreamCommit
}

// CheckResult 是 upgrade check 的输出结构。
type CheckResult struct {
	Repo          string    `json:"repo"`
	Current       string    `json:"current"`
	CurrentSource string    `json:"current_source,omitempty"`
	CurrentDetail string    `json:"current_detail,omitempty"`
	Latest        string    `json:"latest"`
	LatestCommit  string    `json:"latest_commit,omitempty"`
	Comparable    bool      `json:"comparable"`
	UpToDate      bool      `json:"up_to_date"`
	DetectError   string    `json:"detect_error,omitempty"`
	ReleaseURL    string    `json:"release_url,omitempty"`
	PublishedAt   time.Time `json:"published_at,omitempty"`
	Notes         string    `json:"notes,omitempty"`
}

// CheckUpgrade 查询上游最新版本并与本地基准比较，不下载源码。
func CheckUpgrade(ctx context.Context, opts SourceOptions) (*CheckResult, error) {
	workDir, err := opts.workDir()
	if err != nil {
		return nil, err
	}
	repo := opts.Repo
	if repo == "" {
		repo = DefaultRepo
	}
	client := NewGitHubClient(repo, opts.token())
	git, _ := NewGit(workDir)

	var release *Release
	if opts.To != "" {
		if rel, err := client.ReleaseByTag(ctx, opts.To); err == nil {
			release = rel
		} else {
			release = &Release{TagName: opts.To}
		}
	} else {
		release, err = client.LatestRelease(ctx)
		if err != nil {
			return nil, fmt.Errorf("查询最新版本: %w", err)
		}
	}

	current := strings.TrimSpace(opts.From)
	source := "参数 --from"
	if current == "" {
		source = ""
		if state, err := ReadState(workDir); err == nil && state != nil {
			current = state.Ref
			source = "状态文件"
		}
	}
	if current == "" {
		if version, err := ReadFrameworkVersion(workDir); err == nil && version != "" {
			current = version
			source = VersionPath
		}
	}
	if current == "" && git != nil {
		if tag, _ := verifiedUpstreamTag(ctx, client, git); tag != "" {
			current = tag
			source = "git tag"
		}
	}
	detail := ""
	if git != nil {
		detail = git.Describe()
	}
	detectError := ""
	if current == "" && !opts.NoFingerprint {
		local, lerr := LoadTree(workDir)
		if lerr != nil {
			detectError = lerr.Error()
		} else if fingerprint, fpErr := DetectBaseFingerprint(ctx, client, local, release.TagName, 8); fpErr != nil {
			detectError = fpErr.Error()
		} else if fingerprint != nil {
			if fingerprint.Detected {
				current = fingerprint.Tag
				source = "内容指纹"
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "[warn] 按内容指纹推断当前版本为 %s（匹配 %.0f%%）\n",
						fingerprint.Tag, fingerprint.Score*100)
				}
			} else if len(fingerprint.Candidates) > 0 {
				c := fingerprint.Candidates[0]
				detectError = fmt.Sprintf("指纹置信度不足，最接近 %s（%.0f%%）", c.Tag, c.Score*100)
			}
		}
	}

	latestCommit := ""
	if !IsSHA(release.TagName) {
		if commit, err := client.ResolveCommit(ctx, release.TagName); err == nil {
			latestCommit = commit
		}
	}
	_, currentOK := ParseVersion(current)
	_, latestOK := ParseVersion(release.TagName)
	comparable := currentOK && latestOK
	upToDate := comparable && IsUpToDate(current, release.TagName)
	if !comparable && IsSHA(current) && latestCommit != "" && strings.EqualFold(current, latestCommit) {
		comparable, upToDate = true, true
	}
	return &CheckResult{
		Repo:          repo,
		Current:       current,
		CurrentSource: source,
		CurrentDetail: detail,
		Latest:        release.TagName,
		LatestCommit:  latestCommit,
		Comparable:    comparable,
		UpToDate:      upToDate,
		DetectError:   detectError,
		ReleaseURL:    release.HTMLURL,
		PublishedAt:   release.PublishedAt,
		Notes:         release.Body,
	}, nil
}
