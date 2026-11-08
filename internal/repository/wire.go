package repository

import (
	"github.com/google/wire"

	"x-admin/internal/service"
)

// ProviderSet 提供全部仓储实现，并绑定到 service 层定义的端口接口。
var ProviderSet = wire.NewSet(
	NewUserRepo,
	NewRoleRepo,
	NewMenuRepo,
	NewDepartmentRepo,
	NewDictRepo,
	NewParameterRepo,
	NewFileRepo,
	NewJobRepo,
	NewLogRepo,
	NewNotificationRepo,
	NewPasskeyRepo,
	NewExportTaskRepo,
	wire.Bind(new(service.UserRepository), new(*UserRepo)),
	wire.Bind(new(service.RoleRepository), new(*RoleRepo)),
	wire.Bind(new(service.MenuRepository), new(*MenuRepo)),
	wire.Bind(new(service.DepartmentRepository), new(*DepartmentRepo)),
	wire.Bind(new(service.DictRepository), new(*DictRepo)),
	wire.Bind(new(service.ParameterRepository), new(*ParameterRepo)),
	wire.Bind(new(service.FileRepository), new(*FileRepo)),
	wire.Bind(new(service.JobRepository), new(*JobRepo)),
	wire.Bind(new(service.LogRepository), new(*LogRepo)),
	wire.Bind(new(service.NotificationRepository), new(*NotificationRepo)),
	wire.Bind(new(service.PasskeyRepository), new(*PasskeyRepo)),
	wire.Bind(new(service.ExportTaskRepository), new(*ExportTaskRepo)),
)
