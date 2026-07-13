// Mejora progresiva de la landing: cambio de tema, cambio de idioma en vivo y
// detección del SO del visitante. Se bundlea y ejecuta en el navegador (lo
// importa un <script> del layout). El tema inicial ya lo fija un script inline
// en el <head> para evitar parpadeo; aquí solo va el resto.

import { DICT, DEFAULT_LANG, type Lang } from "@/consts/i18n";
import { RELEASE_VERSION } from "@/consts/site";

const root = document.documentElement;

/* ---------- Tema ---------- */
function applyTheme(theme: "light" | "dark"): void {
  root.setAttribute("data-theme", theme);
  try {
    localStorage.setItem("unmessai-theme", theme);
  } catch {
    /* almacenamiento no disponible */
  }
}

const themeToggle = document.getElementById("theme-toggle");
themeToggle?.addEventListener("click", () => {
  applyTheme(root.getAttribute("data-theme") === "dark" ? "light" : "dark");
});

const mql = window.matchMedia?.("(prefers-color-scheme: dark)");
mql?.addEventListener("change", (e) => {
  let stored: string | null = null;
  try {
    stored = localStorage.getItem("unmessai-theme");
  } catch {
    /* ignore */
  }
  if (!stored) applyTheme(e.matches ? "dark" : "light");
});

/* ---------- Idioma ---------- */
function isLang(v: string | null): v is Lang {
  return v === "es" || v === "en";
}

function applyLang(lang: Lang): void {
  const dict = DICT[lang];
  root.setAttribute("lang", lang);

  document.querySelectorAll<HTMLElement>("[data-i18n]").forEach((el) => {
    const key = el.dataset.i18n as keyof typeof dict | undefined;
    if (key && dict[key] != null) {
      el.textContent = (dict[key] as string).replace("{version}", RELEASE_VERSION);
    }
  });

  // Metadatos que dependen del idioma.
  document.title = dict.title;
  document
    .querySelector('meta[name="description"]')
    ?.setAttribute("content", dict.metaDescription);
  document
    .querySelector<HTMLElement>(".skip")
    ?.replaceChildren(document.createTextNode(dict.skip));
  const themeBtn = document.getElementById("theme-toggle");
  themeBtn?.setAttribute("aria-label", dict.themeLabel);

  document
    .querySelectorAll<HTMLButtonElement>("[data-lang-group] [data-lang]")
    .forEach((b) => {
      b.setAttribute("aria-pressed", String(b.dataset.lang === lang));
    });

  try {
    localStorage.setItem("unmessai-lang", lang);
  } catch {
    /* ignore */
  }
}

let savedLang: string | null = null;
try {
  savedLang = localStorage.getItem("unmessai-lang");
} catch {
  /* ignore */
}
// El servidor ya renderizó DEFAULT_LANG; solo reaplicamos si difiere.
const initialLang: Lang = isLang(savedLang) ? savedLang : DEFAULT_LANG;
if (initialLang !== DEFAULT_LANG) applyLang(initialLang);

document
  .querySelectorAll<HTMLButtonElement>("[data-lang-group] [data-lang]")
  .forEach((b) => {
    b.addEventListener("click", () => {
      if (isLang(b.dataset.lang ?? null)) applyLang(b.dataset.lang as Lang);
    });
  });

/* ---------- Detección de SO ---------- */
function detectOS(): "windows" | "mac" | "linux" | null {
  const nav = navigator as Navigator & {
    userAgentData?: { platform?: string };
  };
  // navigator.platform está deprecado; se lee de forma indirecta solo como
  // último recurso tras userAgentData y userAgent.
  const legacyPlatform = (navigator as unknown as Record<string, string>)["platform"];
  const ua = (
    nav.userAgentData?.platform ||
    navigator.userAgent ||
    legacyPlatform ||
    ""
  ).toLowerCase();
  if (ua.includes("win")) return "windows";
  if (ua.includes("mac")) return "mac";
  if (ua.includes("linux") || ua.includes("x11") || ua.includes("android"))
    return "linux";
  return null;
}

const os = detectOS();
if (os) {
  document
    .querySelector<HTMLElement>(`.os-card[data-os="${os}"]`)
    ?.classList.add("is-user");
}
