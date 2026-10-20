package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootedCopyDoesNotFollowReplacedDirectories(t *testing.T) {
	source := t.TempDir()
	destination := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "file"), []byte("inside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "file"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	src, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()
	if err := os.Mkdir(filepath.Join(destination, "dir"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(destination, "dir"), filepath.Join(destination, "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(destination, "dir")); err != nil {
		t.Skip(err)
	}
	if err := copyAtomicRoot(src, "file", dst, "dir/file"); err == nil {
		t.Fatal("copy followed an escaping symlink")
	}
	data, err := os.ReadFile(filepath.Join(outside, "file"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "private" {
		t.Fatal("external file was changed")
	}
}
