package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// LogRepo 提供操作日志与登录日志的数据访问。
// 日志表不使用 Base，因为需要大量按时间、用户过滤的查询。
type LogRepo struct {
	db *gorm.DB
}

// NewLogRepo 创建日志仓储。
func NewLogRepo(db *gorm.DB) *LogRepo { return &LogRepo{db: db} }

// CreateOperation 写入一条操作审计日志。
func (r *LogRepo) CreateOperation(ctx context.Context, log *model.OperationLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListOperations 分页查询操作日志，支持关键字、用户名、状态和创建时间范围过滤。
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

// DeleteOperations 按 ID 批量删除操作日志（软删除）。
func (r *LogRepo) DeleteOperations(ctx context.Context, ids ...uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&model.OperationLog{}).Error
}

// ClearOperations 物理清空操作日志。
func (r *LogRepo) ClearOperations(ctx context.Context) error {
	return r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&model.OperationLog{}).Error
}

// RecentOperations 返回最近若干条操作日志，用于看板展示。
func (r *LogRepo) RecentOperations(ctx context.Context, limit int) ([]model.OperationLog, error) {
	var rows []model.OperationLog
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// CreateLogin 写入一条登录审计日志。
func (r *LogRepo) CreateLogin(ctx context.Context, log *model.LoginLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListLogins 分页查询登录日志，支持关键字、用户名、状态和时间范围过滤。
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

// ClearLogins 物理清空登录日志。
func (r *LogRepo) ClearLogins(ctx context.Context) error {
	return r.db.WithContext(ctx).Unscoped().Where("1 = 1").Delete(&model.LoginLog{}).Error
}

// LoginTrend 按天统计最近 days 天的成功登录次数，缺少的日期由 service 层补齐。
// 失败登录也写 sys_login_logs，趋势图只应统计 status=200 的记录。
func (r *LogRepo) LoginTrend(ctx context.Context, days int) ([]model.TrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	now := time.Now()
	// 从本地时区的当天零点开始，而不是 Truncate(24h)（后者按 UTC 边界截断）。
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -days+1)
	var rows []model.TrendPoint
	err := r.db.WithContext(ctx).Model(&model.LoginLog{}).
		Select("DATE(created_at) AS date, COUNT(*) AS count").
		Where("created_at >= ? AND status = ?", start, 200).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&rows).Error
	return rows, err
}

// CleanBefore 物理删除超过保留期的审计日志（操作日志与登录日志）。
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
