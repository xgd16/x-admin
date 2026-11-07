package service

import (
	"github.com/google/wire"
)

// ProviderSet 提供不依赖配置原语的服务构造器；
// 依赖配置数值（密码长度、上传上限等）的服务由 bootstrap 的适配器提供。
var ProviderSet = wire.NewSet(
	NewAuthService,
	NewNotificationService,
	NewRoleService,
	NewMenuService,
	NewDepartmentService,
	NewDictService,
	NewParameterService,
	NewDashboardService,
	wire.Bind(new(Notifier), new(*NotificationService)),
)
