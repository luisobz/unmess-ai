// store.js — store reactivo mínimo, sin dependencias.
//
// Un único objeto de estado inmutable por convención: las mutaciones pasan por
// set(patch), que fusiona superficialmente y notifica a los suscriptores. Las
// vistas se suscriben a las claves que les importan con watchKeys, de modo que
// solo se repintan cuando cambia su porción del estado. Sustituye al objeto
// `state` mutable global del monolito anterior por un flujo de datos explícito.

// createStore crea un store a partir de un estado inicial.
export function createStore(initial) {
  let state = initial;
  const subscribers = new Set();

  // get devuelve el estado actual (no mutar directamente).
  const get = () => state;

  // set fusiona un parche (objeto o función state→objeto) y notifica una vez.
  const set = (patch) => {
    const delta = typeof patch === "function" ? patch(state) : patch;
    state = Object.assign({}, state, delta);
    for (const fn of subscribers) fn(state);
  };

  // subscribe registra un oyente de cualquier cambio; devuelve la baja.
  const subscribe = (fn) => {
    subscribers.add(fn);
    return () => subscribers.delete(fn);
  };

  return { get, set, subscribe };
}

// watchKeys ejecuta fn cuando cambia alguna de las claves observadas (comparación
// por identidad). Dispara una sola vez por set aunque cambien varias claves a la
// vez, lo que evita repintados y peticiones duplicadas. Con immediate:true (por
// defecto) ejecuta fn una vez al suscribirse para el render inicial.
export function watchKeys(store, keys, fn, { immediate = true } = {}) {
  let prev = pick(store.get(), keys);
  if (immediate) fn(store.get(), null);
  return store.subscribe((s) => {
    const next = pick(s, keys);
    if (!shallowEqual(prev, next)) {
      const old = prev;
      prev = next;
      fn(s, old);
    }
  });
}

function pick(obj, keys) {
  const out = {};
  for (const k of keys) out[k] = obj[k];
  return out;
}

function shallowEqual(a, b) {
  for (const k in a) if (!Object.is(a[k], b[k])) return false;
  return true;
}
