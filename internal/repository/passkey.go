package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// PasskeyRepo 提供 WebAuthn 凭据的数据访问。
type PasskeyRepo struct {
	*Base[model.Passkey]
	db *gorm.DB
}

// NewPasskeyRepo 创建通行密钥仓储。
func NewPasskeyRepo(db *gorm.DB) *PasskeyRepo {
	return &PasskeyRepo{Base: NewBase[model.Passkey](db), db: db}
}

// ListByUser 返回用户绑定的全部设备，最新绑定的在前。
func (r *PasskeyRepo) ListByUser(ctx context.Context, userID uint64) ([]model.Passkey, error) {
	var rows []model.Passkey
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByCredentialID 按凭据 ID 查询，登录时用于定位设备与用户。
func (r *PasskeyRepo) GetByCredentialID(ctx context.Context, credentialID string) (*model.Passkey, error) {
	var row model.Passkey
	if err := r.db.WithContext(ctx).Where("credential_id = ?", credentialID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetForUser 按 ID 和用户 ID 查询，确保用户只能操作自己的设备。
func (r *PasskeyRepo) GetForUser(ctx context.Context, userID, id uint64) (*model.Passkey, error) {
	var row model.Passkey
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// CountByUser 统计用户已绑定的设备数。
func (r *PasskeyRepo) CountByUser(ctx context.Context, userID uint64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.Passkey{}).
		Where("user_id = ?", userID).
		Count(&total).Error
	return total, err
}

// CountAll 统计全站已绑定的设备数。
func (r *PasskeyRepo) CountAll(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.Passkey{}).Count(&total).Error
	return total, err
}

// RenameForUser 修改设备名称；设备必须属于该用户。
func (r *PasskeyRepo) RenameForUser(ctx context.Context, userID, id uint64, name string) error {
	if _, err := r.GetForUser(ctx, userID, id); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.Passkey{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("name", name).Error
}

// DeleteForUser 物理删除设备记录，使其可以再次注册。
// 若使用软删除，唯一的 credential_id 索引会让该凭据永远无法重新绑定。
func (r *PasskeyRepo) DeleteForUser(ctx context.Context, userID, id uint64) error {
	if _, err := r.GetForUser(ctx, userID, id); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Unscoped().
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Passkey{}).Error
}

// UpdateCredential 保存认证器返回的新凭据状态与签名计数器。
// 登录成功后必须更新，否则无法检测认证器克隆。
func (r *PasskeyRepo) UpdateCredential(ctx context.Context, id uint64, credential string, usedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Passkey{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"credential":   credential,
			"last_used_at": usedAt,
		}).Error
}
