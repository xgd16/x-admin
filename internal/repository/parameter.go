package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
)

// ParameterRepo 提供系统参数的数据访问。
type ParameterRepo struct {
	*Base[model.Parameter]
	db *gorm.DB
}

// NewParameterRepo 创建参数仓储。
func NewParameterRepo(db *gorm.DB) *ParameterRepo {
	return &ParameterRepo{Base: NewBase[model.Parameter](db), db: db}
}

// Get 按 ID 查询参数。
func (r *ParameterRepo) Get(ctx context.Context, id uint64) (*model.Parameter, error) {
	return r.FindByID(ctx, id)
}

// List 分页查询参数，支持按 key/名称模糊搜索。
// key 是 MySQL 保留字，因此使用反引号转义。
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

// ListPublic 返回全部公开参数，供未登录页面读取站点名称、Logo 等配置。
func (r *ParameterRepo) ListPublic(ctx context.Context) ([]model.Parameter, error) {
	var rows []model.Parameter
	err := r.db.WithContext(ctx).
		Where("public = ?", true).
		Order("`key` ASC").
		Find(&rows).Error
	return rows, err
}

// GetByKey 按参数键查询。
func (r *ParameterRepo) GetByKey(ctx context.Context, key string) (*model.Parameter, error) {
	var p model.Parameter
	if err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ExistsKey 判断参数键是否已存在；excludeID > 0 时排除自身。
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

// Delete 按 ID 批量删除参数。
// key 在软删除前改成墓碑值，避免之后重建同 key 参数撞唯一索引。
func (r *ParameterRepo) Delete(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []model.Parameter
		if err := tx.Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return err
		}
		for i := range rows {
			if err := tx.Model(&model.Parameter{}).Where("id = ?", rows[i].ID).
				UpdateColumn("`key`", tombstoneValue(rows[i].Key, rows[i].ID, 128)).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.Parameter{}, ids).Error
	})
}
