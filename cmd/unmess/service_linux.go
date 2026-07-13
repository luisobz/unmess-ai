//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const systemdUnitName = "unmessai.service"

func systemdUnitPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolviendo directorio de configuración: %w", err)
	}
	return filepath.Join(dir, "systemd", "user", systemdUnitName), nil
}

func serviceInstall() error {
	exe, err := unmessdPath()
	if err != nil {
		return err
	}
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("creando directorio de unidad: %w", err)
	}
	unit := fmt.Sprintf(`[Unit]
Description=unmessai - versionado automático de ficheros
After=default.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure

[Install]
WantedBy=default.target
`, exe)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("escribiendo unidad systemd: %w", err)
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctl("enable", "--now", systemdUnitName); err != nil {
		return err
	}
	fmt.Printf("servicio instalado: %s\n", unitPath)
	return nil
}

func serviceUninstall() error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	_ = runSystemctl("disable", "--now", systemdUnitName)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("borrando unidad: %w", err)
	}
	_ = runSystemctl("daemon-reload")
	fmt.Println("servicio desinstalado")
	return nil
}

func serviceStart() error { return runSystemctl("start", systemdUnitName) }
func serviceStop() error  { return runSystemctl("stop", systemdUnitName) }

func serviceStatus() (string, error) {
	out, err := exec.Command("systemctl", "--user", "is-active", systemdUnitName).Output()
	state := trimOutput(out)
	if state == "" {
		state = "desconocido"
	}
	// is-active devuelve código != 0 si no está activo; no es un error de la CLI.
	_ = err
	return "estado: " + state, nil
}

func runSystemctl(args ...string) error {
	full := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", full...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %v: %w", args, err)
	}
	return nil
}

func trimOutput(b []byte) string {
	s := string(b)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
