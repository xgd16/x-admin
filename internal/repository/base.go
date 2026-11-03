package repository

import (
	"context"

	"gorm.io/gorm"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = gorm.ErrRecordNotFound

// Base provides the CRUD primitives shared by all repositories.
type Base[T any] struct {
	db *gorm.DB
}

func NewBase[T any](db *gorm.DB) *Base[T] { return &Base[T]{db: db} }

func (b *Base[T]) Create(ctx context.Context, entity *T) error {
	return b.db.WithContext(ctx).Create(entity).Error
}

func (b *Base[T]) Update(ctx context.Context, entity *T) error {
	return b.db.WithContext(ctx).Save(entity).Error
}

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

func (b *Base[T]) Delete(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return b.db.WithContext(ctx).Delete(new(T), ids).Error
}

func (b *Base[T]) FindByID(ctx context.Context, id uint64) (*T, error) {
	var entity T
	if err := b.db.WithContext(ctx).First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (b *Base[T]) Count(ctx context.Context, where string, args ...any) (int64, error) {
	q := b.db.WithContext(ctx).Model(new(T))
	if where != "" {
		q = q.Where(where, args...)
	}
	var total int64
	err := q.Count(&total).Error
	return total, err
}

// List applies optional pagination. pageSize <= 0 means "all rows" (exports).
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

func paginate(q *gorm.DB, page, pageSize int) *gorm.DB {
	if pageSize <= 0 {
		return q
	}
	if page < 1 {
		page = 1
	}
	return q.Offset((page - 1) * pageSize).Limit(pageSize)
}

func like(keyword string) string { return "%" + keyword + "%" }
