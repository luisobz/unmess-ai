"use strict";

// Estado global de la aplicación.
const state = {
  token: null,
  filter: "all",
  // aiFilter tri-estado: "all" (todos) | "only" (solo IA) | "hide" (sin IA).
  aiFilter: localStorage.getItem("unmessai-ai-filter") || "all",
  agentsById: {}, // id -> {name, icon} para pintar chips de agente
  view: "files",  // "files" (versionado clásico) | "agents" (modo Agente)
  agents: [],     // registro completo de /api/agents (con conteo de ficheros)
  sessionCounts: {}, // id de agente -> nº de sesiones (carga perezosa)
  selAgent: null,
  sessions: [],
  sessionDetail: null, // {session, events, files} de la sesión abierta
  query: "",
  files: [],
  selectedPath: null,
  versions: [],
  selectedVersion: null, // versión "to" seleccionada (la de la izquierda en la lista)
  compareTo: null,       // versión "from" contra la que comparar
  mode: "diff",          // "diff" | "content"
  deleted: false,
  paused: false,
};

const REFRESH_MS = 15000;

// --- helpers de red ---

// tokenRefresh deduplica el refresco del token: si varias peticiones reciben
// 401 a la vez, todas esperan al mismo fetchToken en vuelo en vez de disparar
// una estampida contra /api/token.
let tokenRefresh = null;
function refreshToken() {
  if (!tokenRefresh) {
    tokenRefresh = fetchToken().finally(() => { tokenRefresh = null; });
  }
  return tokenRefresh;
}

// api hace una petición autenticada al daemon. Si el daemon se reinició y
// regeneró el token (p. ej. tras una actualización o un `snap refresh`), la
// primera respuesta será 401: en ese caso refrescamos el token una vez y
// reintentamos, de modo que la sesión se recupera sola sin recargar la página.
async function api(path, opts = {}) {
  const send = () => {
    const headers = Object.assign({}, opts.headers || {});
    if (state.token) headers["Authorization"] = "Bearer " + state.token;
    return fetch(path, Object.assign({}, opts, { headers }));
  };
  let res = await send();
  if (res.status === 401) {
    try {
      await refreshToken();
    } catch (_) {
      return res; // el daemon no responde: devolvemos el 401 original
    }
    res = await send();
  }
  return res;
}

async function apiJSON(path, opts = {}) {
  const res = await api(path, opts);
  if (!res.ok) {
    let msg = "Error " + res.status;
    try {
      const j = await res.json();
      if (j && j.error) msg = j.error;
    } catch (_) {}
    throw new Error(msg);
  }
  return res.json();
}

async function fetchToken() {
  const res = await fetch("/api/token");
  if (!res.ok) throw new Error("token");
  const j = await res.json();
  state.token = j.token;
}

// --- utilidades ---

function $(id) { return document.getElementById(id); }

function fmtBytes(n) {
  if (n == null) return "—";
  const u = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0, v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return (i === 0 ? v : v.toFixed(1)) + " " + u[i];
}

function fmtDate(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  return d.toLocaleString(I18N.lang(), { dateStyle: "medium", timeStyle: "short" });
}

function toast(msg, kind) {
  const el = document.createElement("div");
  el.className = "toast" + (kind === "error" ? " error" : "");
  el.setAttribute("role", "status");
  el.textContent = msg;
  $("toast-container").appendChild(el);
  setTimeout(() => { el.style.opacity = "0"; el.style.transition = "opacity .3s"; }, 3500);
  setTimeout(() => el.remove(), 4000);
}

// --- tema ---

function applyTheme(theme) {
  if (theme) document.documentElement.setAttribute("data-theme", theme);
  else document.documentElement.removeAttribute("data-theme");
  const isDark = theme === "dark" ||
    (!theme && window.matchMedia("(prefers-color-scheme: dark)").matches);
  $("theme-icon").textContent = isDark ? "☀️" : "🌙";
}

function initTheme() {
  const saved = localStorage.getItem("unmessai-theme");
  applyTheme(saved);
  $("btn-theme").addEventListener("click", () => {
    const cur = document.documentElement.getAttribute("data-theme");
    const isDark = cur === "dark" ||
      (!cur && window.matchMedia("(prefers-color-scheme: dark)").matches);
    const next = isDark ? "light" : "dark";
    localStorage.setItem("unmessai-theme", next);
    applyTheme(next);
  });
}

// --- status / cabecera ---

let lastStatus = null;

async function refreshStatus() {
  try {
    const s = await apiJSON("/api/status");
    lastStatus = s;
    renderStatus();
    applyProtection(!!s.paused);
  } catch (e) {
    // silencioso en refrescos
  }
}

function renderStatus() {
  if (!lastStatus) return;
  const s = lastStatus;
  $("stat-size").textContent = fmtBytes(s.store_size_bytes);
  $("stat-files").textContent = s.files_tracked;
  $("about-version").textContent = s.version || "—";
  $("storage-detail").textContent =
    t("storage_used", { size: fmtBytes(s.store_size_bytes), files: s.files_tracked });
}

// applyProtection refleja el estado de vigilancia en la píldora de la cabecera.
function applyProtection(paused) {
  state.paused = paused;
  const label = $("prot-label");
  const btn = $("btn-pause");
  if (label) label.textContent = paused ? t("protection_paused") : t("protection_active");
  if (btn) btn.setAttribute("aria-pressed", paused ? "true" : "false");
}

// togglePause pausa o reanuda la vigilancia en el daemon.
async function togglePause() {
  const next = !state.paused;
  try {
    const j = await apiJSON("/api/pause", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ paused: next }),
    });
    applyProtection(!!j.paused);
    toast(j.paused ? t("paused_toast") : t("resumed_toast"));
  } catch (e) {
    toast(e.message, "error");
  }
}

// flushNow fuerza el versionado inmediato de los cambios pendientes del
// debounce ("Versionar ahora"), sin esperar a que venza el reposo.
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

// --- lista de ficheros ---

