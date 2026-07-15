package daemon

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/store"
	"github.com/luisobz/unmess-ai/internal/testutil"
)

// TestProtect cubre la pasada de protección inicial: escribe versión baseline
// de los ficheros sin historial, no toca los ya versionados y respeta los
// mismos filtros que el pipeline (exclude_names, tamaño máximo, gitignore y el
// propio prefix del store).
func TestProtect(t *testing.T) {
	home := t.TempDir()
	t.Setenv("UNMESSAI_HOME", home)
	prefix := filepath.Join(home, "UnmessaiBackups")

	cfg := config.Default()
	cfg.Prefix = "~/UnmessaiBackups"
	cfg.ExcludedPaths = nil
	cfg.MaxFileSizeMB = 1
	cfg.GitignoreAware = true

	st := store.New(prefix, home)
	logger := log.New(io.Discard, "", 0)

	// Sin historial: debe recibir versión inicial.
	nuevo := filepath.Join(home, "docs", "nuevo.txt")
	writeFile(t, nuevo, "contenido original")

	// Con historial previo: no debe tocarse.
	viejo := filepath.Join(home, "viejo.txt")
	writeFile(t, viejo, "v1")
	if _, err := st.WriteVersion(viejo, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, viejo, "v2-en-disco-sin-versionar")

	// exclude_names: node_modules queda fuera.
	writeFile(t, filepath.Join(home, "node_modules", "lib.js"), "no")

	// Tamaño: por encima de max_file_size_mb queda fuera.
	grande := filepath.Join(home, "grande.bin")
	writeFile(t, grande, strings.Repeat("x", 2*1024*1024))

	// gitignored: solo comprobable si git está disponible.
	gitAvailable := testutil.HasGit()
	proj := filepath.Join(home, "proj")
	if gitAvailable {
		if err := os.MkdirAll(proj, 0o755); err != nil {
			t.Fatal(err)
		}
		testutil.InitGitRepo(t, proj)
		writeFile(t, filepath.Join(proj, ".gitignore"), "ignored.txt\n")
		writeFile(t, filepath.Join(proj, "ignored.txt"), "secreto")
		writeFile(t, filepath.Join(proj, "kept.txt"), "conservar")
	}

	sum, err := Protect(cfg, st, []string{home}, logger)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}

	// Con git: docs/nuevo.txt, proj/.gitignore y proj/kept.txt (.git queda
	// fuera por exclude_names). Sin git: solo docs/nuevo.txt.
	wantProtected := 1
	if gitAvailable {
		wantProtected = 3
	}
	if sum.Protected != wantProtected {
		t.Errorf("Protected = %d, esperado %d (resumen %+v)", sum.Protected, wantProtected, sum)
	}
	if sum.Existing != 1 {
		t.Errorf("Existing = %d, esperado 1", sum.Existing)
	}
	if sum.Failed != 0 {
		t.Errorf("Failed = %d, esperado 0", sum.Failed)
	}

	// La baseline conserva el contenido actual del fichero sin historial.
	if c, ok := versionedContent(t, st, "docs/nuevo.txt"); !ok || c != "contenido original" {
		t.Errorf("docs/nuevo.txt: baseline = (%q, %v), esperado el contenido original", c, ok)
	}
	// El fichero ya versionado mantiene una única versión (la antigua).
	versions, err := st.ListVersions("viejo.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Errorf("viejo.txt tiene %d versiones, esperado 1 (Protect no debe reversionar)", len(versions))
	}
	// Los filtrados no entran en el store.
	for _, rel := range []string{"node_modules/lib.js", "grande.bin", "proj/ignored.txt"} {
		if _, ok := versionedContent(t, st, rel); ok {
			t.Errorf("%s fue versionado pese a estar filtrado", rel)
		}
	}

	// Idempotencia: repetir la pasada no escribe nada nuevo.
	sum2, err := Protect(cfg, st, []string{home}, logger)
	if err != nil {
		t.Fatalf("Protect (2ª pasada): %v", err)
	}
	if sum2.Protected != 0 {
		t.Errorf("2ª pasada: Protected = %d, esperado 0", sum2.Protected)
	}
	if sum2.Existing != sum.Protected+sum.Existing {
		t.Errorf("2ª pasada: Existing = %d, esperado %d", sum2.Existing, sum.Protected+sum.Existing)
	}

	// Proteger un fichero suelto (no directorio) también funciona.
	suelto := filepath.Join(home, "suelto.txt")
	writeFile(t, suelto, "suelto")
	sum3, err := Protect(cfg, st, []string{suelto}, logger)
	if err != nil {
		t.Fatalf("Protect (fichero suelto): %v", err)
	}
	if sum3.Protected != 1 {
		t.Errorf("fichero suelto: Protected = %d, esperado 1", sum3.Protected)
	}

	// Ruta inexistente: error.
	if _, err := Protect(cfg, st, []string{filepath.Join(home, "no-existe")}, logger); err == nil {
		t.Errorf("Protect con ruta inexistente no devolvió error")
	}
}
