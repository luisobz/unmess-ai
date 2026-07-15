"use strict";

// Estado global de la aplicación.
const state = {
  token: null,
  filter: "all",
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

async function api(path, opts = {}) {
  const headers = Object.assign({}, opts.headers || {});
  if (state.token) headers["Authorization"] = "Bearer " + state.token;
  const res = await fetch(path, Object.assign({}, opts, { headers }));
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
  try {
    const j = await apiJSON("/api/flush", { method: "POST" });
    toast(j.flushed > 0 ? t("flush_done", { n: j.flushed }) : t("flush_none"));
  } catch (e) {
    toast(t("flush_error", { msg: e.message }), "error");
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

function renderFiles() {
  const list = $("file-list");
  list.innerHTML = "";
  if (state.files.length === 0) {
    list.hidden = true;
    $("files-empty").hidden = false;
    return;
  }
  list.hidden = false;
  $("files-empty").hidden = true;
  for (const f of state.files) {
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

// Comparación por defecto: la versión anterior a la seleccionada, o el disco.
function defaultCompareTo() {
  const idx = state.versions.findIndex((v) => v.name === state.selectedVersion);
  if (idx >= 0 && idx + 1 < state.versions.length) {
    return state.versions[idx + 1].name; // la anterior (más antigua)
  }
  return state.deleted ? null : "current";
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
    viewerMessage(viewer, t("no_previous_version"));
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
  const prefix = "#/file/";
  if (h.startsWith(prefix)) {
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
  }, 400);
}

function handleLiveEvent(ev) {
  switch (ev.type) {
    case "versioned":
    case "restored":
    case "pruned":
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
  renderFiles();
  if (state.selectedPath) {
    renderVersions();
    renderCompareOptions();
    renderViewer();
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
  $("btn-activity").addEventListener("click", () => { openOverlay("activity"); refreshJournal(); });
  $("btn-settings").addEventListener("click", openSettings);
  $("btn-about").addEventListener("click", () => openOverlay("about"));
  $("settings-form").addEventListener("submit", saveSettings);
  $("btn-purge-ignored").addEventListener("click", purgeIgnored);

  window.addEventListener("hashchange", handleHash);

  try {
    await fetchToken();
  } catch (e) {
    toast(t("login_error", { msg: e.message }), "error");
    return;
  }

  await refreshStatus();
  await refreshFiles(false);
  handleHash();
  initAutoRefresh();
  connectEvents();
}

document.addEventListener("DOMContentLoaded", init);
