# unmessai

Protección automática de tus archivos frente a borrados o modificaciones accidentales —
incluidos los provocados por un agente de IA operando sobre tu sistema de archivos.
unmessai versiona en segundo plano cada cambio fuera de tus repositorios git, sin esfuerzo
manual, y te deja restaurar cualquier versión anterior. **Local-first**: tus datos no salen
de tu máquina, sin telemetría, sin nube.

Este documento es la referencia completa del proyecto: qué contiene cada parte, cómo se
compila, qué configuración y variables de entorno existen, y qué hace falta para desplegarlo
de forma totalmente funcional en cada sistema operativo.

---

## 1. Mapa del proyecto

```
unmessai/
├── cmd/
│   ├── unmessd/          Daemon: el proceso en segundo plano que vigila y versiona.
│   └── unmess/           CLI de usuario: status, restore, diff, prune, ui, service…
├── internal/
│   ├── config/           Carga/validación/escritura de config.toml (y sus defaults).
│   ├── watcher/          Vigilancia recursiva del sistema de archivos (fsnotify).
│   ├── debounce/         Coalescencia de escrituras: espera a que el fichero "repose".
│   ├── gitignore/        Exclusión de lo que git ignora (git check-ignore por lotes).
│   ├── store/            Contrato de Store: escribe/lista/restaura versiones + poda.
│   ├── journal/          Journal append-only de actividad (una línea por versión).
│   ├── retention/        Algoritmo puro de retención (qué versiones borrar y cuándo).
│   ├── textdiff/         Diff unificado + detección de binarios.
│   ├── daemon/           Orquestador: watcher → debounce → filtros → store, poda horaria.
│   └── api/              Servidor HTTP local (127.0.0.1 + token) que sirve API y UI.
├── ui/                   Frontend web (HTML/CSS/JS vanilla) embebido en el binario.
├── landing/              Web pública del producto (proyecto Astro, build estático).
├── packaging/
│   ├── linux/            build-deb.sh (paquete .deb) + desktop entry.
│   ├── windows/          unmessai.iss (instalador Inno Setup, sin admin).
│   └── macos/            Plantilla LaunchAgent + guía de firma/notarización/DMG.
├── docs/
│   ├── spec/             Especificaciones de producto originales (la "visión").
│   ├── adr/              Decisiones de arquitectura y su porqué (ADR-0001..0003).
│   └── architecture.md   Contrato técnico normativo (API, layout del store, pipeline).
└── .github/workflows/    CI: vet + tests + cross-compile en cada push.
```

### Cómo encajan las piezas

```
   tus ficheros ──▶ watcher ──▶ debounce ──▶ filtros ──▶ store + journal
                    (por SO)    (60 s)       (exclusiones,   │
                                             gitignore,      ▼
                                             tamaño)    ~/UnmessaiBackups/
                                                          ├── store/<ruta>/<fichero>/vFECHA.ext
                                                          └── var/journal
   UI web (127.0.0.1:48111) ◀── API con token ◀── unmessd
   CLI unmess ─────────────────── lee el store directamente
```

- **`unmessd`** es un único binario que hace todo: vigila, versiona, poda y sirve la UI.
- **`unmess`** es el CLI; funciona aunque el daemon no esté corriendo (opera directo
  sobre el store).
- El **Store** es el contrato central: ficheros planos legibles a
  mano, sin formatos opacos. La UI no distingue qué backend generó las versiones.

---

## 2. Requisitos

### Para compilar (desarrollo)

| Requisito | Versión | Para qué |
|---|---|---|
| Go | ≥ 1.24 | Único requisito real. Compila daemon + CLI + UI embebida. |
| git | cualquiera | Opcional en runtime: activa la exclusión gitignore-aware. Sin git instalado, esa función se desactiva sola (todo lo demás funciona). |

Dependencias Go (se descargan solas con `go build`): `BurntSushi/toml`,
`fsnotify/fsnotify`, `pmezard/go-difflib`. Sin cgo: los binarios son estáticos y
cross-compilan desde cualquier SO.

### Para desplegar (usuario final)

- **Nada más que los binarios.** No hay base de datos, ni servicios externos, ni red:
  todo es local. La UI se abre en cualquier navegador.
- **Linux**: `dpkg` para instalar el `.deb`; systemd (estándar) para el autoarranque.
- **Windows 10/11**: nada extra. El instalador no pide administrador.
- **macOS**: conceder **Acceso Total al Disco** al daemon (Ajustes del Sistema →
  Privacidad y Seguridad) para que pueda vigilar todo tu HOME — sin eso solo ve las
  carpetas con acceso automático.

