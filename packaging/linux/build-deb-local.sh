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

VERSION="${1:-0.1.0-dev}"
ARCH=amd64
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/dist"
PKG="$OUT/unmessai-dev_${VERSION}_${ARCH}"

rm -rf "$PKG"
mkdir -p "$PKG/DEBIAN" "$PKG/usr/bin"

echo "==> Compilando daemon y CLI (Go puro, modo dev)"
(cd "$ROOT" && GOOS=linux GOARCH=$ARCH CGO_ENABLED=0 \
  go build -trimpath \
    -ldflags="-s -w -X 'main.devMode=true' -X 'main.version=${VERSION}'" \
    -o "$PKG/usr/bin/unmessd-dev" ./cmd/unmessd && \
  go build -trimpath \
    -ldflags="-s -w -X 'main.devMode=true' -X 'main.version=${VERSION}'" \
    -o "$PKG/usr/bin/unmess-dev" ./cmd/unmess)

cat > "$PKG/DEBIAN/control" <<EOF
Package: unmessai-dev
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: unmessai <hola@unmess.ai>
Depends: libc6
Description: unmessai — versión de desarrollo local
 Versión de pruebas con config y store independientes (puerto 48222,
 store en ~/UnmessaiBackups-dev). No colisiona con unmessai de producción.
EOF

cat > "$PKG/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
pkill unmessd-dev 2>/dev/null || true
nohup /usr/bin/unmessd-dev >/dev/null 2>&1 &
echo "unmessai-dev instalado en puerto 48222. Ejecuta: unmess-dev ui"
EOF
chmod 0755 "$PKG/DEBIAN/postinst"

dpkg-deb --build --root-owner-group "$PKG" "$OUT/unmessai-dev_${VERSION}_${ARCH}.deb"
echo "OK: $OUT/unmessai-dev_${VERSION}_${ARCH}.deb"
