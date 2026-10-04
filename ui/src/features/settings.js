// features/settings.js — modal de Ajustes: pestañas, idioma, configuración del
// daemon y purga de ignorados.
import { $, h, mount } from "../core/dom.js";
import { apiJSON, jsonBody } from "../core/api.js";
import { fmtBytes } from "../core/format.js";
import { I18N, t } from "../core/i18n.js";
import { toast } from "../core/toast.js";
import { openOverlay, closeOverlay } from "../core/overlay.js";
import { getState } from "../state.js";
import { refreshFiles, clearSelection } from "./files.js";
import { refreshStatus } from "./header.js";

// initSettings cablea el botón, las pestañas, el selector de idioma y el formulario.
export function initSettings() {
  $("btn-settings").addEventListener("click", openSettings);
  $("settings-form").addEventListener("submit", saveSettings);
  $("btn-purge-ignored").addEventListener("click", purgeIgnored);
  initTabs();
  initLangSelect();
}

function initTabs() {
  const tabs = document.querySelectorAll(".settings-tab");
  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      tabs.forEach((tb) => {
        const active = tb === tab;
        tb.classList.toggle("active", active);
        tb.setAttribute("aria-selected", active ? "true" : "false");
      });
      document.querySelectorAll(".settings-panel").forEach((panel) => {
        panel.hidden = panel.dataset.panel !== tab.dataset.tab;
      });
    });
  });
}

function initLangSelect() {
  const sel = $("cfg-lang");
  mount(sel, I18N.languages.map((l) => h("option", { value: l.code, text: l.label })));
  sel.value = I18N.lang();
  sel.addEventListener("change", () => I18N.setLang(sel.value));
}

// openSettings carga la configuración del daemon y abre el modal.
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

// linesToList convierte un textarea (una entrada por línea) en un array limpio.
function linesToList(text) {
  return text
    .split("\n")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
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
    const j = await apiJSON("/api/config", jsonBody("PUT", body));
    closeOverlay("settings");
    toast(j.restart_required ? t("settings_saved_restart") : t("settings_saved"));
  } catch (e) {
    toast(e.message, "error");
  }
}

// purgeIgnored elimina del historial los ficheros trackeados que coinciden con los
// ignore_patterns guardados en el servidor.
async function purgeIgnored() {
  try {
    const j = await apiJSON("/api/forget", jsonBody("POST", { apply_ignores: true }));
    if (j.forgotten > 0) {
      toast(t("purge_ignored_done", { count: j.forgotten, size: fmtBytes(j.freed_bytes) }));
    } else {
      toast(t("purge_ignored_none"));
    }
    const sel = getState().selectedPath;
    if (sel && !getState().files.some((f) => f.path === sel)) clearSelection();
    await refreshFiles();
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
  }
}
