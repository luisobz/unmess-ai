// Package agents es la fuente de verdad sobre qué agentes de IA conoce unmessai
// y cómo reconocer, a partir de una ruta relativa al home, si un fichero
// versionado pertenece a uno de ellos. La UI (modo Agente y el filtro de IA de
// la lista) y las operaciones server-side (sesiones, revert) consumen este
// registro para no duplicar los patrones en varios sitios.
package agents

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

// Agent describe un agente de IA conocido. Los campos exportados se serializan a
// la UI; dirs/files son los patrones internos de reconocimiento.
type Agent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"` // emoji por defecto; la UI puede sustituirlo por su propio icono

	// dirs son prefijos de directorio relativos al home (con "/" final): todo lo
	// que cuelgue de ellos pertenece al agente. files son ficheros exactos.
	dirs  []string
	files []string
}

// registry es la tabla estática de agentes soportados. Conservadora a
// propósito: solo directorios/ficheros que son inequívocamente de un agente,
// para no contaminar la traza con ruido no-IA (p. ej. ~/.vscode son extensiones
// del editor, no actividad de agente, y queda fuera aposta).
var registry = []Agent{
	{ID: "claude", Name: "Claude Code", Icon: "✳️", dirs: []string{".claude/"}, files: []string{".claude.json", ".claude.json.backup"}},
	{ID: "codex", Name: "Codex", Icon: "⬢", dirs: []string{".codex/"}},
	{ID: "gemini", Name: "Gemini", Icon: "♊", dirs: []string{".gemini/"}},
	{ID: "copilot", Name: "GitHub Copilot", Icon: "🐙", dirs: []string{".copilot/"}},
	{ID: "continue", Name: "Continue", Icon: "⏩", dirs: []string{".continue/"}},
	{ID: "commandcode", Name: "CommandCode", Icon: "⌘", dirs: []string{".commandcode/"}},
	{ID: "cursor", Name: "Cursor", Icon: "▱", dirs: []string{".cursor/"}},
}

// Registry devuelve la lista de agentes conocidos (copia para no exponer el
// estado interno).
func Registry() []Agent {
	out := make([]Agent, len(registry))
	copy(out, registry)
	return out
}

// normalize deja la ruta con separadores "/" y sin "./" inicial, para comparar
// igual en cualquier plataforma (las rutas del store son relativas al home).
func normalize(rel string) string {
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(rel, "./")
	return rel
}

// Detect devuelve el ID del agente al que pertenece la ruta relativa, o
// ("", false) si no pertenece a ninguno conocido.
func Detect(rel string) (string, bool) {
	rel = normalize(rel)
	for _, a := range registry {
		for _, f := range a.files {
			if rel == f {
				return a.ID, true
			}
		}
		for _, d := range a.dirs {
			if strings.HasPrefix(rel, d) {
				return a.ID, true
			}
		}
	}
	return "", false
}

// IsAI indica si la ruta pertenece a algún agente conocido.
func IsAI(rel string) bool {
	_, ok := Detect(rel)
	return ok
}

// CountByID cuenta cuántas de las rutas dadas pertenecen a cada agente. Solo
// incluye en el mapa los agentes con al menos una coincidencia.
func CountByID(rels []string) map[string]int {
	counts := make(map[string]int)
	for _, rel := range rels {
		if id, ok := Detect(rel); ok {
			counts[id]++
		}
	}
	return counts
}

// Known indica si el ID corresponde a un agente del registro.
func Known(id string) bool {
	for _, a := range registry {
		if a.ID == id {
			return true
		}
	}
	return false
}

// Transcript indica si rel es un fichero de transcript de sesión nativo de un
// agente conocido (un fichero = una sesión) y devuelve el agente y el
// "proyecto" que agrupa la sesión (vacío si el agente no agrupa por proyecto).
//
// Formatos soportados (verificados sobre instalaciones reales):
//   - Claude Code:  .claude/projects/<proyecto>/<uuid>.jsonl
//   - CommandCode:  .commandcode/projects/<proyecto>/<uuid>.jsonl
//     (cada sesión lleva sidecars <uuid>.checkpoints.jsonl y <uuid>.meta.json
//     que NO son sesiones y quedan excluidos)
//   - Codex:        .codex/sessions/**/rollout-*.jsonl (anidado por fecha)
//   - Gemini (Antigravity): .gemini/antigravity/conversations/<uuid>.pb
func Transcript(rel string) (agentID, project string, ok bool) {
	rel = normalize(rel)
	parts := strings.Split(rel, "/")
	if strings.HasSuffix(rel, ".pb") {
		if len(parts) == 4 && parts[0] == ".gemini" && parts[1] == "antigravity" && parts[2] == "conversations" {
			return "gemini", "", true
		}
		return "", "", false
	}
	if !strings.HasSuffix(rel, ".jsonl") || strings.HasSuffix(rel, ".checkpoints.jsonl") {
		return "", "", false
	}
	switch {
	case len(parts) == 4 && parts[0] == ".claude" && parts[1] == "projects":
		return "claude", strings.TrimPrefix(parts[2], "-"), true
	case len(parts) == 4 && parts[0] == ".commandcode" && parts[1] == "projects":
		return "commandcode", strings.TrimPrefix(parts[2], "-"), true
	case len(parts) >= 3 && parts[0] == ".codex" && parts[1] == "sessions":
		return "codex", "", true
	}
	return "", "", false
}

