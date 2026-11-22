package store_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/store"
)

func quiet() *slog.Logger { return logx.New(io.Discard, slog.LevelInfo) }

func open(t *testing.T, dir string) *store.Store {
	t.Helper()
	s, err := store.Open(dir, quiet())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	want := store.Server{Guild: "1", Channel: "2", Message: "3", Role: "4", Announced: []int64{9}, Seeded: true}
	if err := s.Put(want); err != nil {
		t.Fatal(err)
	}
	got, ok := open(t, dir).Get("1")
	if !ok || fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
		t.Fatalf("got %+v, %v", got, ok)
	}
	b, err := os.ReadFile(filepath.Join(dir, "servers", "1.json"))
	if err != nil || !strings.Contains(string(b), `"seeded": true`) {
		t.Fatalf("file: %s, %v", b, err)
	}
}

func TestGetReturnsACopy(t *testing.T) {
	s := open(t, t.TempDir())
	if err := s.Put(store.Server{Guild: "1", Announced: []int64{1}}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get("1")
	got.Announced[0] = 99
	again, _ := s.Get("1")
	if again.Announced[0] != 1 {
		t.Fatal("Get shares memory with the store")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	if err := s.Put(store.Server{Guild: "1", Channel: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("1"); ok {
		t.Fatal("still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "servers", "1.json")); !os.IsNotExist(err) {
		t.Fatalf("file still there: %v", err)
	}
	if err := s.Delete("77"); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptFileIsSetAside(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(dir, "servers"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "servers", "5.json"), []byte("{"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "servers", "6.json"), []byte(`{"guild":"6","channel":"7"}`), 0o644))
	var buf bytes.Buffer
	s, err := store.Open(dir, logx.New(&buf, slog.LevelInfo))
	must(err)
	if _, ok := s.Get("5"); ok {
		t.Fatal("broken file loaded")
	}
	if _, ok := s.Get("6"); !ok {
		t.Fatal("good file skipped")
	}
	if _, err := os.Stat(filepath.Join(dir, "servers", "5.json.bad")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ERROR: ") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestRejectsBadGuildIDs(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	for _, g := range []string{"", "../x", "12a", "1/2"} {
		if err := s.Put(store.Server{Guild: g}); err == nil {
			t.Fatalf("guild %q accepted", g)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "servers"))
	if len(entries) != 0 {
		t.Fatalf("files written: %v", entries)
	}
}

func TestAnnouncedKeepsNewest20(t *testing.T) {
	var s store.Server
	for i := int64(1); i <= 25; i++ {
		s.MarkAnnounced(i)
	}
	if len(s.Announced) != 20 || s.Announced[0] != 6 || s.Announced[19] != 25 {
		t.Fatalf("%v", s.Announced)
	}
	s.MarkAnnounced(25)
	if len(s.Announced) != 20 {
		t.Fatalf("duplicate added: %v", s.Announced)
	}
	if !s.WasAnnounced(25) || s.WasAnnounced(1) {
		t.Fatal("WasAnnounced")
	}
}

func TestAllSorted(t *testing.T) {
	s := open(t, t.TempDir())
	for _, g := range []string{"3", "1", "2"} {
		if err := s.Put(store.Server{Guild: g}); err != nil {
			t.Fatal(err)
		}
	}
	var gs []string
	for _, sv := range s.All() {
		gs = append(gs, sv.Guild)
	}
	if !slices.Equal(gs, []string{"1", "2", "3"}) {
		t.Fatalf("%v", gs)
	}
}

// When the disk refuses a write, the running bot still remembers the
// change, so it never repeats an announcement or a repost.
func TestPutKeepsMemoryWhenDiskFails(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	servers := filepath.Join(dir, "servers")
	if err := os.Chmod(servers, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(servers, 0o755)
	if err := s.Put(store.Server{Guild: "1", Message: "m2"}); err == nil {
		t.Skip("this file system ignores folder permissions")
	}
	got, ok := s.Get("1")
	if !ok || got.Message != "m2" {
		t.Fatalf("memory lost the change: %+v %v", got, ok)
	}
}
