package s5

import "testing"

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
