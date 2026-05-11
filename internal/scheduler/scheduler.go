package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-radar/internal/model"
	"go-radar/internal/scanners"
	s1scanner "go-radar/internal/scanners/s1"
	s2scanner "go-radar/internal/scanners/s2"
	s3scanner "go-radar/internal/scanners/s3"
	s5scanner "go-radar/internal/scanners/s5"
	s7scanner "go-radar/internal/scanners/s7"

	"gorm.io/gorm"
)

// Spec 描述一个扫描任务的运行参数和对应的配置键。
type Spec struct {
	Name            string // Name 是扫描器编号，例如 s1、s2、s7。
	EnabledKey      string // EnabledKey 是 settings 表中控制启停的键名。
	IntervalKey     string // IntervalKey 是 settings 表中控制间隔秒数的键名。
	EnabledEnvKey   string // EnabledEnvKey 是启停开关的环境变量兜底键名。
	IntervalEnvKey  string // IntervalEnvKey 是扫描间隔的环境变量兜底键名。
	IntervalSeconds int    // IntervalSeconds 是默认扫描间隔秒数。
}

// Scheduler 负责按固定间隔运行已迁移到 Go 的扫描器。
type Scheduler struct {
	db      *gorm.DB           // db 是扫描结果、运行记录和动态配置的读写入口。
	specs   []Spec             // specs 是所有已注册扫描任务的静态定义。
	enabled bool               // enabled 是全局调度器开关；关闭时页面仍可读数据但不会扫描。
	cancel  context.CancelFunc // cancel 用于停止后台 goroutine。
	wg      sync.WaitGroup     // wg 等待所有后台任务安全退出。
}

// New 创建调度器并注册当前 Go 版本支持的扫描器。
func New(db *gorm.DB, enabled bool) *Scheduler {
	return &Scheduler{
		db:      db,
		enabled: enabled,
		specs: []Spec{
			{Name: "s7", EnabledKey: "enable_scanner_s7", IntervalKey: "scan_interval_s7", EnabledEnvKey: "ENABLE_SCANNER_S7", IntervalEnvKey: "SCAN_INTERVAL_S7", IntervalSeconds: envInt("SCAN_INTERVAL_S7", 20)},
			{Name: "s5", EnabledKey: "enable_scanner_s5", IntervalKey: "scan_interval_s5", EnabledEnvKey: "ENABLE_SCANNER_S5", IntervalEnvKey: "SCAN_INTERVAL_S5", IntervalSeconds: envInt("SCAN_INTERVAL_S5", 120)},
			{Name: "s3", EnabledKey: "enable_scanner_s3", IntervalKey: "scan_interval_s3", EnabledEnvKey: "ENABLE_SCANNER_S3", IntervalEnvKey: "SCAN_INTERVAL_S3", IntervalSeconds: envInt("SCAN_INTERVAL_S3", 300)},
			{Name: "s2", EnabledKey: "enable_scanner_s2", IntervalKey: "scan_interval_s2", EnabledEnvKey: "ENABLE_SCANNER_S2", IntervalEnvKey: "SCAN_INTERVAL_S2", IntervalSeconds: envInt("SCAN_INTERVAL_S2", 120)},
			{Name: "s1", EnabledKey: "enable_scanner_s1", IntervalKey: "scan_interval_s1", EnabledEnvKey: "ENABLE_SCANNER_S1", IntervalEnvKey: "SCAN_INTERVAL_S1", IntervalSeconds: envInt("SCAN_INTERVAL_S1", 30)},
		},
	}
}

// Enabled 返回全局调度器是否启用。
func (s *Scheduler) Enabled() bool {
	return s.enabled
}

// Specs 返回任务定义副本，并合并数据库中的动态间隔配置。
func (s *Scheduler) Specs() []Spec {
	specs := append([]Spec(nil), s.specs...)
	for i := range specs {
		specs[i].IntervalSeconds = settingInt(s.db, specs[i].IntervalKey, specs[i].IntervalEnvKey, specs[i].IntervalSeconds)
	}
	return specs
}

// Start 启动每个扫描器对应的后台循环。
func (s *Scheduler) Start(parent context.Context) {
	if !s.enabled {
		log.Print("Go scheduler disabled; set GO_RADAR_ENABLE_SCHEDULER=true to enable placeholder jobs")
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	for _, spec := range s.specs {
		spec := spec
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					s.runPlaceholder(spec)
					intervalSeconds := settingInt(s.db, spec.IntervalKey, spec.IntervalEnvKey, spec.IntervalSeconds)
					timer.Reset(time.Duration(intervalSeconds) * time.Second)
				}
			}
		}()
	}
	log.Printf("Go scheduler enabled with %d placeholder jobs", len(s.specs))
}

