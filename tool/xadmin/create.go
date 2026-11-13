package xadmin

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

// CreateAPIOptions 描述新建 API 面的参数。
type CreateAPIOptions struct {
	Dir       string
	Name      string
	Version   string
	Prefix    string
	Auth      bool
	Table     string
	Migration string
	Force     bool
	// NoConfig 为 true 时不写入 configs/config.yaml，只输出配置片段。
	NoConfig bool
}

// CreateHandlerOptions 描述新建 handler 的参数。
type CreateHandlerOptions struct {
	Dir        string
	API        string
	Version    string
	Name       string
	Path       string
	Permission string
	Force      bool
}

// CreateResult 是生成器的输出，便于 CLI 打印或 JSON 输出。
type CreateResult struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	API       string   `json:"api,omitempty"`
	Version   string   `json:"version"`
	Auth      bool     `json:"auth,omitempty"`
	Table     string   `json:"table,omitempty"`
	Migration string   `json:"migration,omitempty"`
	Files     []string `json:"files"`
	Config    string   `json:"config,omitempty"`
	// ConfigPath 是自动写入的配置文件；为空表示未写入，需要手动添加 Config 片段。
	ConfigPath string   `json:"config_path,omitempty"`
	NextSteps  []string `json:"next_steps,omitempty"`
}

var (
	apiNameRe     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	versionRe     = regexp.MustCompile(`^v[0-9]+$`)
	handlerNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	permissionRe  = regexp.MustCompile(`^[a-z][a-z0-9_-]*(?::[a-z][a-z0-9_-]*)+$`)
)

// builtinSurfaces 是框架内置 API 面，不能通过 create api 重复注册。
// client / merchant / partner 等面都由用户用 create api 生成。
var builtinSurfaces = map[string]bool{
	"admin": true,
}

