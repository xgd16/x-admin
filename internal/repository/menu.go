package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// MenuRepo 提供菜单树与角色授权关系的数据访问。
type MenuRepo struct {
	*Base[model.Menu]
	db *gorm.DB
}

// NewMenuRepo 创建菜单仓储。
func NewMenuRepo(db *gorm.DB) *MenuRepo {
	return &MenuRepo{Base: NewBase[model.Menu](db), db: db}
}

// Get 按 ID 查询菜单。
func (r *MenuRepo) Get(ctx context.Context, id uint64) (*model.Menu, error) {
	return r.FindByID(ctx, id)
}

// All 返回全部菜单，按排序值和 ID 升序，调用方负责组装树。
func (r *MenuRepo) All(ctx context.Context) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&menus).Error
	return menus, err
}

// CountAll 统计全部菜单数量。
func (r *MenuRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}

// ForUser 返回用户通过“启用角色”获得的“启用菜单”。
// 使用 DISTINCT/Group 避免同一菜单被多个角色重复授权时出现重复行。
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

// MenuIDsByRole 返回角色已授权的菜单 ID 列表。
func (r *MenuRepo) MenuIDsByRole(ctx context.Context, roleID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.RoleMenu{}).
		Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	return ids, err
}

// CountChildren 统计直接子菜单数量，删除前用于阻止误删父节点。
func (r *MenuRepo) CountChildren(ctx context.Context, parentID uint64) (int64, error) {
	return r.Count(ctx, "parent_id = ?", parentID)
}

// Delete 在事务中删除菜单并清理角色授权关联。
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
