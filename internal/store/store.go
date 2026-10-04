// Package store implementa el contrato de Store de unmessai:
// versiones planas en store/<ruta-relativa>/<fichero>/vYYYY-MM-DD-HH-MM-SS[.ext] y
// una línea de journal por escritura. Las rutas se guardan relativas a BaseDir
// (por defecto la carpeta personal); los ficheros fuera de BaseDir se ignoran.
package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/luisobz/unmess-ai/internal/journal"
)

// versionLayout es el formato con precisión de segundo usado en los nuevos
// nombres de versión. legacyVersionLayout se conserva al leer stores creados
// por versiones anteriores de unmessai.
const versionLayout = "2006-01-02-15-04-05"

const legacyVersionLayout = "2006-01-02-15-04"

// versionPrefix precede al timestamp en el nombre del fichero de versión.
const versionPrefix = "v"

// Store escribe y consulta versiones bajo prefix, con rutas relativas a baseDir.
type Store struct {
	prefix  string
	baseDir string
	journal *journal.Writer
	mu      sync.Mutex
}

// VersionResult describe una versión recién escrita.
type VersionResult struct {
	RelPath string // ruta relativa (separadores "/")
	Name    string // nombre de la versión, p. ej. v2026-07-11-10-30-45.txt
	Path    string // ruta absoluta del fichero de versión en disco
}

// VersionInfo describe una versión existente en el store.
type VersionInfo struct {
	Name string
	TS   time.Time
	Size int64
	Path string
}

// FileInfo describe un fichero versionado (un directorio hoja del store).
type FileInfo struct {
	RelPath  string
	Versions []VersionInfo // orden descendente por timestamp
}

// New crea un Store. baseDir es la base de las rutas relativas; prefix es la
// raíz del backup (contiene store/ y var/journal).
func New(prefix, baseDir string) *Store {
	return &Store{
		prefix:  prefix,
		baseDir: baseDir,
		journal: journal.NewWriter(filepath.Join(prefix, "var", "journal")),
	}
}

// Prefix devuelve la raíz del backup.
func (s *Store) Prefix() string { return s.prefix }

// BaseDir devuelve la base de las rutas relativas.
func (s *Store) BaseDir() string { return s.baseDir }

// StoreDir devuelve <prefix>/store.
func (s *Store) StoreDir() string { return filepath.Join(s.prefix, "store") }

// JournalPath devuelve la ruta del journal.
func (s *Store) JournalPath() string { return s.journal.Path() }

// Journal devuelve el Writer del journal (para reutilizarlo desde el daemon).
func (s *Store) Journal() *journal.Writer { return s.journal }

// ErrOutsideBase indica que una ruta cae fuera de baseDir.
var ErrOutsideBase = errors.New("ruta fuera de la base")

// RelPath calcula la ruta de absPath relativa a baseDir, con separadores "/".
// Devuelve ErrOutsideBase si absPath no está bajo baseDir.
func (s *Store) RelPath(absPath string) (string, error) {
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return "", fmt.Errorf("resolviendo ruta absoluta: %w", err)
	}
	base, err := filepath.Abs(s.baseDir)
	if err != nil {
		return "", fmt.Errorf("resolviendo baseDir: %w", err)
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrOutsideBase, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrOutsideBase
	}
	return filepath.ToSlash(rel), nil
}

// OriginalPath devuelve la ruta absoluta en disco del fichero cuya ruta
// relativa es relPath.
func (s *Store) OriginalPath(relPath string) string {
	return filepath.Join(s.baseDir, filepath.FromSlash(relPath))
}

// versionDir devuelve el directorio del store que aloja las versiones de relPath.
func (s *Store) versionDir(relPath string) string {
	return filepath.Join(s.StoreDir(), filepath.FromSlash(relPath))
}

// VersionName construye el nombre de versión para un fichero con extensión ext
// (incluyendo el punto, o vacío) en el instante ts.
func VersionName(ts time.Time, ext string) string {
	ts = ts.In(time.Local)
	layout := versionLayout
	if ts.Nanosecond() != 0 {
		layout += ".000000000"
	}
	return versionPrefix + ts.Format(layout) + ext
}

// WriteVersion copia el contenido actual de absPath a una nueva versión y añade
// la línea de journal correspondiente. Los segundos evitan que escrituras
// consecutivas dentro del mismo minuto se pisen (escritura a temporal + rename
// atómico).
func (s *Store) WriteVersion(absPath string, ts time.Time) (VersionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeVersion(absPath, ts)
}

