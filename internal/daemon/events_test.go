package daemon

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/store"
	"github.com/luisobz/unmess-ai/internal/testutil"
)

func TestBrokerPubSub(t *testing.T) {
	b := NewBroker()
	ch, cancel := b.Subscribe()

	b.Publish(Event{Type: EventVersioned, Path: "a"})
	select {
	case ev := <-ch:
		if ev.Type != EventVersioned || ev.Path != "a" {
			t.Fatalf("evento inesperado: %+v", ev)
		}
		if ev.Time.IsZero() {
			t.Errorf("Publish no rellenó Time")
		}
	case <-time.After(time.Second):
		t.Fatal("no se recibió el evento publicado")
	}

	cancel()
	cancel() // debe ser idempotente y no panicar
	// Publicar tras la baja no debe entregar ni panicar.
	b.Publish(Event{Type: EventResumed})
}

func TestBrokerDropsOnSlowSubscriber(t *testing.T) {
	b := NewBroker()
	_, cancel := b.Subscribe() // suscriptor que nunca lee
	defer cancel()

	done := make(chan struct{})
	go func() {
		// Muchos más eventos que el búfer del canal: no debe bloquear.
		for i := 0; i < 1000; i++ {
			b.Publish(Event{Type: EventVersioned})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish se bloqueó con un suscriptor lento")
	}
}

// waitEvent consume eventos del canal hasta encontrar el tipo buscado o agotar d.
func waitEvent(ch <-chan Event, typ EventType, d time.Duration) bool {
	deadline := time.After(d)
	for {
		select {
		case ev := <-ch:
			if ev.Type == typ {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func TestPauseGatesVersioning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("UNMESSAI_HOME", home)
	prefix := filepath.Join(home, "UnmessaiBackups")

	cfg := config.Default()
	cfg.Prefix = "~/UnmessaiBackups"
	cfg.IncludedPaths = []string{"~"}
	cfg.ExcludedPaths = nil
	cfg.GitignoreAware = false

	st := store.New(prefix, home)

	rtCh := make(chan *Runtime, 1)
	opts := Options{
		Logger:            log.New(io.Discard, "", 0),
		Debounce:          40 * time.Millisecond,
		RetentionInterval: time.Hour,
		Hook: func(ctx context.Context, rt *Runtime) error {
			rtCh <- rt
			return nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = Run(ctx, cfg, opts) }()

	var rt *Runtime
	select {
	case rt = <-rtCh:
	case <-time.After(5 * time.Second):
		t.Fatal("el daemon no llegó a estar listo")
	}

	events, unsub := rt.Events.Subscribe()
	defer unsub()

	notes := filepath.Join(home, "notes.txt")
	writeFile(t, notes, "v1")
	testutil.Eventually(t, 10*time.Second, 25*time.Millisecond, func() bool {
		c, ok := versionedContent(t, st, "notes.txt")
		return ok && c == "v1"
	}, "v1 versionado")
	if !waitEvent(events, EventVersioned, 2*time.Second) {
		t.Errorf("no llegó EventVersioned tras el primer versionado")
	}

	// --- Pausar ---
	if !rt.SetPaused(true) {
		t.Errorf("SetPaused(true) debería reportar cambio")
	}
	if !rt.IsPaused() {
		t.Errorf("IsPaused() debería ser true tras pausar")
	}
	if !waitEvent(events, EventPaused, 2*time.Second) {
		t.Errorf("no llegó EventPaused")
	}
	// Idempotencia: pausar de nuevo no reporta cambio.
	if rt.SetPaused(true) {
		t.Errorf("SetPaused(true) repetido no debería reportar cambio")
	}

	// Modificar estando pausado: NO debe versionarse.
	writeFile(t, notes, "v2-mientras-pausa")
	time.Sleep(500 * time.Millisecond) // muy por encima del debounce (40ms)
	if c, _ := versionedContent(t, st, "notes.txt"); c != "v1" {
		t.Errorf("se versionó estando pausado: última versión=%q, esperaba v1", c)
	}

	// --- Reanudar ---
	if !rt.SetPaused(false) {
		t.Errorf("SetPaused(false) debería reportar cambio")
	}
	if !waitEvent(events, EventResumed, 2*time.Second) {
		t.Errorf("no llegó EventResumed")
	}

	// Tras reanudar, un cambio sí debe versionarse.
	writeFile(t, notes, "v3-tras-reanudar")
	testutil.Eventually(t, 10*time.Second, 25*time.Millisecond, func() bool {
		c, ok := versionedContent(t, st, "notes.txt")
		return ok && c == "v3-tras-reanudar"
	}, "v3 versionado tras reanudar")
}
