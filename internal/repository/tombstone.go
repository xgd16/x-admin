package repository

import "fmt"

// tombstoneMarker 标记被软删除记录的唯一字段。
// 它不是合法用户名/角色编码/参数键，因此不会与正常数据冲突。
const tombstoneMarker = "#deleted#"

// tombstoneValue 生成软删除后的唯一字段值。
// 唯一后缀放在末尾，即使原始值很长被截断，后缀仍然保留，保证不会撞唯一索引。
func tombstoneValue(value string, id uint64, maxLen int) string {
	suffix := fmt.Sprintf("%s%d", tombstoneMarker, id)
	if maxLen <= len(suffix) {
		return suffix
	}
	keep := maxLen - len(suffix)
	if len(value) > keep {
		value = value[:keep]
	}
	return value + suffix
}
