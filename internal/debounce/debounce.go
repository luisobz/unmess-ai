// Package debounce implementa coalescencia trailing por clave: una clave se
// libera "delay" después de su último evento, con un techo de latencia "ceiling"
// desde su primer evento pendiente (garantiza liberación bajo actividad
// continua). Es un estado puro sin goroutines ni relojes propios: el llamante
// inyecta el instante en cada operación, lo que lo hace determinista y testeable.
package debounce

import "time"

type entry struct {
	first time.Time
	last  time.Time
}

// Debouncer coalesce eventos por clave de tipo K.
type Debouncer[K comparable] struct {
	delay   time.Duration
	ceiling time.Duration
	pending map[K]*entry
}

// New crea un Debouncer con el retardo trailing indicado. El techo de latencia
// es 5×delay.
func New[K comparable](delay time.Duration) *Debouncer[K] {
	return NewWithCeiling[K](delay, 5*delay)
}

// NewWithCeiling crea un Debouncer con retardo y techo explícitos.
func NewWithCeiling[K comparable](delay, ceiling time.Duration) *Debouncer[K] {
	return &Debouncer[K]{
		delay:   delay,
		ceiling: ceiling,
		pending: make(map[K]*entry),
	}
}

// Add registra un evento para key en el instante now.
func (d *Debouncer[K]) Add(key K, now time.Time) {
	if e, ok := d.pending[key]; ok {
		e.last = now
		return
	}
	d.pending[key] = &entry{first: now, last: now}
}

// deadline devuelve el instante en que una entrada debe liberarse.
func (d *Debouncer[K]) deadline(e *entry) time.Time {
	byIdle := e.last.Add(d.delay)
	byCeiling := e.first.Add(d.ceiling)
	if byCeiling.Before(byIdle) {
		return byCeiling
	}
	return byIdle
}

// Ready devuelve y elimina las claves cuyo plazo ha vencido en el instante now.
func (d *Debouncer[K]) Ready(now time.Time) []K {
	var ready []K
	for k, e := range d.pending {
		if !d.deadline(e).After(now) {
			ready = append(ready, k)
		}
	}
	for _, k := range ready {
		delete(d.pending, k)
	}
	return ready
}

// Flush devuelve y elimina todas las claves pendientes, sin mirar plazos (para
// el apagado limpio).
func (d *Debouncer[K]) Flush() []K {
	keys := make([]K, 0, len(d.pending))
	for k := range d.pending {
		keys = append(keys, k)
	}
	d.pending = make(map[K]*entry)
	return keys
}

// NextDeadline devuelve el instante más próximo en que alguna clave estará lista.
// ok es false si no hay claves pendientes.
func (d *Debouncer[K]) NextDeadline() (time.Time, bool) {
	var min time.Time
	found := false
	for _, e := range d.pending {
		dl := d.deadline(e)
		if !found || dl.Before(min) {
			min = dl
			found = true
		}
	}
	return min, found
}

// Len devuelve el número de claves pendientes.
func (d *Debouncer[K]) Len() int { return len(d.pending) }
