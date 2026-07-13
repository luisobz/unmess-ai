package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// cmdUI abre la UI de unmessai. Por defecto lanza la app nativa (ventana propia
// con bandeja del SO); si la app nativa no está instalada junto al CLI ni en el
// PATH, hace fallback a abrir la UI en el navegador. Con --browser fuerza el
// navegador. Si se pasa una ruta, abre directamente ese fichero.
func cmdUI(configPath string, args []string) error {
	forceBrowser := false
	rest := args[:0:0]
	for _, a := range args {
		if a == "--browser" || a == "-browser" {
			forceBrowser = true
			continue
		}
		rest = append(rest, a)
	}

	cfg, st, err := openStore(configPath)
	if err != nil {
		return err
	}

	var rel string
	if len(rest) > 0 && strings.TrimSpace(rest[0]) != "" {
		rel, err = resolveRel(st, rest[0])
		if err != nil {
			return err
		}
	}

	// Camino preferente: app nativa.
	if !forceBrowser {
		if appPath, ok := findNativeApp(); ok {
			appArgs := []string{}
			if configPath != "" {
				appArgs = append(appArgs, "--config", configPath)
			}
			if rel != "" {
				appArgs = append(appArgs, "--open", rel)
			}
			if err := exec.Command(appPath, appArgs...).Start(); err == nil {
				fmt.Println("Abriendo la app nativa de unmessai…")
				return nil
			}
			// Si no arrancó, seguimos al fallback de navegador.
		}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/", cfg.UI.Port)
	if rel != "" {
		url += "#/file/" + rel
	}
	fmt.Println(url)
	if err := openBrowser(url); err != nil {
		fmt.Printf("(no se pudo abrir el navegador automáticamente: %v)\n", err)
	}
	return nil
}

// findNativeApp localiza el binario de la app nativa (unmess-app) junto al CLI
// o en el PATH.
func findNativeApp() (string, bool) {
	name := "unmess-app"
	if runtime.GOOS == "windows" {
		name = "unmess-app.exe"
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), name)
		if fi, serr := os.Stat(cand); serr == nil && !fi.IsDir() {
			return cand, true
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, true
	}
	return "", false
}

// openBrowser intenta abrir url con el gestor por defecto del sistema. Usa
// exec.LookPath para encontrar el lanzador adecuado según el SO y hace fallback
// si el primero no está disponible.
func openBrowser(url string) error {
	var candidates [][]string
	switch runtime.GOOS {
	case "windows":
		candidates = [][]string{{"rundll32", "url.dll,FileProtocolHandler", url}}
	case "darwin":
		candidates = [][]string{{"open", url}}
	default: // linux y otros unix
		candidates = [][]string{
			{"xdg-open", url},
			{"gio", "open", url},
			{"sensible-browser", url},
			{"x-www-browser", url},
		}
	}

	var lastErr error
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			lastErr = err
			continue
		}
		if err := exec.Command(c[0], c[1:]...).Start(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no se encontró un lanzador de navegador")
	}
	return lastErr
}
