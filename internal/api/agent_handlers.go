package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/luisobz/unmess-ai/internal/agents"
	"github.com/luisobz/unmess-ai/internal/daemon"
	"github.com/luisobz/unmess-ai/internal/store"
)

// El modo Agente de la UI reconstruye "sesiones" de un agente de IA a partir
// del propio store, sin estado adicional:
//
//   - Sesiones nativas: agentes que escriben un transcript por sesión (Claude,
//     CommandCode, Codex). Cada transcript versionado es una sesión, con
//     ventana temporal [primera versión, última versión] del transcript.
//   - Actividad agrupada (fallback): agentes sin transcript reconocible. Las
//     versiones de sus ficheros se agrupan en ventanas separadas por huecos de
//     más de clusterGap.
//
// La correlación de cambios con la ventana es aproximada (granularidad de
// minuto del store + debounce del daemon) y la UI lo comunica; el revert usa
// siempre la versión estrictamente anterior al inicio de la ventana y nunca
// borra ficheros.

const clusterGap = 30 * time.Minute

// transcriptHeadLimit acota cuánto se lee de un transcript para extraer título.
const transcriptHeadLimit = 128 * 1024

// sessionInfo describe una sesión en las respuestas JSON. Changes y
// WorkspaceFiles cuentan solo ficheros del usuario: el estado interno de los
// agentes se usa únicamente para derivar sesiones y ventanas, nunca se enseña.
type sessionInfo struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"` // "native" | "cluster"
	Title             string `json:"title,omitempty"`
	Project           string `json:"project,omitempty"`
	Transcript        string `json:"transcript,omitempty"`
	TranscriptVersion string `json:"transcript_version,omitempty"`
	TranscriptDeleted bool   `json:"transcript_deleted,omitempty"`
	Start             string `json:"start"`
	End               string `json:"end"`
	Changes           int    `json:"changes"`
	WorkspaceFiles    int    `json:"workspace_files"`
}

