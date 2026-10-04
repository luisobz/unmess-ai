// features/header.js — cabecera: estadísticas del store, pausa/reanudación de la
// protección, "Versionar ahora" (flush) y "Proteger existentes".
import { $ } from "../core/dom.js";
import { apiJSON, jsonBody } from "../core/api.js";
import { fmtBytes } from "../core/format.js";
import { t } from "../core/i18n.js";
import { toast } from "../core/toast.js";
import { openOverlay, closeOverlay } from "../core/overlay.js";
import { watchKeys } from "../core/store.js";
import { store, getState, setState } from "../state.js";
import { refreshFiles } from "./files.js";

// initHeader cablea los controles de la cabecera y suscribe sus repintados.
export function initHeader() {
  $("btn-pause").addEventListener("click", togglePause);
  $("btn-flush").addEventListener("click", flushNow);
  $("btn-protect-existing").addEventListener("click", openProtect);
  $("protect-confirm").addEventListener("click", doProtect);
  $("protect-path").addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      doProtect();
    }
  });

  // "lang" entra en las claves para retraducir el texto dinámico (detalle de
  // almacenamiento, etiqueta de protección) al cambiar de idioma.
  watchKeys(store, ["status", "lang"], renderStatus);
  watchKeys(store, ["paused", "lang"], renderProtection);
}

// refreshStatus consulta /api/status y vuelca el resultado al estado.
export async function refreshStatus() {
  try {
    const s = await apiJSON("/api/status");
    setState({ status: s, paused: !!s.paused });
  } catch (_) {
    // silencioso en refrescos
  }
}

// setPaused refleja el estado de vigilancia (lo usan los eventos en vivo).
export function setPaused(paused) {
  setState({ paused });
}

function renderStatus() {
  const s = getState().status;
  if (!s) return;
  $("stat-size").textContent = fmtBytes(s.store_size_bytes);
  $("stat-files").textContent = s.files_tracked;
  $("about-version").textContent = s.version || "—";
  $("storage-detail").textContent = t("storage_used", {
    size: fmtBytes(s.store_size_bytes),
    files: s.files_tracked,
  });
}

// renderProtection refleja el estado de pausa en la píldora de la cabecera.
function renderProtection() {
  const paused = getState().paused;
  $("prot-label").textContent = paused ? t("protection_paused") : t("protection_active");
  $("btn-pause").setAttribute("aria-pressed", paused ? "true" : "false");
}

async function togglePause() {
  const next = !getState().paused;
  try {
    const j = await apiJSON("/api/pause", jsonBody("POST", { paused: next }));
    setPaused(!!j.paused);
    toast(j.paused ? t("paused_toast") : t("resumed_toast"));
  } catch (e) {
    toast(e.message, "error");
  }
}

// flushNow fuerza el versionado inmediato de los cambios pendientes del debounce.
async function flushNow() {
  const btn = $("btn-flush");
  btn.classList.add("spinning");
  try {
    const j = await apiJSON("/api/flush", { method: "POST" });
    toast(j.flushed > 0 ? t("flush_done", { n: j.flushed }) : t("flush_none"));
  } catch (e) {
    toast(t("flush_error", { msg: e.message }), "error");
  } finally {
    btn.classList.remove("spinning");
  }
}

function openProtect() {
  $("protect-path").value = "~";
  openOverlay("protect");
}

// doProtect pide al daemon una pasada de protección inicial: versión baseline de
// los ficheros bajo la ruta indicada que aún no tienen historial.
async function doProtect() {
  const path = $("protect-path").value.trim();
  if (!path) return;
  const btn = $("protect-confirm");
  btn.disabled = true;
  btn.textContent = t("protect_running");
  try {
    const j = await apiJSON("/api/protect", jsonBody("POST", { path }));
    closeOverlay("protect");
    if (j.protected > 0) {
      toast(t("protect_done", { n: j.protected, m: j.existing }));
    } else if (j.scanned === 0) {
      toast(t("protect_none_matched"));
    } else {
      toast(t("protect_all_tracked", { m: j.existing }));
    }
    await refreshFiles();
    await refreshStatus();
  } catch (e) {
    toast(t("protect_error", { msg: e.message }), "error");
  } finally {
    btn.disabled = false;
    btn.textContent = t("protect_confirm");
  }
}
