package s2

import "testing"

func TestIsFundingFlip(t *testing.T) {
	previous := 0.001
	current := -0.0005
	if !IsFundingFlip(&previous, &current) {
		t.Fatal("expected funding flip")
	}
	previous = -0.001
	if IsFundingFlip(&previous, &current) {
		t.Fatal("did not expect negative-to-negative funding flip")
	}
}

func TestComputeOISegments(t *testing.T) {
	values := []float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 210}
	segments, oiChange, oiRising := ComputeOISegments(values)
	if len(segments) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(segments))
	}
	if !oiRising {
		t.Fatal("expected OI rising")
	}
	if oiChange <= 0 {
		t.Fatalf("expected positive OI change, got %f", oiChange)
	}
}
