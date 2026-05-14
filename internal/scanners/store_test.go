package scanners

import "testing"

func TestBetterTokenNameUpgradesSymbolOnlyName(t *testing.T) {
	if got := betterTokenName("STAR", "Star Project", "STAR"); got != "Star Project" {
		t.Fatalf("expected full name upgrade, got %q", got)
	}
}

func TestBetterTokenNameKeepsExistingFullName(t *testing.T) {
	if got := betterTokenName("Star Project", "STAR", "STAR"); got != "Star Project" {
		t.Fatalf("expected existing full name to survive symbol-only update, got %q", got)
	}
}
