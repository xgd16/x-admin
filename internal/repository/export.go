package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// ExportTaskRepo 提供异步导出任务记录的数据访问。
type ExportTaskRepo struct {
	*Base[model.ExportTask]
	db *gorm.DB
}

// NewExportTaskRepo 创建导出任务仓储。
func NewExportTaskRepo(db *gorm.DB) *ExportTaskRepo {
	return &ExportTaskRepo{Base: NewBase[model.ExportTask](db), db: db}
}

// Get 按 ID 查询导出任务。
func (r *ExportTaskRepo) Get(ctx context.Context, id uint64) (*model.ExportTask, error) {
	return r.FindByID(ctx, id)
}

// List 分页查询某个用户的导出任务，按创建时间倒序；status 为 nil 时不过滤。
func (r *ExportTaskRepo) List(ctx context.Context, userID uint64, status *int8, page, pageSize int) ([]model.ExportTask, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.ExportTask{}).Where("user_id = ?", userID)
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = paginate(q.Order("id DESC"), page, pageSize)
	var tasks []model.ExportTask
	if err := q.Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// CountUnfinished 统计用户排队中与生成中的任务数，用于提交限流。
func (r *ExportTaskRepo) CountUnfinished(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ExportTask{}).
		Where("user_id = ? AND status IN ?", userID, []int8{model.ExportPending, model.ExportRunning}).
		Count(&count).Error
	return count, err
}

// ListUnfinished 返回排队中与生成中的任务，用于服务重启后的遗留任务收尾。
func (r *ExportTaskRepo) ListUnfinished(ctx context.Context, limit int) ([]model.ExportTask, error) {
	var tasks []model.ExportTask
	err := r.db.WithContext(ctx).Unscoped().
		Where("status IN ?", []int8{model.ExportPending, model.ExportRunning}).
		Order("id ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// HardDelete 物理删除任务记录，配合清理任务使用。
func (r *ExportTaskRepo) HardDelete(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Unscoped().Delete(&model.ExportTask{}, ids).Error
}

// ListExpired 返回在 before 之前结束的任务（只取 ID 与文件键），最多 limit 条。
func (r *ExportTaskRepo) ListExpired(ctx context.Context, before time.Time, limit int) ([]model.ExportTask, error) {
	var tasks []model.ExportTask
	err := r.db.WithContext(ctx).Unscoped().
		Select("id", "file_key").
		Where("finished_at IS NOT NULL AND finished_at < ?", before).
		Order("id ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}
