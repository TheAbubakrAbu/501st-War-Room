package war_test

import (
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC)

func eq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q does not contain %q", s, sub)
	}
}
