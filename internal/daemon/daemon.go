// Package daemon orquesta el pipeline de unmessd: watcher → debounce → filtros
// → store+journal, más la poda periódica. Expone Run(ctx, cfg, opts) como punto
// de entrada; opts.Hook es el enganche limpio donde otro agente montará después
// el servidor HTTP de la API sobre el mismo Runtime.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/debounce"
	"github.com/luisobz/unmess-ai/internal/gitignore"
	"github.com/luisobz/unmess-ai/internal/ignore"
	"github.com/luisobz/unmess-ai/internal/retention"
	"github.com/luisobz/unmess-ai/internal/store"
	"github.com/luisobz/unmess-ai/internal/watcher"
)

// Runtime agrupa las piezas ya inicializadas del daemon. Es lo que recibe el
// servidor de API a través de Options.Hook, y también la app nativa cuando
// consume Events y controla la pausa.
type Runtime struct {
	Config  *config.Config
	Store   *store.Store
	BaseDir string
	Logger  *log.Logger
	// Events publica versionados, restauraciones, errores y cambios de pausa a
	// suscriptores externos (bandeja/notificaciones nativas, stream SSE).
	Events *Broker

	// pause guarda el estado de vigilancia (ver IsPaused/SetPaused en events.go).
	pause pauseState

	// reloadCh y flushCh comunican peticiones externas (API) con el bucle del
	// daemon. Los inicializa Run; en un Runtime construido a mano (tests) las
	// peticiones se rechazan sin bloquear.
	reloadCh chan config.Config
	flushCh  chan flushRequest
}

type flushRequest struct{ reply chan int }

