package textdiff

import (
	"strings"
	"testing"
)

func TestIsBinary(t *testing.T) {
	if IsBinary([]byte("texto normal\ncon líneas\n")) {
		t.Errorf("texto detectado como binario")
	}
	if !IsBinary([]byte("hola\x00mundo")) {
		t.Errorf("NUL no detectado como binario")
	}
	// NUL más allá de la ventana de sniff no cuenta.
	data := make([]byte, binarySniffLen+10)
	for i := range data {
		data[i] = 'a'
	}
	data[binarySniffLen+5] = 0
	if IsBinary(data) {
		t.Errorf("NUL fuera de la ventana no debe marcar binario")
	}
}

func TestUnifiedIdenticalIsEmpty(t *testing.T) {
	a := []byte("uno\ndos\ntres\n")
	diff, binary, err := Unified("a", "b", a, a)
	if err != nil {
		t.Fatal(err)
	}
	if binary {
		t.Errorf("no debería ser binario")
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("contenidos idénticos deben dar diff vacío, got:\n%s", diff)
	}
	// UnifiedString también da "".
	s, err := UnifiedString("a", "b", a, a)
	if err != nil {
		t.Fatal(err)
	}
	if s != "" {
		t.Errorf("UnifiedString idéntico = %q, quiero vacío", s)
	}
}

func TestUnifiedBinary(t *testing.T) {
	a := []byte("texto\n")
	b := []byte("bin\x00ario")
	diff, binary, err := Unified("a", "b", a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !binary {
		t.Errorf("un lado binario debería marcar binary=true")
	}
	if diff != "" {
		t.Errorf("diff debe estar vacío si es binario")
	}
	s, err := UnifiedString("a", "b", a, b)
	if err != nil {
		t.Fatal(err)
	}
	if s != "binary" {
		t.Errorf("UnifiedString binario = %q, quiero \"binary\"", s)
	}
}

func TestUnifiedContext3(t *testing.T) {
	// 8 líneas; cambia la línea 5. Con contexto 3, el hunk debe incluir las
	// líneas 2..8 (3 antes y 3 después del cambio) y no las líneas 1.
	a := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\n"
	b := "l1\nl2\nl3\nl4\nL5-CAMBIADA\nl6\nl7\nl8\n"
	diff, binary, err := Unified("viejo", "nuevo", []byte(a), []byte(b))
	if err != nil {
		t.Fatal(err)
	}
	if binary {
		t.Fatal("no es binario")
	}
	// Cabeceras del diff unificado.
	if !strings.Contains(diff, "--- viejo") || !strings.Contains(diff, "+++ nuevo") {
		t.Errorf("faltan cabeceras de fichero:\n%s", diff)
	}
	// La línea eliminada y la añadida.
	if !strings.Contains(diff, "-l5") || !strings.Contains(diff, "+L5-CAMBIADA") {
		t.Errorf("no aparece el cambio esperado:\n%s", diff)
	}
	// Contexto: l4 y l6 presentes (con prefijo espacio); l1 fuera del contexto.
	if !strings.Contains(diff, " l4") || !strings.Contains(diff, " l6") {
		t.Errorf("falta contexto adyacente:\n%s", diff)
	}
	if strings.Contains(diff, " l1\n") {
		t.Errorf("l1 no debería estar dentro del contexto (3 líneas):\n%s", diff)
	}
	// Marca de hunk.
	if !strings.Contains(diff, "@@") {
		t.Errorf("falta cabecera de hunk @@:\n%s", diff)
	}
}

func TestUnifiedStringChangeReturnsDiff(t *testing.T) {
	a := []byte("a\nb\n")
	b := []byte("a\nc\n")
	s, err := UnifiedString("x", "y", a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "-b") || !strings.Contains(s, "+c") {
		t.Errorf("UnifiedString no refleja el cambio:\n%s", s)
	}
}
