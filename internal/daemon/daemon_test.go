package daemon

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/journal"
	"github.com/luisobz/unmess-ai/internal/store"
	"github.com/luisobz/unmess-ai/internal/testutil"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// versionedContent devuelve el contenido de la versión más reciente de relPath,
// o ("", false) si el fichero aún no está en el store.
func versionedContent(t *testing.T, st *store.Store, relPath string) (string, bool) {
	t.Helper()
	versions, err := st.ListVersions(relPath)
	if err != nil || len(versions) == 0 {
		return "", false
	}
	data, err := st.VersionContent(relPath, versions[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), true
}

func TestDaemonPipeline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("UNMESSAI_HOME", home)
	prefix := filepath.Join(home, "UnmessaiBackups")

	cfg := config.Default()
	cfg.Prefix = "~/UnmessaiBackups"
	cfg.IncludedPaths = []string{"~"}
	cfg.ExcludedPaths = nil // los defaults apuntan a ~/Downloads etc. inexistentes
	cfg.GitignoreAware = true

	st := store.New(prefix, home)

	ready := make(chan struct{})
	opts := Options{
		Logger:            log.New(io.Discard, "", 0),
		Debounce:          40 * time.Millisecond,
		RetentionInterval: time.Hour,
		Hook: func(ctx context.Context, rt *Runtime) error {
			close(ready)
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, cfg, opts) }()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("el daemon no llegó a estar listo (Hook)")
	}

	// --- Negativos: se crean primero para que tengan al menos tanto tiempo
	// como los positivos de haber sido procesados (y descartados). ---

	// exclude_names: node_modules está en la lista por defecto.
	excludedFile := filepath.Join(home, "node_modules", "lib.js")
	writeFile(t, excludedFile, "no versionar")

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
	}

	// --- Positivos ---
	if gitAvailable {
		writeFile(t, filepath.Join(proj, "kept.txt"), "conservar")
	}
	notes := filepath.Join(home, "notes.txt")
	writeFile(t, notes, "v1")

	// Esperar a que el fichero normal esté versionado.
	testutil.Eventually(t, 10*time.Second, 25*time.Millisecond, func() bool {
		_, ok := versionedContent(t, st, "notes.txt")
		return ok
	}, "notes.txt versionado")

	// Modificar el fichero y esperar a que el contenido nuevo se refleje.
	writeFile(t, notes, "v2-modificado")
	testutil.Eventually(t, 10*time.Second, 25*time.Millisecond, func() bool {
		c, ok := versionedContent(t, st, "notes.txt")
		return ok && c == "v2-modificado"
	}, "modificación de notes.txt versionada")

	if gitAvailable {
		testutil.Eventually(t, 10*time.Second, 25*time.Millisecond, func() bool {
			_, ok := versionedContent(t, st, "proj/kept.txt")
			return ok
		}, "proj/kept.txt versionado")
	}

	// El journal registró las escrituras.
	entries, err := journal.Read(st.JournalPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Errorf("el journal no tiene entradas")
	}

	// --- Verificación de negativos ---
	if _, ok := versionedContent(t, st, "node_modules/lib.js"); ok {
		t.Errorf("un fichero bajo exclude_names (node_modules) fue versionado")
	}
	if gitAvailable {
		if _, ok := versionedContent(t, st, "proj/ignored.txt"); ok {
			t.Errorf("un fichero gitignored fue versionado")
		}
	}

	// --- Apagado limpio con flush ---
	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run devolvió error al apagar: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el daemon no se apagó tras cancelar el contexto")
	}
}

func TestDaemonFlushOnShutdown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("UNMESSAI_HOME", home)
	prefix := filepath.Join(home, "UnmessaiBackups")

	cfg := config.Default()
	cfg.Prefix = "~/UnmessaiBackups"
	cfg.IncludedPaths = []string{"~"}
	cfg.ExcludedPaths = nil
	cfg.GitignoreAware = false

	st := store.New(prefix, home)

	ready := make(chan struct{})
	opts := Options{
		Logger: log.New(io.Discard, "", 0),
		// Debounce largo: el timer normal no disparará durante el test; solo el
		// flush del apagado puede versionar el fichero.
		Debounce:          10 * time.Second,
		RetentionInterval: time.Hour,
		Hook: func(ctx context.Context, rt *Runtime) error {
			close(ready)
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, cfg, opts) }()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("el daemon no llegó a estar listo")
	}

	f := filepath.Join(home, "pendiente.txt")
	writeFile(t, f, "contenido pendiente")

	// Esperar a que el evento entre en el debouncer (pendiente, no liberado).
	testutil.Eventually(t, 5*time.Second, 25*time.Millisecond, func() bool {
		// No hay API pública para inspeccionar el debouncer; damos margen a que
		// fsnotify entregue el evento. El propio Eventually acota el tiempo.
		return true
	}, "margen para el evento")
	time.Sleep(300 * time.Millisecond)

	// Cancelar antes de que venza el debounce (10s): el flush del apagado debe
	// versionar el fichero pendiente.
	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run devolvió error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el daemon no se apagó")
	}

	if c, ok := versionedContent(t, st, "pendiente.txt"); !ok || c != "contenido pendiente" {
		t.Errorf("el flush del apagado no versionó el fichero pendiente (ok=%v, c=%q)", ok, c)
	}
}
