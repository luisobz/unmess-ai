// Package testutil ofrece ayudantes compartidos por los tests de integración
// (polling con deadline y utilidades de git). No forma parte del binario de
// producción; solo lo importan ficheros *_test.go.
package testutil

import (
	"os/exec"
	"testing"
	"time"
)

// Eventually sondea cond cada interval hasta que devuelva true o se agote
// timeout. Falla el test (t.Fatalf) con msg si vence el plazo. Es la alternativa
// determinista a los sleeps fijos en los tests de watcher/daemon.
func Eventually(t *testing.T, timeout, interval time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condición no cumplida en %s: %s", timeout, msg)
		}
		time.Sleep(interval)
	}
}

// HasGit indica si el binario git está disponible en el PATH.
func HasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// RequireGit salta el test si git no está en el PATH.
func RequireGit(t *testing.T) {
	t.Helper()
	if !HasGit() {
		t.Skip("git no está en el PATH; se omite el test")
	}
}

// InitGitRepo inicializa un repositorio git en dir con identidad mínima para que
// las operaciones que la requieran no fallen. Salta el test si git no está.
func InitGitRepo(t *testing.T, dir string) {
	t.Helper()
	RequireGit(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
}
