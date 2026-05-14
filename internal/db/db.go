package db

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go-radar/internal/config"
	"go-radar/internal/insider"
	"go-radar/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开 Go Radar 使用的数据库，并在需要时创建缺失表。
func Open(settings *config.Settings) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)

	switch settings.DatabaseDriver {
	case config.DBDriverSQLite:
		if settings.DatabasePath != ":memory:" {
			if err := os.MkdirAll(filepath.Dir(settings.DatabasePath), 0755); err != nil {
				return nil, fmt.Errorf("create database directory for %q: %w", settings.DatabasePath, err)
			}
		}
		db, err = gorm.Open(sqlite.Open(settings.DatabasePath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			return nil, fmt.Errorf("connect sqlite %q: %w", settings.DatabasePath, err)
		}
	case config.DBDriverPostgres:
		db, err = gorm.Open(postgres.Open(settings.DatabaseURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			return nil, fmt.Errorf("connect postgres %q: %w", settings.DatabaseURL, err)
		}
	default:
		return nil, fmt.Errorf("unsupported database driver %q", settings.DatabaseDriver)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if settings.AutoMigrate {
		models := []any{
			&model.TokenProfile{},
			&model.TokenSnapshot{},
			&model.SignalEvent{},
			&model.WatchlistItem{},
			&model.ScannerRun{},
			&model.AppSetting{},
			&model.ScannerState{},
			&insider.Wallet{},
			&insider.TokenAccount{},
			&insider.Transaction{},
			&insider.PriceRecord{},
			&insider.AlertRule{},
			&insider.AlertHistory{},
			&insider.WalletSnapshot{},
			&insider.NotificationChannel{},
		}
		missingModels := []any{}
		for _, item := range models {
			if !db.Migrator().HasTable(item) {
				missingModels = append(missingModels, item)
			}
		}
		if len(missingModels) > 0 {
			if err := db.AutoMigrate(missingModels...); err != nil {
				return nil, fmt.Errorf("auto migrate schema: %w", err)
			}
		}
	}

	return db, nil
}
