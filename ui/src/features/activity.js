// features/activity.js — panel de Actividad reciente (journal).
import { $, h, mount, activateOnEnter } from "../core/dom.js";
import { apiJSON } from "../core/api.js";
import { fmtDate } from "../core/format.js";
import { openOverlay, closeOverlay } from "../core/overlay.js";
import { selectFile } from "./files.js";

// initActivity cablea el botón que abre el panel.
export function initActivity() {
  $("btn-activity").addEventListener("click", () => {
    openOverlay("activity");
    refreshJournal();
  });
}

// activityOpen indica si el panel está visible (para los refrescos en vivo).
export function activityOpen() {
  return !$("activity-overlay").hidden;
}

// refreshJournal recarga las últimas entradas del journal.
export async function refreshJournal() {
  let entries;
  try {
    entries = await apiJSON("/api/journal?limit=50");
  } catch (_) {
    return;
  }
  const list = $("journal-list");
  if (entries.length === 0) {
    mount(list);
    list.hidden = true;
    $("journal-empty").hidden = false;
    return;
  }
  list.hidden = false;
  $("journal-empty").hidden = true;
  mount(list, entries.map(journalItem));
}

function journalItem(e) {
  const open = () => {
    closeOverlay("activity");
    selectFile(e.path);
  };
  const li = h(
    "li",
    { class: "journal-item", role: "button", tabindex: "0", onClick: open },
    h("div", { class: "j-path", text: e.path }),
    h("div", { class: "j-ts", text: fmtDate(e.ts) }),
  );
  activateOnEnter(li, open);
  return li;
}
