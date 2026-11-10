package handler

import (
	"github.com/google/wire"
)

// ProviderSet 提供除 Auth 外的处理器构造器；
// AuthHandler 依赖配置原语（安全 Cookie、Cookie 有效期），由 bootstrap 的适配器提供。
var ProviderSet = wire.NewSet(
	NewDashboardHandler,
	NewUserHandler,
	NewRoleHandler,
	NewMenuHandler,
	NewDepartmentHandler,
	NewDictHandler,
	NewParameterHandler,
	NewFileHandler,
	NewJobHandler,
	NewLogHandler,
	NewProfileHandler,
	NewNotificationHandler,
	NewExportHandler,
	NewOnlineHandler,
	NewMonitorHandler,
)
