package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/luisobz/unmess-ai/internal/retention"
)

// PruneSummary resume el resultado de una poda.
type PruneSummary struct {
	Examined        int
	DeletedVersions int
	PurgedFiles     int
	FreedBytes      int64
}

// Prune aplica la política de retención a todo el store. Si dryRun es true no
// borra nada, solo contabiliza. now es el instante de referencia.
func (s *Store) Prune(cfg retention.Config, now time.Time, dryRun bool) (PruneSummary, error) {
	files, err := s.ListFiles()
	if err != nil {
		return PruneSummary{}, err
	}
	var sum PruneSummary
	for _, f := range files {
		sum.Examined++

		originalExists := false
		if _, serr := s.OriginalInfo(f.RelPath); serr == nil {
			originalExists = true
		} else if !os.IsNotExist(serr) {
			return sum, fmt.Errorf("comprobando original %q durante la poda: %w", f.RelPath, serr)
		}

		versions := make([]retention.Version, len(f.Versions))
		for i, v := range f.Versions {
			versions[i] = retention.Version{Name: v.Name, TS: v.TS, Size: v.Size}
		}

		decision := retention.Plan(retention.FileInput{
			Path:           f.RelPath,
			Versions:       versions,
			OriginalExists: originalExists,
		}, cfg, now)

		if decision.Purge {
			sum.PurgedFiles++
			for _, v := range f.Versions {
				sum.FreedBytes += v.Size
				sum.DeletedVersions++
			}
			if !dryRun {
				if rerr := s.removeHistory(f.RelPath); rerr != nil {
					return sum, fmt.Errorf("purgando %q: %w", f.RelPath, rerr)
				}
				s.pruneEmptyParents(f.RelPath)
			}
			continue
		}

		for _, dv := range decision.DeleteVersions {
			sum.DeletedVersions++
			sum.FreedBytes += dv.Size
			if !dryRun {
				path := filepath.Join(s.versionDir(f.RelPath), dv.Name)
				if rerr := s.removeVersion(f.RelPath, dv.Name); rerr != nil && !os.IsNotExist(rerr) {
					return sum, fmt.Errorf("borrando versión %q: %w", path, rerr)
				}
			}
		}
	}
	return sum, nil
}

// Forget elimina del store todo el historial de versiones de relPath (deja de
// estar trackeado). El fichero original en disco no se toca. Devuelve los bytes
// liberados.
func (s *Store) Forget(relPath string) (int64, error) {
	versions, err := s.ListVersions(relPath)
	if err != nil {
		return 0, err
	}
	var freed int64
	for _, v := range versions {
		freed += v.Size
	}
	if err := s.removeHistory(relPath); err != nil {
		return 0, fmt.Errorf("olvidando %q: %w", relPath, err)
	}
	s.pruneEmptyParents(relPath)
	return freed, nil
}

// pruneEmptyParents elimina directorios padre que hayan quedado vacíos tras una
// purga, sin salir nunca del store.
func (s *Store) pruneEmptyParents(relPath string) {
	root, err := os.OpenRoot(s.StoreDir())
	if err != nil {
		return
	}
	defer root.Close()
	rel, err := cleanRelativePath(relPath)
	if err != nil {
		return
	}
	for dir := filepath.Dir(rel); dir != "."; dir = filepath.Dir(dir) {
		directory, err := root.Open(dir)
		if err != nil {
			return
		}
		entries, err := directory.ReadDir(1)
		directory.Close()
		if len(entries) > 0 || (err != nil && err != io.EOF) {
			return
		}
		if err := root.Remove(dir); err != nil {
			return
		}
	}
}

func (s *Store) removeHistory(relPath string) error {
	rel, err := cleanRelativePath(relPath)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(s.StoreDir())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(filepath.FromSlash(rel))
}
func (s *Store) removeVersion(relPath, name string) error {
	rel, err := cleanRelativePath(relPath)
	if err != nil {
		return err
	}
	if !safeVersionName(name) {
		return ErrUnsafePath
	}
	root, err := os.OpenRoot(s.StoreDir())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Remove(filepath.Join(rel, name))
}
