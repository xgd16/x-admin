package model

import "time"

// APIUser 是自定义 API 面的终端用户。
//
// 每个 API 面使用独立的数据表（由 go tool xadmin create api --auth --table 指定），
// 因此模型本身不绑定 TableName，仓储统一通过 db.Table(scope.Table) 访问。
// 与后台管理账号 sys_users 完全独立：不参与 RBAC，不允许访问 /admin/v1。
type APIUser struct {
	Base
	Username       string     `gorm:"size:64;not null" json:"username"`
	Password       string     `gorm:"size:100;not null" json:"-"` // bcrypt 摘要，永不返回
	Nickname       string     `gorm:"size:64" json:"nickname"`
	Email          string     `gorm:"size:128;index" json:"email"`
	Phone          string     `gorm:"size:32;index" json:"phone"`
	Avatar         string     `gorm:"size:255" json:"avatar"`
	Status         int8       `gorm:"not null;index" json:"status"`
	SessionVersion int64      `gorm:"not null;default:0" json:"-"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	LastLoginIP    string     `gorm:"size:64" json:"last_login_ip"`
}
