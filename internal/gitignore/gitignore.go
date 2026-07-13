// Package gitignore excluye ficheros ignorados por git. Agrupa las rutas por el
// repositorio git que las contiene y ejecuta `git check-ignore --stdin -z` por
// lote y repo. Si git no está instalado, se desactiva silenciosamente.
package gitignore

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// Filter decide qué rutas ignora git. Es seguro para un único consumidor
// (el flush del daemon); cachea la raíz de repo por directorio.
type Filter struct {
	enabled   bool
	available bool
	gitPath   string
	repoCache map[string]string // dir -> raíz de repo ("" = ninguna)
}

// New crea un Filter. Si enabled es false, o git no está en el PATH, Ignored
// devuelve siempre el conjunto vacío.
func New(enabled bool) *Filter {
	f := &Filter{enabled: enabled, repoCache: make(map[string]string)}
	if enabled {
		if p, err := exec.LookPath("git"); err == nil {
			f.available = true
			f.gitPath = p
		}
	}
	return f
}

// Enabled indica si el filtro está activo (habilitado y con git disponible).
func (f *Filter) Enabled() bool { return f.enabled && f.available }

// Ignored devuelve el subconjunto de paths (absolutos) que git ignora.
func (f *Filter) Ignored(paths []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if !f.Enabled() || len(paths) == 0 {
		return result, nil
	}

	byRepo := make(map[string][]string)
	for _, p := range paths {
		repo := f.repoRoot(filepath.Dir(p))
		if repo == "" {
			continue
		}
		byRepo[repo] = append(byRepo[repo], p)
	}

	for repo, group := range byRepo {
		ignored, err := f.checkIgnore(repo, group)
		if err != nil {
			return nil, err
		}
		for _, p := range ignored {
			result[p] = true
		}
	}
	return result, nil
}

// repoRoot busca hacia arriba desde dir el directorio que contiene .git;
// devuelve "" si no hay repo. Cachea por directorio.
func (f *Filter) repoRoot(dir string) string {
	if r, ok := f.repoCache[dir]; ok {
		return r
	}
	cur := dir
	var visited []string
	root := ""
	for {
		visited = append(visited, cur)
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			root = cur
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	for _, v := range visited {
		f.repoCache[v] = root
	}
	return root
}

// checkIgnore ejecuta `git check-ignore --stdin -z` en repo con las rutas de
// group por stdin (NUL-separadas) y devuelve las ignoradas.
func (f *Filter) checkIgnore(repo string, group []string) ([]string, error) {
	var stdin bytes.Buffer
	for _, p := range group {
		stdin.WriteString(p)
		stdin.WriteByte(0)
	}

	cmd := exec.Command(f.gitPath, "check-ignore", "--stdin", "-z")
	cmd.Dir = repo
	cmd.Stdin = &stdin
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		// Código de salida 1 = ninguna ruta ignorada: no es error.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, nil // ante cualquier fallo de git, no ignorar (fail-open).
	}

	var ignored []string
	for _, chunk := range bytes.Split(stdout.Bytes(), []byte{0}) {
		if len(chunk) == 0 {
			continue
		}
		ignored = append(ignored, string(chunk))
	}
	return ignored, nil
}
