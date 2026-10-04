# Frontend de unmessai

UI web local del daemon: navegación de ficheros y versiones, diff coloreado,
modo Agente (agentes → sesiones → traza → reversión), actividad y ajustes.

**Sin frameworks y sin build step.** Es JavaScript modular con **ES modules
nativos** (`import`/`export`), servido tal cual y embebido en el binario del
daemon con `go:embed` (ver `embed.go`). No hay npm, ni bundler, ni `dist/`: el
daemon se sigue cross-compilando desde cualquier SO con un solo `go build`.

## Estructura

```
ui/
  index.html            Shell semántico y accesible (ARIA). Carga src/main.js como módulo.
  styles/               CSS por dominio; tokens de diseño en styles/common.css.
  assets/               Iconos.
  src/
    main.js             Bootstrap: infraestructura, features, login, carga inicial, refrescos.
    state.js            Store central: única fuente de verdad del estado de la UI.
    core/               Infraestructura reutilizable, agnóstica de la app.
      dom.js            h() (hiperscript declarativo) + helpers ($, mount, clear…).
      store.js          Store reactivo (createStore) + watchKeys.
      api.js            Cliente HTTP autenticado + refresco de token ante 401.
      i18n.js           Traducciones (es/en) y volcado a los atributos data-i18n.
      format.js         fmtBytes, fmtDate.
      router.js         Enrutado por hash (#/file/… , #/agents).
      sse.js            Stream de eventos en vivo (/api/events) con reconexión.
      theme.js  toast.js  overlay.js
    components/
      common.js         Piezas compartidas: badges, chips, líneas de diff, mensajes de visor.
    features/           Un módulo por dominio, con sus acciones y su render.
      header.js         Estadísticas, pausa/reanudación, flush, proteger existentes.
      views.js          Conmutación Versionado ↔ Agente (reactiva al store).
      files.js          Lista, versiones, visor diff/contenido, restaurar, dejar de proteger.
      agents.js         Modo Agente completo, incluida la reversión de sesión.
      settings.js       Modal de ajustes (config del daemon, idioma, purga).
      activity.js       Panel de actividad (journal).
```

## Flujo de datos

Estado inmutable por convención en un único store (`state.js`). El patrón es:

1. Una **acción** (normalmente `fetch` + lógica) muta el estado con `setState(patch)`.
2. `setState` fusiona el parche y notifica a los suscriptores **una vez**.
3. Las **vistas** se suscriben solo a las claves que les importan con
   `watchKeys(store, [claves], render)`, así que se repintan únicamente cuando
   cambia su porción del estado. Agrupar todos los cambios de una acción en un
   solo `setState` evita repintados y peticiones duplicadas.

El render es **declarativo** vía `h(tag, props, ...children)` en lugar de
`document.createElement`; las listas se repintan enteras (no hay virtual DOM),
suficiente para una UI local de este tamaño.

```js
// acción
async function refreshFiles() {
  const files = await apiJSON("/api/files?...");
  setState({ files });               // notifica una vez
}
// vista: se repinta solo si cambian estas claves
watchKeys(store, ["files", "selectedPath", "lang"], renderFileList);
```

El token de sesión **no** vive en el store: lo gestiona `core/api.js`, que
reintenta una vez tras refrescarlo si el daemon devuelve 401 (p. ej. tras un
reinicio del daemon).

### Idioma

`I18N.setLang()` reaplica los textos estáticos (`data-i18n`) y emite
`i18n:changed`; `main.js` sincroniza `store.lang`, de modo que las vistas que
incluyen `"lang"` en sus `watchKeys` retraducen su texto dinámico de forma
reactiva.

## Añadir una feature

1. Crea `src/features/<nombre>.js` con un `init<Nombre>()` que cablee sus
   controles y suscriba sus repintados con `watchKeys`.
2. Si necesita estado compartido, añádelo a `state.js`; si es local y transitorio,
   déjalo en el módulo.
3. Cablea `init<Nombre>()` en `src/main.js`.

## Estilos

Tokens de diseño (color, radios, sombras) como custom properties en
`styles/common.css`. La paleta clara/oscura se define una sola vez con
`light-dark()`; el tema efectivo lo decide `color-scheme`, que `theme.js` fija
mediante el atributo `data-theme` (o sigue la preferencia del SO si no hay
ninguno).

## Desarrollo y verificación

No hay paso de compilación: editar un fichero de `src/` y recargar basta **si se
sirve el directorio en vivo**. El daemon sirve la copia **embebida**, así que para
ver cambios a través de `unmessd` hay que recompilarlo (`go build ./cmd/unmessd`).

Comprobaciones útiles:

```sh
node --check src/**/*.js     # sintaxis de cada módulo (sin ejecutarlo)
go test ./internal/api/...   # el servidor sirve index.html y el grafo de módulos
```
