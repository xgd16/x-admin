// Package config 负责加载 x-admin 的运行配置。
//
// 配置来源的优先级为：内置默认值 < YAML 文件 < X_ADMIN_* 环境变量。
// 环境变量优先，便于容器化部署时覆盖镜像内的配置文件。
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是应用的全部配置，对应 configs/config.yaml 的顶层结构。
type Config struct {
	App      AppConfig      `yaml:"app"`      // 应用元信息
	Server   ServerConfig   `yaml:"server"`   // HTTP 服务参数
	Database DatabaseConfig `yaml:"database"` // MySQL 连接与连接池
	Redis    RedisConfig    `yaml:"redis"`    // 缓存/会话存储
	JWT      JWTConfig      `yaml:"jwt"`      // 访问令牌配置
	Security SecurityConfig `yaml:"security"` // 密码、锁定、验证码等安全策略
	Storage  StorageConfig  `yaml:"storage"`  // 文件上传存储
	WebAuthn WebAuthnConfig `yaml:"webauthn"` // Passkey（WebAuthn）配置
	CORS     CORSConfig     `yaml:"cors"`     // 跨域白名单
}

// AppConfig 描述应用名称、版本和运行环境。
type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Env     string `yaml:"env"` // development | production
}

// ServerConfig 描述 HTTP 服务的监听地址与超时时间。
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`     // 读取请求的超时时间
	WriteTimeout    time.Duration `yaml:"write_timeout"`    // 写响应的超时时间
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"` // 优雅退出等待上限
	// TrustedProxies 是允许携带 X-Forwarded-For / X-Real-IP 的代理网段。
	// 为空时不信任任何代理，ClientIP 始终使用 TCP 对端地址。
	TrustedProxies []string `yaml:"trusted_proxies"`
}

// DatabaseConfig 描述 MySQL DSN 与连接池参数。
type DatabaseConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int           `yaml:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"` // 连接最长复用时间
	AutoMigrate     bool          `yaml:"auto_migrate"`      // 启动时是否自动执行迁移
}

