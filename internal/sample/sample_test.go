package sample_test

import (
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/sample"
)

var testNow = time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC)

func TestSamples(t *testing.T) {
	c, err := sample.Community(testNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Planets) != 273 || c.Source != "community" || !c.Fetched.Equal(testNow) {
		t.Fatalf("community: %d planets, %q, %v", len(c.Planets), c.Source, c.Fetched)
	}
	o, err := sample.Official(testNow)
	if err != nil {
		t.Fatal(err)
	}
	if o.Source != "official" || o.Orders[0].ID != 1278711574 {
		t.Fatalf("official: %q %d", o.Source, o.Orders[0].ID)
	}
	old, err := sample.OfficialWith(testNow, "official-assignment-2026-11-19.json")
	if err != nil {
		t.Fatal(err)
	}
	if old.Orders[0].ID != 2890701424 {
		t.Fatalf("2026-11-19 order: %d", old.Orders[0].ID)
	}
}
