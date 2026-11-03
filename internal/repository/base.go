// Package repository 实现 GORM 数据访问。
//
// 每个聚合一个仓储，只暴露 service 需要的方法；通用 CRUD 由泛型 Base 提供。
package repository

import (
	"context"

	"gorm.io/gorm"
)

// ErrNotFound 表示查询的行不存在。
var ErrNotFound = gorm.ErrRecordNotFound

// Base 提供所有仓储共享的 CRUD 原语。
type Base[T any] struct {
	db *gorm.DB
}

// NewBase 为实体类型 T 创建通用仓储。
func NewBase[T any](db *gorm.DB) *Base[T] { return &Base[T]{db: db} }

// Create 插入一条记录。
func (b *Base[T]) Create(ctx context.Context, entity *T) error {
	return b.db.WithContext(ctx).Create(entity).Error
}

// Update 保存实体的全部字段（含零值）。
func (b *Base[T]) Update(ctx context.Context, entity *T) error {
	return b.db.WithContext(ctx).Save(entity).Error
}

// UpdateFields 只更新指定的字段；没有匹配行时返回 ErrNotFound。
func (b *Base[T]) UpdateFields(ctx context.Context, id uint64, fields map[string]any) error {
	res := b.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete 按主键批量软删除；未传 ID 时直接返回。
func (b *Base[T]) Delete(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return b.db.WithContext(ctx).Delete(new(T), ids).Error
}

// FindByID 按主键查询单条记录。
func (b *Base[T]) FindByID(ctx context.Context, id uint64) (*T, error) {
	var entity T
	if err := b.db.WithContext(ctx).First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

// Count 统计满足条件的记录数；where 为空表示统计全部。
func (b *Base[T]) Count(ctx context.Context, where string, args ...any) (int64, error) {
	q := b.db.WithContext(ctx).Model(new(T))
	if where != "" {
		q = q.Where(where, args...)
	}
	var total int64
	err := q.Count(&total).Error
	return total, err
}

// List 按条件分页查询。
// pageSize <= 0 表示不分页，返回全部记录（导出等场景）。
func (b *Base[T]) List(ctx context.Context, page, pageSize int, order, where string, args ...any) ([]T, int64, error) {
	q := b.db.WithContext(ctx).Model(new(T))
	if where != "" {
		q = q.Where(where, args...)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if order != "" {
		q = q.Order(order)
	}
	if pageSize > 0 {
		if page < 1 {
			page = 1
		}
		q = q.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	var rows []T
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// paginate 给查询追加分页；pageSize <= 0 时保持原查询不变。
func paginate(q *gorm.DB, page, pageSize int) *gorm.DB {
	if pageSize <= 0 {
		return q
	}
	if page < 1 {
		page = 1
	}
	return q.Offset((page - 1) * pageSize).Limit(pageSize)
}

// like 生成模糊匹配参数，调用方负责添加 % 通配符之外的 SQL。
func like(keyword string) string { return "%" + keyword + "%" }
