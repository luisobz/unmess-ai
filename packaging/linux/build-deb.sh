#!/usr/bin/env bash
# Construye el paquete .deb de unmessai:
#   - unmessd  (daemon, Go puro)
#   - unmess   (CLI, Go puro)
#   - unmess-app (app nativa: bandeja + ventana WebView + notificaciones; Wails/CGO)
#   - iconos hicolor, lanzador y autoarranque de la bandeja.
#
# Uso: packaging/linux/build-deb.sh <version>   (p. ej. 0.1.0)
#
# Requisitos para compilar unmess-app en esta máquina (WebKitGTK/GTK3):
#   sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev
# Requiere además dpkg-deb.
set -euo pipefail

VERSION="${1:?uso: build-deb.sh <version>}"
ARCH=amd64
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/dist"
PKG="$OUT/unmessai_${VERSION}_${ARCH}"

rm -rf "$PKG"
mkdir -p "$PKG/DEBIAN" "$PKG/usr/bin" "$PKG/usr/share/applications" \
         "$PKG/etc/xdg/autostart" "$PKG/usr/share/doc/unmessai" \
         "$PKG/usr/share/metainfo"

echo "==> Compilando daemon y CLI (Go puro)"
(cd "$ROOT" && GOOS=linux GOARCH=$ARCH CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o "$PKG/usr/bin/" ./cmd/unmessd ./cmd/unmess)

echo "==> Compilando app nativa (Wails v3, GTK3/WebKit2GTK-4.1)"
(cd "$ROOT" && CGO_ENABLED=1 \
  go build -tags "gui gtk3" -trimpath -ldflags="-s -w" \
  -o "$PKG/usr/bin/unmess-app" ./cmd/unmess-app)

echo "==> Iconos hicolor"
if [ -d "$ROOT/packaging/linux/icons/hicolor" ]; then
  mkdir -p "$PKG/usr/share/icons"
  cp -r "$ROOT/packaging/linux/icons/hicolor" "$PKG/usr/share/icons/"
else
  echo "AVISO: iconos hicolor no encontrados; ejecuta build/appicon/generate.sh" >&2
fi

# La Description debe ser solo ASCII: el App Center (PackageKit) muestra los
# caracteres no ASCII de un .deb local como "?". El texto con acentos correcto
# vive en el metainfo AppStream.
cat > "$PKG/DEBIAN/control" <<EOF
Package: unmessai
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: unmessai <info@unmessai.com>
Homepage: https://unmessai.com
Depends: libc6, libgtk-3-0, libwebkit2gtk-4.1-0, libayatana-appindicator3-1
Recommends: git
Description: Versionado continuo de archivos en segundo plano
 Protege frente a borrados o modificaciones accidentales (incluidos los
 provocados por agentes de IA) versionando cada cambio fuera de repos git
 y permitiendo restaurar versiones anteriores. Local-first.
 Incluye app nativa con icono en la bandeja del sistema, ventana propia y
 notificaciones del escritorio.
EOF

# El autoarranque del daemon por servicio sigue siendo por usuario
# (unmess service install). El autostart de la app (bandeja) se instala como
# entrada XDG y lo arranca la sesión gráfica de cada usuario.
cat > "$PKG/DEBIAN/prerm" <<'EOF'
#!/bin/sh
set -e
# Mata daemon y app nativa antes de desinstalar/actualizar para que el nuevo
# binario (con la UI embebida actualizada) tome el relevo. Si se dejara viva la
# app vieja, su WebView seguiría con el token del daemon anterior y, al arrancar
# el daemon nuevo (token nuevo), toda petición daría 401 "token inválido".
pkill unmessd 2>/dev/null || true
pkill unmess-app 2>/dev/null || true
EOF
chmod 0755 "$PKG/DEBIAN/prerm"

cat > "$PKG/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -q /usr/share/icons/hicolor 2>/dev/null || true
fi
# postinst corre como root durante la instalación: NO arrancar el daemon
# aquí, o quedaría corriendo como root para siempre (~ se expandiría a
# /root en vez de al home del usuario). Solo matamos cualquier daemon o app
# viejos que pudieran haber quedado de una instalación previa (la app vieja
# conservaría el token del daemon anterior y daría 401); el daemon y la app
# correctos los arranca la sesión del usuario (autostart de la bandeja o
# `unmess ui`).
pkill unmessd 2>/dev/null || true
pkill unmess-app 2>/dev/null || true
echo "unmessai instalado. Abre la app desde el lanzador o ejecuta: unmess ui"
EOF
chmod 0755 "$PKG/DEBIAN/postinst"

cp "$ROOT/packaging/linux/unmessai.desktop" "$PKG/usr/share/applications/"
cp "$ROOT/packaging/linux/unmessai-autostart.desktop" "$PKG/etc/xdg/autostart/unmessai.desktop"
cp "$ROOT/README.md" "$PKG/usr/share/doc/unmessai/"

# Metadatos AppStream: fuente autoritativa para el centro de software (nombre,
# editor, licencia y descripción con acentos correctos). Se sustituye la
# versión y la fecha de publicación en la plantilla.
echo "==> Metadatos AppStream + copyright"
META="$PKG/usr/share/metainfo/ai.unmess.unmessai.metainfo.xml"
sed -e "s/@VERSION@/$VERSION/g" -e "s/@DATE@/$(date -u +%F)/g" \
  "$ROOT/packaging/linux/ai.unmess.unmessai.metainfo.xml" > "$META"

# Fichero de copyright en formato legible por máquina (Debian policy). Evita la
# licencia "unknown" en el centro de software y cumple con la política Debian.
cp "$ROOT/packaging/linux/copyright" "$PKG/usr/share/doc/unmessai/copyright"

# Validación no bloqueante del AppStream si hay herramienta disponible.
if command -v appstreamcli >/dev/null 2>&1; then
  appstreamcli validate --no-net "$META" || \
    echo "AVISO: appstreamcli reportó problemas en el metainfo (no bloqueante)" >&2
fi

dpkg-deb --build --root-owner-group "$PKG" "$OUT/unmessai_${VERSION}_${ARCH}.deb"
echo "OK: $OUT/unmessai_${VERSION}_${ARCH}.deb"
