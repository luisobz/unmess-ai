package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
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
// segundo de las versiones históricas + debounce del daemon) y la UI lo comunica; el revert usa
// siempre la versión estrictamente anterior al inicio de la ventana y nunca
// borra ficheros.

const clusterGap = 30 * time.Minute

// transcriptHeadLimit acota cuánto se lee de un transcript para extraer título.
const transcriptHeadLimit = 128 * 1024
const transcriptReadLimit = 16 * 1024 * 1024

// sessionInfo describe una sesión en las respuestas JSON. Changes y
// WorkspaceFiles cuentan solo ficheros del usuario: el estado interno de los
// agentes se usa únicamente para derivar sesiones y ventanas, nunca se enseña.
type sessionInfo struct {
	ID                string `json:"id"`
	Workspace         string `json:"workspace,omitempty"`
	Scope             string `json:"scope"`       // workspace | unknown | outside_base
	TimeSource        string `json:"time_source"` // transcript | backups
	Correlation       string `json:"correlation"`
	Partial           bool   `json:"partial"`
	ConfirmedFiles    int    `json:"confirmed_files"`
	CandidateFiles    int    `json:"candidate_files"`
	InvalidLines      int    `json:"invalid_lines"`
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
	Prev    string `json:"prev,omitempty"`   // versión anterior del fichero ("" = primera conocida)
	First   bool   `json:"first"`            // primera copia conocida; no demuestra creación
	Prompt  string `json:"prompt,omitempty"` // texto del prompt del usuario al que pertenece esta versión
}

// sessionFile resume un fichero del usuario afectado por la sesión.
type sessionReference struct {
	Agent string `json:"agent"`
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
}

type sessionFile struct {
	SharedWith  []sessionReference     `json:"shared_with,omitempty"`
	Path        string                 `json:"path"`
	Attribution string                 `json:"attribution"`
	Evidence    []agents.WriteEvidence `json:"evidence,omitempty"`
	BackedUp    bool                   `json:"backed_up"`
	Versions    int                    `json:"versions"`
	Created     bool                   `json:"created"`
	Deleted     bool                   `json:"deleted"`
	Prior       string                 `json:"prior,omitempty"` // versión previa a la sesión ("" = sin estado anterior)
}

