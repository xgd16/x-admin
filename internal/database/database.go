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

func Open(cfg config.DatabaseConfig, env string) (*gorm.DB, error) {
	level := gormlogger.Warn
	if env == "development" {
		level = gormlogger.Info
	}
	db, err := gorm.Open(gormmysql.Open(NormalizeDSN(cfg.DSN)), &gorm.Config{Logger: gormlogger.Default.LogMode(level),
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

// Migrate applies embedded golang-migrate SQL migrations.
func Migrate(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return MigrateSQL(sqlDB)
}

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

// AutoMigrate is only for tests and local experiments on SQLite.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Department{}, &model.User{}, &model.Role{}, &model.Menu{},
		&model.UserRole{}, &model.RoleMenu{},
		&model.DictType{}, &model.DictItem{}, &model.Parameter{}, &model.File{},
		&model.Job{}, &model.OperationLog{}, &model.LoginLog{},
	)
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func Health(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// NormalizeDSN lets golang-migrate execute one migration file that contains
// several statements. Parameterized application queries stay safe.
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
