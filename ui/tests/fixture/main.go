// Synthetic UI fixture. Never reads the real user's configuration or files.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/luisobz/unmess-ai/internal/api"
	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/daemon"
	"github.com/luisobz/unmess-ai/internal/store"
)

func main() {
	root, err := os.MkdirTemp("", "unmessai-ui-test-")
	must(err)
	defer os.RemoveAll(root)
	base := filepath.Join(root, "home")
	prefix := filepath.Join(root, "backup")
	must(os.MkdirAll(base, 0o700))
	st := store.New(prefix, base)
	cfg := config.Default()
	cfg.Prefix = prefix
	cfg.UI.Port = 48339
	cfg.IncludedPaths = []string{base}
	cfg.DebounceSeconds = 1
	cfg.SetPath(filepath.Join(root, "config.toml"))
	must(cfg.Save())
	t0 := time.Now().Add(-time.Hour).In(time.Local).Truncate(time.Second)
	seed := func(path, content string, ts time.Time) {
		abs := st.OriginalPath(path)
		must(os.MkdirAll(filepath.Dir(abs), 0o700))
		must(os.WriteFile(abs, []byte(content), 0o600))
		_, err := st.WriteVersion(abs, ts)
		must(err)
	}
	line := func(v any) string { b, e := json.Marshal(v); must(e); return string(b) + "\n" }
	cwd := filepath.Join(base, "projects", "demo")
	raw := line(map[string]any{"type": "session_meta", "timestamp": t0.Format(time.RFC3339Nano), "payload": map[string]any{"cwd": cwd}}) +
		line(map[string]any{"type": "response_item", "timestamp": t0.Add(time.Minute).Format(time.RFC3339Nano), "payload": map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "Corrige el login y mejora la documentación"}}}}) +
		line(map[string]any{"type": "response_item", "timestamp": t0.Add(2 * time.Minute).Format(time.RFC3339Nano), "payload": map[string]any{"type": "custom_tool_call", "name": "apply_patch", "call_id": "a", "input": "*** Begin Patch\n*** Update File: src/login.go\n*** Update File: src/conflict.go\n*** Update File: src/shared.go\n*** Add File: ignored.go\n*** End Patch"}}) +
		line(map[string]any{"type": "response_item", "timestamp": t0.Add(3 * time.Minute).Format(time.RFC3339Nano), "payload": map[string]any{"type": "custom_tool_call_output", "call_id": "a", "output": "Success. Updated the following files:\nM src/login.go\nM src/conflict.go\nA ignored.go"}})
	seed(".codex/sessions/2026/10/04/rollout-demo.jsonl", raw, t0.Add(4*time.Minute))
	seed("projects/demo/src/login.go", "package demo\n// old login\n", t0.Add(-time.Hour))
	seed("projects/demo/src/login.go", "package demo\n// fixed login\n", t0.Add(3*time.Minute))
	seed("projects/demo/src/conflict.go", "before", t0.Add(-time.Hour))
	seed("projects/demo/src/conflict.go", "session", t0.Add(3*time.Minute))
	seed("projects/demo/src/conflict.go", "later work", t0.Add(20*time.Minute))
	seed("projects/demo/src/shared.go", "before shared", t0.Add(-time.Hour))
	seed("projects/demo/src/shared.go", "both agents", t0.Add(3*time.Minute))
	shared := line(map[string]any{"type": "user", "timestamp": t0.Format(time.RFC3339Nano), "cwd": cwd, "message": map[string]any{"role": "user", "content": "Otra sesión sobre el archivo compartido"}}) +
		line(map[string]any{"type": "assistant", "timestamp": t0.Add(2 * time.Minute).Format(time.RFC3339Nano), "message": map[string]any{"content": []any{map[string]any{"type": "tool_use", "id": "edit-shared", "name": "Edit", "input": map[string]any{"file_path": filepath.Join(cwd, "src", "shared.go")}}}}}) +
		line(map[string]any{"type": "user", "timestamp": t0.Add(3 * time.Minute).Format(time.RFC3339Nano), "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "edit-shared", "content": "updated"}}}})
	seed(".claude/projects/demo/shared.jsonl", shared, t0.Add(4*time.Minute))
	seed("projects/demo/docs/manual.md", "manual edit", t0.Add(3*time.Minute))
	seed("projects/unrelated/noise.md", "unrelated", t0.Add(3*time.Minute))
	claude := line(map[string]any{"type": "user", "timestamp": t0.Format(time.RFC3339Nano), "cwd": filepath.Join(base, "projects", "another"), "message": map[string]any{"role": "user", "content": "Revisa el otro proyecto"}}) + line(map[string]any{"type": "assistant", "timestamp": t0.Add(5 * time.Minute).Format(time.RFC3339Nano)})
	seed(".claude/projects/another/second.jsonl", claude, t0.Add(6*time.Minute))
	rt := &daemon.Runtime{Store: st, Config: cfg, Events: daemon.NewBroker()}
	srv, err := api.New(rt, "ui-test")
	must(err)
	server := &http.Server{Addr: "127.0.0.1:48339", Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-ch; server.Close() }()
	fmt.Println("UI fixture listening at http://127.0.0.1:48339")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		must(err)
	}
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
