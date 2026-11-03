package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

type LogRepo struct {
	db *gorm.DB
}

func NewLogRepo(db *gorm.DB) *LogRepo { return &LogRepo{db: db} }

func (r *LogRepo) CreateOperation(ctx context.Context, log *model.OperationLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *LogRepo) ListOperations(ctx context.Context, f model.OperationLogFilter) ([]model.OperationLog, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.OperationLog{})
	if f.Keyword != "" {
		kw := like(f.Keyword)
		q = q.Where("username LIKE ? OR path LIKE ? OR body LIKE ?", kw, kw, kw)
	}
	if f.Username != "" {
		q = q.Where("username = ?", f.Username)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.Start != nil {
		q = q.Where("created_at >= ?", *f.Start)
	}
	if f.End != nil {
		q = q.Where("created_at <= ?", *f.End)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = paginate(q.Order("id DESC"), f.Page, f.PageSize)
	var rows []model.OperationLog
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *LogRepo) DeleteOperations(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&model.OperationLog{}).Error
}

func (r *LogRepo) ClearOperations(ctx context.Context) error {
	return r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&model.OperationLog{}).Error
}

func (r *LogRepo) RecentOperations(ctx context.Context, limit int) ([]model.OperationLog, error) {
	var rows []model.OperationLog
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (r *LogRepo) CreateLogin(ctx context.Context, log *model.LoginLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *LogRepo) ListLogins(ctx context.Context, f model.LoginLogFilter) ([]model.LoginLog, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.LoginLog{})
	if f.Keyword != "" {
		kw := like(f.Keyword)
		q = q.Where("username LIKE ? OR ip LIKE ?", kw, kw)
	}
	if f.Username != "" {
		q = q.Where("username = ?", f.Username)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.Start != nil {
		q = q.Where("created_at >= ?", *f.Start)
	}
	if f.End != nil {
		q = q.Where("created_at <= ?", *f.End)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = paginate(q.Order("id DESC"), f.Page, f.PageSize)
	var rows []model.LoginLog
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *LogRepo) ClearLogins(ctx context.Context) error {
	return r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&model.LoginLog{}).Error
}

func (r *LogRepo) LoginTrend(ctx context.Context, days int) ([]model.TrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	start := time.Now().AddDate(0, 0, -days+1).Truncate(24 * time.Hour)
	var rows []model.TrendPoint
	err := r.db.WithContext(ctx).Model(&model.LoginLog{}).
		Select("DATE(created_at) AS date, COUNT(*) AS count").
		Where("created_at >= ?", start).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&rows).Error
	return rows, err
}

// CleanBefore hard-deletes audit rows older than the configured retention.
func (r *LogRepo) CleanBefore(ctx context.Context, days int) error {
	if days <= 0 {
		days = 90
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	if err := r.db.WithContext(ctx).Unscoped().Where("created_at < ?", cutoff).Delete(&model.OperationLog{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Unscoped().Where("created_at < ?", cutoff).Delete(&model.LoginLog{}).Error
}
