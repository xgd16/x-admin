package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// UserRepo 提供用户及其部门、角色的数据访问。
type UserRepo struct {
	*Base[model.User]
	db *gorm.DB
}

// NewUserRepo 创建用户仓储。
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{Base: NewBase[model.User](db), db: db}
}

// Get 按 ID 查询用户，并预加载部门和角色。
func (r *UserRepo) Get(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Preload("Dept").Preload("Roles").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsername 按用户名查询用户，登录流程使用。
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Preload("Dept").Preload("Roles").
		Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ExistsUsername 判断用户名是否已被占用；excludeID > 0 时排除自身（更新场景）。
func (r *UserRepo) ExistsUsername(ctx context.Context, username string, excludeID uint64) (bool, error) {
	q := r.db.WithContext(ctx).Model(&model.User{}).Where("username = ?", username)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// List 按用户名/昵称/邮箱/手机号模糊搜索，并支持部门、状态、角色过滤。
// 关联角色时用 JOIN 会导致同一用户出现多行，因此计数使用 DISTINCT 用户 ID。
func (r *UserRepo) List(ctx context.Context, f model.UserFilter) ([]model.User, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.User{})
	if f.Keyword != "" {
		kw := like(f.Keyword)
		q = q.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ? OR phone LIKE ?", kw, kw, kw, kw)
	}
	if f.DeptID != nil {
		q = q.Where("dept_id = ?", *f.DeptID)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.RoleID != nil {
		q = q.Joins("JOIN sys_user_roles ur ON ur.user_id = sys_users.id").
			Where("ur.role_id = ?", *f.RoleID)
	}
	var total int64
	countQuery := q.Session(&gorm.Session{})
	if err := countQuery.Distinct("sys_users.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = q.Select("sys_users.*").Preload("Dept").Preload("Roles").Order("sys_users.id DESC")
	q = paginate(q, f.Page, f.PageSize)
	var users []model.User
	if err := q.Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// SetRoles 在事务中整体替换用户的角色关联。
func (r *UserRepo) SetRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]model.UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			rows = append(rows, model.UserRole{UserID: userID, RoleID: roleID})
		}
		return tx.Create(&rows).Error
	})
}

// Delete 在事务中删除用户，并清理角色关联和通行密钥。
// 通行密钥是物理删除，避免唯一的 credential_id 索引被软删除记录永久占用。
func (r *UserRepo) Delete(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id IN ?", ids).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id IN ?", ids).Unscoped().Delete(&model.Passkey{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, ids).Error
	})
}

// CountByStatus 统计指定状态的用户数；status 为 nil 时统计全部。
func (r *UserRepo) CountByStatus(ctx context.Context, status *int8) (int64, error) {
	if status == nil {
		return r.Count(ctx, "")
	}
	return r.Count(ctx, "status = ?", *status)
}
