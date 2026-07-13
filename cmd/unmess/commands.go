package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luisobz/unmess-ai/internal/journal"
	"github.com/luisobz/unmess-ai/internal/retention"
	"github.com/luisobz/unmess-ai/internal/textdiff"
)

func cmdStatus(configPath string, args []string) error {
	cfg, st, err := openStore(configPath)
	if err != nil {
		return err
	}
	files, err := st.ListFiles()
	if err != nil {
		return err
	}
	var totalVersions int
	for _, f := range files {
		totalVersions += len(f.Versions)
	}
	size, err := st.Size()
	if err != nil {
		return err
	}
	jLines, err := journal.Count(st.JournalPath())
	if err != nil {
		return err
	}
	journalExists := fileExists(st.JournalPath())

	fmt.Printf("prefix:           %s\n", st.Prefix())
	fmt.Printf("base:             %s\n", st.BaseDir())
	fmt.Printf("ficheros:         %d\n", len(files))
	fmt.Printf("versiones:        %d\n", totalVersions)
	fmt.Printf("tamaño store:     %s\n", humanBytes(size))
	fmt.Printf("journal:          %s (%d líneas)\n", boolYesNo(journalExists), jLines)
	fmt.Printf("debounce:         %ds\n", cfg.DebounceSeconds)
	return nil
}

func cmdLs(configPath string, args []string) error {
	var onlyModified, onlyDeleted bool
	var query string
	for _, a := range args {
		switch a {
		case "--modified":
			onlyModified = true
		case "--deleted":
			onlyDeleted = true
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("opción desconocida: %s", a)
			}
			query = a
		}
	}
	if onlyModified && onlyDeleted {
		return errors.New("--modified y --deleted son excluyentes")
	}

	_, st, err := openStore(configPath)
	if err != nil {
		return err
	}
	files, err := st.ListFiles()
	if err != nil {
		return err
	}
	for _, f := range files {
		if query != "" && !strings.Contains(f.RelPath, query) {
			continue
		}
		deleted := !fileExists(st.OriginalPath(f.RelPath))
		if onlyModified && deleted {
			continue
		}
		if onlyDeleted && !deleted {
			continue
		}
		marker := "modificado"
		if deleted {
			marker = "borrado"
		}
		fmt.Printf("%-11s %3d  %s\n", marker, len(f.Versions), f.RelPath)
	}
	return nil
}

func cmdVersions(configPath string, args []string) error {
	if len(args) < 1 {
		return errors.New("uso: unmess versions <ruta>")
	}
	_, st, err := openStore(configPath)
	if err != nil {
		return err
	}
	rel, err := resolveRel(st, args[0])
	if err != nil {
		return err
	}
	versions, err := st.ListVersions(rel)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("sin versiones para %q", rel)
	}
	for _, v := range versions {
		fmt.Printf("%s  %s  %s\n", v.Name, v.TS.Format(time.RFC3339), humanBytes(v.Size))
	}
	return nil
}

func cmdDiff(configPath string, args []string) error {
	var from, to, path string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				return errors.New("--from requiere un valor")
			}
			from = args[i+1]
			i++
		case "--to":
			if i+1 >= len(args) {
				return errors.New("--to requiere un valor")
			}
			to = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("opción desconocida: %s", args[i])
			}
			path = args[i]
		}
	}
	if path == "" {
		return errors.New("uso: unmess diff <ruta> [--from v] [--to v]")
	}

	_, st, err := openStore(configPath)
	if err != nil {
		return err
	}
	rel, err := resolveRel(st, path)
	if err != nil {
		return err
	}
	versions, err := st.ListVersions(rel)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("sin versiones para %q", rel)
	}

	// Por defecto: última versión (to) vs anterior (from). Con --to current y sin
	// --from, se compara la última versión contra el disco.
	toName := to
	fromName := from
	if toName == "" {
		toName = versions[0].Name
	}
	if fromName == "" {
		if toName == "current" || len(versions) < 2 {
			fromName = versions[0].Name
		} else {
			fromName = versions[1].Name
		}
	}

	fromData, err := st.VersionContent(rel, fromName)
	if err != nil {
		return err
	}
	var toData []byte
	var toLabel string
	if toName == "current" {
		toData, err = os.ReadFile(st.OriginalPath(rel))
		if err != nil {
			return fmt.Errorf("leyendo fichero actual: %w", err)
		}
		toLabel = "current"
	} else {
		toData, err = st.VersionContent(rel, toName)
		if err != nil {
			return err
		}
		toLabel = toName
	}

	out, err := textdiff.UnifiedString(rel+"@"+fromName, rel+"@"+toLabel, fromData, toData)
	if err != nil {
		return err
	}
	if out == "binary" {
		fmt.Println("binary")
		return nil
	}
	if out == "" {
		fmt.Println("(sin diferencias)")
		return nil
	}
	fmt.Print(out)
	return nil
}

