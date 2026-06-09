package rag

import "testing"

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{1, 0}
	c := []float64{0, 1}

	if got := cosineSimilarity(a, b); got < 0.999 {
		t.Fatalf("expected near 1, got %f", got)
	}
	if got := cosineSimilarity(a, c); got > 0.001 {
		t.Fatalf("expected near 0, got %f", got)
	}
}
