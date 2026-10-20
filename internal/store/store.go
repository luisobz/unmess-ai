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
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	p, err := s.ResolveOriginalPath(relPath)
	if err != nil {
		return ""
	}
	return p
}

// ResolveOriginalPath resuelve relPath dentro de baseDir. Los enlaces
// simbólicos que apuntan fuera de la base se rechazan, también cuando el
// destino todavía no existe.
func (s *Store) ResolveOriginalPath(relPath string) (string, error) {
	return safePathUnder(s.baseDir, relPath)
}

// versionDir devuelve el directorio del store que aloja las versiones de relPath.
func (s *Store) versionDir(relPath string) string {
	p, err := safePathUnder(s.StoreDir(), relPath)
	if err != nil {
		return ""
	}
	return p
}

// VersionName construye el nombre de versión para un fichero con extensión ext
// (incluyendo el punto, o vacío) en el instante ts.
func VersionName(ts time.Time, ext string) string {
	return versionPrefix + ts.Format(versionLayout) + ext
}

// WriteVersion copia el contenido actual de absPath a una nueva versión y añade
// la línea de journal correspondiente. Los segundos evitan que escrituras
// consecutivas dentro del mismo minuto se pisen (escritura a temporal + rename
// atómico).
func (s *Store) WriteVersion(absPath string, ts time.Time) (VersionResult, error) {
	relPath, err := s.RelPath(absPath)
	if err != nil {
		return VersionResult{}, err
	}
	relPath, err = cleanRelativePath(relPath)
	if err != nil {
		return VersionResult{}, err
	}
	if err := os.MkdirAll(s.StoreDir(), 0755); err != nil {
		return VersionResult{}, err
	}
	source, err := os.OpenRoot(s.baseDir)
	if err != nil {
		return VersionResult{}, err
	}
	defer source.Close()
	destination, err := os.OpenRoot(s.StoreDir())
	if err != nil {
		return VersionResult{}, err
	}
	defer destination.Close()
	name := VersionName(ts, filepath.Ext(absPath))
	dst := filepath.Join(relPath, name)
	if err := copyAtomicRoot(source, filepath.FromSlash(relPath), destination, dst); err != nil {
		return VersionResult{}, err
	}
	if err := s.journal.Append(ts, relPath); err != nil {
		return VersionResult{}, fmt.Errorf("registrando en journal: %w", err)
	}
	return VersionResult{RelPath: relPath, Name: name, Path: filepath.Join(s.StoreDir(), dst)}, nil
}

// Restore restaura la versión name del fichero relPath sobre su ruta original.
// Si el fichero original existe, escribe antes una copia de seguridad como nueva
// versión (pre-restore safety copy) y devuelve su nombre; en caso contrario
// devuelve "".
func (s *Store) Restore(relPath, name string, ts time.Time) (safety string, err error) {
	relPath, err = cleanRelativePath(relPath)
	if err != nil {
		return "", err
	}
	if !safeVersionName(name) {
		return "", fmt.Errorf("nombre de versión inválido")
	}
	source, err := os.OpenRoot(s.StoreDir())
	if err != nil {
		return "", err
	}
	defer source.Close()
	destination, err := os.OpenRoot(s.baseDir)
	if err != nil {
		return "", err
	}
	defer destination.Close()
	src := filepath.Join(relPath, name)
	if _, err = source.Stat(src); err != nil {
		return "", err
	}
	if _, err = destination.Stat(filepath.FromSlash(relPath)); err == nil {
		safety, err = s.writeSafetyCopy(filepath.Join(s.baseDir, relPath), relPath, ts)
		if err != nil {
			return "", fmt.Errorf("escribiendo copia de seguridad: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err = copyAtomicRoot(source, src, destination, filepath.FromSlash(relPath)); err != nil {
		return "", err
	}
	return safety, nil
}

// ListFiles enumera todos los ficheros versionados (directorios hoja del store
// que contienen versiones).
func (s *Store) ListFiles() ([]FileInfo, error) {
	root, err := os.OpenRoot(s.StoreDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var files []FileInfo
	err = fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		versions, err := s.readVersions(rel)
		if err != nil {
			return err
		}
		if len(versions) > 0 {
			files = append(files, FileInfo{RelPath: filepath.ToSlash(rel), Versions: versions})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listando ficheros del store: %w", err)
	}
	return files, nil
}

// ListVersions devuelve las versiones de relPath en orden descendente.
func (s *Store) ListVersions(relPath string) ([]VersionInfo, error) {
	dir := s.versionDir(relPath)
	if dir == "" {
		return nil, ErrUnsafePath
	}
	return s.readVersions(relPath)
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
	if !safeVersionName(name) {
		return nil, fmt.Errorf("nombre de versión inválido")
	}
	relPath, err := cleanRelativePath(relPath)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.StoreDir())
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFile(filepath.Join(relPath, name))
	if err != nil {
		return nil, fmt.Errorf("leyendo versión %q: %w", name, err)
	}
	return data, nil
}

// ErrUnsafePath indica que una ruta relativa o un enlace simbólico intenta
// salir de la raíz autorizada.
var ErrUnsafePath = errors.New("ruta fuera de la raíz autorizada")

// safeVersionName limita los nombres de versión a un único componente con
// timestamp reconocido, evitando que el nombre complete una ruta.
func safeVersionName(name string) bool {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return false
	}
	_, ok := ParseVersionTime(name)
	return ok
}

// safePathUnder validates a relative path through a confined directory handle.
// Actual reads and mutations must also use os.Root to prevent symlink races.
func safePathUnder(root, relPath string) (string, error) {
	rel, err := cleanRelativePath(relPath)
	if err != nil {
		return "", err
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	directory, err := os.OpenRoot(absoluteRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Join(absoluteRoot, filepath.FromSlash(rel)), nil
		}
		return "", err
	}
	defer directory.Close()
	if _, err = directory.Stat(filepath.FromSlash(rel)); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("%w: %v", ErrUnsafePath, err)
	}
	return filepath.Join(absoluteRoot, filepath.FromSlash(rel)), nil
}

