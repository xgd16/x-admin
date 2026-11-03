package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// NotificationRepo 提供站内通知的数据访问。
type NotificationRepo struct {
	db *gorm.DB
}

// NewNotificationRepo 创建通知仓储。
func NewNotificationRepo(db *gorm.DB) *NotificationRepo { return &NotificationRepo{db: db} }

// Create 写入一条通知。
func (r *NotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

// List 返回收件人的通知，最新的在前。
// unreadOnly 为 true 时只返回 read_at 为空的消息。
func (r *NotificationRepo) List(ctx context.Context, userID uint64, unreadOnly bool, page, pageSize int) ([]model.Notification, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", userID)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = paginate(q.Order("id DESC"), page, pageSize)
	var rows []model.Notification
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// CountUnread 统计收件人的未读消息数。
func (r *NotificationRepo) CountUnread(ctx context.Context, userID uint64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Count(&total).Error
	return total, err
}

// MarkRead 把指定消息标记为已读，操作是幂等的。
// 只有消息不属于该收件人时才返回 ErrNotFound；已读消息重复标记仍然成功。
func (r *NotificationRepo) MarkRead(ctx context.Context, userID, id uint64, at time.Time) error {
	res := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL", id, userID).
		Update("read_at", at)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	// 没有更新行：可能已经读过，也可能消息不存在或不属于当前用户，需要再确认一次。
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkAllRead 把收件人的全部未读消息标记为已读，返回更新行数。
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID uint64, at time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", at)
	return res.RowsAffected, res.Error
}

// CleanBefore 物理删除超过保留期的通知。
func (r *NotificationRepo) CleanBefore(ctx context.Context, days int) error {
	if days <= 0 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return r.db.WithContext(ctx).Unscoped().
		Where("created_at < ?", cutoff).
		Delete(&model.Notification{}).Error
}
