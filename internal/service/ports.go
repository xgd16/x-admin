// Package service 实现 x-admin 的业务规则。
//
// 服务层只依赖本文件定义的仓储接口，不接触 gin.Context 或 GORM；
// 仓储实现由 bootstrap 在启动时注入，测试时可用 fake 替换。
package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
)

// ErrRecordNotFound 是仓储与 service 共享的“记录不存在”错误。
var ErrRecordNotFound = gorm.ErrRecordNotFound

// UserRepository 定义用户数据的读写能力。
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	Get(ctx context.Context, id uint64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	// ListByIDs 批量返回用户基础信息，供在线会话等聚合场景避免 N+1 查询。
	ListByIDs(ctx context.Context, ids []uint64) ([]model.User, error)
	List(ctx context.Context, filter model.UserFilter) ([]model.User, int64, error)
	Count(ctx context.Context, filter model.UserFilter) (int64, error)
	ExportPage(ctx context.Context, filter model.UserFilter, afterID uint64, limit int) ([]model.User, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	SetPassword(ctx context.Context, id uint64, hash string, bumpSessionVersion bool) error
	BumpSessionVersion(ctx context.Context, id uint64) error
	SessionState(ctx context.Context, id uint64) (int8, int64, error)
	Delete(ctx context.Context, ids ...uint64) error
	SetRoles(ctx context.Context, userID uint64, roleIDs []uint64) error
	ExistsUsername(ctx context.Context, username string, excludeID uint64) (bool, error)
	CountByStatus(ctx context.Context, status *int8) (int64, error)
}

// RoleRepository 定义角色数据的读写能力。
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

// MenuRepository 定义菜单与角色授权数据的读写能力。
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

// DepartmentRepository 定义部门数据的读写能力。
type DepartmentRepository interface {
	All(ctx context.Context) ([]model.Department, error)
	Get(ctx context.Context, id uint64) (*model.Department, error)
	Create(ctx context.Context, dept *model.Department) error
	Update(ctx context.Context, dept *model.Department) error
	Delete(ctx context.Context, ids ...uint64) error
	CountChildren(ctx context.Context, parentID uint64) (int64, error)
	CountUsers(ctx context.Context, deptID uint64) (int64, error)
}

// DictRepository 定义字典类型与字典项数据的读写能力。
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

// ParameterRepository 定义系统参数数据的读写能力。
type ParameterRepository interface {
	Create(ctx context.Context, p *model.Parameter) error
	Update(ctx context.Context, p *model.Parameter) error
	Get(ctx context.Context, id uint64) (*model.Parameter, error)
	GetByKey(ctx context.Context, key string) (*model.Parameter, error)
	ListPublic(ctx context.Context) ([]model.Parameter, error)
	List(ctx context.Context, q pagination.Query) ([]model.Parameter, int64, error)
	ExistsKey(ctx context.Context, key string, excludeID uint64) (bool, error)
	Delete(ctx context.Context, ids ...uint64) error
}

// FileRepository 定义文件元数据的数据访问能力。
type FileRepository interface {
	Create(ctx context.Context, f *model.File) error
	List(ctx context.Context, filter model.FileFilter) ([]model.File, int64, error)
	Get(ctx context.Context, id uint64) (*model.File, error)
	Delete(ctx context.Context, ids ...uint64) error
	TotalSize(ctx context.Context) (int64, error)
	CountAll(ctx context.Context) (int64, error)
	CreateShare(ctx context.Context, share *model.FileShare) error
	GetShareByHash(ctx context.Context, hash string) (*model.FileShare, error)
	ListShares(ctx context.Context, fileID uint64) ([]model.FileShare, error)
	RevokeShare(ctx context.Context, fileID, id uint64) error
	DeleteSharesByFile(ctx context.Context, fileID uint64) error
	PruneShares(ctx context.Context, before time.Time) error
}

// JobRepository 定义定时任务数据的读写能力。
type JobRepository interface {
	Create(ctx context.Context, job *model.Job) error
	Update(ctx context.Context, job *model.Job) error
	Get(ctx context.Context, id uint64) (*model.Job, error)
	List(ctx context.Context, page, pageSize int) ([]model.Job, int64, error)
	ListEnabled(ctx context.Context) ([]model.Job, error)
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	Delete(ctx context.Context, ids ...uint64) error
	CreateRun(ctx context.Context, run *model.JobRun) error
	ListRuns(ctx context.Context, jobID uint64, page, pageSize int) ([]model.JobRun, int64, error)
	PruneRuns(ctx context.Context, before time.Time) error
	DeleteRunsByJob(ctx context.Context, jobID uint64) error
	CountAll(ctx context.Context) (int64, error)
}

// Notifier 让各业务服务可以推送站内消息，而不依赖通知模块的实现。
type Notifier interface {
	Notify(ctx context.Context, in NotificationInput) error
}

// NotificationRepository 定义站内通知数据的读写能力。
type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	List(ctx context.Context, userID uint64, unreadOnly bool, page, pageSize int) ([]model.Notification, int64, error)
	CountUnread(ctx context.Context, userID uint64) (int64, error)
	MarkRead(ctx context.Context, userID, id uint64, at time.Time) error
	MarkAllRead(ctx context.Context, userID uint64, at time.Time) (int64, error)
	CleanBefore(ctx context.Context, days int) error
}

