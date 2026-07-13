package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luisobz/unmess-ai/internal/retention"
)

// makeVersion crea directamente un fichero de versión en el store para relPath
// con el timestamp ts, evitando depender del reloj real. content define tamaño.
func makeVersion(t *testing.T, s *Store, relPath string, ts time.Time, content string) {
	t.Helper()
	dir := s.versionDir(relPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := VersionName(ts, filepath.Ext(relPath))
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeOriginal crea el fichero original en baseDir (para OriginalExists=true).
func makeOriginal(t *testing.T, s *Store, relPath, content string) {
	t.Helper()
	p := s.OriginalPath(relPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func daysAgo(now time.Time, d int) time.Time { return now.AddDate(0, 0, -d) }

func versionCount(t *testing.T, s *Store, relPath string) int {
	t.Helper()
	v, err := s.ListVersions(relPath)
	if err != nil {
		t.Fatal(err)
	}
	return len(v)
}

func TestPruneMinKeepProtectsFromMaxVersions(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	// 5 versiones recientes.
	for i := 0; i < 5; i++ {
		makeVersion(t, s, "a.txt", now.Add(-time.Duration(i)*time.Minute), "x")
	}
	makeOriginal(t, s, "a.txt", "orig")

	cfg := retention.Config{MaxVersions: 2, MinKeep: 5}
	sum, err := s.Prune(cfg, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DeletedVersions != 0 {
		t.Errorf("borradas = %d, quiero 0 (min_keep protege)", sum.DeletedVersions)
	}
	if versionCount(t, s, "a.txt") != 5 {
		t.Errorf("quedaron %d versiones, quiero 5", versionCount(t, s, "a.txt"))
	}
}

func TestPruneMaxVersionsDeletesOldest(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	// 5 versiones a minutos decrecientes; la más antigua es la de índice mayor.
	stamps := []time.Time{}
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute)
		stamps = append(stamps, ts)
		makeVersion(t, s, "a.txt", ts, "x")
	}
	makeOriginal(t, s, "a.txt", "orig")

	cfg := retention.Config{MaxVersions: 3, MinKeep: 1}
	sum, err := s.Prune(cfg, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DeletedVersions != 2 {
		t.Errorf("borradas = %d, quiero 2", sum.DeletedVersions)
	}
	// Deben quedar las 3 más recientes.
	remaining, _ := s.ListVersions("a.txt")
	if len(remaining) != 3 {
		t.Fatalf("quedaron %d, quiero 3", len(remaining))
	}
	// Las dos más antiguas ya no existen.
	for _, ts := range stamps[3:] {
		name := VersionName(ts, ".txt")
		if _, err := os.Stat(filepath.Join(s.versionDir("a.txt"), name)); !os.IsNotExist(err) {
			t.Errorf("la versión antigua %q debería haberse borrado", name)
		}
	}
}

func TestPruneMaxAgeRespectsProtected(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	// Todas antiguas (100 días), pero min_keep protege las 2 más recientes.
	makeVersion(t, s, "a.txt", daysAgo(now, 100), "x") // más reciente de las viejas
	makeVersion(t, s, "a.txt", daysAgo(now, 101), "x")
	makeVersion(t, s, "a.txt", daysAgo(now, 102), "x")
	makeOriginal(t, s, "a.txt", "orig")

	cfg := retention.Config{MaxAgeDays: 90, MinKeep: 2}
	sum, err := s.Prune(cfg, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if sum.DeletedVersions != 1 {
		t.Errorf("borradas = %d, quiero 1 (2 protegidas por min_keep)", sum.DeletedVersions)
	}
	if versionCount(t, s, "a.txt") != 2 {
		t.Errorf("quedaron %d, quiero 2", versionCount(t, s, "a.txt"))
	}
}

func TestPrunePurgesOldDeletedEvenWithMinKeep(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	// Fichero borrado (sin original), versión más reciente hace 40 días.
	makeVersion(t, s, "sub/gone.txt", daysAgo(now, 40), "x")
	makeVersion(t, s, "sub/gone.txt", daysAgo(now, 45), "x")

	cfg := retention.Config{DeletedAgeDays: 30, MinKeep: 10}
	sum, err := s.Prune(cfg, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if sum.PurgedFiles != 1 {
		t.Errorf("purgados = %d, quiero 1", sum.PurgedFiles)
	}
	if sum.DeletedVersions != 2 {
		t.Errorf("borradas = %d, quiero 2 (todo el historial)", sum.DeletedVersions)
	}
	if _, err := os.Stat(s.versionDir("sub/gone.txt")); !os.IsNotExist(err) {
		t.Errorf("el directorio del fichero purgado debería haberse eliminado")
	}
}

func TestPruneDeletedNotOldEnoughNotPurged(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	// Borrado, pero la versión más reciente es de hace solo 10 días.
	makeVersion(t, s, "gone.txt", daysAgo(now, 10), "x")

	cfg := retention.Config{DeletedAgeDays: 30, MinKeep: 3}
	sum, err := s.Prune(cfg, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if sum.PurgedFiles != 0 {
		t.Errorf("purgados = %d, quiero 0 (no supera deleted_age_days)", sum.PurgedFiles)
	}
}

func TestPruneDryRunDoesNotDelete(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	for i := 0; i < 5; i++ {
		makeVersion(t, s, "a.txt", now.Add(-time.Duration(i)*time.Minute), "x")
	}
	makeOriginal(t, s, "a.txt", "orig")

	cfg := retention.Config{MaxVersions: 2, MinKeep: 1}
	sum, err := s.Prune(cfg, now, true) // dry-run
	if err != nil {
		t.Fatal(err)
	}
	if sum.DeletedVersions != 3 {
		t.Errorf("resumen borradas = %d, quiero 3 (contabilizadas)", sum.DeletedVersions)
	}
	// Pero nada se borró en disco.
	if versionCount(t, s, "a.txt") != 5 {
		t.Errorf("dry-run borró en disco: quedan %d, quiero 5", versionCount(t, s, "a.txt"))
	}
}

func TestPrunePruneEmptyParentsWithoutLeavingStore(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	// Único fichero bajo un subdirectorio profundo, borrado y antiguo -> purga.
	makeVersion(t, s, "deep/nested/gone.txt", daysAgo(now, 60), "x")

	cfg := retention.Config{DeletedAgeDays: 30, MinKeep: 3}
	if _, err := s.Prune(cfg, now, false); err != nil {
		t.Fatal(err)
	}
	// Los directorios padre vacíos se limpian...
	if _, err := os.Stat(filepath.Join(s.StoreDir(), "deep")); !os.IsNotExist(err) {
		t.Errorf("los directorios padre vacíos deberían haberse limpiado")
	}
	// ...pero el store raíz permanece.
	if _, err := os.Stat(s.StoreDir()); err != nil {
		t.Errorf("el store raíz no debe eliminarse: %v", err)
	}
}

func TestPrunePurgePriorityOverOtherRules(t *testing.T) {
	s, _ := newStore(t)
	now := fixedTS(t, "2026-07-11-12-00")
	// Borrado y antiguo: aunque min_keep/max_versions dirían otra cosa, se purga todo.
	for i := 0; i < 4; i++ {
		makeVersion(t, s, "gone.txt", daysAgo(now, 40+i), "x")
	}
	cfg := retention.Config{MaxVersions: 2, MaxAgeDays: 90, DeletedAgeDays: 30, MinKeep: 3}
	sum, err := s.Prune(cfg, now, false)
	if err != nil {
		t.Fatal(err)
	}
	if sum.PurgedFiles != 1 || sum.DeletedVersions != 4 {
		t.Errorf("purge = %d, borradas = %d; quiero 1 y 4", sum.PurgedFiles, sum.DeletedVersions)
	}
}