func (s *Store) writeVersion(absPath string, ts time.Time) (VersionResult, error) {
	relPath, err := s.RelPath(absPath)
	if err != nil {
		return VersionResult{}, err
	}
	if err := rejectSymlinkParents(s.baseDir, relPath); err != nil {
		return VersionResult{}, err
	}
	if info, err := os.Lstat(absPath); err != nil || !info.Mode().IsRegular() {
		return VersionResult{}, fmt.Errorf("el origen no es un fichero regular")
	}
	ext := filepath.Ext(absPath)
	dir := s.versionDir(relPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return VersionResult{}, fmt.Errorf("creando directorio de versiones: %w", err)
	}
	// Publish with an exclusive hard link: even another daemon or CLI
	// cannot overwrite a version when timestamps collide. The temporary file
	// is complete and synced before its name becomes visible to readers.
	tmp, err := prepareCopy(absPath, dir, 0)
	if err != nil {
		return VersionResult{}, err
	}
	defer os.Remove(tmp)
	var name, dst string
	for attempts := 0; ; attempts++ {
		if attempts > 10000 {
			return VersionResult{}, fmt.Errorf("demasiadas colisiones de versiones")
		}
		name = VersionName(ts, ext)
		dst = filepath.Join(dir, name)
		if err := os.Link(tmp, dst); err != nil {
			if os.IsExist(err) {
				ts = ts.Add(time.Nanosecond)
				continue
			}
			return VersionResult{}, fmt.Errorf("publicando versión sin sobrescribir: %w", err)
		}
		break
	}
	if err := os.Remove(filepath.Join(dir, deletionMarker)); err != nil && !os.IsNotExist(err) {
		return VersionResult{}, err
	}
	if err := s.journal.Append(ts, relPath); err != nil {
		return VersionResult{}, fmt.Errorf("registrando en journal: %w", err)
	}
	return VersionResult{RelPath: relPath, Name: name, Path: dst}, nil
}

// Restore restaura la versión name del fichero relPath sobre su ruta original.
// Si el fichero original existe, escribe antes una copia de seguridad como nueva
// versión (pre-restore safety copy) y devuelve su nombre; en caso contrario
// devuelve "".
func (s *Store) Restore(relPath, name string, ts time.Time) (safety string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// CLI callers receive the same containment guarantees as API callers.
	if err := validRelative(relPath); err != nil {
		return "", err
	}
	if filepath.Base(name) != name || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("nombre de versión inválido")
	}
	if err := rejectSymlinkParents(s.baseDir, relPath); err != nil {
		return "", err
	}
	src := filepath.Join(s.versionDir(relPath), name)
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("versión %q no encontrada: %w", name, err)
	}
	dst := s.OriginalPath(relPath)

	if info, err := os.Lstat(dst); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("el destino no es un fichero regular")
		}
		safety, err = s.writeSafetyCopy(dst, relPath, ts)
		if err != nil {
			return "", fmt.Errorf("escribiendo copia de seguridad: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("comprobando fichero actual: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("creando directorio destino: %w", err)
	}
	if err := copyAtomic(src, dst); err != nil {
		return "", fmt.Errorf("restaurando versión: %w", err)
	}
	if err := os.Remove(filepath.Join(s.versionDir(relPath), deletionMarker)); err != nil && !os.IsNotExist(err) {
		return safety, err
	}
	return safety, nil
}

// ListFiles enumera todos los ficheros versionados (directorios hoja del store
// que contienen versiones).
func (s *Store) ListFiles() ([]FileInfo, error) {
	root := s.StoreDir()
	var files []FileInfo
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		versions, verr := readVersions(path)
		if verr != nil {
			return verr
		}
		if len(versions) == 0 {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		files = append(files, FileInfo{RelPath: filepath.ToSlash(rel), Versions: versions})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listando ficheros del store: %w", err)
	}
	return files, nil
}

// ListVersions devuelve las versiones de relPath en orden descendente.
func (s *Store) ListVersions(relPath string) ([]VersionInfo, error) {
	return readVersions(s.versionDir(relPath))
}

// HasVersions indica si relPath tiene alguna versión en el store.
func (s *Store) HasVersions(relPath string) (bool, error) {
	versions, err := s.ListVersions(relPath)
	if err != nil {
		return false, err
	}
	return len(versions) > 0, nil
}

// VersionContent devuelve el contenido crudo de una versión.
func (s *Store) VersionContent(relPath, name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(s.versionDir(relPath), name))
	if err != nil {
		return nil, fmt.Errorf("leyendo versión %q: %w", name, err)
	}
	return data, nil
}

// Size devuelve el tamaño total del store en bytes.
func (s *Store) Size() (int64, error) {
	var total int64
	err := filepath.WalkDir(s.StoreDir(), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("calculando tamaño del store: %w", err)
	}
	return total, nil
}

// readVersions lee las versiones de un directorio en orden descendente por ts.
func readVersions(dir string) ([]VersionInfo, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("leyendo directorio de versiones: %w", err)
	}
	var versions []VersionInfo
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		ts, ok := ParseVersionTime(e.Name())
		if !ok {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			return nil, fmt.Errorf("stat de versión: %w", ierr)
		}
		versions = append(versions, VersionInfo{
			Name: e.Name(),
			TS:   ts,
			Size: info.Size(),
			Path: filepath.Join(dir, e.Name()),
		})
	}
	// Orden descendente: más reciente primero; desempate por nombre.
	sortVersionsDesc(versions)
	return versions, nil
}

