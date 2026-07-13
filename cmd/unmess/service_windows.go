//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

const scheduledTaskName = "unmessai"

func serviceInstall() error {
	exe, err := unmessdPath()
	if err != nil {
		return err
	}
	// Tarea programada "al iniciar sesión", sin privilegios de administrador.
	cmd := exec.Command("schtasks", "/create", "/sc", "onlogon",
		"/tn", scheduledTaskName, "/tr", exe, "/f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks /create: %w", err)
	}
	fmt.Printf("tarea programada creada: %s\n", scheduledTaskName)
	return nil
}

func serviceUninstall() error {
	cmd := exec.Command("schtasks", "/delete", "/tn", scheduledTaskName, "/f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks /delete: %w", err)
	}
	fmt.Println("tarea programada eliminada")
	return nil
}

func serviceStart() error {
	cmd := exec.Command("schtasks", "/run", "/tn", scheduledTaskName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks /run: %w", err)
	}
	return nil
}

func serviceStop() error {
	cmd := exec.Command("schtasks", "/end", "/tn", scheduledTaskName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks /end: %w", err)
	}
	return nil
}

func serviceStatus() (string, error) {
	out, err := exec.Command("schtasks", "/query", "/tn", scheduledTaskName).Output()
	if err != nil {
		return "estado: no instalado", nil
	}
	return "estado:\n" + string(out), nil
}
