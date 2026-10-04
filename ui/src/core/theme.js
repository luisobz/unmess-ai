// theme.js — tema claro/oscuro con preferencia persistida.
import { $ } from "./dom.js";

const STORAGE_KEY = "unmessai-theme";

// applyTheme fija (o quita) el atributo data-theme y actualiza el icono del botón.
function applyTheme(theme) {
  if (theme) document.documentElement.setAttribute("data-theme", theme);
  else document.documentElement.removeAttribute("data-theme");
  const isDark =
    theme === "dark" ||
    (!theme && window.matchMedia("(prefers-color-scheme: dark)").matches);
  const icon = $("theme-icon");
  if (icon) icon.textContent = isDark ? "☀️" : "🌙";
}

// initTheme aplica la preferencia guardada y cablea el botón de alternar.
export function initTheme() {
  applyTheme(localStorage.getItem(STORAGE_KEY));
  const btn = $("btn-theme");
  if (!btn) return;
  btn.addEventListener("click", () => {
    const cur = document.documentElement.getAttribute("data-theme");
    const isDark =
      cur === "dark" ||
      (!cur && window.matchMedia("(prefers-color-scheme: dark)").matches);
    const next = isDark ? "light" : "dark";
    localStorage.setItem(STORAGE_KEY, next);
    applyTheme(next);
  });
}
