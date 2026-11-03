package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
)

type DictRepo struct {
	types *Base[model.DictType]
	items *Base[model.DictItem]
	db    *gorm.DB
}

func NewDictRepo(db *gorm.DB) *DictRepo {
	return &DictRepo{
		types: NewBase[model.DictType](db),
		items: NewBase[model.DictItem](db),
		db:    db,
	}
}

func (r *DictRepo) CreateType(ctx context.Context, t *model.DictType) error {
	return r.types.Create(ctx, t)
}

func (r *DictRepo) UpdateType(ctx context.Context, t *model.DictType) error {
	return r.types.Update(ctx, t)
}

func (r *DictRepo) GetType(ctx context.Context, id uint64) (*model.DictType, error) {
	return r.types.FindByID(ctx, id)
}

func (r *DictRepo) FindTypeByCode(ctx context.Context, code string) (*model.DictType, error) {
	var t model.DictType
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *DictRepo) ListTypes(ctx context.Context, q pagination.Query, status *int8) ([]model.DictType, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.DictType{})
	if q.Keyword != "" {
		kw := like(q.Keyword)
		query = query.Where("name LIKE ? OR code LIKE ?", kw, kw)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = paginate(query.Order("id DESC"), q.Page, q.PageSize)
	var rows []model.DictType
	if err := query.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *DictRepo) DeleteType(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dict_type_id = ?", id).Delete(&model.DictItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.DictType{}, id).Error
	})
}

func (r *DictRepo) Items(ctx context.Context, typeID uint64) ([]model.DictItem, error) {
	var rows []model.DictItem
	err := r.db.WithContext(ctx).Where("dict_type_id = ?", typeID).
		Order("sort ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *DictRepo) GetItem(ctx context.Context, id uint64) (*model.DictItem, error) {
	return r.items.FindByID(ctx, id)
}

func (r *DictRepo) CreateItem(ctx context.Context, item *model.DictItem) error {
	return r.items.Create(ctx, item)
}

func (r *DictRepo) UpdateItem(ctx context.Context, item *model.DictItem) error {
	return r.items.Update(ctx, item)
}

func (r *DictRepo) DeleteItem(ctx context.Context, id uint64) error {
	return r.items.Delete(ctx, id)
}
