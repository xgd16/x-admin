package model

import (
	"time"

	"x-admin/internal/pkg/pagination"
)

// UserFilter 是用户列表的查询条件。
type UserFilter struct {
	pagination.Query
	DeptID *uint64 `json:"dept_id"` // 按部门过滤
	Status *int8   `json:"status"`  // 按状态过滤
	RoleID *uint64 `json:"role_id"` // 按角色过滤
}

// RoleFilter 是角色列表的查询条件。
type RoleFilter struct {
	pagination.Query
	Status *int8 `json:"status"`
}

// FileFilter 是文件列表的查询条件。
type FileFilter struct {
	pagination.Query
	MIME string `form:"mime" json:"mime"` // MIME 前缀匹配，例如 image/
	// Kind 按文件大类过滤，支持逗号组合：
	// image、video、audio、document、archive、other。
	Kind string `form:"kind" json:"kind"`
}

// OperationLogFilter 是操作日志的查询条件。
type OperationLogFilter struct {
	pagination.Query
	Username string     `json:"username"`
	Status   *int       `json:"status"`
	Start    *time.Time `json:"start"`
	End      *time.Time `json:"end"`
}

// LoginLogFilter 是登录日志的查询条件。
type LoginLogFilter struct {
	pagination.Query
	Username string     `json:"username"`
	Status   *int       `json:"status"`
	Start    *time.Time `json:"start"`
	End      *time.Time `json:"end"`
}

// TrendPoint 是登录趋势图上的一个数据点。
type TrendPoint struct {
	Date  string `json:"date"` // yyyy-MM-dd
	Count int64  `json:"count"`
}
