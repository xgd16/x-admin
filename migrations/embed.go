// Package migrations 持有版本化 SQL 迁移脚本。
package migrations

import "embed"

// FS 是构建时嵌入二进制的全部 .sql 迁移文件，启动时由 golang-migrate 执行。
//
//go:embed *.sql
var FS embed.FS
