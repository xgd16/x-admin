package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
)

// FilenameTemplateKey 是导出文件名模板在 sys_parameters 中的参数键。
// 通过导出记录页的可视化配置维护，参数管理页只读展示。
const FilenameTemplateKey = "export.filename_template"

// 文件名片段类型：任务标题、模块标识、时间戳、自定义文本。
const (
	FilenameSegmentTitle  = "title"
	FilenameSegmentModule = "module"
	FilenameSegmentTime   = "time"
	FilenameSegmentText   = "text"
)

// FilenameTimeFormats 是时间片段允许的格式（Go layout），
// 白名单保证生成的文件名在 Windows 等平台安全（不含冒号等非法字符）。
var FilenameTimeFormats = []string{
	"20060102150405",      // 20261108195800
	"2006-01-02",          // 2026-11-08
	"2006-01-02_1504",     // 2026-11-08_1958
	"2006-01-02_15-04-05", // 2026-11-08_19-58-00
}

// maxFilenameSegments 限制片段数量，防止配置被塞入过多内容。
const maxFilenameSegments = 8

// FilenameSegment 是文件名模板中的一个片段。
type FilenameSegment struct {
	Type   string `json:"type"`             // title | module | time | text
	Format string `json:"format,omitempty"` // time 片段的时间格式（Go layout）
	Value  string `json:"value,omitempty"`  // text 片段的固定文本
}

// FilenameTemplate 是全局的导出文件名模板：片段按顺序拼接成下载文件名。
type FilenameTemplate struct {
	Segments []FilenameSegment `json:"segments"`
}

// DefaultFilenameTemplate 返回默认模板：模块标识 + 紧凑时间戳，
// 与未引入模板功能前的文件名行为保持一致。
func DefaultFilenameTemplate() FilenameTemplate {
	return FilenameTemplate{Segments: []FilenameSegment{
		{Type: FilenameSegmentModule},
		{Type: FilenameSegmentTime, Format: "20060102150405"},
	}}
}

// FilenameTemplate 返回当前生效的文件名模板。
// 参数不存在或内容非法时回退默认模板；查询失败（如数据库不可用）返回错误，
// 避免静默回退让用户误以为配置丢失。
func (s *ExportService) FilenameTemplate(ctx context.Context) (FilenameTemplate, error) {
	p, err := s.params.GetByKey(ctx, FilenameTemplateKey)
	switch {
	case err == nil:
	case IsNotFound(err):
		return DefaultFilenameTemplate(), nil
	default:
		return FilenameTemplate{}, err
	}
	if p.Value == "" {
		return DefaultFilenameTemplate(), nil
	}
	var tpl FilenameTemplate
	if json.Unmarshal([]byte(p.Value), &tpl) != nil {
		return DefaultFilenameTemplate(), nil
	}
	normalized, ok := normalizeFilenameTemplate(tpl)
	if !ok {
		return DefaultFilenameTemplate(), nil
	}
	return normalized, nil
}

// UpdateFilenameTemplate 校验并保存文件名模板。
func (s *ExportService) UpdateFilenameTemplate(ctx context.Context, tpl FilenameTemplate) (FilenameTemplate, error) {
	normalized, ok := normalizeFilenameTemplate(tpl)
	if !ok {
		return FilenameTemplate{}, apperr.New(apperr.CodeInvalidArgument, "文件名模板不合法：至少保留一个片段，标题/模块/时间最多一个，时间格式需在可选列表内")
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return FilenameTemplate{}, err
	}
	value := string(raw)
	existing, err := s.params.GetByKey(ctx, FilenameTemplateKey)
	switch {
	case err == nil:
		existing.Value = value
		err = s.params.Update(ctx, existing)
	case IsNotFound(err):
		err = s.params.Create(ctx, &model.Parameter{
			Key:    FilenameTemplateKey,
			Value:  value,
			Name:   "导出文件名模板",
			Public: false,
			Remark: "在“导出记录”页可视化配置；值为 JSON 片段数组，请勿在此直接编辑",
		})
	default:
	}
	if err != nil {
		return FilenameTemplate{}, err
	}
	return normalized, nil
}

// normalizeFilenameTemplate 校验并规整模板：过滤未知类型、去重、截断文本。
// 返回 false 表示没有任何可用片段，需要回退默认模板。
func normalizeFilenameTemplate(tpl FilenameTemplate) (FilenameTemplate, bool) {
	seen := make(map[string]bool, 4)
	out := make([]FilenameSegment, 0, len(tpl.Segments))
	for _, seg := range tpl.Segments {
		if len(out) >= maxFilenameSegments {
			break
		}
		switch seg.Type {
		case FilenameSegmentTitle, FilenameSegmentModule:
			if seen[seg.Type] {
				continue
			}
			seen[seg.Type] = true
			out = append(out, FilenameSegment{Type: seg.Type})
		case FilenameSegmentTime:
			if seen[seg.Type] {
				continue
			}
			if !validTimeFormat(seg.Format) {
				continue
			}
			seen[seg.Type] = true
			out = append(out, FilenameSegment{Type: seg.Type, Format: seg.Format})
		case FilenameSegmentText:
			value := strings.TrimSpace(seg.Value)
			if value == "" {
				continue
			}
			out = append(out, FilenameSegment{Type: seg.Type, Value: truncateRunes(value, 32)})
		default:
			// 未知片段类型直接丢弃，兼容旧配置。
		}
	}
	if len(out) == 0 {
		return FilenameTemplate{}, false
	}
	return FilenameTemplate{Segments: out}, true
}

// validTimeFormat 判断时间格式是否在白名单内。
func validTimeFormat(format string) bool {
	for _, f := range FilenameTimeFormats {
		if f == format {
			return true
		}
	}
	return false
}

// ComposeFilename 按模板把任务信息拼成下载文件名（不含扩展名）。
// 任一片段拼出空串则跳过；整体为空时回退“模块-时间”。
// 后台生成与文件名预览共用此函数，保证所见即所得。
func ComposeFilename(tpl FilenameTemplate, module, title string, now time.Time) string {
	parts := make([]string, 0, len(tpl.Segments))
	for _, seg := range tpl.Segments {
		switch seg.Type {
		case FilenameSegmentTitle:
			parts = append(parts, sanitizeFilenamePart(title))
		case FilenameSegmentModule:
			parts = append(parts, sanitizeFilenamePart(module))
		case FilenameSegmentTime:
			parts = append(parts, now.Format(seg.Format))
		case FilenameSegmentText:
			parts = append(parts, sanitizeFilenamePart(seg.Value))
		}
	}
	name := sanitizeFilenameJoin(parts)
	if name == "" {
		name = sanitizeFilenamePart(module) + "-" + now.Format("20060102150405")
	}
	return truncateRunes(name, 100)
}

// filenameInvalidChars 是各操作系统通用的文件名非法字符。
var filenameInvalidChars = strings.NewReplacer(
	"/", "-", "\\", "-", ":", "-", "*", "", "?", "", "\"", "",
	"<", "", ">", "", "|", "", "\t", "", "\n", "", "\r", "",
)

// sanitizeFilenamePart 清洗单个片段：去非法字符与首尾空白。
func sanitizeFilenamePart(part string) string {
	return strings.TrimSpace(filenameInvalidChars.Replace(part))
}

// sanitizeFilenameJoin 用下划线拼接片段，并压缩连续的下划线。
func sanitizeFilenameJoin(parts []string) string {
	joined := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			joined = append(joined, p)
		}
	}
	name := strings.Join(joined, "_")
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	return name
}
