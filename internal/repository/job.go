package repository

import (
	"context"

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
