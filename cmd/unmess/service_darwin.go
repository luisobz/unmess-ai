//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const launchdLabel = "ai.unmess.unmessd"

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolviendo carpeta personal: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func serviceInstall() error {
	exe, err := unmessdPath()
	if err != nil {
		return err
	}
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("creando LaunchAgents: %w", err)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
`, launchdLabel, exe)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("escribiendo plist: %w", err)
	}
	if err := runLaunchctl("load", plistPath); err != nil {
		return err
	}
	fmt.Printf("servicio instalado: %s\n", plistPath)
	return nil
}

func serviceUninstall() error {
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}
	_ = runLaunchctl("unload", plistPath)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("borrando plist: %w", err)
	}
	fmt.Println("servicio desinstalado")
	return nil
}

func serviceStart() error {
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}
	return runLaunchctl("load", plistPath)
}

func serviceStop() error {
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}
	return runLaunchctl("unload", plistPath)
}

func serviceStatus() (string, error) {
	out, _ := exec.Command("launchctl", "list").Output()
	if len(out) > 0 {
		for _, line := range splitLines(string(out)) {
			if containsStr(line, launchdLabel) {
				return "estado: cargado (" + line + ")", nil
			}
		}
	}
	return "estado: no cargado", nil
}

func runLaunchctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl %v: %w", args, err)
	}
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func containsStr(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
