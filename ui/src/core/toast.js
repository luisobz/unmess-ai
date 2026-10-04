// toast.js — notificaciones efímeras en la esquina.
import { h } from "./dom.js";

let container = null;

// initToasts fija el contenedor donde se apilan los avisos.
export function initToasts(el) {
  container = el;
}

// toast muestra un aviso; kind "error" lo marca visualmente. Se desvanece solo.
export function toast(msg, kind) {
  if (!container) return;
  const el = h("div", {
    class: "toast" + (kind === "error" ? " error" : ""),
    role: "status",
    text: msg,
  });
  container.appendChild(el);
  setTimeout(() => {
    el.style.transition = "opacity .3s";
    el.style.opacity = "0";
  }, 3500);
  setTimeout(() => el.remove(), 4000);
}