async function refreshFiles(preserveSelection = true) {
  let files;
  try {
    const qs = new URLSearchParams({ q: state.query, filter: state.filter });
    files = await apiJSON("/api/files?" + qs.toString());
  } catch (e) {
    toast(e.message, "error");
    return;
  }
  state.files = files;
  renderFiles();
  if (preserveSelection && state.selectedPath) {
    highlightSelectedFile();
  }
}

// --- filtro de IA (tri-estado) ---

const AI_FILTER_CYCLE = { all: "only", only: "hide", hide: "all" };

// loadAgents trae el registro de agentes: chips de la lista de ficheros y
// columna de agentes del modo Agente.
async function loadAgents() {
  try {
    const list = await apiJSON("/api/agents");
    const map = {};
    for (const a of list) map[a.id] = { name: a.name, icon: a.icon };
    state.agentsById = map;
    state.agents = list;
  } catch (_) {
    // sin registro: los chips de agente simplemente no se pintan
  }
}

// visibleFiles aplica el filtro de IA sobre la lista completa ya cargada. El
// filtrado es en cliente: el servidor ya clasificó cada fichero (campo agent).
function visibleFiles() {
  if (state.aiFilter === "only") return state.files.filter((f) => f.agent);
  if (state.aiFilter === "hide") return state.files.filter((f) => !f.agent);
  return state.files;
}

function applyAiToggleUI() {
  const btn = $("btn-ai-filter");
  if (!btn) return;
  btn.classList.toggle("state-only", state.aiFilter === "only");
  btn.classList.toggle("state-hide", state.aiFilter === "hide");
  const key = state.aiFilter === "only" ? "ai_filter_only"
    : state.aiFilter === "hide" ? "ai_filter_hide"
      : "ai_filter_all";
  const label = t(key);
  btn.title = label;
  btn.setAttribute("aria-label", label);
  btn.setAttribute("aria-pressed", state.aiFilter === "all" ? "false" : "true");
}

function cycleAiFilter() {
  state.aiFilter = AI_FILTER_CYCLE[state.aiFilter] || "all";
  localStorage.setItem("unmessai-ai-filter", state.aiFilter);
  applyAiToggleUI();
  renderFiles();
  if (state.selectedPath) highlightSelectedFile();
}

function renderFiles() {
  const list = $("file-list");
  list.innerHTML = "";
  const files = visibleFiles();
  if (files.length === 0) {
    list.hidden = true;
    $("files-empty").hidden = false;
    return;
  }
  list.hidden = false;
  $("files-empty").hidden = true;
  for (const f of files) {
    const li = document.createElement("li");
    li.className = "file-item";
    li.dataset.path = f.path;
    if (f.path === state.selectedPath) li.classList.add("selected");
    li.setAttribute("role", "button");
    li.setAttribute("tabindex", "0");

    const p = document.createElement("div");
    p.className = "fi-path";
    p.textContent = f.path;

    const meta = document.createElement("div");
    meta.className = "fi-meta";
    const vers = document.createElement("span");
    vers.textContent = f.versions + " " + (f.versions === 1 ? t("version_singular") : t("version_plural"));
    const date = document.createElement("span");
    date.textContent = fmtDate(f.last_version_at);
    meta.appendChild(vers);
    meta.appendChild(date);
    if (f.agent) {
      const ag = state.agentsById[f.agent];
      const chip = document.createElement("span");
      chip.className = "fi-agent";
      const ic = document.createElement("span");
      ic.className = "fi-agent-icon";
      ic.textContent = ag ? ag.icon : "✳";
      chip.appendChild(ic);
      chip.appendChild(document.createTextNode(ag ? ag.name : f.agent));
      meta.appendChild(chip);
    }
    if (f.deleted) {
      const b = document.createElement("span");
      b.className = "badge badge-deleted";
      b.textContent = t("deleted_badge");
      meta.appendChild(b);
    }

    li.appendChild(p);
    li.appendChild(meta);
    const open = () => selectFile(f.path);
    li.addEventListener("click", open);
    li.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); open(); }
    });
    list.appendChild(li);
  }
}

function highlightSelectedFile() {
  document.querySelectorAll(".file-item").forEach((el) => {
    el.classList.toggle("selected", el.dataset.path === state.selectedPath);
  });
}

// --- selección de fichero ---

async function selectFile(path, updateHash = true) {
  state.selectedPath = path;
  const f = state.files.find((x) => x.path === path);
  state.deleted = f ? f.deleted : false;
  highlightSelectedFile();
  if (updateHash) location.hash = "#/file/" + encodeURI(path);

  try {
    state.versions = await apiJSON("/api/versions?path=" + encodeURIComponent(path));
  } catch (e) {
    toast(e.message, "error");
    return;
  }
  // Selección por defecto: la versión más reciente (la primera, desc).
  state.selectedVersion = state.versions.length ? state.versions[0].name : null;
  state.compareTo = defaultCompareTo();

  $("panel-empty").hidden = true;
  $("file-view").hidden = false;
  $("file-path").textContent = path;
  $("file-deleted-badge").hidden = !state.deleted;
  $("btn-restore").disabled = state.versions.length === 0;

  renderVersions();
  renderCompareOptions();
  await renderViewer();
}

// clearSelection vuelve al estado sin fichero seleccionado (tras olvidar uno).
function clearSelection() {
  state.selectedPath = null;
  state.versions = [];
  state.selectedVersion = null;
  $("file-view").hidden = true;
  $("panel-empty").hidden = false;
  if (location.hash) history.replaceState(null, "", location.pathname);
  highlightSelectedFile();
}

// Comparación por defecto: la versión anterior a la seleccionada.
// Si no hay anterior (primera versión), no comparamos con nada.
function defaultCompareTo() {
  const idx = state.versions.findIndex((v) => v.name === state.selectedVersion);
  if (idx >= 0 && idx + 1 < state.versions.length) {
    return state.versions[idx + 1].name;
  }
  return null;
}

