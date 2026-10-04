// features/files.js — modo Versionado: lista de ficheros, historial de versiones,
// visor diff/contenido, restauración y "dejar de proteger".
//
// El estado vive en el store; este módulo aporta las acciones (fetch + set) y los
// repintados, suscritos a las claves relevantes con watchKeys. Nada de fetch se
// dispara por duplicado: cada acción agrupa sus cambios en un único set.
import { $, h, mount, clear, activateOnEnter } from "../core/dom.js";
import { api, apiJSON, jsonBody } from "../core/api.js";
import { fmtBytes, fmtDate } from "../core/format.js";
import { t } from "../core/i18n.js";
import { toast } from "../core/toast.js";
import { openOverlay, closeOverlay } from "../core/overlay.js";
import { fileHash, navigate } from "../core/router.js";
import { watchKeys } from "../core/store.js";
import { store, getState, setState } from "../state.js";
import {
  badge,
  deletedBadge,
  agentChip,
  viewerMessage,
  renderDiffLines,
} from "../components/common.js";
import { refreshStatus } from "./header.js";

const AI_FILTER_CYCLE = { all: "only", only: "hide", hide: "all" };

// initFiles cablea los controles del modo Versionado y suscribe sus repintados.
export function initFiles() {
  $("search").addEventListener("input", (e) => {
    setState({ query: e.target.value });
    refreshFiles();
  });
  document.querySelectorAll(".filter").forEach((btn) => {
    btn.addEventListener("click", () => setFilter(btn.dataset.filter));
  });
  $("btn-ai-filter").addEventListener("click", cycleAiFilter);

  $("mode-diff").addEventListener("click", () => setMode("diff"));
  $("mode-content").addEventListener("click", () => setMode("content"));
  $("btn-restore").addEventListener("click", openRestore);
  $("restore-confirm").addEventListener("click", doRestore);
  $("btn-untrack").addEventListener("click", openUntrack);
  $("untrack-confirm").addEventListener("click", doUntrack);
  $("btn-copy-path").addEventListener("click", copySelectedPath);

  watchKeys(
    store,
    ["files", "aiFilter", "selectedPath", "agentsById", "lang"],
    renderFileList,
  );
  watchKeys(store, ["aiFilter", "lang"], renderAiToggle);
  watchKeys(
    store,
    ["selectedPath", "deleted", "versions", "selectedVersion", "lang"],
    renderFilePanel,
  );
  watchKeys(store, ["mode", "lang"], renderModeToggle);
  watchKeys(
    store,
    ["selectedPath", "selectedVersion", "compareTo", "mode"],
    renderViewer,
  );
}

// --- acciones: lista ---

// refreshFiles recarga la lista aplicando búsqueda y filtro del servidor.
let listRequest = 0;
let fileRequest = 0;
let viewerRequest = 0;

export async function refreshFiles() {
  const request = ++listRequest;
  try {
    const qs = new URLSearchParams({
      q: getState().query,
      filter: getState().filter,
    });
    const files = await apiJSON("/api/files?" + qs.toString());
    if (request === listRequest) setState({ files });
  } catch (e) {
    toast(e.message, "error");
  }
}

function setFilter(filter) {
  document.querySelectorAll(".filter").forEach((b) => {
    const active = b.dataset.filter === filter;
    b.classList.toggle("active", active);
    b.setAttribute("aria-selected", active ? "true" : "false");
  });
  setState({ filter });
  refreshFiles();
}

function cycleAiFilter() {
  const aiFilter = AI_FILTER_CYCLE[getState().aiFilter] || "all";
  localStorage.setItem("unmessai-ai-filter", aiFilter);
  setState({ aiFilter });
}

// visibleFiles aplica el filtro de IA (en cliente) sobre la lista ya cargada.
function visibleFiles() {
  const { files, aiFilter } = getState();
  if (aiFilter === "only") return files.filter((f) => f.agent);
  if (aiFilter === "hide") return files.filter((f) => !f.agent);
  return files;
}

// --- acciones: selección y versiones ---

// selectFile abre un fichero: carga sus versiones y fija la selección por defecto
// (versión más reciente, comparada con la anterior).
export async function selectFile(
  path,
  { updateHash = true, preserve = false } = {},
) {
  const request = ++fileRequest;
  let versions;
  try {
    versions = await apiJSON("/api/versions?path=" + encodeURIComponent(path));
  } catch (e) {
    toast(e.message, "error");
    return;
  }
  if (request !== fileRequest) return;
  const f = getState().files.find((x) => x.path === path);
  const previous = getState();
  const keep =
    preserve &&
    previous.selectedPath === path &&
    versions.some((v) => v.name === previous.selectedVersion);
  const selectedVersion = keep
    ? previous.selectedVersion
    : versions.length
      ? versions[0].name
      : null;
  setState({
    selectedPath: path,
    deleted: f ? f.deleted : false,
    versions,
    selectedVersion,
    compareTo: keep
      ? previous.compareTo
      : defaultCompareTo(versions, selectedVersion),
  });
  if (updateHash) navigate(fileHash(path));
}

