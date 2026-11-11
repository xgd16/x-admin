// Package database 封装 MySQL 连接、连接池以及版本化迁移。
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source"
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
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	driver, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		return err
	}
	if err := checkSchemaVersion(m, src); err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf(
				"database: migrate: %w; the database version is not part of the squashed migration history, "+
					"upgrade to v1.3.0 first or rebuild the database",
				err,
			)
		}
		return fmt.Errorf("database: migrate: %w", err)
	}
	return nil
}

// checkSchemaVersion 在升级前处理版本冲突。
// 迁移历史压缩后，旧库的 schema_migrations.version 可能大于内置基线；
// 若直接交给 golang-migrate，只会得到 "no migration found for version N"。
func checkSchemaVersion(m *migrate.Migrate, src source.Driver) error {
	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return nil // 全新数据库，直接执行基线。
	}
	if err != nil {
		return fmt.Errorf("database: read schema version: %w", err)
	}
	if dirty {
		return fmt.Errorf("database: schema version %d is dirty; repair the failed migration or rebuild the database", version)
	}

	latest, err := latestMigrationVersion(src)
	if err != nil {
		return fmt.Errorf("database: inspect migrations: %w", err)
	}
	if version <= latest {
		return nil
	}

	return fmt.Errorf(
		"database: schema version %d is not covered by this binary's migrations (latest %d); "+
			"if this database is v1.3.0 (schema version 16), back it up and run "+
			"`UPDATE schema_migrations SET version = %d, dirty = 0`; "+
			"for older versions, upgrade to v1.3.0 first or rebuild the database",
		version, latest, latest,
	)
}

// latestMigrationVersion 返回 embed 中最新的迁移版本号。
func latestMigrationVersion(src source.Driver) (uint, error) {
	version, err := src.First()
	if err != nil {
		return 0, err
	}
	for {
		next, err := src.Next(version)
		if errors.Is(err, os.ErrNotExist) {
			return version, nil
		}
		if err != nil {
			return 0, err
		}
		version = next
	}
}

// AutoMigrate 仅用于测试和本地实验（SQLite），生产环境请使用 Migrate。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Department{}, &model.User{}, &model.Role{}, &model.Menu{},
		&model.UserRole{}, &model.RoleMenu{},
		&model.DictType{}, &model.DictItem{}, &model.Parameter{}, &model.File{}, &model.FileShare{},
		&model.Job{}, &model.JobRun{}, &model.OperationLog{}, &model.LoginLog{}, &model.Notification{}, &model.Passkey{},
		&model.ExportTask{},
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
