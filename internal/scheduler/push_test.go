package scheduler

import (
	"testing"
	"time"

	"go-radar/internal/model"
	"go-radar/internal/scanners"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDecidePushMatchesCriticalPolicies(t *testing.T) {
	resonance := &model.SignalEvent{Source: "system", SignalType: "resonance", Priority: "high"}
	if decision := decidePush(resonance, false); decision.channel != "immediate" || decision.cooldownExempt {
		t.Fatalf("unexpected resonance decision: %#v", decision)
	}

	s5MediumMomentum := &model.SignalEvent{Source: "s5", SignalType: "momentum", Priority: "medium"}
	if decision := decidePush(s5MediumMomentum, false); decision.channel != "immediate" || decision.quotaKey != "s5_momentum_medium" {
		t.Fatalf("unexpected s5 decision: %#v", decision)
	}

	s3Medium := &model.SignalEvent{Source: "s3", SignalType: "heat", Priority: "medium"}
	if decision := decidePush(s3Medium, false); decision.channel != "digest" {
		t.Fatalf("unexpected s3 decision: %#v", decision)
	}
}

func TestS5MediumMomentumQuotaUsesSetting(t *testing.T) {
	db := openSchedulerTestDB(t)
	s := New(db, true)

	if got := s.immediateQuotaLimit("s5_momentum_medium"); got != 1 {
		t.Fatalf("expected default quota 1, got %d", got)
	}

	db.Create(&model.AppSetting{Key: "s5_momentum_medium_quota", ValueJSON: "2"})
	if got := s.immediateQuotaLimit("s5_momentum_medium"); got != 2 {
		t.Fatalf("expected configured quota 2, got %d", got)
	}
}

func TestCreateResonanceSignalsForCrossSourceToken(t *testing.T) {
	db := openSchedulerTestDB(t)
	s := New(db, true)

	payloads := []scanners.SignalPayload{
		{Source: "s3", Chain: "bsc", Address: "0xabc", Symbol: "ABC", Name: "ABC", SignalType: "heat", Priority: "medium", Score: 70, Reason: "heat"},
		{Source: "s5", Chain: "bsc", Address: "0xabc", Symbol: "ABC", Name: "ABC", SignalType: "momentum", Priority: "medium", Score: 80, Reason: "momentum"},
	}
	var newSignals []*model.SignalEvent
	for _, payload := range payloads {
		stored, created, err := scanners.StoreSignalEvent(db, payload, 30)
		if err != nil {
			t.Fatalf("store signal: %v", err)
		}
		if created {
			newSignals = append(newSignals, stored)
		}
	}

	resonance, err := s.createResonanceSignals(newSignals)
	if err != nil {
		t.Fatalf("create resonance: %v", err)
	}
	if len(resonance) != 1 {
		t.Fatalf("expected one deduped resonance signal, got %d", len(resonance))
	}
	if resonance[0].Source != "system" || resonance[0].SignalType != "resonance" || resonance[0].Priority != "high" {
		t.Fatalf("unexpected resonance: %#v", resonance[0])
	}
}

func TestFindBlockingRecentPushHonorsPriorityUpgrade(t *testing.T) {
	db := openSchedulerTestDB(t)
	s := New(db, true)
	pushedAt := time.Now().UTC().Format(time.RFC3339Nano)
	db.Create(&model.SignalEvent{
		Source:     "s5",
		Chain:      "bsc",
		Address:    "0xabc",
		Symbol:     "ABC",
		SignalType: "momentum",
		Priority:   "medium",
		Score:      80,
		Reason:     "old",
		DedupeKey:  "old",
		CreatedAt:  pushedAt,
		PushedAt:   &pushedAt,
	})

	medium := &model.SignalEvent{Chain: "bsc", Address: "0xabc", Priority: "medium"}
	blocking, err := s.findBlockingRecentPush(medium, pushDecision{channel: "immediate"}, false, map[string]*model.SignalEvent{})
	if err != nil {
		t.Fatalf("find blocking: %v", err)
	}
	if blocking == nil {
		t.Fatalf("expected medium signal to be blocked")
	}

	high := &model.SignalEvent{Chain: "bsc", Address: "0xabc", Priority: "high"}
	blocking, err = s.findBlockingRecentPush(high, pushDecision{channel: "immediate"}, false, map[string]*model.SignalEvent{})
	if err != nil {
		t.Fatalf("find blocking: %v", err)
	}
	if blocking != nil {
		t.Fatalf("expected high priority upgrade to bypass cooldown")
	}
}

func openSchedulerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TokenProfile{}, &model.TokenSnapshot{}, &model.SignalEvent{}, &model.WatchlistItem{}, &model.ScannerRun{}, &model.AppSetting{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	return db
}
