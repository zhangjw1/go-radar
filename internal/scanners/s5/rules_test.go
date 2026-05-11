package s5

import (
	"testing"

	"go-radar/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEvaluateMomentumRequiresEnoughHistory(t *testing.T) {
	result := EvaluateMomentum(nil, MomentumRow{MC: 100, Volume: 10, Price: 1, Buys1H: 1}, 3, 5)
	if result.Triggered || result.Reason != "not_enough_history" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEvaluateMomentumRejectsFlatState(t *testing.T) {
	history := []MomentumRow{
		{MC: 100, Volume: 10, Price: 1, Buys1H: 1},
		{MC: 110, Volume: 12, Price: 1.1, Buys1H: 2},
	}
	result := EvaluateMomentum(history, MomentumRow{MC: 110, Volume: 12, Price: 1.1, Buys1H: 2}, 3, 5)
	if result.Triggered || result.Reason != "no_state_change" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEvaluateMomentumAcceptsThreeUpRounds(t *testing.T) {
	history := []MomentumRow{
		{MC: 100, Volume: 10, Price: 1, Buys1H: 10},
		{MC: 110, Volume: 15, Price: 1.1, Buys1H: 12},
	}
	result := EvaluateMomentum(history, MomentumRow{MC: 120, Volume: 18, Price: 1.2, Buys1H: 14}, 3, 5)
	if !result.Triggered || result.PctGain <= 5 || !result.BuysOK {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEvaluateMomentumRejectsSmallGain(t *testing.T) {
	history := []MomentumRow{
		{MC: 100, Volume: 10, Price: 1, Buys1H: 10},
		{MC: 101, Volume: 11, Price: 1.01, Buys1H: 10},
	}
	result := EvaluateMomentum(history, MomentumRow{MC: 102, Volume: 12, Price: 1.02, Buys1H: 10}, 3, 5)
	if result.Triggered || result.Reason != "gain_too_small" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestClassifyNarrativeFindsBinanceOnBSC(t *testing.T) {
	category, matched := ClassifyNarrative("CZ Pancake", "BNB", "bsc")
	if category != "binance_cz" || len(matched) == 0 {
		t.Fatalf("unexpected narrative: %s %#v", category, matched)
	}
}

func TestHasPriorSignalDetectsExistingS5Discovery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SignalEvent{}); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	scanner := NewScanner(db)

	if scanner.hasPriorSignal("bsc", "0xabc", "narrative_tagged") {
		t.Fatalf("did not expect prior signal before insert")
	}
	db.Create(&model.SignalEvent{
		Source:     "s5",
		Chain:      "bsc",
		Address:    "0xabc",
		Symbol:     "ABC",
		SignalType: "narrative_tagged",
		Priority:   "medium",
		DedupeKey:  "existing",
		CreatedAt:  "2026-05-11T00:00:00Z",
	})

	if !scanner.hasPriorSignal("BSC", "0xABC", "narrative_tagged") {
		t.Fatalf("expected prior signal to be detected")
	}
	if scanner.hasPriorSignal("bsc", "0xabc", "flap_support") {
		t.Fatalf("did not expect different signal type to match")
	}
}
