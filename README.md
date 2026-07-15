# unmessai

Protección automática de archivos frente a borrados y modificaciones accidentales,
incluidos los provocados por agentes de IA que operan sobre el sistema de archivos.
unmessai versiona en segundo plano cada cambio fuera de los repositorios git y permite
restaurar cualquier versión anterior. Es una herramienta local-first: los datos no salen
de la máquina, no hay telemetría ni servicios en la nube.

Este documento es la referencia técnica del proyecto: estructura, compilación,
configuración, despliegue por sistema operativo y estado actual.

---

## 1. Estructura del proyecto

```
unmessai/
├── cmd/
│   ├── unmessd/          Daemon: vigila, versiona, poda y sirve la UI local.
│   ├── unmess/           CLI: status, restore, diff, prune, ui, service, config.
│   └── unmess-app/       Aplicación nativa: icono de bandeja, ventana WebView y
│                         notificaciones del sistema. Se compila con -tags gui.
├── internal/
│   ├── config/           Carga, validación y escritura de config.toml.
│   ├── watcher/          Vigilancia recursiva del sistema de archivos (fsnotify).
│   ├── debounce/         Coalescencia de escrituras por fichero.
│   ├── gitignore/        Exclusión de lo que git ignora (git check-ignore por lotes).
│   ├── store/            Contrato de Store: escritura, listado, restauración y poda.
│   ├── journal/          Registro append-only de actividad.
│   ├── retention/        Algoritmo de retención (qué versiones eliminar y cuándo).
│   ├── textdiff/         Diff unificado y detección de binarios.
│   ├── daemon/           Orquestador del pipeline y recarga de configuración.
│   └── api/              Servidor HTTP local (127.0.0.1, con token) para API y UI.
├── ui/                   Frontend web (HTML/CSS/JS sin frameworks) embebido en unmessd.
├── landing/              Web pública del producto (Astro, build estático).
├── packaging/
│   ├── linux/            build-deb.sh (paquete .deb), desktop entries e iconos.
│   ├── windows/          unmessai.iss (instalador Inno Setup) y normas de uso.
│   └── macos/            Plantilla LaunchAgent y guía de firma, notarización y DMG.
├── docs/                 Documentación operativa (github-setup.md: secrets de CI/CD).
└── .github/workflows/    CI (vet, tests, cross-compile) y pipeline de release.
```

### Arquitectura

```
   ficheros del usuario ──▶ watcher ──▶ debounce ──▶ filtros ──▶ store + journal
                            (por SO)    (60 s)       (exclusiones,    │
                                                     gitignore,       ▼
                                                     tamaño)     ~/UnmessaiBackups/
                                                                  ├── store/<ruta>/<fichero>/vFECHA.ext
                                                                  └── var/journal
   UI web (127.0.0.1:48111) ◀── API con token ◀── unmessd
   App nativa (unmess-app) ──── misma API ──────── unmessd
   CLI (unmess) ──────────────── lee el store directamente
```

- `unmessd` es un único binario autocontenido: vigila, versiona, poda y sirve la UI.
- `unmess` es el CLI. Funciona aunque el daemon no esté en ejecución, porque opera
  directamente sobre el store.
- `unmess-app` es el punto de entrada del usuario final: muestra la UI en una ventana
  propia, mantiene un icono en la bandeja del sistema y emite notificaciones nativas.
  Cerrar la ventana la oculta a la bandeja; la aplicación solo termina desde la opción
  "Salir" del menú de bandeja. Si el daemon no está en marcha, la propia app lo arranca.
- El Store es el contrato central: ficheros planos legibles, sin formatos opacos.

---

## 2. Requisitos

### Compilación

| Requisito | Versión | Notas |
|---|---|---|
| Go | 1.24 o superior | Compila daemon, CLI y UI embebida en cualquier SO. |
| git | cualquiera | Opcional en runtime: activa la exclusión gitignore-aware. Si no está instalado, la función se desactiva de forma automática. |

Dependencias Go principales (se resuelven con `go build`): `BurntSushi/toml`,
`fsnotify/fsnotify`, `pmezard/go-difflib` y, solo para la app nativa, Wails v3.
El daemon y el CLI se compilan sin CGO y cross-compilan desde cualquier SO.

La app nativa tiene requisitos por plataforma:

| Plataforma | CGO | Requisitos de build |
|---|---|---|
| Windows | No | Ninguno adicional. Cross-compila desde Linux o macOS (el backend WebView2 usa syscalls). |
| Linux | Sí | `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev`. |
| macOS | Sí | Compilar en un equipo macOS (WKWebView). Ver `packaging/macos/README.md`. |

### Ejecución (usuario final)

