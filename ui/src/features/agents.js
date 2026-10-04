// features/agents.js — modo Agente: agentes → sesiones → traza de cambios y
// reversión de sesión (plan dry-run → confirmación → deshacer).
//
// El estado compartido (agents, agentsById, sesiones, detalle) vive en el store;
// los flujos son secuenciales (cada acción carga y repinta su columna), así que el
// render es explícito. El modo se inicializa a sí mismo suscribiéndose a la vista.
import { $, h, mount, clear, activateOnEnter } from "../core/dom.js";
import { api, apiJSON, jsonBody } from "../core/api.js";
import { fmtDate } from "../core/format.js";
import { t } from "../core/i18n.js";
import { toast } from "../core/toast.js";
import { openOverlay, closeOverlay } from "../core/overlay.js";
import { watchKeys } from "../core/store.js";
import { store, getState, setState } from "../state.js";
import { statChip, viewerMessage, renderDiffLines } from "../components/common.js";
import { setView } from "./views.js";
import { selectFile, refreshFiles } from "./files.js";
import { refreshStatus } from "./header.js";

// initAgents cablea los controles del modo Agente y su autoarranque por vista.
export function initAgents() {
  $("btn-agents-refresh").addEventListener("click", refreshAgentsView);
  $("btn-transcript").addEventListener("click", downloadTranscript);
  $("btn-session-revert").addEventListener("click", () => openSessionRevert(""));
  $("revert-confirm").addEventListener("click", doSessionRevert);
  $("revert-undo").addEventListener("click", undoSessionRevert);
  $("agentdiff-history").addEventListener("click", openAgentFileHistory);

  // Al entrar en el modo Agente por primera vez, se inicializa la columna.
  watchKeys(store, ["view"], (s, prev) => {
    if (s.view === "agents" && (!prev || prev.view !== "agents")) initAgentsView();
  }, { immediate: false });
  // Cambio de idioma: repinta el texto dinámico ya presente.
  watchKeys(store, ["lang"], rerenderAgents, { immediate: false });
}

// loadAgents trae el registro de agentes (chips de la lista de ficheros y columna
// del modo Agente). Se llama al arrancar y en cada refresco.
export async function loadAgents() {
  try {
    const list = await apiJSON("/api/agents");
    const agentsById = {};
    for (const a of list) agentsById[a.id] = { name: a.name, icon: a.icon };
    setState({ agents: list, agentsById });
  } catch (_) {
    // sin registro: los chips de agente simplemente no se pintan
  }
}

// initAgentsView pinta la columna de agentes y autoselecciona el primero activo.
async function initAgentsView() {
  await loadAgents();
  renderAgentList();
  const active = getState().agents.filter((a) => a.files > 0);
  if (!getState().selAgent && active.length > 0) selectAgent(active[0].id);
}

