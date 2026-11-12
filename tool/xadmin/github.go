package xadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const githubAPIBase = "https://api.github.com"

// Release 是 GitHub Release 的简化结构。
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}

// Tag 是 GitHub tag 的简化结构。
type Tag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// TreeEntry 是 Git tree API 返回的文件项。
type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
}

// GitHubClient 负责版本查询、commit 解析和源码归档下载。
type GitHubClient struct {
	Repo    string
	Token   string
	APIBase string
	HTTP    *http.Client
}

// NewGitHubClient 创建客户端。token 为空时使用匿名请求（每小时 60 次）。
func NewGitHubClient(repo, token string) *GitHubClient {
	if repo == "" {
		repo = DefaultRepo
	}
	return &GitHubClient{
		Repo:    repo,
		Token:   token,
		APIBase: githubAPIBase,
		HTTP: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// TokenFromEnv 按优先级读取 GitHub token。
// 最后会尝试 gh CLI 的登录态，避免本地开发时被匿名 API 速率限制卡住。
func TokenFromEnv() string {
	for _, key := range []string{"X_ADMIN_GITHUB_TOKEN", "GITHUB_TOKEN", "GH_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ghAuthToken()
}

var (
	ghTokenOnce sync.Once
	ghToken     string
)

func ghAuthToken() string {
	ghTokenOnce.Do(func() {
		path, err := exec.LookPath("gh")
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, path, "auth", "token").Output()
		if err == nil {
			ghToken = strings.TrimSpace(string(out))
		}
	})
	return ghToken
}

func (c *GitHubClient) apiURL(format string, args ...any) string {
	base := strings.TrimRight(c.APIBase, "/")
	return base + fmt.Sprintf(format, args...)
}

func (c *GitHubClient) newRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "x-admin-tool/"+ToolVersion)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

func (c *GitHubClient) doJSON(ctx context.Context, method, rawURL string, out any) error {
	req, err := c.newRequest(ctx, method, rawURL)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("GitHub API 速率限制已用尽，请设置 GITHUB_TOKEN 后重试 (%s)", c.Repo)
		}
		return fmt.Errorf("GitHub API %s: %s: %s", rawURL, resp.Status, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("解析 GitHub 响应: %w", err)
	}
	return nil
}

// LatestRelease 返回最新正式 Release；仓库没有 Release 时退化为最新 tag。
func (c *GitHubClient) LatestRelease(ctx context.Context) (*Release, error) {
	var rel Release
	err := c.doJSON(ctx, http.MethodGet, c.apiURL("/repos/%s/releases/latest", c.Repo), &rel)
	if err == nil {
		return &rel, nil
	}
	// 没有 Release 或只有 draft 时，/releases/latest 会 404，尝试 tag。
	tag, tagErr := c.LatestTag(ctx)
	if tagErr != nil {
		return nil, err
	}
	return &Release{TagName: tag}, nil
}

// ReleaseByTag 返回指定 tag 的 Release；不存在时返回原始错误。
func (c *GitHubClient) ReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	var rel Release
	u := c.apiURL("/repos/%s/releases/tags/%s", c.Repo, url.PathEscape(tag))
	if err := c.doJSON(ctx, http.MethodGet, u, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// LatestTag 返回仓库最新的 tag 名称。
func (c *GitHubClient) LatestTag(ctx context.Context) (string, error) {
	var tags []Tag
	u := c.apiURL("/repos/%s/tags?per_page=1", c.Repo)
	if err := c.doJSON(ctx, http.MethodGet, u, &tags); err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("仓库 %s 没有可用的 Release 或 tag", c.Repo)
	}
	return tags[0].Name, nil
}

// ListTags 返回仓库的 tag 列表。
func (c *GitHubClient) ListTags(ctx context.Context, perPage int) ([]Tag, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 100
	}
	var tags []Tag
	u := c.apiURL("/repos/%s/tags?per_page=%d", c.Repo, perPage)
	if err := c.doJSON(ctx, http.MethodGet, u, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// CommitTree 返回指定 commit 的文件树：path -> git blob SHA。
// 用于没有 git 历史的项目按文件内容指纹识别基准版本。
func (c *GitHubClient) CommitTree(ctx context.Context, commit string) (map[string]string, error) {
	var commitObj struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	u := c.apiURL("/repos/%s/git/commits/%s", c.Repo, commit)
	if err := c.doJSON(ctx, http.MethodGet, u, &commitObj); err != nil {
		return nil, err
	}
	if commitObj.Tree.SHA == "" {
		return nil, fmt.Errorf("commit %s 没有 tree", ShortSHA(commit))
	}
	var treeObj struct {
		Tree      []TreeEntry `json:"tree"`
		Truncated bool        `json:"truncated"`
	}
	u = c.apiURL("/repos/%s/git/trees/%s?recursive=1", c.Repo, commitObj.Tree.SHA)
	if err := c.doJSON(ctx, http.MethodGet, u, &treeObj); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(treeObj.Tree))
	for _, e := range treeObj.Tree {
		if e.Type == "blob" {
			out[e.Path] = e.SHA
		}
	}
	return out, nil
}

// ResolveCommit 把 tag、分支或短 SHA 解析成完整 commit SHA。
func (c *GitHubClient) ResolveCommit(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("版本引用为空")
	}
	if IsSHA(ref) {
		return strings.ToLower(ref), nil
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	u := c.apiURL("/repos/%s/commits/%s", c.Repo, url.PathEscape(ref))
	if err := c.doJSON(ctx, http.MethodGet, u, &commit); err != nil {
		return "", fmt.Errorf("解析版本 %q: %w", ref, err)
	}
	if commit.SHA == "" {
		return "", fmt.Errorf("版本 %q 没有对应的 commit", ref)
	}
	return commit.SHA, nil
}

// OpenArchive 打开指定 commit 的源码 tar.gz。
// 使用 API tarball 地址，私有仓库的重定向 URL 自带签名，不需要二次鉴权。
func (c *GitHubClient) OpenArchive(ctx context.Context, commit string) (io.ReadCloser, error) {
	u := c.apiURL("/repos/%s/tarball/%s", c.Repo, commit)
	req, err := c.newRequest(ctx, http.MethodGet, u)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("下载源码归档失败: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return resp.Body, nil
}

// DownloadTree 下载并解析指定 commit 的源码。
func (c *GitHubClient) DownloadTree(ctx context.Context, commit string) (Tree, error) {
	body, err := c.OpenArchive(ctx, commit)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return ExtractTarGz(body)
}
