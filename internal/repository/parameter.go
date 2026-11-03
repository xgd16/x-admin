package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
)

type ParameterRepo struct {
	*Base[model.Parameter]
	db *gorm.DB
}

func NewParameterRepo(db *gorm.DB) *ParameterRepo {
	return &ParameterRepo{Base: NewBase[model.Parameter](db), db: db}
}

func (r *ParameterRepo) Get(ctx context.Context, id uint64) (*model.Parameter, error) {
	return r.FindByID(ctx, id)
}

func (r *ParameterRepo) List(ctx context.Context, q pagination.Query) ([]model.Parameter, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Parameter{})
	if q.Keyword != "" {
		kw := like(q.Keyword)
		query = query.Where("`key` LIKE ? OR name LIKE ?", kw, kw)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = paginate(query.Order("id DESC"), q.Page, q.PageSize)
	var rows []model.Parameter
	if err := query.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *ParameterRepo) GetByKey(ctx context.Context, key string) (*model.Parameter, error) {
	var p model.Parameter
	if err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ParameterRepo) ExistsKey(ctx context.Context, key string, excludeID uint64) (bool, error) {
	q := r.db.WithContext(ctx).Model(&model.Parameter{}).Where("`key` = ?", key)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ParameterRepo) Delete(ctx context.Context, ids ...uint64) error {
	return r.Base.Delete(ctx, ids...)
}