// sessionEvent es una versión de un fichero del usuario escrita dentro de la
// ventana de la sesión.
type sessionEvent struct {
	TS      string `json:"ts"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Prev    string `json:"prev,omitempty"` // versión anterior del fichero ("" = primera conocida)
	First   bool   `json:"first"`          // primera versión conocida del fichero (creación probable)
	Prompt  string `json:"prompt,omitempty"` // texto del prompt del usuario al que pertenece esta versión
}

// sessionFile resume un fichero del usuario afectado por la sesión.
type sessionFile struct {
	Path     string `json:"path"`
	Versions int    `json:"versions"`
	Created  bool   `json:"created"`
	Deleted  bool   `json:"deleted"`
	Prior    string `json:"prior,omitempty"` // versión previa a la sesión ("" = sin estado anterior)
}

// sessionSpan es la representación interna de una sesión.
type sessionSpan struct {
	info  sessionInfo
	start time.Time
	end   time.Time
}

// buildSessions deriva las sesiones del agente a partir del estado del store.
func (s *Server) buildSessions(agentID string, files []store.FileInfo) []sessionSpan {
	var spans []sessionSpan

	// Sesiones nativas: un transcript versionado = una sesión.
	for _, f := range files {
		aid, project, ok := agents.Transcript(f.RelPath)
		if !ok || aid != agentID || len(f.Versions) == 0 {
			continue
		}
		newest, oldest := f.Versions[0], f.Versions[len(f.Versions)-1]
		spans = append(spans, sessionSpan{
			info: sessionInfo{
				ID:                f.RelPath,
				Kind:              "native",
				Title:             s.transcriptTitle(f),
				Project:           project,
				Transcript:        f.RelPath,
				TranscriptVersion: newest.Name,
				TranscriptDeleted: !fileExists(s.store.OriginalPath(f.RelPath)),
				Start:             oldest.TS.Format(time.RFC3339),
				End:               newest.TS.Format(time.RFC3339),
			},
			start: oldest.TS,
			end:   newest.TS,
		})
	}

	// Fallback: sin transcripts, agrupar la actividad del agente por huecos
	// temporales. No es una sesión real y la UI lo etiqueta como tal.
	if len(spans) == 0 {
		var stamps []time.Time
		for _, f := range files {
			if id, ok := agents.Detect(f.RelPath); !ok || id != agentID {
				continue
			}
			for _, v := range f.Versions {
				stamps = append(stamps, v.TS)
			}
		}
		sort.Slice(stamps, func(i, j int) bool { return stamps[i].Before(stamps[j]) })
		for i := 0; i < len(stamps); {
			j := i
			for j+1 < len(stamps) && stamps[j+1].Sub(stamps[j]) <= clusterGap {
				j++
			}
			start, end := stamps[i], stamps[j]
			spans = append(spans, sessionSpan{
				info: sessionInfo{
					ID:    "cluster:" + start.Format(time.RFC3339),
					Kind:  "cluster",
					Start: start.Format(time.RFC3339),
					End:   end.Format(time.RFC3339),
				},
				start: start,
				end:   end,
			})
			i = j + 1
		}
	}

	// Rellenar contadores de cambios y ordenar: más recientes primero.
	for i := range spans {
		events, sfiles := s.sessionTrace(spans[i], files)
		spans[i].info.Changes = len(events)
		spans[i].info.WorkspaceFiles = len(sfiles)
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].end.After(spans[j].end) })
	return spans
}

// transcriptTitle extrae el título de la sesión. Prioridad: el sidecar de
// título si el agente lo tiene (meta.json de CommandCode, overview.txt de
// Antigravity), leído del disco o de su última versión en el store; si no, el
// propio transcript. Mejor esfuerzo: "" si no hay nada.
func (s *Server) transcriptTitle(f store.FileInfo) string {
	if sidecarRel, kind, ok := agents.TitleSidecar(f.RelPath); ok {
		if data, err := readHead(s.store.OriginalPath(sidecarRel), transcriptHeadLimit); err == nil {
			if title := agents.TitleFromSidecar(kind, data); title != "" {
				return title
			}
		}
		if vs, err := s.store.ListVersions(sidecarRel); err == nil && len(vs) > 0 {
			if data, verr := s.store.VersionContent(sidecarRel, vs[0].Name); verr == nil {
				if title := agents.TitleFromSidecar(kind, data); title != "" {
					return title
				}
			}
		}
	}
	if data, err := readHead(s.store.OriginalPath(f.RelPath), transcriptHeadLimit); err == nil {
		if title := agents.TitleFromTranscript(data); title != "" {
			return title
		}
	}
	if len(f.Versions) > 0 {
		if data, err := s.store.VersionContent(f.RelPath, f.Versions[0].Name); err == nil {
			if len(data) > transcriptHeadLimit {
				data = data[:transcriptHeadLimit]
			}
			return agents.TitleFromTranscript(data)
		}
	}
	return ""
}

// readHead lee como máximo limit bytes del fichero.
func readHead(path string, limit int64) ([]byte, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return io.ReadAll(io.LimitReader(fh, limit))
}

// sessionTrace calcula la traza de una sesión: cada versión de un fichero del
// usuario escrita dentro de la ventana (evento) y el resumen por fichero. El
// estado interno de los agentes (transcripts, configs, cachés) queda fuera:
// solo sirve para derivar sesiones y ventanas, al usuario le interesan sus
// propios ficheros. También filtra ficheros de sistema (p. ej. .npm, .local,
// snap) ajenos al proyecto, usando el prefijo de ruta dominante entre los
// ficheros del workspace.
func (s *Server) sessionTrace(span sessionSpan, files []store.FileInfo) ([]sessionEvent, []sessionFile) {
	prefix := workspacePrefix(files, span.start, span.end)

	var events []sessionEvent
	var sfiles []sessionFile
	for _, f := range files {
		if _, ok := agents.Detect(f.RelPath); ok {
			continue
		}
		if prefix != "" && !strings.HasPrefix(f.RelPath, prefix) {
			continue
		}

		inWindow := 0
		created := false
		var prior string
		// Versions viene en orden descendente: el prev de la versión i es i+1.
		for i, v := range f.Versions {
			if v.TS.Before(span.start) {
				if prior == "" {
					prior = v.Name
				}
				continue
			}
			if v.TS.After(span.end) {
				continue
			}
			inWindow++
			first := i == len(f.Versions)-1
			prev := ""
			if !first {
				prev = f.Versions[i+1].Name
			}
			if first {
				created = true
			}
			events = append(events, sessionEvent{
				TS:      v.TS.Format(time.RFC3339),
				Path:    f.RelPath,
				Version: v.Name,
				Prev:    prev,
				First:   first,
			})
		}
		if inWindow == 0 {
			continue
		}
		deleted := !fileExists(s.store.OriginalPath(f.RelPath)) &&
			!f.Versions[0].TS.After(span.end)
		sfiles = append(sfiles, sessionFile{
			Path:     f.RelPath,
			Versions: inWindow,
			Created:  created,
			Deleted:  deleted,
			Prior:    prior,
		})
	}
	// Cronología: más reciente primero (como la muestra la UI).
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].TS == events[j].TS {
			return events[i].Path < events[j].Path
		}
		return events[i].TS > events[j].TS
	})
	sort.SliceStable(sfiles, func(i, j int) bool { return sfiles[i].Path < sfiles[j].Path })
	return events, sfiles
}

// workspacePrefix encuentra el prefijo de ruta dominante entre los ficheros
// del usuario afectados por la ventana. Agrupa por el primer componente de ruta
// y devuelve el que más ficheros cubre, siempre que supere la mayoría absoluta.
// Así los ficheros de sistema sueltos (.npm, .local, snap) quedan filtrados
// mientras que los del proyecto se conservan. Vacío = sin filtro.
func workspacePrefix(files []store.FileInfo, start, end time.Time) string {
	var paths []string
	for _, f := range files {
		if _, ok := agents.Detect(f.RelPath); ok {
			continue
		}
		for _, v := range f.Versions {
			if !v.TS.Before(start) && !v.TS.After(end) {
				paths = append(paths, f.RelPath)
				break
			}
		}
	}
	if len(paths) < 2 {
		return ""
	}
	counts := make(map[string]int)
	for _, p := range paths {
		counts[strings.SplitN(p, "/", 2)[0]]++
	}
	var best string
	bestCount := 0
	for prefix, count := range counts {
		if count > bestCount {
			bestCount = count
			best = prefix
		}
	}
	if bestCount > len(paths)/2 {
		return best + "/"
	}
	return ""
}

// promptEntry es un prompt de usuario extraído del transcript con su timestamp.
type promptEntry struct {
	ts   time.Time
	text string
}

// assignPrompts asigna a cada evento el texto del prompt del usuario que lo
// precede inmediatamente según el timestamp del prompt. Lee el transcript de la
// sesión (del disco, o del store si fue borrado) y extrae las entradas del
// usuario (role=user). Los eventos quedan ordenados como estaban pero cada uno
// lleva el texto del prompt que lo causó en el campo Prompt.
func assignPrompts(span sessionSpan, events []sessionEvent, srv *Server) {
	transcript := span.info.Transcript
	if transcript == "" {
		return
	}
	prompts, err := extractPrompts(transcript, srv)
	if err != nil || len(prompts) == 0 {
		return
	}
	for i := range events {
		ts, err := time.Parse(time.RFC3339, events[i].TS)
		if err != nil {
			continue
		}
		var best promptEntry
		for _, p := range prompts {
			if !p.ts.After(ts) && (best.text == "" || p.ts.After(best.ts)) {
				best = p
			}
		}
		if best.text != "" {
			events[i].Prompt = clipPromptText(best.text)
		}
	}
}

// extractPrompts lee el transcript y extrae las entradas de usuario con sus
// timestamps. Soporta los formatos de CommandCode/Claude (type=user) y Codex
// (payload.role=user).
func extractPrompts(transcript string, srv *Server) ([]promptEntry, error) {
	abs := srv.store.OriginalPath(transcript)
	raw, err := readHead(abs, transcriptHeadLimit)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		vers, verr := srv.store.ListVersions(transcript)
		if verr != nil || len(vers) == 0 {
			return nil, fmt.Errorf("transcript no encontrado")
		}
		raw, err = srv.store.VersionContent(transcript, vers[0].Name)
		if err != nil {
			return nil, err
		}
	}
	return parsePrompts(raw), nil
}

// parsePrompts escanea el transcript JSONL en busca de mensajes del usuario.
func parsePrompts(data []byte) []promptEntry {
	var entries []promptEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for i := 0; i < 500 && sc.Scan(); i++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var obj map[string]any
		if json.Unmarshal(line, &obj) != nil {
			continue
		}
		var ts time.Time
		var text string
		if obj["type"] == "user" {
			if s := agents.UserText(obj["message"]); s != "" {
				text = s
			}
		}
		if payload, _ := obj["payload"].(map[string]any); payload != nil && payload["role"] == "user" {
			if s := agents.UserText(payload); s != "" {
				text = s
			}
		}
		if text == "" {
			continue
		}
		if t, _ := obj["timestamp"].(string); t != "" {
			ts, _ = time.Parse(time.RFC3339, t)
		}
		if ts.IsZero() {
			if t, _ := obj["ts"].(string); t != "" {
				ts, _ = time.Parse(time.RFC3339, t)
			}
		}
		entries = append(entries, promptEntry{ts: ts, text: text})
	}
	return entries
}

// clipPromptText colapsa y recorta el texto de un prompt a una línea.
func clipPromptText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 120
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

// handleAgentSessions lista las sesiones detectadas de un agente.
// GET /api/agent/sessions?agent=<id>
func (s *Server) handleAgentSessions(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent")
	if !agents.Known(agentID) {
		writeErr(w, http.StatusBadRequest, "agente desconocido")
		return
	}
	files, err := s.store.ListFiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	spans := s.buildSessions(agentID, files)
	out := make([]sessionInfo, 0, len(spans))
	for _, sp := range spans {
		out = append(out, sp.info)
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent": agentID, "sessions": out})
}

// handleAgentSession devuelve el detalle (traza) de una sesión.
// GET /api/agent/session?agent=<id>&id=<session-id>
func (s *Server) handleAgentSession(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent")
	if !agents.Known(agentID) {
		writeErr(w, http.StatusBadRequest, "agente desconocido")
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "falta id de sesión")
		return
	}
	files, err := s.store.ListFiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, sp := range s.buildSessions(agentID, files) {
		if sp.info.ID != id {
			continue
		}
		events, sfiles := s.sessionTrace(sp, files)
		assignPrompts(sp, events, s)
		writeJSON(w, http.StatusOK, map[string]any{
			"session": sp.info,
			"events":  events,
			"files":   sfiles,
		})
		return
	}
	writeErr(w, http.StatusNotFound, "sesión no encontrada")
}

// agentRevertRequest es el cuerpo de POST /api/agent/revert. Deshace los
// cambios de una ventana de sesión sobre los ficheros del espacio de trabajo
// (nunca sobre el estado interno de agentes). Target acota a un fichero o
// carpeta (prefijo relativo al home); vacío = todos los afectados.
type agentRevertRequest struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Target string `json:"target,omitempty"`
	DryRun bool   `json:"dry_run"`
}

// revertItem es la decisión (y resultado) por fichero.
type revertItem struct {
	Path   string `json:"path"`
	Action string `json:"action"` // revert | restore_deleted | skip_no_prior | skip_unchanged
	Prior  string `json:"prior,omitempty"`
	Safety string `json:"safety_version,omitempty"`
	Error  string `json:"error,omitempty"`
}

type revertResponse struct {
	DryRun           bool         `json:"dry_run"`
	Items            []revertItem `json:"items"`
	Reverted         int          `json:"reverted"`
	RestoredDeleted  int          `json:"restored_deleted"`
	SkippedNoPrior   int          `json:"skipped_no_prior"`
	SkippedUnchanged int          `json:"skipped_unchanged"`
	Failed           int          `json:"failed"`
}

// handleAgentRevert planifica (dry_run) o ejecuta la reversión de los cambios
// de una ventana de sesión. Por cada fichero del workspace con versiones en la
// ventana restaura la versión estrictamente anterior al inicio; los ficheros
// sin estado anterior se conservan siempre (nunca se borra nada). Cada
// restauración crea una safety version, así que la operación entera se puede
// deshacer. Muta estado, así que exige origen local.
func (s *Server) handleAgentRevert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.requireLocal(w, r) {
		return
	}
	var req agentRevertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo JSON inválido")
		return
	}
	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "start inválido (RFC3339)")
		return
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "end inválido (RFC3339)")
		return
	}
	if end.Before(start) {
		writeErr(w, http.StatusBadRequest, "ventana invertida (end < start)")
		return
	}
	target := ""
	if strings.TrimSpace(req.Target) != "" {
		target, err = cleanRelPath(req.Target)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	files, err := s.store.ListFiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	prefix := workspacePrefix(files, start, end)

	resp := revertResponse{DryRun: req.DryRun, Items: []revertItem{}}
	for _, f := range files {
		if _, ok := agents.Detect(f.RelPath); ok {
			continue // estado interno de agentes: fuera del revert
		}
		if prefix != "" && !strings.HasPrefix(f.RelPath, prefix) {
			continue
		}
		if target != "" && f.RelPath != target && !strings.HasPrefix(f.RelPath, target+"/") {
			continue
		}
		item, affected := classifyRevert(s.store, f, start, end)
		if !affected {
			continue
		}
		switch item.Action {
		case "skip_no_prior":
			resp.SkippedNoPrior++
		case "skip_unchanged":
			resp.SkippedUnchanged++
		case "revert", "restore_deleted":
			if !req.DryRun {
				safety, rerr := s.store.Restore(f.RelPath, item.Prior, time.Now())
				if rerr != nil {
					item.Error = rerr.Error()
					resp.Failed++
					resp.Items = append(resp.Items, item)
					continue
				}
				item.Safety = safety
			}
			if item.Action == "revert" {
				resp.Reverted++
			} else {
				resp.RestoredDeleted++
			}
		}
		resp.Items = append(resp.Items, item)
	}
	slices.SortStableFunc(resp.Items, func(a, b revertItem) int {
		return strings.Compare(a.Path, b.Path)
	})

	if !req.DryRun && resp.Reverted+resp.RestoredDeleted > 0 && s.rt.Events != nil {
		s.rt.Events.Publish(daemon.Event{
			Type:    daemon.EventRestored,
			Path:    fmt.Sprintf("%d fichero(s)", resp.Reverted+resp.RestoredDeleted),
			Message: "Reversión de sesión completada",
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// classifyRevert decide qué hacer con un fichero para deshacer la ventana
// [start, end]. affected=false si el fichero no tiene versiones en la ventana.
func classifyRevert(st *store.Store, f store.FileInfo, start, end time.Time) (revertItem, bool) {
	item := revertItem{Path: f.RelPath}
	inWindow := false
	for _, v := range f.Versions {
		if !v.TS.Before(start) && !v.TS.After(end) {
			inWindow = true
		}
		// prior: la más reciente estrictamente anterior al inicio (orden desc).
		if item.Prior == "" && v.TS.Before(start) {
			item.Prior = v.Name
		}
	}
	if !inWindow {
		return item, false
	}
	if item.Prior == "" {
		// Creado en la sesión (o primer versionado dentro de ella): no hay
		// estado anterior que restaurar y nunca borramos, así que se conserva.
		item.Action = "skip_no_prior"
		return item, true
	}
	abs := st.OriginalPath(f.RelPath)
	fi, err := os.Lstat(abs)
	if err != nil {
		item.Action = "restore_deleted"
		return item, true
	}
	// Igualdad con la versión previa: primero por tamaño (barato), luego bytes.
	var priorSize int64 = -1
	for _, v := range f.Versions {
		if v.Name == item.Prior {
			priorSize = v.Size
			break
		}
	}
	if priorSize == fi.Size() {
		cur, cerr := os.ReadFile(abs)
		prev, perr := st.VersionContent(f.RelPath, item.Prior)
		if cerr == nil && perr == nil && bytes.Equal(cur, prev) {
			item.Action = "skip_unchanged"
			return item, true
		}
	}
	item.Action = "revert"
	return item, true
}
