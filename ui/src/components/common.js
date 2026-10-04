// components/common.js — piezas de UI reutilizables entre features.
import { h } from "../core/dom.js";
import { t } from "../core/i18n.js";

// badge crea una etiqueta pequeña (p. ej. "actual", "borrado").
export function badge(text, variant) {
  return h("span", { class: "badge" + (variant ? " badge-" + variant : ""), text });
}

// deletedBadge es el distintivo de fichero borrado.
export function deletedBadge() {
  return badge(t("deleted_badge"), "deleted");
}

// agentChip pinta el chip de agente de un fichero. `agent` es {name, icon} o null;
// id es el identificador crudo por si no hay entrada en el registro.
export function agentChip(agent, id) {
  return h(
    "span",
    { class: "fi-agent" },
    h("span", { class: "fi-agent-icon", text: agent ? agent.icon : "✳" }),
    agent ? agent.name : id,
  );
}

// statChip crea un chip de estadística con un número destacado y una etiqueta.
export function statChip(n, label, variant) {
  return h(
    "span",
    { class: "stat-chip" + (variant ? " " + variant : "") },
    h("strong", { text: String(n) }),
    " " + label,
  );
}

// viewerMessage sustituye el contenido de un visor por un mensaje centrado.
export function viewerMessage(viewer, msg) {
  viewer.replaceChildren(h("div", { class: "viewer-empty", text: msg }));
}

// diffClass clasifica una línea de un diff unificado para colorearla.
export function diffClass(line) {
  if (line.startsWith("+++") || line.startsWith("---")) return "diff-meta";
  if (line.startsWith("@@")) return "diff-hunk";
  if (line.startsWith("+")) return "diff-add";
  if (line.startsWith("-")) return "diff-del";
  return "";
}

// renderDiffLines pinta un diff unificado como líneas coloreadas dentro de un
// fragmento (las líneas vacías se sustituyen por un espacio para conservar alto).
export function renderDiffLines(text) {
  const frag = document.createDocumentFragment();
  for (const line of text.split("\n")) {
    frag.appendChild(
      h("span", { class: "diff-line " + diffClass(line), text: line === "" ? " " : line }),
    );
  }
  return frag;
}
