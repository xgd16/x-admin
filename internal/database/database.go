// Package database 封装 MySQL 连接、连接池以及版本化迁移。
package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"x-admin/internal/config"
	"x-admin/internal/model"
	"x-admin/migrations"
)

// Open 建立 GORM 连接并配置连接池。
// 开发环境打印 SQL，其他环境只记录慢查询和错误；连接建立后会立即 Ping 一次。
func Open(cfg config.DatabaseConfig, env string) (*gorm.DB, error) {
	level := gormlogger.Warn
	if env == "development" {
		level = gormlogger.Info
	}
	db, err := gorm.Open(gormmysql.Open(NormalizeDSN(cfg.DSN)), &gorm.Config{Logger: gormlogger.Default.LogMode(level),
		// 使用本地时区，保证 created_at / updated_at 与服务器时间一致。
		NowFunc: func() time.Time { return time.Now().Local() },
	})
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return db, nil
}

// Migrate 执行内嵌到二进制中的 golang-migrate SQL 迁移。
func Migrate(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return MigrateSQL(sqlDB)
}

// MigrateSQL 从 embed.FS 读取迁移脚本并升级到最新版本。
// 没有新版本时 migrate.ErrNoChange 属于正常情况，不视为错误。
func MigrateSQL(sqlDB *sql.DB) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	driver, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", source, "mysql", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("database: migrate: %w", err)
	}
	return nil
}

// AutoMigrate 仅用于测试和本地实验（SQLite），生产环境请使用 Migrate。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Department{}, &model.User{}, &model.Role{}, &model.Menu{},
		&model.UserRole{}, &model.RoleMenu{},
		&model.DictType{}, &model.DictItem{}, &model.Parameter{}, &model.File{}, &model.FileShare{},
		&model.Job{}, &model.JobRun{}, &model.OperationLog{}, &model.LoginLog{}, &model.Notification{}, &model.Passkey{},
	)
}

// Close 关闭底层数据库连接池。
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Health 通过 Ping 判断数据库是否可用，供 /healthz 使用。
func Health(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// NormalizeDSN 确保 DSN 开启 multiStatements。
// golang-migrate 的单个迁移文件可能包含多条语句；业务查询仍使用参数化，不受影响。
func NormalizeDSN(dsn string) string {
	if strings.Contains(dsn, "multiStatements=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "multiStatements=true"
}
