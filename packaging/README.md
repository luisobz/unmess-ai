# Packaging de unmessai

## Binarios

`unmessd` (daemon) y `unmess` (CLI) son **Go puro** y se cross-compilan desde cualquier SO:

```sh
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/linux-amd64/   ./cmd/unmessd ./cmd/unmess
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/windows-amd64/ ./cmd/unmessd ./cmd/unmess
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/darwin-arm64/  ./cmd/unmessd ./cmd/unmess
GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/darwin-amd64/  ./cmd/unmessd ./cmd/unmess
```

La **app nativa** `unmess-app` (bandeja + ventana WebView + notificaciones, Wails v3) usa
CGO y el WebView de cada SO, así que **no** se cross-compila: se construye por plataforma
con la etiqueta de build `gui` (más `gtk3` en Linux). Ver el README de cada SO.

## Iconos

Todos los formatos de icono salen de `build/appicon/icon.svg`:

```sh
bash build/appicon/generate.sh   # PNG hicolor, .ico (Windows), .icns (macOS)
```

Detalles de las fuentes SVG y los estados de bandeja en `build/appicon/README.md`.

## Por SO

- `linux/` — `build-deb.sh` construye el `.deb` con los tres binarios, iconos hicolor,
  lanzador y autoarranque de la bandeja. Ver `linux/README.md`.
- `windows/` — Inno Setup (`unmessai.iss`) + embebido de icono con go-winres. Ver
  `windows/README.md`.
- `macos/` — `make-app.sh` ensambla `unmessai.app` (Info.plist + `.icns`) + guía de
  firma/notarización. Ver `macos/README.md`.

## Autoarranque

- La **app** (bandeja) arranca con la sesión gráfica: entrada XDG autostart en Linux,
  acceso directo en la carpeta Inicio en Windows. La app arranca el daemon si hace falta.
- Para un daemon **sin interfaz** (servidor/headless), `unmess service install` sigue
  registrando el autoarranque por usuario (systemd de usuario / LaunchAgent / Scheduled
  Task).

Nota Linux: unmessai en Linux usa inotify por defecto; el store son ficheros planos
legibles por cualquier herramienta.
