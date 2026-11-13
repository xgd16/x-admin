package xadmin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// surfaceConfigRel 是 create api --auth 自动写入的本地配置文件。
const surfaceConfigRel = "configs/config.yaml"

// writeSurfaceConfig 把 API 面写入 configs/config.yaml 的 api 段。
//
// 返回写入的相对路径；配置文件不存在时返回空字符串，由调用方提示手动添加。
// 写入时按 API 面名称 A-Z 排序，方便对比和排查。
func writeSurfaceConfig(workDir, name, prefix, table string) (string, error) {
	path := filepath.Join(workDir, filepath.FromSlash(surfaceConfigRel))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("读取 %s: %w", surfaceConfigRel, err)
	}
	updated, err := upsertSurfaceConfig(data, name, prefix, table)
	if err != nil {
		return "", fmt.Errorf("更新 %s: %w", surfaceConfigRel, err)
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, updated, mode); err != nil {
		return "", fmt.Errorf("写入 %s: %w", surfaceConfigRel, err)
	}
	return surfaceConfigRel, nil
}

// upsertSurfaceConfig 在 YAML 文本中插入或更新 api.<name>，并保持 api 段按名称排序。
//
// 常规块状配置按原文行块重排，原有注释、空行和缩进都会保留；
// 只有内联写法（api: {client: ...}）等特殊情况才退回 YAML 节点重写。
func upsertSurfaceConfig(data []byte, name, prefix, table string) ([]byte, error) {
	text := string(data)
	trailingNewline := strings.HasSuffix(text, "\n")
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("解析 YAML: %w", err)
	}
	api, err := yamlAPINode(&doc)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(text, "\n")
	start, end := findTopLevelSection(lines, "api")
	if start < 0 && api == nil {
		// 没有 api 段时直接追加，保留文件其余内容。
		joined := appendAPISection(lines, name, prefix, table)
		return finishSurfaceConfig(joined, crlf, trailingNewline), nil
	}
	if start >= 0 {
		if out, ok := upsertSurfaceText(lines, start, end, api, name, prefix, table); ok {
			return finishSurfaceConfig(out, crlf, trailingNewline), nil
		}
	}

	// 内联 api 段等非常规写法：退回节点重写。
	updated, err := upsertSurfaceConfigYAML([]byte(text), name, prefix, table)
	if err != nil {
		return nil, err
	}
	return finishSurfaceConfig(strings.Split(string(updated), "\n"), crlf, trailingNewline), nil
}

func finishSurfaceConfig(lines []string, crlf, trailingNewline bool) []byte {
	joined := strings.Join(lines, "\n")
	if trailingNewline {
		joined = strings.TrimRight(joined, "\n") + "\n"
	} else {
		joined = strings.TrimRight(joined, "\n")
	}
	if crlf {
		joined = strings.ReplaceAll(joined, "\n", "\r\n")
	}
	return []byte(joined)
}

// yamlAPINode 返回顶层 api 节点的值节点；没有 api 时返回 nil。
func yamlAPINode(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind == 0 {
		return nil, nil
	}
	root := doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil, nil
		}
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("顶层不是 YAML 映射")
	}
	api := mappingValue(root, "api")
	if api == nil {
		return nil, nil
	}
	if api.Kind != yaml.MappingNode {
		if api.Kind == yaml.ScalarNode && api.Tag == "!!null" {
			return nil, nil
		}
		return nil, fmt.Errorf("api 不是 YAML 映射")
	}
	return api, nil
}

// surfaceTextEntry 是 api 段里一个 API 面的原文块。
type surfaceTextEntry struct {
	name   string
	lines  []string // 头部注释 + key 行 + 字段
	keyPos int      // key 行在 lines 中的位置
}

func (e *surfaceTextEntry) keyIndent() int {
	return leadingSpaces(e.lines[e.keyPos])
}

