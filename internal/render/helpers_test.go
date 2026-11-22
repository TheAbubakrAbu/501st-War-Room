package render_test

import (
	"math"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC)

func near(t *testing.T, x, y, wantX, wantY float64) {
	t.Helper()
	if math.Abs(x-wantX) > 0.01 || math.Abs(y-wantY) > 0.01 {
		t.Fatalf("got (%v, %v), want (%v, %v)", x, y, wantX, wantY)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
