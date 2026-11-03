package repository

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
)

// DepartmentRepo 提供部门树与用户归属的数据访问。
type DepartmentRepo struct {
	*Base[model.Department]
	db *gorm.DB
}

// NewDepartmentRepo 创建部门仓储。
func NewDepartmentRepo(db *gorm.DB) *DepartmentRepo {
	return &DepartmentRepo{Base: NewBase[model.Department](db), db: db}
}

// Get 按 ID 查询部门。
func (r *DepartmentRepo) Get(ctx context.Context, id uint64) (*model.Department, error) {
	return r.FindByID(ctx, id)
}

// All 返回全部部门，按排序值和 ID 升序，调用方负责组装树。
func (r *DepartmentRepo) All(ctx context.Context) ([]model.Department, error) {
	var rows []model.Department
	err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&rows).Error
	return rows, err
}

// CountChildren 统计直接子部门数量。
func (r *DepartmentRepo) CountChildren(ctx context.Context, parentID uint64) (int64, error) {
	return r.Count(ctx, "parent_id = ?", parentID)
}

// CountUsers 统计部门下的用户数量，删除前用于冲突检查。
func (r *DepartmentRepo) CountUsers(ctx context.Context, deptID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("dept_id = ?", deptID).Count(&count).Error
	return count, err
}
