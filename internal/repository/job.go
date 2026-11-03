package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

type JobRepo struct {
	*Base[model.Job]
	db *gorm.DB
}

func NewJobRepo(db *gorm.DB) *JobRepo {
	return &JobRepo{Base: NewBase[model.Job](db), db: db}
}

func (r *JobRepo) Get(ctx context.Context, id uint64) (*model.Job, error) {
	return r.FindByID(ctx, id)
}

func (r *JobRepo) List(ctx context.Context, page, pageSize int) ([]model.Job, int64, error) {
	return r.Base.List(ctx, page, pageSize, "id DESC", "")
}

func (r *JobRepo) ListEnabled(ctx context.Context) ([]model.Job, error) {
	var rows []model.Job
	err := r.db.WithContext(ctx).Where("status = ?", model.StatusEnabled).Find(&rows).Error
	return rows, err
}

func (r *JobRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}
