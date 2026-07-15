package daemon

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/gitignore"
	"github.com/luisobz/unmess-ai/internal/store"
)

// ProtectSummary resume una pasada de protección inicial (ver Protect).
type ProtectSummary struct {
	Scanned   int // ficheros que pasaron todos los filtros
	Protected int // versiones iniciales escritas
	Existing  int // ya tenían historial en el store
	Failed    int // fallos al escribir la versión inicial
}

// Protect escribe una versión inicial ("baseline") de cada fichero bajo paths
// que aún no tiene historial en el store. El watcher solo puede versionar el
// contenido que hay en disco cuando llega el evento, así que un fichero que
// existía antes de instalar unmessai y nunca ha cambiado no tiene estado
// anterior que restaurar si algo lo borra o lo sobrescribe; esta pasada cierra
// ese hueco. Aplica los mismos filtros que el pipeline de vigilancia
// (excluded_paths, exclude_names, ignore_patterns, gitignore y tamaño máximo)
// y no toca los ficheros ya versionados, por lo que es idempotente y barata de
// repetir. paths admite ficheros o directorios en rutas absolutas.
func Protect(cfg *config.Config, st *store.Store, paths []string, logger *log.Logger) (ProtectSummary, error) {
	var sum ProtectSummary
	if logger == nil {
		logger = log.New(os.Stderr, "unmess ", log.LstdFlags)
	}
	prefix, err := cfg.PrefixExpanded()
	if err != nil {
		return sum, err
	}
	excluded, err := cfg.ExcludedPathsExpanded()
	if err != nil {
		return sum, err
	}
	flt := newFilter(cfg, prefix, excluded, st.BaseDir())
	gi := gitignore.New(cfg.GitignoreAware)

	var candidates []string
	for _, root := range paths {
		info, serr := os.Lstat(root)
		if serr != nil {
			return sum, fmt.Errorf("accediendo a %q: %w", root, serr)
		}
		if !info.IsDir() {
			if !flt.rejectCheap(root, st, logger) {
				candidates = append(candidates, root)
			}
			continue
		}
		werr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				// Directorio o entrada ilegible: se omite sin abortar la pasada.
				logger.Printf("proteger: aviso: %v", err)
				return nil
			}
			if d.IsDir() {
				if flt.excludeDir(p) {
					return filepath.SkipDir
				}
				return nil
			}
			if flt.rejectCheap(p, st, logger) {
				return nil
			}
			candidates = append(candidates, p)
			return nil
		})
		if werr != nil {
			return sum, fmt.Errorf("recorriendo %q: %w", root, werr)
		}
	}

	ignored, gerr := gi.Ignored(candidates)
	if gerr != nil {
		logger.Printf("proteger: gitignore: %v", gerr)
		ignored = nil
	}

	now := time.Now()
	for _, p := range candidates {
		if ignored[p] {
			continue
		}
		sum.Scanned++
		rel, rerr := st.RelPath(p)
		if rerr != nil {
			continue
		}
		has, herr := st.HasVersions(rel)
		if herr != nil {
			logger.Printf("proteger: consultando historial de %s: %v", rel, herr)
			sum.Failed++
			continue
		}
		if has {
			sum.Existing++
			continue
		}
		if _, werr := st.WriteVersion(p, now); werr != nil {
			logger.Printf("proteger: error versionando %s: %v", p, werr)
			sum.Failed++
			continue
		}
		sum.Protected++
	}
	if sum.Protected > 0 {
		logger.Printf("proteger: %d versión(es) inicial(es) escritas (%d ya con historial, %d fallos)",
			sum.Protected, sum.Existing, sum.Failed)
	}
	return sum, nil
}