// CreateAPI 生成一个新的 API 面：
//   - internal/handler/<name>/<version>/handler.go
//   - internal/router/<name>_<version>.go（init 中注册 Surface）
//   - 开启 --auth 时额外生成用户表迁移，并在 Surface 中注册注册/登录/刷新/登出/me 接口
//
// 生成后会自动写入 configs/config.yaml 的 api 段（按名称排序）；
// 没有配置文件时只输出配置片段。
func CreateAPI(opts CreateAPIOptions) (*CreateResult, error) {
	workDir, err := prepareCreateDir(opts.Dir)
	if err != nil {
		return nil, err
	}
	name, err := normalizeCreateName("api", opts.Name)
	if err != nil {
		return nil, err
	}
	if builtinSurfaces[name] {
		return nil, fmt.Errorf("%q 是内置 API 面；用 create handler --api %s 添加接口", name, name)
	}
	version, err := normalizeVersion(opts.Version)
	if err != nil {
		return nil, err
	}
	defaultPrefix := "/" + name + "/" + version
	if suffix := "-" + version; strings.HasSuffix(name, suffix) {
		defaultPrefix = "/" + strings.TrimSuffix(name, suffix) + "/" + version
	}
	prefix, err := normalizePrefix(opts.Prefix, defaultPrefix)
	if err != nil {
		return nil, err
	}
	if !opts.Auth && (opts.Table != "" || opts.Migration != "") {
		return nil, fmt.Errorf("--table / --migration 只能与 --auth 一起使用")
	}

	table := ""
	migration := ""
	if opts.Auth {
		table, err = normalizeTable(opts.Table, defaultTableName(name))
		if err != nil {
			return nil, err
		}
		migration, err = normalizeMigration(opts.Migration, workDir)
		if err != nil {
			return nil, err
		}
	}

	module, err := ModulePath(workDir)
	if err != nil {
		return nil, fmt.Errorf("读取 go.mod 失败: %w", err)
	}

	data := createTemplateData{
		Module:  module,
		APIName: name,
		Version: version,
		Prefix:  prefix,
		Package: packageName(name, version),
		Auth:    opts.Auth,
		Table:   table,
	}
	handlerPath := filepath.Join(workDir, "internal", "handler", name, version, "handler.go")
	routerPath := filepath.Join(workDir, "internal", "router", fileName(name)+"_"+version+".go")

	// 一个 Surface 名称只能注册一次；不同版本使用不同的 --name（例如 partner-v2）。
	if err := ensureSurfaceNotRegistered(workDir, name, routerPath); err != nil {
		return nil, err
	}

	routerTemplate := apiRouterTmpl
	if opts.Auth {
		routerTemplate = apiRouterAuthTmpl
	}
	files := []createFile{
		{path: handlerPath, tmpl: apiHandlerTmpl, gofmt: true},
		{path: routerPath, tmpl: routerTemplate, gofmt: true},
	}
	if opts.Auth {
		base := filepath.Join(workDir, "migrations", migration+"_"+table)
		files = append(files,
			createFile{path: base + ".up.sql", tmpl: apiAuthMigrationUpTmpl},
			createFile{path: base + ".down.sql", tmpl: apiAuthMigrationDownTmpl},
		)
	}
	created, err := renderCreateFiles(workDir, files, data, opts.Force)
	if err != nil {
		return nil, err
	}

	config := fmt.Sprintf("api:\n  %s:\n    enabled: true\n    prefix: %s\n", name, prefix)
	if opts.Auth {
		config += "    table: " + table + "\n"
	}
	configPath := ""
	if !opts.NoConfig {
		configPath, err = writeSurfaceConfig(workDir, name, prefix, table)
		if err != nil {
			return nil, fmt.Errorf("%w；生成的文件已保留，可手动修正 %s 或加 --no-config 重试", err, surfaceConfigRel)
		}
	}
	steps := []string{"运行 go build ./... 验证生成代码"}
	if configPath == "" {
		steps = append([]string{"把 config 片段加入 configs/config.yaml"}, steps...)
	}
	if opts.Auth {
		steps = append(steps,
			"启动应用让迁移创建 "+table+" 表（或手动执行 migrations/"+migration+"_"+table+".up.sql）",
			"注册/登录接口：POST "+prefix+"/auth/register、POST "+prefix+"/auth/login、POST "+prefix+"/auth/refresh",
		)
	}
	steps = append(steps, "继续用 go tool xadmin create handler --api "+name+" --version "+version+" --name <module> 添加业务接口")

	return &CreateResult{
		Kind:       "api",
		Name:       name,
		Version:    version,
		Auth:       opts.Auth,
		Table:      table,
		Migration:  migration,
		Files:      created,
		Config:     config,
		ConfigPath: configPath,
		NextSteps:  steps,
	}, nil
}

