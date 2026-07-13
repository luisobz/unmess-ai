// Package textdiff produce un diff unificado (contexto 3) entre dos contenidos,
// con detección de binario: si en los primeros 8000 bytes de cualquiera de los
// lados aparece un byte NUL, se considera binario y no se genera diff.
package textdiff

import (
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// binarySniffLen es la ventana inspeccionada para detectar binarios.
const binarySniffLen = 8000

// contextLines es el número de líneas de contexto del diff unificado.
const contextLines = 3

// IsBinary indica si data parece binario (NUL en los primeros 8000 bytes).
func IsBinary(data []byte) bool {
	n := len(data)
	if n > binarySniffLen {
		n = binarySniffLen
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// Unified devuelve el diff unificado entre a y b. Si cualquiera de los dos es
// binario, devuelve binary=true y diff vacío.
func Unified(aName, bName string, a, b []byte) (diff string, binary bool, err error) {
	if IsBinary(a) || IsBinary(b) {
		return "", true, nil
	}
	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(a)),
		B:        difflib.SplitLines(string(b)),
		FromFile: aName,
		ToFile:   bName,
		Context:  contextLines,
	}
	out, derr := difflib.GetUnifiedDiffString(ud)
	if derr != nil {
		return "", false, fmt.Errorf("generando diff unificado: %w", derr)
	}
	return out, false, nil
}

// UnifiedString es como Unified pero devuelve el marcador "binary" en lugar de
// un booleano cuando algún lado es binario. Útil para salidas de texto directas.
func UnifiedString(aName, bName string, a, b []byte) (string, error) {
	diff, binary, err := Unified(aName, bName, a, b)
	if err != nil {
		return "", err
	}
	if binary {
		return "binary", nil
	}
	if strings.TrimSpace(diff) == "" {
		return "", nil
	}
	return diff, nil
}
