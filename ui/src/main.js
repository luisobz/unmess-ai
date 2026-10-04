// main.js — arranque de la UI de unmessai.
//
// Orquesta el bootstrap: infraestructura (i18n, tema, overlays, toasts), cableado
// de cada feature, autenticación local, carga inicial y los dos motores de
// actualización (polling de respaldo cada 15 s y stream de eventos en vivo).
import { $ } from "./core/dom.js";
import { fetchToken } from "./core/api.js";
import { I18N, t } from "./core/i18n.js";
import { initTheme } from "./core/theme.js";
import { initOverlays, openOverlay } from "./core/overlay.js";
import { initToasts, toast } from "./core/toast.js";
import { parseHash, onRouteChange } from "./core/router.js";
import { connectEvents } from "./core/sse.js";
import { getState, setState } from "./state.js";
import { initHeader, refreshStatus, setPaused } from "./features/header.js";
import { initViews, setView } from "./features/views.js";
import { initFiles, refreshFiles, selectFile } from "./features/files.js";
import {
  initAgents,
  loadAgents,
  liveRefreshAgents,
} from "./features/agents.js";
import { initSettings } from "./features/settings.js";
import {
  initActivity,
  activityOpen,
  refreshJournal,
} from "./features/activity.js";

const REFRESH_MS = 15000;
const LIVE_COALESCE_MS = 400;

async function init() {
  // Infraestructura.
  initToasts($("toast-container"));
  I18N.apply();
  initTheme();
  initOverlays();

  // El cambio de idioma sincroniza el store para que las vistas se repinten.
  document.addEventListener("i18n:changed", () =>
    setState({ lang: I18N.lang() }),
  );

  // Features.
  initHeader();
  initViews();
  initFiles();
  initAgents();
  initSettings();
  initActivity();
  $("btn-about").addEventListener("click", () => openOverlay("about"));

  // Enrutado por hash.
  onRouteChange(applyRoute);

  // Autenticación local: sin token no hay nada que cargar.
  try {
    await fetchToken();
  } catch (e) {
    toast(t("login_error", { msg: e.message }), "error");
    return;
  }

  // Carga inicial.
  await loadAgents();
  await refreshStatus();
  await refreshFiles();
  applyRoute(parseHash());

  // Motores de actualización.
  initAutoRefresh();
  connectEvents(handleLiveEvent);
}

// applyRoute reconcilia el estado con una ruta (sin volver a tocar el hash).
function applyRoute(route) {
  if (getState().view !== route.view)
    setView(route.view, { updateHash: false });
  if (route.path && route.path !== getState().selectedPath) {
    selectFile(route.path, { updateHash: false });
  }
}

// initAutoRefresh es el polling de respaldo: refresca cuando la pestaña está
// visible, por si algún evento en vivo se perdiera.
function initAutoRefresh() {
  setInterval(() => {
    if (document.visibilityState !== "visible") return;
    refreshFiles();
    refreshStatus();
    if (activityOpen()) refreshJournal();
    liveRefreshAgents();
  }, REFRESH_MS);
}

// scheduleLiveRefresh agrupa ráfagas de eventos en un único refresco.
let liveTimer = null;
function scheduleLiveRefresh() {
  if (liveTimer) return;
  liveTimer = setTimeout(() => {
    liveTimer = null;
    refreshFiles();
    refreshStatus();
    if (activityOpen()) refreshJournal();
    const sel = getState().selectedPath;
    if (sel)
      selectFile(sel, { updateHash: false, preserve: true }).catch(() => {});
    liveRefreshAgents();
  }, LIVE_COALESCE_MS);
}

function handleLiveEvent(ev) {
  switch (ev.type) {
    case "versioned":
    case "restored":
    case "pruned":
    case "protected":
      scheduleLiveRefresh();
      break;
    case "paused":
      setPaused(true);
      break;
    case "resumed":
      setPaused(false);
      break;
    case "error":
      if (ev.message) toast(ev.message, "error");
      break;
  }
}

document.addEventListener("DOMContentLoaded", init);
