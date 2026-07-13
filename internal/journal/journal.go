// Package journal implementa el journal de actividad append-only: una línea
// "TIMESTAMP\truta/relativa" por cada escritura versionada.
// TIMESTAMP en RFC3339 con offset local.
package journal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// timeLayout es RFC3339 con offset local, p. ej. 2026-07-11T10:30:12+02:00.
const timeLayout = time.RFC3339

// Entry es una línea decodificada del journal.
type Entry struct {
	TS   time.Time
	Path string
}

// Writer añade entradas al journal de forma concurrente-segura dentro del
// proceso. Abre el fichero en modo O_APPEND en cada escritura (frecuencia baja
// gracias al debounce); no requiere fsync.
type Writer struct {
	path string
	mu   sync.Mutex
}

// NewWriter crea un Writer para el fichero en path (se crea al primer Append).
func NewWriter(path string) *Writer {
	return &Writer{path: path}
}

// Path devuelve la ruta del journal.
func (w *Writer) Path() string { return w.path }

// Append añade una línea "TIMESTAMP\trelPath". relPath se normaliza a
// separadores "/". ts se formatea con offset local.
func (w *Writer) Append(ts time.Time, relPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return fmt.Errorf("creando directorio del journal: %w", err)
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("abriendo journal: %w", err)
	}
	defer f.Close()
	line := ts.Format(timeLayout) + "\t" + filepath.ToSlash(relPath) + "\n"
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("escribiendo en journal: %w", err)
	}
	return nil
}

// Read devuelve todas las entradas del journal en orden de llegada. Si el
// fichero no existe devuelve nil sin error.
func Read(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("abriendo journal: %w", err)
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		e, ok := parseLine(line)
		if !ok {
			continue
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("leyendo journal: %w", err)
	}
	return entries, nil
}

// Tail devuelve las últimas n entradas (orden de llegada). n <= 0 devuelve nil.
func Tail(path string, n int) ([]Entry, error) {
	if n <= 0 {
		return nil, nil
	}
	all, err := Read(path)
	if err != nil {
		return nil, err
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

// Count devuelve el número de líneas válidas del journal.
func Count(path string) (int, error) {
	all, err := Read(path)
	if err != nil {
		return 0, err
	}
	return len(all), nil
}

func parseLine(line string) (Entry, bool) {
	tab := strings.IndexByte(line, '\t')
	if tab < 0 {
		return Entry{}, false
	}
	ts, err := time.Parse(timeLayout, line[:tab])
	if err != nil {
		return Entry{}, false
	}
	return Entry{TS: ts, Path: line[tab+1:]}, true
}
