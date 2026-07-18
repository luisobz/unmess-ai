package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/daemon"
	"github.com/luisobz/unmess-ai/internal/store"
)

const testPort = 48111

func newTestServer(t *testing.T) (*Server, *store.Store, string) {
	t.Helper()
	base := t.TempDir()
	prefix := t.TempDir()
	st := store.New(prefix, base)
	cfg := config.Default()
	cfg.UI.Port = testPort
	rt := &daemon.Runtime{
		Config:  cfg,
		Store:   st,
		BaseDir: base,
		Logger:  log.New(io.Discard, "", 0),
		Events:  daemon.NewBroker(),
	}
	srv, err := New(rt, "test-version")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, st, base
}

// writeVersion escribe content en <base>/rel y crea una versión con timestamp ts.
func writeVersion(t *testing.T, st *store.Store, base, rel, content string, ts time.Time) {
	t.Helper()
	abs := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := st.WriteVersion(abs, ts); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}
}

type reqOpts struct {
	token   string
	host    string
	origin  string
	body    string
	headers map[string]string
}

func do(t *testing.T, srv *Server, method, target string, o reqOpts) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if o.body != "" {
		body = strings.NewReader(o.body)
	}
	req := httptest.NewRequest(method, target, body)
	if o.host == "" {
		o.host = "127.0.0.1:48111"
	}
	req.Host = o.host
	if o.token != "" {
		req.Header.Set("Authorization", "Bearer "+o.token)
	}
	if o.origin != "" {
		req.Header.Set("Origin", o.origin)
	}
	if o.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestAuth(t *testing.T) {
	srv, _, _ := newTestServer(t)

	// Sin token → 401.
	if rec := do(t, srv, "GET", "/api/status", reqOpts{}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin token: esperado 401, obtenido %d", rec.Code)
	}
	// Token inválido → 401.
	if rec := do(t, srv, "GET", "/api/status", reqOpts{token: "malo"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token inválido: esperado 401, obtenido %d", rec.Code)
	}
	// Token válido → 200.
	if rec := do(t, srv, "GET", "/api/status", reqOpts{token: srv.Token()}); rec.Code != http.StatusOK {
		t.Fatalf("token válido: esperado 200, obtenido %d", rec.Code)
	}
}

func TestTokenEndpoint(t *testing.T) {
	srv, _, _ := newTestServer(t)

	// Petición local legítima → 200 y devuelve el token.
	rec := do(t, srv, "GET", "/api/token", reqOpts{host: "127.0.0.1:48111"})
	if rec.Code != http.StatusOK {
		t.Fatalf("token local: esperado 200, obtenido %d", rec.Code)
	}
	var tok struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tok); err != nil {
		t.Fatalf("decodificando token: %v", err)
	}
	if tok.Token != srv.Token() {
		t.Fatalf("token devuelto no coincide")
	}

	// Origin cruzado → 403.
	rec = do(t, srv, "GET", "/api/token", reqOpts{host: "127.0.0.1:48111", origin: "http://evil.example.com"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("origin cruzado: esperado 403, obtenido %d", rec.Code)
	}

	// Host no local → 403.
	rec = do(t, srv, "GET", "/api/token", reqOpts{host: "evil.example.com"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("host no local: esperado 403, obtenido %d", rec.Code)
	}

	// Sec-Fetch-Site: cross-site → 403.
	rec = do(t, srv, "GET", "/api/token", reqOpts{host: "127.0.0.1:48111", headers: map[string]string{"Sec-Fetch-Site": "cross-site"}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sec-fetch-site cross-site: esperado 403, obtenido %d", rec.Code)
	}
}

func TestFilesAndVersions(t *testing.T) {
	srv, st, _ := newTestServer(t)
	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	writeVersion(t, st, srv.store.BaseDir(), "notas.txt", "hola\n", base)
	writeVersion(t, st, srv.store.BaseDir(), "notas.txt", "hola mundo\n", base.Add(time.Minute))
	writeVersion(t, st, srv.store.BaseDir(), "sub/otro.txt", "x\n", base.Add(2*time.Minute))
	_ = st

	// files
	rec := do(t, srv, "GET", "/api/files", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("files: %d", rec.Code)
	}
	var files []fileEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatalf("decodificar files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("esperados 2 ficheros, obtenidos %d", len(files))
	}

	// búsqueda q (case-insensitive substring)
	rec = do(t, srv, "GET", "/api/files?q=OTRO", reqOpts{token: srv.Token()})
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatalf("decodificar files q: %v", err)
	}
	if len(files) != 1 || files[0].Path != "sub/otro.txt" {
		t.Fatalf("búsqueda q inesperada: %+v", files)
	}

	// versions de notas.txt
	rec = do(t, srv, "GET", "/api/versions?path=notas.txt", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("versions: %d", rec.Code)
	}
	var versions []versionEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &versions); err != nil {
		t.Fatalf("decodificar versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("esperadas 2 versiones, obtenidas %d", len(versions))
	}
}

func TestAgentClassification(t *testing.T) {
	srv, st, _ := newTestServer(t)
	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	writeVersion(t, st, srv.store.BaseDir(), ".claude/projects/x/sess.jsonl", "{}\n", base)
	writeVersion(t, st, srv.store.BaseDir(), ".claude.json", "{}\n", base.Add(time.Minute))
	writeVersion(t, st, srv.store.BaseDir(), ".gemini/history", "x\n", base.Add(2*time.Minute))
	writeVersion(t, st, srv.store.BaseDir(), "Documentos/nota.md", "hola\n", base.Add(3*time.Minute))

	// /api/files: cada fichero lleva su agente (o vacío).
	rec := do(t, srv, "GET", "/api/files", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("files: %d", rec.Code)
	}
	var files []fileEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &files); err != nil {
		t.Fatalf("decodificar files: %v", err)
	}
	got := map[string]string{}
	for _, f := range files {
		got[f.Path] = f.Agent
	}
	want := map[string]string{
		".claude/projects/x/sess.jsonl": "claude",
		".claude.json":                  "claude",
		".gemini/history":               "gemini",
		"Documentos/nota.md":            "",
	}
	for path, wantAgent := range want {
		if got[path] != wantAgent {
			t.Errorf("agente de %q = %q; quería %q", path, got[path], wantAgent)
		}
	}

	// /api/agents: registro con conteos por agente.
	rec = do(t, srv, "GET", "/api/agents", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("agents: %d", rec.Code)
	}
	var ag []agentEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &ag); err != nil {
		t.Fatalf("decodificar agents: %v", err)
	}
	counts := map[string]int{}
	for _, a := range ag {
		counts[a.ID] = a.Files
	}
	if counts["claude"] != 2 {
		t.Errorf("claude Files = %d; quería 2", counts["claude"])
	}
	if counts["gemini"] != 1 {
		t.Errorf("gemini Files = %d; quería 1", counts["gemini"])
	}
	if counts["cursor"] != 0 {
		t.Errorf("cursor Files = %d; quería 0", counts["cursor"])
	}
}

