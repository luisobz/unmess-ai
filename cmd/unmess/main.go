// Command unmess es la CLI de usuario de unmessai. En v1 opera directamente
// sobre el store (offline), usando los mismos paquetes internos que el daemon.
package main

import (
	"fmt"
	"os"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/store"
)

const usage = `unmess — CLI de unmessai

Uso:
  unmess status
  unmess ls [--modified|--deleted] [texto]
  unmess versions <ruta>
  unmess diff <ruta> [--from <versión>] [--to <versión|current>]
  unmess restore <ruta> [--version <versión>] [--yes]
  unmess prune [--dry-run]
  unmess config [path | get <clave> | set <clave> <valor>]
  unmess ui [ruta] [--browser]
  unmess service <install|uninstall|start|stop|status>

Opciones globales:
  --config <ruta>   ruta alternativa a config.toml
`

// devMode, inyectado vía -ldflags para builds locales; cuando es "true", el
// CLI usa la config y puerto de desarrollo.
var devMode = ""

func main() {
	args := os.Args[1:]
	// Extrae --config global (antes o después del subcomando).
	configPath, args := extractConfig(args)

	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	cmd := args[0]
	rest := args[1:]

	var err error
	switch cmd {
	case "status":
		err = cmdStatus(configPath, rest)
	case "ls":
		err = cmdLs(configPath, rest)
	case "versions":
		err = cmdVersions(configPath, rest)
	case "diff":
		err = cmdDiff(configPath, rest)
	case "restore":
		err = cmdRestore(configPath, rest)
	case "prune":
		err = cmdPrune(configPath, rest)
	case "config":
		err = cmdConfig(configPath, rest)
	case "ui":
		err = cmdUI(configPath, rest)
	case "service":
		err = cmdService(configPath, rest)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "subcomando desconocido: %q\n\n%s", cmd, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// extractConfig extrae un flag global --config/-config de args (soporta
// "--config X" y "--config=X") desde cualquier posición.
func extractConfig(args []string) (string, []string) {
	var path string
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--config" || a == "-config":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case len(a) > 9 && a[:9] == "--config=":
			path = a[9:]
		case len(a) > 8 && a[:8] == "-config=":
			path = a[8:]
		default:
			out = append(out, a)
		}
	}
	return path, out
}

// loadConfig carga (o crea) la configuración.
func loadConfig(configPath string) (*config.Config, error) {
	return config.LoadOrCreate(configPath, devMode == "true")
}

// openStore carga la config y construye el store con prefix y baseDir resueltos.
func openStore(configPath string) (*config.Config, *store.Store, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, nil, err
	}
	prefix, err := cfg.PrefixExpanded()
	if err != nil {
		return nil, nil, err
	}
	baseDir, err := config.HomeDir()
	if err != nil {
		return nil, nil, err
	}
	return cfg, store.New(prefix, baseDir), nil
}

// resolveRel convierte una ruta dada por el usuario (relativa al cwd, absoluta,
// o ya relativa al baseDir) en la ruta relativa canónica del store.
func resolveRel(st *store.Store, arg string) (string, error) {
	// ¿Es ya una ruta relativa válida del store con versiones?
	if _, err := os.Stat(st.OriginalPath(arg)); err == nil {
		if rel, rerr := st.RelPath(st.OriginalPath(arg)); rerr == nil {
			return rel, nil
		}
	}
	abs, err := absPath(arg)
	if err != nil {
		return "", err
	}
	rel, err := st.RelPath(abs)
	if err == nil {
		return rel, nil
	}
	// Último recurso: tratar el argumento como ruta relativa del store tal cual.
	return normalizeRel(arg), nil
}
