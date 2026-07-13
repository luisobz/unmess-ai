// Package retention implementa el algoritmo de poda del ADR-0003 como pieza pura
// (sin I/O): recibe, por fichero, la lista de versiones y si el original existe,
// y devuelve qué versiones borrar o si purgar el historial completo.
package retention

import "time"

// Config es la política de retención (sección [retention] del TOML).
type Config struct {
	MaxVersions    int
	MaxAgeDays     int
	DeletedAgeDays int
	MinKeep        int
}

// Version identifica una versión por su nombre, su timestamp (parseado del
// nombre) y su tamaño.
type Version struct {
	Name string
	TS   time.Time
	Size int64
}

// FileInput es la entrada por fichero al algoritmo.
type FileInput struct {
	Path           string
	Versions       []Version
	OriginalExists bool
}

// Decision es el resultado por fichero. Si Purge es true se elimina el historial
// completo (y DeleteVersions se ignora); en caso contrario se borran las
// versiones de DeleteVersions.
type Decision struct {
	DeleteVersions []Version
	Purge          bool
}

// Plan aplica el algoritmo del ADR-0003 sobre f y devuelve la decisión. now es
// el instante de referencia (inyectable para tests).
//
// Orden:
//  1. Nunca tocar las MinKeep versiones más recientes.
//  2. Borrar versiones que excedan MaxVersions (las más antiguas primero).
//  3. Borrar versiones con antigüedad > MaxAgeDays.
//  4. Si el original ya no existe y su versión más reciente tiene antigüedad
//     > DeletedAgeDays: purgar el historial completo.
func Plan(f FileInput, cfg Config, now time.Time) Decision {
	versions := make([]Version, len(f.Versions))
	copy(versions, f.Versions)
	sortDesc(versions)

	if len(versions) == 0 {
		return Decision{}
	}

	// Regla 4: purga completa de ficheros borrados y antiguos. Tiene prioridad:
	// si aplica, se elimina todo el historial (incluidas las protegidas).
	if cfg.DeletedAgeDays > 0 && !f.OriginalExists {
		cutoff := now.AddDate(0, 0, -cfg.DeletedAgeDays)
		if versions[0].TS.Before(cutoff) {
			return Decision{Purge: true}
		}
	}

	// Regla 1: conjunto protegido (las MinKeep más recientes).
	protected := cfg.MinKeep
	if protected < 0 {
		protected = 0
	}
	if protected > len(versions) {
		protected = len(versions)
	}
	isProtected := func(i int) bool { return i < protected }

	toDelete := make(map[int]struct{})

	// Regla 2: exceso sobre MaxVersions, las más antiguas primero (índices altos).
	if cfg.MaxVersions > 0 && len(versions) > cfg.MaxVersions {
		for i := cfg.MaxVersions; i < len(versions); i++ {
			if !isProtected(i) {
				toDelete[i] = struct{}{}
			}
		}
	}

	// Regla 3: antigüedad > MaxAgeDays.
	if cfg.MaxAgeDays > 0 {
		cutoff := now.AddDate(0, 0, -cfg.MaxAgeDays)
		for i, v := range versions {
			if isProtected(i) {
				continue
			}
			if v.TS.Before(cutoff) {
				toDelete[i] = struct{}{}
			}
		}
	}

	if len(toDelete) == 0 {
		return Decision{}
	}
	deleted := make([]Version, 0, len(toDelete))
	for i := range versions {
		if _, ok := toDelete[i]; ok {
			deleted = append(deleted, versions[i])
		}
	}
	return Decision{DeleteVersions: deleted}
}

func sortDesc(v []Version) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && lessDesc(v[j], v[j-1]); j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func lessDesc(a, b Version) bool {
	if a.TS.Equal(b.TS) {
		return a.Name > b.Name
	}
	return a.TS.After(b.TS)
}
