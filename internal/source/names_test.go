package source_test

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/logx"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/source"
)

func TestNamesEmbedded(t *testing.T) {
	n := source.LoadNames("", logx.New(io.Discard, slog.LevelInfo))
	eq(t, n.Get(7), source.PlanetName{Name: "Zea Rugosia", Sector: "Ferris"})
	eq(t, n.Get(99).Name, "Sangis") // the names file has these two swapped
	eq(t, n.Get(273).Name, "Alderidge Cove")
	eq(t, n.Get(11).Name, "Cerberus IIIc")
	eq(t, n.Get(3).Name, "Widow's Harbor")
	eq(t, n.Get(107).Name, "Pöpli IX")
	eq(t, n.Get(0).Name, "Super Earth")
	eq(t, n.Get(263), source.PlanetName{Name: "Planet 263", Sector: ""})
}

func TestDisplayAndTitleCase(t *testing.T) {
	eq(t, source.Display("CERBERUS IIIc"), "Cerberus IIIc")
	eq(t, source.Display("WAYWARD"), "Wayward")
	eq(t, source.Display("NEW INSIGHT"), "New Insight")
	eq(t, source.Display("NEW PLANET IV"), "New Planet IV")
	eq(t, source.TitleCase("CHARBAL-VII"), "Charbal-VII")
	eq(t, source.TitleCase("WIDOW'S HARBOR"), "Widow's Harbor")
	eq(t, source.TitleCase("I"), "I")
	eq(t, source.TitleCase("MIX"), "Mix")
	eq(t, source.TitleCase("PÖPLI IX"), "Pöpli IX")
	eq(t, source.TitleCase("  TWO  SPACES "), "Two Spaces")
}

func TestNamesCache(t *testing.T) {
	p := filepath.Join(t.TempDir(), "planets.json")
	log := logx.New(io.Discard, slog.LevelInfo)
	n := source.LoadNames(p, log)
	n.Update(map[int]source.PlanetName{7: {Name: "Zea Rugosia Prime", Sector: "Ferris"}})
	eq(t, source.LoadNames(p, log).Get(7).Name, "Zea Rugosia Prime")

	// An Update with nothing new leaves the file alone.
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	must(t, os.Chtimes(p, old, old))
	n.Update(map[int]source.PlanetName{7: {Name: "Zea Rugosia Prime", Sector: "Ferris"}, 0: {Name: "Super Earth", Sector: "Sol"}})
	st, err := os.Stat(p)
	must(t, err)
	if !st.ModTime().Equal(old) {
		t.Fatalf("file rewritten: %v", st.ModTime())
	}

	// A broken cache file logs WARN and falls back to the embedded names.
	must(t, os.WriteFile(p, []byte("{"), 0o644))
	var buf bytes.Buffer
	b := source.LoadNames(p, logx.New(&buf, slog.LevelInfo))
	eq(t, b.Get(7).Name, "Zea Rugosia")
	contains(t, buf.String(), "WARN: ")
}

func TestNamesSaveFailureKeepsGoing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "planets.json")
	must(t, os.Mkdir(p, 0o755)) // a folder in the way: saving fails
	var buf bytes.Buffer
	n := source.LoadNames(p, logx.New(&buf, slog.LevelInfo))
	buf.Reset()
	n.Update(map[int]source.PlanetName{7: {Name: "New Name", Sector: "Ferris"}})
	eq(t, n.Get(7).Name, "New Name")
	contains(t, buf.String(), "WARN: ")
}

// The cache adds to the built-in names: a planet the cache lacks keeps its
// built-in name.
func TestNamesCacheMergesWithBuiltIn(t *testing.T) {
	p := filepath.Join(t.TempDir(), "planets.json")
	must(t, os.WriteFile(p, source.EncodeNames(map[int]source.PlanetName{7: {Name: "Zea Rugosia Prime", Sector: "Ferris"}}), 0o644))
	n := source.LoadNames(p, logx.New(io.Discard, slog.LevelInfo))
	eq(t, n.Get(7).Name, "Zea Rugosia Prime")
	eq(t, n.Get(0).Name, "Super Earth")
}
