# Linux

- `build-deb.sh <version>` construye el `.deb` (requiere `dpkg-deb`). Instala en `/usr/bin`
  `unmessd`, `unmess` y `unmess-app` (app nativa), los iconos hicolor, el lanzador
  (`unmessai.desktop`) y el autoarranque de la bandeja (`/etc/xdg/autostart`).
- Para compilar `unmess-app` hacen falta las librerías de WebView/bandeja:
  `sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev`.
  El `.deb` declara sus equivalentes de runtime en `Depends`.
- La app abre con `unmess ui` o desde el lanzador; se autoarranca en la bandeja al iniciar
  sesión. Muestra icono en la bandeja/barra superior (AppIndicator/StatusNotifierItem).
- Para un daemon sin interfaz, `unmess service install` registra la unidad systemd de
  usuario `~/.config/systemd/user/unmessai.service` (sin root).
- Flatpak: en evaluación. No incluido en v1.
- Iconos: se generan con `build/appicon/generate.sh` en
  `packaging/linux/icons/hicolor/...` (ver `build/appicon/README.md`).
