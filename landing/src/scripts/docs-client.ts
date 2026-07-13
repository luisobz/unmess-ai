// Cambio de idioma en vivo para la página /docs/. El servidor renderiza
// DEFAULT_LANG y este script lo sobreescribe si el usuario eligió otro.

import { DOCS, type DocsPage } from "@/consts/docs";

function applyDocs(lang: "es" | "en"): void {
  const d: DocsPage = DOCS[lang];

  const titleEl = document.getElementById("page-title");
  if (titleEl) titleEl.textContent = d.pageTitle;

  const subtitleEl = document.getElementById("page-subtitle");
  if (subtitleEl) subtitleEl.textContent = d.subtitle;

  const tocLabel = document.getElementById("toc-label");
  if (tocLabel) tocLabel.textContent = d.tocLabel;

  document.querySelectorAll<HTMLElement>("[data-i18n-heading]").forEach((el) => {
    const id = el.dataset.i18nHeading;
    const sec = id && d.sections.find((s) => s.id === id);
    if (sec) el.textContent = sec.title;
  });

  document.querySelectorAll<HTMLElement>("[data-i18n-toc]").forEach((el) => {
    const id = el.dataset.i18nToc;
    const sec = id && d.sections.find((s) => s.id === id);
    if (sec) el.textContent = sec.title;
  });

  document.querySelectorAll<HTMLElement>("[data-i18n-section]").forEach((el) => {
    const id = el.dataset.i18nSection;
    const sec = id && d.sections.find((s) => s.id === id);
    if (sec) el.innerHTML = sec.html;
  });
}

function isLang(v: string | null): v is "es" | "en" {
  return v === "es" || v === "en";
}

let saved: string | null = null;
try {
  saved = localStorage.getItem("unmessai-lang");
} catch { /* ignore */ }

// El servidor ya renderizó DEFAULT_LANG; solo aplicamos si difiere.
const DEFAULT_LANG = "es";
const initial = isLang(saved) ? saved : DEFAULT_LANG;
if (initial !== DEFAULT_LANG) applyDocs(initial);

// Escuchamos cambios de idioma del sistema principal (client.ts emite
// i18n:changed al hacer applyLang). Si no existe, caemos en el listener
// directo de los botones de idioma.
document.addEventListener("i18n:changed", ((e: Event) => {
  const lang = (e as CustomEvent<{ lang: string }>).detail.lang;
  if (isLang(lang)) applyDocs(lang);
}) as EventListener);

// Fallback: escuchamos clicks en el toggle de idioma directamente.
document
  .querySelectorAll<HTMLButtonElement>("[data-lang-group] [data-lang]")
  .forEach((b) => {
    b.addEventListener("click", () => {
      if (isLang(b.dataset.lang ?? null)) applyDocs(b.dataset.lang as "es" | "en");
    });
  });
