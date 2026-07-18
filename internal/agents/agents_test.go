package agents

import (
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		rel     string
		wantID  string
		wantHit bool
	}{
		// Claude: directorio y ficheros exactos.
		{".claude/projects/foo/abc.jsonl", "claude", true},
		{".claude.json", "claude", true},
		{".claude.json.backup", "claude", true},
		// Otros agentes por directorio.
		{".codex/sessions/2026/07/rollout-1.jsonl", "codex", true},
		{".gemini/history", "gemini", true},
		{".copilot/state.json", "copilot", true},
		{".continue/config.json", "continue", true},
		{".commandcode/projects/x", "commandcode", true},
		{".cursor/logs/a.log", "cursor", true},
		// Separadores Windows normalizados.
		{`.claude\projects\foo\abc.jsonl`, "claude", true},
		// No agentes / falsos positivos evitados.
		{"Documentos/informe.md", "", false},
		{".vscode/extensions/x", "", false},    // editor, no agente (fuera aposta)
		{".clauderc", "", false},               // no debe casar con el prefijo ".claude"
		{"proyecto/.claudecopia/x", "", false}, // ".claude/" no es prefijo de esta ruta
	}
	for _, c := range cases {
		id, ok := Detect(c.rel)
		if ok != c.wantHit || id != c.wantID {
			t.Errorf("Detect(%q) = (%q, %v); quería (%q, %v)", c.rel, id, ok, c.wantID, c.wantHit)
		}
	}
}

func TestCountByID(t *testing.T) {
	rels := []string{
		".claude/a", ".claude/b", ".claude.json",
		".gemini/x",
		"Documentos/informe.md",
	}
	counts := CountByID(rels)
	if counts["claude"] != 3 {
		t.Errorf("claude = %d; quería 3", counts["claude"])
	}
	if counts["gemini"] != 1 {
		t.Errorf("gemini = %d; quería 1", counts["gemini"])
	}
	if _, ok := counts["cursor"]; ok {
		t.Errorf("cursor no debería aparecer sin coincidencias")
	}
}

func TestTranscript(t *testing.T) {
	cases := []struct {
		rel     string
		agentID string
		project string
		ok      bool
	}{
		{".claude/projects/-home-luis-Proyectos-x/abc-123.jsonl", "claude", "home-luis-Proyectos-x", true},
		{".commandcode/projects/home-luis-app/def.jsonl", "commandcode", "home-luis-app", true},
		{".codex/sessions/2026/07/16/rollout-2026-07-16T10-00-00-x.jsonl", "codex", "", true},
		// Antigravity: una conversación .pb = una sesión de Gemini.
		{".gemini/antigravity/conversations/dce839df-8c64.pb", "gemini", "", true},
		// Sidecars de CommandCode: NO son sesiones (causaban duplicados).
		{".commandcode/projects/home-luis-app/def.checkpoints.jsonl", "", "", false},
		{".commandcode/projects/home-luis-app/def.meta.json", "", "", false},
		// No transcripts: config del agente, otras extensiones, profundidad rara.
		{".claude.json", "", "", false},
		{".claude/settings.json", "", "", false},
		{".claude/projects/x/sub/abc.jsonl", "", "", false},
		{".gemini/history.jsonl", "", "", false},
		{".gemini/antigravity/brain/x/overview.pb", "", "", false},
	}
	for _, c := range cases {
		id, proj, ok := Transcript(c.rel)
		if id != c.agentID || proj != c.project || ok != c.ok {
			t.Errorf("Transcript(%q) = (%q, %q, %v); quería (%q, %q, %v)",
				c.rel, id, proj, ok, c.agentID, c.project, c.ok)
		}
	}
}

