package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// JobRepo 提供定时任务定义的数据访问。
type JobRepo struct {
	*Base[model.Job]
	db *gorm.DB
}

// NewJobRepo 创建定时任务仓储。
func NewJobRepo(db *gorm.DB) *JobRepo {
	return &JobRepo{Base: NewBase[model.Job](db), db: db}
}

// Get 按 ID 查询任务。
func (r *JobRepo) Get(ctx context.Context, id uint64) (*model.Job, error) {
	return r.FindByID(ctx, id)
}

// List 分页查询任务，按 ID 倒序返回。
func (r *JobRepo) List(ctx context.Context, page, pageSize int) ([]model.Job, int64, error) {
	return r.Base.List(ctx, page, pageSize, "id DESC", "")
}

// ListEnabled 返回全部启用状态的任务，启动时用于恢复调度。
func (r *JobRepo) ListEnabled(ctx context.Context) ([]model.Job, error) {
	var rows []model.Job
	err := r.db.WithContext(ctx).Where("status = ?", model.StatusEnabled).Find(&rows).Error
	return rows, err
}

// CountAll 统计任务数量。
func (r *JobRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}

// CreateRun 写入一条任务执行记录。
func (r *JobRepo) CreateRun(ctx context.Context, run *model.JobRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// ListRuns 分页查询任务执行记录；jobID 为 0 时返回全部任务。
func (r *JobRepo) ListRuns(ctx context.Context, jobID uint64, page, pageSize int) ([]model.JobRun, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.JobRun{})
	if jobID > 0 {
		q = q.Where("job_id = ?", jobID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = q.Order("started_at DESC, id DESC")
	q = paginate(q, page, pageSize)
	var rows []model.JobRun
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// PruneRuns 物理删除早于 before 的执行记录，避免历史无限增长。
func (r *JobRepo) PruneRuns(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("started_at < ?", before).
		Delete(&model.JobRun{}).Error
}

// DeleteRunsByJob 在删除任务时清理其执行记录。
func (r *JobRepo) DeleteRunsByJob(ctx context.Context, jobID uint64) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("job_id = ?", jobID).
		Delete(&model.JobRun{}).Error
}