// fieldIndent 返回字段行的缩进，取面配置内的最小缩进。
func (e *surfaceTextEntry) fieldIndent() int {
	keyIndent := e.keyIndent()
	indent := -1
	for i := e.keyPos + 1; i < len(e.lines); i++ {
		trimmed := strings.TrimSpace(e.lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		current := leadingSpaces(e.lines[i])
		if current <= keyIndent {
			continue
		}
		if indent < 0 || current < indent {
			indent = current
		}
	}
	if indent < 0 {
		return keyIndent + 2
	}
	return indent
}

// upsertSurfaceText 按原文行块插入/更新 API 面并排序。
//
// 返回 ok=false 时表示当前写法不适合按行处理，由调用方退回 YAML 节点重写。
func upsertSurfaceText(lines []string, start, end int, api *yaml.Node, name, prefix, table string) ([]string, bool) {
	if api != nil && api.Kind != yaml.MappingNode {
		return nil, false
	}
	entries, preamble, trailer, separated, ok := parseSurfaceText(lines[start+1 : end])
	if !ok || !sameSurfaceNames(api, entries) {
		return nil, false
	}

	found := false
	for i := range entries {
		if entries[i].name != name {
			continue
		}
		entries[i].setFields(prefix, table)
		found = true
		break
	}
	if !found {
		keyIndent := detectIndentUnit(lines)
		if len(entries) > 0 {
			keyIndent = entries[0].keyIndent()
		}
		unit := detectIndentUnit(lines)
		entries = append(entries, newSurfaceTextEntry(name, prefix, table, keyIndent, unit))
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	apiLine := "api:"
	if comment := lineComment(lines[start]); comment != "" {
		apiLine += " " + comment
	}

	body := joinSurfaceText(preamble, entries, trailer, separated)
	var out []string
	out = append(out, lines[:start]...)
	out = append(out, apiLine)
	out = append(out, body...)
	if end < len(lines) && len(body) > 0 {
		last := body[len(body)-1]
		lastTrimmed := strings.TrimSpace(last)
		// 顶层注释通常属于下一个配置段，直接贴着它；其余情况补一个空行。
		if lastTrimmed != "" && !strings.HasPrefix(lastTrimmed, "#") {
			out = append(out, "")
		}
	}
	out = append(out, lines[end:]...)
	return out, true
}

// parseSurfaceText 把 api 段拆成「文件头注释 + 各 API 面原文块 + 尾注释」。
func parseSurfaceText(body []string) (entries []surfaceTextEntry, preamble, trailer []string, separated, ok bool) {
	keyIndent := -1
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(line)
		if keyIndent < 0 || indent < keyIndent {
			keyIndent = indent
		}
	}
	if keyIndent < 0 {
		return nil, trimTrailingBlank(body), nil, false, true
	}
	if keyIndent == 0 {
		return nil, nil, nil, false, false
	}

	type keyLine struct {
		index int
		name  string
	}
	var keys []keyLine
	for i, line := range body {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(line)
		if indent > keyIndent {
			// 面配置的字段行，跟随所属 API 面。
			continue
		}
		if indent < keyIndent {
			return nil, nil, nil, false, false
		}
		key, valid := mappingKey(trimmed)
		if !valid {
			return nil, nil, nil, false, false
		}
		keys = append(keys, keyLine{index: i, name: key})
	}
	if len(keys) == 0 {
		return nil, trimTrailingBlank(body), nil, false, true
	}

	// 紧贴在 key 上方的连续注释跟着这个面一起移动。
	starts := make([]int, len(keys))
	for i, key := range keys {
		start := key.index
		for start-1 >= 0 {
			prev := strings.TrimSpace(body[start-1])
			if !strings.HasPrefix(prev, "#") {
				break
			}
			start--
		}
		starts[i] = start
	}

	preamble = trimTrailingBlank(body[:starts[0]])
	for i, key := range keys {
		end := len(body)
		if i+1 < len(keys) {
			end = starts[i+1]
		}
		lines := append([]string(nil), body[starts[i]:end]...)
		entries = append(entries, surfaceTextEntry{
			name:   key.name,
			lines:  lines,
			keyPos: key.index - starts[i],
		})
	}

	// 最后一段里「空行 + 注释」通常描述下一个配置段，放到 api 段末尾，避免跟着面排序。
	last := &entries[len(entries)-1]
	lastContent := -1
	for i := len(last.lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(last.lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lastContent = i
			break
		}
	}
	if lastContent >= 0 && lastContent+1 < len(last.lines) {
		tail := last.lines[lastContent+1:]
		if onlyBlankAndComments(tail) && isSurfaceTrailer(tail, last.keyIndent()) {
			trailer = append([]string(nil), tail...)
			last.lines = last.lines[:lastContent+1]
		}
	}

	for i := range entries {
		trimmed := trimTrailingBlank(entries[i].lines)
		if i < len(entries)-1 && len(trimmed) < len(entries[i].lines) {
			separated = true
		}
		entries[i].lines = trimmed
	}
	return entries, preamble, trailer, separated, true
}

// joinSurfaceText 重新拼装 api 段内容，保持原有的空行风格。
func joinSurfaceText(preamble []string, entries []surfaceTextEntry, trailer []string, separated bool) []string {
	var out []string
	preamble = trimTrailingBlank(preamble)
	if len(preamble) > 0 {
		out = append(out, preamble...)
		out = append(out, "")
	}
	for i, entry := range entries {
		if i > 0 && separated {
			out = append(out, "")
		}
		out = append(out, entry.lines...)
	}
	trailer = trimTrailingBlank(trimLeadingBlank(trailer))
	if len(trailer) > 0 {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, trailer...)
	}
	return out
}

