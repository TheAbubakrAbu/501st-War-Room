package render_test

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/render"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/sample"
	"github.com/TheAbubakrAbu/501st-War-Room/internal/war"
)

func TestProject(t *testing.T) {
	x, y := render.Project(0, 0, 0.944)
	near(t, x, y, 600, 600)
	x, y = render.Project(0.944, 0.944, 0.944)
	near(t, x, y, 1108, 92) // y points up
	x, y = render.Project(-0.944, -0.944, 0.944)
	near(t, x, y, 92, 1108)
}

func maxAbs(s *war.Snapshot) float64 {
	m := 0.0
	for _, p := range s.Planets {
		m = math.Max(m, math.Max(math.Abs(p.X), math.Abs(p.Y)))
	}
	return m
}

func rgb(img image.Image, x, y float64) (r, g, b int) {
	c := img.At(int(math.Round(x)), int(math.Round(y)))
	R, G, B, _ := c.RGBA()
	return int(R >> 8), int(G >> 8), int(B >> 8)
}

func abs(n int) int { return max(n, -n) }

func TestMapFromFixtures(t *testing.T) {
	s, err := sample.Community(testNow)
	must(t, err)
	m := maxAbs(s)
	if math.Abs(m-0.944) > 0.001 {
		t.Fatalf("maxAbs %v", m)
	}
	start := time.Now()
	b, err := render.Map(s)
	took := time.Since(start)
	must(t, err)
	img, err := png.Decode(bytes.NewReader(b))
	must(t, err)
	if img.Bounds() != image.Rect(0, 0, render.Size, render.Size) {
		t.Fatalf("bounds %v", img.Bounds())
	}
	if len(b) >= 1<<20 {
		t.Fatalf("%d bytes", len(b))
	}
	if took >= time.Second && !raceEnabled {
		t.Fatalf("took %v", took)
	}
	t.Logf("%d bytes in %v", len(b), took)

	// The white center of Super Earth.
	r, g, bl := rgb(img, 600, 600)
	if r <= 200 || g <= 200 || bl <= 200 {
		t.Fatalf("Super Earth center is %d %d %d", r, g, bl)
	}
	// Oasis: Illuminate, no battle.
	oasis := s.Planet(94)
	x, y := render.Project(oasis.X, oasis.Y, m)
	r, g, bl = rgb(img, x, y)
	if abs(r-0xB0) > 40 || abs(g-0x7B) > 40 || abs(bl-0xFF) > 40 {
		t.Fatalf("Oasis is %d %d %d", r, g, bl)
	}
	// The bottom of Matar Bay's battle ring is Automaton red.
	mb := s.Planet(200)
	x, y = render.Project(mb.X, mb.Y, m)
	r, g, bl = rgb(img, x, y+13)
	if r <= g+60 || r <= bl+60 {
		t.Fatalf("Matar Bay ring is %d %d %d", r, g, bl)
	}

	if dir := os.Getenv("PREVIEW_DIR"); dir != "" {
		must(t, os.WriteFile(filepath.Join(dir, "warroom-preview.png"), b, 0o644))
		old, err := sample.OfficialWith(testNow, "official-assignment-2026-11-19.json")
		must(t, err)
		b2, err := render.Map(old)
		must(t, err)
		must(t, os.WriteFile(filepath.Join(dir, "warroom-preview-1119.png"), b2, 0o644))
	}
}

func TestMapIsDeterministic(t *testing.T) {
	s, err := sample.Community(testNow)
	must(t, err)
	a, err := render.Map(s)
	must(t, err)
	b, err := render.Map(s)
	must(t, err)
	if !bytes.Equal(a, b) {
		t.Fatal("two pictures of the same snapshot differ")
	}
}

func TestMapSmallSnapshot(t *testing.T) {
	s := war.NewSnapshot(testNow, "community", []war.Planet{
		{Index: 0, Name: "Super Earth", Owner: war.SuperEarth},
		{Index: 1, Name: "A", Owner: war.Terminids, X: 0.5, Y: 0.5, Waypoints: []int{0}},
		{Index: 2, Name: "B", Owner: war.Automatons, X: -0.5, Y: 0.2},
	}, nil)
	b, err := render.Map(s)
	must(t, err)
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

func TestMapEmptyAndCentered(t *testing.T) {
	// Only Super Earth: every position is 0, which must not divide by zero.
	s := war.NewSnapshot(testNow, "community", []war.Planet{{Index: 0, Name: "Super Earth", Owner: war.SuperEarth}}, nil)
	b, err := render.Map(s)
	must(t, err)
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}

// The palette fallback keeps the map's colors close enough to read.
func TestQuantizeKeepsColors(t *testing.T) {
	s, err := sample.Community(testNow)
	must(t, err)
	b, err := render.Map(s)
	must(t, err)
	img, err := png.Decode(bytes.NewReader(b))
	must(t, err)
	rgba := image.NewRGBA(img.Bounds())
	for y := range render.Size {
		for x := range render.Size {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	q := render.Quantize(rgba)
	if len(q.Palette) > 256 {
		t.Fatalf("%d colors", len(q.Palette))
	}
	oasis := s.Planet(94)
	x, y := render.Project(oasis.X, oasis.Y, maxAbs(s))
	r, g, bl := rgb(q, x, y)
	if abs(r-0xB0) > 40 || abs(g-0x7B) > 40 || abs(bl-0xFF) > 40 {
		t.Fatalf("Oasis is %d %d %d", r, g, bl)
	}
	r, g, bl = rgb(q, 600, 600)
	if r <= 200 || g <= 200 || bl <= 200 {
		t.Fatalf("Super Earth center is %d %d %d", r, g, bl)
	}
	var buf bytes.Buffer
	must(t, png.Encode(&buf, q))
	if buf.Len() >= 1<<20 {
		t.Fatalf("%d bytes", buf.Len())
	}
}

// A Major Order target Super Earth holds says so, instead of "0.0%".
func TestLabelLines(t *testing.T) {
	held := &war.Planet{Name: "Hellmire", Owner: war.SuperEarth, Health: 1000000, MaxHealth: 1000000, Players: 1234}
	name, stat := render.Label(held)
	if name != "HELLMIRE" || stat != "HELD · 1,234" {
		t.Fatalf("held: %q %q", name, stat)
	}
	enemy := &war.Planet{Name: "Zea Rugosia", Owner: war.Illuminate, Health: 850000, MaxHealth: 1700000, Players: 10}
	if _, stat := render.Label(enemy); stat != "50.0% · 10" {
		t.Fatalf("enemy: %q", stat)
	}
	defense := &war.Planet{Name: "Alderidge Cove", Owner: war.SuperEarth, Players: 5,
		Defense: &war.Defense{Enemy: war.Illuminate, Health: 275000, MaxHealth: 550000}}
	if _, stat := render.Label(defense); stat != "50.0% · 5" {
		t.Fatalf("defense: %q", stat)
	}
}