---

## 3. Compilación

```sh
go build ./...        # compila todo
go test ./...         # suite de tests (~78% cobertura)
go vet ./...

# Binarios de release para los 3 SO (desde cualquier SO):
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/linux-amd64/   ./cmd/...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/windows-amd64/ ./cmd/...
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/darwin-arm64/  ./cmd/...
GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/darwin-amd64/  ./cmd/...
```

La UI web va **dentro** del binario (`go:embed`): no hay paso de build de frontend ni
assets que desplegar aparte.

---

## 4. Configuración

Fichero único `config.toml`. **Se crea solo, con comentarios y estos defaults, la primera
vez que arranca el daemon** — para probar no hace falta configurar nada.

Ubicación por SO:

| SO | Ruta |
|---|---|
| Linux | `~/.config/unmessai/config.toml` |
| macOS | `~/Library/Application Support/unmessai/config.toml` |
| Windows | `%APPDATA%\unmessai\config.toml` |

Contenido y defaults:

```toml
# Raíz del backup: contiene store/ (las versiones) y var/journal (la actividad).
prefix = "~/UnmessaiBackups"

# Segundos de "reposo" tras la última escritura antes de guardar versión.
# (Techo de latencia: 5× este valor bajo escritura continua.)
debounce_seconds = 60

# Qué se vigila y qué no.
included_paths = ["~"]
excluded_paths = ["~/Downloads", "~/Descargas", "~/Videos", "~/Vídeos", "~/Music", "~/Música"]
# Nombres de directorio excluidos a cualquier profundidad:
exclude_names = [".git", "node_modules", "dist", "build", ".cache", ".venv", "target", "__pycache__"]
# Patrones de ignorado (glob estilo gitignore, SIN distinguir mayúsculas:
# "**/cache/**" también excluye Cache/). Sin "/" = cualquier componente a
# cualquier nivel; con "/" se ancla a la carpeta personal; "**" = cero o más
# niveles. Coincidir con un directorio excluye todo su subárbol.
ignore_patterns = [".config", "**/*.log", "**/cache/**"]
# Excluir automáticamente lo que git ignora en cada repo detectado:
gitignore_aware = true
# No versionar ficheros mayores de (MB):
max_file_size_mb = 100

[retention]           # poda automática (al arrancar y cada hora)
max_versions = 50     # máx. versiones por fichero
max_age_days = 90     # borrar versiones más antiguas que esto
deleted_age_days = 30 # purgar historial completo de ficheros borrados hace > N días
min_keep = 3          # proteger SIEMPRE las N versiones más recientes

[ui]
port = 48111          # puerto local de la UI/API (solo 127.0.0.1)
```

Se puede editar a mano, con `unmess config set <clave> <valor>`, o desde Ajustes en la UI.
Los cambios (salvo `[retention]`) requieren reiniciar el daemon; la UI te lo avisa.

---

## 5. Variables de entorno

Solo existen **dos**, y ambas son opcionales (pensadas para tests, depuración o
instalaciones no estándar). Un despliegue normal **no necesita ninguna**:

| Variable | Efecto | Default si no está |
|---|---|---|
| `UNMESSAI_CONFIG` | Ruta alternativa del `config.toml`. Equivale al flag `-config` de `unmessd`. | La ruta estándar del SO (tabla de arriba) |
| `UNMESSAI_HOME` | Reubica la "base" de las rutas relativas del store (lo que normalmente es tu carpeta personal). Solo se vigilan/versionan ficheros bajo esta base. | Tu carpeta personal (`os.UserHomeDir()`) |

No hay claves de API, ni secretos, ni URLs de servicios: no hay nada externo que
configurar. El único "secreto" es el token de la UI, que **se genera solo** en cada
arranque y se guarda en `<prefix>/var/token` (permisos 0600).

---

## 6. Despliegue por sistema operativo

### Linux

```sh
# Opción A: paquete .deb
packaging/linux/build-deb.sh 0.1.0        # requiere dpkg-deb
sudo dpkg -i dist/unmessai_0.1.0_amd64.deb
unmess service install                     # autoarranque (systemd de usuario, sin root)
unmess ui                                  # abre la UI en el navegador

# Opción B: binarios sueltos
go build -o ~/.local/bin/ ./cmd/... && unmess service install
```

El servicio queda en `~/.config/systemd/user/unmessai.service`
(`systemctl --user status unmessai` para verlo).