// clearSelection vuelve al estado sin fichero seleccionado (tras olvidar uno).
export function clearSelection() {
  fileRequest++;
  viewerRequest++;
  setState({ selectedPath: null, versions: [], selectedVersion: null });
  if (location.hash) navigate("", { push: false });
}

// defaultCompareTo devuelve la versión anterior a la seleccionada, o null si es
// la primera (no hay con qué comparar).
function defaultCompareTo(versions, selectedVersion) {
  const idx = versions.findIndex((v) => v.name === selectedVersion);
  if (idx >= 0 && idx + 1 < versions.length) return versions[idx + 1].name;
  return null;
}

function pickVersion(name) {
  const versions = getState().versions;
  setState({
    selectedVersion: name,
    compareTo: defaultCompareTo(versions, name),
  });
}

function setMode(mode) {
  setState({ mode });
}

// --- repintados ---

function renderFileList() {
  const list = $("file-list");
  const files = visibleFiles();
  if (files.length === 0) {
    clear(list);
    list.hidden = true;
    $("files-empty").hidden = false;
    return;
  }
  list.hidden = false;
  $("files-empty").hidden = true;
  const sel = getState().selectedPath;
  const byId = getState().agentsById;
  mount(
    list,
    files.map((f) => fileItem(f, f.path === sel, byId)),
  );
}

function fileItem(f, selected, byId) {
  const meta = h(
    "div",
    { class: "fi-meta" },
    h("span", {
      text:
        f.versions +
        " " +
        (f.versions === 1 ? t("version_singular") : t("version_plural")),
    }),
    h("span", { text: fmtDate(f.last_version_at) }),
    f.agent && agentChip(byId[f.agent], f.agent),
    f.deleted && deletedBadge(),
  );
  const li = h(
    "li",
    {
      class: "file-item" + (selected ? " selected" : ""),
      role: "button",
      tabindex: "0",
      dataset: { path: f.path },
      onClick: () => selectFile(f.path),
    },
    h("div", { class: "fi-path", text: f.path }),
    meta,
  );
  activateOnEnter(li, () => selectFile(f.path));
  return li;
}

function renderAiToggle() {
  const btn = $("btn-ai-filter");
  const aiFilter = getState().aiFilter;
  btn.classList.toggle("state-only", aiFilter === "only");
  btn.classList.toggle("state-hide", aiFilter === "hide");
  const key =
    aiFilter === "only"
      ? "ai_filter_only"
      : aiFilter === "hide"
        ? "ai_filter_hide"
        : "ai_filter_all";
  const label = t(key);
  btn.title = label;
  btn.setAttribute("aria-label", label);
  btn.setAttribute("aria-pressed", aiFilter === "all" ? "false" : "true");
}

// renderFilePanel alterna entre el panel vacío y la vista de fichero, y repinta
// cabecera, versiones y opciones de comparación.
function renderFilePanel() {
  const { selectedPath, deleted, versions } = getState();
  if (!selectedPath) {
    $("file-view").hidden = true;
    $("panel-empty").hidden = false;
    return;
  }
  $("panel-empty").hidden = true;
  $("file-view").hidden = false;
  $("file-path").textContent = selectedPath;
  $("file-deleted-badge").hidden = !deleted;
  $("btn-restore").disabled = versions.length === 0;
  renderVersions();
  renderCompareOptions();
}

function renderVersions() {
  const { versions, selectedVersion } = getState();
  mount(
    $("version-list"),
    versions.map((v, i) => versionItem(v, i, selectedVersion)),
  );
}

function versionItem(v, i, selectedVersion) {
  const current = i === 0; // la más reciente
  const name = h(
    "div",
    { class: "v-name" },
    v.name,
    current && badge(t("current_badge"), "current"),
  );
  const li = h(
    "li",
    {
      class:
        "version-item" +
        (current ? " current" : "") +
        (v.name === selectedVersion ? " selected" : ""),
      role: "button",
      tabindex: "0",
      onClick: () => pickVersion(v.name),
    },
    name,
    h("div", {
      class: "v-meta",
      text: fmtDate(v.ts) + " · " + fmtBytes(v.size),
    }),
  );
  activateOnEnter(li, () => pickVersion(v.name));
  return li;
}

