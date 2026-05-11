package s7

import "testing"

func TestComputeBlockWindow(t *testing.T) {
	from, to := ComputeBlockWindow(1000, nil, 40, 6)
	if from != 961 || to != 1000 {
		t.Fatalf("unexpected initial window: %d %d", from, to)
	}
	last := int64(990)
	from, to = ComputeBlockWindow(1000, &last, 40, 6)
	if from != 985 || to != 1000 {
		t.Fatalf("unexpected overlap window: %d %d", from, to)
	}
}

func TestClassifyRecipient(t *testing.T) {
	kind, name := ClassifyRecipient("0x7a250d5630b4cf539739df2c5dacb4c659f2488d")
	if kind != "dex" || name != "Uniswap V2 Router" {
		t.Fatalf("unexpected dex: %s %s", kind, name)
	}
	kind, name = ClassifyRecipient("0x28c6c06298d514db089934071355e5743bf21d60")
	if kind != "cex" || name != "Binance Hot Wallet" {
		t.Fatalf("unexpected cex: %s %s", kind, name)
	}
	kind, name = ClassifyRecipient("0x0000000000000000000000000000000000000001")
	if kind != "unknown" || name != "" {
		t.Fatalf("unexpected unknown: %s %s", kind, name)
	}
}

func TestDecodeRPCString(t *testing.T) {
	dynamic := "0x" +
		"0000000000000000000000000000000000000000000000000000000000000020" +
		"0000000000000000000000000000000000000000000000000000000000000003" +
		"4d4b520000000000000000000000000000000000000000000000000000000000"
	fixed := "0x" + "4d4b520000000000000000000000000000000000000000000000000000000000"
	if DecodeRPCString(dynamic) != "MKR" {
		t.Fatalf("dynamic decode failed: %q", DecodeRPCString(dynamic))
	}
	if DecodeRPCString(fixed) != "MKR" {
		t.Fatalf("fixed decode failed: %q", DecodeRPCString(fixed))
	}
}

func TestResolvePriority(t *testing.T) {
	large := 1_500_000.0
	small := 50_000.0
	if ResolvePriority("dex", &small) != "high" {
		t.Fatal("expected dex high")
	}
	if ResolvePriority("cex", &large) != "high" {
		t.Fatal("expected large cex high")
	}
	if ResolvePriority("cex", &small) != "medium" {
		t.Fatal("expected small cex medium")
	}
}