// CreateHandler 在指定 API 面和版本下生成 handler 与路由模块。
// 路由模块在 init 中调用 router.RegisterModule，不需要手改现有路由文件。
func CreateHandler(opts CreateHandlerOptions) (*CreateResult, error) {
	workDir, err := prepareCreateDir(opts.Dir)
	if err != nil {
		return nil, err
	}
	api, err := normalizeCreateName("api", opts.API)
	if err != nil {
		return nil, err
	}
	version, err := normalizeVersion(opts.Version)
	if err != nil {
		return nil, err
	}
	name, err := normalizeCreateName("handler", opts.Name)
	if err != nil {
		return nil, err
	}
	routePath, err := normalizePrefix(opts.Path, "/"+strings.ReplaceAll(name, "_", "-"))
	if err != nil {
		return nil, err
	}
	permission := strings.TrimSpace(opts.Permission)
	if strings.EqualFold(permission, "none") {
		permission = ""
	} else if permission == "" && api == "admin" {
		permission = "system:" + name
	}
	if permission != "" {
		if !permissionRe.MatchString(permission) {
			return nil, fmt.Errorf("权限码 %q 不合法：使用 模块:资源 格式，例如 system:order", permission)
		}
	}
	module, err := ModulePath(workDir)
	if err != nil {
		return nil, fmt.Errorf("读取 go.mod 失败: %w", err)
	}

	typeName := exportedName(name)
	data := createTemplateData{
		Module:     module,
		APIName:    api,
		Version:    version,
		Name:       name,
		Path:       routePath,
		Type:       typeName,
		Package:    packageName(api, version),
		Permission: permission,
	}
	handlerPath := filepath.Join(workDir, "internal", "handler", api, version, fileName(name)+".go")
	routerPath := filepath.Join(workDir, "internal", "router", fileName(api)+"_"+version+"_"+fileName(name)+".go")

	files := []createFile{
		{path: handlerPath, tmpl: handlerTmpl, gofmt: true},
		{path: routerPath, tmpl: handlerRouterTmpl, gofmt: true},
	}
	created, err := renderCreateFiles(workDir, files, data, opts.Force)
	if err != nil {
		return nil, err
	}

	steps := []string{
		"在 " + relativeTo(workDir, handlerPath) + " 中接入 service，替换 TODO",
		"运行 go build ./... 验证生成代码",
	}
	if !builtinSurfaces[api] {
		surfaceFile := filepath.Join(workDir, "internal", "router", fileName(api)+"_"+version+".go")
		if _, err := os.Stat(surfaceFile); err != nil {
			steps = append([]string{
				"没有找到 API 面 " + api + " 的注册文件；先运行 go tool xadmin create api --name " + api + " --version " + version,
			}, steps...)
		}
	} else if version != "v1" {
		steps = append([]string{
			"内置 " + api + " 面当前默认挂载 v1；" + version + " 模块需要先增加对应 Surface 才会生效",
		}, steps...)
	}
	if api == "client" || api == "merchant" {
		steps = append(steps, "需要登录的接口要等 SurfaceAuth["+api+"] 接入后才会挂载")
	}
	if permission != "" {
		steps = append(steps, "后台权限码已使用 "+permission+":list/create/update/delete；记得在菜单种子中授权")
	}

	return &CreateResult{
		Kind:      "handler",
		Name:      name,
		API:       api,
		Version:   version,
		Files:     created,
		NextSteps: steps,
	}, nil
}

// createTemplateData 是生成模板的公共数据。
type createTemplateData struct {
	Module     string
	APIName    string
	Version    string
	Prefix     string
	Name       string
	Path       string
	Type       string
	Package    string
	Permission string
	Auth       bool
	Table      string
}

func ensureSurfaceNotRegistered(workDir, name, target string) error {
	matches, err := filepath.Glob(filepath.Join(workDir, "internal", "router", fileName(name)+"*.go"))
	if err != nil {
		return err
	}
	for _, path := range matches {
		if path == target {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), `RegisterSurface("`+name+`"`) {
			return fmt.Errorf("API 面 %q 已存在于 %s；需要新版本时使用不同的 --name，例如 %s-%s",
				name, relativeTo(workDir, path), name, versionFromPath(path))
		}
	}
	return nil
}

func versionFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".go")
	if i := strings.LastIndex(base, "_"); i >= 0 {
		return base[i+1:]
	}
	return "v2"
}

func prepareCreateDir(dir string) (string, error) {
	workDir, err := absProjectDir(dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err != nil {
		return "", fmt.Errorf("%s 不是 Go 项目（缺少 go.mod）", workDir)
	}
	if _, err := os.Stat(filepath.Join(workDir, "internal")); err != nil {
		return "", fmt.Errorf("%s 缺少 internal 目录，请在 x-admin 项目根目录运行", workDir)
	}
	return workDir, nil
}

func normalizeCreateName(kind, value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("需要 --name")
	}
	if kind == "handler" {
		if !handlerNameRe.MatchString(value) {
			return "", fmt.Errorf("handler 名称 %q 不合法：小写字母开头，只允许 a-z、0-9、_、-", value)
		}
		return value, nil
	}
	if !apiNameRe.MatchString(value) {
		return "", fmt.Errorf("API 面名称 %q 不合法：小写字母开头，只允许 a-z、0-9、-", value)
	}
	return value, nil
}

func normalizeVersion(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "v1"
	}
	if !versionRe.MatchString(value) {
		return "", fmt.Errorf("API 版本 %q 不合法：使用 v1、v2 这样的格式", value)
	}
	return value, nil
}

func normalizePrefix(value, fallback string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	if !strings.HasPrefix(value, "/") || value == "/" || strings.HasSuffix(value, "/") || strings.Contains(value, "//") {
		return "", fmt.Errorf("路径 %q 不合法：必须以 / 开头、不能以 / 结尾、不能包含 //", value)
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return "", fmt.Errorf("路径 %q 不能包含空白字符", value)
	}
	return value, nil
}

var tableNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// defaultTableName 根据 API 面名称推导账号表名，例如 client -> client_users。
func defaultTableName(api string) string {
	return strings.ReplaceAll(api, "-", "_") + "_users"
}

func normalizeTable(value, fallback string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = fallback
	}
	if !tableNameRe.MatchString(value) {
		return "", fmt.Errorf("表名 %q 不合法：小写字母开头，只允许 a-z、0-9、_", value)
	}
	if len(value) > 64 {
		return "", fmt.Errorf("表名 %q 太长（最多 64 字符）", value)
	}
	return value, nil
}

// normalizeMigration 返回六位迁移编号。
// 未指定时取当前最大编号 + 1，保证新迁移永远排在已有迁移之后。
func normalizeMigration(value, workDir string) (string, error) {
	if strings.TrimSpace(value) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n <= 0 || n > 999999 {
			return "", fmt.Errorf("迁移编号 %q 不合法：使用 1-999999 的数字", value)
		}
		return fmt.Sprintf("%06d", n), nil
	}
	n, err := nextMigrationNumber(workDir)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n), nil
}

var createMigrationNameRe = regexp.MustCompile(`^(\d{6})_.*\.up\.sql$`)

func nextMigrationNumber(workDir string) (int, error) {
	dir := filepath.Join(workDir, "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	max := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := createMigrationNameRe.FindStringSubmatch(entry.Name())
		if len(m) != 2 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err == nil && n > max {
			max = n
		}
	}
	return max + 1, nil
}

func packageName(api, version string) string {
	replacer := strings.NewReplacer("-", "", "_", "")
	base := replacer.Replace(api)
	if strings.HasSuffix(base, version) {
		return base
	}
	return base + version
}

func fileName(value string) string {
	replacer := strings.NewReplacer("-", "_")
	return replacer.Replace(value)
}

// exportedName 把 order_item / order-item 转成 OrderItem。
func exportedName(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	if b.Len() == 0 {
		return "Resource"
	}
	return b.String()
}

func relativeTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// createFile 是一个待生成的文件。
type createFile struct {
	path  string
	tmpl  *template.Template
	gofmt bool
}

// renderCreateFiles 先检查、渲染并格式化全部文件，再统一写入，避免半成品。
func renderCreateFiles(workDir string, files []createFile, data createTemplateData, force bool) ([]string, error) {
	type rendered struct {
		path string
		data []byte
	}
	out := make([]rendered, 0, len(files))
	for _, f := range files {
		if !force {
			if _, err := os.Stat(f.path); err == nil {
				return nil, fmt.Errorf("%s 已存在；确认要覆盖时加 --force", relativeTo(workDir, f.path))
			}
		}
		var raw strings.Builder
		if err := f.tmpl.Execute(&raw, data); err != nil {
			return nil, fmt.Errorf("渲染 %s: %w", relativeTo(workDir, f.path), err)
		}
		content := []byte(raw.String())
		if f.gofmt {
			formatted, err := format.Source(content)
			if err != nil {
				return nil, fmt.Errorf("生成的 %s 不是合法 Go 代码: %w", relativeTo(workDir, f.path), err)
			}
			content = formatted
		}
		out = append(out, rendered{path: f.path, data: content})
	}
	created := make([]string, 0, len(out))
	for _, f := range out {
		if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
			return created, err
		}
		if err := os.WriteFile(f.path, f.data, 0o644); err != nil {
			return created, err
		}
		created = append(created, relativeTo(workDir, f.path))
	}
	return created, nil
}

