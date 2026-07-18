package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luisobz/unmess-ai/internal/journal"
)

// fixedTS construye un instante local determinista.
func fixedTS(t *testing.T, s string) time.Time {
	t.Helper()
	if len(s) == len("2006-01-02-15-04") {
		s += "-00"
	}
	ts, err := time.ParseInLocation("2006-01-02-15-04-05", s, time.Local)
	if err != nil {
		t.Fatalf("parseando ts %q: %v", s, err)
	}
	return ts
}

// newStore crea un Store con prefix y baseDir en directorios temporales
// separados y devuelve también baseDir para escribir originales.
func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	prefix := t.TempDir()
	base := t.TempDir()
	return New(prefix, base), base
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteVersionLayoutWithExtension(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "sub", "informe.txt")
	writeFile(t, orig, "hola")
	ts := fixedTS(t, "2026-07-11-10-30")

	res, err := s.WriteVersion(orig, ts)
	if err != nil {
		t.Fatal(err)
	}
	if res.RelPath != "sub/informe.txt" {
		t.Errorf("RelPath = %q, quiero sub/informe.txt", res.RelPath)
	}
	if res.Name != "v2026-07-11-10-30-00.txt" {
		t.Errorf("Name = %q, quiero v2026-07-11-10-30-00.txt", res.Name)
	}
	want := filepath.Join(s.StoreDir(), "sub", "informe.txt", "v2026-07-11-10-30-00.txt")
	if res.Path != want {
		t.Errorf("Path = %q, quiero %q", res.Path, want)
	}
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("versión no escrita: %v", err)
	}
	if string(got) != "hola" {
		t.Errorf("contenido = %q, quiero hola", got)
	}
}

func TestWriteVersionNoExtension(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "Makefile")
	writeFile(t, orig, "all:")
	ts := fixedTS(t, "2026-07-11-10-30")

	res, err := s.WriteVersion(orig, ts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "v2026-07-11-10-30-00" {
		t.Errorf("Name = %q, quiero v2026-07-11-10-30-00 (sin extensión)", res.Name)
	}
}

func TestParseVersionTimeSupportsSecondAndLegacyMinuteFormats(t *testing.T) {
	for _, name := range []string{
		"v2026-07-11-10-30-45.txt",
		"v2026-07-11-10-30.txt", // store creado antes de añadir segundos
	} {
		ts, ok := ParseVersionTime(name)
		if !ok {
			t.Errorf("ParseVersionTime(%q) no reconoció una versión válida", name)
			continue
		}
		if ts.Year() != 2026 || ts.Minute() != 30 {
			t.Errorf("ParseVersionTime(%q) = %v", name, ts)
		}
	}
}

func TestWriteVersionSameMinuteKeepsBothVersions(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "a.txt")
	ts := fixedTS(t, "2026-07-11-10-30")

	writeFile(t, orig, "v1")
	if _, err := s.WriteVersion(orig, ts); err != nil {
		t.Fatal(err)
	}
	writeFile(t, orig, "v2-más-largo")
	if _, err := s.WriteVersion(orig, ts.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	versions, err := s.ListVersions("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("versiones = %d, quiero 2 (segundos distintos en el mismo minuto)", len(versions))
	}
	got, err := s.VersionContent("a.txt", versions[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2-más-largo" {
		t.Errorf("contenido = %q, quiero la última escritura", got)
	}
}

func TestCopyAtomicLeavesNoTemp(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "a.txt")
	writeFile(t, orig, "contenido")
	if _, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-10-30")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.StoreDir(), "a.txt")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("quedó un temporal: %q", e.Name())
		}
	}
	if len(ents) != 1 {
		t.Errorf("entradas = %d, quiero 1 (solo la versión)", len(ents))
	}
}

