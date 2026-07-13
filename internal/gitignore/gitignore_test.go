package gitignore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/luisobz/unmess-ai/internal/testutil"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIgnoredMarksGitignoredFiles(t *testing.T) {
	testutil.RequireGit(t)
	repo := t.TempDir()
	testutil.InitGitRepo(t, repo)
	write(t, filepath.Join(repo, ".gitignore"), "ignored.log\nbuild/\n")

	ignored := filepath.Join(repo, "ignored.log")
	kept := filepath.Join(repo, "kept.txt")
	inBuild := filepath.Join(repo, "build", "out.bin")
	write(t, ignored, "x")
	write(t, kept, "x")
	write(t, inBuild, "x")

	f := New(true)
	if !f.Enabled() {
		t.Skip("git no disponible pese a RequireGit")
	}
	res, err := f.Ignored([]string{ignored, kept, inBuild})
	if err != nil {
		t.Fatal(err)
	}
	if !res[ignored] {
		t.Errorf("ignored.log debería estar ignorado")
	}
	if !res[inBuild] {
		t.Errorf("build/out.bin debería estar ignorado")
	}
	if res[kept] {
		t.Errorf("kept.txt NO debería estar ignorado")
	}
}

func TestIgnoredMultiRepo(t *testing.T) {
	testutil.RequireGit(t)
	repoA := t.TempDir()
	repoB := t.TempDir()
	testutil.InitGitRepo(t, repoA)
	testutil.InitGitRepo(t, repoB)
	write(t, filepath.Join(repoA, ".gitignore"), "*.tmp\n")
	write(t, filepath.Join(repoB, ".gitignore"), "secret.txt\n")

	aIgnored := filepath.Join(repoA, "x.tmp")
	aKept := filepath.Join(repoA, "x.txt")
	bIgnored := filepath.Join(repoB, "secret.txt")
	bKept := filepath.Join(repoB, "public.txt")
	for _, p := range []string{aIgnored, aKept, bIgnored, bKept} {
		write(t, p, "x")
	}

	f := New(true)
	if !f.Enabled() {
		t.Skip("git no disponible")
	}
	res, err := f.Ignored([]string{aIgnored, aKept, bIgnored, bKept})
	if err != nil {
		t.Fatal(err)
	}
	if !res[aIgnored] || !res[bIgnored] {
		t.Errorf("los ignorados de ambos repos deben detectarse: %v", res)
	}
	if res[aKept] || res[bKept] {
		t.Errorf("los no ignorados no deben marcarse: %v", res)
	}
}

func TestIgnoredPathsOutsideAnyRepo(t *testing.T) {
	testutil.RequireGit(t)
	dir := t.TempDir() // sin git init
	p := filepath.Join(dir, "a.txt")
	write(t, p, "x")
	f := New(true)
	if !f.Enabled() {
		t.Skip("git no disponible")
	}
	res, err := f.Ignored([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("rutas fuera de repo no deben marcarse: %v", res)
	}
}

func TestDisabledFilter(t *testing.T) {
	f := New(false)
	if f.Enabled() {
		t.Errorf("filtro deshabilitado no debe estar Enabled")
	}
	res, err := f.Ignored([]string{"/cualquier/ruta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("filtro deshabilitado no ignora nada: %v", res)
	}
}
