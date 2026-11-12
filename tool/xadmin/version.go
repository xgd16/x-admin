package xadmin

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// gitDescribeSuffixRe 匹配 git describe 的 "5-gabc123" / "5-gabc123-dirty" 后缀。
// 这类后缀是提交距离信息，不是语义化版本的预发布标记。
var gitDescribeSuffixRe = regexp.MustCompile(`^\d+-g[0-9a-f]+(?:-dirty)?$`)

// Version 是简化的语义化版本，只用于比较 Release tag。
type Version struct {
	Major int
	Minor int
	Patch int
	Pre   string
}

// ParseVersion 解析 v1.2.3、1.2.3-rc.1、v1.2.3-5-gabc 这类字符串。
// 解析失败时返回 false，调用方应退化为字符串比较。
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return Version{}, false
	}
	// 只取开头的数字段，允许后面跟 -pre、+build 或 git describe 的后缀。
	versionPart := s
	if i := strings.IndexByte(versionPart, '+'); i >= 0 {
		versionPart = versionPart[:i]
	}
	core := versionPart
	if i := strings.IndexByte(core, '-'); i >= 0 {
		core = core[:i]
	}
	parts := strings.Split(core, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return Version{}, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, false
		}
		nums[i] = n
	}
	pre := ""
	if i := strings.IndexByte(versionPart, '-'); i >= 0 {
		suffix := versionPart[i+1:]
		if !gitDescribeSuffixRe.MatchString(suffix) {
			pre = suffix
		}
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre}, true
}

// CompareVersion 返回 -1、0、1。无法解析时按字符串是否相等退化。
func CompareVersion(a, b string) int {
	va, oka := ParseVersion(a)
	vb, okb := ParseVersion(b)
	if !oka || !okb {
		switch {
		case a == b:
			return 0
		case a < b:
			return -1
		default:
			return 1
		}
	}
	if va.Major != vb.Major {
		return cmpInt(va.Major, vb.Major)
	}
	if va.Minor != vb.Minor {
		return cmpInt(va.Minor, vb.Minor)
	}
	if va.Patch != vb.Patch {
		return cmpInt(va.Patch, vb.Patch)
	}
	// 正式版高于同号预发布版。
	switch {
	case va.Pre == "" && vb.Pre == "":
		return 0
	case va.Pre == "":
		return 1
	case vb.Pre == "":
		return -1
	case va.Pre == vb.Pre:
		return 0
	case va.Pre < vb.Pre:
		return -1
	default:
		return 1
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// IsUpToDate 判断 latest 是否不高于 current。
func IsUpToDate(current, latest string) bool {
	if current == "" || latest == "" {
		return false
	}
	return CompareVersion(latest, current) <= 0
}

// IsSHA 判断字符串是否像完整的 git commit SHA。
func IsSHA(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// ShortSHA 返回前 8 位，方便输出。
func ShortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// FormatVersion 输出 vX.Y.Z 形式。
func FormatVersion(v Version) string {
	s := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}
