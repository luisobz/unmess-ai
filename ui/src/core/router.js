// router.js — enrutado por hash, minimalista.
//
// Rutas:
//   #/agents            → modo Agente
//   #/file/<ruta>       → modo Versionado con un fichero abierto
//   (vacío)             → modo Versionado sin selección
// El router solo parsea y notifica; la reconciliación con el estado (qué vista
// mostrar, qué fichero abrir) la hace quien registra onRouteChange, en main.js.

// parseHash traduce el hash actual a una ruta estructurada.
export function parseHash() {
  const h = location.hash;
  if (h.startsWith("#/agents")) return { view: "agents", path: null };
  const prefix = "#/file/";
  if (h.startsWith(prefix)) {
    return { view: "files", path: decodeURI(h.slice(prefix.length)) };
  }
  return { view: "files", path: null };
}

// fileHash construye el hash de un fichero.
export function fileHash(path) {
  return "#/file/" + encodeURI(path);
}

// navigate escribe una ruta en la URL. push:false usa replaceState para no
// ensuciar el historial (p. ej. al limpiar la selección).
export function navigate(hash, { push = true } = {}) {
  if (push) {
    location.hash = hash;
  } else if (hash) {
    history.replaceState(null, "", location.pathname + hash);
  } else {
    history.replaceState(null, "", location.pathname);
  }
}

// onRouteChange registra un oyente de cambios de hash.
export function onRouteChange(fn) {
  window.addEventListener("hashchange", () => fn(parseHash()));
}
