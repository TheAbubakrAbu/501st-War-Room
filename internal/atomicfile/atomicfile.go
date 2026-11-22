// Package atomicfile writes files so a crash or a full disk never leaves a
// half-written one behind: readers see the old file or the new one.
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write replaces path with data. It writes a temporary file in the same
// folder, syncs it, renames it over path and syncs the folder. The folder is
// created when missing.
func Write(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Chmod(0o644); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	if d, err := os.Open(dir); err == nil {
		d.Sync() // best effort: some file systems cannot sync folders
		d.Close()
	}
	return nil
}
