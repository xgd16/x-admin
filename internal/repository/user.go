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

// SetPassword 更新密码哈希；bumpSessionVersion 为 true 时原子递增会话版本，
// 使该账号此前签发的 access / refresh token 立即失效。
func (r *UserRepo) SetPassword(ctx context.Context, id uint64, hash string, bumpSessionVersion bool) error {
	fields := map[string]any{"password": hash}
	if bumpSessionVersion {
		fields["session_version"] = gorm.Expr("session_version + 1")
	}
	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// BumpSessionVersion 递增会话版本，撤销该用户当前所有 access / refresh token。
func (r *UserRepo) BumpSessionVersion(ctx context.Context, id uint64) error {
	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		UpdateColumn("session_version", gorm.Expr("session_version + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SessionState 只读取状态与会话版本，供文件访问等高频率鉴权使用。
func (r *UserRepo) SessionState(ctx context.Context, id uint64) (int8, int64, error) {
	var row struct {
		Status         int8
		SessionVersion int64
	}
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Select("status", "session_version").First(&row, id).Error; err != nil {
		return 0, 0, err
	}
	return row.Status, row.SessionVersion, nil
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
		seen := make(map[uint64]struct{}, len(roleIDs))
		rows := make([]model.UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			if _, ok := seen[roleID]; ok {
				continue
			}
			seen[roleID] = struct{}{}
			rows = append(rows, model.UserRole{UserID: userID, RoleID: roleID})
		}
		return tx.Create(&rows).Error
	})
}

// Delete 在事务中删除用户，并清理角色关联和通行密钥。
// 通行密钥是物理删除，避免唯一的 credential_id 索引被软删除记录永久占用；
// 用户名在软删除前改成墓碑值，否则重建同名账号会撞 uk_users_username。
func (r *UserRepo) Delete(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var users []model.User
		if err := tx.Where("id IN ?", ids).Find(&users).Error; err != nil {
			return err
		}
		for i := range users {
			if err := tx.Model(&model.User{}).Where("id = ?", users[i].ID).
				UpdateColumn("username", tombstoneValue(users[i].Username, users[i].ID, 64)).Error; err != nil {
				return err
			}
		}
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
