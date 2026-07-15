// Cambio de idioma en vivo para la página /docs/. El servidor renderiza
// DEFAULT_LANG y este script lo sobreescribe si el usuario eligió otro.

import { DOCS, type DocsPage } from "@/consts/docs";

const COPY_SVG =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M6 15H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v1"/></svg>';
const CHECK_SVG =
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.5l4.5 4.5L19 7"/></svg>';

// Divide una línea en [código, comentario], donde el comentario empieza en un
// '#' a principio de línea o precedido por un espacio. En estos bloques el '#'
// solo aparece en comentarios, así que la heurística es segura.
function splitComment(line: string): [string, string | null] {
  for (let i = 0; i < line.length; i++) {
    if (line[i] === "#" && (i === 0 || /\s/.test(line[i - 1]))) {
      return [line.slice(0, i), line.slice(i)];
    }
  }
  return [line, null];
}

// Reconstruye el <code> resaltando comentarios sin interpretar HTML: se opera
// siempre sobre texto (createTextNode / textContent), así las entidades del
// contenido original se re-escapan correctamente.
function highlightComments(code: HTMLElement, text: string): void {
  const frag = document.createDocumentFragment();
  const lines = text.split("\n");
  lines.forEach((line, idx) => {
    const [pre, comment] = splitComment(line);
    if (pre) frag.appendChild(document.createTextNode(pre));
    if (comment !== null) {
      const span = document.createElement("span");
      span.className = "c";
      span.textContent = comment;
      frag.appendChild(span);
    }
    if (idx < lines.length - 1) frag.appendChild(document.createTextNode("\n"));
  });
  code.textContent = "";
  code.appendChild(frag);
}

// Envuelve cada <pre> de la documentación en un bloque .code con gutter de
// números de línea y botón de copiar. Idempotente: se salta los ya procesados.
function enhanceCode(root: ParentNode = document): void {
  root.querySelectorAll<HTMLPreElement>(".docs-content pre").forEach((pre) => {
    if (pre.closest(".code")) return; // ya envuelto
    const code = pre.querySelector("code");
    const raw = (code?.textContent ?? pre.textContent ?? "").replace(/\n$/, "");

    const block = document.createElement("div");
    block.className = "code";

    const nums = document.createElement("div");
    nums.className = "code__nums";
    nums.setAttribute("aria-hidden", "true");
    const count = raw.split("\n").length;
    for (let i = 1; i <= count; i++) {
      const s = document.createElement("span");
      s.textContent = String(i);
      nums.appendChild(s);
    }

    const copy = document.createElement("button");
    copy.type = "button";
    copy.className = "code__copy";
    copy.setAttribute("aria-label", "Copiar");
    copy.innerHTML = COPY_SVG;
    let resetTimer: number | undefined;
    copy.addEventListener("click", () => {
      void navigator.clipboard?.writeText(raw).then(() => {
        copy.classList.add("is-copied");
        copy.innerHTML = CHECK_SVG;
        window.clearTimeout(resetTimer);
        resetTimer = window.setTimeout(() => {
          copy.classList.remove("is-copied");
          copy.innerHTML = COPY_SVG;
        }, 1600);
      });
    });

    if (code) highlightComments(code, raw);

    pre.replaceWith(block);
    block.append(nums, pre, copy);
  });
}

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

  // El innerHTML re-inyectado trae <pre> planos: hay que volver a decorarlos.
  enhanceCode();
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
else enhanceCode(); // el HTML del servidor aún no está decorado

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