// PasskeyRepository 定义 WebAuthn 凭据数据的读写能力。
type PasskeyRepository interface {
	ListByUser(ctx context.Context, userID uint64) ([]model.Passkey, error)
	GetByCredentialID(ctx context.Context, credentialID string) (*model.Passkey, error)
	GetForUser(ctx context.Context, userID, id uint64) (*model.Passkey, error)
	CountByUser(ctx context.Context, userID uint64) (int64, error)
	CountAll(ctx context.Context) (int64, error)
	Create(ctx context.Context, passkey *model.Passkey) error
	RenameForUser(ctx context.Context, userID, id uint64, name string) error
	DeleteForUser(ctx context.Context, userID, id uint64) error
	UpdateCredential(ctx context.Context, id uint64, credential string, usedAt time.Time) error
}

// LogRepository 定义审计日志数据的读写能力。
type LogRepository interface {
	CreateOperation(ctx context.Context, log *model.OperationLog) error
	ListOperations(ctx context.Context, filter model.OperationLogFilter) ([]model.OperationLog, int64, error)
	CountOperations(ctx context.Context, filter model.OperationLogFilter) (int64, error)
	ExportOperationsPage(ctx context.Context, filter model.OperationLogFilter, afterID uint64, limit int) ([]model.OperationLog, error)
	DeleteOperations(ctx context.Context, ids ...uint64) error
	ClearOperations(ctx context.Context) error
	RecentOperations(ctx context.Context, limit int) ([]model.OperationLog, error)
	CreateLogin(ctx context.Context, log *model.LoginLog) error
	ListLogins(ctx context.Context, filter model.LoginLogFilter) ([]model.LoginLog, int64, error)
	CountLogins(ctx context.Context, filter model.LoginLogFilter) (int64, error)
	ExportLoginsPage(ctx context.Context, filter model.LoginLogFilter, afterID uint64, limit int) ([]model.LoginLog, error)
	ClearLogins(ctx context.Context) error
	LoginTrend(ctx context.Context, days int) ([]model.TrendPoint, error)
}

// ExportTaskRepository 定义异步导出任务记录的读写能力。
type ExportTaskRepository interface {
	Create(ctx context.Context, task *model.ExportTask) error
	UpdateFields(ctx context.Context, id uint64, fields map[string]any) error
	Get(ctx context.Context, id uint64) (*model.ExportTask, error)
	List(ctx context.Context, userID uint64, status *int8, page, pageSize int) ([]model.ExportTask, int64, error)
	Delete(ctx context.Context, ids ...uint64) error
	HardDelete(ctx context.Context, ids []uint64) error
	CountUnfinished(ctx context.Context, userID uint64) (int64, error)
	ListUnfinished(ctx context.Context, limit int) ([]model.ExportTask, error)
	ListExpired(ctx context.Context, before time.Time, limit int) ([]model.ExportTask, error)
}
