package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

type FileRepo struct {
	*Base[model.File]
	db *gorm.DB
}

func NewFileRepo(db *gorm.DB) *FileRepo {
	return &FileRepo{Base: NewBase[model.File](db), db: db}
}

func (r *FileRepo) Get(ctx context.Context, id uint64) (*model.File, error) {
	return r.FindByID(ctx, id)
}

func (r *FileRepo) List(ctx context.Context, f model.FileFilter) ([]model.File, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.File{})
	if f.Keyword != "" {
		kw := like(f.Keyword)
		q = q.Where("name LIKE ? OR mime LIKE ?", kw, kw)
	}
	if f.MIME != "" {
		q = q.Where("mime LIKE ?", f.MIME+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = paginate(q.Order("id DESC"), f.Page, f.PageSize)
	var rows []model.File
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *FileRepo) TotalSize(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&model.File{}).Select("COALESCE(SUM(size),0)").Scan(&total).Error
	return total, err
}

func (r *FileRepo) CountAll(ctx context.Context) (int64, error) {
	return r.Count(ctx, "")
}
