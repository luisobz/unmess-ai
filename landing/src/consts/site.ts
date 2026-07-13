// Datos estructurales de la landing (independientes del idioma). Los textos
// traducibles viven en i18n.ts; aquí solo lo que no cambia entre idiomas:
// identidad, tarjetas de descarga y enlaces de apoyo.

import type { Lang } from "@/consts/i18n";

export const SITE = {
  name: "unmessai",
  url: "https://unmess.ai",
} as const;

/** Clave de la etiqueta de descarga por SO dentro de Copy. */
type DownloadCtaKey = "dlLinux" | "dlMac" | "dlWin";
type DownloadAltKey = "dlLinuxAlt" | "dlMacAlt";

/** Versión publicada y base de descarga de los assets en GitHub Releases. */
export const RELEASE_VERSION = "v0.1.0";
const REL = `https://github.com/luisobz/unmess-ai/releases/download/${RELEASE_VERSION}`;
export const RELEASES_URL = "https://github.com/luisobz/unmess-ai/releases/latest";

export interface Download {
  os: "linux" | "mac" | "windows";
  name: string; // nombre del SO, no se traduce
  fmt: string; // formato/compatibilidad, neutro entre idiomas
  ctaKey: DownloadCtaKey;
  href: string;
  /** Enlace secundario opcional (arquitectura alterna / binarios portables). */
  altKey?: DownloadAltKey;
  altHref?: string;
}

export const DOWNLOADS: Download[] = [
  {
    os: "linux", name: "Linux", fmt: "Debian / Ubuntu · .deb", ctaKey: "dlLinux",
    href: `${REL}/unmessai_0.1.0_amd64.deb`,
    altKey: "dlLinuxAlt", altHref: `${REL}/unmessai-v0.1.0-linux-amd64.tar.gz`,
  },
  {
    os: "mac", name: "macOS", fmt: "Apple silicon & Intel · .tar.gz", ctaKey: "dlMac",
    href: `${REL}/unmessai-v0.1.0-darwin-arm64.tar.gz`,
    altKey: "dlMacAlt", altHref: `${REL}/unmessai-v0.1.0-darwin-amd64.tar.gz`,
  },
  {
    os: "windows", name: "Windows", fmt: "Windows 10/11 · .zip", ctaKey: "dlWin",
    href: `${REL}/unmessai-v0.1.0-windows-amd64.zip`,
  },
];

export interface Donation {
  id: "kofi" | "ghsponsors";
  label: string;
  href: string;
}

export const DONATIONS: Donation[] = [
  { id: "kofi", label: "Ko-fi", href: "https://ko-fi.com/luisobz" },
  { id: "ghsponsors", label: "GitHub Sponsors", href: "https://github.com/sponsors/luisobz" },
];

export const LANG_LABELS: Record<Lang, string> = { es: "ES", en: "EN" };
