package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

type DepartmentRepo struct {
	*Base[model.Department]
	db *gorm.DB
}

func NewDepartmentRepo(db *gorm.DB) *DepartmentRepo {
	return &DepartmentRepo{Base: NewBase[model.Department](db), db: db}
}

func (r *DepartmentRepo) Get(ctx context.Context, id uint64) (*model.Department, error) {
	return r.FindByID(ctx, id)
}

func (r *DepartmentRepo) All(ctx context.Context) ([]model.Department, error) {
	var rows []model.Department
	err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (r *DepartmentRepo) CountChildren(ctx context.Context, parentID uint64) (int64, error) {
	return r.Count(ctx, "parent_id = ?", parentID)
}

func (r *DepartmentRepo) CountUsers(ctx context.Context, deptID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("dept_id = ?", deptID).Count(&count).Error
	return count, err
}