function renderCompareOptions() {
  const { versions, selectedVersion, compareTo, deleted } = getState();
  const sel = $("compare-to");
  mount(
    sel,
    h("option", {
      value: "current",
      text: t("compare_current"),
      disabled: deleted,
    }),
    versions
      .filter((v) => v.name !== selectedVersion)
      .map((v) => h("option", { value: v.name, text: v.name })),
  );
  sel.value = compareTo || "current";
  sel.onchange = () => setState({ compareTo: sel.value });
}

function renderModeToggle() {
  const mode = getState().mode;
  toggleTab($("mode-diff"), mode === "diff");
  toggleTab($("mode-content"), mode === "content");
  $("compare-controls").style.visibility =
    mode === "diff" ? "visible" : "hidden";
}

function toggleTab(btn, active) {
  btn.classList.toggle("active", active);
  btn.setAttribute("aria-selected", active ? "true" : "false");
}

// renderViewer pinta el diff o el contenido de la versión seleccionada.
async function renderViewer() {
  const request = ++viewerRequest;
  const viewer = $("viewer");
  const { selectedPath, selectedVersion, compareTo, mode } = getState();
  if (!selectedVersion) {
    viewerMessage(viewer, t("no_versions"));
    return;
  }
  if (mode === "diff" && !compareTo) {
    setMode("content");
    return;
  }
  viewerMessage(viewer, t("loading"));
  try {
    const qs =
      mode === "content"
        ? new URLSearchParams({ path: selectedPath, version: selectedVersion })
        : new URLSearchParams({
            path: selectedPath,
            from: compareTo,
            to: selectedVersion,
          });
    const res = await api(
      (mode === "content" ? "/api/content?" : "/api/diff?") + qs,
    );
    const text = await res.text();
    if (request !== viewerRequest) return;
    if (res.status === 415 || (mode === "diff" && text.trim() === "binary")) {
      viewerMessage(viewer, t("binary_content"));
      return;
    }
    if (!res.ok) {
      viewerMessage(
        viewer,
        t(mode === "content" ? "load_content_error" : "load_diff_error"),
      );
      return;
    }
    if (mode === "content") viewer.replaceChildren(h("pre", { text }));
    else if (!text.trim()) viewerMessage(viewer, t("no_differences"));
    else viewer.replaceChildren(renderDiffLines(text));
  } catch (e) {
    if (request === viewerRequest)
      viewerMessage(viewer, t("load_content_error"));
  }
}

// --- restaurar ---

let restoreContext = null;
let untrackContext = null;

function openRestore() {
  const { selectedVersion, selectedPath } = getState();
  if (!selectedVersion) return;
  restoreContext = { selectedPath, selectedVersion };
  $("restore-desc").textContent = t("restore_desc", {
    path: selectedPath,
    version: selectedVersion,
  });
  openOverlay("restore");
}

async function doRestore() {
  if (!restoreContext) return;
  const { selectedPath, selectedVersion } = restoreContext;
  const btn = $("restore-confirm");
  if (btn.disabled) return;
  btn.disabled = true;
  try {
    const j = await apiJSON(
      "/api/restore",
      jsonBody("POST", { path: selectedPath, version: selectedVersion }),
    );
    closeOverlay("restore");
    toast(
      j.safety_version
        ? t("restored_with_safety", { version: j.safety_version })
        : t("restored_ok"),
    );
    await refreshFiles();
    await selectFile(selectedPath, { updateHash: false });
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
  } finally {
    btn.disabled = false;
  }
}

// --- dejar de proteger (olvidar historial) ---

function openUntrack() {
  const path = getState().selectedPath;
  if (!path) return;
  untrackContext = path;
  $("untrack-desc").textContent = t("untrack_desc", { path });
  openOverlay("untrack");
}

async function doUntrack() {
  const path = untrackContext;
  if (!path) return;
  try {
    const j = await apiJSON("/api/forget", jsonBody("POST", { path }));
    closeOverlay("untrack");
    toast(t("untracked_ok", { size: fmtBytes(j.freed_bytes) }));
    clearSelection();
    await refreshFiles();
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
  }
}

async function copySelectedPath() {
  const path = getState().selectedPath;
  if (!path) return;
  try {
    await navigator.clipboard.writeText(path);
    toast(t("path_copied"));
  } catch (_) {
    // portapapeles no disponible (permiso denegado): sin efecto
  }
}
