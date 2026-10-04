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
import {
  statChip,
  viewerMessage,
  renderDiffLines,
} from "../components/common.js";
import { setView } from "./views.js";
import { selectFile, refreshFiles } from "./files.js";
import { refreshStatus } from "./header.js";

// initAgents cablea los controles del modo Agente y su autoarranque por vista.
let sessionRequest = 0;
let sessionsRequest = 0;
let selectedSession = null;
let showCandidates = false;
let fileQuery = "";

export function initAgents() {
  $("session-search").addEventListener("input", renderSessions);
  $("session-file-search").addEventListener("input", (e) => {
    fileQuery = e.target.value;
    renderSessionFiles();
  });
  $("session-candidates").addEventListener("change", (e) => {
    showCandidates = e.target.checked;
    renderSessionFiles();
    renderSessionEvents();
  });
  $("btn-agents-refresh").addEventListener("click", refreshAgentsView);
  $("btn-transcript").addEventListener("click", downloadTranscript);
  $("btn-session-revert").addEventListener("click", () =>
    openSessionRevert(""),
  );
  $("revert-confirm").addEventListener("click", doSessionRevert);
  $("revert-undo").addEventListener("click", undoSessionRevert);
  $("agentdiff-history").addEventListener("click", openAgentFileHistory);

  // Al entrar en el modo Agente por primera vez, se inicializa la columna.
  watchKeys(
    store,
    ["view"],
    (s, prev) => {
      if (s.view === "agents" && (!prev || prev.view !== "agents"))
        initAgentsView();
    },
    { immediate: false },
  );
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
  const active = getState().agents.filter((a) => a.files > 0 || a.detected);
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
    const { selAgent } = getState();
    if (selAgent) {
      await loadSessions(selAgent, true);
      if (getState().sessionDetail)
        await selectSession(getState().sessionDetail.session.id, {
          preserve: true,
        });
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
  const active = getState().agents.filter((a) => a.files > 0 || a.detected);
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
          setState((s) => ({
            sessionCounts: {
              ...s.sessionCounts,
              [a.id]: (j.sessions || []).length,
            },
          }));
          renderAgentList();
        })
        .catch(() => {})
        .finally(() => sessionCountsInFlight.delete(a.id));
    }
  }
}

function agentItem(a, selected, count) {
  const sub =
    count != null
      ? t("agents_sessions_n", { n: count })
      : t("agents_files_n", { n: a.files });
  const li = h(
    "li",
    {
      class: "agent-item" + (selected ? " selected" : ""),
      role: "button",
      "aria-pressed": String(selected),
      tabindex: "0",
      onClick: () => selectAgent(a.id),
    },
    h("span", { class: "agent-ic", text: a.icon }),
    h(
      "div",
      {},
      h("div", { class: "a-name", text: a.name }),
      h("div", { class: "a-sub", text: sub }),
    ),
  );
  activateOnEnter(li, () => selectAgent(a.id));
  return li;
}

async function selectAgent(id) {
  sessionRequest++;
  selectedSession = null;
  $("session-search").value = "";
  const ag = getState().agentsById[id] || { name: id, icon: "✳" };
  // Vaciar la columna al instante: dejar las sesiones del agente anterior
  // visibles mientras cargan las nuevas invita a clicar datos equivocados.
  setState({ selAgent: id, sessionDetail: null, sessions: [] });
  renderAgentList();
  $("sessions-agent-icon").textContent = ag.icon;
  $("sessions-agent-name").textContent = ag.name;
  $("sessions-count").textContent = t("loading");
  clear($("session-list"));
  $("session-list").hidden = true;
  $("sessions-empty").hidden = true;
  $("session-view").hidden = true;
  $("session-empty").hidden = false;
  await loadSessions(id);
}