// TitleSidecar devuelve, si existe para este transcript, la ruta relativa del
// fichero hermano que contiene el título de la sesión y el extractor a usar:
//   - CommandCode: <uuid>.meta.json ({"title": "..."})            → "meta"
//   - Antigravity: brain/<uuid>/.system_generated/logs/overview.txt → "overview"
//
// ok=false si el agente no tiene sidecar de título (Claude/Codex extraen del
// propio transcript).
func TitleSidecar(rel string) (sidecarRel, kind string, ok bool) {
	rel = normalize(rel)
	if strings.HasPrefix(rel, ".commandcode/projects/") && strings.HasSuffix(rel, ".jsonl") {
		return strings.TrimSuffix(rel, ".jsonl") + ".meta.json", "meta", true
	}
	if strings.HasPrefix(rel, ".gemini/antigravity/conversations/") && strings.HasSuffix(rel, ".pb") {
		uuid := strings.TrimSuffix(rel[strings.LastIndex(rel, "/")+1:], ".pb")
		return ".gemini/antigravity/brain/" + uuid + "/.system_generated/logs/overview.txt", "overview", true
	}
	return "", "", false
}

// TitleFromSidecar extrae el título del contenido de un sidecar según su kind.
func TitleFromSidecar(kind string, data []byte) string {
	switch kind {
	case "meta":
		var meta struct {
			Title string `json:"title"`
		}
		if json.Unmarshal(data, &meta) == nil {
			return clipTitle(meta.Title)
		}
	case "overview":
		return titleFromOverview(data)
	}
	return ""
}

// titleFromOverview saca el título de un overview.txt de Antigravity: JSONL de
// steps; el primer USER_INPUT lleva en content la petición del usuario envuelta
// en <USER_REQUEST>…</USER_REQUEST>.
func titleFromOverview(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for i := 0; i < 200 && sc.Scan(); i++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var step struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		}
		if json.Unmarshal(line, &step) != nil || step.Type != "USER_INPUT" {
			continue
		}
		txt := step.Content
		if i := strings.Index(txt, "<USER_REQUEST>"); i >= 0 {
			txt = txt[i+len("<USER_REQUEST>"):]
			if j := strings.Index(txt, "</USER_REQUEST>"); j >= 0 {
				txt = txt[:j]
			}
		}
		if s := clipTitle(txt); s != "" {
			return s
		}
	}
	return ""
}

// TitleFromTranscript extrae un título legible de un transcript jsonl, con
// mejor esfuerzo y tolerancia a formatos: el primer mensaje del usuario o, en
// su defecto, el resumen que algunos agentes escriben al inicio del fichero.
// Devuelve "" si no se reconoce nada (el llamador pone un título de respaldo).
func TitleFromTranscript(data []byte) string {
	const maxLines = 200
	var summary string
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for i := 0; i < maxLines && sc.Scan(); i++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var obj map[string]any
		if json.Unmarshal(line, &obj) != nil {
			continue
		}
		if s, _ := obj["summary"].(string); s != "" && obj["type"] == "summary" && summary == "" {
			summary = s
		}
		if obj["type"] == "user" {
			if s := userText(obj["message"]); s != "" {
				return clipTitle(s)
			}
		}
		// Codex: {"type":"response_item","payload":{"type":"message","role":"user","content":[...]}}
		if payload, _ := obj["payload"].(map[string]any); payload != nil {
			if payload["role"] == "user" {
				if s := contentText(payload["content"]); s != "" {
					return clipTitle(s)
				}
			}
		}
	}
	return clipTitle(summary)
}

// userText extrae el texto de un message estilo Claude Code:
// {"role":"user","content": "str" | [{"type":"text","text":"..."}]}.
func userText(message any) string {
	return UserText(message)
}

// UserText extrae el texto legible del usuario de un objeto de mensaje de
// agente que tenga los campos role/content.
func UserText(message any) string {
	m, _ := message.(map[string]any)
	if m == nil {
		return ""
	}
	return contentText(m["content"])
}

// contentText extrae el primer texto útil de un content string o lista de
// bloques {text: "..."}. Descarta contenido inyectado por el sistema (tags
// <...>, avisos "Caveat:") que no describe la intención de la sesión.
func contentText(content any) string {
	switch c := content.(type) {
	case string:
		return usefulText(c)
	case []any:
		for _, blk := range c {
			b, _ := blk.(map[string]any)
			if b == nil {
				continue
			}
			if s, _ := b["text"].(string); s != "" {
				if u := usefulText(s); u != "" {
					return u
				}
			}
		}
	}
	return ""
}

func usefulText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "<") || strings.HasPrefix(s, "Caveat:") {
		return ""
	}
	return s
}

// clipTitle colapsa el whitespace y recorta a una longitud de título.
func clipTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 80
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}