// Stop 请求后台任务退出，并等待所有任务完成。
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// runPlaceholder 根据配置决定本轮是否执行具体扫描器。
func (s *Scheduler) runPlaceholder(spec Spec) {
	if !s.runtimeBool(spec.EnabledKey, envBool(spec.EnabledEnvKey, false)) {
		s.recordRun(spec.Name, "skipped", map[string]any{"reason": "disabled"}, "")
		return
	}
	if spec.Name == "s2" {
		s.runScanner("s2", s2scanner.NewScanner(s.db).Scan)
		return
	}
	if spec.Name == "s3" {
		s.runScanner("s3", s3scanner.NewScanner(s.db).Scan)
		return
	}
	if spec.Name == "s1" {
		s.runScanner("s1", s1scanner.NewScanner(s.db).Scan)
		return
	}
	if spec.Name == "s7" {
		s.runScanner("s7", s7scanner.NewScanner(s.db).Scan)
		return
	}
	if spec.Name == "s5" {
		s.runScanner("s5", s5scanner.NewScanner(s.db).Scan)
		return
	}
	s.recordRun(spec.Name, "skipped", map[string]any{"reason": "go_scanner_not_migrated"}, "scanner not migrated to Go yet")
}

// runScanner 执行一次扫描、入库快照和信号，并处理共振信号与 Telegram 推送。
func (s *Scheduler) runScanner(name string, scan func(context.Context) (scanners.Result, error)) {
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, err := scan(ctx)
	if err != nil {
		s.recordRunWithStart(name, startedAt, "error", map[string]any{}, err.Error(), 0, 0)
		return
	}

	snapshotCount := 0
	for _, snapshot := range result.Snapshots {
		if err := scanners.StoreSnapshot(s.db, snapshot); err != nil {
			s.recordRunWithStart(name, startedAt, "error", map[string]any{"warnings": result.Warnings}, fmt.Sprintf("store snapshot: %v", err), len(result.Signals), snapshotCount)
			return
		}
		snapshotCount++
	}
	newSignals := []*model.SignalEvent{}
	signalCount := 0
	for _, signal := range result.Signals {
		stored, created, err := scanners.StoreSignalEvent(s.db, signal, settingInt(s.db, "signal_time_bucket_minutes", "SIGNAL_TIME_BUCKET_MINUTES", 30))
		if err != nil {
			s.recordRunWithStart(name, startedAt, "error", map[string]any{"warnings": result.Warnings}, fmt.Sprintf("store signal: %v", err), signalCount, snapshotCount)
			return
		}
		if created {
			signalCount++
			newSignals = append(newSignals, stored)
		}
	}
	resonanceSignals, err := s.createResonanceSignals(newSignals)
	if err != nil {
		s.recordRunWithStart(name, startedAt, "error", map[string]any{"warnings": result.Warnings}, fmt.Sprintf("resonance: %v", err), signalCount, snapshotCount)
		return
	}
	signalCount += len(resonanceSignals)

	pushedIDs, err := s.pushSignals(ctx, newSignals, resonanceSignals, name)
	if err != nil {
		s.recordRunWithStart(name, startedAt, "error", map[string]any{"warnings": result.Warnings}, fmt.Sprintf("push signals: %v", err), signalCount, snapshotCount)
		return
	}
	if err := s.markSignalsPushed(pushedIDs); err != nil {
		s.recordRunWithStart(name, startedAt, "error", map[string]any{"warnings": result.Warnings}, fmt.Sprintf("mark pushed: %v", err), signalCount, snapshotCount)
		return
	}
	metadata := result.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["warnings"] = result.Warnings
	metadata["pushed_count"] = len(pushedIDs)
	metadata["resonance_count"] = len(resonanceSignals)
	s.recordRunWithStart(name, startedAt, "ok", metadata, "", signalCount, snapshotCount)
}

// envBool 从环境变量解析布尔配置。
func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "on")
}

// envInt 从环境变量解析正整数配置。
func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// recordRun 使用当前时间记录一次扫描运行结果。
func (s *Scheduler) recordRun(scanner string, status string, metadata map[string]any, errorText string) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.recordRunWithStart(scanner, now, status, metadata, errorText, 0, 0)
}

// recordRunWithStart 写入完整 scanner_runs 记录。
func (s *Scheduler) recordRunWithStart(scanner string, startedAt string, status string, metadata map[string]any, errorText string, signalCount int, snapshotCount int) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	metadataJSON, _ := json.Marshal(metadata)
	run := model.ScannerRun{
		Scanner:       scanner,
		StartedAt:     startedAt,
		FinishedAt:    now,
		Status:        status,
		Error:         errorText,
		SignalCount:   int64(signalCount),
		SnapshotCount: int64(snapshotCount),
		MetadataJSON:  string(metadataJSON),
	}
	if err := s.db.Create(&run).Error; err != nil {
		log.Printf("record scanner run for %s failed: %v", scanner, err)
	}
}

// runtimeBool 优先读取 settings 表中的布尔配置，缺失时使用 fallback。
func (s *Scheduler) runtimeBool(key string, fallback bool) bool {
	var row model.AppSetting
	if err := s.db.Where("key = ?", key).First(&row).Error; err == nil {
		var value bool
		if json.Unmarshal([]byte(row.ValueJSON), &value) == nil {
			return value
		}
	}
	return fallback
}
