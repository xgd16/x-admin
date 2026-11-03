package model

import "time"

// 通知类型，前端据此选择图标与颜色。
const (
	NotificationInfo    = "info"
	NotificationSuccess = "success"
	NotificationWarning = "warning"
	NotificationError   = "error"
)

// Notification 是命令栏中展示的站内消息。
// UserID 将消息限定给单个收件人，Link 指向消息相关的页面。
type Notification struct {
	Base
	UserID  uint64     `gorm:"not null;index:idx_notifications_user_read" json:"user_id"`
	Title   string     `gorm:"size:128;not null" json:"title"`
	Content string     `gorm:"size:500;not null;default:''" json:"content"`
	Type    string     `gorm:"size:16;not null;default:'info'" json:"type"`
	Link    string     `gorm:"size:255;not null;default:''" json:"link"`
	ActorID uint64     `gorm:"not null;default:0" json:"actor_id"`               // 触发该消息的操作人，0 表示系统
	ReadAt  *time.Time `gorm:"index:idx_notifications_user_read" json:"read_at"` // 为空表示未读
}

// TableName 指定通知表名。
func (Notification) TableName() string { return "sys_notifications" }
