package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const deletionMarker = ".missing-since"

// MarkDeleted records the first observed absence, including a removed folder's
// backed-up descendants. It does not invent a copy of already deleted bytes.
func (s *Store) MarkDeleted(abs string, ts time.Time) error {
	rel, err := s.RelPath(abs)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.ListFiles()
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.RelPath != rel && !strings.HasPrefix(f.RelPath, rel+"/") {
			continue
		}
		if _, err := os.Lstat(s.OriginalPath(f.RelPath)); !os.IsNotExist(err) {
			continue
		}
		if err := s.markMissing(f.RelPath, ts); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) markMissing(rel string, ts time.Time) error {
	p := filepath.Join(s.versionDir(rel), deletionMarker)
	if _, err := os.Lstat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.WriteString(ts.Format(time.RFC3339Nano))
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(p)
		return fmt.Errorf("registrando ausencia: %w", err)
	}
	return nil
}

// MissingSince is an observation, not proof of who deleted the file.
func (s *Store) MissingSince(rel string) time.Time {
	data, err := os.ReadFile(filepath.Join(s.versionDir(rel), deletionMarker))
	if err != nil {
		return time.Time{}
	}
	ts, _ := time.Parse(time.RFC3339Nano, string(data))
	return ts
}
