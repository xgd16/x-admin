package model

import (
	"time"

	"x-admin/internal/pkg/pagination"
)

// UserFilter 是用户列表的查询条件。
// form 标签必须保留：列表与导出接口都通过 ShouldBindQuery 绑定查询参数。
type UserFilter struct {
	pagination.Query
	DeptID *uint64 `form:"dept_id" json:"dept_id"` // 按部门过滤
	Status *int8   `form:"status" json:"status"`   // 按状态过滤
	RoleID *uint64 `form:"role_id" json:"role_id"` // 按角色过滤
}

// RoleFilter 是角色列表的查询条件。
type RoleFilter struct {
	pagination.Query
	Status *int8 `form:"status" json:"status"`
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
	Username string     `form:"username" json:"username"`
	Status   *int       `form:"status" json:"status"`
	Start    *time.Time `form:"start" json:"start"`
	End      *time.Time `form:"end" json:"end"`
}

// LoginLogFilter 是登录日志的查询条件。
type LoginLogFilter struct {
	pagination.Query
	Username string     `form:"username" json:"username"`
	Status   *int       `form:"status" json:"status"`
	Start    *time.Time `form:"start" json:"start"`
	End      *time.Time `form:"end" json:"end"`
}

// TrendPoint 是登录趋势图上的一个数据点。
type TrendPoint struct {
	Date  string `json:"date"` // yyyy-MM-dd
	Count int64  `json:"count"`
}
