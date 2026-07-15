#!/usr/bin/env bash
# Construye un .deb de desarrollo local que NO colisiona con la instalación
# de producción:
#   - Nombre de paquete: unmessai-dev
#   - Binarios: unmessd-dev, unmess-dev
#   - Puerto de UI: 48222
#   - Store: ~/UnmessaiBackups-dev
#   - Config: ~/.config/unmessai-dev/config.toml
#
# Uso: packaging/linux/build-deb-local.sh [version]
set -euo pipefail

VERSION="${1:-$(cat .version)-dev}"
ARCH=amd64
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/dist"
PKG="$OUT/unmessai-dev_${VERSION}_${ARCH}"

rm -rf "$PKG"
mkdir -p "$PKG/DEBIAN" "$PKG/usr/bin" \
         "$PKG/usr/share/doc/unmessai-dev" \
         "$PKG/usr/share/metainfo"

echo "==> Compilando daemon y CLI (Go puro, modo dev)"
(cd "$ROOT" && GOOS=linux GOARCH=$ARCH CGO_ENABLED=0 \
  go build -trimpath \
    -ldflags="-s -w -X 'main.devMode=true' -X 'main.version=${VERSION}'" \
    -o "$PKG/usr/bin/unmessd-dev" ./cmd/unmessd && \
  go build -trimpath \
    -ldflags="-s -w -X 'main.devMode=true' -X 'main.version=${VERSION}'" \
    -o "$PKG/usr/bin/unmess-dev" ./cmd/unmess)

# La Description debe ser solo ASCII: el App Center (PackageKit) muestra los
# caracteres no ASCII de un .deb local como "?". El texto con acentos correcto
# vive en el metainfo AppStream.
cat > "$PKG/DEBIAN/control" <<EOF
Package: unmessai-dev
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: unmessai <hola@unmessai.com>
Homepage: https://unmessai.com
Depends: libc6
Description: unmessai (dev) - build local de pruebas
 Paquete de desarrollo con config y store separados (puerto 48222,
 store en ~/UnmessaiBackups-dev). No interfiere con el paquete
 unmessai estable.
EOF

cat > "$PKG/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
# postinst corre como root durante la instalación: NO arrancar el daemon
# aquí, o quedaría corriendo como root para siempre (~ se expandiría a
# /root en vez de al home del usuario). Solo matamos cualquier daemon viejo
# que pudiera haber quedado de una instalación previa con este bug;
# `unmess-dev ui` arranca el daemon correcto como el usuario actual.
pkill unmessd-dev 2>/dev/null || true
echo "unmessai-dev instalado en puerto 48222. Ejecuta: unmess-dev ui"
EOF
chmod 0755 "$PKG/DEBIAN/postinst"

echo "==> Metadatos AppStream + copyright"
META="$PKG/usr/share/metainfo/ai.unmess.unmessai-dev.metainfo.xml"
sed -e "s/@VERSION@/$VERSION/g" -e "s/@DATE@/$(date -u +%F)/g" \
  "$ROOT/packaging/linux/ai.unmess.unmessai-dev.metainfo.xml" > "$META"

cp "$ROOT/packaging/linux/copyright" "$PKG/usr/share/doc/unmessai-dev/copyright"

dpkg-deb --build --root-owner-group "$PKG" "$OUT/unmessai-dev_${VERSION}_${ARCH}.deb"
echo "OK: $OUT/unmessai-dev_${VERSION}_${ARCH}.deb"