func sameSurfaceNames(api *yaml.Node, entries []surfaceTextEntry) bool {
	if api == nil {
		return len(entries) == 0
	}
	names := make(map[string]bool)
	for i := 0; i+1 < len(api.Content); i += 2 {
		names[api.Content[i].Value] = true
	}
	if len(names) != len(entries) {
		return false
	}
	for _, entry := range entries {
		if !names[entry.name] {
			return false
		}
	}
	return true
}

func newSurfaceTextEntry(name, prefix, table string, keyIndent, unit int) surfaceTextEntry {
	if keyIndent <= 0 {
		keyIndent = unit
	}
	key := strings.Repeat(" ", keyIndent) + name + ":"
	field := strings.Repeat(" ", keyIndent+unit)
	lines := []string{
		key,
		field + "enabled: true",
		field + "prefix: " + prefix,
	}
	if table != "" {
		lines = append(lines, field+"table: "+table)
	}
	return surfaceTextEntry{name: name, lines: lines}
}

// setFields 更新 enabled/prefix/table，保留字段行尾注释和其他字段。
func (e *surfaceTextEntry) setFields(prefix, table string) {
	indent := e.fieldIndent()
	fields := []struct {
		name  string
		value string
	}{
		{name: "enabled", value: "true"},
		{name: "prefix", value: prefix},
	}
	if table != "" {
		fields = append(fields, struct {
			name  string
			value string
		}{name: "table", value: table})
	}

	last := e.keyPos
	for _, field := range fields {
		index := e.findField(field.name, indent)
		if index < 0 {
			continue
		}
		e.lines[index] = replaceFieldValue(e.lines[index], field.value)
		if index > last {
			last = index
		}
	}
	var inserts []string
	for _, field := range fields {
		if e.findField(field.name, indent) >= 0 {
			continue
		}
		inserts = append(inserts, strings.Repeat(" ", indent)+field.name+": "+field.value)
	}
	if len(inserts) == 0 {
		return
	}
	position := last + 1
	merged := make([]string, 0, len(e.lines)+len(inserts))
	merged = append(merged, e.lines[:position]...)
	merged = append(merged, inserts...)
	merged = append(merged, e.lines[position:]...)
	e.lines = merged
}

