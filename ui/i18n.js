"use strict";

// i18n.js — traducciones de la UI de unmessai.
//
// Para añadir un idioma: añade una clave a I18N.translations con el mismo
// conjunto de claves que "es" y una entrada en I18N.languages. Todo lo demás
// (selector de ajustes, atributos data-i18n del HTML) funciona sin tocar nada.
const I18N = (() => {
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
      files_heading: "Archivos",
      no_files_title: "Aún no hay ficheros versionados.",
      no_files_body: "unmessai versiona tus archivos en segundo plano. En cuanto modifiques un fichero vigilado aparecerá aquí su historial.",
      storage: "Almacenamiento",
      storage_used: "{size} en {files} ficheros",
      select_file_title: "Selecciona un fichero",
      select_file_body: "Elige un fichero de la izquierda para ver sus versiones, comparar cambios y restaurar.",
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
      cfg_ignore_patterns_hint: "Uno por línea, estilo gitignore y sin distinguir mayúsculas: \".config\" (carpeta en cualquier nivel), \"**/*.log\", \"**/cache/**\".",
      cfg_exclude_names: "Nombres de carpeta excluidos",
      cfg_exclude_names_hint: "Uno por línea; se excluyen en cualquier nivel (p. ej. node_modules).",
      cfg_gitignore: "Respetar .gitignore",
      cfg_gitignore_hint: "No versionar lo que git ya ignora en cada repositorio.",
      purge_ignored: "Purgar ignorados del historial",
      purge_ignored_hint: "Elimina del historial los ficheros ya trackeados que coinciden con los patrones de ignorado.",
      purge_ignored_done: "{count} fichero(s) olvidados, {size} liberados.",
      purge_ignored_none: "Ningún fichero trackeado coincide con los patrones.",
      retention_max_versions: "Máx. versiones por fichero",
      retention_max_age: "Máx. antigüedad (días)",
      retention_deleted_age: "Purgar borrados tras (días)",
      retention_min_keep: "Mínimo a conservar",
      cancel: "Cancelar",
      save: "Guardar",
      settings_saved: "Ajustes guardados y aplicados.",
      settings_saved_restart: "Ajustes guardados. Reinicia unmessai para aplicar la carpeta de versiones o el puerto.",
      flush_now: "Versionar ahora",
      flush_done: "{n} cambio(s) pendiente(s) versionado(s).",
      flush_none: "Todo al día: no había cambios pendientes.",
      flush_error: "No se pudo versionar ahora: {msg}",
      // Acerca de
      about_title: "Acerca de unmessai",
      about_desc: "unmessai protege tus archivos versionándolos automáticamente en segundo plano. Local-first: nada sale de tu equipo.",
      about_version: "Versión",
      website: "unmessai.com",
      about_thanks: "Gracias por apoyar el proyecto:",
      // Restaurar
      restore_title: "Restaurar versión",
      restore_desc: "Vas a restaurar \"{path}\" a la versión {version}.",
      restore_safety: "Antes de restaurar se guardará automáticamente una copia de seguridad del estado actual como una versión nueva, para que nunca pierdas datos.",
      restore_confirm: "Restaurar",
      restored_with_safety: "Restaurado. Copia de seguridad guardada: {version}",
      restored_ok: "Restaurado correctamente.",
      // Dejar de proteger
      untrack_title: "Dejar de proteger",
      untrack_desc: "Se eliminará todo el historial de versiones de \"{path}\". El fichero en disco no se toca.",
      untrack_note: "Si el fichero no coincide con ningún patrón de ignorado ni ruta excluida, volverá a versionarse en el próximo cambio. Añádelo a los patrones de ignorado en Ajustes → Ignorados para dejar de trackearlo definitivamente.",
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
      files_heading: "Files",
      no_files_title: "No versioned files yet.",
      no_files_body: "unmessai versions your files in the background. As soon as a watched file changes, its history will appear here.",
      storage: "Storage",
      storage_used: "{size} across {files} files",
      select_file_title: "Select a file",
      select_file_body: "Pick a file on the left to browse its versions, compare changes and restore.",
      versions: "Versions",
      version_singular: "version",
      version_plural: "versions",
      current_badge: "current",
      deleted_badge: "deleted",
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
      cfg_ignore_patterns_hint: "One per line, gitignore-style and case-insensitive: \".config\" (folder at any level), \"**/*.log\", \"**/cache/**\".",
      cfg_exclude_names: "Excluded folder names",
      cfg_exclude_names_hint: "One per line; excluded at any level (e.g. node_modules).",
      cfg_gitignore: "Respect .gitignore",
      cfg_gitignore_hint: "Skip whatever git already ignores in each repository.",
      purge_ignored: "Purge ignored from history",
      purge_ignored_hint: "Removes already-tracked files matching the ignore patterns from the history.",
      purge_ignored_done: "{count} file(s) forgotten, {size} freed.",
      purge_ignored_none: "No tracked file matches the patterns.",
      retention_max_versions: "Max versions per file",
      retention_max_age: "Max age (days)",
      retention_deleted_age: "Purge deleted after (days)",
      retention_min_keep: "Minimum to keep",
      cancel: "Cancel",
      save: "Save",
      settings_saved: "Settings saved and applied.",
      settings_saved_restart: "Settings saved. Restart unmessai to apply the versions folder or the port.",
      flush_now: "Version now",
      flush_done: "{n} pending change(s) versioned.",
      flush_none: "All caught up: no pending changes.",
      flush_error: "Could not version now: {msg}",
      about_title: "About unmessai",
      about_desc: "unmessai protects your files by versioning them automatically in the background. Local-first: nothing leaves your machine.",
      about_version: "Version",
      website: "unmessai.com",
      about_thanks: "Thanks for supporting the project:",
      restore_title: "Restore version",
      restore_desc: "You are about to restore \"{path}\" to version {version}.",
      restore_safety: "Before restoring, a safety copy of the current state is saved automatically as a new version, so you never lose data.",
      restore_confirm: "Restore",
      restored_with_safety: "Restored. Safety copy saved: {version}",
      restored_ok: "Restored successfully.",
      untrack_title: "Stop protecting",
      untrack_desc: "The whole version history of \"{path}\" will be deleted. The file on disk is not touched.",
      untrack_note: "If the file does not match any ignore pattern or excluded path, it will be versioned again on its next change. Add it to the ignore patterns in Settings → Ignored to stop tracking it for good.",
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

  // t traduce una clave, con sustitución de {placeholders}.
  function t(key, params) {
    const dict = translations[lang()] || translations.es;
    let s = dict[key] != null ? dict[key] : (translations.es[key] != null ? translations.es[key] : key);
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
    document.dispatchEvent(new CustomEvent("i18n:changed", { detail: { lang: lang() } }));
  }

  return { t, lang, setLang, apply, languages };
})();

const t = I18N.t;
