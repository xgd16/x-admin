package model

import "time"

type User struct {
	Base
	Username    string      `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Password    string      `gorm:"size:100;not null" json:"-"`
	Nickname    string      `gorm:"size:64" json:"nickname"`
	Email       string      `gorm:"size:128;index" json:"email"`
	Phone       string      `gorm:"size:32;index" json:"phone"`
	Avatar      string      `gorm:"size:255" json:"avatar"`
	DeptID      *uint64     `gorm:"index" json:"dept_id"`
	Dept        *Department `gorm:"foreignKey:DeptID" json:"dept,omitempty"`
	Status      int8        `gorm:"not null;index" json:"status"`
	LastLoginAt *time.Time  `json:"last_login_at"`
	LastLoginIP string      `gorm:"size:64" json:"last_login_ip"`
	Remark      string      `gorm:"size:255" json:"remark"`
	Roles       []Role      `gorm:"many2many:sys_user_roles;constraint:OnDelete:CASCADE" json:"roles,omitempty"`
}

func (User) TableName() string { return "sys_users" }

type Role struct {
	Base
	Name   string `gorm:"size:64;not null" json:"name"`
	Code   string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Status int8   `gorm:"not null;index" json:"status"`
	Sort   int    `gorm:"not null;default:0" json:"sort"`
	Remark string `gorm:"size:255" json:"remark"`
	Menus  []Menu `gorm:"many2many:sys_role_menus;constraint:OnDelete:CASCADE" json:"menus,omitempty"`
}

func (Role) TableName() string { return "sys_roles" }

type Menu struct {
	Base
	ParentID   uint64 `gorm:"not null;default:0;index" json:"parent_id"`
	Name       string `gorm:"size:64;not null" json:"name"`
	Type       int8   `gorm:"not null;default:2;index" json:"type"`
	Path       string `gorm:"size:200" json:"path"`
	Component  string `gorm:"size:200" json:"component"`
	Permission string `gorm:"size:100;index" json:"permission"`
	Icon       string `gorm:"size:64" json:"icon"`
	Sort       int    `gorm:"not null;default:0" json:"sort"`
	Visible    bool   `gorm:"not null" json:"visible"`
	Status     int8   `gorm:"not null;index" json:"status"`
}

func (Menu) TableName() string { return "sys_menus" }

type UserRole struct {
	UserID uint64 `gorm:"primaryKey" json:"user_id"`
	RoleID uint64 `gorm:"primaryKey" json:"role_id"`
}

func (UserRole) TableName() string { return "sys_user_roles" }

type RoleMenu struct {
	RoleID uint64 `gorm:"primaryKey" json:"role_id"`
	MenuID uint64 `gorm:"primaryKey" json:"menu_id"`
}

func (RoleMenu) TableName() string { return "sys_role_menus" }
