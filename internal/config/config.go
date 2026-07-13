// Package config carga, valida y escribe la configuración común de unmessai
// (config.toml, esquema del ADR-0003). Es la única fuente de verdad para la
// ubicación del fichero, los valores por defecto, la expansión de "~" y el
// acceso get/set usado por `unmess config`.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Retention agrupa la política de poda (sección [retention] del TOML).
type Retention struct {
	MaxVersions    int `toml:"max_versions"`
	MaxAgeDays     int `toml:"max_age_days"`
	DeletedAgeDays int `toml:"deleted_age_days"`
	MinKeep        int `toml:"min_keep"`
}

// UI agrupa los ajustes del servidor local (sección [ui] del TOML).
type UI struct {
	Port int `toml:"port"`
}

// Config es el esquema completo definido en el ADR-0003. Las rutas se guardan
// tal cual se escribieron (pueden contener "~"); usa los métodos *Expanded para
// obtener rutas absolutas.
type Config struct {
	Prefix          string    `toml:"prefix"`
	DebounceSeconds int       `toml:"debounce_seconds"`
	IncludedPaths   []string  `toml:"included_paths"`
	ExcludedPaths   []string  `toml:"excluded_paths"`
	ExcludeNames    []string  `toml:"exclude_names"`
	IgnorePatterns  []string  `toml:"ignore_patterns"`
	GitignoreAware  bool      `toml:"gitignore_aware"`
	MaxFileSizeMB   int       `toml:"max_file_size_mb"`
	Retention       Retention `toml:"retention"`
	UI              UI        `toml:"ui"`

	// path es la ruta desde donde se cargó/guardó; no se serializa.
	path string `toml:"-"`
}

// Default devuelve la configuración por defecto del ADR-0003.
func Default() *Config {
	return &Config{
		Prefix:          "~/UnmessaiBackups",
		DebounceSeconds: 60,
		IncludedPaths:   []string{"~"},
		ExcludedPaths:   []string{"~/Downloads", "~/Descargas", "~/Videos", "~/Vídeos", "~/Music", "~/Música"},
		ExcludeNames:    []string{".git", "node_modules", "dist", "build", ".cache", ".venv", "target", "__pycache__"},
		IgnorePatterns:  []string{".config", "**/*.log", "**/cache/**"},
		GitignoreAware:  true,
		MaxFileSizeMB:   100,
		Retention: Retention{
			MaxVersions:    50,
			MaxAgeDays:     90,
			DeletedAgeDays: 30,
			MinKeep:        3,
		},
		UI: UI{Port: 48111},
	}
}

// HomeDir resuelve la carpeta personal del usuario. UNMESSAI_HOME tiene prioridad
// (usado en tests y para reubicar la base de rutas relativas); si no está,
// os.UserHomeDir().
func HomeDir() (string, error) {
	if h := os.Getenv("UNMESSAI_HOME"); h != "" {
		return h, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolviendo carpeta personal: %w", err)
	}
	return h, nil
}

// DefaultPath devuelve la ruta del fichero de configuración: UNMESSAI_CONFIG si
// está definido, o os.UserConfigDir()/unmessai/config.toml.
func DefaultPath() (string, error) {
	if p := os.Getenv("UNMESSAI_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolviendo directorio de configuración: %w", err)
	}
	return filepath.Join(dir, "unmessai", "config.toml"), nil
}

// LoadOrCreate carga la configuración desde path. Si path es "", usa DefaultPath.
// Si el fichero no existe lo crea con los valores por defecto y comentarios, y
// devuelve esos valores.
func LoadOrCreate(path string) (*Config, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("accediendo a config %q: %w", path, err)
		}
		cfg := Default()
		cfg.path = path
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return Load(path)
}

// Load lee y decodifica el fichero de configuración en path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo config %q: %w", path, err)
	}
	cfg := Default()
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parseando config %q: %w", path, err)
	}
	cfg.path = path
	return cfg, nil
}

// Path devuelve la ruta del fichero de configuración asociado.
func (c *Config) Path() string { return c.path }

// SetPath fija la ruta de destino para Save.
func (c *Config) SetPath(p string) { c.path = p }

// Save escribe la configuración en su path con comentarios legibles.
func (c *Config) Save() error {
	if c.path == "" {
		return fmt.Errorf("config sin ruta de destino")
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return fmt.Errorf("creando directorio de config: %w", err)
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(c.render()), 0o644); err != nil {
		return fmt.Errorf("escribiendo config temporal: %w", err)
	}
	if err := os.Rename(tmp, c.path); err != nil {
		return fmt.Errorf("renombrando config: %w", err)
	}
	return nil
}

// PrefixExpanded devuelve prefix con "~" expandido a ruta absoluta.
func (c *Config) PrefixExpanded() (string, error) { return ExpandUser(c.Prefix) }

