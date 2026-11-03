package model

import (
	"time"

	"x-admin/internal/pkg/pagination"
)

type UserFilter struct {
	pagination.Query
	DeptID *uint64 `json:"dept_id"`
	Status *int8   `json:"status"`
	RoleID *uint64 `json:"role_id"`
}

type RoleFilter struct {
	pagination.Query
	Status *int8 `json:"status"`
}

type FileFilter struct {
	pagination.Query
	MIME string `form:"mime" json:"mime"`
	// Kind filters by a comma-separated list of file kinds:
	// image, video, audio, document, archive, other.
	Kind string `form:"kind" json:"kind"`
}

type OperationLogFilter struct {
	pagination.Query
	Username string     `json:"username"`
	Status   *int       `json:"status"`
	Start    *time.Time `json:"start"`
	End      *time.Time `json:"end"`
}

type LoginLogFilter struct {
	pagination.Query
	Username string     `json:"username"`
	Status   *int       `json:"status"`
	Start    *time.Time `json:"start"`
	End      *time.Time `json:"end"`
}

type TrendPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}
