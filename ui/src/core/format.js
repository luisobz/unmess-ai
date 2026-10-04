// format.js — formateo de valores para la UI (tamaños y fechas).
import { I18N } from "./i18n.js";

// fmtBytes formatea un número de bytes en unidades binarias legibles.
export function fmtBytes(n) {
  if (n == null) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return (i === 0 ? v : v.toFixed(1)) + " " + units[i];
}

// fmtDate formatea una fecha ISO en el locale actual. Si no es válida, devuelve
// la cadena original en lugar de "Invalid Date".
export function fmtDate(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  return d.toLocaleString(I18N.lang(), { dateStyle: "medium", timeStyle: "short" });
}