function renderVersions() {
  const list = $("version-list");
  list.innerHTML = "";
  state.versions.forEach((v, i) => {
    const li = document.createElement("li");
    li.className = "version-item";
    if (i === 0) li.classList.add("current"); // más reciente
    if (v.name === state.selectedVersion) li.classList.add("selected");
    li.setAttribute("role", "button");
    li.setAttribute("tabindex", "0");

    const name = document.createElement("div");
    name.className = "v-name";
    name.textContent = v.name;
    if (i === 0) {
      const b = document.createElement("span");
      b.className = "badge badge-current";
      b.textContent = t("current_badge");
      name.appendChild(b);
    }
    const meta = document.createElement("div");
    meta.className = "v-meta";
    meta.textContent = fmtDate(v.ts) + " · " + fmtBytes(v.size);
    li.appendChild(name);
    li.appendChild(meta);

    const pick = () => {
      state.selectedVersion = v.name;
      state.compareTo = defaultCompareTo();
      renderVersions();
      renderCompareOptions();
      renderViewer();
    };
    li.addEventListener("click", pick);
    li.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); pick(); }
    });
    list.appendChild(li);
  });
}

function renderCompareOptions() {
  const sel = $("compare-to");
  sel.innerHTML = "";
  const optCurrent = document.createElement("option");
  optCurrent.value = "current";
  optCurrent.textContent = t("compare_current");
  if (state.deleted) optCurrent.disabled = true;
  sel.appendChild(optCurrent);
  for (const v of state.versions) {
    if (v.name === state.selectedVersion) continue;
    const o = document.createElement("option");
    o.value = v.name;
    o.textContent = v.name;
    sel.appendChild(o);
  }
  sel.value = state.compareTo || "current";
  sel.onchange = () => { state.compareTo = sel.value; renderViewer(); };
  // El selector de comparación solo aplica en modo diff.
  $("compare-controls").style.visibility = state.mode === "diff" ? "visible" : "hidden";
}

// --- visor (diff / contenido) ---

function viewerMessage(viewer, msg) {
  viewer.innerHTML = "";
  const div = document.createElement("div");
  div.className = "viewer-empty";
  div.textContent = msg;
  viewer.appendChild(div);
}

async function renderViewer() {
  const viewer = $("viewer");
  if (!state.selectedVersion) {
    viewerMessage(viewer, t("no_versions"));
    return;
  }
  if (state.mode === "content") {
    await renderContent(viewer);
  } else {
    await renderDiff(viewer);
  }
}

async function renderContent(viewer) {
  const qs = new URLSearchParams({ path: state.selectedPath, version: state.selectedVersion });
  const res = await api("/api/content?" + qs.toString());
  if (res.status === 415) {
    viewerMessage(viewer, t("binary_content"));
    return;
  }
  if (!res.ok) {
    viewerMessage(viewer, t("load_content_error"));
    return;
  }
  const text = await res.text();
  const pre = document.createElement("pre");
  pre.textContent = text;
  viewer.innerHTML = "";
  viewer.appendChild(pre);
}

async function renderDiff(viewer) {
  const from = state.compareTo;
  const to = state.selectedVersion;
  if (!from) {
    setMode("content");
    return;
  }
  const qs = new URLSearchParams({ path: state.selectedPath, from: from, to: to });
  const res = await api("/api/diff?" + qs.toString());
  if (!res.ok) {
    viewerMessage(viewer, t("load_diff_error"));
    return;
  }
  const text = await res.text();
  if (text.trim() === "binary") {
    viewerMessage(viewer, t("binary_diff"));
    return;
  }
  if (text.trim() === "") {
    viewerMessage(viewer, t("no_differences"));
    return;
  }
  viewer.innerHTML = "";
  const frag = document.createDocumentFragment();
  for (const line of text.split("\n")) {
    const span = document.createElement("span");
    span.className = "diff-line " + diffClass(line);
    span.textContent = line === "" ? " " : line;
    frag.appendChild(span);
  }
  viewer.appendChild(frag);
}

function diffClass(line) {
  if (line.startsWith("+++") || line.startsWith("---")) return "diff-meta";
  if (line.startsWith("@@")) return "diff-hunk";
  if (line.startsWith("+")) return "diff-add";
  if (line.startsWith("-")) return "diff-del";
  return "";
}

// --- modo diff/contenido ---

function setMode(mode) {
  state.mode = mode;
  $("mode-diff").classList.toggle("active", mode === "diff");
  $("mode-diff").setAttribute("aria-selected", mode === "diff");
  $("mode-content").classList.toggle("active", mode === "content");
  $("mode-content").setAttribute("aria-selected", mode === "content");
  $("compare-controls").style.visibility = mode === "diff" ? "visible" : "hidden";
  renderViewer();
}

// --- restaurar ---

function openRestore() {
  if (!state.selectedVersion) return;
  $("restore-desc").textContent =
    t("restore_desc", { path: state.selectedPath, version: state.selectedVersion });
  openOverlay("restore");
}

async function doRestore() {
  try {
    const j = await apiJSON("/api/restore", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: state.selectedPath, version: state.selectedVersion }),
    });
    closeOverlay("restore");
    if (j.safety_version) {
      toast(t("restored_with_safety", { version: j.safety_version }));
    } else {
      toast(t("restored_ok"));
    }
    await refreshFiles();
    await selectFile(state.selectedPath, false);
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
  }
}

// --- dejar de proteger (olvidar historial) ---

function openUntrack() {
  if (!state.selectedPath) return;
  $("untrack-desc").textContent = t("untrack_desc", { path: state.selectedPath });
  openOverlay("untrack");
}

async function doUntrack() {
  try {
    const j = await apiJSON("/api/forget", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: state.selectedPath }),
    });
    closeOverlay("untrack");
    toast(t("untracked_ok", { size: fmtBytes(j.freed_bytes) }));
    clearSelection();
    await refreshFiles(false);
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
  }
}

// --- proteger ficheros existentes (versión inicial) ---

function openProtect() {
  $("protect-path").value = "~";
  openOverlay("protect");
}