// ParseVersionTime extrae el timestamp del nombre de una versión
// (vYYYY-MM-DD-HH-MM-SS[.ext]). También acepta el formato histórico sin
// segundos para que las versiones existentes sigan siendo accesibles. Devuelve
// ok=false si no encaja.
func ParseVersionTime(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, versionPrefix) {
		return time.Time{}, false
	}
	body := name[len(versionPrefix):]
	candidates := []string{body}
	if ext := filepath.Ext(name); ext != "" {
		candidates = append(candidates, strings.TrimSuffix(body, ext))
	}
	for _, candidate := range candidates {
		for _, layout := range []string{versionLayout + ".000000000", versionLayout, legacyVersionLayout} {
			if ts, err := time.ParseInLocation(layout, candidate, time.Local); err == nil {
				return ts, true
			}
		}
	}
	return time.Time{}, false
}

func sortVersionsDesc(v []VersionInfo) {
	// Inserción simple: listas cortas por fichero.
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && lessDesc(v[j], v[j-1]); j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func lessDesc(a, b VersionInfo) bool {
	if a.TS.Equal(b.TS) {
		return a.Name > b.Name
	}
	return a.TS.After(b.TS)
}

// writeSafetyCopy guarda el contenido actual de absPath como versión previa a un
// restore. A diferencia del versionado normal, nunca debe sobrescribir una
// versión existente (podría machacar justo la versión que se va a restaurar, o
// una versión aún no reflejada en disco): si el nombre del segundo actual está
// ocupado, la publicación exclusiva añade precisión de nanosegundos. Si el contenido en
// disco ya es idéntico a la versión más reciente, la reutiliza sin escribir
// nada.
func (s *Store) writeSafetyCopy(absPath, relPath string, ts time.Time) (string, error) {
	versions, err := s.ListVersions(relPath)
	if err != nil {
		return "", err
	}
	if len(versions) > 0 {
		if same, cerr := filesEqual(absPath, versions[0].Path); cerr == nil && same {
			return versions[0].Name, nil
		}
	}
	res, err := s.writeVersion(absPath, ts)
	if err != nil {
		return "", err
	}
	return res.Name, nil
}

// filesEqual compara el contenido de dos ficheros.
func filesEqual(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if ia.Size() != ib.Size() {
		return false, nil
	}
	fa, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer fb.Close()

	bufA := make([]byte, 64*1024)
	bufB := make([]byte, 64*1024)
	for {
		na, ea := io.ReadFull(fa, bufA)
		nb, eb := io.ReadFull(fb, bufB)
		if na != nb || !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}
		if ea == io.EOF || ea == io.ErrUnexpectedEOF {
			if eb == io.EOF || eb == io.ErrUnexpectedEOF {
				return true, nil
			}
			return false, nil
		}
		if ea != nil {
			return false, ea
		}
		if eb != nil {
			return false, eb
		}
	}
}

// copyAtomic copia src a dst escribiendo en un temporal en el mismo directorio y
// renombrando (atómico dentro del mismo sistema de ficheros).
func copyAtomic(src, dst string) error {
	var mode os.FileMode
	if info, err := os.Lstat(dst); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := prepareCopy(src, filepath.Dir(dst), mode)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("renombrando temporal: %w", err)
	}
	return nil
}

func prepareCopy(src, dir string, mode os.FileMode) (name string, err error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("abriendo origen: %w", err)
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("el origen no es un fichero regular")
	}
	if mode == 0 {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	name = tmp.Name()
	tempName := name
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tempName)
		}
	}()
	if _, err = io.Copy(tmp, in); err != nil {
		return "", fmt.Errorf("copiando contenido: %w", err)
	}
	if err = tmp.Chmod(mode); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	return name, nil
}

func validRelative(rel string) error {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "\\") {
		return ErrOutsideBase
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".." || part == "." || part == "" {
			return ErrOutsideBase
		}
	}
	return nil
}

func rejectSymlinkParents(base, rel string) error {
	p := base
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts[:len(parts)-1] {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restauración rechazada: directorio simbólico")
		}
	}
	return nil
}

// OriginalContent reads a regular file without following symlink directories.
func (s *Store) OriginalContent(rel string) ([]byte, error) {
	if err := validRelative(rel); err != nil {
		return nil, err
	}
	if err := rejectSymlinkParents(s.baseDir, rel); err != nil {
		return nil, err
	}
	path := s.OriginalPath(rel)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("el origen no es un fichero regular")
	}
	return os.ReadFile(path)
}
