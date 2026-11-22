package bot

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/poller"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/sample"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
	"github.com/bwmarrin/discordgo"
)

var testNow = time.Date(2026, 11, 21, 22, 30, 0, 0, time.UTC)

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func contains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q\ndoes not contain %q", s, sub)
	}
}

func prefix(t *testing.T, s, p string) {
	t.Helper()
	if !strings.HasPrefix(s, p) {
		t.Fatalf("%q\ndoes not start with %q", s, p)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// community is the 2026-11-21 community snapshot.
func community(t *testing.T) *war.Snapshot {
	t.Helper()
	s, err := sample.Community(testNow)
	must(t, err)
	return s
}

func state(t *testing.T) *poller.State {
	t.Helper()
	return &poller.State{Snap: community(t), Map: []byte("png")}
}

// withOrders is s with its orders replaced.
func withOrders(s *war.Snapshot, orders []war.Order) *war.Snapshot {
	return war.NewSnapshot(s.Fetched, s.Source, s.Planets, orders)
}

func field(t *testing.T, fields []*discordgo.MessageEmbedField, name string) string {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			return f.Value
		}
	}
	t.Fatalf("no field %q", name)
	return ""
}
