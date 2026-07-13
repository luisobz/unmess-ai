// Package watcher define una interfaz de vigilancia recursiva multiplataforma y
// su implementación sobre fsnotify (sin cgo, cross-compila a linux/darwin/
// windows). fsnotify no es recursivo: esta capa añade la recursividad haciendo
// un walk inicial que da de alta cada directorio y, al crearse un directorio
// nuevo, lo da de alta y escanea su contenido retroactivamente emitiendo
// eventos por los ficheros ya presentes.
package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Op es el tipo de cambio observado.
type Op int

const (
	// OpCreate: creación de fichero o directorio.
	OpCreate Op = iota
	// OpWrite: escritura sobre un fichero.
	OpWrite
	// OpRemove: borrado.
	OpRemove
	// OpRename: renombrado (origen).
	OpRename
)

func (o Op) String() string {
	switch o {
	case OpCreate:
		return "create"
	case OpWrite:
		return "write"
	case OpRemove:
		return "remove"
	case OpRename:
		return "rename"
	default:
		return "unknown"
	}
}

// Event es un cambio observado en el sistema de ficheros.
type Event struct {
	Path string
	Op   Op
}

// Watcher es la interfaz de vigilancia recursiva.
type Watcher interface {
	Events() <-chan Event
	Errors() <-chan error
	Add(root string) error
	Close() error
}

// ExcludeFunc decide si un directorio debe excluirse de la vigilancia (p. ej.
// el propio prefix del store, o nombres como .git/node_modules). Recibe la ruta
// absoluta del directorio.
type ExcludeFunc func(dir string) bool

type fsWatcher struct {
	w       *fsnotify.Watcher
	events  chan Event
	errs    chan error
	exclude ExcludeFunc

	mu    sync.Mutex
	added map[string]struct{}

	closeOnce sync.Once
	done      chan struct{}
}

// New crea un Watcher basado en fsnotify. exclude puede ser nil.
func New(exclude ExcludeFunc) (Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creando watcher fsnotify: %w", err)
	}
	fw := &fsWatcher{
		w:       w,
		events:  make(chan Event, 1024),
		errs:    make(chan error, 64),
		exclude: exclude,
		added:   make(map[string]struct{}),
		done:    make(chan struct{}),
	}
	go fw.loop()
	return fw, nil
}

func (fw *fsWatcher) Events() <-chan Event { return fw.events }
func (fw *fsWatcher) Errors() <-chan error { return fw.errs }

// Add da de alta root y todos sus subdirectorios (walk inicial, sin emitir
// eventos por los ficheros existentes).
func (fw *fsWatcher) Add(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolviendo raíz %q: %w", root, err)
	}
	return fw.addTree(abs, false)
}

// addTree da de alta dir y sus subdirectorios. Si emit es true, emite un evento
// OpCreate por cada fichero regular encontrado (escaneo retroactivo de un
// directorio recién creado).
func (fw *fsWatcher) addTree(dir string, emit bool) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Directorio inaccesible: continuar con el resto.
			return nil
		}
		if d.IsDir() {
			if fw.exclude != nil && fw.exclude(path) {
				return filepath.SkipDir
			}
			if aerr := fw.addDir(path); aerr != nil {
				fw.reportError(aerr)
			}
			return nil
		}
		if emit && d.Type().IsRegular() {
			fw.emit(Event{Path: path, Op: OpCreate})
		}
		return nil
	})
}

func (fw *fsWatcher) addDir(dir string) error {
	fw.mu.Lock()
	if _, ok := fw.added[dir]; ok {
		fw.mu.Unlock()
		return nil
	}
	fw.added[dir] = struct{}{}
	fw.mu.Unlock()

	if err := fw.w.Add(dir); err != nil {
		fw.mu.Lock()
		delete(fw.added, dir)
		fw.mu.Unlock()
		return fmt.Errorf("vigilando %q: %w", dir, err)
	}
	return nil
}

func (fw *fsWatcher) loop() {
	for {
		select {
		case <-fw.done:
			return
		case ev, ok := <-fw.w.Events:
			if !ok {
				return
			}
			fw.handle(ev)
		case err, ok := <-fw.w.Errors:
			if !ok {
				return
			}
			fw.reportError(err)
		}
	}
}

func (fw *fsWatcher) handle(ev fsnotify.Event) {
	// Un directorio recién creado se da de alta y su contenido preexistente se
	// emite retroactivamente como OpCreate.
	if ev.Op.Has(fsnotify.Create) {
		if info, err := os.Lstat(ev.Name); err == nil && info.IsDir() {
			if fw.exclude == nil || !fw.exclude(ev.Name) {
				if aerr := fw.addTree(ev.Name, true); aerr != nil {
					fw.reportError(aerr)
				}
			}
			return
		}
	}

	switch {
	case ev.Op.Has(fsnotify.Write):
		fw.emit(Event{Path: ev.Name, Op: OpWrite})
	case ev.Op.Has(fsnotify.Create):
		fw.emit(Event{Path: ev.Name, Op: OpCreate})
	case ev.Op.Has(fsnotify.Remove):
		fw.forget(ev.Name)
		fw.emit(Event{Path: ev.Name, Op: OpRemove})
	case ev.Op.Has(fsnotify.Rename):
		fw.forget(ev.Name)
		fw.emit(Event{Path: ev.Name, Op: OpRename})
	}
}

// forget descarta el registro de un directorio que ya no existe (para permitir
// re-alta si se recrea con el mismo nombre).
func (fw *fsWatcher) forget(path string) {
	fw.mu.Lock()
	delete(fw.added, path)
	fw.mu.Unlock()
}

func (fw *fsWatcher) emit(ev Event) {
	select {
	case fw.events <- ev:
	case <-fw.done:
	}
}

func (fw *fsWatcher) reportError(err error) {
	select {
	case fw.errs <- err:
	case <-fw.done:
	default:
	}
}

func (fw *fsWatcher) Close() error {
	var err error
	fw.closeOnce.Do(func() {
		close(fw.done)
		err = fw.w.Close()
	})
	return err
}
