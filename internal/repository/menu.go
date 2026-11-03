package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

type MenuRepo struct {
	*Base[model.Menu]
	db *gorm.DB
}

func NewMenuRepo(db *gorm.DB) *MenuRepo {
	return &MenuRepo{Base: NewBase[model.Menu](db), db: db}
}

func (r *MenuRepo) Get(ctx context.Context, id uint64) (*model.Menu, error) {
	return r.FindByID(ctx, id)
}

func (r *MenuRepo) All(ctx context.Context) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&menus).Error
	return menus, err
}

func (r *MenuRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}

// ForUser returns enabled menus granted through the user's roles.
func (r *MenuRepo) ForUser(ctx context.Context, userID uint64) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.WithContext(ctx).
		Table("sys_menus AS m").
		Select("m.*").
		Joins("JOIN sys_role_menus rm ON rm.menu_id = m.id").
		Joins("JOIN sys_roles r ON r.id = rm.role_id AND r.status = ? AND r.deleted_at IS NULL", model.StatusEnabled).
		Joins("JOIN sys_user_roles ur ON ur.role_id = r.id").
		Where("ur.user_id = ? AND m.status = ? AND m.deleted_at IS NULL", userID, model.StatusEnabled).
		Group("m.id").
		Order("m.sort ASC, m.id ASC").
		Scan(&menus).Error
	return menus, err
}

func (r *MenuRepo) MenuIDsByRole(ctx context.Context, roleID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.RoleMenu{}).
		Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	return ids, err
}

func (r *MenuRepo) CountChildren(ctx context.Context, parentID uint64) (int64, error) {
	return r.Count(ctx, "parent_id = ?", parentID)
}

func (r *MenuRepo) Delete(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_id IN ?", ids).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Menu{}, ids).Error
	})
}
