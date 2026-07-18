// Package api implementa el servidor HTTP local de unmessai: sirve la UI web
// embebida y expone la API JSON. Bind exclusivo a 127.0.0.1:<ui.port>;
// autenticación por token local
// (Authorization: Bearer) generado al arrancar y persistido en <prefix>/var/token
// con permisos 0600. Se monta sobre el daemon vía daemon.Options.Hook.
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/daemon"
	"github.com/luisobz/unmess-ai/internal/store"
)

// Server encapsula el estado del servidor HTTP local.
type Server struct {
	rt      *daemon.Runtime
	store   *store.Store
	token   string
	port    int
	version string
	handler http.Handler

	// mu protege el acceso concurrente a la configuración (rt.Config), que puede
	// mutarse desde PUT /api/config mientras otros handlers la leen.
	mu sync.RWMutex
}

// New construye el servidor: genera el token, lo persiste en <prefix>/var/token
// (0600) y prepara el enrutado. No abre ningún socket.
func New(rt *daemon.Runtime, version string) (*Server, error) {
	if rt == nil || rt.Store == nil || rt.Config == nil {
		return nil, fmt.Errorf("runtime incompleto")
	}
	token, err := generateToken()
	if err != nil {
		return nil, err
	}
	if err := writeTokenFile(rt.Store.Prefix(), token); err != nil {
		return nil, err
	}
	s := &Server{
		rt:      rt,
		store:   rt.Store,
		token:   token,
		port:    rt.Config.UI.Port,
		version: version,
	}
	s.handler = s.routes()
	return s, nil
}

// Token devuelve el token de sesión (útil en tests).
func (s *Server) Token() string { return s.token }

// Handler expone el http.Handler para tests con httptest.
func (s *Server) Handler() http.Handler { return s.handler }

// Serve monta el servidor sobre el Runtime del daemon y arranca a escuchar en
// 127.0.0.1:<port>. Pensado para usarse como cuerpo de daemon.Options.Hook:
// devuelve rápido (el daemon continúa con su bucle de eventos). Si el puerto está
// ocupado, registra el aviso y devuelve nil (el daemon sigue vivo sin UI). Cierra
// el servidor limpiamente cuando ctx se cancela.
func Serve(ctx context.Context, rt *daemon.Runtime, version string) error {
	logger := rt.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "unmessd ", log.LstdFlags)
	}
	srv, err := New(rt, version)
	if err != nil {
		logger.Printf("api: no se pudo inicializar la UI: %v", err)
		return nil
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(srv.port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Printf("api: puerto %s ocupado o inaccesible (%v); el daemon sigue sin UI", addr, err)
		return nil
	}

	httpSrv := &http.Server{
		Handler:           srv.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	go func() {
		if serr := httpSrv.Serve(ln); serr != nil && serr != http.ErrServerClosed {
			logger.Printf("api: servidor detenido: %v", serr)
		}
	}()

	logger.Printf("UI disponible en http://127.0.0.1:%d/ (token en %s)", srv.port, tokenPath(rt.Store.Prefix()))
	return nil
}

// routes construye el enrutador.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// UI embebida y assets estáticos.
	mux.HandleFunc("/", s.handleUI)

	// API JSON.
	mux.HandleFunc("/api/token", s.handleToken)
	mux.HandleFunc("/api/status", s.protected(s.handleStatus))
	mux.HandleFunc("/api/agents", s.protected(s.handleAgents))
	mux.HandleFunc("/api/agent/sessions", s.protected(s.handleAgentSessions))
	mux.HandleFunc("/api/agent/session", s.protected(s.handleAgentSession))
	mux.HandleFunc("/api/agent/revert", s.protected(s.handleAgentRevert))
	mux.HandleFunc("/api/pause", s.protected(s.handlePause))
	mux.HandleFunc("/api/events", s.protected(s.handleEvents))
	mux.HandleFunc("/api/files", s.protected(s.handleFiles))
	mux.HandleFunc("/api/versions", s.protected(s.handleVersions))
	mux.HandleFunc("/api/content", s.protected(s.handleContent))
	mux.HandleFunc("/api/diff", s.protected(s.handleDiff))
	mux.HandleFunc("/api/restore", s.protected(s.handleRestore))
	mux.HandleFunc("/api/forget", s.protected(s.handleForget))
	mux.HandleFunc("/api/journal", s.protected(s.handleJournal))
	mux.HandleFunc("/api/config", s.protected(s.handleConfig))
	mux.HandleFunc("/api/prune", s.protected(s.handlePrune))
	mux.HandleFunc("/api/flush", s.protected(s.handleFlush))
	mux.HandleFunc("/api/protect", s.protected(s.handleProtect))

	return mux
}