### Windows

```
1. Cross-compilar (sección 3) → dist\windows-amd64\
2. Compilar el instalador:  iscc /DAppVersion=0.1.0 packaging\windows\unmessai.iss
3. Ejecutar dist\unmessai-setup-0.1.0.exe   (NO pide administrador)
```

El instalador copia los binarios, crea accesos directos y registra el autoarranque
(Scheduled Task "at log on" vía `unmess service install`). Desinstala limpio.

### macOS

```sh
# En un Mac (o runner CI macOS) — pasos completos en packaging/macos/README.md:
1. Compilar (o cross-compilar) los binarios darwin.
2. codesign con Developer ID + notarización (xcrun notarytool) + .dmg.
3. El usuario: copia binarios, `unmess service install` (LaunchAgent), concede
   Acceso Total al Disco al daemon, `unmess ui`.
```

Distribución **fuera** de la App Store (el sandbox de la Store impide vigilar rutas
arbitrarias). Sin firma/notarización, Gatekeeper bloqueará el binario en Macs ajenos:
para uso propio basta `xattr -d com.apple.quarantine`.

### Landing page

La landing es un proyecto **Astro** (`output: static`) en `landing/`. Genera HTML estático
sin peticiones externas; toda la interactividad (tema, idioma ES/EN, detección de SO) es un
`<script>` vanilla bundleado, sin islas de framework.

```sh
cd landing && npm install && npm run build   # resultado en landing/dist/
```

Sube `landing/dist/` a cualquier hosting estático (GitHub Pages, Netlify, Cloudflare Pages,
un nginx…). Detalles y estructura en `landing/README.md`.

---

## 7. Uso diario (CLI y UI)

```sh
unmessd                          # daemon en foreground (para probar; en producción usa el servicio)
unmess status                    # resumen: prefix, nº ficheros, versiones, tamaño, journal
unmess ls [--modified|--deleted] [texto]
unmess versions docs/informe.txt
unmess diff docs/informe.txt                 # última versión vs anterior
unmess diff docs/informe.txt --to current    # última versión vs disco
unmess restore docs/informe.txt [--version v2026-07-11-15-23.txt] [--yes]
unmess prune [--dry-run]         # aplicar retención a mano
unmess config path|get k|set k v
unmess ui [ruta]                 # abre la UI (opcionalmente sobre un fichero concreto)
unmess service install|uninstall|start|stop|status
```

**Restaurar nunca es destructivo**: antes de pisar el fichero actual se guarda una copia
de seguridad como versión nueva (y si colisiona de nombre con una versión existente, busca
un hueco — jamás sobrescribe historial).

**UI web** (`http://127.0.0.1:48111`): navegador de ficheros y versiones, diff coloreado,
toggle diff/contenido, búsqueda, filtros modificados/borrados, restaurar con confirmación,
ajustes, actividad reciente y "Acerca de" (con donación). Seguridad: solo escucha en
127.0.0.1, exige token (comparación en tiempo constante) y valida Host/Origin contra CSRF.

---

## 8. Estado del proyecto y qué falta para "producción pública"

Funcional y verificado end-to-end hoy: daemon + CLI + UI + tests (verde, `-race` limpio)
+ CI + cross-builds a los 3 SO. Lo que queda **antes de un release público**:

1. **Firma de binarios**: certificado Authenticode (Windows) y Developer ID + notarización
   (macOS). Sin esto, SmartScreen/Gatekeeper avisan o bloquean.
2. **Firma de binarios**: certificado Authenticode (Windows) y Developer ID + notarización
   (macOS). Sin esto, SmartScreen/Gatekeeper avisan o bloquean.
3. **Enlaces reales** en landing y "Acerca de": descargas de releases, PayPal/Ko-fi/GitHub
   Sponsors, página de docs (hoy son placeholders `TODO`).
4. **Iconos de bandeja nativos** por SO + notificaciones.
5. Autoactualización (Sparkle/winget/apt), Flatpak, dedup por hardlinks, FSEvents nativo
   en macOS.

## 9. Dónde seguir leyendo

- El **código** es la fuente de verdad: `internal/store` (contrato de Store), `internal/api`
  (API HTTP), `internal/daemon` (pipeline de vigilancia y poda), `cmd/` (binarios).
- `packaging/` — empaquetado e instalación por SO (cada subcarpeta tiene su README).
- `landing/` — web estática del producto y página de documentación de usuario (`/docs`).
- `docs/github-setup.md` — secrets y variables de CI/CD para el despliegue.
