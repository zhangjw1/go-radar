package s3

import "testing"

func TestDetectVolumeSurge(t *testing.T) {
	surged, ratio := DetectVolumeSurge(300, []float64{100, 110, 90}, 2.5)
	if !surged || ratio <= 2.5 {
		t.Fatalf("expected surge over threshold, surged=%v ratio=%f", surged, ratio)
	}
	surged, ratio = DetectVolumeSurge(150, []float64{100, 110, 90}, 2.5)
	if surged || ratio >= 2.5 {
		t.Fatalf("expected no surge under threshold, surged=%v ratio=%f", surged, ratio)
	}
}

func TestBuildSignalTypes(t *testing.T) {
	types := BuildSignalTypes(50, 5.2, -0.05, true, 3)
	if len(types) != 1 || types[0] != "heat_plus_oi_negative_funding" {
		t.Fatalf("expected combined heat/oi/funding signal, got %#v", types)
	}
	types = BuildSignalTypes(50, 5.2, -0.05, false, 3)
	if len(types) != 1 || types[0] != "heat_plus_oi" {
		t.Fatalf("expected funding-missing signal to ignore negative funding, got %#v", types)
	}
	types = BuildSignalTypes(0, 9, 0.01, true, 3)
	if len(types) != 1 || types[0] != "oi_anomaly" {
		t.Fatalf("expected pure oi anomaly, got %#v", types)
	}
}
