// i18n.js — traducciones de la UI de unmessai.
//
// Para añadir un idioma: añade una clave a `translations` con el mismo conjunto
// de claves que "es" y una entrada en `languages`. Todo lo demás (selector de
// ajustes, atributos data-i18n del HTML) funciona sin tocar nada.

const translations = {
  es: {
    app_title: "unmessai — historial de versiones",
    skip_to_content: "Saltar al contenido",
    stat_store: "Store",
    stat_files: "Ficheros",
    protection_active: "Protección activa",
    protection_paused: "Protección en pausa",
    pause: "Pausar",
    resume: "Reanudar",
    paused_toast: "Protección en pausa.",
    resumed_toast: "Protección reanudada.",
    activity: "Actividad",
    settings: "Ajustes",
    about: "Acerca de",
    toggle_theme: "Cambiar tema claro/oscuro",
    search_files: "Buscar ficheros…",
    search_files_label: "Buscar ficheros",
    filter_all: "Todos",
    filter_modified: "Modificados",
    filter_deleted: "Borrados",
    ai_filter_all: "Filtro IA: mostrando todos los ficheros",
    ai_filter_only: "Filtro IA: solo ficheros de agentes de IA",
    ai_filter_hide: "Filtro IA: ocultando ficheros de agentes de IA",
    refresh: "Refrescar",
    mode_label: "Modo actual",
    mode_files: "Versionado",
    mode_agents: "Agente",
    loading: "Cargando…",
    load_sessions_error: "No se pudo cargar la sesión. Vuelve a intentarlo.",
    search_sessions: "Buscar sesión o proyecto",
    search_session_files: "Filtrar archivos de la sesión",
    confirmed_n: "{n} con evidencia",
    candidates_n: "{n} coincidencias",
    scope_unknown: "Proyecto sin identificar",
    stat_confirmed: "con escritura registrada",
    stat_candidates: "coincidencias temporales",
    stat_unprotected: "sin copia en esta ventana",
    session_evidence_note:
      "La atribución se basa en herramientas de escritura con resultado correcto registradas en el transcript. No identifica cambios realizados por comandos de shell ni demuestra que una versión guardada contenga exclusivamente cambios de este agente.",
    scope_unknown_note:
      "Sin proyecto identificado: las coincidencias pueden pertenecer a otros proyectos.",
    scope_outside_note:
      "El proyecto está fuera de la base protegida. Solo se muestran escrituras explícitas sobre archivos dentro de la base.",
    time_backup_note:
      "El intervalo procede de las copias guardadas, no de los tiempos originales de la sesión.",
    transcript_partial_note:
      "Lectura parcial o registros dañados: la auditoría puede estar incompleta.",
    show_candidates: "Mostrar también coincidencias temporales ({n})",
    restore_confirmed_only:
      "Restaurar solo archivos con escritura registrada; se omiten cambios posteriores.",
    session_files_heading: "Archivos de esta sesión",
    no_matches: "Sin resultados para esta búsqueda.",
    no_session_files:
      "No hay archivos guardados ni escrituras reconocidas en esta ventana.",
    no_confirmed_files:
      "No hay escrituras confirmadas por herramientas compatibles. Puedes mostrar las coincidencias temporales; no prueban quién modificó esos archivos.",
    attribution_tool: "Escritura registrada",
    attribution_temporal: "Sin atribución confirmada",
    file_unprotected: "Sin copia en la sesión",
    file_missing: "Ausente ahora",
    file_first_backup: "Primera copia conocida",
    file_versioned: "Con historial",
    no_backup_recovery:
      "La operación está registrada, pero no hay una copia guardada durante esta sesión. Revisa las exclusiones y protege el proyecto antes de trabajar.",
    no_prior_recovery:
      "No hay copia anterior al inicio de esta sesión. La primera copia no demuestra que el archivo se creara entonces.",
    tool_evidence:
      "Herramientas: {tools} · {n} operación(es) con resultado correcto.",
    temporal_evidence:
      "Este archivo coincide con el intervalo de la sesión. Otro agente o el usuario pudo modificarlo.",
    plan_skip_conflict:
      "Trabajo posterior, compartido o no verificable: se conserva ({n})",
    revert_failures:
      "{n} archivo(s) no se pudieron restaurar; revisa los errores del plan.",
    prompt_context: "Contexto temporal del usuario",
    shared_file_note:
      "Este archivo también tiene escrituras registradas en otra sesión simultánea. La copia guardada puede contener cambios de ambas:",
    agents_heading: "Familias de agentes",
    no_agents_title: "Sin actividad de agentes.",
    no_agents_body:
      "Cuando un agente de IA (Claude Code, Gemini, Codex…) trabaje en tu equipo, sus sesiones aparecerán aquí automáticamente.",
    agents_tip:
      "Las familias aparecen al detectar datos locales. Solo las sesiones con transcript guardado permiten auditar escrituras registradas.",
    agents_files_n: "{n} ficheros",
    agents_sessions_n: "{n} sesiones",
    sessions_found: "{n} sesiones encontradas",
    no_sessions_title: "Sin sesiones detectadas.",
    no_sessions_body:
      "Este agente aún no tiene historial versionado. Asegúrate de que su carpeta está dentro de las rutas vigiladas.",
    session_select_title: "Selecciona una sesión",
    session_select_body:
      "Elige una sesión para revisar el trabajo del agente, seguir su traza de cambios y revertir lo que necesites.",
    session_untitled: "Sesión",
    session_kind_cluster: "Actividad agrupada",
    changes_n: "{n} copias guardadas",
    change_singular: "cambio",
    change_plural: "cambios",
    session_transcript: "Exportar sesión",
    session_revert: "Restaurar sesión",
    session_chronology: "Versiones guardadas durante la sesión",
    session_no_changes:
      "No hay versiones guardadas en esta ventana. Esto no demuestra que el agente no haya modificado archivos; revisa la evidencia de herramientas y las exclusiones.",
    session_scope_note:
      "Las copias reflejan el contenido observado por unmessai. La coincidencia temporal no prueba quién modificó un archivo.",
    stat_modified: "modificados",
    stat_created: "sin copia anterior",
    stat_deleted: "ausentes ahora",
    ev_new: "Primera copia",
    ev_modified: "Modificado",
    ev_view_diff: "Ver cambios",
    ev_view_file: "Ver fichero",
    ev_open_history: "Ver historial",
    ev_revert_file: "Revertir",
    revert_title: "Restaurar cambios de la sesión",
    revert_scope_all:
      "Se restaurarán solo archivos con escritura registrada en esta sesión. Las coincidencias temporales quedan fuera; el trabajo posterior se conserva.",
    revert_scope_one: "Ámbito: {path}",
    revert_plan_loading: "Calculando el plan de reversión…",
    revert_none: "No hay nada que restaurar en este ámbito.",
    plan_revert: "Se restaurarán a su estado anterior ({n})",
    plan_restore_deleted: "Borrados que se recuperarán ({n})",
    plan_skip_no_prior: "Sin copia anterior: se conservan ({n})",
    plan_skip_unchanged: "Ya en su estado anterior ({n})",
    plan_failed: "Fallos ({n})",
    revert_safety:
      "Antes de sobrescribir un archivo existente se guarda una copia de seguridad. Deshacer recupera esas copias; los archivos recuperados que estaban ausentes no se vuelven a borrar.",
    revert_confirm: "Restaurar",
    revert_running: "Restaurando…",
    revert_done: "Reversión completada: {n} fichero(s) restaurados.",
    revert_undo: "Deshacer reversión",
    revert_undo_done: "Reversión deshecha: {n} fichero(s) recuperados.",
    transcript_error: "No se pudo descargar el transcript: {msg}",
    files_heading: "Archivos",
    no_files_title: "Aún no hay ficheros versionados.",
    no_files_body:
      "unmessai versiona tus archivos en segundo plano. En cuanto modifiques un fichero vigilado aparecerá aquí su historial.",
    storage: "Almacenamiento",
    storage_used: "{size} en {files} ficheros",
    select_file_title: "Selecciona un fichero",
    select_file_body:
      "Elige un fichero de la izquierda para ver sus versiones, comparar cambios y restaurar.",
    versions: "Versiones",
    version_singular: "versión",
    version_plural: "versiones",
    current_badge: "actual",
    deleted_badge: "borrado",
    restore_version: "Restaurar versión",
    untrack: "Dejar de proteger",
    copy_path: "Copiar ruta",
    path_copied: "Ruta copiada.",
    mode_diff: "Diff",
    mode_content: "Contenido",
    compare_with: "Comparar con",
    compare_current: "En disco (actual)",
    no_versions: "Sin versiones.",
    binary_content: "Contenido binario: no se puede mostrar.",
    binary_diff: "Contenido binario: no se puede mostrar el diff.",
    load_content_error: "No se pudo cargar el contenido.",
    load_diff_error: "No se pudo cargar el diff.",
    no_previous_version: "No hay versión anterior con la que comparar.",
    no_differences: "Sin diferencias entre estas versiones.",
    recent_activity: "Actividad reciente",
    no_activity: "Sin actividad todavía.",
    close: "Cerrar",
    // Ajustes
    settings_title: "Ajustes",
    tab_general: "General",
    tab_watch: "Vigilancia",
    tab_ignored: "Ignorados",
    tab_retention: "Retención",
    language: "Idioma",
    language_hint: "Idioma de la interfaz.",
    cfg_prefix: "Carpeta del backup",
    cfg_prefix_hint: "Raíz donde se guardan las versiones (solo lectura).",
    cfg_port: "Puerto de la UI",
    cfg_port_hint: "Puerto local del servidor de la interfaz.",
    cfg_debounce: "Debounce (segundos)",
    cfg_debounce_hint: "Espera tras el último cambio antes de versionar.",
    cfg_included: "Rutas vigiladas",
    cfg_included_hint: "Una por línea. Admiten ~.",
    cfg_excluded: "Rutas excluidas",
    cfg_excluded_hint: "Una por línea. No se vigilan.",
    cfg_ignore_patterns: "Patrones de ignorado",
    cfg_ignore_patterns_hint:
      'Uno por línea, estilo gitignore y sin distinguir mayúsculas: ".config" (carpeta en cualquier nivel), "**/*.log", "**/cache/**".',
    cfg_exclude_names: "Nombres de carpeta excluidos",
    cfg_exclude_names_hint:
      "Uno por línea; se excluyen en cualquier nivel (p. ej. node_modules).",
    cfg_gitignore: "Respetar .gitignore",
    cfg_gitignore_hint:
      "No versionar lo que git ya ignora en cada repositorio.",
    purge_ignored: "Purgar ignorados del historial",
    purge_ignored_hint:
      "Elimina del historial los ficheros ya trackeados que coinciden con los patrones de ignorado.",
    purge_ignored_done: "{count} fichero(s) olvidados, {size} liberados.",
    purge_ignored_none: "Ningún fichero trackeado coincide con los patrones.",
    retention_max_versions: "Máx. versiones por fichero",
    retention_max_age: "Máx. antigüedad (días)",
    retention_deleted_age: "Purgar borrados tras (días)",
    retention_min_keep: "Mínimo a conservar",
    cancel: "Cancelar",
    save: "Guardar",
    settings_saved: "Ajustes guardados y aplicados.",
    settings_saved_restart:
      "Ajustes guardados. Reinicia unmessai para aplicar la carpeta de versiones o el puerto.",
    flush_now: "Versionar ahora",
    flush_done: "{n} cambio(s) pendiente(s) versionado(s).",
    flush_none: "Todo al día: no había cambios pendientes.",
    flush_error: "No se pudo versionar ahora: {msg}",
    // Proteger existentes
    protect_existing: "Proteger existentes",
    protect_title: "Proteger ficheros existentes",
    protect_desc:
      "unmessai versiona un fichero la primera vez que cambia, así que lo que no ha cambiado desde la instalación aún no tiene historial: si algo lo borra o lo sobrescribe, no habría estado anterior que restaurar. Esta acción guarda una versión inicial de esos ficheros.",
    protect_path_label: "Ruta a proteger",
    protect_path_hint:
      "Fichero o carpeta; ~ es tu carpeta personal. Se aplican los filtros de vigilancia (exclusiones, ignorados, tamaño máximo).",
    protect_note:
      "Los ficheros que ya tienen historial no se tocan: puedes repetirlo cuando quieras.",
    protect_confirm: "Proteger",
    protect_running: "Protegiendo…",
    protect_done:
      "{n} fichero(s) protegidos con versión inicial ({m} ya tenían historial).",
    protect_all_tracked:
      "Todo protegido: los {m} fichero(s) encontrados ya tenían historial.",
    protect_none_matched:
      "Ningún fichero coincide (revisa la ruta, los filtros y las exclusiones).",
    protect_error: "No se pudo proteger: {msg}",
    // Acerca de
    about_title: "Acerca de unmessai",
    about_desc:
      "unmessai protege tus archivos versionándolos automáticamente en segundo plano. Local-first: nada sale de tu equipo.",
    about_version: "Versión",
    website: "unmessai.com",
    about_thanks: "Gracias por apoyar el proyecto:",
    // Restaurar
    restore_title: "Restaurar versión",
    restore_desc: 'Vas a restaurar "{path}" a la versión {version}.',
    restore_safety:
      "Antes de restaurar se guardará automáticamente una copia de seguridad del estado actual como una versión nueva, para que nunca pierdas datos.",
    restore_confirm: "Restaurar",
    restored_with_safety: "Restaurado. Copia de seguridad guardada: {version}",
    restored_ok: "Restaurado correctamente.",
    // Dejar de proteger
    untrack_title: "Dejar de proteger",
    untrack_desc:
      'Se eliminará todo el historial de versiones de "{path}". El fichero en disco no se toca.',
    untrack_note:
      "Si el fichero no coincide con ningún patrón de ignorado ni ruta excluida, volverá a versionarse en el próximo cambio. Añádelo a los patrones de ignorado en Ajustes → Ignorados para dejar de trackearlo definitivamente.",
    untrack_confirm: "Eliminar historial",
    untracked_ok: "Historial eliminado ({size} liberados).",
    // Genéricos
    login_error: "No se pudo iniciar sesión local: {msg}",
    new_version_saved: "Nueva versión guardada",
    restore_done: "Restauración completada",
  },
  en: {
    app_title: "unmessai — version history",
    skip_to_content: "Skip to content",
    stat_store: "Store",
    stat_files: "Files",
    protection_active: "Protection active",
    protection_paused: "Protection paused",
    pause: "Pause",
    resume: "Resume",
    paused_toast: "Protection paused.",
    resumed_toast: "Protection resumed.",
    activity: "Activity",
    settings: "Settings",
    about: "About",
    toggle_theme: "Toggle light/dark theme",
    search_files: "Search files…",
    search_files_label: "Search files",
    filter_all: "All",
    filter_modified: "Modified",
    filter_deleted: "Deleted",
    ai_filter_all: "AI filter: showing all files",
    ai_filter_only: "AI filter: only AI-agent files",
    ai_filter_hide: "AI filter: hiding AI-agent files",
    refresh: "Refresh",
    mode_label: "Current mode",
    mode_files: "Versioning",
    mode_agents: "Agent",
    loading: "Loading\u2026",
    load_sessions_error: "Could not load the session. Try again.",
    search_sessions: "Search sessions or projects",
    search_session_files: "Filter session files",
    confirmed_n: "{n} with evidence",
    candidates_n: "{n} time matches",
    scope_unknown: "Unknown workspace",
    stat_confirmed: "with recorded writes",
    stat_candidates: "time matches",
    stat_unprotected: "without a backup in this window",
    session_evidence_note:
      "Attribution comes from recorded write tools with successful results. It does not identify shell command edits or prove a saved version contains changes exclusively from this agent.",
    scope_unknown_note:
      "No workspace identified: time matches may belong to other projects.",
    scope_outside_note:
      "The workspace is outside the protected base. Only explicit writes to files within the base are shown.",
    time_backup_note:
      "The interval comes from saved backups rather than original session timestamps.",
    transcript_partial_note:
      "Partial reading or damaged records: this audit may be incomplete.",
    show_candidates: "Also show temporal matches ({n})",
    restore_confirmed_only:
      "Restore only files with recorded writes; later work is skipped.",
    session_files_heading: "Files in this session",
    no_matches: "No matches for this search.",
    no_session_files: "No saved files or recognized writes in this window.",
    no_confirmed_files:
      "No writes confirmed by supported tools. You can show time matches; they do not prove who modified those files.",
    attribution_tool: "Recorded write",
    attribution_temporal: "Unconfirmed attribution",
    file_unprotected: "No backup in session",
    file_missing: "Missing now",
    file_first_backup: "First known backup",
    file_versioned: "Has history",
    no_backup_recovery:
      "The operation was recorded, but no backup was saved during this session. Check exclusions and protect the workspace before working.",
    no_prior_recovery:
      "No backup predates this session. A first backup does not prove the file was created then.",
    tool_evidence:
      "Tools: {tools} \u00b7 {n} operation(s) with successful results.",
    temporal_evidence:
      "This file matches the session interval. Another agent or the user may have edited it.",
    plan_skip_conflict: "Later, shared or unverifiable work: preserved ({n})",
    revert_failures:
      "{n} file(s) could not be restored; review the plan errors.",
    prompt_context: "User time context",
    shared_file_note:
      "This file also has recorded writes in another concurrent session. The saved backup may contain changes from both:",
    agents_heading: "Agent families",
    no_agents_title: "No agent activity yet.",
    no_agents_body:
      "When an AI agent (Claude Code, Gemini, Codex…) works on your machine, its sessions will show up here automatically.",
    agents_tip:
      "Families appear when local data is detected. Auditing recorded writes requires a saved session transcript.",
    agents_files_n: "{n} files",
    agents_sessions_n: "{n} sessions",
    sessions_found: "{n} sessions found",
    no_sessions_title: "No sessions detected.",
    no_sessions_body:
      "This agent has no versioned history yet. Make sure its folder is inside the watched paths.",
    session_select_title: "Select a session",
    session_select_body:
      "Pick a session to review the agent's work, follow its trail of changes and revert what you need.",
    session_untitled: "Session",
    session_kind_cluster: "Grouped activity",
    changes_n: "{n} saved backups",
    change_singular: "change",
    change_plural: "changes",
    session_transcript: "Export session",
    session_revert: "Restore session",
    session_chronology: "Versions saved during the session",
    session_no_changes:
      "The agent didn't change any versioned file in this window. If it worked inside a git repository, git covers those changes, not this trail.",
    session_scope_note:
      "Backups reflect the content observed by unmessai. A time match does not prove who modified a file.",
    stat_modified: "modified",
    stat_created: "without prior backup",
    stat_deleted: "missing now",
    ev_new: "First backup",
    ev_modified: "Modified",
    ev_view_diff: "View changes",
    ev_view_file: "View file",
    ev_open_history: "View history",
    ev_revert_file: "Revert",
    revert_title: "Restore session changes",
    revert_scope_all:
      "Only files with recorded writes in this session will be restored. Temporal matches are excluded; later work is preserved.",
    revert_scope_one: "Scope: {path}",
    revert_plan_loading: "Computing revert plan…",
    revert_none: "Nothing to restore in this scope.",
    plan_revert: "Will be restored to their previous state ({n})",
    plan_restore_deleted: "Deleted files that will be recovered ({n})",
    plan_skip_no_prior: "Without prior backup: preserved ({n})",
    plan_skip_unchanged: "Already at their previous state ({n})",
    plan_failed: "Failures ({n})",
    revert_safety:
      "Before overwriting an existing file, a safety copy is saved. Undo restores those copies; recovered missing files are not deleted again.",
    revert_confirm: "Restore",
    revert_running: "Restoring…",
    revert_done: "Revert completed: {n} file(s) restored.",
    revert_undo: "Undo revert",
    revert_undo_done: "Revert undone: {n} file(s) recovered.",
    transcript_error: "Could not download transcript: {msg}",
    files_heading: "Files",
    no_files_title: "No versioned files yet.",
    no_files_body:
      "unmessai versions your files in the background. As soon as a watched file changes, its history will appear here.",
    storage: "Storage",
    storage_used: "{size} across {files} files",
    select_file_title: "Select a file",
    select_file_body:
      "Pick a file on the left to browse its versions, compare changes and restore.",
    versions: "Versions",
    version_singular: "version",
    version_plural: "versions",
    current_badge: "current",
    deleted_badge: "missing now",
    restore_version: "Restore version",
    untrack: "Stop protecting",
    copy_path: "Copy path",
    path_copied: "Path copied.",
    mode_diff: "Diff",
    mode_content: "Content",
    compare_with: "Compare with",
    compare_current: "On disk (current)",
    no_versions: "No versions.",
    binary_content: "Binary content: cannot be displayed.",
    binary_diff: "Binary content: diff cannot be displayed.",
    load_content_error: "Could not load content.",
    load_diff_error: "Could not load diff.",
    no_previous_version: "No previous version to compare with.",
    no_differences: "No differences between these versions.",
    recent_activity: "Recent activity",
    no_activity: "No activity yet.",
    close: "Close",
    settings_title: "Settings",
    tab_general: "General",
    tab_watch: "Watching",
    tab_ignored: "Ignored",
    tab_retention: "Retention",
    language: "Language",
    language_hint: "Interface language.",
    cfg_prefix: "Backup folder",
    cfg_prefix_hint: "Root where versions are stored (read-only).",
    cfg_port: "UI port",
    cfg_port_hint: "Local port of the interface server.",
    cfg_debounce: "Debounce (seconds)",
    cfg_debounce_hint: "Wait after the last change before versioning.",
    cfg_included: "Watched paths",
    cfg_included_hint: "One per line. ~ is supported.",
    cfg_excluded: "Excluded paths",
    cfg_excluded_hint: "One per line. Not watched.",
    cfg_ignore_patterns: "Ignore patterns",
    cfg_ignore_patterns_hint:
      'One per line, gitignore-style and case-insensitive: ".config" (folder at any level), "**/*.log", "**/cache/**".',
    cfg_exclude_names: "Excluded folder names",
    cfg_exclude_names_hint:
      "One per line; excluded at any level (e.g. node_modules).",
    cfg_gitignore: "Respect .gitignore",
    cfg_gitignore_hint: "Skip whatever git already ignores in each repository.",
    purge_ignored: "Purge ignored from history",
    purge_ignored_hint:
      "Removes already-tracked files matching the ignore patterns from the history.",
    purge_ignored_done: "{count} file(s) forgotten, {size} freed.",
    purge_ignored_none: "No tracked file matches the patterns.",
    retention_max_versions: "Max versions per file",
    retention_max_age: "Max age (days)",
    retention_deleted_age: "Purge deleted after (days)",
    retention_min_keep: "Minimum to keep",
    cancel: "Cancel",
    save: "Save",
    settings_saved: "Settings saved and applied.",
    settings_saved_restart:
      "Settings saved. Restart unmessai to apply the versions folder or the port.",
    flush_now: "Version now",
    flush_done: "{n} pending change(s) versioned.",
    flush_none: "All caught up: no pending changes.",
    flush_error: "Could not version now: {msg}",
    protect_existing: "Protect existing",
    protect_title: "Protect existing files",
    protect_desc:
      "unmessai versions a file the first time it changes, so anything that hasn't changed since installation has no history yet: if something deletes or overwrites it, there would be no previous state to restore. This action saves an initial version of those files.",
    protect_path_label: "Path to protect",
    protect_path_hint:
      "File or folder; ~ is your home folder. Watch filters apply (exclusions, ignores, max size).",
    protect_note:
      "Files that already have history are left untouched: you can run this again anytime.",
    protect_confirm: "Protect",
    protect_running: "Protecting…",
    protect_done:
      "{n} file(s) protected with an initial version ({m} already had history).",
    protect_all_tracked:
      "All protected: the {m} file(s) found already had history.",
    protect_none_matched:
      "No file matches (check the path, filters and exclusions).",
    protect_error: "Could not protect: {msg}",
    about_title: "About unmessai",
    about_desc:
      "unmessai protects your files by versioning them automatically in the background. Local-first: nothing leaves your machine.",
    about_version: "Version",
    website: "unmessai.com",
    about_thanks: "Thanks for supporting the project:",
    restore_title: "Restore version",
    restore_desc: 'You are about to restore "{path}" to version {version}.',
    restore_safety:
      "Before restoring, a safety copy of the current state is saved automatically as a new version, so you never lose data.",
    restore_confirm: "Restore",
    restored_with_safety: "Restored. Safety copy saved: {version}",
    restored_ok: "Restored successfully.",
    untrack_title: "Stop protecting",
    untrack_desc:
      'The whole version history of "{path}" will be deleted. The file on disk is not touched.',
    untrack_note:
      "If the file does not match any ignore pattern or excluded path, it will be versioned again on its next change. Add it to the ignore patterns in Settings → Ignored to stop tracking it for good.",
    untrack_confirm: "Delete history",
    untracked_ok: "History deleted ({size} freed).",
    login_error: "Local sign-in failed: {msg}",
    new_version_saved: "New version saved",
    restore_done: "Restore completed",
  },
};

