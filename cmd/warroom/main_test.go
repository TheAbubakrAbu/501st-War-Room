package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

func TestSelfTestFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--self-test"}, noEnv, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}
	if !strings.HasSuffix(out.String(), "All checks passed\n") {
		t.Fatalf("stdout: %q", out.String())
	}
}

func TestPreviewFixtures(t *testing.T) {
	p := filepath.Join(t.TempDir(), "p.png")
	var out, errOut bytes.Buffer
	if code := run([]string{"--preview", p, "--fixtures"}, noEnv, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 1200 || b.Dy() != 1200 {
		t.Fatalf("bounds %v", b)
	}
	var embed struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(out.Bytes(), &embed); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out.String())
	}
	if embed.Title != "📡 Galactic War Briefing" {
		t.Fatalf("title %q", embed.Title)
	}
}

func TestMissingToken(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, noEnv, &out, &errOut); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "ERROR: DISCORD_TOKEN is not set") {
		t.Fatalf("stdout: %q", out.String())
	}
}

func TestBadFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"--nope"}, noEnv, &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
}