- No hay base de datos, servicios externos ni dependencias de red. Todo es local.
- **Linux**: `dpkg` para instalar el paquete y systemd de usuario para el autoarranque.
- **Windows 10/11**: el runtime WebView2, incluido de serie en Windows 11 y en la
  mayoría de instalaciones de Windows 10 actualizadas. Ver la sección 6 para los
  avisos de instalación.
- **macOS**: conceder Acceso Total al Disco al daemon (Ajustes del Sistema, Privacidad
  y Seguridad) para que pueda vigilar la carpeta personal completa.

---

## 3. Compilación

```sh
go build ./...        # compila daemon, CLI y el stub de la app
go test ./...         # suite de tests
go vet ./...

# Daemon y CLI de release para los tres SO, desde cualquier SO:
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/linux-amd64/   ./cmd/unmessd ./cmd/unmess
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/windows-amd64/ ./cmd/unmessd ./cmd/unmess
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/darwin-arm64/  ./cmd/unmessd ./cmd/unmess
GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/darwin-amd64/  ./cmd/unmessd ./cmd/unmess
```

La app nativa requiere la etiqueta de build `gui`. Sin ella, `./cmd/...` compila un stub
de `unmess-app` que imprime un aviso y termina; ese stub existe para que la compilación
cruzada del resto del proyecto no dependa de las librerías de WebView de cada SO.

```sh
# Windows (cross-compila desde cualquier SO; -H windowsgui evita la consola):
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags gui -ldflags="-s -w -H windowsgui" \
  -o dist/windows-amd64/unmess-app.exe ./cmd/unmess-app

# Linux (GTK3/WebKitGTK, requiere CGO y las librerías de desarrollo):
CGO_ENABLED=1 go build -tags "gui gtk3" -o dist/linux-amd64/unmess-app ./cmd/unmess-app
```