// doProtect pide al daemon una pasada de protección inicial: versión baseline
// de los ficheros bajo la ruta indicada que aún no tienen historial.
async function doProtect() {
  const path = $("protect-path").value.trim();
  if (!path) return;
  const btn = $("protect-confirm");
  btn.disabled = true;
  btn.textContent = t("protect_running");
  try {
    const j = await apiJSON("/api/protect", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path }),
    });
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

// --- actividad (journal) ---

async function refreshJournal() {
  let entries;
  try {
    entries = await apiJSON("/api/journal?limit=50");
  } catch (e) {
    return;
  }
  const list = $("journal-list");
  list.innerHTML = "";
  if (entries.length === 0) {
    list.hidden = true;
    $("journal-empty").hidden = false;
    return;
  }
  list.hidden = false;
  $("journal-empty").hidden = true;
  for (const e of entries) {
    const li = document.createElement("li");
    li.className = "journal-item";
    li.setAttribute("role", "button");
    li.setAttribute("tabindex", "0");
    const path = document.createElement("div");
    path.className = "j-path";
    path.textContent = e.path;
    const ts = document.createElement("div");
    ts.className = "j-ts";
    ts.textContent = fmtDate(e.ts);
    li.appendChild(path);
    li.appendChild(ts);
    const open = () => { closeOverlay("activity"); selectFile(e.path); };
    li.addEventListener("click", open);
    li.addEventListener("keydown", (ev) => {
      if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); open(); }
    });
    list.appendChild(li);
  }
}

// --- modo Agente ---

// switchView alterna entre el modo Versionado clásico y el modo Agente.
function switchView(v, updateHash = true) {
  state.view = v;
  $("files-layout").hidden = v !== "files";
  $("agents-layout").hidden = v !== "agents";
  $("view-files").classList.toggle("active", v === "files");
  $("view-files").setAttribute("aria-selected", v === "files" ? "true" : "false");
  $("view-agents").classList.toggle("active", v === "agents");
  $("view-agents").setAttribute("aria-selected", v === "agents" ? "true" : "false");
  if (updateHash) {
    if (v === "agents") {
      location.hash = "#/agents";
    } else if (location.hash.startsWith("#/agents")) {
      history.replaceState(null, "", location.pathname +
        (state.selectedPath ? "#/file/" + encodeURI(state.selectedPath) : ""));
    }
  }
  if (v === "agents") initAgentsView();
}

// initAgentsView pinta la columna de agentes y autoselecciona el primero activo.
async function initAgentsView() {
  await loadAgents();
  renderAgentList();
  const active = state.agents.filter((a) => a.files > 0);
  if (!state.selAgent && active.length > 0) {
    selectAgent(active[0].id);
  }
}

// refreshAgentsView recarga todo el modo Agente: registro, contadores de
// sesiones, la lista del agente abierto y el detalle de la sesión abierta.
async function refreshAgentsView() {
  const btn = $("btn-agents-refresh");
  btn.classList.add("spinning");
  try {
    state.sessionCounts = {};
    await loadAgents();
    renderAgentList();
    if (state.selAgent) {
      await loadSessions(state.selAgent, true);
      if (state.sessionDetail) {
        await selectSession(state.sessionDetail.session.id);
      }
    }
  } finally {
    btn.classList.remove("spinning");
  }
}

// sessionCountsInFlight evita pedir dos veces las sesiones del mismo agente
// mientras se rellenan los contadores de la columna.
const sessionCountsInFlight = new Set();

function renderAgentList() {
  const list = $("agent-list");
  list.innerHTML = "";
  const active = state.agents.filter((a) => a.files > 0);
  $("agents-empty").hidden = active.length > 0;
  list.hidden = active.length === 0;
  for (const a of active) {
    const li = document.createElement("li");
    li.className = "agent-item";
    if (a.id === state.selAgent) li.classList.add("selected");
    li.setAttribute("role", "button");
    li.setAttribute("tabindex", "0");

    const ic = document.createElement("span");
    ic.className = "agent-ic";
    ic.textContent = a.icon;

    const txt = document.createElement("div");
    const name = document.createElement("div");
    name.className = "a-name";
    name.textContent = a.name;
    const sub = document.createElement("div");
    sub.className = "a-sub";
    const n = state.sessionCounts[a.id];
    sub.textContent = n != null
      ? t("agents_sessions_n", { n })
      : t("agents_files_n", { n: a.files });
    txt.appendChild(name);
    txt.appendChild(sub);

    li.appendChild(ic);
    li.appendChild(txt);
    const open = () => selectAgent(a.id);
    li.addEventListener("click", open);
    li.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); open(); }
    });
    list.appendChild(li);

    // Contador de sesiones en segundo plano (una vez por agente).
    if (n == null && !sessionCountsInFlight.has(a.id)) {
      sessionCountsInFlight.add(a.id);
      apiJSON("/api/agent/sessions?agent=" + encodeURIComponent(a.id))
        .then((j) => {
          state.sessionCounts[a.id] = (j.sessions || []).length;
          renderAgentList();
        })
        .catch(() => {})
        .finally(() => sessionCountsInFlight.delete(a.id));
    }
  }
}

async function selectAgent(id) {
  state.selAgent = id;
  state.sessionDetail = null;
  renderAgentList();
  const ag = state.agentsById[id] || { name: id, icon: "✳" };
  $("sessions-agent-icon").textContent = ag.icon;
  $("sessions-agent-name").textContent = ag.name;
  $("sessions-count").textContent = "";
  // Vaciar la columna al instante: dejar las sesiones del agente anterior
  // visibles mientras cargan las nuevas invita a clicar datos equivocados.
  state.sessions = [];
  $("session-list").innerHTML = "";
  $("session-list").hidden = true;
  $("sessions-empty").hidden = true;
  $("session-view").hidden = true;
  $("session-empty").hidden = false;
  await loadSessions(id);
}

