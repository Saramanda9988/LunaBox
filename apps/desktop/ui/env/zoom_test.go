package env

import (
	"math"
	"testing"
)

func TestNormalizeZoom(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{
		{0, 1}, {-1, 1}, {math.NaN(), 1},
		{1, 1}, {1.25, 1.25}, {1.3, 1.25}, {0.5, 0.8}, {3, 1.5}, {1.04, 1},
	} {
		if got := NormalizeZoom(tc.in); got != tc.want {
			t.Errorf("NormalizeZoom(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestNextZoom(t *testing.T) {
	for _, tc := range []struct {
		cur  float64
		dir  int
		want float64
	}{
		{1, 1, 1.1}, {1, -1, 0.9}, {0, 1, 1.1}, {1.5, 1, 1.5}, {0.8, -1, 0.8}, {1.3, -1, 1.1},
	} {
		if got := NextZoom(tc.cur, tc.dir); got != tc.want {
			t.Errorf("NextZoom(%v, %d) = %v, want %v", tc.cur, tc.dir, got, tc.want)
		}
	}
}
