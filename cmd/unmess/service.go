package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func cmdService(configPath string, args []string) error {
	_ = configPath
	if len(args) < 1 {
		return errors.New("uso: unmess service <install|uninstall|start|stop|status>")
	}
	switch args[0] {
	case "install":
		return serviceInstall()
	case "uninstall":
		return serviceUninstall()
	case "start":
		return serviceStart()
	case "stop":
		return serviceStop()
	case "status":
		out, err := serviceStatus()
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	default:
		return fmt.Errorf("acción de servicio desconocida: %q", args[0])
	}
}

// unmessdPath localiza el binario del daemon: junto al binario unmess actual, o
// en el PATH.
func unmessdPath() (string, error) {
	exeName := "unmessd"
	if runtime.GOOS == "windows" {
		exeName = "unmessd.exe"
	}
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), exeName)
		if _, serr := os.Stat(candidate); serr == nil {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath(exeName); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("no se encontró el binario %q (colócalo junto a unmess o en el PATH)", exeName)
}
