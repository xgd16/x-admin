// Package model 定义数据库模型与查询过滤器。
//
// 该包只包含数据结构与 GORM 标签，不写业务逻辑；表名统一通过 TableName 指定。
package model

import (
	"time"

	"gorm.io/gorm"
)

// 通用启用/禁用状态。
const (
	StatusDisabled int8 = 0 // 停用
	StatusEnabled  int8 = 1 // 启用
)

// 菜单类型：目录只用于分组，菜单对应一个页面，按钮对应一个权限点。
const (
	MenuDirectory int8 = 1 // 目录
	MenuMenu      int8 = 2 // 菜单
	MenuButton    int8 = 3 // 按钮
)

// Base 是所有业务表共用的主键与时间戳字段，删除采用软删除。
type Base struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 软删除标记，不返回给前端
}
