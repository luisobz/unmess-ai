# Icono de la app

Fuentes vectoriales (SVG) de las que `generate.sh` deriva todos los formatos por SO.

## Diseño

El icono es un escudo de "restaurar": tile con degradado + una flecha circular de
rebobinado que envuelve un símbolo central. Se vectorizó (potrace) a partir del
arte aprobado y se recompuso como SVG para escalar sin pérdida.

- `icon.svg` — **icono de la app** (tile oscuro casi negro + check, a juego con la
  identidad monocroma del sitio). Fuente única del icono de
  aplicación: de aquí salen `cmd/unmess-app/assets/icon.png`, los iconos hicolor de
  Linux, el `.ico` (Windows) y el `.icns` (macOS), y el favicon/logo de la UI web.
- `tile-paused.svg` — tile ámbar + pausa → icono de bandeja **en pausa**.
- `tile-error.svg` — tile rojo + exclamación → icono de bandeja **en error** (parpadeo).
- `glyph-active.svg` / `glyph-paused.svg` / `glyph-error.svg` — glifos monocromos
  sueltos (para un icono "template" de la barra de menú de macOS u otros usos).

Los tres estados comparten la misma flecha de rebobinado y solo cambian el símbolo
central (check / pausa / exclamación), de modo que se leen como una familia.

## Regenerar

Tras editar cualquier `*.svg`:

```sh
bash build/appicon/generate.sh
```

Genera los PNG hicolor, el `.ico`, el `.icns` y los iconos de estado de bandeja
(`cmd/unmess-app/assets/tray-paused.png` y `tray-error.png`). Requisitos:
`rsvg-convert` (librsvg2-bin), `icotool` (icoutils), `png2icns` (icnsutils);
`optipng` opcional.

## Notas

- El icono de bandeja **activo** reutiliza `icon.png` (el tile azul). Pausa y error
  usan los tiles ámbar/rojo; el estado también se refleja en el tooltip.
- La app usa colores de tile (siempre visibles sobre cualquier panel). Si en el
  futuro se quiere un icono monocromo que se adapte a temas claro/oscuro del panel,
  los `glyph-*.svg` son el punto de partida (p. ej. `SetTemplateIcon`/`SetDarkModeIcon`).
