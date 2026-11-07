//go:build wireinject

package bootstrap

import (
	"github.com/google/wire"
	"gorm.io/gorm"

	"x-admin/internal/config"
	"x-admin/internal/handler"
	"x-admin/internal/jobs"
	"x-admin/internal/middleware"
	"x-admin/internal/pkg/captcha"
	"x-admin/internal/pkg/security"
	"x-admin/internal/repository"
	"x-admin/internal/router"
	"x-admin/internal/service"
)

// newApp 由 wire 生成实现（wire_gen.go）：按依赖图自动装配
// 全部仓储、服务与处理器，并返回装配资源的清理函数（失败路径由
// wire 调用，成功路径由 App.Close 调用）。修改本文件后运行 `make wire` 重新生成。
func newApp(cfg config.Config, db *gorm.DB) (*App, func(), error) {
	wire.Build(
		// 各层提供者集合
		repository.ProviderSet,
		service.ProviderSet,
		handler.ProviderSet,
		// 基础设施与服务适配器（providers.go）
		provideCache,
		provideStorage,
		provideTokens,
		provideHasher,
		provideCaptcha,
		provideWebAuthn,
		provideAuthOptions,
		provideLogService,
		provideUserService,
		provideFileService,
		provideProfileService,
		providePasskeyService,
		provideJobService,
		provideJobExecutor,
		provideAuthHandler,
		provideRouter,
		assembleApp,
		jobs.New,
		// 路由处理器聚合
		wire.Struct(new(router.Handlers), "*"),
		// 接口绑定：实现类型 → 消费方定义的端口
		wire.Bind(new(security.PasswordHasher), new(*security.BcryptHasher)),
		wire.Bind(new(service.CaptchaVerifier), new(*captcha.Generator)),
		wire.Bind(new(service.JobScheduler), new(*jobs.Cron)),
		wire.Bind(new(middleware.OperationRecorder), new(*service.LogService)),
	)
	return nil, nil, nil
}
