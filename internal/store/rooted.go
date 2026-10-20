package store

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReadOriginal opens relative to a directory handle so a concurrent symlink
// replacement cannot redirect the read outside the configured base.
func (s *Store) ReadOriginal(relPath string) ([]byte, error) {
	rel, err := cleanRelativePath(relPath)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.baseDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(filepath.FromSlash(rel))
}

func (s *Store) OriginalInfo(relPath string) (os.FileInfo, error) {
	rel, err := cleanRelativePath(relPath)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.baseDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Stat(filepath.FromSlash(rel))
}

func copyAtomicRoot(srcRoot *os.Root, src string, dstRoot *os.Root, dst string) (err error) {
	in, err := srcRoot.Open(src)
	if err != nil {
		return fmt.Errorf("abriendo origen: %w", err)
	}
	defer in.Close()
	if err = dstRoot.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmpName := filepath.Join(filepath.Dir(dst), ".tmp-"+rand.Text())
	tmp, err := dstRoot.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("creando temporal: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dstRoot.Remove(tmpName)
		}
	}()
	if _, err = io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return dstRoot.Rename(tmpName, dst)
}
