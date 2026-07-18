#!/usr/bin/env bash
# Limpia el entorno de desarrollo de unmessai-dev:
#   - Para el daemon y quita la unidad systemd
#   - Mata procesos huérfanos
#   - apt purge del paquete
#   - (opcional) borra config y store
#
# Uso: packaging/linux/clean-dev.sh [--hard]
set -euo pipefail

HARD=false
for arg in "$@"; do
  [ "$arg" = "--hard" ] && HARD=true
done

echo "==> Parando daemon dev..."
systemctl --user stop unmessai-dev.service 2>/dev/null || true
systemctl --user disable unmessai-dev.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/unmessai-dev.service"
systemctl --user daemon-reload 2>/dev/null || true

# Limpiar también la unidad antigua (unmessai.service) que se generaba antes
# de diferenciar dev/prod. Quitar cuando ya nadie tenga la versión antigua.
systemctl --user stop unmessai.service 2>/dev/null || true
systemctl --user disable unmessai.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/unmessai.service"
systemctl --user daemon-reload 2>/dev/null || true

echo "==> Matando procesos huérfanos..."
pkill -f unmessd-dev 2>/dev/null || true
pkill -f unmess-dev 2>/dev/null || true

echo "==> Purgando paquete..."
if dpkg -s unmessai-dev >/dev/null 2>&1; then
  sudo apt purge -y unmessai-dev
fi

if $HARD; then
  echo "==> Borrando config y store dev..."
  rm -rf "$HOME/.config/unmessai-dev" "$HOME/UnmessaiBackups-dev"
  echo "==> Entorno dev limpio (--hard: config y store borrados)"
else
  echo "==> Entorno dev limpio (config y store conservados)"
fi