// RedisConfig 描述缓存驱动与连接信息。
type RedisConfig struct {
	Driver   string `yaml:"driver"` // redis | memory
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// JWTConfig 描述访问令牌的签发参数。
type JWTConfig struct {
	Secret     string        `yaml:"secret"`      // HS256 签名密钥，生产环境禁止使用默认值
	Issuer     string        `yaml:"issuer"`      // 签发者，解析时会校验
	AccessTTL  time.Duration `yaml:"access_ttl"`  // access token 有效期
	RefreshTTL time.Duration `yaml:"refresh_ttl"` // 勾选“记住我”时的 refresh token 有效期
	SessionTTL time.Duration `yaml:"session_ttl"` // 未勾选“记住我”时的 refresh token 有效期
}

// SecurityConfig 描述密码强度、登录锁定和验证码等安全策略。
type SecurityConfig struct {
	BcryptCost      int           `yaml:"bcrypt_cost"`      // bcrypt 计算成本
	MaxLoginFails   int           `yaml:"max_login_fails"`  // 连续失败多少次锁定账号
	LockDuration    time.Duration `yaml:"lock_duration"`    // 账号锁定时长
	CaptchaTTL      time.Duration `yaml:"captcha_ttl"`      // 验证码有效期
	PasswordMinLen  int           `yaml:"password_min_len"` // 密码最小长度
	PermissionCache time.Duration `yaml:"permission_cache"` // 权限缓存时长（预留）
}

// StorageConfig 描述本地文件存储的位置、访问前缀和单文件大小上限。
type StorageConfig struct {
	Root        string `yaml:"root"`          // 物理存储根目录
	URLPrefix   string `yaml:"url_prefix"`    // 对外访问的 URL 前缀
	MaxUploadMB int64  `yaml:"max_upload_mb"` // 单文件大小上限（MB）
}

// WebAuthnConfig 配置 Passkey（WebAuthn）流程。
// RPID 必须是有效域名，不能带协议和端口；RPOrigins 是完整的来源地址。
type WebAuthnConfig struct {
	RPDisplayName string   `yaml:"rp_display_name"`
	RPID          string   `yaml:"rp_id"`
	RPOrigins     []string `yaml:"rp_origins"`
}

// CORSConfig 描述允许跨域访问的来源列表，支持通配符 "*"。
type CORSConfig struct {
	Origins []string `yaml:"origins"`
}

// Default 返回一套可直接在本地开发环境运行的默认配置。
func Default() Config {
	return Config{
		App: AppConfig{Name: "x-admin", Version: "1.0.0", Env: "development"},
		Server: ServerConfig{
			Host: "0.0.0.0", Port: 8080,
			ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, ShutdownTimeout: 10 * time.Second,
			// 默认信任本机与容器/内网私有网段，覆盖 Docker Compose 中的 Nginx 反代。
			// 后端直接暴露到公网时应通过配置收紧该列表。
			TrustedProxies: []string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		},
		Database: DatabaseConfig{
			DSN:             "xadmin:xadmin@tcp(127.0.0.1:3306)/xadmin?charset=utf8mb4&parseTime=True&loc=Local",
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
			AutoMigrate:     true,
		},
		Redis: RedisConfig{Driver: "redis", Addr: "127.0.0.1:6379", DB: 0},
		JWT: JWTConfig{
			Secret: "x-admin-dev-secret-change-me", Issuer: "x-admin",
			AccessTTL: 2 * time.Hour, RefreshTTL: 7 * 24 * time.Hour, SessionTTL: 12 * time.Hour,
		},
		Security: SecurityConfig{
			BcryptCost: 10, MaxLoginFails: 5, LockDuration: 15 * time.Minute,
			CaptchaTTL: 5 * time.Minute, PasswordMinLen: 8, PermissionCache: 5 * time.Minute,
		},
		Storage: StorageConfig{Root: "data/uploads", URLPrefix: "/uploads", MaxUploadMB: 20},
		WebAuthn: WebAuthnConfig{
			RPDisplayName: "x-admin",
			RPID:          "localhost",
			RPOrigins:     []string{"http://localhost", "http://localhost:5173"},
		},
		CORS: CORSConfig{Origins: []string{"http://localhost:5173"}},
	}
}

// Load 读取配置：先以默认值为底，YAML 存在时覆盖，最后应用 X_ADMIN_* 环境变量。
// 配置文件不存在不算错误，方便只用环境变量部署；其他读取错误会直接返回。
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(b, &cfg); err != nil {
				return cfg, fmt.Errorf("config: %w", err)
			}
		case !os.IsNotExist(err):
			return cfg, fmt.Errorf("config: %w", err)
		}
	}
	applyEnv(&cfg)
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// applyEnv 用环境变量覆盖配置。未设置或无法解析的变量会被忽略，保持原值。
func applyEnv(cfg *Config) {
	set(&cfg.App.Env, "X_ADMIN_APP_ENV")
	set(&cfg.Server.Host, "X_ADMIN_SERVER_HOST")
	setInt(&cfg.Server.Port, "X_ADMIN_SERVER_PORT")
	if v, ok := os.LookupEnv("X_ADMIN_TRUSTED_PROXIES"); ok {
		// 显式设置为空或 none 表示不信任任何代理。
		if strings.TrimSpace(v) == "" || strings.EqualFold(strings.TrimSpace(v), "none") {
			cfg.Server.TrustedProxies = nil
		} else {
			cfg.Server.TrustedProxies = splitAndTrim(v)
		}
	}
	set(&cfg.Database.DSN, "X_ADMIN_DATABASE_DSN")
	set(&cfg.Redis.Driver, "X_ADMIN_REDIS_DRIVER")
	set(&cfg.Redis.Addr, "X_ADMIN_REDIS_ADDR")
	set(&cfg.Redis.Password, "X_ADMIN_REDIS_PASSWORD")
	setInt(&cfg.Redis.DB, "X_ADMIN_REDIS_DB")
	set(&cfg.JWT.Secret, "X_ADMIN_JWT_SECRET")
	set(&cfg.JWT.Issuer, "X_ADMIN_JWT_ISSUER")
	setDuration(&cfg.JWT.AccessTTL, "X_ADMIN_JWT_ACCESS_TTL")
	setDuration(&cfg.JWT.RefreshTTL, "X_ADMIN_JWT_REFRESH_TTL")
	setDuration(&cfg.JWT.SessionTTL, "X_ADMIN_JWT_SESSION_TTL")
	set(&cfg.Storage.Root, "X_ADMIN_STORAGE_ROOT")
	set(&cfg.Storage.URLPrefix, "X_ADMIN_STORAGE_URL_PREFIX")
	setInt64(&cfg.Storage.MaxUploadMB, "X_ADMIN_STORAGE_MAX_MB")
	set(&cfg.WebAuthn.RPDisplayName, "X_ADMIN_WEBAUTHN_RP_DISPLAY_NAME")
	set(&cfg.WebAuthn.RPID, "X_ADMIN_WEBAUTHN_RP_ID")
	// 列表类型的配置用逗号分隔，例如 "https://a.com,https://b.com"。
	if v := os.Getenv("X_ADMIN_WEBAUTHN_RP_ORIGINS"); v != "" {
		cfg.WebAuthn.RPOrigins = splitAndTrim(v)
	}
	if v := os.Getenv("X_ADMIN_CORS_ORIGINS"); v != "" {
		cfg.CORS.Origins = splitAndTrim(v)
	}
}

// validate 做启动前的关键校验，避免带着明显错误的配置运行。
func validate(cfg Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("config: invalid server port")
	}
	if len(cfg.JWT.Secret) < 16 {
		return fmt.Errorf("config: jwt secret must be at least 16 chars")
	}
	if cfg.App.Env == "production" {
		// 同时拒绝开发默认值和 Compose/.env.example 中的示例密钥，
		// 避免生产环境带着公开密钥启动。
		switch cfg.JWT.Secret {
		case Default().JWT.Secret, "change-me-x-admin-production-secret-2026":
			return fmt.Errorf("config: default jwt secret is not allowed in production")
		}
	}
	if cfg.Database.DSN == "" {
		return fmt.Errorf("config: database dsn is required")
	}
	if cfg.Storage.Root == "" {
		return fmt.Errorf("config: storage root is required")
	}
	for _, proxy := range cfg.Server.TrustedProxies {
		if net.ParseIP(proxy) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(proxy); err != nil {
			return fmt.Errorf("config: invalid trusted proxy %q", proxy)
		}
	}
	return nil
}

// set 在环境变量存在时覆盖字符串配置。
func set(target *string, key string) {
	if v, ok := os.LookupEnv(key); ok {
		*target = v
	}
}

// setInt 在环境变量存在且可解析为整数时覆盖 int 配置。
func setInt(target *int, key string) {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			*target = n
		}
	}
}

// setInt64 在环境变量存在且可解析为 int64 时覆盖配置。
func setInt64(target *int64, key string) {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			*target = n
		}
	}
}

// setDuration 在环境变量存在且符合 time.ParseDuration 格式时覆盖时长配置。
func setDuration(target *time.Duration, key string) {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			*target = d
		}
	}
}

// splitAndTrim 按逗号拆分并去掉空白项，用于解析列表型环境变量。
func splitAndTrim(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
