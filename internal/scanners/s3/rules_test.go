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
	types := BuildSignalTypes(50, 5.2, -0.05, 3)
	for _, want := range []string{"heat", "heat_plus_oi", "heat_plus_negative_funding"} {
		if !contains(types, want) {
			t.Fatalf("expected %s in %#v", want, types)
		}
	}
	types = BuildSignalTypes(0, 9, 0.01, 3)
	if len(types) != 1 || types[0] != "oi_anomaly" {
		t.Fatalf("expected pure oi anomaly, got %#v", types)
	}
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