func TestRelPathOutsideBase(t *testing.T) {
	s, base := newStore(t)

	// Dentro de base: ok.
	rel, err := s.RelPath(filepath.Join(base, "x", "y.txt"))
	if err != nil {
		t.Fatalf("ruta interna rechazada: %v", err)
	}
	if rel != "x/y.txt" {
		t.Errorf("rel = %q, quiero x/y.txt", rel)
	}

	// Fuera de base: ErrOutsideBase.
	outside := filepath.Join(filepath.Dir(base), "otro", "z.txt")
	if _, err := s.RelPath(outside); err != ErrOutsideBase {
		t.Errorf("err = %v, quiero ErrOutsideBase", err)
	}
}

func TestListVersionsDescending(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "a.txt")
	stamps := []string{"2026-07-11-10-30", "2026-07-11-10-32", "2026-07-11-10-31"}
	for _, st := range stamps {
		writeFile(t, orig, "c-"+st)
		if _, err := s.WriteVersion(orig, fixedTS(t, st)); err != nil {
			t.Fatal(err)
		}
	}
	versions, err := s.ListVersions("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 {
		t.Fatalf("versiones = %d, quiero 3", len(versions))
	}
	for i := 1; i < len(versions); i++ {
		if versions[i-1].TS.Before(versions[i].TS) {
			t.Errorf("orden no descendente en %d: %v antes de %v", i, versions[i-1].TS, versions[i].TS)
		}
	}
	if versions[0].Name != "v2026-07-11-10-32-00.txt" {
		t.Errorf("primera = %q, quiero la más reciente", versions[0].Name)
	}
}

func TestListFilesDescendingAndFinds(t *testing.T) {
	s, base := newStore(t)
	writeFile(t, filepath.Join(base, "a.txt"), "a")
	writeFile(t, filepath.Join(base, "d", "b.txt"), "b")
	if _, err := s.WriteVersion(filepath.Join(base, "a.txt"), fixedTS(t, "2026-07-11-10-30")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteVersion(filepath.Join(base, "d", "b.txt"), fixedTS(t, "2026-07-11-10-31")); err != nil {
		t.Fatal(err)
	}
	files, err := s.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("ficheros = %d, quiero 2", len(files))
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.RelPath] = true
		if len(f.Versions) == 0 {
			t.Errorf("%q sin versiones", f.RelPath)
		}
		// Cada fichero: versiones en orden descendente.
		for i := 1; i < len(f.Versions); i++ {
			if f.Versions[i-1].TS.Before(f.Versions[i].TS) {
				t.Errorf("%q versiones no descendentes", f.RelPath)
			}
		}
	}
	if !seen["a.txt"] || !seen["d/b.txt"] {
		t.Errorf("faltan ficheros: %v", seen)
	}
}

func TestRestoreSafetyCopyWhenFileExists(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "a.txt")

	// Versión antigua "v1".
	writeFile(t, orig, "v1")
	old, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-10-30"))
	if err != nil {
		t.Fatal(err)
	}
	// El fichero en disco cambia a "actual" (distinto de la última versión).
	writeFile(t, orig, "actual-en-disco")

	safety, err := s.Restore("a.txt", old.Name, fixedTS(t, "2026-07-11-11-00"))
	if err != nil {
		t.Fatal(err)
	}
	if safety == "" {
		t.Fatal("no se creó copia de seguridad para fichero existente")
	}
	// La safety copy guarda el contenido que había en disco.
	sc, err := s.VersionContent("a.txt", safety)
	if err != nil {
		t.Fatal(err)
	}
	if string(sc) != "actual-en-disco" {
		t.Errorf("safety = %q, quiero actual-en-disco", sc)
	}
	// El fichero en disco ahora es la versión restaurada.
	got, err := os.ReadFile(orig)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v1" {
		t.Errorf("restaurado = %q, quiero v1", got)
	}
	// La versión original no fue tocada.
	orig1, err := s.VersionContent("a.txt", old.Name)
	if err != nil {
		t.Fatal(err)
	}
	if string(orig1) != "v1" {
		t.Errorf("versión original modificada: %q", orig1)
	}
}

