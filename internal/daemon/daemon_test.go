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

// TestDaemonFlushAndReload cubre las dos peticiones externas del bucle:
// RequestFlush (versionar ya lo pendiente, sin esperar el debounce) y
// RequestReload (aplicar una config nueva en caliente, sin reiniciar).
func TestDaemonFlushAndReload(t *testing.T) {
	home := t.TempDir()
	t.Setenv("UNMESSAI_HOME", home)
	prefix := filepath.Join(home, "UnmessaiBackups")

	vigilada := filepath.Join(home, "vigilada")
	extra := filepath.Join(home, "extra")
	for _, d := range []string{vigilada, extra} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Default()
	cfg.Prefix = "~/UnmessaiBackups"
	cfg.IncludedPaths = []string{"~/vigilada"}
	cfg.ExcludedPaths = nil
	cfg.GitignoreAware = false

	st := store.New(prefix, home)

	var rt *Runtime
	ready := make(chan struct{})
	opts := Options{
		Logger: log.New(io.Discard, "", 0),
		// Debounce enorme: solo RequestFlush (o el apagado) puede versionar.
		Debounce:          time.Hour,
		RetentionInterval: time.Hour,
		Hook: func(ctx context.Context, r *Runtime) error {
			rt = r
			close(ready)
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, cfg, opts) }()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("el daemon no llegó a estar listo")
	}

	// --- RequestFlush: con debounce de una hora, solo el flush versiona. ---
	writeFile(t, filepath.Join(vigilada, "uno.txt"), "v1")
	testutil.Eventually(t, 10*time.Second, 50*time.Millisecond, func() bool {
		// El watcher necesita un instante en entregar el evento: flush en cada
		// intento hasta que el fichero aparezca versionado.
		if _, err := rt.RequestFlush(context.Background()); err != nil {
			t.Fatalf("RequestFlush: %v", err)
		}
		_, ok := versionedContent(t, st, "vigilada/uno.txt")
		return ok
	}, "flush bajo demanda versiona vigilada/uno.txt")

	// --- RequestReload: añadir ~/extra a las rutas vigiladas. Antes de la
	// recarga sus eventos ni entran al watcher, así que verlo versionado
	// prueba que el pipeline nuevo está activo con la config nueva. ---
	ncfg := *cfg
	ncfg.IncludedPaths = []string{"~/vigilada", "~/extra"}
	rt.RequestReload(ncfg)

	testutil.Eventually(t, 10*time.Second, 100*time.Millisecond, func() bool {
		writeFile(t, filepath.Join(extra, "dos.txt"), "v2")
		if _, err := rt.RequestFlush(context.Background()); err != nil {
			t.Fatalf("RequestFlush tras reload: %v", err)
		}
		_, ok := versionedContent(t, st, "extra/dos.txt")
		return ok
	}, "tras la recarga se versiona extra/dos.txt")

	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run devolvió error al apagar: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el daemon no se apagó")
	}
}
