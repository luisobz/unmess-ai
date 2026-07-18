package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// seedClaudeSession siembra una sesión nativa de Claude: transcript con dos
// versiones (ventana 10:00–10:30) y actividad alrededor.
func seedClaudeSession(t *testing.T, srv *Server) (base time.Time) {
	t.Helper()
	st := srv.store
	dir := st.BaseDir()
	base = time.Date(2026, 7, 15, 10, 0, 0, 0, time.Local)

	transcript := ".claude/projects/-home-luis-proj/abc-123.jsonl"
	writeVersion(t, st, dir, transcript,
		`{"type":"user","message":{"role":"user","content":"refactor de pagos"}}`+"\n", base)
	writeVersion(t, st, dir, transcript,
		`{"type":"user","message":{"role":"user","content":"refactor de pagos"}}`+"\n{\"type\":\"assistant\"}\n", base.Add(30*time.Minute))

	// Workspace: nota.md existía antes (prior a las 09:00) y cambió en ventana.
	writeVersion(t, st, dir, "Documentos/nota.md", "v-anterior\n", base.Add(-time.Hour))
	writeVersion(t, st, dir, "Documentos/nota.md", "v-sesion\n", base.Add(10*time.Minute))

	// Workspace: creado durante la ventana (sin prior).
	writeVersion(t, st, dir, "Documentos/nuevo.md", "creado\n", base.Add(15*time.Minute))

	// Fichero propio del agente (no transcript) tocado en ventana.
	writeVersion(t, st, dir, ".claude.json", "{}\n", base.Add(5*time.Minute))

	// Otro agente activo a la vez: debe quedar fuera de la traza.
	writeVersion(t, st, dir, ".gemini/history", "x\n", base.Add(12*time.Minute))

	// Workspace fuera de la ventana: no debe aparecer.
	writeVersion(t, st, dir, "Documentos/ajeno.md", "z\n", base.Add(2*time.Hour))
	return base
}