async function loadSessions(id, preserve = false) {
  let j;
  try {
    j = await apiJSON("/api/agent/sessions?agent=" + encodeURIComponent(id));
  } catch (e) {
    toast(e.message, "error");
    return;
  }
  if (state.selAgent !== id) return; // el usuario cambió de agente mientras cargaba
  state.sessions = j.sessions || [];
  state.sessionCounts[id] = state.sessions.length;
  $("sessions-count").textContent = t("sessions_found", { n: state.sessions.length });
  renderAgentList();
  renderSessions();
  if (preserve && state.sessionDetail &&
      !state.sessions.some((s) => s.id === state.sessionDetail.session.id)) {
    state.sessionDetail = null;
    $("session-view").hidden = true;
    $("session-empty").hidden = false;
  }
}

// sessionTitle resuelve el título a mostrar de una sesión.
function sessionTitle(s) {
  if (s.title) return s.title;
  if (s.kind === "cluster") return t("session_kind_cluster") + " · " + fmtDate(s.start);
  return t("session_untitled") + " · " + fmtDate(s.start);
}

function renderSessions() {
  const list = $("session-list");
  list.innerHTML = "";
  $("sessions-empty").hidden = state.sessions.length > 0;
  list.hidden = state.sessions.length === 0;
  const selID = state.sessionDetail ? state.sessionDetail.session.id : null;
  for (const s of state.sessions) {
    const li = document.createElement("li");
    li.className = "session-item";
    if (s.id === selID) li.classList.add("selected");
    li.setAttribute("role", "button");
    li.setAttribute("tabindex", "0");

    const title = document.createElement("div");
    title.className = "s-title";
    title.textContent = sessionTitle(s);
    const meta = document.createElement("div");
    meta.className = "s-meta";
    meta.textContent = fmtDate(s.end) + " · " + t("changes_n", { n: s.changes });
    li.appendChild(title);
    li.appendChild(meta);

    const open = () => selectSession(s.id);
    li.addEventListener("click", open);
    li.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); open(); }
    });
    list.appendChild(li);
  }
}

async function selectSession(id) {
  let j;
  try {
    const qs = new URLSearchParams({ agent: state.selAgent, id });
    j = await apiJSON("/api/agent/session?" + qs.toString());
  } catch (e) {
    toast(e.message, "error");
    return;
  }
  state.sessionDetail = j;
  renderSessions();
  renderSessionView();
}

function renderSessionView() {
  const d = state.sessionDetail;
  if (!d) return;
  const s = d.session;
  $("session-empty").hidden = true;
  $("session-view").hidden = false;

  $("session-title").textContent = sessionTitle(s);
  const metaParts = [fmtDate(s.start) + " → " + fmtDate(s.end)];
  if (s.project) metaParts.push(s.project);
  if (s.kind === "cluster") metaParts.push(t("session_kind_cluster"));
  metaParts.push(t("changes_n", { n: s.changes }));
  $("session-meta").textContent = metaParts.join(" · ");

  $("btn-transcript").hidden = !s.transcript;

  // Chips de resumen a partir del estado por fichero.
  const files = d.files || [];
  const created = files.filter((f) => f.created).length;
  const deleted = files.filter((f) => f.deleted).length;
  const modified = files.length - files.filter((f) => f.created || f.deleted).length;
  const stats = $("session-stats");
  stats.innerHTML = "";
  const chip = (n, label, cls) => {
    const el = document.createElement("span");
    el.className = "stat-chip" + (cls ? " " + cls : "");
    const strong = document.createElement("strong");
    strong.textContent = n;
    el.appendChild(strong);
    el.appendChild(document.createTextNode(" " + label));
    stats.appendChild(el);
  };
  chip(modified, t("stat_modified"));
  chip(created, t("stat_created"), "chip-add");
  chip(deleted, t("stat_deleted"), "chip-del");

  // El revert solo tiene sentido si la sesión tocó ficheros del usuario.
  $("btn-session-revert").disabled = files.length === 0;

  renderSessionEvents();
}

function renderSessionEvents() {
  const d = state.sessionDetail;
  const list = $("session-events");
  list.innerHTML = "";
  const events = d.events || [];
  $("events-empty").hidden = events.length > 0;
  list.hidden = events.length === 0;

  const anyPrompt = events.some((ev) => ev.prompt);
  if (!anyPrompt) {
    renderFlatEvents(list, events);
    return;
  }
  renderGroupedEvents(list, events);
}

function renderFlatEvents(list, events) {
  for (const ev of events) {
    list.appendChild(makeEventItem(ev));
  }
}

function renderGroupedEvents(list, events) {
  const groups = [];
  for (const ev of events) {
    const key = ev.prompt || "";
    let group = groups[groups.length - 1];
    if (!group || group.prompt !== key) {
      group = { prompt: key, events: [] };
      groups.push(group);
    }
    group.events.push(ev);
  }

  for (const group of groups) {
    if (group.prompt) {
      const header = document.createElement("li");
      header.className = "prompt-header";
      header.textContent = group.prompt;
      list.appendChild(header);
    }
    for (const ev of group.events) {
      list.appendChild(makeEventItem(ev));
    }
  }
}

function makeEventItem(ev) {
  const li = document.createElement("li");
  li.className = "event-item";

  const time = document.createElement("span");
  time.className = "ev-time";
  time.textContent = fmtDate(ev.ts);

  const type = document.createElement("span");
  type.className = "ev-type " + (ev.first ? "t-new" : "t-mod");
  type.textContent = ev.first ? t("ev_new") : t("ev_modified");

  const path = document.createElement("span");
  path.className = "ev-path";
  path.textContent = ev.path;
  path.title = ev.path;

  li.appendChild(time);
  li.appendChild(type);
  li.appendChild(path);

  const actions = document.createElement("span");
  actions.className = "ev-actions";
  const viewBtn = document.createElement("button");
  viewBtn.type = "button";
  viewBtn.className = "btn btn-small";
  viewBtn.textContent = ev.first ? t("ev_view_file") : t("ev_view_diff");
  viewBtn.addEventListener("click", () => openAgentDiff(ev));
  actions.appendChild(viewBtn);
  const revBtn = document.createElement("button");
  revBtn.type = "button";
  revBtn.className = "btn btn-small";
  revBtn.textContent = t("ev_revert_file");
  revBtn.addEventListener("click", () => openSessionRevert(ev.path));
  actions.appendChild(revBtn);
  li.appendChild(actions);
  return li;
}