// sessionSpan es la representación interna de una sesión.
type sessionSpan struct {
	info    sessionInfo
	start   time.Time
	end     time.Time
	prompts []agents.Prompt
	writes  map[string][]agents.WriteEvidence
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
		metadata := agents.TranscriptData{}
		partial := false
		if strings.HasSuffix(f.RelPath, ".jsonl") {
			if data, err := readHead(newest.Path, transcriptReadLimit+1); err == nil {
				partial = len(data) > transcriptReadLimit
				if partial {
					data = data[:transcriptReadLimit]
				}
				metadata = agents.ParseTranscript(data)
			} else {
				partial = true
			}
		}
		start, end := oldest.TS, newest.TS
		timeSource := "backups"
		if !metadata.Start.IsZero() && !metadata.End.IsZero() {
			start, end = metadata.Start, metadata.End
			// Include the bounded delay between a transcript event and its
			// stored file versions, without using a later baseline as activity.
			delay := time.Duration(s.currentConfig().DebounceSeconds) * 5 * time.Second
			if delay < 0 {
				delay = 0
			}
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
			end = end.Add(delay)
			timeSource = "transcript"
		}
		workspace, scope := "", "unknown"
		if metadata.CWD != "" {
			if filepath.IsAbs(metadata.CWD) {
				if rel, err := s.store.RelPath(metadata.CWD); err == nil {
					workspace, scope = rel, "workspace"
				} else {
					scope = "outside_base"
				}
			}
		}
		title := metadata.Title
		if title == "" {
			title = s.transcriptTitle(f)
		}
		if scope == "workspace" {
			project = workspace
		}
		spans = append(spans, sessionSpan{
			prompts: metadata.Prompts,
			writes:  s.relativeWrites(metadata.Writes),
			info: sessionInfo{
				Workspace: workspace, Scope: scope, TimeSource: timeSource,
				Correlation: "temporal", Partial: partial, InvalidLines: metadata.InvalidLines,
				ID:                f.RelPath,
				Kind:              "native",
				Title:             title,
				Project:           project,
				Transcript:        f.RelPath,
				TranscriptVersion: newest.Name,
				TranscriptDeleted: !fileExists(s.store.OriginalPath(f.RelPath)),
				Start:             start.Format(time.RFC3339Nano),
				End:               end.Format(time.RFC3339Nano),
			},
			start: start,
			end:   end,
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
					ID:    "cluster:" + start.Format(time.RFC3339Nano),
					Kind:  "cluster",
					Scope: "unknown", TimeSource: "backups", Correlation: "temporal",
					Start: start.Format(time.RFC3339Nano),
					End:   end.Format(time.RFC3339Nano),
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
		for _, f := range sfiles {
			if f.Attribution == "tool" {
				spans[i].info.ConfirmedFiles++
			} else {
				spans[i].info.CandidateFiles++
			}
		}
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
	// Never infer the workspace by counting unrelated concurrent changes.
	prefix := ""

	if span.info.Scope == "workspace" && span.info.Workspace != "." {
		prefix = span.info.Workspace + "/"
	}

	events := []sessionEvent{}
	sfiles := []sessionFile{}
	for _, f := range files {
		if span.info.Scope == "outside_base" && len(span.writes[f.RelPath]) == 0 {
			continue
		}
		if !workspaceFile(f.RelPath) {
			continue
		}
		if prefix != "" && !strings.HasPrefix(f.RelPath, prefix) && len(span.writes[f.RelPath]) == 0 {
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
				TS:      v.TS.Format(time.RFC3339Nano),
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
		attribution := "temporal"
		if len(span.writes[f.RelPath]) > 0 {
			attribution = "tool"
		}
		sfiles = append(sfiles, sessionFile{
			Attribution: attribution, Evidence: span.writes[f.RelPath], BackedUp: true,
			Path:     f.RelPath,
			Versions: inWindow,
			Created:  created,
			Deleted:  deleted,
			Prior:    prior,
		})
	}
	seen := make(map[string]bool)
	for _, f := range sfiles {
		seen[f.Path] = true
	}
	for path, evidence := range span.writes {
		if seen[path] || !workspaceFile(path) {
			continue
		}
		sfiles = append(sfiles, sessionFile{Path: path, Attribution: "tool", Evidence: evidence, BackedUp: false,
			Deleted: !fileExists(s.store.OriginalPath(path))})
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

// assignPrompts asigna a cada evento el texto del prompt del usuario que lo
// precede inmediatamente según el timestamp del prompt. Lee el transcript de la
// sesión (del disco, o del store si fue borrado) y extrae las entradas del
// usuario (role=user). Los eventos quedan ordenados como estaban pero cada uno
// lleva contexto de usuario cercano; no demuestra causalidad.
func assignPrompts(span sessionSpan, events []sessionEvent, srv *Server) {
	prompts := span.prompts
	for i := range events {
		ts, err := time.Parse(time.RFC3339Nano, events[i].TS)
		if err != nil {
			continue
		}
		idx := sort.Search(len(prompts), func(j int) bool { return prompts[j].Time.After(ts) }) - 1
		if idx >= 0 {
			events[i].Prompt = clipPromptText(prompts[idx].Text)
		}
	}
}

// workspaceFile excludes application state. Matching times never prove authorship.
func workspaceFile(rel string) bool {
	if agents.IsAI(rel) {
		return false
	}
	first := strings.SplitN(rel, "/", 2)[0]
	return !strings.HasPrefix(first, ".") && first != "snap" && first != "AppData" && first != "Library"
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
		shared := s.sharedWrites(sp, agentID, files)
		for i := range sfiles {
			sfiles[i].SharedWith = shared[sfiles[i].Path]
		}
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
	Agent   string `json:"agent,omitempty"`
	Session string `json:"session,omitempty"`
	Start   string `json:"start"`
	End     string `json:"end"`
	Target  string `json:"target,omitempty"`
	DryRun  bool   `json:"dry_run"`
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
	SkippedConflict  int          `json:"skipped_conflict"`
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
	var start, end time.Time
	var err error
	if req.Session == "" {
		start, err = time.Parse(time.RFC3339Nano, req.Start)
		if err != nil {
			writeErr(w, 400, "start inválido (RFC3339)")
			return
		}
		end, err = time.Parse(time.RFC3339Nano, req.End)
		if err != nil {
			writeErr(w, 400, "end inválido (RFC3339)")
			return
		}
		if end.Before(start) {
			writeErr(w, 400, "ventana invertida (end < start)")
			return
		}
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

	prefix := ""
	var allowed map[string]bool
	var shared map[string][]sessionReference
	if req.Agent != "" || req.Session != "" {
		if !agents.Known(req.Agent) || req.Session == "" {
			writeErr(w, 400, "agente o sesión inválidos")
			return
		}
		var selected *sessionSpan
		for _, span := range s.buildSessions(req.Agent, files) {
			if span.info.ID == req.Session {
				sp := span
				selected = &sp
				break
			}
		}
		if selected == nil {
			writeErr(w, 404, "sesión no encontrada")
			return
		}
		if selected.info.Scope == "outside_base" && len(selected.writes) == 0 {
			writeErr(w, 400, "proyecto fuera de la base protegida")
			return
		}
		if len(selected.writes) == 0 && target == "" {
			writeErr(w, 400, "elige un fichero: no hay escrituras confirmadas para restaurar toda la sesión")
			return
		}
		start, end = selected.start, selected.end
		shared = s.sharedWrites(*selected, req.Agent, files)
		allowed = make(map[string]bool)
		if target == "" {
			for path := range selected.writes {
				allowed[path] = true
			}
		} else {
			_, visible := s.sessionTrace(*selected, files)
			for _, f := range visible {
				allowed[f.Path] = true
			}
		}
		if selected.info.Scope == "workspace" && selected.info.Workspace != "." {
			prefix = selected.info.Workspace + "/"
		}
	}

	resp := revertResponse{DryRun: req.DryRun, Items: []revertItem{}}
	for _, f := range files {
		if !workspaceFile(f.RelPath) {
			continue
		}
		if allowed != nil && !allowed[f.RelPath] {
			continue
		}
		if prefix != "" && !strings.HasPrefix(f.RelPath, prefix) && allowed == nil {
			continue
		}
		if target != "" && f.RelPath != target && !strings.HasPrefix(f.RelPath, target+"/") {
			continue
		}
		item, affected := classifyRevert(s.store, f, start, end)
		if !affected {
			continue
		}
		if len(shared[f.RelPath]) > 0 && (item.Action == "revert" || item.Action == "restore_deleted") {
			item.Action = "skip_conflict"
		}
		if req.Session != "" && item.Action == "restore_deleted" {
			// Absence is current state, not proof of deletion during a session.
			since := s.store.MissingSince(f.RelPath)
			if since.IsZero() || since.Before(start) || since.After(end) {
				item.Action = "skip_conflict"
			}
		}
		switch item.Action {
		case "skip_no_prior":
			resp.SkippedNoPrior++
		case "skip_unchanged":
			resp.SkippedUnchanged++
		case "skip_conflict":
			resp.SkippedConflict++
		case "error":
			resp.Failed++
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
		if !os.IsNotExist(err) {
			item.Action = "error"
			item.Error = err.Error()
			return item, true
		}
		if f.Versions[0].TS.After(end) {
			item.Action = "skip_conflict"
			return item, true
		}
		item.Action = "restore_deleted"
		return item, true
	}
	if !fi.Mode().IsRegular() {
		item.Action = "skip_conflict"
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
	// Preserve later versions and edits not yet backed up. The UI may show
	// temporal candidates, but must never revert a newer workspace state.
	if f.Versions[0].TS.After(end) {
		item.Action = "skip_conflict"
		return item, true
	}
	cur, cerr := os.ReadFile(abs)
	latest, lerr := st.VersionContent(f.RelPath, f.Versions[0].Name)
	if cerr != nil || lerr != nil || !bytes.Equal(cur, latest) {
		item.Action = "skip_conflict"
		return item, true
	}
	item.Action = "revert"
	return item, true
}

// relativeWrites resolves only explicit tool paths, never shell snippets.
func (s *Server) relativeWrites(writes []agents.WriteEvidence) map[string][]agents.WriteEvidence {
	out := make(map[string][]agents.WriteEvidence)
	for _, w := range writes {
		abs := w.Path
		if !filepath.IsAbs(abs) {
			if !filepath.IsAbs(w.CWD) {
				continue
			}
			abs = filepath.Join(w.CWD, abs)
		}
		rel, err := s.store.RelPath(abs)
		if err != nil || !workspaceFile(rel) {
			continue
		}
		w.Path = rel
		out[rel] = append(out[rel], w)
	}
	return out
}

// handleAgentTranscript exports the exact stored snapshot used by the audit,
// including binary transcripts. It never reads mutable live agent state.
func (s *Server) handleAgentTranscript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, 405, "método no permitido")
		return
	}
	rel, err := cleanRelPath(r.URL.Query().Get("id"))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	id, _, ok := agents.Transcript(rel)
	if !ok || id != r.URL.Query().Get("agent") {
		writeErr(w, 400, "transcript inválido")
		return
	}
	versions, err := s.store.ListVersions(rel)
	if err != nil || len(versions) == 0 {
		writeErr(w, 404, "sin transcript guardado")
		return
	}
	requested := r.URL.Query().Get("version")
	selected := versions[0]
	if requested != "" {
		found := false
		for _, v := range versions {
			if v.Name == requested {
				selected = v
				found = true
				break
			}
		}
		if !found {
			writeErr(w, 404, "versión de transcript no encontrada")
			return
		}
	}
	fh, err := os.Open(selected.Path)
	if err != nil {
		writeErr(w, 404, "transcript no disponible")
		return
	}
	defer fh.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(rel)}))
	http.ServeContent(w, r, filepath.Base(rel), selected.TS, fh)
}

func (s *Server) sharedWrites(span sessionSpan, agentID string, files []store.FileInfo) map[string][]sessionReference {
	out := make(map[string][]sessionReference)
	for _, family := range agents.Registry() {
		for _, other := range s.buildSessions(family.ID, files) {
			if family.ID == agentID && other.info.ID == span.info.ID {
				continue
			}
			if other.end.Before(span.start) || other.start.After(span.end) {
				continue
			}
			for path := range other.writes {
				out[path] = append(out[path], sessionReference{Agent: family.ID, ID: other.info.ID, Title: other.info.Title})
			}
		}
	}
	return out
}