func TestContentAndDiff(t *testing.T) {
	srv, st, _ := newTestServer(t)
	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	writeVersion(t, st, srv.store.BaseDir(), "notas.txt", "linea1\nlinea2\n", base)
	writeVersion(t, st, srv.store.BaseDir(), "notas.txt", "linea1\nlinea2 mod\n", base.Add(time.Minute))

	versions, _ := st.ListVersions("notas.txt")
	newest := versions[0].Name
	oldest := versions[1].Name

	// content de la versión más reciente
	rec := do(t, srv, "GET", "/api/content?path=notas.txt&version="+newest, reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("content: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "linea2 mod") {
		t.Fatalf("content inesperado: %q", rec.Body.String())
	}

	// diff explícito oldest→newest
	rec = do(t, srv, "GET", "/api/diff?path=notas.txt&from="+oldest+"&to="+newest, reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("diff: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "-linea2") || !strings.Contains(body, "+linea2 mod") {
		t.Fatalf("diff inesperado: %q", body)
	}

	// diff por defecto (sin from/to)
	rec = do(t, srv, "GET", "/api/diff?path=notas.txt", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("diff por defecto: %d", rec.Code)
	}
}

func TestContentBinary(t *testing.T) {
	srv, st, _ := newTestServer(t)
	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	writeVersion(t, st, srv.store.BaseDir(), "bin.dat", "abc\x00def", base)
	versions, _ := st.ListVersions("bin.dat")
	rec := do(t, srv, "GET", "/api/content?path=bin.dat&version="+versions[0].Name, reqOpts{token: srv.Token()})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("binario: esperado 415, obtenido %d", rec.Code)
	}
	var j struct {
		Binary bool `json:"binary"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &j)
	if !j.Binary {
		t.Fatalf("esperado binary:true")
	}
}

func TestRestore(t *testing.T) {
	srv, st, base := newTestServer(t)
	ts := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	writeVersion(t, st, base, "doc.txt", "versión antigua\n", ts)
	// Modifica el fichero en disco (estado "actual" distinto).
	abs := filepath.Join(base, "doc.txt")
	if err := os.WriteFile(abs, []byte("estado actual\n"), 0o644); err != nil {
		t.Fatalf("write actual: %v", err)
	}

	versions, _ := st.ListVersions("doc.txt")
	oldName := versions[0].Name

	rec := do(t, srv, "POST", "/api/restore", reqOpts{
		token:  srv.Token(),
		origin: "http://127.0.0.1:48111",
		body:   `{"path":"doc.txt","version":"` + oldName + `"}`,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("restore: esperado 200, obtenido %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		SafetyVersion string `json:"safety_version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decodificar restore: %v", err)
	}
	if resp.SafetyVersion == "" {
		t.Fatalf("esperada safety_version no vacía")
	}
	// El fichero en disco debe ser ahora la versión antigua.
	got, _ := os.ReadFile(abs)
	if string(got) != "versión antigua\n" {
		t.Fatalf("restore no aplicado: %q", string(got))
	}
}

