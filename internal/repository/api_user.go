package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// APIUserRepo 提供自定义 API 面终端用户的数据访问。
// 每个 API 面使用独立表，调用方必须传入 scope.Table，避免跨面读写。
type APIUserRepo struct {
	db *gorm.DB
}

// NewAPIUserRepo 创建 API 用户仓储。
func NewAPIUserRepo(db *gorm.DB) *APIUserRepo {
	return &APIUserRepo{db: db}
}

// Create 向指定 API 面用户表插入账号。
func (r *APIUserRepo) Create(ctx context.Context, table string, user *model.APIUser) error {
	return r.db.WithContext(ctx).Table(table).Create(user).Error
}

// FindByUsername 按用户名查询指定 API 面的账号。
func (r *APIUserRepo) FindByUsername(ctx context.Context, table, username string) (*model.APIUser, error) {
	var user model.APIUser
	err := r.db.WithContext(ctx).Table(table).
		Where("username = ? AND deleted_at IS NULL", username).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Get 按主键查询指定 API 面的账号。
func (r *APIUserRepo) Get(ctx context.Context, table string, id uint64) (*model.APIUser, error) {
	var user model.APIUser
	err := r.db.WithContext(ctx).Table(table).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateFields 更新指定账号的字段。
func (r *APIUserRepo) UpdateFields(ctx context.Context, table string, id uint64, fields map[string]any) error {
	res := r.db.WithContext(ctx).Table(table).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
