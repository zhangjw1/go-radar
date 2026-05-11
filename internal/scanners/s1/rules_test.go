package s1

import (
	"strings"
	"testing"
)

func TestTriggerMatchesListingTitle(t *testing.T) {
	title := "Binance Will List Chip (CHIP) with Seed Tag Applied"
	if !IsTrigger(title) {
		t.Fatal("expected trigger")
	}
	if got := ExtractSymbol(title); got != "CHIP" {
		t.Fatalf("symbol mismatch: got %q", got)
	}
}

func TestRateProjectPrefersDarling(t *testing.T) {
	tier, reason := RateProject(10_000_000, 100_000_000, nil, "unknown", true)
	if tier != "S" {
		t.Fatalf("expected S tier, got %q", tier)
	}
	if !strings.Contains(strings.ToLower(reason), "darling") {
		t.Fatalf("expected darling reason, got %q", reason)
	}
}
