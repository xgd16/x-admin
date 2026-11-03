package service

import (
	"context"

	"gorm.io/gorm"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
)

// ErrRecordNotFound is shared by repositories and services.
var ErrRecordNotFound = gorm.ErrRecordNotFound

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	Get(ctx context.Context, id uint64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, filter model.UserFilter) ([]model.User, int64, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	Delete(ctx context.Context, ids ...uint64) error
	SetRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
	ExistsUsername(ctx context.Context, username string, excludeID uint64) (bool, error)
	CountByStatus(ctx context.Context, status *int8) (int64, error)
}

type RoleRepository interface {
	Create(ctx context.Context, role *model.Role) error
	Update(ctx context.Context, role *model.Role) error
	Get(ctx context.Context, id uint64) (*model.Role, error)
	FindByCode(ctx context.Context, code string) (*model.Role, error)
	List(ctx context.Context, filter model.RoleFilter) ([]model.Role, int64, error)
	All(ctx context.Context) ([]model.Role, error)
	ExistsCode(ctx context.Context, code string, excludeID uint64) (bool, error)
	SetMenus(ctx context.Context, roleID uint64, menuIDs []uint64) error
	CountUsers(ctx context.Context, roleID uint64) (int64, error)
	CountAll(ctx context.Context) (int64, error)
	Delete(ctx context.Context, ids ...uint64) error
}

type MenuRepository interface {
	All(ctx context.Context) ([]model.Menu, error)
	ForUser(ctx context.Context, userID uint64) ([]model.Menu, error)
	Get(ctx context.Context, id uint64) (*model.Menu, error)
	Create(ctx context.Context, menu *model.Menu) error
	Update(ctx context.Context, menu *model.Menu) error
	Delete(ctx context.Context, ids ...uint64) error
	MenuIDsByRole(ctx context.Context, roleID uint64) ([]uint64, error)
	CountChildren(ctx context.Context, parentID uint64) (int64, error)
	CountAll(ctx context.Context) (int64, error)
}

type DepartmentRepository interface {
	All(ctx context.Context) ([]model.Department, error)
	Get(ctx context.Context, id uint64) (*model.Department, error)
	Create(ctx context.Context, dept *model.Department) error
	Update(ctx context.Context, dept *model.Department) error
	Delete(ctx context.Context, ids ...uint64) error
	CountChildren(ctx context.Context, parentID uint64) (int64, error)
	CountUsers(ctx context.Context, deptID uint64) (int64, error)
}

type DictRepository interface {
	CreateType(ctx context.Context, t *model.DictType) error
	UpdateType(ctx context.Context, t *model.DictType) error
	GetType(ctx context.Context, id uint64) (*model.DictType, error)
	FindTypeByCode(ctx context.Context, code string) (*model.DictType, error)
	ListTypes(ctx context.Context, q pagination.Query, status *int8) ([]model.DictType, int64, error)
	DeleteType(ctx context.Context, id uint64) error
	Items(ctx context.Context, typeID uint64) ([]model.DictItem, error)
	GetItem(ctx context.Context, id uint64) (*model.DictItem, error)
	CreateItem(ctx context.Context, item *model.DictItem) error
	UpdateItem(ctx context.Context, item *model.DictItem) error
	DeleteItem(ctx context.Context, id uint64) error
}

type ParameterRepository interface {
	Create(ctx context.Context, p *model.Parameter) error
	Update(ctx context.Context, p *model.Parameter) error
	Get(ctx context.Context, id uint64) (*model.Parameter, error)
	GetByKey(ctx context.Context, key string) (*model.Parameter, error)
	List(ctx context.Context, q pagination.Query) ([]model.Parameter, int64, error)
	ExistsKey(ctx context.Context, key string, excludeID uint64) (bool, error)
	Delete(ctx context.Context, ids ...uint64) error
}

type FileRepository interface {
	Create(ctx context.Context, f *model.File) error
	List(ctx context.Context, filter model.FileFilter) ([]model.File, int64, error)
	Get(ctx context.Context, id uint64) (*model.File, error)
	Delete(ctx context.Context, ids ...uint64) error
	TotalSize(ctx context.Context) (int64, error)
	CountAll(ctx context.Context) (int64, error)
}

type JobRepository interface {
	Create(ctx context.Context, job *model.Job) error
	Update(ctx context.Context, job *model.Job) error
	Get(ctx context.Context, id uint64) (*model.Job, error)
	List(ctx context.Context, page, pageSize int) ([]model.Job, int64, error)
	ListEnabled(ctx context.Context) ([]model.Job, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	Delete(ctx context.Context, ids ...uint64) error
	CountAll(ctx context.Context) (int64, error)
}

type LogRepository interface {
	CreateOperation(ctx context.Context, log *model.OperationLog) error
	ListOperations(ctx context.Context, filter model.OperationLogFilter) ([]model.OperationLog, int64, error)
	DeleteOperations(ctx context.Context, ids ...uint64) error
	ClearOperations(ctx context.Context) error
	RecentOperations(ctx context.Context, limit int) ([]model.OperationLog, error)
	CreateLogin(ctx context.Context, log *model.LoginLog) error
	ListLogins(ctx context.Context, filter model.LoginLogFilter) ([]model.LoginLog, int64, error)
	ClearLogins(ctx context.Context) error
	LoginTrend(ctx context.Context, days int) ([]model.TrendPoint, error)
}
