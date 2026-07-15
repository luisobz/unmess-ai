package daemon

import (
	"sync"
	"sync/atomic"
	"time"
)

// EventType enumera los tipos de evento que el daemon publica para consumidores
// externos (por ejemplo la app nativa con bandeja y notificaciones del SO).
type EventType string

const (
	// EventVersioned: se guardó una nueva versión de un fichero.
	EventVersioned EventType = "versioned"
	// EventRestored: se restauró una versión (lo emite la capa de API tras el
	// restore, porque el restore vive en el servidor, no en el bucle del daemon).
	EventRestored EventType = "restored"
	// EventError: error relevante para el usuario (por ejemplo, falló versionar).
	EventError EventType = "error"
	// EventPaused: la vigilancia se pausó.
	EventPaused EventType = "paused"
	// EventResumed: la vigilancia se reanudó.
	EventResumed EventType = "resumed"
	// EventPruned: la poda liberó espacio.
	EventPruned EventType = "pruned"
	// EventProtected: una pasada de protección inicial escribió versiones de
	// ficheros existentes sin historial (lo emite la capa de API tras Protect).
	EventProtected EventType = "protected"
)

// Event es una notificación puntual del daemon. Se serializa tal cual sobre el
// stream SSE de /api/events.
type Event struct {
	Type    EventType `json:"type"`
	Path    string    `json:"path,omitempty"`
	Message string    `json:"message,omitempty"`
	Time    time.Time `json:"time"`
}

// Broker distribuye eventos del daemon a múltiples suscriptores. Publish nunca
// bloquea: si un suscriptor va lento, se descartan sus eventos en vez de frenar
// al daemon. Es seguro para uso concurrente.
type Broker struct {
	mu   sync.Mutex
	subs map[int]chan Event
	next int
}

// NewBroker crea un broker vacío.
func NewBroker() *Broker {
	return &Broker{subs: make(map[int]chan Event)}
}

// Subscribe registra un suscriptor y devuelve su canal de recepción junto a una
// función para darse de baja (idempotente). El canal tiene búfer; los eventos
// que no quepan mientras el suscriptor está ocupado se descartan.
func (b *Broker) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, 32)
	b.subs[id] = ch
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if c, ok := b.subs[id]; ok {
				delete(b.subs, id)
				close(c)
			}
		})
	}
}

// Publish entrega ev a todos los suscriptores sin bloquear. Rellena Time si viene
// a cero.
func (b *Broker) Publish(ev Event) {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default: // suscriptor lento: se descarta el evento
		}
	}
}

// pauseState envuelve el flag atómico de pausa de la vigilancia.
type pauseState struct {
	paused atomic.Bool
}

// IsPaused indica si la vigilancia está pausada.
func (rt *Runtime) IsPaused() bool { return rt.pause.paused.Load() }

// SetPaused cambia el estado de vigilancia y publica el evento correspondiente
// si el estado cambió. Devuelve true cuando hubo cambio real.
func (rt *Runtime) SetPaused(p bool) bool {
	changed := rt.pause.paused.Swap(p) != p
	if changed && rt.Events != nil {
		if p {
			rt.Events.Publish(Event{Type: EventPaused, Message: "Vigilancia pausada"})
		} else {
			rt.Events.Publish(Event{Type: EventResumed, Message: "Vigilancia reanudada"})
		}
	}
	return changed
}