async function loadSessions(id, preserve = false) {
  const request = ++sessionsRequest;
  $("session-list").setAttribute("aria-busy", "true");
  let j;
  try {
    j = await apiJSON("/api/agent/sessions?agent=" + encodeURIComponent(id));
  } catch (e) {
    if (request !== sessionsRequest || getState().selAgent !== id) return;
    $("session-list").setAttribute("aria-busy", "false");
    $("sessions-count").textContent = t("load_sessions_error");
    toast(e.message, "error");
    return;
  }
  if (request !== sessionsRequest || getState().selAgent !== id) return; // el usuario cambió de agente mientras cargaba
  $("session-list").setAttribute("aria-busy", "false");
  const sessions = j.sessions || [];
  setState((s) => ({
    sessions,
    sessionCounts: { ...s.sessionCounts, [id]: sessions.length },
  }));
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
  if (s.kind === "cluster")
    return t("session_kind_cluster") + " · " + fmtDate(s.start);
  return t("session_untitled") + " · " + fmtDate(s.start);
}

function renderSessions() {
  const { sessions: allSessions } = getState();
  const query = $("session-search").value.trim().toLocaleLowerCase();
  const sessions = allSessions.filter((s) =>
    [s.title, s.project, s.id].some((v) =>
      (v || "").toLocaleLowerCase().includes(query),
    ),
  );
  const list = $("session-list");
  $("sessions-empty").hidden = sessions.length > 0;
  list.hidden = sessions.length === 0;
  const selID = selectedSession;
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
      "aria-pressed": String(selected),
      tabindex: "0",
      onClick: () => selectSession(s.id),
    },
    h("div", { class: "s-title", text: sessionTitle(s) }),
    h("div", { class: "s-meta", text: fmtDate(s.start) }),
    h("div", { class: "s-project", text: s.workspace || t("scope_unknown") }),
    h("div", {
      class: "s-evidence",
      text:
        t("confirmed_n", { n: s.confirmed_files || 0 }) +
        " · " +
        t("candidates_n", { n: s.candidate_files || 0 }),
    }),
  );
  activateOnEnter(li, () => selectSession(s.id));
  return li;
}

async function selectSession(id, { preserve = false } = {}) {
  const agent = getState().selAgent;
  const request = ++sessionRequest;
  selectedSession = id;
  renderSessions();
  if (!preserve) {
    setState({ sessionDetail: null });
    $("session-view").hidden = true;
    $("session-empty").hidden = false;
    $("session-empty").setAttribute("aria-busy", "true");
    $("session-empty").querySelector(".panel-empty-title").textContent =
      t("loading");
    fileQuery = "";
    $("session-file-search").value = "";
    showCandidates = false;
    $("session-candidates").checked = false;
  }
  try {
    const j = await apiJSON(
      "/api/agent/session?" + new URLSearchParams({ agent, id }),
    );
    if (request !== sessionRequest || agent !== getState().selAgent) return;
    setState({ sessionDetail: j });
    renderSessions();
    renderSessionView();
  } catch (e) {
    if (request !== sessionRequest || agent !== getState().selAgent) return;
    $("session-empty").querySelector(".panel-empty-title").textContent = t(
      "load_sessions_error",
    );
    toast(e.message, "error");
  } finally {
    if (request === sessionRequest)
      $("session-empty").setAttribute("aria-busy", "false");
  }
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
  metaParts.push(t("confirmed_n", { n: s.confirmed_files || 0 }));
  $("session-meta").textContent = metaParts.join(" · ");

  $("btn-transcript").hidden = !s.transcript;

  const files = d.files || [];
  const confirmed = files.filter((f) => f.attribution === "tool");
  const candidates = files.length - confirmed.length;
  mount(
    $("session-stats"),
    statChip(confirmed.length, t("stat_confirmed"), "chip-add"),
    statChip(candidates, t("stat_candidates")),
    statChip(
      confirmed.filter((f) => !f.backed_up).length,
      t("stat_unprotected"),
      "chip-del",
    ),
  );
  const notes = [t("session_evidence_note")];
  if (s.scope === "unknown") notes.push(t("scope_unknown_note"));
  if (s.scope === "outside_base") notes.push(t("scope_outside_note"));
  if (s.time_source === "backups") notes.push(t("time_backup_note"));
  if (s.partial || s.invalid_lines) notes.push(t("transcript_partial_note"));
  $("session-reliability").textContent = notes.join(" ");
  $("session-candidate-label").textContent = t("show_candidates", {
    n: candidates,
  });
  $("btn-session-revert").disabled = !confirmed.some(
    (f) => f.prior && f.backed_up,
  );
  $("btn-session-revert").title = t("restore_confirmed_only");
  renderSessionFiles();
  renderSessionEvents();
}