func cleanRelativePath(relPath string) (string, error) {
	if relPath == "" || filepath.IsAbs(relPath) || filepath.VolumeName(relPath) != "" ||
		strings.HasPrefix(relPath, `/`) || strings.HasPrefix(relPath, `\`) || hasWindowsDrivePrefix(relPath) {
		return "", ErrUnsafePath
	}
	rel := strings.ReplaceAll(filepath.ToSlash(relPath), `\`, `/`)
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return "", ErrUnsafePath
	}
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == ".." || path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "../") {
		return "", ErrUnsafePath
	}
	return cleaned, nil
}

// hasWindowsDrivePrefix also rejects drive-relative paths on non-Windows hosts,
// where filepath.VolumeName does not recognize the Windows volume syntax.
func hasWindowsDrivePrefix(relPath string) bool {
	return len(relPath) >= 2 && ((relPath[0] >= 'A' && relPath[0] <= 'Z') ||
		(relPath[0] >= 'a' && relPath[0] <= 'z')) && relPath[1] == ':'
}

// Size devuelve el tamaño total del store en bytes.
func (s *Store) Size() (int64, error) {
	root, err := os.OpenRoot(s.StoreDir())
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer root.Close()
	var total int64
	err = fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := root.Lstat(rel)
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// readVersions lee las versiones de un directorio en orden descendente por ts.
func (s *Store) readVersions(relPath string) ([]VersionInfo, error) {
	root, err := os.OpenRoot(s.StoreDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	directory, err := root.Open(filepath.FromSlash(relPath))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	ents, err := directory.ReadDir(-1)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("leyendo directorio de versiones: %w", err)
	}
	var versions []VersionInfo
	for _, e := range ents {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		ts, ok := ParseVersionTime(e.Name())
		if !ok {
			continue
		}
		info, ierr := root.Lstat(filepath.Join(relPath, e.Name()))
		if ierr != nil {
			return nil, fmt.Errorf("stat de versión: %w", ierr)
		}
		versions = append(versions, VersionInfo{
			Name: e.Name(),
			TS:   ts,
			Size: info.Size(),
			Path: filepath.Join(s.StoreDir(), relPath, e.Name()),
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
	if ext := filepath.Ext(name); ext != "" {
		body = strings.TrimSuffix(body, ext)
	}
	for _, layout := range []string{versionLayout, legacyVersionLayout} {
		ts, err := time.ParseInLocation(layout, body, time.Local)
		if err == nil {
			return ts, true
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
// ocupado, avanza de segundo en segundo hasta uno libre. Si el contenido en
// disco ya es idéntico a la versión más reciente, la reutiliza sin escribir
// nada.
func (s *Store) writeSafetyCopy(absPath, relPath string, ts time.Time) (string, error) {
	versions, err := s.ListVersions(relPath)
	if err != nil {
		return "", err
	}
	if len(versions) > 0 {
		if same, cerr := s.originalEqualsVersion(relPath, versions[0].Name); cerr == nil && same {
			return versions[0].Name, nil
		}
	}
	ext := filepath.Ext(absPath)
	versionsRoot, err := os.OpenRoot(s.StoreDir())
	if err != nil {
		return "", err
	}
	defer versionsRoot.Close()
	sts := ts
	for i := 0; ; i++ {
		if i > 10000 {
			return "", fmt.Errorf("sin nombre libre para la copia de seguridad de %q", relPath)
		}
		name := VersionName(sts, ext)
		if _, serr := versionsRoot.Lstat(filepath.Join(relPath, name)); os.IsNotExist(serr) {
			break
		}
		sts = sts.Add(time.Second)
	}
	res, err := s.WriteVersion(absPath, sts)
	if err != nil {
		return "", err
	}
	return res.Name, nil
}

// filesEqual compara el contenido de dos ficheros.
func (s *Store) originalEqualsVersion(relPath, name string) (bool, error) {
	base, err := os.OpenRoot(s.baseDir)
	if err != nil {
		return false, err
	}
	defer base.Close()
	versions, err := os.OpenRoot(s.StoreDir())
	if err != nil {
		return false, err
	}
	defer versions.Close()
	fa, err := base.Open(filepath.FromSlash(relPath))
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := versions.Open(filepath.Join(relPath, name))
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