Para el ejecutable de Windows conviene embeber antes el icono y el manifiesto con
[go-winres](https://github.com/tc-hib/go-winres); el procedimiento completo está en
`packaging/windows/README.md`.

La UI web va embebida en el binario del daemon mediante `go:embed`: no hay paso de
build de frontend ni assets que desplegar por separado.

---

## 4. Configuración

Toda la configuración vive en un único fichero `config.toml`. Si no existe, el daemon
lo crea en el primer arranque con valores por defecto comentados. En Windows, el
instalador lo genera durante la instalación con las rutas que elige el usuario.

Ubicación por SO:

| SO | Ruta |
|---|---|
| Linux | `~/.config/unmessai/config.toml` |
| macOS | `~/Library/Application Support/unmessai/config.toml` |
| Windows | `%APPDATA%\unmessai\config.toml` |

Contenido y valores por defecto:

```toml
# Raíz del backup: contiene store/ (las versiones) y var/journal (la actividad).
prefix = "~/UnmessaiBackups"

# Segundos de reposo tras la última escritura antes de guardar versión.
debounce_seconds = 60

# Qué se vigila y qué no.
included_paths = ["~"]
excluded_paths = ["~/Downloads", "~/Descargas", "~/Videos", "~/Vídeos", "~/Music", "~/Música"]
# Nombres de directorio excluidos a cualquier profundidad:
exclude_names = [".git", "node_modules", "dist", "build", ".cache", ".venv", "target", "__pycache__"]
# Patrones de ignorado (glob estilo gitignore, sin distinguir mayúsculas):
ignore_patterns = [".config", "**/*.log", "**/cache/**"]
# Excluir automáticamente lo que git ignora en cada repositorio detectado:
gitignore_aware = true
# No versionar ficheros mayores de este tamaño (MB):
max_file_size_mb = 100

[retention]           # poda automática (al arrancar y cada hora)
max_versions = 50     # máximo de versiones por fichero
max_age_days = 90     # eliminar versiones más antiguas que este umbral
deleted_age_days = 30 # purgar el historial de ficheros borrados hace más de N días
min_keep = 3          # proteger siempre las N versiones más recientes

[ui]
port = 48111          # puerto local de la UI y la API (solo 127.0.0.1)
```

Los valores por defecto se adaptan al sistema operativo. En Windows se excluye además
`~/AppData` (estado interno de aplicaciones, con cachés y ficheros bloqueados) y el
patrón `ntuser.dat*`; en macOS se excluye `~/Library`. El daemon excluye siempre su
propia carpeta de versiones, con independencia de la configuración.

### Aplicación de cambios

- **Desde la UI (Ajustes)**: los cambios se guardan y el daemon los aplica en caliente,
  reconstruyendo la vigilancia sin reiniciar el proceso. Las dos excepciones son la
  carpeta de versiones (`prefix`) y el puerto (`ui.port`), que se fijan al arrancar;
  si cambian, la UI indica que es necesario reiniciar.
- **Desde el CLI (`unmess config set`) o editando el fichero a mano**: requieren
  reiniciar el daemon, que solo relee el fichero al arrancar.

---

## 5. Variables de entorno

Existen dos, ambas opcionales y orientadas a tests, depuración o instalaciones no
estándar. Un despliegue normal no necesita ninguna.

| Variable | Efecto | Valor por defecto |
|---|---|---|
| `UNMESSAI_CONFIG` | Ruta alternativa del `config.toml`. Equivale al flag `-config` de `unmessd`. | Ruta estándar del SO (tabla anterior) |
| `UNMESSAI_HOME` | Reubica la base de las rutas relativas del store. Solo se vigilan ficheros bajo esta base. | Carpeta personal del usuario |

No hay claves de API, secretos ni URLs de servicios externos. El token de la UI se
genera en cada arranque y se guarda en `<prefix>/var/token` con permisos 0600.

---

## 6. Despliegue por sistema operativo

### Linux

```sh
# Opción A: paquete .deb
packaging/linux/build-deb.sh 0.2.0        # requiere dpkg-deb y las librerías GTK de la sección 2
sudo dpkg -i dist/unmessai_0.2.0_amd64.deb
unmess service install                     # autoarranque (systemd de usuario, sin root)
unmess ui                                  # abre la UI en el navegador

# Opción B: binarios sueltos
go build -o ~/.local/bin/ ./cmd/unmessd ./cmd/unmess && unmess service install
```

El servicio queda en `~/.config/systemd/user/unmessai.service` y puede consultarse con
`systemctl --user status unmessai`.

### Windows

Generación del instalador (automatizada en CI; ver sección 8):

```
1. Compilar los binarios, incluida la app con -tags gui (sección 3) → dist\windows-amd64\
2. Compilar el instalador:  iscc /DAppVersion=0.2.0 packaging\windows\unmessai.iss
3. Resultado: dist\unmessai-setup-v0.2.0.exe
```

El instalador guía al usuario por un asistente en español o inglés: aceptación de las
normas de uso, elección de la carpeta a vigilar y de la carpeta de versiones, y tareas
opcionales (acceso directo de escritorio, autoarranque). Al finalizar escribe
`%APPDATA%\unmessai\config.toml` con las rutas elegidas, copia los binarios, crea los
accesos directos y registra el autoarranque, que lanza `unmess-app.exe --background`
al iniciar sesión. En una actualización sobre una instalación existente, el asistente
omite la página de configuración y respeta el `config.toml` presente; los procesos en
ejecución se detienen automáticamente antes de copiar los binarios. La desinstalación
elimina binarios y accesos directos y conserva la configuración y las versiones del
usuario.

Avisos de instalación que conviene conocer:

- **La instalación es por usuario y no requiere administrador** (`PrivilegesRequired=lowest`).
  No hay que ejecutar el instalador con permisos elevados; hacerlo no aporta nada y
  puede asociar el autoarranque al perfil equivocado.
- **SmartScreen**: mientras los binarios no estén firmados con un certificado
  Authenticode, Windows muestra el aviso "Windows protegió su PC" al ejecutar el
  instalador. Se continúa desde "Más información" y "Ejecutar de todas formas".
- **Smart App Control**: en equipos Windows 11 con esta función en modo estricto, un
  ejecutable sin firma puede quedar bloqueado sin opción de continuar. En ese caso el
  usuario tiene que desactivar Smart App Control o desbloquear el fichero desde sus
  propiedades. La solución definitiva es firmar los binarios (sección 9).
- **Antivirus**: los binarios de Go sin firma producen ocasionalmente falsos positivos
  en motores antivirus. Si ocurre, basta con restaurar el fichero desde la cuarentena;
  la firma de código también elimina este problema.
- **WebView2**: la ventana de la app usa el runtime WebView2, incluido en Windows 11.
  En un Windows 10 sin él, la ventana no llega a renderizar; se resuelve instalando el
  "WebView2 Evergreen Runtime" desde la web de Microsoft.

### macOS

```sh
# En un equipo macOS (o runner de CI). Pasos completos en packaging/macos/README.md:
1. Compilar los binarios darwin (la app nativa requiere compilación en macOS).
2. Firmar con Developer ID, notarizar (xcrun notarytool) y empaquetar el .dmg.
3. Instalación: copiar binarios, `unmess service install` (LaunchAgent), conceder
   Acceso Total al Disco al daemon y abrir la UI con `unmess ui`.
```

La distribución se hace fuera de la App Store, cuyo sandbox impide vigilar rutas
arbitrarias. Sin firma y notarización, Gatekeeper bloquea el binario en otros equipos;
para uso propio puede eliminarse la cuarentena con `xattr -d com.apple.quarantine`.

### Landing

La landing es un proyecto Astro con salida estática en `landing/`. Genera HTML sin
peticiones externas; la interactividad (tema, idioma, detección de SO) es JavaScript
propio sin frameworks.

```sh
cd landing && pnpm install && pnpm build   # resultado en landing/dist/
```

El contenido de `landing/dist/` puede servirse desde cualquier hosting estático.
Estructura y detalles en `landing/README.md`.

---

## 7. Uso diario

### CLI

```sh
unmessd                          # daemon en primer plano (pruebas; en producción, el servicio)
unmess status                    # resumen: prefix, número de ficheros, versiones, tamaño
unmess ls [--modified|--deleted] [texto]
unmess versions docs/informe.txt
unmess diff docs/informe.txt                 # última versión frente a la anterior
unmess diff docs/informe.txt --to current    # última versión frente al disco
unmess restore docs/informe.txt [--version v2026-07-11-15-23.txt] [--yes]
unmess prune [--dry-run]         # aplicar la retención bajo demanda
unmess config path|get k|set k v
unmess ui [ruta]                 # abre la UI, opcionalmente sobre un fichero concreto
unmess service install|uninstall|start|stop|status
```

Restaurar nunca es destructivo: antes de sobrescribir el fichero actual se guarda una
copia como versión nueva, y los nombres de versión existentes no se reutilizan.

### Aplicación nativa y UI

La app (`unmess-app`) muestra la UI en una ventana propia y deja un icono en la bandeja
del sistema. Cerrar la ventana la oculta a la bandeja; el menú del icono permite abrirla
de nuevo, pausar y reanudar la protección, y salir. La misma UI puede abrirse en un
navegador en `http://127.0.0.1:48111` (o con `unmess ui --browser`).

Funciones principales de la UI: navegación de ficheros y versiones, diff coloreado,
búsqueda, filtros de modificados y borrados, restauración con confirmación, actividad
reciente y ajustes. El botón "Versionar ahora" de la barra superior fuerza el versionado
inmediato de los cambios pendientes del debounce, útil antes de una operación delicada
o cuando no se quiere esperar al intervalo configurado (internamente llama a
`POST /api/flush`).

Seguridad del servidor local: escucha solo en 127.0.0.1, exige un token por sesión con
comparación en tiempo constante y valida las cabeceras Host y Origin contra CSRF.

---

## 8. Integración continua y releases

- `ci.yml` se ejecuta en cada push: `go vet`, tests, compilación cruzada del daemon y
  el CLI para los tres SO, compilación de la app nativa de Windows (con `-tags gui`,
  sin CGO) y de la app de Linux con GTK3.
- `release.yml` se ejecuta al etiquetar `vX.Y.Z` y consta de tres jobs:
  1. `build` (Linux): compila daemon y CLI para las cuatro dianas, la app nativa de
     Windows (cross-compilada) y la de Linux; empaqueta los tar.gz, el zip de Windows
     y el paquete .deb.
  2. `installer-windows` (Windows): compila el instalador Inno Setup a partir de los
     binarios del job anterior.
  3. `publish`: publica todos los artefactos en la GitHub Release y, si está
     configurado, los sincroniza con el servidor de descargas.

Para compilar el instalador de Windows desde Linux en local puede usarse la imagen
Docker `amake/innosetup`; el procedimiento está en `packaging/windows/README.md`.

---

## 9. Estado del proyecto

Funcional y verificado de extremo a extremo: daemon, CLI, app nativa (Windows y Linux),
UI, instalador de Windows, suite de tests y CI. Pendiente antes de un release público:

1. **Firma de binarios**: certificado Authenticode en Windows (elimina los avisos de
   SmartScreen y los bloqueos de Smart App Control) y Developer ID con notarización en
   macOS (requisito de Gatekeeper).
2. **Empaquetado de macOS**: bundle .app, firma y DMG (la guía está en
   `packaging/macos/README.md`; falta ejecutarla en CI con un runner macOS).
3. **Enlaces definitivos** en la landing y en "Acerca de": descargas, donaciones y
   página de documentación.
4. Mejoras posteriores: autoactualización (winget, apt, Sparkle), Flatpak,
   deduplicación por hardlinks, FSEvents nativo en macOS.

## 10. Referencias

- El código es la fuente de verdad: `internal/store` (contrato del Store),
  `internal/api` (API HTTP), `internal/daemon` (pipeline y recarga), `cmd/` (binarios).
- `packaging/`: empaquetado e instalación por SO; cada subcarpeta tiene su README.
- `landing/`: web del producto y documentación de usuario.
- `docs/github-setup.md`: secrets y variables de CI/CD para el despliegue.