// --- visor de cambios de un evento ---

let agentDiffPath = null;

async function openAgentDiff(ev) {
  agentDiffPath = ev.path;
  $("agentdiff-title").textContent = ev.path + " · " + ev.version;
  const viewer = $("agentdiff-viewer");
  viewer.innerHTML = "";
  openOverlay("agentdiff");

  let res;
  if (ev.prev) {
    const qs = new URLSearchParams({ path: ev.path, from: ev.prev, to: ev.version });
    res = await api("/api/diff?" + qs.toString());
  } else {
    const qs = new URLSearchParams({ path: ev.path, version: ev.version });
    res = await api("/api/content?" + qs.toString());
  }
  if (res.status === 415) {
    viewerMessage(viewer, t("binary_content"));
    return;
  }
  if (!res.ok) {
    viewerMessage(viewer, t(ev.prev ? "load_diff_error" : "load_content_error"));
    return;
  }
  const text = await res.text();
  if (!ev.prev) {
    const pre = document.createElement("pre");
    pre.textContent = text;
    viewer.appendChild(pre);
    return;
  }
  if (text.trim() === "") {
    viewerMessage(viewer, t("no_differences"));
    return;
  }
  const frag = document.createDocumentFragment();
  for (const line of text.split("\n")) {
    const span = document.createElement("span");
    span.className = "diff-line " + diffClass(line);
    span.textContent = line === "" ? " " : line;
    frag.appendChild(span);
  }
  viewer.appendChild(frag);
}

// openAgentFileHistory salta al historial completo del fichero en modo clásico.
function openAgentFileHistory() {
  if (!agentDiffPath) return;
  closeOverlay("agentdiff");
  switchView("files");
  selectFile(agentDiffPath);
}

// --- exportar sesión (descarga del transcript) ---

async function downloadTranscript() {
  const d = state.sessionDetail;
  if (!d || !d.session.transcript) return;
  const s = d.session;
  try {
    // Del disco si sigue vivo; de la última versión del store si fue borrado.
    const version = s.transcript_deleted ? s.transcript_version : "current";
    const qs = new URLSearchParams({ path: s.transcript, version });
    const res = await api("/api/content?" + qs.toString());
    if (!res.ok) throw new Error("HTTP " + res.status);
    const text = await res.text();
    const blob = new Blob([text], { type: "application/x-ndjson" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = s.transcript.split("/").pop();
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 5000);
  } catch (e) {
    toast(t("transcript_error", { msg: e.message }), "error");
  }
}

// --- reversión de sesión ---

// revertCtx guarda la operación en curso: ventana, ámbito, plan y manifiesto.
let revertCtx = null;

// openSessionRevert abre el modal con un dry-run del plan. target acota a un
// fichero o carpeta; vacío = todos los ficheros afectados de la sesión.
async function openSessionRevert(target) {
  const d = state.sessionDetail;
  if (!d) return;
  revertCtx = { start: d.session.start, end: d.session.end, target: target || "", manifest: null };
  $("revert-scope").textContent = revertCtx.target
    ? t("revert_scope_one", { path: revertCtx.target })
    : t("revert_scope_all");
  $("revert-plan").innerHTML = "<p class='revert-empty'>" + t("revert_plan_loading") + "</p>";
  $("revert-confirm").hidden = false;
  $("revert-confirm").disabled = true;
  $("revert-confirm").textContent = t("revert_confirm");
  $("revert-undo").hidden = true;
  openOverlay("revert");

  try {
    const plan = await apiJSON("/api/agent/revert", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ start: revertCtx.start, end: revertCtx.end, target: revertCtx.target, dry_run: true }),
    });
    revertCtx.plan = plan;
    renderRevertPlan(plan);
  } catch (e) {
    $("revert-plan").innerHTML = "";
    toast(e.message, "error");
  }
}

// renderRevertPlan pinta el plan agrupado por acción.
function renderRevertPlan(plan) {
  const box = $("revert-plan");
  box.innerHTML = "";
  const groups = [
    ["revert", "plan_revert"],
    ["restore_deleted", "plan_restore_deleted"],
    ["skip_no_prior", "plan_skip_no_prior"],
    ["skip_unchanged", "plan_skip_unchanged"],
  ];
  let actionable = 0;
  for (const [action, key] of groups) {
    const items = (plan.items || []).filter((it) => it.action === action);
    if (items.length === 0) continue;
    if (action === "revert" || action === "restore_deleted") actionable += items.length;
    const g = document.createElement("div");
    g.className = "revert-group";
    const title = document.createElement("p");
    title.className = "revert-group-title";
    title.textContent = t(key, { n: items.length });
    const ul = document.createElement("ul");
    for (const it of items) {
      const li = document.createElement("li");
      li.textContent = it.path + (it.prior ? " ← " + it.prior : "");
      ul.appendChild(li);
    }
    g.appendChild(title);
    g.appendChild(ul);
    box.appendChild(g);
  }
  if ((plan.items || []).length === 0) {
    box.innerHTML = "<p class='revert-empty'>" + t("revert_none") + "</p>";
  }
  $("revert-confirm").disabled = actionable === 0;
}

// doSessionRevert ejecuta el plan confirmado.
async function doSessionRevert() {
  if (!revertCtx) return;
  const btn = $("revert-confirm");
  btn.disabled = true;
  btn.textContent = t("revert_running");
  try {
    const res = await apiJSON("/api/agent/revert", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ start: revertCtx.start, end: revertCtx.end, target: revertCtx.target, dry_run: false }),
    });
    revertCtx.manifest = res;
    renderRevertPlan(res);
    const n = res.reverted + res.restored_deleted;
    toast(t("revert_done", { n }));
    btn.hidden = true;
    // Deshacer disponible si alguna restauración dejó safety version.
    $("revert-undo").hidden = !(res.items || []).some((it) => it.safety_version);
    await refreshFiles();
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
    btn.disabled = false;
  } finally {
    btn.textContent = t("revert_confirm");
  }
}

