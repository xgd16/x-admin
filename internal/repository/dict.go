package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
)

// DictRepo 同时管理字典类型与字典项两个实体。
type DictRepo struct {
	types *Base[model.DictType]
	items *Base[model.DictItem]
	db    *gorm.DB
}

// NewDictRepo 创建字典仓储。
func NewDictRepo(db *gorm.DB) *DictRepo {
	return &DictRepo{
		types: NewBase[model.DictType](db),
		items: NewBase[model.DictItem](db),
		db:    db,
	}
}

// CreateType 新增字典类型。
func (r *DictRepo) CreateType(ctx context.Context, t *model.DictType) error {
	return r.types.Create(ctx, t)
}

// UpdateType 保存字典类型。
func (r *DictRepo) UpdateType(ctx context.Context, t *model.DictType) error {
	return r.types.Update(ctx, t)
}

// GetType 按 ID 查询字典类型。
func (r *DictRepo) GetType(ctx context.Context, id uint64) (*model.DictType, error) {
	return r.types.FindByID(ctx, id)
}

// FindTypeByCode 按编码查询字典类型，用于唯一性校验。
func (r *DictRepo) FindTypeByCode(ctx context.Context, code string) (*model.DictType, error) {
	var t model.DictType
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTypes 分页查询字典类型，支持名称/编码关键字和状态过滤。
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

// DeleteType 在事务中删除字典类型及其全部字典项。
// 编码在软删除前改成墓碑值，避免之后重建同编码字典撞唯一索引。
func (r *DictRepo) DeleteType(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t model.DictType
		if err := tx.First(&t, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.DictType{}).Where("id = ?", id).
			UpdateColumn("code", tombstoneValue(t.Code, t.ID, 64)).Error; err != nil {
			return err
		}
		if err := tx.Where("dict_type_id = ?", id).Delete(&model.DictItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.DictType{}, id).Error
	})
}

// Items 返回某个字典类型下的全部字典项，按排序值升序。
func (r *DictRepo) Items(ctx context.Context, typeID uint64) ([]model.DictItem, error) {
	var rows []model.DictItem
	err := r.db.WithContext(ctx).Where("dict_type_id = ?", typeID).
		Order("sort ASC, id ASC").Find(&rows).Error
	return rows, err
}

// GetItem 按 ID 查询字典项。
func (r *DictRepo) GetItem(ctx context.Context, id uint64) (*model.DictItem, error) {
	return r.items.FindByID(ctx, id)
}

// CreateItem 新增字典项。
func (r *DictRepo) CreateItem(ctx context.Context, item *model.DictItem) error {
	return r.items.Create(ctx, item)
}

// UpdateItem 保存字典项。
func (r *DictRepo) UpdateItem(ctx context.Context, item *model.DictItem) error {
	return r.items.Update(ctx, item)
}

// DeleteItem 按 ID 删除字典项。
func (r *DictRepo) DeleteItem(ctx context.Context, id uint64) error {
	return r.items.Delete(ctx, id)
}