// refreshAgentsView recarga todo el modo Agente: registro, contadores de sesiones,
// la lista del agente abierto y el detalle de la sesión abierta.
async function refreshAgentsView() {
  const btn = $("btn-agents-refresh");
  btn.classList.add("spinning");
  try {
    setState({ sessionCounts: {} });
    await loadAgents();
    renderAgentList();
    const { selAgent, sessionDetail } = getState();
    if (selAgent) {
      await loadSessions(selAgent, true);
      if (getState().sessionDetail) await selectSession(getState().sessionDetail.session.id);
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
  const active = getState().agents.filter((a) => a.files > 0);
  $("agents-empty").hidden = active.length > 0;
  list.hidden = active.length === 0;
  const { selAgent, sessionCounts } = getState();
  mount(
    list,
    active.map((a) => agentItem(a, a.id === selAgent, sessionCounts[a.id])),
  );
  // Contador de sesiones en segundo plano (una vez por agente).
  for (const a of active) {
    if (sessionCounts[a.id] == null && !sessionCountsInFlight.has(a.id)) {
      sessionCountsInFlight.add(a.id);
      apiJSON("/api/agent/sessions?agent=" + encodeURIComponent(a.id))
        .then((j) => {
          setState((s) => ({ sessionCounts: { ...s.sessionCounts, [a.id]: (j.sessions || []).length } }));
          renderAgentList();
        })
        .catch(() => {})
        .finally(() => sessionCountsInFlight.delete(a.id));
    }
  }
}

function agentItem(a, selected, count) {
  const sub = count != null ? t("agents_sessions_n", { n: count }) : t("agents_files_n", { n: a.files });
  const li = h(
    "li",
    {
      class: "agent-item" + (selected ? " selected" : ""),
      role: "button",
      tabindex: "0",
      onClick: () => selectAgent(a.id),
    },
    h("span", { class: "agent-ic", text: a.icon }),
    h("div", {}, h("div", { class: "a-name", text: a.name }), h("div", { class: "a-sub", text: sub })),
  );
  activateOnEnter(li, () => selectAgent(a.id));
  return li;
}

async function selectAgent(id) {
  const ag = getState().agentsById[id] || { name: id, icon: "✳" };
  // Vaciar la columna al instante: dejar las sesiones del agente anterior
  // visibles mientras cargan las nuevas invita a clicar datos equivocados.
  setState({ selAgent: id, sessionDetail: null, sessions: [] });
  renderAgentList();
  $("sessions-agent-icon").textContent = ag.icon;
  $("sessions-agent-name").textContent = ag.name;
  $("sessions-count").textContent = "";
  clear($("session-list"));
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
  if (getState().selAgent !== id) return; // el usuario cambió de agente mientras cargaba
  const sessions = j.sessions || [];
  setState((s) => ({ sessions, sessionCounts: { ...s.sessionCounts, [id]: sessions.length } }));
  $("sessions-count").textContent = t("sessions_found", { n: sessions.length });
  renderAgentList();
  renderSessions();
  const detail = getState().sessionDetail;
  if (preserve && detail && !sessions.some((s) => s.id === detail.session.id)) {
    setState({ sessionDetail: null });
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
  const { sessions, sessionDetail } = getState();
  const list = $("session-list");
  $("sessions-empty").hidden = sessions.length > 0;
  list.hidden = sessions.length === 0;
  const selID = sessionDetail ? sessionDetail.session.id : null;
  mount(
    list,
    sessions.map((s) => sessionItem(s, s.id === selID)),
  );
}

function sessionItem(s, selected) {
  const li = h(
    "li",
    {
      class: "session-item" + (selected ? " selected" : ""),
      role: "button",
      tabindex: "0",
      onClick: () => selectSession(s.id),
    },
    h("div", { class: "s-title", text: sessionTitle(s) }),
    h("div", { class: "s-meta", text: fmtDate(s.end) + " · " + t("changes_n", { n: s.changes }) }),
  );
  activateOnEnter(li, () => selectSession(s.id));
  return li;
}

async function selectSession(id) {
  let j;
  try {
    const qs = new URLSearchParams({ agent: getState().selAgent, id });
    j = await apiJSON("/api/agent/session?" + qs.toString());
  } catch (e) {
    toast(e.message, "error");
    return;
  }
  setState({ sessionDetail: j });
  renderSessions();
  renderSessionView();
}

function renderSessionView() {
  const d = getState().sessionDetail;
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
  mount(
    $("session-stats"),
    statChip(modified, t("stat_modified")),
    statChip(created, t("stat_created"), "chip-add"),
    statChip(deleted, t("stat_deleted"), "chip-del"),
  );

  // El revert solo tiene sentido si la sesión tocó ficheros del usuario.
  $("btn-session-revert").disabled = files.length === 0;
  renderSessionEvents();
}

function renderSessionEvents() {
  const d = getState().sessionDetail;
  const list = $("session-events");
  const events = d.events || [];
  $("events-empty").hidden = events.length > 0;
  list.hidden = events.length === 0;

  // Si algún evento trae prompt, se agrupan por prompt; si no, lista plana.
  const anyPrompt = events.some((ev) => ev.prompt);
  if (!anyPrompt) {
    mount(list, events.map(eventItem));
    return;
  }
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
  mount(
    list,
    groups.map((g) => [
      g.prompt && h("li", { class: "prompt-header", text: g.prompt }),
      g.events.map(eventItem),
    ]),
  );
}

function eventItem(ev) {
  return h(
    "li",
    { class: "event-item" },
    h("span", { class: "ev-time", text: fmtDate(ev.ts) }),
    h("span", { class: "ev-type " + (ev.first ? "t-new" : "t-mod"), text: ev.first ? t("ev_new") : t("ev_modified") }),
    h("span", { class: "ev-path", text: ev.path, title: ev.path }),
    h(
      "span",
      { class: "ev-actions" },
      h("button", {
        type: "button",
        class: "btn btn-small",
        text: ev.first ? t("ev_view_file") : t("ev_view_diff"),
        onClick: () => openAgentDiff(ev),
      }),
      h("button", {
        type: "button",
        class: "btn btn-small",
        text: t("ev_revert_file"),
        onClick: () => openSessionRevert(ev.path),
      }),
    ),
  );
}

// --- visor de cambios de un evento ---

let agentDiffPath = null;

async function openAgentDiff(ev) {
  agentDiffPath = ev.path;
  $("agentdiff-title").textContent = ev.path + " · " + ev.version;
  const viewer = $("agentdiff-viewer");
  clear(viewer);
  openOverlay("agentdiff");

  let res;
  if (ev.prev) {
    const qs = new URLSearchParams({ path: ev.path, from: ev.prev, to: ev.version });
    res = await api("/api/diff?" + qs.toString());
  } else {
    const qs = new URLSearchParams({ path: ev.path, version: ev.version });
    res = await api("/api/content?" + qs.toString());
  }
  if (res.status === 415) return viewerMessage(viewer, t("binary_content"));
  if (!res.ok) return viewerMessage(viewer, t(ev.prev ? "load_diff_error" : "load_content_error"));
  const text = await res.text();
  if (!ev.prev) {
    viewer.replaceChildren(h("pre", { text }));
    return;
  }
  if (text.trim() === "") return viewerMessage(viewer, t("no_differences"));
  viewer.replaceChildren(renderDiffLines(text));
}

// openAgentFileHistory salta al historial completo del fichero en modo clásico.
function openAgentFileHistory() {
  if (!agentDiffPath) return;
  closeOverlay("agentdiff");
  setView("files");
  selectFile(agentDiffPath);
}

// --- exportar sesión (descarga del transcript) ---

async function downloadTranscript() {
  const d = getState().sessionDetail;
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
    const a = h("a", { href: URL.createObjectURL(blob), download: s.transcript.split("/").pop() });
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
  const d = getState().sessionDetail;
  if (!d) return;
  revertCtx = { start: d.session.start, end: d.session.end, target: target || "", manifest: null };
  $("revert-scope").textContent = revertCtx.target
    ? t("revert_scope_one", { path: revertCtx.target })
    : t("revert_scope_all");
  mount($("revert-plan"), h("p", { class: "revert-empty", text: t("revert_plan_loading") }));
  const confirm = $("revert-confirm");
  confirm.hidden = false;
  confirm.disabled = true;
  confirm.textContent = t("revert_confirm");
  $("revert-undo").hidden = true;
  openOverlay("revert");

  try {
    const plan = await apiJSON(
      "/api/agent/revert",
      jsonBody("POST", { start: revertCtx.start, end: revertCtx.end, target: revertCtx.target, dry_run: true }),
    );
    revertCtx.plan = plan;
    renderRevertPlan(plan);
  } catch (e) {
    clear($("revert-plan"));
    toast(e.message, "error");
  }
}

const REVERT_GROUPS = [
  ["revert", "plan_revert"],
  ["restore_deleted", "plan_restore_deleted"],
  ["skip_no_prior", "plan_skip_no_prior"],
  ["skip_unchanged", "plan_skip_unchanged"],
];

// renderRevertPlan pinta el plan agrupado por acción.
function renderRevertPlan(plan) {
  const box = $("revert-plan");
  const items = plan.items || [];
  if (items.length === 0) {
    mount(box, h("p", { class: "revert-empty", text: t("revert_none") }));
    $("revert-confirm").disabled = true;
    return;
  }
  let actionable = 0;
  const groups = [];
  for (const [action, key] of REVERT_GROUPS) {
    const group = items.filter((it) => it.action === action);
    if (group.length === 0) continue;
    if (action === "revert" || action === "restore_deleted") actionable += group.length;
    groups.push(
      h(
        "div",
        { class: "revert-group" },
        h("p", { class: "revert-group-title", text: t(key, { n: group.length }) }),
        h("ul", {}, group.map((it) => h("li", { text: it.path + (it.prior ? " ← " + it.prior : "") }))),
      ),
    );
  }
  mount(box, groups);
  $("revert-confirm").disabled = actionable === 0;
}

// doSessionRevert ejecuta el plan confirmado.
async function doSessionRevert() {
  if (!revertCtx) return;
  const btn = $("revert-confirm");
  btn.disabled = true;
  btn.textContent = t("revert_running");
  try {
    const res = await apiJSON(
      "/api/agent/revert",
      jsonBody("POST", { start: revertCtx.start, end: revertCtx.end, target: revertCtx.target, dry_run: false }),
    );
    revertCtx.manifest = res;
    renderRevertPlan(res);
    toast(t("revert_done", { n: res.reverted + res.restored_deleted }));
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
      await apiJSON("/api/restore", jsonBody("POST", { path: it.path, version: it.safety_version }));
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

// liveRefreshAgents mantiene la lista de sesiones al día mientras el agente
// trabaja (lo llama el coordinador de eventos en vivo).
export function liveRefreshAgents() {
  const { view, selAgent } = getState();
  if (view === "agents" && selAgent) loadSessions(selAgent, true).catch(() => {});
}

// rerenderAgents repinta el texto dinámico del modo Agente tras un cambio de idioma.
function rerenderAgents() {
  if (getState().view !== "agents") return;
  renderAgentList();
  renderSessions();
  if (getState().sessionDetail) renderSessionView();
  const { selAgent, sessions } = getState();
  if (selAgent) $("sessions-count").textContent = t("sessions_found", { n: sessions.length });
}
