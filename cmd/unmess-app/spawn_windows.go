//go:build gui && windows

package main

import (
	"os/exec"
	"syscall"
)

// hideSpawnedConsole evita que unmessd (binario de consola) abra una ventana de
// terminal al ser lanzado desde la app, que es de subsistema GUI y no tiene
// consola que heredarle.
func hideSpawnedConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
