package retention

import (
	"sort"
	"testing"
	"time"
)

var refNow = time.Date(2026, 7, 11, 12, 0, 0, 0, time.Local)

// mkVersions crea versiones con timestamps a "days" días de refNow. El primer
// elemento es el más antiguo; se pasan desordenados a Plan a propósito.
func v(name string, daysAgo int) Version {
	return Version{Name: name, TS: refNow.AddDate(0, 0, -daysAgo), Size: 1}
}

func deletedNames(d Decision) []string {
	names := make([]string, 0, len(d.DeleteVersions))
	for _, x := range d.DeleteVersions {
		names = append(names, x.Name)
	}
	sort.Strings(names)
	return names
}

func TestPlanNoVersions(t *testing.T) {
	d := Plan(FileInput{Path: "a", OriginalExists: true}, Config{MaxVersions: 1, MinKeep: 0}, refNow)
	if d.Purge || len(d.DeleteVersions) != 0 {
		t.Errorf("sin versiones debe ser decisión vacía: %+v", d)
	}
}

func TestPlanAllProtected(t *testing.T) {
	vs := []Version{v("v1", 100), v("v2", 200), v("v3", 300)}
	cfg := Config{MaxVersions: 1, MaxAgeDays: 10, MinKeep: 3}
	d := Plan(FileInput{Versions: vs, OriginalExists: true}, cfg, refNow)
	if d.Purge || len(d.DeleteVersions) != 0 {
		t.Errorf("todo protegido por min_keep: %+v", d)
	}
}

func TestPlanMaxVersionsDeletesOldest(t *testing.T) {
	// 4 versiones; recientes -> más antiguas.
	vs := []Version{v("d1", 1), v("d2", 2), v("d3", 3), v("d4", 4)}
	cfg := Config{MaxVersions: 2, MinKeep: 1}
	d := Plan(FileInput{Versions: vs, OriginalExists: true}, cfg, refNow)
	got := deletedNames(d)
	// Deben borrarse las 2 más antiguas (d3, d4); d1,d2 se conservan.
	if len(got) != 2 || got[0] != "d3" || got[1] != "d4" {
		t.Errorf("borradas = %v, quiero [d3 d4]", got)
	}
}

func TestPlanMaxAgeRespectsProtected(t *testing.T) {
	// Todas muy antiguas; min_keep=2 protege las 2 más recientes.
	vs := []Version{v("r1", 100), v("r2", 101), v("o1", 102), v("o2", 103)}
	cfg := Config{MaxAgeDays: 90, MinKeep: 2}
	d := Plan(FileInput{Versions: vs, OriginalExists: true}, cfg, refNow)
	got := deletedNames(d)
	if len(got) != 2 || got[0] != "o1" || got[1] != "o2" {
		t.Errorf("borradas = %v, quiero [o1 o2] (r1,r2 protegidas)", got)
	}
}

func TestPlanMaxAgeBoundaryNotOlder(t *testing.T) {
	// Exactamente en el corte (no "antes de"): no se borra.
	vs := []Version{v("recent", 0), v("edge", 90)}
	cfg := Config{MaxAgeDays: 90, MinKeep: 0}
	d := Plan(FileInput{Versions: vs, OriginalExists: true}, cfg, refNow)
	if len(d.DeleteVersions) != 0 {
		t.Errorf("borradas = %v, quiero 0 (el borde no es 'más antiguo que')", deletedNames(d))
	}
}

func TestPlanPurgeOnlyIfDeletedAndOldEnough(t *testing.T) {
	vs := []Version{v("v1", 40), v("v2", 45)}

	// Original existe: nunca se purga aunque sea antiguo.
	d := Plan(FileInput{Versions: vs, OriginalExists: true}, Config{DeletedAgeDays: 30, MinKeep: 0}, refNow)
	if d.Purge {
		t.Errorf("no debe purgar si el original existe")
	}

	// Borrado y la versión más reciente supera deleted_age_days: purga.
	d = Plan(FileInput{Versions: vs, OriginalExists: false}, Config{DeletedAgeDays: 30, MinKeep: 0}, refNow)
	if !d.Purge {
		t.Errorf("debe purgar: borrado y antiguo")
	}

	// Borrado pero la versión más reciente NO supera el umbral: no purga.
	recent := []Version{v("v1", 10), v("v2", 45)}
	d = Plan(FileInput{Versions: recent, OriginalExists: false}, Config{DeletedAgeDays: 30, MinKeep: 0}, refNow)
	if d.Purge {
		t.Errorf("no debe purgar: la versión más reciente es de hace 10 días")
	}
}

func TestPlanPurgePriorityOverMinKeep(t *testing.T) {
	vs := []Version{v("v1", 40), v("v2", 41), v("v3", 42)}
	cfg := Config{MaxVersions: 2, MaxAgeDays: 90, DeletedAgeDays: 30, MinKeep: 10}
	d := Plan(FileInput{Versions: vs, OriginalExists: false}, cfg, refNow)
	if !d.Purge {
		t.Errorf("la purga tiene prioridad incluso con min_keep alto")
	}
	if len(d.DeleteVersions) != 0 {
		t.Errorf("cuando Purge=true, DeleteVersions se ignora: %+v", d.DeleteVersions)
	}
}

func TestPlanZeroConfigDisablesEachRule(t *testing.T) {
	vs := []Version{v("v1", 1), v("v2", 400), v("v3", 800)}

	// Todo a cero: nada se borra ni purga (aunque haya versiones muy antiguas).
	d := Plan(FileInput{Versions: vs, OriginalExists: false}, Config{}, refNow)
	if d.Purge || len(d.DeleteVersions) != 0 {
		t.Errorf("config a cero desactiva todo: %+v", d)
	}

	// max_versions=0 no limita el número.
	d = Plan(FileInput{Versions: vs, OriginalExists: true}, Config{MaxVersions: 0, MinKeep: 0}, refNow)
	if len(d.DeleteVersions) != 0 {
		t.Errorf("max_versions=0 no debe borrar: %v", deletedNames(d))
	}

	// max_age_days=0 no aplica antigüedad.
	d = Plan(FileInput{Versions: vs, OriginalExists: true}, Config{MaxAgeDays: 0, MinKeep: 0}, refNow)
	if len(d.DeleteVersions) != 0 {
		t.Errorf("max_age_days=0 no debe borrar: %v", deletedNames(d))
	}

	// deleted_age_days=0 no purga.
	d = Plan(FileInput{Versions: vs, OriginalExists: false}, Config{DeletedAgeDays: 0, MinKeep: 0}, refNow)
	if d.Purge {
		t.Errorf("deleted_age_days=0 no debe purgar")
	}
}

func TestPlanCombinedMaxVersionsAndAge(t *testing.T) {
	// 5 versiones; 3 recientes, 2 antiguas. max_versions=4 borra 1 (la más
	// antigua), max_age_days=90 borra las 2 antiguas. Unión sin duplicar.
	vs := []Version{
		v("a", 1), v("b", 2), v("c", 3), // recientes
		v("d", 100), v("e", 200), // antiguas
	}
	cfg := Config{MaxVersions: 4, MaxAgeDays: 90, MinKeep: 1}
	d := Plan(FileInput{Versions: vs, OriginalExists: true}, cfg, refNow)
	got := deletedNames(d)
	if len(got) != 2 || got[0] != "d" || got[1] != "e" {
		t.Errorf("borradas = %v, quiero [d e]", got)
	}
}