// protected exige Authorization: Bearer <token> en todos los endpoints salvo
// /api/token. Los endpoints mutantes verifican además el origen local dentro de
// su propio handler.
func (s *Server) protected(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checkToken(r) {
			writeErr(w, http.StatusUnauthorized, "no autorizado: token ausente o inválido")
			return
		}
		h(w, r)
	}
}

// checkToken valida la cabecera Authorization: Bearer <token> en tiempo
// constante.
func (s *Server) checkToken(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := strings.TrimSpace(h[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

// localOK aplica la defensa CSRF: Host debe ser 127.0.0.1[:puerto] o
// localhost[:puerto]; Origin, si está presente, debe apuntar al mismo host local
// y puerto; Sec-Fetch-Site, si está presente, debe ser "none" o "same-origin".
func (s *Server) localOK(r *http.Request) bool {
	if !hostAllowed(r.Host, s.port) {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		if !originAllowed(origin, s.port) {
			return false
		}
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "none", "same-origin":
		return true
	default:
		return false
	}
}

// requireLocal responde 403 y devuelve false si la petición no es local legítima.
func (s *Server) requireLocal(w http.ResponseWriter, r *http.Request) bool {
	if !s.localOK(r) {
		writeErr(w, http.StatusForbidden, "petición no local rechazada")
		return false
	}
	return true
}

// hostAllowed comprueba que host (cabecera Host) sea 127.0.0.1 o localhost, con
// puerto ausente o igual al del servidor.
func hostAllowed(host string, port int) bool {
	name, p := splitHostPort(host)
	if name != "127.0.0.1" && name != "localhost" && name != "[::1]" && name != "::1" {
		return false
	}
	if p != "" && p != strconv.Itoa(port) {
		return false
	}
	return true
}

// originAllowed comprueba que origin sea http://127.0.0.1:<port> o
// http://localhost:<port> (esquema http, host local, puerto coincidente).
func originAllowed(origin string, port int) bool {
	rest, ok := strings.CutPrefix(origin, "http://")
	if !ok {
		return false
	}
	name, p := splitHostPort(rest)
	if name != "127.0.0.1" && name != "localhost" && name != "[::1]" && name != "::1" {
		return false
	}
	if p != "" && p != strconv.Itoa(port) {
		return false
	}
	return true
}

// splitHostPort separa "host:puerto" tolerando la ausencia de puerto y las
// direcciones IPv6 entre corchetes. No falla: si no hay puerto devuelve "".
func splitHostPort(hostport string) (host, port string) {
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		return h, p
	}
	return hostport, ""
}

// generateToken produce 32 bytes crypto/rand en hexadecimal.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generando token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// tokenPath devuelve <prefix>/var/token.
func tokenPath(prefix string) string {
	return filepath.Join(prefix, "var", "token")
}

// writeTokenFile persiste el token en <prefix>/var/token con permisos 0600.
func writeTokenFile(prefix, token string) error {
	dir := filepath.Join(prefix, "var")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creando directorio var: %w", err)
	}
	p := tokenPath(prefix)
	// Escritura atómica y con permisos restrictivos.
	tmp, err := os.CreateTemp(dir, ".token-*")
	if err != nil {
		return fmt.Errorf("creando token temporal: %w", err)
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("permisos del token: %w", err)
	}
	if _, err := tmp.WriteString(token); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("escribiendo token: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cerrando token: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renombrando token: %w", err)
	}
	return nil
}

// --- helpers de respuesta y validación de rutas ---

// writeJSON serializa v como JSON con el status dado.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr responde {error: msg} con el status dado.
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// cleanRelPath valida y normaliza una ruta relativa del store: siempre relativa,
// limpia, sin "..", nunca absoluta. Devuelve error si no cumple.
func cleanRelPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("ruta vacía")
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) {
		return "", fmt.Errorf("la ruta debe ser relativa")
	}
	// Normaliza a separadores "/" y rechaza componentes "..".
	slash := filepath.ToSlash(p)
	cleaned := path.Clean(slash)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("ruta inválida")
	}
	for _, comp := range strings.Split(cleaned, "/") {
		if comp == ".." {
			return "", fmt.Errorf("ruta con \"..\" no permitida")
		}
	}
	return cleaned, nil
}

// pathParam extrae y valida el parámetro "path" de la query.
func pathParam(r *http.Request) (string, error) {
	return cleanRelPath(r.URL.Query().Get("path"))
}

// currentConfig devuelve una copia superficial de la config actual bajo lock de
// lectura.
func (s *Server) currentConfig() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.rt.Config
}
