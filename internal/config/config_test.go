package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadOrCreateWritesDefaultsWithComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	cfg, err := LoadOrCreate(path, false)
	if err != nil {
		t.Fatal(err)
	}
	// Devuelve los valores por defecto.
	if !reflect.DeepEqual(cfg, withPath(Default(), path)) {
		t.Errorf("config creada != Default")
	}
	// El fichero existe y contiene comentarios.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no se creó el fichero: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "# ") {
		t.Errorf("el fichero creado no contiene comentarios")
	}
	for _, key := range []string{"prefix", "debounce_seconds", "[retention]", "[ui]"} {
		if !strings.Contains(s, key) {
			t.Errorf("falta %q en el fichero generado", key)
		}
	}
}

func TestLoadOrCreateExistingLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "prefix = \"/tmp/backups\"\ndebounce_seconds = 5\n[ui]\nport = 1234\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadOrCreate(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != "/tmp/backups" || cfg.DebounceSeconds != 5 || cfg.UI.Port != 1234 {
		t.Errorf("valores no cargados: %+v", cfg)
	}
	// Los campos ausentes conservan el valor por defecto.
	if cfg.MaxFileSizeMB != Default().MaxFileSizeMB {
		t.Errorf("MaxFileSizeMB = %d, quiero el default", cfg.MaxFileSizeMB)
	}
}

func TestExpandUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("UNMESSAI_HOME", home)

	cases := map[string]string{
		"~":          home,
		"~/x/y":      filepath.Join(home, "x", "y"),
		"/abs/path":  "/abs/path",
		"relativo/x": "relativo/x",
		"~sinbarra":  "~sinbarra", // no es "~" ni "~/": literal
	}
	for in, want := range cases {
		got, err := ExpandUser(in)
		if err != nil {
			t.Fatalf("ExpandUser(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ExpandUser(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestRoundtripLoadSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	orig := Default()
	orig.SetPath(path)
	orig.Prefix = "~/MisBackups"
	orig.DebounceSeconds = 30
	orig.ExcludeNames = []string{".git", "node_modules"}
	orig.Retention.MinKeep = 7
	if err := orig.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(orig, loaded) {
		t.Errorf("roundtrip inestable:\n orig=%+v\n load=%+v", orig, loaded)
	}
	// Y un segundo ciclo es idéntico byte a byte.
	loaded.SetPath(path + ".2")
	if err := loaded.Save(); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(path)
	b, _ := os.ReadFile(path + ".2")
	if string(a) != string(b) {
		t.Errorf("la serialización no es estable entre ciclos")
	}
}

func TestValidate(t *testing.T) {
	valid := Default()
	if err := valid.Validate(); err != nil {
		t.Errorf("los defaults deben ser válidos: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*Config)
	}{
		{"debounce cero", func(c *Config) { c.DebounceSeconds = 0 }},
		{"tamaño negativo", func(c *Config) { c.MaxFileSizeMB = -1 }},
		{"min_keep negativo", func(c *Config) { c.Retention.MinKeep = -1 }},
		{"puerto cero", func(c *Config) { c.UI.Port = 0 }},
		{"puerto alto", func(c *Config) { c.UI.Port = 70000 }},
	}
	for _, tc := range cases {
		c := Default()
		tc.mut(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: Validate debía fallar", tc.name)
		}
	}
}

func TestDefaultPathRespectsEnv(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "mi-config.toml")
	t.Setenv("UNMESSAI_CONFIG", custom)
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Errorf("DefaultPath = %q, quiero %q", got, custom)
	}
	// LoadOrCreate("") lo respeta y crea ahí.
	if _, err := LoadOrCreate("", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Errorf("no se creó config en UNMESSAI_CONFIG: %v", err)
	}
}

func TestGetSet(t *testing.T) {
	c := Default()
	if err := c.Set("retention.max_versions", "12"); err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get("retention.max_versions"); v != "12" {
		t.Errorf("Get retention.max_versions = %q, quiero 12", v)
	}
	if err := c.Set("exclude_names", "a, b ,c"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.ExcludeNames, []string{"a", "b", "c"}) {
		t.Errorf("splitList = %v", c.ExcludeNames)
	}
	// Valor inválido.
	if err := c.Set("ui.port", "no-numero"); err == nil {
		t.Errorf("Set con valor no numérico debía fallar")
	}
	// Clave desconocida.
	if _, err := c.Get("no.existe"); err == nil {
		t.Errorf("Get de clave desconocida debía fallar")
	}
}

// withPath devuelve una copia de cfg con la ruta fijada (para comparaciones).
func withPath(cfg *Config, p string) *Config {
	cfg.SetPath(p)
	return cfg
}