// undoSessionRevert restaura las safety versions del manifiesto (deshace la
// reversión). Los borrados recuperados sin safety se conservan: nunca borramos.
async function undoSessionRevert() {
  const m = revertCtx && revertCtx.manifest;
  if (!m) return;
  const btn = $("revert-undo");
  btn.disabled = true;
  let n = 0;
  try {
    for (const it of m.items || []) {
      if (!it.safety_version) continue;
      await apiJSON("/api/restore", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path: it.path, version: it.safety_version }),
      });
      n++;
    }
    toast(t("revert_undo_done", { n }));
    closeOverlay("revert");
    await refreshFiles();
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
  } finally {
    btn.disabled = false;
  }
}

// --- ajustes ---

function initSettingsTabs() {
  const tabs = document.querySelectorAll(".settings-tab");
  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      tabs.forEach((tb) => {
        tb.classList.toggle("active", tb === tab);
        tb.setAttribute("aria-selected", tb === tab ? "true" : "false");
      });
      document.querySelectorAll(".settings-panel").forEach((panel) => {
        panel.hidden = panel.dataset.panel !== tab.dataset.tab;
      });
    });
  });
}

function initLangSelect() {
  const sel = $("cfg-lang");
  sel.innerHTML = "";
  for (const l of I18N.languages) {
    const o = document.createElement("option");
    o.value = l.code;
    o.textContent = l.label;
    sel.appendChild(o);
  }
  sel.value = I18N.lang();
  sel.addEventListener("change", () => I18N.setLang(sel.value));
}

async function openSettings() {
  try {
    const c = await apiJSON("/api/config");
    $("cfg-lang").value = I18N.lang();
    $("cfg-prefix").value = c.prefix;
    $("cfg-debounce").value = c.debounce_seconds;
    $("cfg-port").value = c.port;
    $("cfg-included").value = (c.included_paths || []).join("\n");
    $("cfg-excluded").value = (c.excluded_paths || []).join("\n");
    $("cfg-exclude-names").value = (c.exclude_names || []).join("\n");
    $("cfg-ignore-patterns").value = (c.ignore_patterns || []).join("\n");
    $("cfg-max-versions").value = c.retention.max_versions;
    $("cfg-max-age").value = c.retention.max_age_days;
    $("cfg-deleted-age").value = c.retention.deleted_age_days;
    $("cfg-min-keep").value = c.retention.min_keep;
    $("cfg-gitignore").checked = !!c.gitignore_aware;
    openOverlay("settings");
  } catch (e) {
    toast(e.message, "error");
  }
}

function linesToList(text) {
  return text.split("\n").map((s) => s.trim()).filter((s) => s.length > 0);
}

async function saveSettings(ev) {
  ev.preventDefault();
  const body = {
    prefix: $("cfg-prefix").value,
    debounce_seconds: parseInt($("cfg-debounce").value, 10),
    port: parseInt($("cfg-port").value, 10),
    included_paths: linesToList($("cfg-included").value),
    excluded_paths: linesToList($("cfg-excluded").value),
    exclude_names: linesToList($("cfg-exclude-names").value),
    ignore_patterns: linesToList($("cfg-ignore-patterns").value),
    gitignore_aware: $("cfg-gitignore").checked,
    max_file_size_mb: 100,
    retention: {
      max_versions: parseInt($("cfg-max-versions").value, 10),
      max_age_days: parseInt($("cfg-max-age").value, 10),
      deleted_age_days: parseInt($("cfg-deleted-age").value, 10),
      min_keep: parseInt($("cfg-min-keep").value, 10),
    },
  };
  try {
    // Conserva max_file_size_mb del servidor.
    const cur = await apiJSON("/api/config");
    body.max_file_size_mb = cur.max_file_size_mb;
    const j = await apiJSON("/api/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    closeOverlay("settings");
    toast(j.restart_required ? t("settings_saved_restart") : t("settings_saved"));
  } catch (e) {
    toast(e.message, "error");
  }
}

// purgeIgnored elimina del historial los ficheros trackeados que coinciden con
// los ignore_patterns guardados en el servidor.
async function purgeIgnored() {
  try {
    const j = await apiJSON("/api/forget", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ apply_ignores: true }),
    });
    if (j.forgotten > 0) {
      toast(t("purge_ignored_done", { count: j.forgotten, size: fmtBytes(j.freed_bytes) }));
    } else {
      toast(t("purge_ignored_none"));
    }
    if (state.selectedPath && !state.files.some((f) => f.path === state.selectedPath)) {
      clearSelection();
    }
    await refreshFiles();
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
  }
}

// --- overlays ---

let lastFocus = null;

function openOverlay(name) {
  lastFocus = document.activeElement;
  const ov = $(name + "-overlay");
  ov.hidden = false;
  const focusable = ov.querySelector("input, button, select, textarea, [tabindex]");
  if (focusable) focusable.focus();
}

function closeOverlay(name) {
  $(name + "-overlay").hidden = true;
  if (lastFocus && lastFocus.focus) lastFocus.focus();
}

function initOverlays() {
  document.querySelectorAll("[data-close]").forEach((btn) => {
    btn.addEventListener("click", () => closeOverlay(btn.dataset.close));
  });
  document.querySelectorAll(".overlay").forEach((ov) => {
    ov.addEventListener("click", (e) => { if (e.target === ov) ov.hidden = true; });
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      document.querySelectorAll(".overlay:not([hidden])").forEach((ov) => (ov.hidden = true));
    }
  });
}

// --- routing hash ---

function handleHash() {
  const h = location.hash;
  if (h.startsWith("#/agents")) {
    if (state.view !== "agents") switchView("agents", false);
    return;
  }
  const prefix = "#/file/";
  if (h.startsWith(prefix)) {
    if (state.view !== "files") switchView("files", false);
    const path = decodeURI(h.slice(prefix.length));
    if (path && path !== state.selectedPath) {
      selectFile(path, false);
    }
  }
}