func (e *surfaceTextEntry) findField(field string, indent int) int {
	for i := e.keyPos + 1; i < len(e.lines); i++ {
		trimmed := strings.TrimSpace(e.lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if leadingSpaces(e.lines[i]) != indent {
			continue
		}
		key, ok := mappingKey(trimmed)
		if ok && key == field {
			return i
		}
	}
	return -1
}

func replaceFieldValue(line, value string) string {
	trimmed := strings.TrimSpace(line)
	index := strings.Index(trimmed, ":")
	if index < 0 {
		return line
	}
	field := trimmed[:index]
	comment := commentSuffix(trimmed[index+1:])
	return strings.Repeat(" ", leadingSpaces(line)) + field + ": " + value + comment
}

// commentSuffix 取出行尾注释（含前面的空格）。
func commentSuffix(rest string) string {
	for i := 0; i < len(rest); i++ {
		if rest[i] != '#' {
			continue
		}
		if i == 0 || rest[i-1] == ' ' || rest[i-1] == '\t' {
			return " " + strings.TrimSpace(rest[i:])
		}
	}
	return ""
}

// appendAPISection 在文件末尾追加新的 api 段。
func appendAPISection(lines []string, name, prefix, table string) []string {
	lines = trimTrailingBlank(lines)
	unit := detectIndentUnit(lines)
	entry := newSurfaceTextEntry(name, prefix, table, unit, unit)
	var out []string
	out = append(out, lines...)
	if len(out) > 0 {
		out = append(out, "")
	}
	out = append(out, "api:")
	out = append(out, entry.lines...)
	out = append(out, "")
	return out
}

func mappingKey(trimmed string) (string, bool) {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
		return "", false
	}
	index := strings.Index(trimmed, ":")
	if index <= 0 {
		return "", false
	}
	key := strings.TrimSpace(trimmed[:index])
	if len(key) >= 2 && (key[0] == '"' || key[0] == '\'') && key[len(key)-1] == key[0] {
		key = key[1 : len(key)-1]
	}
	if key == "" {
		return "", false
	}
	rest := trimmed[index+1:]
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "#") {
		return "", false
	}
	return key, true
}

func onlyBlankAndComments(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return false
		}
	}
	return true
}

// isSurfaceTrailer 判断最后一段末尾的注释是否属于 api 段尾部（而不是最后一个面）。
// 空行隔开的注释，或缩进不比面名称更深的注释，都视为段尾注释。
func isSurfaceTrailer(tail []string, keyIndent int) bool {
	if strings.TrimSpace(tail[0]) == "" {
		return true
	}
	for _, line := range tail {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if leadingSpaces(line) > keyIndent {
			return false
		}
	}
	return true
}

func trimTrailingBlank(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

func trimLeadingBlank(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return lines[start:]
}

func leadingSpaces(line string) int {
	count := 0
	for count < len(line) && line[count] == ' ' {
		count++
	}
	return count
}

// detectIndentUnit 取文档里最小的正缩进，作为新配置的缩进单位。
func detectIndentUnit(lines []string) int {
	unit := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(line)
		if indent > 0 && (unit == 0 || indent < unit) {
			unit = indent
		}
	}
	if unit <= 0 {
		unit = 2
	}
	return unit
}

// upsertSurfaceConfigYAML 是内联 api 段的回退实现：按 YAML 节点重写 api 段。
func upsertSurfaceConfigYAML(data []byte, name, prefix, table string) ([]byte, error) {
	text := string(data)
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("解析 YAML: %w", err)
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			root = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			doc.Content = []*yaml.Node{root}
		} else {
			root = doc.Content[0]
		}
	} else if doc.Kind == 0 {
		// 空文件（或只有注释）时直接创建 api 段。
		root = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("顶层不是 YAML 映射")
	}

	api := mappingValue(root, "api")
	if api == nil {
		api = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, scalarNode("api"), api)
	} else if api.Kind != yaml.MappingNode {
		if api.Kind == yaml.ScalarNode && api.Tag == "!!null" {
			api.Kind = yaml.MappingNode
			api.Tag = "!!map"
			api.Value = ""
			api.Style = 0
		} else {
			return nil, fmt.Errorf("api 不是 YAML 映射")
		}
	}
	// 统一输出块状映射，避免沿用原来的流式写法导致换行后难以阅读。
	api.Style = 0
	upsertSurface(api, name, prefix, table)
	sortMapping(api)

	body, err := encodeYAMLNode(api)
	if err != nil {
		return nil, fmt.Errorf("序列化 api 段: %w", err)
	}
	section := indentLines(body, "  ")

	start, end := findTopLevelSection(strings.Split(text, "\n"), "api")
	var joined string
	if start < 0 {
		base := strings.TrimRight(text, "\n")
		if base == "" {
			joined = "api:\n" + section + "\n"
		} else {
			joined = base + "\n\napi:\n" + section + "\n"
		}
	} else {
		lines := strings.Split(text, "\n")
		last := end - 1
		for last >= start {
			trimmed := strings.TrimSpace(lines[last])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				last--
				continue
			}
			break
		}
		apiLine := "api:"
		if comment := lineComment(lines[start]); comment != "" {
			apiLine += " " + comment
		}
		var out []string
		out = append(out, lines[:start]...)
		out = append(out, apiLine)
		if section != "" {
			out = append(out, strings.Split(section, "\n")...)
		}
		tail := lines[last+1:]
		if len(tail) > 0 && strings.TrimSpace(tail[0]) != "" {
			out = append(out, "")
		}
		out = append(out, tail...)
		joined = strings.Join(out, "\n")
	}
	if crlf {
		joined = strings.ReplaceAll(joined, "\n", "\r\n")
	}
	return []byte(joined), nil
}