func TestPrune(t *testing.T) {
	srv, st, _ := newTestServer(t)
	base := time.Date(2026, 7, 11, 10, 0, 0, 0, time.Local)
	writeVersion(t, st, srv.store.BaseDir(), "p.txt", "a\n", base)

	rec := do(t, srv, "POST", "/api/prune", reqOpts{
		token:  srv.Token(),
		origin: "http://127.0.0.1:48111",
		body:   `{"dry_run":true}`,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("prune: esperado 200, obtenido %d (%s)", rec.Code, rec.Body.String())
	}
	var sum pruneResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatalf("decodificar prune: %v", err)
	}
	if sum.Examined != 1 {
		t.Fatalf("esperado examined=1, obtenido %d", sum.Examined)
	}
}

func TestPathTraversal(t *testing.T) {
	srv, _, _ := newTestServer(t)
	cases := []string{
		"/api/versions?path=../secret",
		"/api/versions?path=../../etc/passwd",
		"/api/versions?path=/etc/passwd",
		"/api/content?path=..%2F..%2Fetc%2Fpasswd&version=current",
	}
	for _, c := range cases {
		rec := do(t, srv, "GET", c, reqOpts{token: srv.Token()})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: esperado 400, obtenido %d", c, rec.Code)
		}
	}
}

func TestConfigGetPut(t *testing.T) {
	srv, _, _ := newTestServer(t)
	// El PUT necesita persistir: fija la ruta de config a un temporal.
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	srv.rt.Config.SetPath(cfgPath)

	// GET
	rec := do(t, srv, "GET", "/api/config", reqOpts{token: srv.Token()})
	if rec.Code != http.StatusOK {
		t.Fatalf("config get: %d", rec.Code)
	}
	var dto configDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decodificar config: %v", err)
	}

	// PUT cambiando solo retención → restart_required=false
	dto.Retention.MaxVersions = 10
	b, _ := json.Marshal(dto)
	rec = do(t, srv, "PUT", "/api/config", reqOpts{token: srv.Token(), origin: "http://127.0.0.1:48111", body: string(b)})
	if rec.Code != http.StatusOK {
		t.Fatalf("config put retención: %d (%s)", rec.Code, rec.Body.String())
	}
	var r1 struct {
		Restart bool `json:"restart_required"`
	}
	json.Unmarshal(rec.Body.Bytes(), &r1)
	if r1.Restart {
		t.Fatalf("cambio de retención no debería requerir reinicio")
	}

	// PUT cambiando debounce → se aplica en caliente, restart_required=false
	dto.DebounceSeconds = 5
	b, _ = json.Marshal(dto)
	rec = do(t, srv, "PUT", "/api/config", reqOpts{token: srv.Token(), origin: "http://127.0.0.1:48111", body: string(b)})
	var r2 struct {
		Restart bool `json:"restart_required"`
	}
	json.Unmarshal(rec.Body.Bytes(), &r2)
	if r2.Restart {
		t.Fatalf("cambio de debounce se aplica en caliente: no debería requerir reinicio")
	}

	// PUT cambiando prefix (raíz del store) → estructural, restart_required=true
	dto.Prefix = dto.Prefix + "-otro"
	b, _ = json.Marshal(dto)
	rec = do(t, srv, "PUT", "/api/config", reqOpts{token: srv.Token(), origin: "http://127.0.0.1:48111", body: string(b)})
	var r3 struct {
		Restart bool `json:"restart_required"`
	}
	json.Unmarshal(rec.Body.Bytes(), &r3)
	if !r3.Restart {
		t.Fatalf("cambio de prefix debería requerir reinicio")
	}
}

