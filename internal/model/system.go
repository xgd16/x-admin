package model

import "time"

type Department struct {
	Base
	ParentID uint64 `gorm:"not null;default:0;index" json:"parent_id"`
	Name     string `gorm:"size:64;not null" json:"name"`
	Leader   string `gorm:"size:64" json:"leader"`
	Phone    string `gorm:"size:32" json:"phone"`
	Email    string `gorm:"size:128" json:"email"`
	Sort     int    `gorm:"not null;default:0" json:"sort"`
	Status   int8   `gorm:"not null;index" json:"status"`
}

func (Department) TableName() string { return "sys_departments" }

type DictType struct {
	Base
	Name   string `gorm:"size:64;not null" json:"name"`
	Code   string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Status int8   `gorm:"not null;index" json:"status"`
	Remark string `gorm:"size:255" json:"remark"`
}

func (DictType) TableName() string { return "sys_dict_types" }

type DictItem struct {
	Base
	DictTypeID uint64 `gorm:"not null;index" json:"dict_type_id"`
	Label      string `gorm:"size:64;not null" json:"label"`
	Value      string `gorm:"size:64;not null" json:"value"`
	Sort       int    `gorm:"not null;default:0" json:"sort"`
	Status     int8   `gorm:"not null;index" json:"status"`
	TagColor   string `gorm:"size:32" json:"tag_color"`
}

func (DictItem) TableName() string { return "sys_dict_items" }

type Parameter struct {
	Base
	Key    string `gorm:"size:128;uniqueIndex;not null" json:"key"`
	Value  string `gorm:"type:text" json:"value"`
	Name   string `gorm:"size:64;not null" json:"name"`
	Public bool   `gorm:"not null;default:false" json:"public"`
	Remark string `gorm:"size:255" json:"remark"`
}

func (Parameter) TableName() string { return "sys_parameters" }

type File struct {
	Base
	Name       string `gorm:"size:255;not null" json:"name"`
	Key        string `gorm:"size:255;not null;uniqueIndex" json:"key"`
	URL        string `gorm:"size:255;not null" json:"url"`
	MIME       string `gorm:"size:128" json:"mime"`
	Ext        string `gorm:"size:32" json:"ext"`
	Size       int64  `gorm:"not null;default:0" json:"size"`
	UploaderID uint64 `gorm:"index" json:"uploader_id"`
}

func (File) TableName() string { return "sys_files" }

type Job struct {
	Base
	Name       string     `gorm:"size:64;not null" json:"name"`
	Cron       string     `gorm:"size:64;not null" json:"cron"`
	Handler    string     `gorm:"size:64;not null" json:"handler"`
	Params     string     `gorm:"type:text" json:"params"`
	Status     int8       `gorm:"not null;index" json:"status"`
	LastRunAt  *time.Time `json:"last_run_at"`
	LastResult string     `gorm:"size:255" json:"last_result"`
	Remark     string     `gorm:"size:255" json:"remark"`
}

func (Job) TableName() string { return "sys_jobs" }