func TestAgentSessionsNative(t *testing.T) {
	srv, _, _ := newTestServer(t)
	seedClaudeSession(t, srv)

	rec := do(t, srv, "GET", "/api/agent/sessions?agent=claude", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Agent    string        `json:"agent"`
		Sessions []sessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if len(out.Sessions) != 1 {
		t.Fatalf("esperada 1 sesión, obtenidas %d: %+v", len(out.Sessions), out.Sessions)
	}
	ses := out.Sessions[0]
	if ses.Kind != "native" || ses.Transcript != ".claude/projects/-home-luis-proj/abc-123.jsonl" {
		t.Errorf("sesión inesperada: %+v", ses)
	}
	if ses.Title != "refactor de pagos" {
		t.Errorf("título = %q; quería %q", ses.Title, "refactor de pagos")
	}
	if ses.Project != "home-luis-proj" {
		t.Errorf("proyecto = %q", ses.Project)
	}
	// Cambios en ventana: solo los ficheros del usuario (nota.md y nuevo.md).
	// El estado interno de agentes (.claude.json, .gemini/...) queda fuera de
	// la traza visible, igual que ajeno.md por estar fuera de la ventana.
	if ses.Changes != 2 {
		t.Errorf("changes = %d; quería 2", ses.Changes)
	}
	if ses.WorkspaceFiles != 2 {
		t.Errorf("workspace=%d; quería 2", ses.WorkspaceFiles)
	}

	// Detalle de la sesión.
	rec = do(t, srv, "GET", "/api/agent/session?agent=claude&id="+url.QueryEscape(ses.ID), reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("session detail: %d %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Session sessionInfo    `json:"session"`
		Events  []sessionEvent `json:"events"`
		Files   []sessionFile  `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decodificar detalle: %v", err)
	}
	if len(detail.Events) != 2 {
		t.Fatalf("eventos = %d; quería 2: %+v", len(detail.Events), detail.Events)
	}
	// Cronología descendente: el más reciente primero.
	if detail.Events[0].Path != "Documentos/nuevo.md" || !detail.Events[0].First {
		t.Errorf("primer evento inesperado: %+v", detail.Events[0])
	}
	byPath := map[string]sessionFile{}
	for _, f := range detail.Files {
		byPath[f.Path] = f
	}
	if f := byPath["Documentos/nota.md"]; f.Prior == "" || f.Created {
		t.Errorf("nota.md mal clasificado: %+v", f)
	}
	if f := byPath["Documentos/nuevo.md"]; f.Prior != "" || !f.Created {
		t.Errorf("nuevo.md mal clasificado: %+v", f)
	}
	if _, ok := byPath[".claude.json"]; ok {
		t.Errorf(".claude.json no debe aparecer en la traza visible")
	}

	// Agente desconocido → 400.
	rec = do(t, srv, "GET", "/api/agent/sessions?agent=nadie", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("agente desconocido: %d; quería 400", rec.Code)
	}
}

func TestAgentSessionsCluster(t *testing.T) {
	srv, st, base := newTestServer(t)
	t0 := time.Date(2026, 7, 15, 9, 0, 0, 0, time.Local)
	// Copilot sin transcript reconocible: dos ráfagas separadas por más de
	// 30 min forman dos clusters de actividad.
	writeVersion(t, st, base, ".copilot/state.json", "a\n", t0)
	writeVersion(t, st, base, ".copilot/settings.json", "b\n", t0.Add(5*time.Minute))
	writeVersion(t, st, base, ".copilot/state.json", "c\n", t0.Add(2*time.Hour))
	// Fichero del usuario tocado durante la primera ráfaga: único "cambio".
	writeVersion(t, st, base, "Documentos/nota.md", "x\n", t0.Add(3*time.Minute))

	rec := do(t, srv, "GET", "/api/agent/sessions?agent=copilot", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions: %d", rec.Code)
	}
	var out struct {
		Sessions []sessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if len(out.Sessions) != 2 {
		t.Fatalf("esperados 2 clusters, obtenidos %d: %+v", len(out.Sessions), out.Sessions)
	}
	for _, ses := range out.Sessions {
		if ses.Kind != "cluster" {
			t.Errorf("kind = %q; quería cluster", ses.Kind)
		}
	}
	// Más reciente primero; los cambios cuentan SOLO ficheros del usuario.
	if out.Sessions[0].Changes != 0 || out.Sessions[1].Changes != 1 {
		t.Errorf("changes = %d,%d; quería 0,1", out.Sessions[0].Changes, out.Sessions[1].Changes)
	}
}

// TestAgentSessionsCommandCode verifica el formato real de CommandCode: cada
// sesión tiene <uuid>.jsonl + <uuid>.checkpoints.jsonl + <uuid>.meta.json.
// Solo el .jsonl es la sesión (los sidecars duplicaban sesiones) y el título
// sale del meta.json.
func TestAgentSessionsCommandCode(t *testing.T) {
	srv, st, base := newTestServer(t)
	t0 := time.Date(2026, 7, 15, 10, 0, 0, 0, time.Local)
	dir := ".commandcode/projects/home-luis-app/"
	writeVersion(t, st, base, dir+"0979f59e.jsonl", `{"type":"user","message":{"content":"hola"}}`+"\n", t0)
	writeVersion(t, st, base, dir+"0979f59e.jsonl", "x\n", t0.Add(10*time.Minute))
	writeVersion(t, st, base, dir+"0979f59e.checkpoints.jsonl", "{}\n", t0.Add(2*time.Minute))
	writeVersion(t, st, base, dir+"0979f59e.meta.json", `{"title":"Add creator signature to footer"}`+"\n", t0.Add(3*time.Minute))

	rec := do(t, srv, "GET", "/api/agent/sessions?agent=commandcode", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions: %d", rec.Code)
	}
	var out struct {
		Sessions []sessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if len(out.Sessions) != 1 {
		t.Fatalf("esperada 1 sesión (sin duplicados por sidecars), obtenidas %d: %+v", len(out.Sessions), out.Sessions)
	}
	ses := out.Sessions[0]
	if ses.Kind != "native" || ses.Transcript != dir+"0979f59e.jsonl" {
		t.Errorf("sesión inesperada: %+v", ses)
	}
	if ses.Title != "Add creator signature to footer" {
		t.Errorf("título = %q; quería el del meta.json", ses.Title)
	}
	if ses.Project != "home-luis-app" {
		t.Errorf("proyecto = %q", ses.Project)
	}
}

// TestAgentSessionsAntigravity verifica el formato real de Gemini/Antigravity:
// una conversación .pb = una sesión, con título en el overview.txt del brain.
func TestAgentSessionsAntigravity(t *testing.T) {
	srv, st, base := newTestServer(t)
	t0 := time.Date(2026, 7, 15, 9, 0, 0, 0, time.Local)
	conv := ".gemini/antigravity/conversations/"
	brain := ".gemini/antigravity/brain/"

	// Dos conversaciones separadas (antes: un único cluster).
	writeVersion(t, st, base, conv+"aaa-111.pb", "\x08\x01binario\n", t0)
	writeVersion(t, st, base, conv+"aaa-111.pb", "\x08\x02binario2\n", t0.Add(8*time.Minute))
	writeVersion(t, st, base, conv+"bbb-222.pb", "\x08\x01otro\n", t0.Add(1*time.Hour))
	overview := `{"step_index":0,"type":"USER_INPUT","content":"<USER_REQUEST>\nañade un test basico\n</USER_REQUEST>\n<ADDITIONAL_METADATA>x</ADDITIONAL_METADATA>"}` + "\n"
	writeVersion(t, st, base, brain+"aaa-111/.system_generated/logs/overview.txt", overview, t0.Add(time.Minute))
	// Actividad interna extra que NO debe crear sesiones ni salir en la traza.
	writeVersion(t, st, base, ".gemini/GEMINI.md", "notas\n", t0.Add(2*time.Minute))

	rec := do(t, srv, "GET", "/api/agent/sessions?agent=gemini", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions: %d", rec.Code)
	}
	var out struct {
		Sessions []sessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if len(out.Sessions) != 2 {
		t.Fatalf("esperadas 2 sesiones nativas, obtenidas %d: %+v", len(out.Sessions), out.Sessions)
	}
	// Más reciente primero: bbb-222.
	if out.Sessions[0].Transcript != conv+"bbb-222.pb" || out.Sessions[0].Kind != "native" {
		t.Errorf("sesión [0] inesperada: %+v", out.Sessions[0])
	}
	if got := out.Sessions[1].Title; got != "añade un test basico" {
		t.Errorf("título desde overview = %q", got)
	}
	// La traza visible no incluye el estado interno del agente.
	if out.Sessions[1].Changes != 0 {
		t.Errorf("changes = %d; quería 0 (solo hubo actividad interna)", out.Sessions[1].Changes)
	}
}

func TestAgentRevert(t *testing.T) {
	srv, st, dir := newTestServer(t)
	base := time.Date(2026, 7, 15, 10, 0, 0, 0, time.Local)
	start := base.Format(time.RFC3339)
	end := base.Add(30 * time.Minute).Format(time.RFC3339)

	// Caso "revert": prior + cambio en ventana; el disco tiene lo de la sesión.
	writeVersion(t, st, dir, "docs/editado.md", "antes\n", base.Add(-time.Hour))
	writeVersion(t, st, dir, "docs/editado.md", "durante\n", base.Add(10*time.Minute))

	// Caso "skip_no_prior": creado en la ventana.
	writeVersion(t, st, dir, "docs/creado.md", "nuevo\n", base.Add(12*time.Minute))

	// Caso "skip_unchanged": cambió en ventana pero el disco ya vale lo previo.
	writeVersion(t, st, dir, "docs/igual.md", "estable\n", base.Add(-time.Hour))
	writeVersion(t, st, dir, "docs/igual.md", "tocado\n", base.Add(14*time.Minute))
	if err := os.WriteFile(filepath.Join(dir, "docs/igual.md"), []byte("estable\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Caso "restore_deleted": con prior, tocado en ventana y borrado después.
	writeVersion(t, st, dir, "docs/borrado.md", "previo\n", base.Add(-time.Hour))
	writeVersion(t, st, dir, "docs/borrado.md", "durante\n", base.Add(16*time.Minute))
	if err := os.Remove(filepath.Join(dir, "docs/borrado.md")); err != nil {
		t.Fatal(err)
	}

	// Fichero de agente cambiado en ventana: el revert no debe tocarlo jamás.
	writeVersion(t, st, dir, ".claude.json", "agente-antes\n", base.Add(-time.Hour))
	writeVersion(t, st, dir, ".claude.json", "agente-durante\n", base.Add(18*time.Minute))

	body := `{"start":"` + start + `","end":"` + end + `","dry_run":true}`
	rec := do(t, srv, "POST", "/api/agent/revert", reqOpts{token: srv.Token(), body: body})
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run: %d %s", rec.Code, rec.Body.String())
	}
	var plan revertResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decodificar plan: %v", err)
	}
	if !plan.DryRun || plan.Reverted != 1 || plan.RestoredDeleted != 1 ||
		plan.SkippedNoPrior != 1 || plan.SkippedUnchanged != 1 || plan.Failed != 0 {
		t.Fatalf("plan inesperado: %+v", plan)
	}
	actions := map[string]string{}
	for _, it := range plan.Items {
		actions[it.Path] = it.Action
		if it.Safety != "" {
			t.Errorf("dry-run no debe crear safety versions: %+v", it)
		}
	}
	want := map[string]string{
		"docs/editado.md": "revert",
		"docs/creado.md":  "skip_no_prior",
		"docs/igual.md":   "skip_unchanged",
		"docs/borrado.md": "restore_deleted",
	}
	for p, a := range want {
		if actions[p] != a {
			t.Errorf("%s: acción %q; quería %q", p, actions[p], a)
		}
	}
	if _, ok := actions[".claude.json"]; ok {
		t.Errorf("el revert incluyó estado interno del agente")
	}
	// El dry-run no toca el disco.
	if got, _ := os.ReadFile(filepath.Join(dir, "docs/editado.md")); string(got) != "durante\n" {
		t.Fatalf("dry-run modificó el disco: %q", got)
	}

	// Ejecutar de verdad.
	body = `{"start":"` + start + `","end":"` + end + `","dry_run":false}`
	rec = do(t, srv, "POST", "/api/agent/revert", reqOpts{token: srv.Token(), body: body})
	if rec.Code != http.StatusOK {
		t.Fatalf("revert: %d %s", rec.Code, rec.Body.String())
	}
	var res revertResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decodificar resultado: %v", err)
	}
	if res.Reverted != 1 || res.RestoredDeleted != 1 || res.Failed != 0 {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "docs/editado.md")); string(got) != "antes\n" {
		t.Errorf("editado.md no revertido: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "docs/borrado.md")); string(got) != "previo\n" {
		t.Errorf("borrado.md no restaurado: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "docs/creado.md")); string(got) != "nuevo\n" {
		t.Errorf("creado.md debía conservarse: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, ".claude.json")); string(got) != "agente-durante\n" {
		t.Errorf(".claude.json debía quedar intacto: %q", got)
	}
	// El revert del fichero con contenido en disco crea safety version, y esa
	// safety permite deshacer la operación con /api/restore.
	var editado revertItem
	for _, it := range res.Items {
		if it.Path == "docs/editado.md" {
			editado = it
		}
	}
	if editado.Safety == "" {
		t.Fatalf("revert sin safety version: %+v", res.Items)
	}
	undo := `{"path":"docs/editado.md","version":"` + editado.Safety + `"}`
	rec = do(t, srv, "POST", "/api/restore", reqOpts{token: srv.Token(), body: undo})
	if rec.Code != http.StatusOK {
		t.Fatalf("undo: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "docs/editado.md")); string(got) != "durante\n" {
		t.Errorf("undo no recuperó el estado post-sesión: %q", got)
	}

	// Target acota la operación a un fichero concreto.
	body = `{"start":"` + start + `","end":"` + end + `","target":"docs/editado.md","dry_run":true}`
	rec = do(t, srv, "POST", "/api/agent/revert", reqOpts{token: srv.Token(), body: body})
	var scoped revertResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &scoped); err != nil {
		t.Fatalf("decodificar scoped: %v", err)
	}
	if len(scoped.Items) != 1 || scoped.Items[0].Path != "docs/editado.md" {
		t.Errorf("target no acotó: %+v", scoped.Items)
	}
}
