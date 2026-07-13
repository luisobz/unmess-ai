// Package ignore implementa el matcher de ignore_patterns: patrones glob al
// estilo gitignore, siempre case-insensitive, aplicados sobre rutas relativas a
// la base vigilada (separador "/").
//
// Semántica:
//   - Un patrón sin "/" (p. ej. ".config" o "*.log") coincide con cualquier
//     componente de la ruta, en cualquier nivel.
//   - Un patrón con "/" se ancla a la base (p. ej. ".config/Code/**"); "**"
//     coincide con cero o más componentes ("**/cache/**" = cualquier directorio
//     "cache" en cualquier nivel y todo su contenido).
//   - Coincidir con un directorio excluye todo su subárbol.
//   - Mayúsculas/minúsculas se ignoran siempre ("**/cache/**" también excluye
//     "Cache").
package ignore

import (
	"path"
	"strings"
)

// Matcher evalúa un conjunto de patrones de ignorado compilados.
type Matcher struct {
	patterns [][]string
}

// New compila los patrones. Los vacíos se descartan.
func New(patterns []string) *Matcher {
	m := &Matcher{}
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		p = strings.Trim(p, "/")
		if p == "" {
			continue
		}
		segs := strings.Split(p, "/")
		if len(segs) == 1 {
			// Sin "/": coincide con cualquier componente en cualquier nivel.
			segs = []string{"**", segs[0]}
		}
		m.patterns = append(m.patterns, segs)
	}
	return m
}

// Empty indica si no hay ningún patrón compilado.
func (m *Matcher) Empty() bool { return len(m.patterns) == 0 }

// Match indica si relPath (relativa a la base, separador "/") coincide con
// algún patrón. Una coincidencia sobre un directorio intermedio también cuenta:
// el subárbol completo queda ignorado.
func (m *Matcher) Match(relPath string) bool {
	if len(m.patterns) == 0 {
		return false
	}
	rel := strings.ToLower(strings.Trim(strings.ReplaceAll(relPath, "\\", "/"), "/"))
	if rel == "" || rel == "." {
		return false
	}
	segs := strings.Split(rel, "/")
	for _, pat := range m.patterns {
		if matchPrefix(pat, segs) {
			return true
		}
	}
	return false
}

// matchPrefix indica si el patrón consume un prefijo (o la totalidad) de los
// componentes de la ruta; "**" cubre cero o más componentes.
func matchPrefix(pat, segs []string) bool {
	if len(pat) == 0 {
		return true
	}
	if pat[0] == "**" {
		if matchPrefix(pat[1:], segs) {
			return true
		}
		for i := range segs {
			if matchPrefix(pat[1:], segs[i+1:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	if err != nil || !ok {
		return false
	}
	return matchPrefix(pat[1:], segs[1:])
}