func TestUIServed(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rec := do(t, srv, "GET", "/", reqOpts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("UI /: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unmessai") {
		t.Fatalf("index.html no servido correctamente")
	}
	rec = do(t, srv, "GET", "/scripts/app.js", reqOpts{})
	if rec.Code != http.StatusOK {
		t.Fatalf("app.js: %d", rec.Code)
	}
}

func TestPauseAndStatus(t *testing.T) {
	srv, _, _ := newTestServer(t)
	tok := srv.Token()

	// Estado inicial: no pausado, vigilando.
	var st struct {
		Watching bool `json:"watching"`
		Paused   bool `json:"paused"`
	}
	rec := do(t, srv, "GET", "/api/status", reqOpts{token: tok})
	if rec.Code != http.StatusOK {
		t.Fatalf("status inicial: %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if st.Paused || !st.Watching {
		t.Fatalf("estado inicial: paused=%v watching=%v", st.Paused, st.Watching)
	}

	// Pausar.
	rec = do(t, srv, "POST", "/api/pause", reqOpts{token: tok, body: `{"paused":true}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("pause: esperado 200, obtenido %d (%s)", rec.Code, rec.Body.String())
	}
	var pr struct {
		Paused bool `json:"paused"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pr)
	if !pr.Paused {
		t.Fatalf("pause: respuesta paused=false")
	}

	// Status refleja la pausa.
	rec = do(t, srv, "GET", "/api/status", reqOpts{token: tok})
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if !st.Paused || st.Watching {
		t.Fatalf("status tras pausar: paused=%v watching=%v", st.Paused, st.Watching)
	}

	// Petición no local (host ajeno) → 403 aunque el token sea válido.
	rec = do(t, srv, "POST", "/api/pause", reqOpts{token: tok, host: "evil.example.com", body: `{"paused":false}`})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("pause no local: esperado 403, obtenido %d", rec.Code)
	}

	// Reanudar.
	rec = do(t, srv, "POST", "/api/pause", reqOpts{token: tok, body: `{"paused":false}`})
	if rec.Code != http.StatusOK {
		t.Fatalf("resume: %d", rec.Code)
	}
	rec = do(t, srv, "GET", "/api/status", reqOpts{token: tok})
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if st.Paused || !st.Watching {
		t.Fatalf("status tras reanudar: paused=%v watching=%v", st.Paused, st.Watching)
	}
}

func TestForget(t *testing.T) {
	srv, st, base := newTestServer(t)
	ts := time.Date(2026, 7, 13, 10, 0, 0, 0, time.Local)
	writeVersion(t, st, base, "docs/keep.txt", "contenido\n", ts)
	writeVersion(t, st, base, ".config/app/estado.json", "{}\n", ts)
	writeVersion(t, st, base, "proyecto/salida.LOG", "log\n", ts)

	// Olvidar un fichero concreto.
	rec := do(t, srv, "POST", "/api/forget", reqOpts{
		token:  srv.Token(),
		origin: "http://127.0.0.1:48111",
		body:   `{"path":"docs/keep.txt"}`,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("forget: esperado 200, obtenido %d (%s)", rec.Code, rec.Body.String())
	}
	var resp forgetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decodificar forget: %v", err)
	}
	if resp.Forgotten != 1 || resp.FreedBytes == 0 {
		t.Fatalf("esperado forgotten=1 y bytes>0, obtenido %+v", resp)
	}
	if vs, _ := st.ListVersions("docs/keep.txt"); len(vs) != 0 {
		t.Fatalf("el historial de docs/keep.txt debería estar vacío")
	}
	// El fichero en disco no se toca.
	if _, err := os.Stat(filepath.Join(base, "docs/keep.txt")); err != nil {
		t.Fatalf("el original no debe borrarse: %v", err)
	}

	// Purga por ignore_patterns (defaults: .config, **/*.log, **/cache/**);
	// el .LOG en mayúsculas también debe coincidir (case-insensitive).
	rec = do(t, srv, "POST", "/api/forget", reqOpts{
		token:  srv.Token(),
		origin: "http://127.0.0.1:48111",
		body:   `{"apply_ignores":true}`,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("forget apply_ignores: esperado 200, obtenido %d (%s)", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decodificar forget: %v", err)
	}
	if resp.Forgotten != 2 {
		t.Fatalf("esperado forgotten=2 (config + log), obtenido %+v", resp)
	}
	files, _ := st.ListFiles()
	if len(files) != 0 {
		t.Fatalf("el store debería quedar vacío, quedan %d", len(files))
	}
}

func TestProtect(t *testing.T) {
	srv, st, base := newTestServer(t)
	ts := time.Date(2026, 7, 13, 10, 0, 0, 0, time.Local)

	// Un fichero ya versionado y dos existentes sin historial.
	writeVersion(t, st, base, "docs/tracked.txt", "v1\n", ts)
	for rel, content := range map[string]string{
		"docs/legacy.txt": "contenido previo a la instalación\n",
		"notas.md":        "notas\n",
	} {
		abs := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rec := do(t, srv, "POST", "/api/protect", reqOpts{
		token:  srv.Token(),
		origin: "http://127.0.0.1:48111",
		body:   `{"path":"` + strings.ReplaceAll(base, `\`, `\\`) + `"}`,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("protect: esperado 200, obtenido %d (%s)", rec.Code, rec.Body.String())
	}
	var resp protectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decodificar protect: %v", err)
	}
	if resp.Protected != 2 || resp.Existing != 1 || resp.Failed != 0 {
		t.Fatalf("esperado protected=2 existing=1 failed=0, obtenido %+v", resp)
	}
	// Las baselines existen y conservan el contenido en disco.
	vs, err := st.ListVersions("docs/legacy.txt")
	if err != nil || len(vs) != 1 {
		t.Fatalf("docs/legacy.txt: esperada 1 versión, obtenidas %d (err=%v)", len(vs), err)
	}
	data, err := st.VersionContent("docs/legacy.txt", vs[0].Name)
	if err != nil || string(data) != "contenido previo a la instalación\n" {
		t.Fatalf("baseline de docs/legacy.txt incorrecta: %q (err=%v)", data, err)
	}
	// El ya versionado no gana versiones nuevas.
	if vs, _ := st.ListVersions("docs/tracked.txt"); len(vs) != 1 {
		t.Fatalf("docs/tracked.txt: esperada 1 versión, obtenidas %d", len(vs))
	}

	// Ruta fuera de la carpeta personal → 400.
	outside := t.TempDir()
	rec = do(t, srv, "POST", "/api/protect", reqOpts{
		token:  srv.Token(),
		origin: "http://127.0.0.1:48111",
		body:   `{"path":"` + strings.ReplaceAll(outside, `\`, `\\`) + `"}`,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("protect fuera de base: esperado 400, obtenido %d", rec.Code)
	}

	// Ruta inexistente → 400.
	rec = do(t, srv, "POST", "/api/protect", reqOpts{
		token:  srv.Token(),
		origin: "http://127.0.0.1:48111",
		body:   `{"path":"no-existe-esta-carpeta"}`,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("protect inexistente: esperado 400, obtenido %d", rec.Code)
	}

	// Sin origen local legítimo → 403.
	rec = do(t, srv, "POST", "/api/protect", reqOpts{
		token:  srv.Token(),
		origin: "http://evil.example",
		body:   `{"path":"~"}`,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("protect con origen externo: esperado 403, obtenido %d", rec.Code)
	}
}
