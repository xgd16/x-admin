package model

import "time"

// Department 是组织架构中的部门节点，通过 ParentID 组成树。
type Department struct {
	Base
	ParentID uint64 `gorm:"not null;default:0;index" json:"parent_id"` // 0 表示根部门
	Name     string `gorm:"size:64;not null" json:"name"`
	Leader   string `gorm:"size:64" json:"leader"`
	Phone    string `gorm:"size:32" json:"phone"`
	Email    string `gorm:"size:128" json:"email"`
	Sort     int    `gorm:"not null;default:0" json:"sort"`
	Status   int8   `gorm:"not null;index" json:"status"`
}

// TableName 指定部门表名。
func (Department) TableName() string { return "sys_departments" }

// DictType 是字典类型，Code 唯一，例如 user_status。
type DictType struct {
	Base
	Name   string `gorm:"size:64;not null" json:"name"`
	Code   string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Status int8   `gorm:"not null;index" json:"status"`
	Remark string `gorm:"size:255" json:"remark"`
}

// TableName 指定字典类型表名。
func (DictType) TableName() string { return "sys_dict_types" }

// DictItem 是字典项，Label 用于展示，Value 用于存储。
type DictItem struct {
	Base
	DictTypeID uint64 `gorm:"not null;index" json:"dict_type_id"`
	Label      string `gorm:"size:64;not null" json:"label"`
	Value      string `gorm:"size:64;not null" json:"value"`
	Sort       int    `gorm:"not null;default:0" json:"sort"`
	Status     int8   `gorm:"not null;index" json:"status"`
	TagColor   string `gorm:"size:32" json:"tag_color"` // 前端标签颜色
}

// TableName 指定字典项表名。
func (DictItem) TableName() string { return "sys_dict_items" }

// Parameter 是系统参数，Key 唯一；Public 表示是否允许非管理员读取。
type Parameter struct {
	Base
	Key    string `gorm:"size:128;uniqueIndex;not null" json:"key"`
	Value  string `gorm:"type:text" json:"value"`
	Name   string `gorm:"size:64;not null" json:"name"`
	Public bool   `gorm:"not null;default:false" json:"public"`
	Remark string `gorm:"size:255" json:"remark"`
}

// TableName 指定系统参数表名。
func (Parameter) TableName() string { return "sys_parameters" }

// File 是上传文件的元数据，Key 为存储层的相对路径。
type File struct {
	Base
	Name       string `gorm:"size:255;not null" json:"name"` // 原始文件名
	Key        string `gorm:"size:255;not null;uniqueIndex" json:"key"`
	URL        string `gorm:"size:255;not null" json:"url"`
	MIME       string `gorm:"size:128" json:"mime"`
	Ext        string `gorm:"size:32" json:"ext"`
	Size       int64  `gorm:"not null;default:0" json:"size"`
	UploaderID uint64 `gorm:"index" json:"uploader_id"`
}

// TableName 指定文件表名。
func (File) TableName() string { return "sys_files" }

// Job 是定时任务定义，Handler 决定实际执行逻辑，Cron 为标准 5 段表达式。
type Job struct {
	Base
	Name       string     `gorm:"size:64;not null" json:"name"`
	Cron       string     `gorm:"size:64;not null" json:"cron"`
	Handler    string     `gorm:"size:64;not null" json:"handler"` // clean_logs | clean_notifications | noop
	Params     string     `gorm:"type:text" json:"params"`         // JSON 参数
	Status     int8       `gorm:"not null;index" json:"status"`
	LastRunAt  *time.Time `json:"last_run_at"`
	LastResult string     `gorm:"size:255" json:"last_result"`
	Remark     string     `gorm:"size:255" json:"remark"`
}

// TableName 指定定时任务表名。
func (Job) TableName() string { return "sys_jobs" }
