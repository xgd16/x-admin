package model

import "time"

// User 是系统账号，包含资料、所属部门、状态以及双重验证相关字段。
type User struct {
	Base
	Username    string      `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Password    string      `gorm:"size:100;not null" json:"-"` // bcrypt 摘要，永不返回
	Nickname    string      `gorm:"size:64" json:"nickname"`
	Email       string      `gorm:"size:128;index" json:"email"`
	Phone       string      `gorm:"size:32;index" json:"phone"`
	Avatar      string      `gorm:"size:255" json:"avatar"`
	DeptID      *uint64     `gorm:"index" json:"dept_id"` // 可为空，表示未分配部门
	Dept        *Department `gorm:"foreignKey:DeptID" json:"dept,omitempty"`
	Status      int8        `gorm:"not null;index" json:"status"`
	LastLoginAt *time.Time  `json:"last_login_at"`
	LastLoginIP string      `gorm:"size:64" json:"last_login_ip"`
	Remark      string      `gorm:"size:255" json:"remark"`
	// TOTP 用于 Google Authenticator 形式的双重验证。
	// 密钥与恢复码摘要只能在服务端使用，绝不能下发到客户端。
	TOTPSecret        string `gorm:"size:64" json:"-"`
	TOTPEnabled       bool   `gorm:"not null;default:false" json:"totp_enabled"`
	TOTPRecoveryCodes string `gorm:"size:1024" json:"-"` // 恢复码 SHA-256 摘要的 JSON 数组
	// SessionVersion 用于让安全变更（例如开启双重验证）之前签发的 refresh token 立即失效。
	SessionVersion int64  `gorm:"not null;default:0" json:"-"`
	Roles          []Role `gorm:"many2many:sys_user_roles;constraint:OnDelete:CASCADE" json:"roles,omitempty"`
}

// TableName 指定用户表名。
func (User) TableName() string { return "sys_users" }

// Role 是 RBAC 角色，Code 为稳定标识，super_admin 拥有全部权限。
type Role struct {
	Base
	Name   string `gorm:"size:64;not null" json:"name"`
	Code   string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Status int8   `gorm:"not null;index" json:"status"`
	Sort   int    `gorm:"not null;default:0" json:"sort"`
	Remark string `gorm:"size:255" json:"remark"`
	Menus  []Menu `gorm:"many2many:sys_role_menus;constraint:OnDelete:CASCADE" json:"menus,omitempty"`
}

// TableName 指定角色表名。
func (Role) TableName() string { return "sys_roles" }

// Menu 是目录、菜单或按钮节点，Permission 为按钮级权限码。
type Menu struct {
	Base
	ParentID   uint64 `gorm:"not null;default:0;index" json:"parent_id"` // 0 表示根节点
	Name       string `gorm:"size:64;not null" json:"name"`
	Type       int8   `gorm:"not null;default:2;index" json:"type"` // 1 目录 / 2 菜单 / 3 按钮
	Path       string `gorm:"size:200" json:"path"`
	Component  string `gorm:"size:200" json:"component"`
	Permission string `gorm:"size:100;index" json:"permission"` // 形如 system:user:list
	Icon       string `gorm:"size:64" json:"icon"`
	Sort       int    `gorm:"not null;default:0" json:"sort"`
	Visible    bool   `gorm:"not null" json:"visible"` // 是否在导航中显示
	Status     int8   `gorm:"not null;index" json:"status"`
}

// TableName 指定菜单表名。
func (Menu) TableName() string { return "sys_menus" }

// UserRole 是用户与角色的多对多关联。
type UserRole struct {
	UserID uint64 `gorm:"primaryKey" json:"user_id"`
	RoleID uint64 `gorm:"primaryKey" json:"role_id"`
}

// TableName 指定用户角色关联表名。
func (UserRole) TableName() string { return "sys_user_roles" }

// RoleMenu 是角色与菜单的多对多关联。
type RoleMenu struct {
	RoleID uint64 `gorm:"primaryKey" json:"role_id"`
	MenuID uint64 `gorm:"primaryKey" json:"menu_id"`
}

// TableName 指定角色菜单关联表名。
func (RoleMenu) TableName() string { return "sys_role_menus" }
