// features/views.js — conmutación entre el modo Versionado y el modo Agente.
//
// El modo activo vive en store.view. Las piezas que dependen de él (mostrar la
// capa correcta, marcar la píldora de la cabecera) se repintan reactivamente; el
// modo Agente se inicializa a sí mismo suscribiéndose a este estado (ver
// features/agents.js). Aquí solo vive la conmutación y la sincronía con el hash.
import { $ } from "../core/dom.js";
import { fileHash, navigate } from "../core/router.js";
import { watchKeys } from "../core/store.js";
import { store, getState, setState } from "../state.js";

// initViews cablea la píldora de modo y suscribe el repintado de las capas.
export function initViews() {
  $("view-files").addEventListener("click", () => setView("files"));
  $("view-agents").addEventListener("click", () => setView("agents"));
  watchKeys(store, ["view"], (s) => renderView(s.view));
}

// setView cambia de modo. updateHash sincroniza la URL con la vista (se desactiva
// cuando el cambio proviene ya del propio hash, para no entrar en bucle).
export function setView(view, { updateHash = true } = {}) {
  if (updateHash) {
    if (view === "agents") {
      navigate("#/agents");
    } else if (location.hash.startsWith("#/agents")) {
      const sel = getState().selectedPath;
      navigate(sel ? fileHash(sel) : "", { push: false });
    }
  }
  setState({ view });
}

function renderView(view) {
  $("files-layout").hidden = view !== "files";
  $("agents-layout").hidden = view !== "agents";
  toggleSeg($("view-files"), view === "files");
  toggleSeg($("view-agents"), view === "agents");
}

function toggleSeg(btn, active) {
  btn.classList.toggle("active", active);
  btn.setAttribute("aria-selected", active ? "true" : "false");
}
