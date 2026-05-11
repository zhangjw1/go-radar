package db

import (
	"fmt"
	"os"
	"path/filepath"

	"go-radar/internal/config"
	"go-radar/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开 Go Radar 使用的 SQLite 数据库，并在需要时仅创建缺失表。
func Open(settings *config.Settings) (*gorm.DB, error) {
	if settings.DatabasePath != ":memory:" {
		// SQLite 文件可能位于新建目录下，打开前先确保父目录存在。
		if err := os.MkdirAll(filepath.Dir(settings.DatabasePath), 0755); err != nil {
			return nil, fmt.Errorf("create database directory for %q: %w", settings.DatabasePath, err)
		}
	}
	db, err := gorm.Open(sqlite.Open(settings.DatabasePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("connect sqlite %q: %w", settings.DatabasePath, err)
	}
	if settings.AutoMigrate {
		missingModels := []any{}
		// 只迁移缺失的表，避免 Go 版本改动 Python 已经创建好的表结构。
		for _, item := range []any{
			&model.TokenProfile{},
			&model.TokenSnapshot{},
			&model.SignalEvent{},
			&model.WatchlistItem{},
			&model.ScannerRun{},
			&model.AppSetting{},
		} {
			if !db.Migrator().HasTable(item) {
				missingModels = append(missingModels, item)
			}
		}
		if len(missingModels) > 0 {
			if err := db.AutoMigrate(missingModels...); err != nil {
				return nil, fmt.Errorf("auto migrate sqlite schema: %w", err)
			}
		}
	}
	return db, nil
}
