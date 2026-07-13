#!/usr/bin/env bash
# Ensambla unmessai.app a partir de un binario darwin de la app nativa.
#
# Uso: packaging/macos/make-app.sh <version> <ruta-binario-unmess-app> [ruta-unmessd] [ruta-unmess]
#
# El binario de la app nativa (unmess-app) DEBE compilarse en macOS con la
# etiqueta `gui` (usa WKWebView vía CGO); no se puede compilar cruzado desde
# Linux/Windows:
#
#   CGO_ENABLED=1 go build -tags gui -o unmess-app ./cmd/unmess-app
#
# Para un binario universal: compilar arm64 y amd64 y unirlos con
#   lipo -create unmess-app-arm64 unmess-app-amd64 -output unmess-app
#
# Requiere el icono en build/appicon/icon.icns (build/appicon/generate.sh).
set -euo pipefail

VERSION="${1:?uso: make-app.sh <version> <unmess-app> [unmessd] [unmess]}"
APPBIN="${2:?falta la ruta al binario unmess-app (darwin)}"
DAEMON="${3:-}"
CLI="${4:-}"

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/dist"
APP="$OUT/unmessai.app"
ICNS="$ROOT/build/appicon/icon.icns"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp "$APPBIN" "$APP/Contents/MacOS/unmess-app"
chmod +x "$APP/Contents/MacOS/unmess-app"
[ -n "$DAEMON" ] && cp "$DAEMON" "$APP/Contents/MacOS/unmessd"
[ -n "$CLI" ] && cp "$CLI" "$APP/Contents/MacOS/unmess"

if [ -f "$ICNS" ]; then
  cp "$ICNS" "$APP/Contents/Resources/icon.icns"
else
  echo "AVISO: $ICNS no existe; ejecuta build/appicon/generate.sh" >&2
fi

sed "s/__VERSION__/$VERSION/g" "$ROOT/packaging/macos/Info.plist" > "$APP/Contents/Info.plist"

echo "OK: $APP"
echo "Siguiente: firmar y notarizar (ver packaging/macos/README.md)."