func TestTitleFromTranscript(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{
			name: "primer mensaje de usuario (content string)",
			data: `{"type":"user","message":{"role":"user","content":"arregla el bug del login"}}` + "\n",
			want: "arregla el bug del login",
		},
		{
			name: "content en bloques, saltando tags del sistema",
			data: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"<system-reminder>x</system-reminder>"},{"type":"text","text":"refactor del módulo de pagos"}]}}` + "\n",
			want: "refactor del módulo de pagos",
		},
		{
			name: "caveat descartado, summary como respaldo",
			data: `{"type":"summary","summary":"Sesión sobre pagos","leafUuid":"x"}` + "\n" +
				`{"type":"user","message":{"role":"user","content":"Caveat: the messages below..."}}` + "\n",
			want: "Sesión sobre pagos",
		},
		{
			name: "codex payload",
			data: `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"añade tests"}]}}` + "\n",
			want: "añade tests",
		},
		{
			name: "línea corrupta tolerada",
			data: "no-json\n{rotura\n" + `{"type":"user","message":{"content":"hola"}}` + "\n",
			want: "hola",
		},
		{
			name: "nada reconocible",
			data: `{"type":"assistant","message":{"content":"respuesta"}}` + "\n",
			want: "",
		},
	}
	for _, c := range cases {
		if got := TitleFromTranscript([]byte(c.data)); got != c.want {
			t.Errorf("%s: TitleFromTranscript = %q; quería %q", c.name, got, c.want)
		}
	}
	// Título largo recortado a 80 runas + elipsis.
	long := strings.Repeat("palabra ", 30)
	got := TitleFromTranscript([]byte(`{"type":"user","message":{"content":"` + long + `"}}`))
	if len([]rune(got)) > 82 || !strings.HasSuffix(got, "…") {
		t.Errorf("título largo mal recortado: %q (%d runas)", got, len([]rune(got)))
	}
}

func TestTitleSidecar(t *testing.T) {
	// CommandCode: meta.json hermano del transcript.
	rel, kind, ok := TitleSidecar(".commandcode/projects/home-luis-app/0979f59e.jsonl")
	if !ok || kind != "meta" || rel != ".commandcode/projects/home-luis-app/0979f59e.meta.json" {
		t.Errorf("commandcode sidecar = (%q, %q, %v)", rel, kind, ok)
	}
	// Antigravity: overview.txt en el brain con el mismo uuid.
	rel, kind, ok = TitleSidecar(".gemini/antigravity/conversations/dce839df-8c64.pb")
	if !ok || kind != "overview" ||
		rel != ".gemini/antigravity/brain/dce839df-8c64/.system_generated/logs/overview.txt" {
		t.Errorf("antigravity sidecar = (%q, %q, %v)", rel, kind, ok)
	}
	// Claude/Codex: sin sidecar (título desde el propio transcript).
	if _, _, ok := TitleSidecar(".claude/projects/-p/abc.jsonl"); ok {
		t.Error("claude no debería tener sidecar")
	}
}

func TestTitleFromSidecar(t *testing.T) {
	if got := TitleFromSidecar("meta", []byte(`{"title":"Add creator signature to footer"}`)); got != "Add creator signature to footer" {
		t.Errorf("meta = %q", got)
	}
	if got := TitleFromSidecar("meta", []byte("no json")); got != "" {
		t.Errorf("meta corrupto = %q; quería vacío", got)
	}
	overview := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","content":"<USER_REQUEST>\nañade un test basico a cada app\n</USER_REQUEST>\n<ADDITIONAL_METADATA>ruido</ADDITIONAL_METADATA>"}` + "\n" +
		`{"step_index":1,"type":"RUN_COMMAND","content":"git push"}` + "\n"
	if got := TitleFromSidecar("overview", []byte(overview)); got != "añade un test basico a cada app" {
		t.Errorf("overview = %q", got)
	}
	// Sin USER_INPUT reconocible → vacío (el llamador pone respaldo).
	if got := TitleFromSidecar("overview", []byte(`{"type":"RUN_COMMAND","content":"x"}`)); got != "" {
		t.Errorf("overview sin user input = %q", got)
	}
}

func TestRegistryIsCopy(t *testing.T) {
	r := Registry()
	if len(r) == 0 {
		t.Fatal("registry vacío")
	}
	r[0].Name = "MUTADO"
	if Registry()[0].Name == "MUTADO" {
		t.Error("Registry() expone el estado interno: mutación filtrada")
	}
}