// mappingValue 返回映射中 key 对应的值节点。
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

type yamlPair struct {
	key   *yaml.Node
	value *yaml.Node
}

func mappingPairs(mapping *yaml.Node) []yamlPair {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	pairs := make([]yamlPair, 0, len(mapping.Content)/2)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		pairs = append(pairs, yamlPair{key: mapping.Content[i], value: mapping.Content[i+1]})
	}
	return pairs
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// upsertSurface 插入或更新一个 API 面配置。
func upsertSurface(api *yaml.Node, name, prefix, table string) {
	value := mappingValue(api, name)
	if value == nil {
		value = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		api.Content = append(api.Content, scalarNode(name), value)
	} else if value.Kind != yaml.MappingNode {
		*value = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	value.Style = 0
	setSurfaceField(value, "enabled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	setSurfaceField(value, "prefix", scalarNode(prefix))
	if table != "" {
		setSurfaceField(value, "table", scalarNode(table))
	}
	// enabled/prefix/table 固定在前面，issuer 等其他字段保持原有顺序。
	order := []string{"enabled", "prefix", "table"}
	pairs := mappingPairs(value)
	sort.SliceStable(pairs, func(i, j int) bool {
		return fieldRank(pairs[i].key.Value, order) < fieldRank(pairs[j].key.Value, order)
	})
	value.Content = value.Content[:0]
	for _, pair := range pairs {
		value.Content = append(value.Content, pair.key, pair.value)
	}
}

func setSurfaceField(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		// 保留用户写在字段上的注释。
		old := mapping.Content[i+1]
		value.HeadComment = old.HeadComment
		value.LineComment = old.LineComment
		value.FootComment = old.FootComment
		mapping.Content[i+1] = value
		return
	}
	mapping.Content = append(mapping.Content, scalarNode(key), value)
}

func fieldRank(key string, order []string) int {
	for i, name := range order {
		if key == name {
			return i
		}
	}
	return len(order)
}

// sortMapping 按 key 的字典序重排映射，保持 value 与注释的对应关系。
func sortMapping(mapping *yaml.Node) {
	pairs := mappingPairs(mapping)
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].key.Value < pairs[j].key.Value
	})
	mapping.Content = mapping.Content[:0]
	for _, pair := range pairs {
		mapping.Content = append(mapping.Content, pair.key, pair.value)
	}
}

func encodeYAMLNode(node *yaml.Node) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func indentLines(value, prefix string) string {
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// isTopLevelKey 判断一行是否是顶层 key（允许 key 与冒号之间有空格）。
func isTopLevelKey(line, key string) bool {
	if !strings.HasPrefix(line, key) {
		return false
	}
	rest := strings.TrimLeft(line[len(key):], " \t")
	return strings.HasPrefix(rest, ":")
}

// lineComment 返回行尾注释（含 #），没有注释时返回空字符串。
// 只有空白后的 # 才视为注释，避免把引号里的 # 当成注释。
func lineComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] != '#' {
			continue
		}
		if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
			return strings.TrimSpace(line[i:])
		}
	}
	return ""
}

// findTopLevelSection 找到顶层 key 的行区间 [start, end)。
// 找不到时返回 (-1, -1)。
func findTopLevelSection(lines []string, key string) (int, int) {
	start := -1
	for i, line := range lines {
		if !isTopLevelKey(line, key) {
			continue
		}
		start = i
		break
	}
	if start < 0 {
		return -1, -1
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if lines[i][0] != ' ' && lines[i][0] != '\t' {
			end = i
			break
		}
	}
	return start, end
}