// RequestFlush pide al bucle del daemon que versione ya los cambios pendientes
// del debounce y devuelve cuántos había en cola. Seguro desde otras goroutines.
func (rt *Runtime) RequestFlush(ctx context.Context) (int, error) {
	if rt.flushCh == nil {
		return 0, errors.New("el daemon no está en ejecución")
	}
	req := flushRequest{reply: make(chan int, 1)}
	select {
	case rt.flushCh <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	select {
	case n := <-req.reply:
		return n, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// RequestReload aplica en caliente una nueva configuración: el bucle vacía lo
// pendiente y reconstruye watcher, filtros, debounce y retención. El prefix del
// store y el puerto HTTP se fijan al arrancar y quedan fuera (requieren
// reiniciar). Si había una recarga pendiente sin atender, la sustituye.
func (rt *Runtime) RequestReload(cfg config.Config) {
	if rt.reloadCh == nil {
		return
	}
	for {
		select {
		case rt.reloadCh <- cfg:
			return
		default:
			select {
			case <-rt.reloadCh:
			default:
			}
		}
	}
}

// Options configura la ejecución del daemon.
type Options struct {
	// Logger de salida legible; si es nil se usa uno a stderr.
	Logger *log.Logger
	// Debounce sobreescribe el retardo derivado de la config (útil en tests).
	// 0 = usar debounce_seconds de la config.
	Debounce time.Duration
	// RetentionInterval sobreescribe el intervalo de poda (por defecto 1 hora).
	RetentionInterval time.Duration
	// Hook se invoca una vez el Runtime está listo, antes de entrar en el bucle
	// de eventos. Aquí se montará el servidor de API. Puede ser nil. Si devuelve
	// error, el daemon aborta el arranque.
	Hook func(ctx context.Context, rt *Runtime) error
}

// Run arranca el daemon y bloquea hasta que ctx se cancela. Realiza un flush de
// los cambios pendientes antes de salir. Ante una petición de recarga (ver
// RequestReload) reconstruye el pipeline con la nueva configuración sin tirar
// el proceso ni el servidor de API.
func Run(ctx context.Context, cfg *config.Config, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "unmessd ", log.LstdFlags)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config inválida: %w", err)
	}

	baseDir, err := config.HomeDir()
	if err != nil {
		return err
	}
	// El prefix del store se fija al arrancar: cambiarlo requiere reiniciar
	// (el resto de la config se aplica en caliente vía RequestReload).
	prefix, err := cfg.PrefixExpanded()
	if err != nil {
		return err
	}

	st := store.New(prefix, baseDir)
	rt := &Runtime{
		Config: cfg, Store: st, BaseDir: baseDir, Logger: logger, Events: NewBroker(),
		reloadCh: make(chan config.Config, 1),
		flushCh:  make(chan flushRequest),
	}

	if opts.Hook != nil {
		if err := opts.Hook(ctx, rt); err != nil {
			return fmt.Errorf("hook de arranque: %w", err)
		}
	}

	snap := *cfg
	for {
		next, err := rt.runPipeline(ctx, snap, opts, prefix)
		if err != nil {
			return err
		}
		if next == nil {
			logger.Printf("apagado limpio")
			return nil
		}
		snap = *next
		logger.Printf("configuración recargada en caliente")
	}
}

// runPipeline construye watcher, filtros, debounce y retención a partir de una
// instantánea de la config y ejecuta el bucle de eventos. Devuelve (nil, nil)
// en apagado limpio, o la nueva config cuando llega una petición de recarga.
func (rt *Runtime) runPipeline(ctx context.Context, cfg config.Config, opts Options, prefix string) (*config.Config, error) {
	logger := rt.Logger
	st := rt.Store

	included, err := (&cfg).IncludedPathsExpanded()
	if err != nil {
		return nil, err
	}
	excluded, err := (&cfg).ExcludedPathsExpanded()
	if err != nil {
		return nil, err
	}

	flt := newFilter(&cfg, prefix, excluded, rt.BaseDir)
	gi := gitignore.New(cfg.GitignoreAware)

	exclude := func(dir string) bool { return flt.excludeDir(dir) }
	w, err := watcher.New(exclude)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	for _, root := range included {
		if _, serr := os.Stat(root); serr != nil {
			logger.Printf("aviso: ruta incluida no accesible %q: %v", root, serr)
			continue
		}
		if err := w.Add(root); err != nil {
			logger.Printf("aviso: no se pudo vigilar %q: %v", root, err)
		}
	}
	logger.Printf("vigilando %d ruta(s); store en %s", len(included), prefix)

	delay := opts.Debounce
	if delay <= 0 {
		delay = time.Duration(cfg.DebounceSeconds) * time.Second
	}
	deb := debounce.New[string](delay)

	retInterval := opts.RetentionInterval
	if retInterval <= 0 {
		retInterval = time.Hour
	}
	retCfg := retention.Config{
		MaxVersions:    cfg.Retention.MaxVersions,
		MaxAgeDays:     cfg.Retention.MaxAgeDays,
		DeletedAgeDays: cfg.Retention.DeletedAgeDays,
		MinKeep:        cfg.Retention.MinKeep,
	}
	runPrune := func() {
		sum, perr := st.Prune(retCfg, time.Now(), false)
		if perr != nil {
			logger.Printf("poda: error: %v", perr)
			return
		}
		if sum.DeletedVersions > 0 || sum.PurgedFiles > 0 {
			logger.Printf("poda: %d ficheros examinados, %d versiones borradas, %d ficheros purgados, %d bytes liberados",
				sum.Examined, sum.DeletedVersions, sum.PurgedFiles, sum.FreedBytes)
			if rt.Events != nil {
				rt.Events.Publish(Event{Type: EventPruned, Message: fmt.Sprintf("Poda: %d versiones eliminadas, %d bytes liberados", sum.DeletedVersions, sum.FreedBytes)})
			}
		}
	}
	runPrune()

	retTicker := time.NewTicker(retInterval)
	defer retTicker.Stop()

	// Timer del debounce; arranca parado.
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	timerActive := false

	rearm := func() {
		next, ok := deb.NextDeadline()
		if !ok {
			if timerActive {
				timer.Stop()
				timerActive = false
			}
			return
		}
		d := time.Until(next)
		if d < 0 {
			d = 0
		}
		if timerActive {
			timer.Stop()
		}
		timer.Reset(d)
		timerActive = true
	}

	flush := func(keys []string) {
		if len(keys) == 0 {
			return
		}
		rt.processFlush(keys, flt, gi, st, logger)
	}

	for {
		select {
		case <-ctx.Done():
			flush(deb.Flush())
			return nil, nil

		case newCfg := <-rt.reloadCh:
			// Antes de reconstruir, lo pendiente se versiona con los filtros
			// vigentes: nada se pierde por recargar.
			flush(deb.Flush())
			return &newCfg, nil

		case req := <-rt.flushCh:
			keys := deb.Flush()
			flush(keys)
			rearm()
			req.reply <- len(keys)

		case ev, ok := <-w.Events():
			if !ok {
				flush(deb.Flush())
				return nil, nil
			}
			// Con la vigilancia pausada se ignoran los eventos de entrada: los
			// cambios de este intervalo no se versionan (esa es la intención de
			// "pausar"). Lo ya acumulado antes de pausar se deja fluir.
			if rt.IsPaused() {
				continue
			}
			if ev.Op == watcher.OpWrite || ev.Op == watcher.OpCreate {
				deb.Add(ev.Path, time.Now())
				rearm()
			}
			// OpRemove/OpRename no generan versiones (el historial permanece).

		case werr, ok := <-w.Errors():
			if ok && werr != nil {
				logger.Printf("watcher: %v", werr)
			}

		case <-timer.C:
			timerActive = false
			flush(deb.Ready(time.Now()))
			rearm()

		case <-retTicker.C:
			runPrune()
		}
	}
}

// processFlush aplica los filtros baratos, el filtro gitignore por lote y
// escribe las versiones resultantes.
func (rt *Runtime) processFlush(keys []string, flt *filter, gi *gitignore.Filter, st *store.Store, logger *log.Logger) {
	candidates := make([]string, 0, len(keys))
	for _, p := range keys {
		if flt.rejectCheap(p, st, logger) {
			continue
		}
		candidates = append(candidates, p)
	}
	if len(candidates) == 0 {
		return
	}

	ignored, err := gi.Ignored(candidates)
	if err != nil {
		logger.Printf("gitignore: %v", err)
		ignored = nil
	}

	now := time.Now()
	for _, p := range candidates {
		if ignored[p] {
			continue
		}
		res, werr := st.WriteVersion(p, now)
		if werr != nil {
			if errors.Is(werr, store.ErrOutsideBase) {
				logger.Printf("ignorado (fuera de base): %s", p)
				continue
			}
			logger.Printf("error versionando %s: %v", p, werr)
			if rt.Events != nil {
				rt.Events.Publish(Event{Type: EventError, Path: p, Message: "No se pudo versionar: " + werr.Error()})
			}
			continue
		}
		logger.Printf("versionado %s -> %s", res.RelPath, res.Name)
		if rt.Events != nil {
			rt.Events.Publish(Event{Type: EventVersioned, Path: res.RelPath, Message: "Nueva versión guardada"})
		}
	}
}

// filter agrupa los criterios de exclusión baratos (todo salvo gitignore).
type filter struct {
	prefix       string
	excluded     []string
	excludeNames map[string]struct{}
	maxBytes     int64
	base         string
	ignored      *ignore.Matcher
}

func newFilter(cfg *config.Config, prefix string, excluded []string, baseDir string) *filter {
	names := make(map[string]struct{}, len(cfg.ExcludeNames))
	for _, n := range cfg.ExcludeNames {
		names[n] = struct{}{}
	}
	var maxBytes int64
	if cfg.MaxFileSizeMB > 0 {
		maxBytes = int64(cfg.MaxFileSizeMB) * 1024 * 1024
	}
	return &filter{
		prefix:       prefix,
		excluded:     excluded,
		excludeNames: names,
		maxBytes:     maxBytes,
		base:         baseDir,
		ignored:      ignore.New(cfg.IgnorePatterns),
	}
}

// matchesIgnored evalúa ignore_patterns sobre la ruta relativa a la base. Fuera
// de la base no aplica (esas rutas se descartan por otros criterios).
func (f *filter) matchesIgnored(path string) bool {
	if f.ignored.Empty() || f.base == "" {
		return false
	}
	rel, err := filepath.Rel(f.base, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return f.ignored.Match(filepath.ToSlash(rel))
}

// excludeDir decide si un directorio debe quedar fuera de la vigilancia.
func (f *filter) excludeDir(dir string) bool {
	if pathWithin(dir, f.prefix) {
		return true
	}
	for _, ex := range f.excluded {
		if pathWithin(dir, ex) {
			return true
		}
	}
	if _, ok := f.excludeNames[filepath.Base(dir)]; ok {
		return true
	}
	return f.matchesIgnored(dir)
}

// rejectCheap aplica, en orden, los filtros: prefix propio → excluded_paths →
// exclude_names → tamaño → no fichero regular. Devuelve true si hay que descartar.
func (f *filter) rejectCheap(path string, st *store.Store, logger *log.Logger) bool {
	if pathWithin(path, f.prefix) {
		return true
	}
	for _, ex := range f.excluded {
		if pathWithin(path, ex) {
			return true
		}
	}
	if f.hasExcludedComponent(path) {
		return true
	}
	if f.matchesIgnored(path) {
		return true
	}

	info, err := os.Lstat(path)
	if err != nil {
		// El fichero pudo desaparecer entre el evento y el flush.
		return true
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return true
	}
	if f.maxBytes > 0 && info.Size() > f.maxBytes {
		return true
	}
	if _, rerr := st.RelPath(path); rerr != nil {
		return true
	}
	return false
}

// hasExcludedComponent comprueba si algún componente de la ruta está en
// exclude_names.
func (f *filter) hasExcludedComponent(path string) bool {
	for _, comp := range strings.Split(filepath.ToSlash(path), "/") {
		if comp == "" {
			continue
		}
		if _, ok := f.excludeNames[comp]; ok {
			return true
		}
	}
	return false
}

// pathWithin indica si path es base o está contenido en base.
func pathWithin(path, base string) bool {
	if base == "" {
		return false
	}
	pa, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	ba, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	if pa == ba {
		return true
	}
	rel, err := filepath.Rel(ba, pa)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
