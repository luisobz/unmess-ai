// state.js — store central de la UI (única fuente de verdad del estado).
//
// Sustituye al objeto `state` mutable global del monolito anterior. Las vistas se
// suscriben a sus claves con watchKeys (ver core/store.js) y las acciones de cada
// feature mutan el estado con store.set(...). El token de sesión NO vive aquí:
// lo gestiona core/api.js.
import { createStore } from "./core/store.js";
import { I18N } from "./core/i18n.js";

export const store = createStore({
  // Idioma activo. Cambia con I18N.setLang; main.js lo sincroniza aquí para que
  // las vistas repinten su texto dinámico de forma reactiva (ver watchKeys "lang").
  lang: I18N.lang(),

  // Navegación
  view: "files", // "files" (versionado clásico) | "agents" (modo Agente)

  // Cabecera / estado del daemon
  paused: false,
  status: null, // última respuesta de /api/status
  agentsById: {}, // id -> {name, icon} para pintar chips de agente en la lista

  // Modo Versionado
  filter: "all", // "all" | "modified" | "deleted"
  // aiFilter tri-estado: "all" (todos) | "only" (solo IA) | "hide" (sin IA)
  aiFilter: localStorage.getItem("unmessai-ai-filter") || "all",
  query: "",
  files: [],
  selectedPath: null,
  deleted: false,
  versions: [],
  selectedVersion: null, // versión "to" (la de la izquierda en la lista)
  compareTo: null, // versión "from" contra la que comparar
  mode: "diff", // "diff" | "content"

  // Modo Agente
  agents: [], // registro completo de /api/agents (con conteo de ficheros)
  sessionCounts: {}, // id de agente -> nº de sesiones (carga perezosa)
  selAgent: null,
  sessions: [],
  sessionDetail: null, // {session, events, files} de la sesión abierta
});

// get/set/subscribe reexpuestos para comodidad de las features.
export const getState = store.get;
export const setState = store.set;
