#!/usr/bin/env bash
# Genera todos los formatos de icono a partir de build/appicon/icon.svg.
#
# Salidas:
#   - cmd/unmess-app/assets/icon.png     (icono embebido: ventana y notificaciones)
#   - cmd/unmess-app/assets/tray-*.png   (iconos de bandeja a 22px: activo/pausa/error)
#   - packaging/linux/icons/hicolor/...  (tema de iconos freedesktop + SVG escalable)
#   - build/appicon/icon.ico             (Windows)
#   - build/appicon/icon.icns            (macOS)
#
# Requisitos: rsvg-convert (librsvg2-bin), icotool (icoutils), png2icns (icnsutils),
# opcional optipng. Reejecutar tras cambiar icon.svg.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
svg="$here/icon.svg"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

sizes=(16 24 32 48 64 128 256 512 1024)
render() { rsvg-convert -w "$1" -h "$1" "$svg" -o "$2"; command -v optipng >/dev/null && optipng -quiet -o2 "$2" || true; }

echo "==> PNGs base"
for s in "${sizes[@]}"; do render "$s" "$tmp/icon-$s.png"; done

echo "==> Icono embebido de la app (256px)"
mkdir -p "$root/cmd/unmess-app/assets"
cp "$tmp/icon-256.png" "$root/cmd/unmess-app/assets/icon.png"

echo "==> Iconos de estado de bandeja (activo/pausa/error, 22px)"
# A 22px, el tamaño nativo del panel: un pixmap grande reescalado por el host
# SNI se ve diminuto/borroso en GNOME.
rsvg-convert -w 22 -h 22 "$svg"                  -o "$root/cmd/unmess-app/assets/tray-active.png"
rsvg-convert -w 22 -h 22 "$here/tile-paused.svg" -o "$root/cmd/unmess-app/assets/tray-paused.png"
rsvg-convert -w 22 -h 22 "$here/tile-error.svg"  -o "$root/cmd/unmess-app/assets/tray-error.png"
command -v optipng >/dev/null && optipng -quiet -o2 "$root/cmd/unmess-app/assets/tray-active.png" "$root/cmd/unmess-app/assets/tray-paused.png" "$root/cmd/unmess-app/assets/tray-error.png" || true

echo "==> Tema de iconos Linux (hicolor)"
for s in 16 24 32 48 64 128 256 512; do
  d="$root/packaging/linux/icons/hicolor/${s}x${s}/apps"
  mkdir -p "$d"; cp "$tmp/icon-$s.png" "$d/unmessai.png"
done
mkdir -p "$root/packaging/linux/icons/hicolor/scalable/apps"
cp "$svg" "$root/packaging/linux/icons/hicolor/scalable/apps/unmessai.svg"

echo "==> Windows .ico"
icotool -c -o "$here/icon.ico" \
  "$tmp/icon-16.png" "$tmp/icon-24.png" "$tmp/icon-32.png" \
  "$tmp/icon-48.png" "$tmp/icon-64.png" "$tmp/icon-128.png" "$tmp/icon-256.png"

echo "==> macOS .icns"
png2icns "$here/icon.icns" \
  "$tmp/icon-16.png" "$tmp/icon-32.png" "$tmp/icon-48.png" \
  "$tmp/icon-128.png" "$tmp/icon-256.png" "$tmp/icon-512.png" "$tmp/icon-1024.png" >/dev/null

echo "OK"
