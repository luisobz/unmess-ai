package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// collector drena Events en background y permite consultar si se vio una ruta.
type collector struct {
	mu     sync.Mutex
	events []Event
	done   chan struct{}
}

func newCollector(w Watcher) *collector {
	c := &collector{done: make(chan struct{})}
	go func() {
		for {
			select {
			case <-c.done:
				return
			case ev, ok := <-w.Events():
				if !ok {
					return
				}
				c.mu.Lock()
				c.events = append(c.events, ev)
				c.mu.Unlock()
			}
		}
	}()
	return c
}

func (c *collector) sawPath(path string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e.Path == path {
			return true
		}
	}
	return false
}

func (c *collector) stop() { close(c.done) }

// waitFor sondea cond hasta timeout con polling frecuente.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout esperando: %s", msg)
}

func TestWatcherDetectsWriteAndRetroactiveRecursion(t *testing.T) {
	root := t.TempDir()
	// Un fichero preexistente para modificar después.
	existing := filepath.Join(root, "a.txt")
	if err := os.WriteFile(existing, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(root); err != nil {
		t.Fatal(err)
	}
	c := newCollector(w)
	defer c.stop()

	// Modificar el fichero existente -> OpWrite.
	if err := os.WriteFile(existing, []byte("v2-modificado"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool { return c.sawPath(existing) }, "evento de escritura de a.txt")

	// Crear un subdirectorio NUEVO con un fichero dentro (recursión retroactiva).
	newDir := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	newFile := filepath.Join(newDir, "nuevo.txt")
	if err := os.WriteFile(newFile, []byte("contenido"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool { return c.sawPath(newFile) },
		"evento del fichero en subdirectorio nuevo (recursión retroactiva)")

	// Un fichero creado más tarde en ese subdir nuevo también debe verse
	// (el subdir quedó vigilado).
	laterFile := filepath.Join(newDir, "posterior.txt")
	if err := os.WriteFile(laterFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool { return c.sawPath(laterFile) },
		"evento de fichero creado en subdir ya vigilado")
}

func TestWatcherExcludeFunc(t *testing.T) {
	root := t.TempDir()
	excluded := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(excluded, 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := New(func(dir string) bool {
		return filepath.Base(dir) == "node_modules"
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(root); err != nil {
		t.Fatal(err)
	}
	c := newCollector(w)
	defer c.stop()

	// Fichero dentro del directorio excluido: no debe generar eventos.
	inExcluded := filepath.Join(excluded, "lib.js")
	if err := os.WriteFile(inExcluded, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Fichero fuera: sí debe verse (sirve de barrera temporal).
	included := filepath.Join(root, "app.js")
	if err := os.WriteFile(included, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool { return c.sawPath(included) }, "evento del fichero incluido")

	if c.sawPath(inExcluded) {
		t.Errorf("no debería haber evento del directorio excluido: %s", inExcluded)
	}
}
