package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/daemon"
	"github.com/luisobz/unmess-ai/internal/ignore"
	"github.com/luisobz/unmess-ai/internal/journal"
	"github.com/luisobz/unmess-ai/internal/retention"
	"github.com/luisobz/unmess-ai/internal/textdiff"
	"github.com/luisobz/unmess-ai/ui"
)

// handleUI sirve la UI embebida y sus assets. Sin autenticación (la página pide
// el token con fetch("/api/token") al cargar).
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	data, err := ui.Assets.ReadFile(name)
	if err != nil {
		// El enrutado hash vive todo bajo "/", así que cualquier otra ruta es 404.
		http.NotFound(w, r)
		return
	}
	switch filepath.Ext(name) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	}
	// Evita cacheo agresivo durante el desarrollo/uso local.
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// handleToken devuelve el token de sesión, solo a peticiones locales legítimas.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.requireLocal(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": s.token})
}

// statusResponse es la respuesta de GET /api/status.
type statusResponse struct {
	Version        string `json:"version"`
	Watching       bool   `json:"watching"`
	Paused         bool   `json:"paused"`
	StorePath      string `json:"store_path"`
	StoreSizeBytes int64  `json:"store_size_bytes"`
	FilesTracked   int    `json:"files_tracked"`
	JournalLines   int    `json:"journal_lines"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	files, err := s.store.ListFiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	size, err := s.store.Size()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	lines, err := journal.Count(s.store.JournalPath())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	paused := s.rt.IsPaused()
	writeJSON(w, http.StatusOK, statusResponse{
		Version:        s.version,
		Watching:       !paused,
		Paused:         paused,
		StorePath:      s.store.Prefix(),
		StoreSizeBytes: size,
		FilesTracked:   len(files),
		JournalLines:   lines,
	})
}

// pauseRequest es el cuerpo de POST /api/pause.
type pauseRequest struct {
	Paused bool `json:"paused"`
}

// handlePause pausa o reanuda la vigilancia del daemon. Devuelve el estado
// resultante. Muta estado, así que exige origen local.
func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.requireLocal(w, r) {
		return
	}
	var req pauseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo JSON inválido")
		return
	}
	s.rt.SetPaused(req.Paused)
	writeJSON(w, http.StatusOK, map[string]bool{"paused": s.rt.IsPaused()})
}

// handleEvents expone un stream Server-Sent Events con los eventos del daemon
// (versionado, restauración, error, pausa/reanudación, poda). La app nativa lo
// consume para disparar notificaciones del SO. Requiere token (vía protected) y
// origen local.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocal(w, r) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming no soportado")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, cancel := s.rt.Events.Subscribe()
	defer cancel()

	// Ping periódico para mantener viva la conexión y detectar clientes muertos.
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := writeSSE(w, string(ev.Type), data); err != nil {
				return
			}
			flusher.Flush()
		case <-ping.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeSSE escribe un evento SSE (event: <tipo>\ndata: <json>\n\n).
func writeSSE(w http.ResponseWriter, event string, data []byte) (int, error) {
	return w.Write([]byte("event: " + event + "\ndata: " + string(data) + "\n\n"))
}

// fileEntry describe un fichero versionado en GET /api/files.
type fileEntry struct {
	Path          string `json:"path"`
	Versions      int    `json:"versions"`
	LastVersionAt string `json:"last_version_at"`
	Deleted       bool   `json:"deleted"`
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	switch filter {
	case "all", "modified", "deleted":
	default:
		writeErr(w, http.StatusBadRequest, "filter debe ser all|modified|deleted")
		return
	}

	files, err := s.store.ListFiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]fileEntry, 0, len(files))
	for _, f := range files {
		if q != "" && !strings.Contains(strings.ToLower(f.RelPath), q) {
			continue
		}
		deleted := !fileExists(s.store.OriginalPath(f.RelPath))
		if filter == "modified" && deleted {
			continue
		}
		if filter == "deleted" && !deleted {
			continue
		}
		last := ""
		if len(f.Versions) > 0 {
			last = f.Versions[0].TS.Format(time.RFC3339)
		}
		out = append(out, fileEntry{
			Path:          f.RelPath,
			Versions:      len(f.Versions),
			LastVersionAt: last,
			Deleted:       deleted,
		})
	}
	// Orden estable: más recientes arriba (por última versión desc).
	slices.SortStableFunc(out, func(a, b fileEntry) int {
		if a.LastVersionAt == b.LastVersionAt {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(b.LastVersionAt, a.LastVersionAt)
	})
	writeJSON(w, http.StatusOK, out)
}

// versionEntry describe una versión en GET /api/versions.
type versionEntry struct {
	Name string `json:"name"`
	TS   string `json:"ts"`
	Size int64  `json:"size"`
}

func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	rel, err := pathParam(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	versions, err := s.store.ListVersions(rel)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]versionEntry, 0, len(versions))
	for _, v := range versions {
		out = append(out, versionEntry{Name: v.Name, TS: v.TS.Format(time.RFC3339), Size: v.Size})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleContent(w http.ResponseWriter, r *http.Request) {
	rel, err := pathParam(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	version := r.URL.Query().Get("version")
	if version == "" {
		version = "current"
	}
	data, status, err := s.readVersionData(rel, version)
	if err != nil {
		writeErr(w, status, err.Error())
		return
	}
	if textdiff.IsBinary(data) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]bool{"binary": true})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// readVersionData devuelve el contenido de una versión (o del disco si
// version=="current"), junto con el status HTTP a usar si hay error.
func (s *Server) readVersionData(rel, version string) ([]byte, int, error) {
	if version == "current" {
		data, err := os.ReadFile(s.store.OriginalPath(rel))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, http.StatusNotFound, err
			}
			return nil, http.StatusInternalServerError, err
		}
		return data, http.StatusOK, nil
	}
	name, err := cleanVersion(version)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	data, err := s.store.VersionContent(rel, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, http.StatusNotFound, err
		}
		return nil, http.StatusNotFound, err
	}
	return data, http.StatusOK, nil
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	rel, err := pathParam(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	versions, err := s.store.ListVersions(rel)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(versions) == 0 {
		writeErr(w, http.StatusNotFound, "sin versiones para la ruta")
		return
	}

	to := r.URL.Query().Get("to")
	from := r.URL.Query().Get("from")
	// Por defecto: última versión (to) vs anterior (from). Con to=current sin
	// from, se compara la última versión contra el disco.
	if to == "" {
		to = versions[0].Name
	}
	if from == "" {
		if to == "current" || len(versions) < 2 {
			from = versions[0].Name
		} else {
			from = versions[1].Name
		}
	}

	fromData, status, err := s.readVersionData(rel, from)
	if err != nil {
		writeErr(w, status, err.Error())
		return
	}
	toData, status, err := s.readVersionData(rel, to)
	if err != nil {
		writeErr(w, status, err.Error())
		return
	}

	out, err := textdiff.UnifiedString(rel+"@"+from, rel+"@"+to, fromData, toData)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(out))
}

// restoreRequest es el cuerpo de POST /api/restore.
type restoreRequest struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.requireLocal(w, r) {
		return
	}
	var req restoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo JSON inválido")
		return
	}
	rel, err := cleanRelPath(req.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := cleanVersion(req.Version)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	safety, err := s.store.Restore(rel, name, time.Now())
	if err != nil {
		if s.rt.Events != nil {
			s.rt.Events.Publish(daemon.Event{Type: daemon.EventError, Path: rel, Message: "Fallo al restaurar: " + err.Error()})
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.rt.Events != nil {
		s.rt.Events.Publish(daemon.Event{Type: daemon.EventRestored, Path: rel, Message: "Restauración completada"})
	}
	writeJSON(w, http.StatusOK, map[string]string{"safety_version": safety})
}

// forgetRequest es el cuerpo de POST /api/forget. Con Path se olvida un único
// fichero; con ApplyIgnores=true se olvidan todos los que coinciden con los
// ignore_patterns actuales.
type forgetRequest struct {
	Path         string `json:"path"`
	ApplyIgnores bool   `json:"apply_ignores"`
}

type forgetResponse struct {
	Forgotten  int   `json:"forgotten"`
	FreedBytes int64 `json:"freed_bytes"`
}

// handleForget elimina historial del store: deja de trackear ficheros. Muta
// estado, así que exige origen local.
func (s *Server) handleForget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.requireLocal(w, r) {
		return
	}
	var req forgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo JSON inválido")
		return
	}

	var resp forgetResponse
	if req.ApplyIgnores {
		c := s.currentConfig()
		m := ignore.New(c.IgnorePatterns)
		if m.Empty() {
			writeJSON(w, http.StatusOK, resp)
			return
		}
		files, err := s.store.ListFiles()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, f := range files {
			if !m.Match(f.RelPath) {
				continue
			}
			freed, ferr := s.store.Forget(f.RelPath)
			if ferr != nil {
				writeErr(w, http.StatusInternalServerError, ferr.Error())
				return
			}
			resp.Forgotten++
			resp.FreedBytes += freed
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	rel, err := cleanRelPath(req.Path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	freed, err := s.store.Forget(rel)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Forgotten = 1
	resp.FreedBytes = freed
	writeJSON(w, http.StatusOK, resp)
}

// journalEntry describe una entrada en GET /api/journal.
type journalEntry struct {
	TS   string `json:"ts"`
	Path string `json:"path"`
}

func (s *Server) handleJournal(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	entries, err := journal.Tail(s.store.JournalPath(), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// journal.Tail devuelve orden de llegada; queremos las más recientes primero.
	out := make([]journalEntry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		out = append(out, journalEntry{TS: entries[i].TS.Format(time.RFC3339), Path: entries[i].Path})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- config ---

type retentionDTO struct {
	MaxVersions    int `json:"max_versions"`
	MaxAgeDays     int `json:"max_age_days"`
	DeletedAgeDays int `json:"deleted_age_days"`
	MinKeep        int `json:"min_keep"`
}

type configDTO struct {
	Prefix          string       `json:"prefix"`
	DebounceSeconds int          `json:"debounce_seconds"`
	IncludedPaths   []string     `json:"included_paths"`
	ExcludedPaths   []string     `json:"excluded_paths"`
	ExcludeNames    []string     `json:"exclude_names"`
	IgnorePatterns  []string     `json:"ignore_patterns"`
	GitignoreAware  bool         `json:"gitignore_aware"`
	MaxFileSizeMB   int          `json:"max_file_size_mb"`
	Retention       retentionDTO `json:"retention"`
	Port            int          `json:"port"`
}

func toDTO(c config.Config) configDTO {
	return configDTO{
		Prefix:          c.Prefix,
		DebounceSeconds: c.DebounceSeconds,
		IncludedPaths:   c.IncludedPaths,
		ExcludedPaths:   c.ExcludedPaths,
		ExcludeNames:    c.ExcludeNames,
		IgnorePatterns:  c.IgnorePatterns,
		GitignoreAware:  c.GitignoreAware,
		MaxFileSizeMB:   c.MaxFileSizeMB,
		Retention: retentionDTO{
			MaxVersions:    c.Retention.MaxVersions,
			MaxAgeDays:     c.Retention.MaxAgeDays,
			DeletedAgeDays: c.Retention.DeletedAgeDays,
			MinKeep:        c.Retention.MinKeep,
		},
		Port: c.UI.Port,
	}
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, toDTO(s.currentConfig()))
	case http.MethodPut:
		s.handleConfigPut(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
	}
}

func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	if !s.requireLocal(w, r) {
		return
	}
	var dto configDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeErr(w, http.StatusBadRequest, "cuerpo JSON inválido")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cur := s.rt.Config
	old := *cur

	// Aplica los valores del DTO sobre la config actual.
	cur.Prefix = dto.Prefix
	cur.DebounceSeconds = dto.DebounceSeconds
	cur.IncludedPaths = dto.IncludedPaths
	cur.ExcludedPaths = dto.ExcludedPaths
	cur.ExcludeNames = dto.ExcludeNames
	cur.IgnorePatterns = dto.IgnorePatterns
	cur.GitignoreAware = dto.GitignoreAware
	cur.MaxFileSizeMB = dto.MaxFileSizeMB
	cur.Retention.MaxVersions = dto.Retention.MaxVersions
	cur.Retention.MaxAgeDays = dto.Retention.MaxAgeDays
	cur.Retention.DeletedAgeDays = dto.Retention.DeletedAgeDays
	cur.Retention.MinKeep = dto.Retention.MinKeep
	cur.UI.Port = dto.Port

	if err := cur.Validate(); err != nil {
		*cur = old // revertir
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := cur.Save(); err != nil {
		*cur = old // revertir
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	// El daemon aplica en caliente todo salvo lo estructural: el prefix del
	// store y el puerto HTTP se fijan al arrancar el proceso, así que solo
	// esos dos exigen reinicio.
	restart := old.Prefix != cur.Prefix || old.UI.Port != cur.UI.Port
	if configChanged(old, *cur) {
		s.rt.RequestReload(*cur)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"restart_required": restart})
}

// configChanged indica si cambió algún campo de la configuración.
func configChanged(a, b config.Config) bool {
	if a.Prefix != b.Prefix ||
		a.DebounceSeconds != b.DebounceSeconds ||
		a.GitignoreAware != b.GitignoreAware ||
		a.MaxFileSizeMB != b.MaxFileSizeMB ||
		a.UI.Port != b.UI.Port ||
		a.Retention != b.Retention {
		return true
	}
	if !slices.Equal(a.IncludedPaths, b.IncludedPaths) ||
		!slices.Equal(a.ExcludedPaths, b.ExcludedPaths) ||
		!slices.Equal(a.ExcludeNames, b.ExcludeNames) ||
		!slices.Equal(a.IgnorePatterns, b.IgnorePatterns) {
		return true
	}
	return false
}

// handleFlush versiona inmediatamente los cambios pendientes del debounce
// ("versionar ahora" en la UI). Muta estado, así que exige origen local.
func (s *Server) handleFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.requireLocal(w, r) {
		return
	}
	n, err := s.rt.RequestFlush(r.Context())
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"flushed": n})
}

// pruneRequest es el cuerpo de POST /api/prune.
type pruneRequest struct {
	DryRun bool `json:"dry_run"`
}

type pruneResponse struct {
	Examined        int   `json:"examined"`
	DeletedVersions int   `json:"deleted_versions"`
	PurgedFiles     int   `json:"purged_files"`
	FreedBytes      int64 `json:"freed_bytes"`
}

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	if !s.requireLocal(w, r) {
		return
	}
	var req pruneRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "cuerpo JSON inválido")
			return
		}
	}

	c := s.currentConfig()
	sum, err := s.store.Prune(retention.Config{
		MaxVersions:    c.Retention.MaxVersions,
		MaxAgeDays:     c.Retention.MaxAgeDays,
		DeletedAgeDays: c.Retention.DeletedAgeDays,
		MinKeep:        c.Retention.MinKeep,
	}, time.Now(), req.DryRun)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pruneResponse{
		Examined:        sum.Examined,
		DeletedVersions: sum.DeletedVersions,
		PurgedFiles:     sum.PurgedFiles,
		FreedBytes:      sum.FreedBytes,
	})
}

// --- helpers ---

func fileExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// errBadVersion indica un nombre de versión inválido.
var errBadVersion = errors.New("nombre de versión inválido")

// cleanVersion valida un nombre de versión (o "current"): sin separadores ni "..".
func cleanVersion(v string) (string, error) {
	if v == "" {
		return "", errBadVersion
	}
	if strings.ContainsAny(v, `/\`) || strings.Contains(v, "..") {
		return "", errBadVersion
	}
	return v, nil
}
