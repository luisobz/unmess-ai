package ignore

import "testing"

func TestMatcher(t *testing.T) {
	m := New([]string{".config", "**/*.log", "**/cache/**"})

	cases := []struct {
		rel  string
		want bool
	}{
		// .config: cualquier componente, subárbol completo.
		{".config", true},
		{".config/Code/settings.json", true},
		{"proyectos/.config/x", true},
		{".CONFIG/foo", true}, // case-insensitive
		{".configuracion/foo", false},

		// **/*.log: cualquier fichero .log en cualquier nivel.
		{"app.log", true},
		{"a/b/c/error.LOG", true},
		{"a/b/log.txt", false},

		// **/cache/**: cualquier directorio cache y su contenido.
		{"cache", true},
		{"cache/x", true},
		{"a/Cache/y/z", true},
		{"a/cachetada/y", false},
	}
	for _, c := range cases {
		if got := m.Match(c.rel); got != c.want {
			t.Errorf("Match(%q) = %v, quería %v", c.rel, got, c.want)
		}
	}
}

func TestMatcherAnchored(t *testing.T) {
	m := New([]string{".config/Code/**"})
	if !m.Match(".config/Code/x/y") {
		t.Errorf("patrón anclado no coincide con .config/Code/x/y")
	}
	if !m.Match(".config/code") {
		t.Errorf("el propio directorio anclado debería coincidir")
	}
	if m.Match("otro/.config/Code/x") {
		t.Errorf("patrón anclado no debería coincidir en subniveles")
	}
}

func TestMatcherEmptyAndInvalid(t *testing.T) {
	m := New([]string{"", "  ", "[bad"})
	if m.Match("cualquiera") {
		t.Errorf("patrones vacíos/incorrectos no deberían coincidir")
	}
	if !New(nil).Empty() {
		t.Errorf("Empty() debería ser true sin patrones")
	}
}
