package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// RoleRepo 提供角色及其菜单授权、用户关联的数据访问。
type RoleRepo struct {
	*Base[model.Role]
	db *gorm.DB
}

// NewRoleRepo 创建角色仓储。
func NewRoleRepo(db *gorm.DB) *RoleRepo {
	return &RoleRepo{Base: NewBase[model.Role](db), db: db}
}

// Get 按 ID 查询角色，并预加载已授权菜单。
func (r *RoleRepo) Get(ctx context.Context, id uint64) (*model.Role, error) {
	var role model.Role
	if err := r.db.WithContext(ctx).Preload("Menus").First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// FindByCode 按角色编码查询，用于唯一性校验和超级管理员判断。
func (r *RoleRepo) FindByCode(ctx context.Context, code string) (*model.Role, error) {
	var role model.Role
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// List 分页查询角色，支持名称/编码关键字与状态过滤。
func (r *RoleRepo) List(ctx context.Context, f model.RoleFilter) ([]model.Role, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Role{})
	if f.Keyword != "" {
		kw := like(f.Keyword)
		q = q.Where("name LIKE ? OR code LIKE ?", kw, kw)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = q.Order("sort ASC, id ASC")
	q = paginate(q, f.Page, f.PageSize)
	var roles []model.Role
	if err := q.Find(&roles).Error; err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// All 返回全部启用角色，按排序值升序，用于下拉选择。
func (r *RoleRepo) All(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.WithContext(ctx).Where("status = ?", model.StatusEnabled).
		Order("sort ASC, id ASC").Find(&roles).Error
	return roles, err
}

// ExistsCode 判断角色编码是否已存在；excludeID > 0 时排除自身。
func (r *RoleRepo) ExistsCode(ctx context.Context, code string, excludeID uint64) (bool, error) {
	q := r.db.WithContext(ctx).Model(&model.Role{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountAll 统计全部角色数量。
func (r *RoleRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}

// SetMenus 在事务中整体替换角色的菜单授权。
func (r *RoleRepo) SetMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		rows := make([]model.RoleMenu, 0, len(menuIDs))
		for _, menuID := range menuIDs {
			rows = append(rows, model.RoleMenu{RoleID: roleID, MenuID: menuID})
		}
		return tx.Create(&rows).Error
	})
}

// CountUsers 统计绑定到该角色的用户数，删除角色前用于冲突检查。
func (r *RoleRepo) CountUsers(ctx context.Context, roleID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.UserRole{}).Where("role_id = ?", roleID).Count(&count).Error
	return count, err
}

// Delete 在事务中删除角色并清理菜单、用户关联。
func (r *RoleRepo) Delete(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id IN ?", ids).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id IN ?", ids).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Role{}, ids).Error
	})
}