func TestRestoreSafetyNeverOverwritesExistingVersionOnMinuteCollision(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "a.txt")

	// Versión existente en el minuto 11-00.
	writeFile(t, orig, "version-11-00")
	if _, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-11-00")); err != nil {
		t.Fatal(err)
	}
	// Otra versión más reciente para restaurar.
	writeFile(t, orig, "version-11-05")
	target, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-11-05"))
	if err != nil {
		t.Fatal(err)
	}
	// Disco tiene contenido distinto de cualquier versión.
	writeFile(t, orig, "contenido-nuevo-en-disco")

	// Restore con ts en el minuto 11-00 (colisiona con versión existente).
	safety, err := s.Restore("a.txt", target.Name, fixedTS(t, "2026-07-11-11-00"))
	if err != nil {
		t.Fatal(err)
	}
	// La safety NO puede ser el nombre de la versión existente del minuto 11-00.
	if safety == "v2026-07-11-11-00-00.txt" {
		t.Fatal("la copia de seguridad sobrescribió una versión existente")
	}
	// La versión 11-00 conserva su contenido.
	v1100, err := s.VersionContent("a.txt", "v2026-07-11-11-00-00.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(v1100) != "version-11-00" {
		t.Errorf("versión 11-00 alterada: %q", v1100)
	}
	// La safety guarda el contenido que había en disco, en un minuto libre.
	sc, err := s.VersionContent("a.txt", safety)
	if err != nil {
		t.Fatal(err)
	}
	if string(sc) != "contenido-nuevo-en-disco" {
		t.Errorf("safety = %q", sc)
	}
}

func TestRestoreSafetyReusesIdenticalLatest(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "a.txt")

	writeFile(t, orig, "v1")
	if _, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-10-30")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, orig, "latest")
	latest, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-10-40"))
	if err != nil {
		t.Fatal(err)
	}
	// El disco es idéntico a la última versión: no se debe escribir nada nuevo.
	before, err := s.ListVersions("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	safety, err := s.Restore("a.txt", "v2026-07-11-10-30-00.txt", fixedTS(t, "2026-07-11-11-00"))
	if err != nil {
		t.Fatal(err)
	}
	if safety != latest.Name {
		t.Errorf("safety = %q, quiero reutilizar la última versión %q", safety, latest.Name)
	}
	after, err := s.ListVersions("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("se escribió una versión nueva: antes %d, después %d", len(before), len(after))
	}
}

func TestRestoreDeletedFileNoSafety(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "a.txt")
	writeFile(t, orig, "contenido")
	v, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-10-30"))
	if err != nil {
		t.Fatal(err)
	}
	// Borrar el original.
	if err := os.Remove(orig); err != nil {
		t.Fatal(err)
	}
	safety, err := s.Restore("a.txt", v.Name, fixedTS(t, "2026-07-11-11-00"))
	if err != nil {
		t.Fatal(err)
	}
	if safety != "" {
		t.Errorf("safety = %q, quiero vacío para fichero borrado", safety)
	}
	got, err := os.ReadFile(orig)
	if err != nil {
		t.Fatalf("no se restauró: %v", err)
	}
	if string(got) != "contenido" {
		t.Errorf("restaurado = %q", got)
	}
}

func TestSize(t *testing.T) {
	s, base := newStore(t)
	if sz, err := s.Size(); err != nil || sz != 0 {
		t.Fatalf("Size inicial = %d, %v; quiero 0", sz, err)
	}
	orig := filepath.Join(base, "a.txt")
	writeFile(t, orig, "12345")
	if _, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-10-30")); err != nil {
		t.Fatal(err)
	}
	sz, err := s.Size()
	if err != nil {
		t.Fatal(err)
	}
	if sz != 5 {
		t.Errorf("Size = %d, quiero 5", sz)
	}
}

func TestJournalWrittenOnVersion(t *testing.T) {
	s, base := newStore(t)
	orig := filepath.Join(base, "sub", "a.txt")
	writeFile(t, orig, "x")
	if _, err := s.WriteVersion(orig, fixedTS(t, "2026-07-11-10-30")); err != nil {
		t.Fatal(err)
	}
	entries, err := journal.Read(s.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "sub/a.txt" {
		t.Errorf("journal = %+v, quiero una línea sub/a.txt", entries)
	}
}
