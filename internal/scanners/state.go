package scanners

import (
	"encoding/json"
	"errors"
	"strings"

	"go-radar/internal/model"

	"gorm.io/gorm"
)

func LoadState(db *gorm.DB, source string, key string, target any) (bool, error) {
	if db == nil {
		return false, nil
	}
	var row model.ScannerState
	err := db.Where("source = ? AND key = ?", strings.ToLower(source), strings.TrimSpace(key)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if target == nil {
		return true, nil
	}
	if err := json.Unmarshal([]byte(row.ValueJSON), target); err != nil {
		return true, err
	}
	return true, nil
}

func SaveState(db *gorm.DB, source string, key string, value any) error {
	if db == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	source = strings.ToLower(source)
	key = strings.TrimSpace(key)
	now := nowString()
	var row model.ScannerState
	err = db.Where("source = ? AND key = ?", source, key).First(&row).Error
	if err == nil {
		row.ValueJSON = string(encoded)
		row.UpdatedAt = now
		return db.Save(&row).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	row = model.ScannerState{Source: source, Key: key, ValueJSON: string(encoded), UpdatedAt: now}
	return db.Create(&row).Error
}
