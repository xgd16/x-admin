package xadmin

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
)

const (
	fingerprintMinScore  = 0.30
	fingerprintMinMargin = 0.03
)

// FingerprintCandidate 是一个候选基准版本及其相似度。
type FingerprintCandidate struct {
	Tag     string  `json:"tag"`
	Score   float64 `json:"score"`
	Matched int     `json:"matched"`
	Total   int     `json:"total"`
}

// FingerprintResult 是内容指纹识别结果。
type FingerprintResult struct {
	Tag        string                 `json:"tag"`
	Commit     string                 `json:"commit"`
	Score      float64                `json:"score"`
	Matched    int                    `json:"matched"`
	Total      int                    `json:"total"`
	Detected   bool                   `json:"detected"`
	Confidence string                 `json:"confidence"`
	Candidates []FingerprintCandidate `json:"candidates,omitempty"`
}

func isFingerprintIgnored(path string) bool {
	return path == StatePath || path == VersionPath
}

// localBlobSHA 计算与 git hash-object 一致的 blob SHA。
// 统一 CRLF 为 LF，避免 Windows 检出导致全部文件指纹不一致。
func localBlobSHA(data []byte) string {
	if !isBinary(data) {
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	}
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	_, _ = h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// scoreTree 用本地文件内容匹配上游 tree，返回命中数和上游文件总数。
// 分母只取上游文件数，用户新增的业务文件不会拉低相似度。
func scoreTree(local Tree, upstream map[string]string) (matched, total int) {
	for path, upstreamSHA := range upstream {
		if isFingerprintIgnored(path) {
			continue
		}
		total++
		file, ok := local[path]
		if !ok {
			continue
		}
		if localBlobSHA(file.Data) == upstreamSHA {
			matched++
		}
	}
	return matched, total
}

// selectCandidateTags 过滤并排序候选 tag。
func selectCandidateTags(tags []Tag, toRef string, max int) []Tag {
	if max <= 0 {
		max = 12
	}
	var candidates []Tag
	for _, tag := range tags {
		if tag.Name == "" || tag.Commit.SHA == "" {
			continue
		}
		if _, ok := ParseVersion(tag.Name); !ok {
			continue
		}
		if _, ok := ParseVersion(toRef); ok && CompareVersion(tag.Name, toRef) > 0 {
			continue
		}
		candidates = append(candidates, tag)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return CompareVersion(candidates[i].Name, candidates[j].Name) > 0
	})
	if len(candidates) > max {
		candidates = candidates[:max]
	}
	return candidates
}

// DetectBaseFingerprint 在没有 git 历史、也没有版本状态文件时，
// 用本地文件内容与上游 tag 的文件树比对，推断用户是从哪个版本开始的。
func DetectBaseFingerprint(ctx context.Context, client *GitHubClient, local Tree, toRef string, maxCandidates int) (*FingerprintResult, error) {
	tags, err := client.ListTags(ctx, 100)
	if err != nil {
		return nil, fmt.Errorf("获取上游 tag 列表: %w", err)
	}
	candidates := selectCandidateTags(tags, toRef, maxCandidates)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("没有可用的 release tag 用于内容指纹识别")
	}

	result := &FingerprintResult{}
	var lastErr error
	for _, tag := range candidates {
		tree, err := client.CommitTree(ctx, tag.Commit.SHA)
		if err != nil {
			lastErr = err
			continue
		}
		matched, total := scoreTree(local, tree)
		if total == 0 {
			continue
		}
		score := float64(matched) / float64(total)
		result.Candidates = append(result.Candidates, FingerprintCandidate{
			Tag: tag.Name, Score: score, Matched: matched, Total: total,
		})
		if score > result.Score {
			result.Score = score
			result.Tag = tag.Name
			result.Commit = tag.Commit.SHA
			result.Matched = matched
			result.Total = total
		}
	}
	if len(result.Candidates) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("获取上游文件树失败: %w", lastErr)
		}
		return nil, fmt.Errorf("没有可比对的上游文件树")
	}

	sort.SliceStable(result.Candidates, func(i, j int) bool {
		return result.Candidates[i].Score > result.Candidates[j].Score
	})
	second := 0.0
	if len(result.Candidates) > 1 {
		second = result.Candidates[1].Score
	}
	result.Detected = result.Score >= fingerprintMinScore &&
		(len(result.Candidates) == 1 || result.Score-second >= fingerprintMinMargin)
	switch {
	case result.Score >= 0.80 && result.Score-second >= 0.15:
		result.Confidence = "high"
	case result.Score >= 0.50:
		result.Confidence = "medium"
	default:
		result.Confidence = "low"
	}
	return result, nil
}