// The audit starts with one row per file, not a repeated timeline of versions.
function renderSessionFiles() {
  const d = getState().sessionDetail;
  if (!d) return;
  const files = (d.files || []).filter(
    (f) =>
      (showCandidates || f.attribution === "tool") &&
      f.path.toLocaleLowerCase().includes(fileQuery.toLocaleLowerCase()),
  );
  const opened = new Set(
    [...$("session-files").querySelectorAll("details[open]")].map(
      (el) => el.dataset.path,
    ),
  );
  const groups = new Map();
  for (const f of files) {
    const folder = f.path.includes("/")
      ? f.path.slice(0, f.path.lastIndexOf("/"))
      : ".";
    if (!groups.has(folder)) groups.set(folder, []);
    groups.get(folder).push(f);
  }
  $("session-files-empty").hidden = files.length > 0;
  $("session-files-empty").textContent = fileQuery
    ? t("no_matches")
    : t(showCandidates ? "no_session_files" : "no_confirmed_files");
  mount(
    $("session-files"),
    [...groups.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([folder, members]) =>
        h(
          "section",
          { class: "session-folder" },
          h("h3", {
            class: "folder-title",
            text: folder + " · " + members.length,
          }),
          h("ul", { class: "audit-files" }, members.map(fileAuditItem)),
        ),
      ),
  );
  for (const el of $("session-files").querySelectorAll("details"))
    el.open = opened.has(el.dataset.path);
}

function fileAuditItem(f) {
  const d = getState().sessionDetail;
  const events = (d.events || []).filter((ev) => ev.path === f.path);
  const evidence = f.evidence || [];
  const latest = events[0];
  const first = events[events.length - 1];
  const confirmed = f.attribution === "tool";
  const relative =
    d.session.workspace && f.path.startsWith(d.session.workspace + "/")
      ? f.path.slice(d.session.workspace.length + 1)
      : f.path;
  const details = h(
    "details",
    {
      class: "audit-file",
      dataset: { path: f.path, attribution: f.attribution },
    },
    h(
      "summary",
      {},
      h("span", { class: "audit-file-name", text: relative, title: f.path }),
      h("span", {
        class: "audit-badge " + (confirmed ? "confirmed" : "candidate"),
        text: t(confirmed ? "attribution_tool" : "attribution_temporal"),
      }),
      h("span", {
        class: "audit-file-status",
        text: t(
          !f.backed_up
            ? "file_unprotected"
            : f.deleted
              ? "file_missing"
              : f.created
                ? "file_first_backup"
                : "file_versioned",
        ),
      }),
    ),
    h(
      "div",
      { class: "audit-file-body" },
      h("p", { class: "audit-full-path", text: f.path }),
      h("p", {
        class: "audit-evidence",
        text: confirmed
          ? t("tool_evidence", {
              tools: [...new Set(evidence.map((e) => e.tool))].join(", "),
              n: evidence.length,
            })
          : t("temporal_evidence"),
      }),
      (f.shared_with || []).length > 0 &&
        h(
          "div",
          { class: "audit-shared" },
          h("p", { text: t("shared_file_note") }),
          (f.shared_with || []).map((other) =>
            h("button", {
              type: "button",
              class: "btn btn-small",
              text:
                (getState().agentsById[other.agent]?.name || other.agent) +
                " · " +
                (other.title || t("session_untitled")),
              onClick: async () => {
                await selectAgent(other.agent);
                if (getState().selAgent === other.agent)
                  selectSession(other.id);
              },
            }),
          ),
        ),
      evidence.length > 0 &&
        h(
          "ul",
          { class: "tool-evidence" },
          evidence.map((e) =>
            h("li", { text: fmtDate(e.time) + " · " + e.tool }),
          ),
        ),
      h(
        "div",
        { class: "audit-actions" },
        latest &&
          h("button", {
            type: "button",
            class: "btn btn-small",
            text: t(f.prior || first?.prev ? "ev_view_diff" : "ev_view_file"),
            onClick: () =>
              openAgentDiff({ ...latest, prev: f.prior || first?.prev || "" }),
          }),
        f.backed_up &&
          h("button", {
            type: "button",
            class: "btn btn-small",
            text: t("ev_open_history"),
            onClick: () => {
              setView("files");
              selectFile(f.path);
            },
          }),
        f.prior &&
          f.backed_up &&
          h("button", {
            type: "button",
            class: "btn btn-small",
            text: t("ev_revert_file"),
            onClick: () => openSessionRevert(f.path),
          }),
      ),
      !f.backed_up &&
        h("p", { class: "audit-warning", text: t("no_backup_recovery") }),
      f.backed_up &&
        !f.prior &&
        h("p", { class: "audit-warning", text: t("no_prior_recovery") }),
    ),
  );
  return h("li", {}, details);
}