func cmdRestore(configPath string, args []string) error {
	var version, path string
	var yes bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version":
			if i+1 >= len(args) {
				return errors.New("--version requiere un valor")
			}
			version = args[i+1]
			i++
		case "--yes", "-y":
			yes = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("opción desconocida: %s", args[i])
			}
			path = args[i]
		}
	}
	if path == "" {
		return errors.New("uso: unmess restore <ruta> [--version v] [--yes]")
	}

	_, st, err := openStore(configPath)
	if err != nil {
		return err
	}
	rel, err := resolveRel(st, path)
	if err != nil {
		return err
	}
	versions, err := st.ListVersions(rel)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("sin versiones para %q", rel)
	}
	if version == "" {
		version = versions[0].Name
	}

	if !yes {
		fmt.Printf("Restaurar %q a la versión %s.\n", rel, version)
		if fileExists(st.OriginalPath(rel)) {
			fmt.Println("Se guardará antes una copia de seguridad del estado actual.")
		}
		fmt.Print("¿Continuar? [s/N]: ")
		if !confirm() {
			fmt.Println("cancelado")
			return nil
		}
	}

	safety, err := st.Restore(rel, version, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("restaurado %q a %s\n", rel, version)
	if safety != "" {
		fmt.Printf("copia de seguridad: %s\n", safety)
	}
	return nil
}

func cmdPrune(configPath string, args []string) error {
	var dryRun bool
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("opción desconocida: %s", a)
		}
	}
	cfg, st, err := openStore(configPath)
	if err != nil {
		return err
	}
	sum, err := st.Prune(retention.Config{
		MaxVersions:    cfg.Retention.MaxVersions,
		MaxAgeDays:     cfg.Retention.MaxAgeDays,
		DeletedAgeDays: cfg.Retention.DeletedAgeDays,
		MinKeep:        cfg.Retention.MinKeep,
	}, time.Now(), dryRun)
	if err != nil {
		return err
	}
	prefix := ""
	if dryRun {
		prefix = "[dry-run] "
	}
	fmt.Printf("%sexaminados:        %d\n", prefix, sum.Examined)
	fmt.Printf("%sversiones borradas: %d\n", prefix, sum.DeletedVersions)
	fmt.Printf("%sficheros purgados:  %d\n", prefix, sum.PurgedFiles)
	fmt.Printf("%sbytes liberados:    %s\n", prefix, humanBytes(sum.FreedBytes))
	return nil
}

func cmdConfig(configPath string, args []string) error {
	if len(args) == 0 {
		// Imprime la configuración actual y su ruta.
		cfg, err := loadConfig(configPath)
		if err != nil {
			return err
		}
		fmt.Printf("# %s\n", cfg.Path())
		for _, k := range configKeys {
			v, _ := cfg.Get(k)
			fmt.Printf("%s = %s\n", k, v)
		}
		return nil
	}

	switch args[0] {
	case "path":
		cfg, err := loadConfig(configPath)
		if err != nil {
			return err
		}
		fmt.Println(cfg.Path())
		return nil
	case "get":
		if len(args) < 2 {
			return errors.New("uso: unmess config get <clave>")
		}
		cfg, err := loadConfig(configPath)
		if err != nil {
			return err
		}
		v, err := cfg.Get(args[1])
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	case "set":
		if len(args) < 3 {
			return errors.New("uso: unmess config set <clave> <valor>")
		}
		cfg, err := loadConfig(configPath)
		if err != nil {
			return err
		}
		if err := cfg.Set(args[1], args[2]); err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("%s = %s\n", args[1], args[2])
		return nil
	default:
		return fmt.Errorf("uso: unmess config [path | get <clave> | set <clave> <valor>]")
	}
}

// configKeys es el orden de claves mostrado por `unmess config`.
var configKeys = []string{
	"prefix",
	"debounce_seconds",
	"included_paths",
	"excluded_paths",
	"exclude_names",
	"gitignore_aware",
	"max_file_size_mb",
	"retention.max_versions",
	"retention.max_age_days",
	"retention.deleted_age_days",
	"retention.min_keep",
	"ui.port",
}

// --- helpers ---

func absPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolviendo ruta %q: %w", p, err)
	}
	return abs, nil
}

func normalizeRel(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}

func fileExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func confirm() bool {
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return false
	}
	resp := strings.ToLower(strings.TrimSpace(sc.Text()))
	return resp == "s" || resp == "si" || resp == "sí" || resp == "y" || resp == "yes"
}

func boolYesNo(b bool) string {
	if b {
		return "sí"
	}
	return "no"
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
