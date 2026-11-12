package xadmin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// detectConfigAdditions 对比上游 config.example.yaml 与本地 configs/config.yaml。
// 只报告缺失的键，不读取也不回写任何敏感值。
func detectConfigAdditions(target Tree, workDir string) []ConfigChange {
	example, ok := target["configs/config.example.yaml"]
	if !ok {
		return nil
	}
	userPath := filepath.Join(workDir, "configs", "config.yaml")
	userData, err := os.ReadFile(userPath)
	if err != nil {
		return nil
	}
	exampleFlat, err := FlattenYAML(example.Data)
	if err != nil {
		return nil
	}
	userFlat, err := FlattenYAML(userData)
	if err != nil {
		return nil
	}
	var added []ConfigChange
	for key, val := range exampleFlat {
		if _, exists := userFlat[key]; !exists {
			added = append(added, ConfigChange{Key: key, Value: val})
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i].Key < added[j].Key })
	return added
}

// ConfigRemovedKeys 返回本地有、上游模板已经没有的配置键。doctor 使用。
func ConfigRemovedKeys(example, user []byte) ([]ConfigChange, error) {
	exampleFlat, err := FlattenYAML(example)
	if err != nil {
		return nil, err
	}
	userFlat, err := FlattenYAML(user)
	if err != nil {
		return nil, err
	}
	var removed []ConfigChange
	for key, val := range userFlat {
		if _, exists := exampleFlat[key]; !exists {
			removed = append(removed, ConfigChange{Key: key, Value: val})
		}
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i].Key < removed[j].Key })
	return removed, nil
}

// FlattenYAML 把嵌套 YAML 展平成 a.b.c -> 值字符串。
func FlattenYAML(data []byte) (map[string]string, error) {
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	flattenValue("", root, out)
	return out, nil
}

func flattenValue(prefix string, v any, out map[string]string) {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			if prefix != "" {
				out[prefix] = "{}"
			}
			return
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			next := k
			if prefix != "" {
				next = prefix + "." + k
			}
			flattenValue(next, val[k], out)
		}
	case map[any]any:
		converted := make(map[string]any, len(val))
		for k, vv := range val {
			converted[fmt.Sprint(k)] = vv
		}
		flattenValue(prefix, converted, out)
	case []any:
		if prefix != "" {
			out[prefix] = fmt.Sprintf("[%d items]", len(val))
		}
	case nil:
		if prefix != "" {
			out[prefix] = "null"
		}
	default:
		if prefix != "" {
			out[prefix] = fmt.Sprint(val)
		}
	}
}
