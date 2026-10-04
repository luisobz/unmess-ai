// dom.js — utilidades de creación y manipulación de DOM.
//
// h() es un pequeño hiperscript declarativo que sustituye a las cadenas de
// document.createElement/appendChild dispersas por la UI: describe un elemento
// (etiqueta, props, hijos) y devuelve el nodo listo para montar. No hay virtual
// DOM ni diffing; las listas se repintan enteras, que es más que suficiente para
// una UI local de este tamaño y mantiene el modelo mental simple.

// h crea un elemento. props admite:
//   class            → className
//   text             → textContent
//   html             → innerHTML (usar solo con contenido controlado)
//   dataset {..}     → data-* en bloque
//   style {..}       → estilos inline en bloque
//   onClick, onInput → addEventListener("click"/"input", fn)
//   value/checked/disabled/hidden/selected → propiedad (no atributo)
//   cualquier otro   → setAttribute (role, aria-*, tabindex, type…)
// Un valor null/false/undefined en una prop se ignora.
export function h(tag, props, ...children) {
  const el = document.createElement(tag);
  if (props) {
    for (const key in props) {
      const v = props[key];
      if (v == null || v === false) continue;
      if (key === "class") el.className = v;
      else if (key === "text") el.textContent = v;
      else if (key === "html") el.innerHTML = v;
      else if (key === "dataset") Object.assign(el.dataset, v);
      else if (key === "style" && typeof v === "object") Object.assign(el.style, v);
      else if (key[0] === "o" && key[1] === "n" && typeof v === "function") {
        el.addEventListener(key.slice(2).toLowerCase(), v);
      } else if (PROP_KEYS.has(key)) {
        el[key] = v;
      } else {
        el.setAttribute(key, v === true ? "" : v);
      }
    }
  }
  append(el, children);
  return el;
}

// Props que se asignan como propiedad del elemento y no como atributo, porque su
// reflejo como atributo no es fiable (p. ej. value tras interacción del usuario).
const PROP_KEYS = new Set(["value", "checked", "disabled", "hidden", "selected"]);

// append añade hijos: strings/números como texto, nodos tal cual, arrays de forma
// recursiva; ignora null/false/undefined para permitir `cond && h(...)`.
function append(el, children) {
  for (const c of children) {
    if (c == null || c === false || c === true) continue;
    if (Array.isArray(c)) append(el, c);
    else if (c instanceof Node) el.appendChild(c);
    else el.appendChild(document.createTextNode(String(c)));
  }
}

// $ es el atajo clásico a getElementById.
export function $(id) {
  return document.getElementById(id);
}

// clear vacía un contenedor.
export function clear(el) {
  el.replaceChildren();
}

// mount vacía un contenedor y monta en él los nodos indicados.
export function mount(el, ...children) {
  clear(el);
  append(el, children);
}

// show/hide alternan el atributo hidden de un elemento.
export function show(el, visible) {
  el.hidden = !visible;
}

// on registra un listener y devuelve una función para quitarlo.
export function on(el, type, handler, opts) {
  el.addEventListener(type, handler, opts);
  return () => el.removeEventListener(type, handler, opts);
}

// activateOnEnter hace que un elemento no nativo (role="button") se active con
// Enter o Espacio, replicando el comportamiento de un botón real.
export function activateOnEnter(el, fn) {
  el.addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      fn(e);
    }
  });
}
