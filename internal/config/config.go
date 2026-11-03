package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
	Security SecurityConfig `yaml:"security"`
	Storage  StorageConfig  `yaml:"storage"`
	CORS     CORSConfig     `yaml:"cors"`
}

type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Env     string `yaml:"env"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type DatabaseConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	AutoMigrate     bool          `yaml:"auto_migrate"`
}

type RedisConfig struct {
	Driver   string `yaml:"driver"` // redis | memory
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWTConfig struct {
	Secret     string        `yaml:"secret"`
	Issuer     string        `yaml:"issuer"`
	AccessTTL  time.Duration `yaml:"access_ttl"`
	RefreshTTL time.Duration `yaml:"refresh_ttl"`
}

type SecurityConfig struct {
	BcryptCost      int           `yaml:"bcrypt_cost"`
	MaxLoginFails   int           `yaml:"max_login_fails"`
	LockDuration    time.Duration `yaml:"lock_duration"`
	CaptchaTTL      time.Duration `yaml:"captcha_ttl"`
	PasswordMinLen  int           `yaml:"password_min_len"`
	PermissionCache time.Duration `yaml:"permission_cache"`
}

type StorageConfig struct {
	Root        string `yaml:"root"`
	URLPrefix   string `yaml:"url_prefix"`
	MaxUploadMB int64  `yaml:"max_upload_mb"`
}

type CORSConfig struct {
	Origins []string `yaml:"origins"`
}

func Default() Config {
	return Config{
		App:    AppConfig{Name: "x-admin", Version: "1.0.0", Env: "development"},
		Server: ServerConfig{Host: "0.0.0.0", Port: 8080, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, ShutdownTimeout: 10 * time.Second},
		Database: DatabaseConfig{
			DSN:             "xadmin:xadmin@tcp(127.0.0.1:3306)/xadmin?charset=utf8mb4&parseTime=True&loc=Local",
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
			AutoMigrate:     true,
		},
		Redis: RedisConfig{Driver: "redis", Addr: "127.0.0.1:6379", DB: 0},
		JWT:   JWTConfig{Secret: "x-admin-dev-secret-change-me", Issuer: "x-admin", AccessTTL: 2 * time.Hour, RefreshTTL: 7 * 24 * time.Hour},
		Security: SecurityConfig{
			BcryptCost: 10, MaxLoginFails: 5, LockDuration: 15 * time.Minute,
			CaptchaTTL: 5 * time.Minute, PasswordMinLen: 8, PermissionCache: 5 * time.Minute,
		},
		Storage: StorageConfig{Root: "data/uploads", URLPrefix: "/uploads", MaxUploadMB: 20},
		CORS:    CORSConfig{Origins: []string{"http://localhost:5173"}},
	}
}

// Load reads YAML (when present) over defaults, then applies X_ADMIN_* env vars.
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

func applyEnv(cfg *Config) {
	set(&cfg.App.Env, "X_ADMIN_APP_ENV")
	set(&cfg.Server.Host, "X_ADMIN_SERVER_HOST")
	setInt(&cfg.Server.Port, "X_ADMIN_SERVER_PORT")
	set(&cfg.Database.DSN, "X_ADMIN_DATABASE_DSN")
	set(&cfg.Redis.Driver, "X_ADMIN_REDIS_DRIVER")
	set(&cfg.Redis.Addr, "X_ADMIN_REDIS_ADDR")
	set(&cfg.Redis.Password, "X_ADMIN_REDIS_PASSWORD")
	setInt(&cfg.Redis.DB, "X_ADMIN_REDIS_DB")
	set(&cfg.JWT.Secret, "X_ADMIN_JWT_SECRET")
	set(&cfg.JWT.Issuer, "X_ADMIN_JWT_ISSUER")
	setDuration(&cfg.JWT.AccessTTL, "X_ADMIN_JWT_ACCESS_TTL")
	setDuration(&cfg.JWT.RefreshTTL, "X_ADMIN_JWT_REFRESH_TTL")
	set(&cfg.Storage.Root, "X_ADMIN_STORAGE_ROOT")
	set(&cfg.Storage.URLPrefix, "X_ADMIN_STORAGE_URL_PREFIX")
	setInt64(&cfg.Storage.MaxUploadMB, "X_ADMIN_STORAGE_MAX_MB")
	if v := os.Getenv("X_ADMIN_CORS_ORIGINS"); v != "" {
		cfg.CORS.Origins = splitAndTrim(v)
	}
}

func validate(cfg Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("config: invalid server port")
	}
	if len(cfg.JWT.Secret) < 16 {
		return fmt.Errorf("config: jwt secret must be at least 16 chars")
	}
	if cfg.App.Env == "production" && cfg.JWT.Secret == Default().JWT.Secret {
		return fmt.Errorf("config: default jwt secret is not allowed in production")
	}
	if cfg.Database.DSN == "" {
		return fmt.Errorf("config: database dsn is required")
	}
	if cfg.Storage.Root == "" {
		return fmt.Errorf("config: storage root is required")
	}
	return nil
}

func set(target *string, key string) {
	if v, ok := os.LookupEnv(key); ok {
		*target = v
	}
}

func setInt(target *int, key string) {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			*target = n
		}
	}
}

func setInt64(target *int64, key string) {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			*target = n
		}
	}
}

func setDuration(target *time.Duration, key string) {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			*target = d
		}
	}
}

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