function renderSessionEvents() {
  const d = getState().sessionDetail;
  const list = $("session-events");
  const events = (d.events || []).filter(
    (ev) =>
      showCandidates ||
      (d.files || []).some(
        (f) => f.path === ev.path && f.attribution === "tool",
      ),
  );
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
      g.prompt &&
        h("li", {
          class: "prompt-header",
          text: t("prompt_context") + " · " + g.prompt,
          title: t("temporal_evidence"),
        }),
      g.events.map(eventItem),
    ]),
  );
}

function eventItem(ev) {
  return h(
    "li",
    { class: "event-item" },
    h("span", { class: "ev-time", text: fmtDate(ev.ts) }),
    h("span", {
      class: "ev-type " + (ev.first ? "t-new" : "t-mod"),
      text: ev.first ? t("ev_new") : t("ev_modified"),
    }),
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

let diffRequest = 0;

async function openAgentDiff(ev) {
  const request = ++diffRequest;
  try {
    agentDiffPath = ev.path;
    $("agentdiff-title").textContent = ev.path + " · " + ev.version;
    const viewer = $("agentdiff-viewer");
    clear(viewer);
    openOverlay("agentdiff");

    let res;
    if (ev.prev) {
      const qs = new URLSearchParams({
        path: ev.path,
        from: ev.prev,
        to: ev.version,
      });
      res = await api("/api/diff?" + qs.toString());
    } else {
      const qs = new URLSearchParams({ path: ev.path, version: ev.version });
      res = await api("/api/content?" + qs.toString());
    }
    if (request !== diffRequest) return;
    if (res.status === 415) return viewerMessage(viewer, t("binary_content"));
    if (!res.ok)
      return viewerMessage(
        viewer,
        t(ev.prev ? "load_diff_error" : "load_content_error"),
      );
    const text = await res.text();
    if (request !== diffRequest) return;
    if (!ev.prev) {
      viewer.replaceChildren(h("pre", { text }));
      return;
    }
    if (text.trim() === "") return viewerMessage(viewer, t("no_differences"));
    viewer.replaceChildren(renderDiffLines(text));
  } catch (e) {
    if (request === diffRequest) toast(e.message, "error");
  }
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
    // Exportar exactamente la copia guardada que se ha auditado.
    const qs = new URLSearchParams({
      agent: getState().selAgent,
      id: s.id,
      version: s.transcript_version,
    });
    const res = await api("/api/agent/transcript?" + qs.toString());
    if (!res.ok) throw new Error("HTTP " + res.status);
    const blob = await res.blob();
    const a = h("a", {
      href: URL.createObjectURL(blob),
      download: s.transcript.split("/").pop(),
    });
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
  revertCtx = {
    agent: getState().selAgent,
    session: d.session.id,
    start: d.session.start,
    end: d.session.end,
    target: target || "",
    manifest: null,
  };
  const context = revertCtx;
  $("revert-undo").disabled = false;
  $("revert-scope").textContent = revertCtx.target
    ? t("revert_scope_one", { path: revertCtx.target })
    : t("revert_scope_all");
  mount(
    $("revert-plan"),
    h("p", { class: "revert-empty", text: t("revert_plan_loading") }),
  );
  const confirm = $("revert-confirm");
  confirm.hidden = false;
  confirm.disabled = true;
  confirm.textContent = t("revert_confirm");
  $("revert-undo").hidden = true;
  openOverlay("revert");

  try {
    const plan = await apiJSON(
      "/api/agent/revert",
      jsonBody("POST", { ...context, dry_run: true }),
    );
    if (revertCtx !== context) return;
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
  ["skip_conflict", "plan_skip_conflict"],
  ["error", "plan_failed"],
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
    if (action === "revert" || action === "restore_deleted")
      actionable += group.length;
    groups.push(
      h(
        "div",
        { class: "revert-group" },
        h("p", {
          class: "revert-group-title",
          text: t(key, { n: group.length }),
        }),
        h(
          "ul",
          {},
          group.map((it) =>
            h("li", {
              text:
                it.path +
                (it.error
                  ? " · " + it.error
                  : it.prior
                    ? " ← " + it.prior
                    : ""),
            }),
          ),
        ),
      ),
    );
  }
  mount(box, groups);
  $("revert-confirm").disabled = actionable === 0;
}

// doSessionRevert ejecuta el plan confirmado.
async function doSessionRevert() {
  if (!revertCtx) return;
  const context = revertCtx;
  const btn = $("revert-confirm");
  btn.disabled = true;
  btn.textContent = t("revert_running");
  try {
    const res = await apiJSON(
      "/api/agent/revert",
      jsonBody("POST", {
        agent: context.agent,
        session: context.session,
        start: context.start,
        end: context.end,
        target: context.target,
        dry_run: false,
      }),
    );
    context.manifest = res;
    if (revertCtx !== context) {
      await refreshFiles();
      await refreshStatus();
      return;
    }
    renderRevertPlan(res);
    toast(
      t("revert_done", { n: res.reverted + res.restored_deleted }),
      res.failed ? "error" : undefined,
    );
    if (res.failed) toast(t("revert_failures", { n: res.failed }), "error");
    btn.hidden = true;
    // Deshacer disponible si alguna restauración dejó safety version.
    $("revert-undo").hidden = !(res.items || []).some(
      (it) => it.safety_version,
    );
    await refreshFiles();
    await refreshStatus();
  } catch (e) {
    toast(e.message, "error");
    btn.disabled = false;
  } finally {
    if (revertCtx === context) btn.textContent = t("revert_confirm");
  }
}

// undoSessionRevert restaura las safety versions del manifiesto (deshace la
// reversión). Los borrados recuperados sin safety se conservan: nunca borramos.
async function undoSessionRevert() {
  const context = revertCtx;
  const m = context && context.manifest;
  if (!m) return;
  const btn = $("revert-undo");
  btn.disabled = true;
  let n = 0;
  try {
    for (const it of m.items || []) {
      if (!it.safety_version) continue;
      await apiJSON(
        "/api/restore",
        jsonBody("POST", { path: it.path, version: it.safety_version }),
      );
      n++;
    }
    toast(t("revert_undo_done", { n }));
    if (revertCtx === context) closeOverlay("revert");
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
  if (view === "agents") {
    loadAgents().then(renderAgentList);
    if (selAgent)
      loadSessions(selAgent, true)
        .then(() => {
          if (getState().sessionDetail)
            selectSession(getState().sessionDetail.session.id, {
              preserve: true,
            });
        })
        .catch(() => {});
  }
}

// rerenderAgents repinta el texto dinámico del modo Agente tras un cambio de idioma.
function rerenderAgents() {
  if (getState().view !== "agents") return;
  renderAgentList();
  renderSessions();
  if (getState().sessionDetail) renderSessionView();
  const { selAgent, sessions } = getState();
  if (selAgent)
    $("sessions-count").textContent = t("sessions_found", {
      n: sessions.length,
    });
}
