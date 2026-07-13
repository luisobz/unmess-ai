package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendCreatesDirsAndFormat(t *testing.T) {
	// Ruta con directorios intermedios inexistentes.
	path := filepath.Join(t.TempDir(), "var", "sub", "journal")
	w := NewWriter(path)
	ts := time.Date(2026, 7, 11, 10, 30, 12, 0, time.FixedZone("CEST", 2*3600))
	if err := w.Append(ts, "docs/informe.txt"); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("el journal no se creó: %v", err)
	}
	line := strings.TrimRight(string(raw), "\n")
	// Formato: "RFC3339\truta" con un tab.
	tab := strings.IndexByte(line, '\t')
	if tab < 0 {
		t.Fatalf("línea sin tab: %q", line)
	}
	if line[:tab] != "2026-07-11T10:30:12+02:00" {
		t.Errorf("timestamp = %q, quiero RFC3339 local", line[:tab])
	}
	if line[tab+1:] != "docs/informe.txt" {
		t.Errorf("ruta = %q", line[tab+1:])
	}
}

func TestAppendNormalizesSeparators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	w := NewWriter(path)
	// Ruta con separador de SO: se normaliza a "/".
	rel := filepath.Join("a", "b", "c.txt")
	if err := w.Append(time.Now(), rel); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "a/b/c.txt" {
		t.Errorf("entries = %+v, quiero a/b/c.txt", entries)
	}
}

func TestTailReturnsLastN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	w := NewWriter(path)
	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	for i := 0; i < 5; i++ {
		if err := w.Append(base.Add(time.Duration(i)*time.Minute), "f"+string(rune('0'+i))+".txt"); err != nil {
			t.Fatal(err)
		}
	}
	tail, err := Tail(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 2 {
		t.Fatalf("tail = %d, quiero 2", len(tail))
	}
	if tail[0].Path != "f3.txt" || tail[1].Path != "f4.txt" {
		t.Errorf("tail = %+v, quiero las 2 últimas en orden de llegada", tail)
	}
}

func TestTailMoreThanAvailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	w := NewWriter(path)
	_ = w.Append(time.Now(), "a.txt")
	tail, err := Tail(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 1 {
		t.Errorf("tail = %d, quiero 1", len(tail))
	}
}

func TestTailNonPositive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	_ = NewWriter(path).Append(time.Now(), "a.txt")
	if tail, err := Tail(path, 0); err != nil || tail != nil {
		t.Errorf("Tail(0) = %v, %v; quiero nil, nil", tail, err)
	}
}

func TestReadNonexistentIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-existe")
	entries, err := Read(path)
	if err != nil {
		t.Errorf("Read de fichero inexistente devolvió error: %v", err)
	}
	if entries != nil {
		t.Errorf("entries = %v, quiero nil", entries)
	}
	tail, err := Tail(path, 5)
	if err != nil || tail != nil {
		t.Errorf("Tail de fichero inexistente = %v, %v", tail, err)
	}
	if n, err := Count(path); err != nil || n != 0 {
		t.Errorf("Count = %d, %v; quiero 0", n, err)
	}
}

func TestReadSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal")
	content := "línea-sin-tab-ni-fecha\n" +
		"2026-07-11T10:30:12+02:00\tok.txt\n" +
		"\n" // línea vacía
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "ok.txt" {
		t.Errorf("entries = %+v, quiero solo la línea válida", entries)
	}
}
