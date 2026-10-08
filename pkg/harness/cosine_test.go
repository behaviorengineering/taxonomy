package harness

import "testing"

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0, 0}
	if CosineSimilarity(a, b) < 0.99 {
		t.Fatal("expected ~1")
	}
	orth := []float64{0, 1, 0}
	if CosineSimilarity(a, orth) > 0.01 {
		t.Fatal("expected ~0")
	}
	if CosineSimilarity(nil, b) != 0 {
		t.Fatal("nil")
	}
}
