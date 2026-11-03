package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	StatusDisabled int8 = 0
	StatusEnabled  int8 = 1
)

const (
	MenuDirectory int8 = 1
	MenuMenu      int8 = 2
	MenuButton    int8 = 3
)

type Base struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
