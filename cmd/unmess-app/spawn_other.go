//go:build gui && !windows

package main

import "os/exec"

// hideSpawnedConsole solo tiene trabajo en Windows (subsistemas GUI/consola);
// en el resto de SO los procesos no abren ventanas de terminal por sí solos.
func hideSpawnedConsole(*exec.Cmd) {}