var apiHandlerTmpl = template.Must(template.New("api-handler").Parse(`// Package {{.Package}} 提供 {{.APIName}} {{.Version}} API 的 HTTP 处理器。
package {{.Package}}

import "github.com/gin-gonic/gin"

// Handler 是 {{.APIName}} {{.Version}} API 的入口处理器。
// 业务处理器按资源拆分到同包文件，例如 order.go、user.go。
type Handler struct{}

// NewHandler 创建 Handler。需要依赖时在这里注入 service。
func NewHandler() *Handler {
	return &Handler{}
}

// Register 注册公开接口。
func (h *Handler) Register(api *gin.RouterGroup) {
	// api.GET("/config", h.Config)
}

// RegisterSecured 注册需要登录/签名校验的接口。
func (h *Handler) RegisterSecured(secured *gin.RouterGroup) {
	// secured.GET("/me", h.Me)
}
`))

var apiRouterTmpl = template.Must(template.New("api-router").Parse(`package router

import (
	"github.com/gin-gonic/gin"

	"{{.Module}}/internal/config"
	{{.Package}} "{{.Module}}/internal/handler/{{.APIName}}/{{.Version}}"
)

func init() {
	RegisterSurface("{{.APIName}}", func(d Deps, cfg config.APISurfaceConfig) Surface {
		return Surface{
			Name:         "{{.APIName}}",
			Prefix:       cfg.Prefix,
			Enabled:      cfg.Enabled,
			Version:      "{{.Version}}",
			RateLimitKey: "rl:{{.APIName}}:global",
			Auth:         d.SurfaceAuth["{{.APIName}}"],
			Mount: func(api, secured *gin.RouterGroup) {
				h := {{.Package}}.NewHandler()
				h.Register(api)
				if secured != nil {
					h.RegisterSecured(secured)
				}
			},
		}
	})
}
`))

var apiRouterAuthTmpl = template.Must(template.New("api-router-auth").Parse(`package router

import (
	"time"

	"github.com/gin-gonic/gin"

	"{{.Module}}/internal/config"
	"{{.Module}}/internal/handler"
	{{.Package}} "{{.Module}}/internal/handler/{{.APIName}}/{{.Version}}"
	"{{.Module}}/internal/middleware"
	"{{.Module}}/internal/service"
)

func init() {
	RegisterSurface("{{.APIName}}", func(d Deps, cfg config.APISurfaceConfig) Surface {
		tokens := d.SurfaceTokens["{{.APIName}}"]
		if tokens == nil {
			panic("api surface {{.APIName}}: token manager is not configured")
		}
		if d.SurfaceAuthService == nil {
			panic("api surface {{.APIName}}: SurfaceAuthService is not configured")
		}
		table := cfg.Table
		if table == "" {
			table = "{{.Table}}"
		}
		auth := handler.NewSurfaceAuthHandler(d.SurfaceAuthService, tokens, service.SurfaceScope{
			Name:  "{{.APIName}}",
			Table: table,
		})
		return Surface{
			Name:         "{{.APIName}}",
			Prefix:       cfg.Prefix,
			Enabled:      cfg.Enabled,
			Version:      "{{.Version}}",
			RateLimitKey: "rl:{{.APIName}}:global",
			Auth:         middleware.Auth(tokens, auth, auth),
			Mount: func(api, secured *gin.RouterGroup) {
				api.POST("/auth/register", middleware.RateLimit(d.Cache, "rl:{{.APIName}}-register", 10, time.Minute), auth.Register)
				api.POST("/auth/login", middleware.RateLimit(d.Cache, "rl:{{.APIName}}-login", 10, time.Minute), auth.Login)
				api.POST("/auth/refresh", middleware.RateLimit(d.Cache, "rl:{{.APIName}}-refresh", 20, time.Minute), auth.Refresh)
				secured.POST("/auth/logout", auth.Logout)
				secured.GET("/auth/me", auth.Me)

				h := {{.Package}}.NewHandler()
				h.Register(api)
				if secured != nil {
					h.RegisterSecured(secured)
				}
			},
		}
	})
}
`))

var sqlQuoteFuncs = template.FuncMap{"bt": func() string { return "`" }}