const languages = [
  { code: "es", label: "Español" },
  { code: "en", label: "English" },
];

const STORAGE_KEY = "unmessai-lang";
let current = null;

function detect() {
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved && translations[saved]) return saved;
  const nav = (navigator.language || "es").slice(0, 2).toLowerCase();
  return translations[nav] ? nav : "es";
}

function lang() {
  if (!current) current = detect();
  return current;
}

function setLang(code) {
  if (!translations[code]) return;
  current = code;
  localStorage.setItem(STORAGE_KEY, code);
  apply();
}

// t traduce una clave, con sustitución de {placeholders}. Si falta en el idioma
// activo cae al español y, en último término, a la propia clave.
function t(key, params) {
  const dict = translations[lang()] || translations.es;
  let s =
    dict[key] != null
      ? dict[key]
      : translations.es[key] != null
        ? translations.es[key]
        : key;
  if (params) {
    for (const k of Object.keys(params)) {
      s = s.split("{" + k + "}").join(String(params[k]));
    }
  }
  return s;
}

// apply vuelca las traducciones sobre los atributos data-i18n del documento:
//   data-i18n              → textContent
//   data-i18n-placeholder  → placeholder
//   data-i18n-aria-label   → aria-label
//   data-i18n-title        → title
// y emite el evento "i18n:changed" para que las vistas repinten su texto dinámico.
function apply() {
  document.documentElement.lang = lang();
  document.title = t("app_title");
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.dataset.i18n);
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
    el.setAttribute("placeholder", t(el.dataset.i18nPlaceholder));
  });
  document.querySelectorAll("[data-i18n-aria-label]").forEach((el) => {
    el.setAttribute("aria-label", t(el.dataset.i18nAriaLabel));
  });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => {
    el.setAttribute("title", t(el.dataset.i18nTitle));
  });
  document.dispatchEvent(
    new CustomEvent("i18n:changed", { detail: { lang: lang() } }),
  );
}

export const I18N = { t, lang, setLang, apply, languages };
export { t };
