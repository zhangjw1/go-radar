package scheduler

import (
	"context"
	"strings"
	"testing"

	"go-radar/internal/model"
	"go-radar/internal/scanners"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSpecsReadIntervalsFromEnvironment(t *testing.T) {
	t.Setenv("SCAN_INTERVAL_S5", "77")
	scheduler := New(nil, false)

	var s5 Spec
	for _, spec := range scheduler.Specs() {
		if spec.Name == "s5" {
			s5 = spec
			break
		}
	}
	if s5.IntervalSeconds != 77 {
		t.Fatalf("expected s5 interval 77, got %d", s5.IntervalSeconds)
	}
}

func TestSpecsReadRuntimeIntervalsFromSettings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AppSetting{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	db.Create(&model.AppSetting{Key: "scan_interval_s5", ValueJSON: "88"})

	scheduler := New(db, false)
	for _, spec := range scheduler.Specs() {
		if spec.Name == "s5" && spec.IntervalSeconds != 88 {
			t.Fatalf("expected runtime s5 interval 88, got %d", spec.IntervalSeconds)
		}
	}
}

func TestRunPlaceholderRecordsSkippedRun(t *testing.T) {
	t.Setenv("ENABLE_SCANNER_SX", "true")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ScannerRun{}, &model.AppSetting{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	scheduler := New(db, true)
	scheduler.runPlaceholder(Spec{Name: "sx", EnabledKey: "enable_scanner_sx", EnabledEnvKey: "ENABLE_SCANNER_SX"})

	var run model.ScannerRun
	if err := db.First(&run).Error; err != nil {
		t.Fatalf("read scanner run: %v", err)
	}
	if run.Scanner != "sx" || run.Status != "skipped" || run.Error == "" {
		t.Fatalf("unexpected scanner run: %#v", run)
	}
}

func TestRunScannerRecordsPanicWithStack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ScannerRun{}, &model.AppSetting{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	scheduler := New(db, true)
	scheduler.runScanner("sx", func(context.Context) (scanners.Result, error) {
		panic("kaboom")
	})

	var run model.ScannerRun
	if err := db.First(&run).Error; err != nil {
		t.Fatalf("read scanner run: %v", err)
	}
	if run.Scanner != "sx" || run.Status != "error" || !strings.Contains(run.Error, "panic: kaboom") {
		t.Fatalf("unexpected panic run: %#v", run)
	}
	if !strings.Contains(run.MetadataJSON, `"stack"`) || !strings.Contains(run.MetadataJSON, "kaboom") {
		t.Fatalf("expected panic stack metadata, got %s", run.MetadataJSON)
	}
}