// IncludedPathsExpanded devuelve included_paths con "~" expandido.
func (c *Config) IncludedPathsExpanded() ([]string, error) { return expandAll(c.IncludedPaths) }

// ExcludedPathsExpanded devuelve excluded_paths con "~" expandido.
func (c *Config) ExcludedPathsExpanded() ([]string, error) { return expandAll(c.ExcludedPaths) }

func expandAll(in []string) ([]string, error) {
	out := make([]string, len(in))
	for i, p := range in {
		e, err := ExpandUser(p)
		if err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

// ExpandUser sustituye un "~" inicial por la carpeta personal (HomeDir).
func ExpandUser(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~"+string(filepath.Separator)) {
		home, err := HomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// Validate comprueba invariantes mínimas del esquema.
func (c *Config) Validate() error {
	if c.DebounceSeconds < 1 {
		return fmt.Errorf("debounce_seconds debe ser >= 1")
	}
	if c.MaxFileSizeMB < 0 {
		return fmt.Errorf("max_file_size_mb no puede ser negativo")
	}
	if c.Retention.MinKeep < 0 {
		return fmt.Errorf("retention.min_keep no puede ser negativo")
	}
	if c.UI.Port < 1 || c.UI.Port > 65535 {
		return fmt.Errorf("ui.port fuera de rango")
	}
	return nil
}

// render produce el TOML con comentarios a partir de los valores actuales.
func (c *Config) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Configuración de unmessai (esquema ADR-0003).\n")
	fmt.Fprintf(&b, "# Las rutas admiten \"~\" como carpeta personal.\n\n")
	fmt.Fprintf(&b, "# Raíz del backup: contiene store/ y var/journal.\n")
	fmt.Fprintf(&b, "prefix = %s\n", tomlString(c.Prefix))
	fmt.Fprintf(&b, "# Segundos de coalescencia tras el último cambio de un fichero.\n")
	fmt.Fprintf(&b, "debounce_seconds = %d\n", c.DebounceSeconds)
	fmt.Fprintf(&b, "# Rutas vigiladas.\n")
	fmt.Fprintf(&b, "included_paths = %s\n", tomlStringArray(c.IncludedPaths))
	fmt.Fprintf(&b, "# Rutas excluidas de la vigilancia.\n")
	fmt.Fprintf(&b, "excluded_paths = %s\n", tomlStringArray(c.ExcludedPaths))
	fmt.Fprintf(&b, "# Nombres de directorio excluidos en cualquier nivel.\n")
	fmt.Fprintf(&b, "exclude_names = %s\n", tomlStringArray(c.ExcludeNames))
	fmt.Fprintf(&b, "# Patrones de ignorado (glob estilo gitignore, sin distinguir mayúsculas):\n")
	fmt.Fprintf(&b, "#   \".config\"      cualquier componente llamado .config y su subárbol\n")
	fmt.Fprintf(&b, "#   \"**/*.log\"     cualquier fichero .log en cualquier nivel\n")
	fmt.Fprintf(&b, "#   \"**/cache/**\"  cualquier directorio cache (o Cache) y su contenido\n")
	fmt.Fprintf(&b, "ignore_patterns = %s\n", tomlStringArray(c.IgnorePatterns))
	fmt.Fprintf(&b, "# Excluir lo ignorado por git en cada repo detectado.\n")
	fmt.Fprintf(&b, "gitignore_aware = %s\n", strconv.FormatBool(c.GitignoreAware))
	fmt.Fprintf(&b, "# No versionar ficheros mayores de este tamaño (MB).\n")
	fmt.Fprintf(&b, "max_file_size_mb = %d\n\n", c.MaxFileSizeMB)
	fmt.Fprintf(&b, "[retention]\n")
	fmt.Fprintf(&b, "# Máx. versiones por fichero.\n")
	fmt.Fprintf(&b, "max_versions = %d\n", c.Retention.MaxVersions)
	fmt.Fprintf(&b, "# Purgar versiones más antiguas que N días.\n")
	fmt.Fprintf(&b, "max_age_days = %d\n", c.Retention.MaxAgeDays)
	fmt.Fprintf(&b, "# Purgar historial completo de ficheros borrados hace > N días.\n")
	fmt.Fprintf(&b, "deleted_age_days = %d\n", c.Retention.DeletedAgeDays)
	fmt.Fprintf(&b, "# Proteger siempre las N versiones más recientes.\n")
	fmt.Fprintf(&b, "min_keep = %d\n\n", c.Retention.MinKeep)
	fmt.Fprintf(&b, "[ui]\n")
	fmt.Fprintf(&b, "port = %d\n", c.UI.Port)
	return b.String()
}

func tomlString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func tomlStringArray(in []string) string {
	parts := make([]string, len(in))
	for i, s := range in {
		parts[i] = tomlString(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
