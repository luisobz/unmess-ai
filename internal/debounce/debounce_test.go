package debounce

import (
	"sort"
	"testing"
	"time"
)

var t0 = time.Date(2026, 7, 11, 12, 0, 0, 0, time.Local)

const delay = 10 * time.Second

func sortedKeys(k []string) []string {
	s := append([]string(nil), k...)
	sort.Strings(s)
	return s
}

func TestReadyOnlyAfterIdleDelay(t *testing.T) {
	d := New[string](delay)
	d.Add("a", t0)

	if r := d.Ready(t0.Add(delay - time.Nanosecond)); len(r) != 0 {
		t.Errorf("listo antes del delay: %v", r)
	}
	if r := d.Ready(t0.Add(delay)); len(r) != 1 || r[0] != "a" {
		t.Errorf("no listo al vencer el delay: %v", r)
	}
	// Ya consumido: no vuelve a aparecer.
	if r := d.Ready(t0.Add(2 * delay)); len(r) != 0 {
		t.Errorf("clave consumida reaparece: %v", r)
	}
}

func TestAddCoalescesAndDelaysDeadline(t *testing.T) {
	d := New[string](delay)
	d.Add("a", t0)
	// Un segundo Add dentro del delay reinicia el trailing.
	d.Add("a", t0.Add(delay/2))
	if d.Len() != 1 {
		t.Errorf("Add repetido no debe crear otra entrada: Len=%d", d.Len())
	}
	// El deadline original ya no vale.
	if r := d.Ready(t0.Add(delay)); len(r) != 0 {
		t.Errorf("listo con el deadline viejo tras coalescencia: %v", r)
	}
	// Debe estar listo delay después del ÚLTIMO Add.
	if r := d.Ready(t0.Add(delay/2 + delay)); len(r) != 1 {
		t.Errorf("no listo tras delay del último evento: %v", r)
	}
}

func TestCeilingUnderContinuousAdds(t *testing.T) {
	d := New[string](delay) // techo = 5*delay
	// Adds continuos cada delay/2 impiden que venza el idle, pero el techo
	// (5*delay desde el primer evento) fuerza la liberación.
	now := t0
	d.Add("a", now)
	for i := 0; i < 20; i++ {
		now = now.Add(delay / 2)
		d.Add("a", now)
		if !now.Before(t0.Add(5 * delay)) {
			break
		}
	}
	// Justo antes del techo: no listo (el idle nunca venció).
	if r := d.Ready(t0.Add(5*delay - time.Second)); len(r) != 0 {
		t.Errorf("liberado antes del techo: %v", r)
	}
	// En el techo: listo pese a Adds continuos.
	if r := d.Ready(t0.Add(5 * delay)); len(r) != 1 {
		t.Errorf("el techo de latencia no forzó la liberación: %v", r)
	}
}

func TestNextDeadline(t *testing.T) {
	d := New[string](delay)
	if _, ok := d.NextDeadline(); ok {
		t.Errorf("sin pendientes NextDeadline debe ser ok=false")
	}
	d.Add("a", t0)
	d.Add("b", t0.Add(5*time.Second))
	dl, ok := d.NextDeadline()
	if !ok {
		t.Fatal("debería haber deadline")
	}
	// El más próximo es el de "a": t0+delay.
	if !dl.Equal(t0.Add(delay)) {
		t.Errorf("NextDeadline = %v, quiero %v", dl, t0.Add(delay))
	}
}

func TestFlushEmptiesAll(t *testing.T) {
	d := New[string](delay)
	d.Add("a", t0)
	d.Add("b", t0)
	d.Add("c", t0)
	keys := d.Flush()
	if got := sortedKeys(keys); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("Flush = %v, quiero [a b c]", got)
	}
	if d.Len() != 0 {
		t.Errorf("tras Flush Len=%d, quiero 0", d.Len())
	}
	// Sin pendientes, Ready no devuelve nada.
	if r := d.Ready(t0.Add(100 * delay)); len(r) != 0 {
		t.Errorf("tras Flush Ready no debe devolver nada: %v", r)
	}
}

func TestCustomCeiling(t *testing.T) {
	// Techo explícito menor que 5*delay.
	d := NewWithCeiling[string](delay, 2*delay)
	d.Add("a", t0)
	d.Add("a", t0.Add(delay-time.Second))
	// El techo (t0+2*delay) llega antes de que el idle del segundo Add venza.
	if r := d.Ready(t0.Add(2 * delay)); len(r) != 1 {
		t.Errorf("el techo explícito no se respetó: %v", r)
	}
}