// --- auto-refresh ---

function initAutoRefresh() {
  setInterval(() => {
    if (document.visibilityState !== "visible") return;
    refreshFiles();
    refreshStatus();
    if (!$("activity-overlay").hidden) refreshJournal();
  }, REFRESH_MS);
}

// --- stream de eventos en vivo (SSE) ---

// scheduleLiveRefresh agrupa ráfagas de eventos en un único refresco.
let liveRefreshTimer = null;
function scheduleLiveRefresh() {
  if (liveRefreshTimer) return;
  liveRefreshTimer = setTimeout(() => {
    liveRefreshTimer = null;
    refreshFiles();
    refreshStatus();
    if (!$("activity-overlay").hidden) refreshJournal();
    if (state.selectedPath) {
      // Refresca las versiones del fichero abierto sin perder el foco.
      selectFile(state.selectedPath, false).catch(() => {});
    }
    if (state.view === "agents" && state.selAgent) {
      // Mantiene la lista de sesiones al día mientras el agente trabaja.
      loadSessions(state.selAgent, true).catch(() => {});
    }
  }, 400);
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
      applyProtection(true);
      break;
    case "resumed":
      applyProtection(false);
      break;
    case "error":
      if (ev.message) toast(ev.message, "error");
      break;
  }
}

// connectEvents mantiene abierto el stream /api/events. Usa fetch con lectura por
// streaming (no EventSource) para poder enviar el token en la cabecera. Reconecta
// automáticamente si la conexión se cae.
async function connectEvents() {
  try {
    const res = await api("/api/events", { headers: { Accept: "text/event-stream" } });
    if (!res.ok || !res.body) throw new Error("events " + res.status);
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = "";
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      let idx;
      while ((idx = buf.indexOf("\n\n")) >= 0) {
        const block = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        for (const line of block.split("\n")) {
          const data = line.startsWith("data:") ? line.slice(5).trim() : "";
          if (!data) continue;
          try { handleLiveEvent(JSON.parse(data)); } catch (_) {}
        }
      }
    }
  } catch (_) {
    // conexión caída o no soportada: se reintenta abajo
  }
  setTimeout(connectEvents, 3000);
}

// --- init ---

// rerenderForLang repinta los textos dinámicos tras un cambio de idioma.
function rerenderForLang() {
  renderStatus();
  applyProtection(state.paused);
  applyAiToggleUI();
  renderFiles();
  if (state.selectedPath) {
    renderVersions();
    renderCompareOptions();
    renderViewer();
  }
  if (state.view === "agents") {
    renderAgentList();
    renderSessions();
    if (state.sessionDetail) renderSessionView();
    if (state.selAgent && state.sessions) {
      $("sessions-count").textContent = t("sessions_found", { n: state.sessions.length });
    }
  }
}

async function copySelectedPath() {
  if (!state.selectedPath) return;
  try {
    await navigator.clipboard.writeText(state.selectedPath);
    toast(t("path_copied"));
  } catch (_) {
    // portapapeles no disponible (permiso denegado): sin efecto
  }
}

async function init() {
  I18N.apply();
  initTheme();
  initOverlays();
  initSettingsTabs();
  initLangSelect();

  document.addEventListener("i18n:changed", rerenderForLang);

  $("search").addEventListener("input", (e) => {
    state.query = e.target.value;
    refreshFiles();
  });
  document.querySelectorAll(".filter").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".filter").forEach((b) => {
        b.classList.remove("active");
        b.setAttribute("aria-selected", "false");
      });
      btn.classList.add("active");
      btn.setAttribute("aria-selected", "true");
      state.filter = btn.dataset.filter;
      refreshFiles();
    });
  });

  $("mode-diff").addEventListener("click", () => setMode("diff"));
  $("mode-content").addEventListener("click", () => setMode("content"));
  $("btn-restore").addEventListener("click", openRestore);
  $("restore-confirm").addEventListener("click", doRestore);
  $("btn-untrack").addEventListener("click", openUntrack);
  $("untrack-confirm").addEventListener("click", doUntrack);
  $("btn-copy-path").addEventListener("click", copySelectedPath);

  $("btn-pause").addEventListener("click", togglePause);
  $("btn-flush").addEventListener("click", flushNow);
  $("btn-protect-existing").addEventListener("click", openProtect);
  $("protect-confirm").addEventListener("click", doProtect);
  $("protect-path").addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); doProtect(); }
  });
  $("btn-activity").addEventListener("click", () => { openOverlay("activity"); refreshJournal(); });
  $("btn-settings").addEventListener("click", openSettings);
  $("btn-about").addEventListener("click", () => openOverlay("about"));
  $("settings-form").addEventListener("submit", saveSettings);
  $("btn-purge-ignored").addEventListener("click", purgeIgnored);
  $("btn-ai-filter").addEventListener("click", cycleAiFilter);

  $("view-files").addEventListener("click", () => switchView("files"));
  $("view-agents").addEventListener("click", () => switchView("agents"));
  $("btn-agents-refresh").addEventListener("click", refreshAgentsView);
  $("btn-transcript").addEventListener("click", downloadTranscript);
  $("btn-session-revert").addEventListener("click", () => openSessionRevert(""));
  $("revert-confirm").addEventListener("click", doSessionRevert);
  $("revert-undo").addEventListener("click", undoSessionRevert);
  $("agentdiff-history").addEventListener("click", openAgentFileHistory);

  window.addEventListener("hashchange", handleHash);

  try {
    await fetchToken();
  } catch (e) {
    toast(t("login_error", { msg: e.message }), "error");
    return;
  }

  applyAiToggleUI();
  await loadAgents();
  await refreshStatus();
  await refreshFiles(false);
  handleHash();
  initAutoRefresh();
  connectEvents();
}

document.addEventListener("DOMContentLoaded", init);
