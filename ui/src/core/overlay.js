// overlay.js — apertura/cierre de modales y drawers con gestión de foco.
//
// Cada overlay es <div id="<name>-overlay" class="overlay" hidden>. Los botones
// con data-close="<name>" cierran el suyo; clic en el fondo o Escape cierran el
// que esté abierto. Al abrir se guarda el foco previo y se lo devuelve al cerrar.
import { $ } from "./dom.js";

let lastFocus = null;

// openOverlay muestra un overlay y enfoca su primer control.
export function openOverlay(name) {
  lastFocus = document.activeElement;
  const ov = $(name + "-overlay");
  if (!ov) return;
  ov.hidden = false;
  const focusable = ov.querySelector("input, button, select, textarea, [tabindex]");
  if (focusable) focusable.focus();
}

// closeOverlay oculta un overlay y devuelve el foco a quien lo abrió.
export function closeOverlay(name) {
  const ov = $(name + "-overlay");
  if (ov) ov.hidden = true;
  if (lastFocus && lastFocus.focus) lastFocus.focus();
}

// initOverlays cablea los cierres comunes: botones data-close, clic en el fondo
// y tecla Escape.
export function initOverlays() {
  document.querySelectorAll("[data-close]").forEach((btn) => {
    btn.addEventListener("click", () => closeOverlay(btn.dataset.close));
  });
  document.querySelectorAll(".overlay").forEach((ov) => {
    ov.addEventListener("click", (e) => {
      if (e.target === ov) ov.hidden = true;
    });
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      document.querySelectorAll(".overlay:not([hidden])").forEach((ov) => {
        ov.hidden = true;
      });
    }
  });
}