var apiAuthMigrationUpTmpl = template.Must(template.New("api-auth-migration-up").Funcs(sqlQuoteFuncs).Parse(`-- 自定义 API 面 {{.APIName}} 的用户表。由 go tool xadmin create api --auth 生成。
CREATE TABLE IF NOT EXISTS {{bt}}{{.Table}}{{bt}} (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username        VARCHAR(64)     NOT NULL,
  password        VARCHAR(100)    NOT NULL,
  nickname        VARCHAR(64)     NOT NULL DEFAULT '',
  email           VARCHAR(128)    NOT NULL DEFAULT '',
  phone           VARCHAR(32)     NOT NULL DEFAULT '',
  avatar          VARCHAR(255)    NOT NULL DEFAULT '',
  status          TINYINT         NOT NULL DEFAULT 1,
  session_version BIGINT          NOT NULL DEFAULT 0,
  last_login_at   DATETIME(3)     NULL,
  last_login_ip   VARCHAR(64)     NOT NULL DEFAULT '',
  created_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at      DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at      DATETIME(3)     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_username (username),
  KEY idx_status (status),
  KEY idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`))

var apiAuthMigrationDownTmpl = template.Must(template.New("api-auth-migration-down").Funcs(sqlQuoteFuncs).Parse(`DROP TABLE IF EXISTS {{bt}}{{.Table}}{{bt}};
`))

var handlerTmpl = template.Must(template.New("handler").Parse(`// Package {{.Package}} 提供 {{.APIName}} {{.Version}} API 的 HTTP 处理器。
package {{.Package}}

import (
	"github.com/gin-gonic/gin"

	{{if .Permission}}"{{.Module}}/internal/middleware"
	{{end}}"{{.Module}}/internal/pkg/response"
)

// {{.Type}}Handler 处理 {{.Name}} 相关请求。
type {{.Type}}Handler struct{}

// New{{.Type}}Handler 创建处理器。需要依赖时在这里注入 service。
func New{{.Type}}Handler() *{{.Type}}Handler {
	return &{{.Type}}Handler{}
}

// Register 注册公开接口。默认留空，确认无需登录的接口再放进来。
func (h *{{.Type}}Handler) Register(api *gin.RouterGroup) {
	// api.GET("{{.Path}}/public", h.Public)
}

// RegisterSecured 注册需要登录/签名校验的接口。
func (h *{{.Type}}Handler) RegisterSecured(secured *gin.RouterGroup) {
	secured.GET("{{.Path}}", {{if .Permission}}middleware.Require("{{.Permission}}:list"), {{end}}h.List)
	secured.GET("{{.Path}}/:id", {{if .Permission}}middleware.Require("{{.Permission}}:list"), {{end}}h.Get)
	secured.POST("{{.Path}}", {{if .Permission}}middleware.Require("{{.Permission}}:create"), {{end}}h.Create)
	secured.PUT("{{.Path}}/:id", {{if .Permission}}middleware.Require("{{.Permission}}:update"), {{end}}h.Update)
	secured.DELETE("{{.Path}}/:id", {{if .Permission}}middleware.Require("{{.Permission}}:delete"), {{end}}h.Delete)
}

// List 返回列表。TODO: 接入 service。
func (h *{{.Type}}Handler) List(c *gin.Context) {
	response.OK(c, nil)
}

// Get 返回详情。TODO: 接入 service。
func (h *{{.Type}}Handler) Get(c *gin.Context) {
	response.OK(c, nil)
}

// Create 创建资源。TODO: 接入 service。
func (h *{{.Type}}Handler) Create(c *gin.Context) {
	response.OK(c, nil)
}

// Update 更新资源。TODO: 接入 service。
func (h *{{.Type}}Handler) Update(c *gin.Context) {
	response.OK(c, nil)
}

// Delete 删除资源。TODO: 接入 service。
func (h *{{.Type}}Handler) Delete(c *gin.Context) {
	response.OK(c, nil)
}
`))

var handlerRouterTmpl = template.Must(template.New("handler-router").Parse(`package router

import (
	"github.com/gin-gonic/gin"

	{{.Package}} "{{.Module}}/internal/handler/{{.APIName}}/{{.Version}}"
)

func init() {
	RegisterModule(Module{
		API:     "{{.APIName}}",
		Version: "{{.Version}}",
		Register: func(d Deps, api, secured *gin.RouterGroup) {
			h := {{.Package}}.New{{.Type}}Handler()
			h.Register(api)
			if secured != nil {
				h.RegisterSecured(secured)
			}
		},
	})
}
`))
