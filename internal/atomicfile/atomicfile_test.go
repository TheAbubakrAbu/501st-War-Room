package atomicfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheAbubakrAbu/501st-War-Room/internal/atomicfile"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "x.json")
	if err := atomicfile.Write(p, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := atomicfile.Write(p, []byte("two")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "two" {
		t.Fatalf("read %q, %v", b, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 || entries[0].Name() != "x.json" {
		t.Fatalf("folder holds %v", entries)
	}
}

func TestAtomicWriteFailsCleanly(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")
	if err := os.Mkdir(p, 0o755); err != nil { // a folder where the file should go
		t.Fatal(err)
	}
	if err := atomicfile.Write(p, []byte("one")); err == nil {
		t.Fatal("want an error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}
