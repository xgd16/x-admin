package model

import "time"

// 异步导出任务状态。
const (
	ExportPending  int8 = 0 // 排队中
	ExportRunning  int8 = 1 // 生成中
	ExportSuccess  int8 = 2 // 已完成
	ExportFailed   int8 = 3 // 失败
	ExportCanceled int8 = 4 // 已取消
)

// ExportTask 是一次异步导出任务的记录，包含进度与产物文件信息。
// FileKey 是存储层的相对路径，不下发到前端；下载一律走导出接口。
type ExportTask struct {
	Base
	UserID     uint64     `gorm:"not null;index:idx_export_tasks_user,priority:1" json:"user_id"`
	Module     string     `gorm:"size:64;not null" json:"module"`
	Title      string     `gorm:"size:128;not null" json:"title"`
	Status     int8       `gorm:"not null;default:0;index" json:"status"`
	Total      int64      `gorm:"not null;default:0" json:"total"`
	Processed  int64      `gorm:"not null;default:0" json:"processed"`
	FileKey    string     `gorm:"size:255" json:"-"`
	FileName   string     `gorm:"size:255" json:"file_name"`
	FileSize   int64      `gorm:"not null;default:0" json:"file_size"`
	Error      string     `gorm:"size:512" json:"error,omitempty"`
	Canceled   bool       `gorm:"not null;default:false" json:"-"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// TableName 指定导出任务表名。
func (ExportTask) TableName() string { return "sys_export_tasks" }
