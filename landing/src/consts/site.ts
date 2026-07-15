// Datos estructurales de la landing (independientes del idioma). Los textos
// traducibles viven en i18n.ts; aquí solo lo que no cambia entre idiomas:
// identidad, tarjetas de descarga y enlaces de apoyo.

import type { Lang } from "@/consts/i18n";

export const SITE = {
  name: "unmessai",
  url: "https://unmessai.com",
} as const;

/** Clave de la etiqueta de descarga por SO dentro de Copy. */
type DownloadCtaKey = "dlLinux" | "dlMac" | "dlWin";
type DownloadAltKey = "dlLinuxAlt" | "dlMacAlt" | "dlWinAlt";

/** Versión publicada inyectada por Vite desde el .version raíz del repo. */
declare const __UNMESSAI_VERSION_RAW__: string;
declare const __UNMESSAI_VERSION__: string;
export const RELEASE_VERSION_RAW: string = __UNMESSAI_VERSION_RAW__;
export const RELEASE_VERSION: string = __UNMESSAI_VERSION__;
const REL = `https://github.com/luisobz/unmess-ai/releases/download/${RELEASE_VERSION}`;
/** Base de descarga de los artefactos de la release actual (sin barra final). */
export const RELEASE_ASSETS_BASE = REL;
export const RELEASES_URL = "https://github.com/luisobz/unmess-ai/releases/latest";

export interface Download {
  os: "linux" | "mac" | "windows";
  name: string;
  fmt: string;
  ctaKey: DownloadCtaKey;
  href: string;
  altKey?: DownloadAltKey;
  altHref?: string;
}

const V = RELEASE_VERSION_RAW;

/** Nombres de fichero de los artefactos publicados para la versión actual.
 *  Coinciden con lo que produce .github/workflows/release.yml. */
export const ASSET_FILES = {
  deb: `unmessai_${V}_amd64.deb`,
  linuxTar: `unmessai-${RELEASE_VERSION}-linux-amd64.tar.gz`,
  macArmTar: `unmessai-${RELEASE_VERSION}-darwin-arm64.tar.gz`,
  macIntelTar: `unmessai-${RELEASE_VERSION}-darwin-amd64.tar.gz`,
  winZip: `unmessai-${RELEASE_VERSION}-windows-amd64.zip`,
  winSetup: `unmessai-setup-${RELEASE_VERSION}.exe`,
} as const;

export const DOWNLOADS: Download[] = [
  {
    os: "linux", name: "Linux", fmt: "Debian / Ubuntu · .deb", ctaKey: "dlLinux",
    href: `${REL}/unmessai_${V}_amd64.deb`,
    altKey: "dlLinuxAlt", altHref: `${REL}/unmessai-v${V}-linux-amd64.tar.gz`,
  },
  {
    os: "mac", name: "macOS", fmt: "Apple silicon & Intel · .tar.gz", ctaKey: "dlMac",
    href: `${REL}/unmessai-v${V}-darwin-arm64.tar.gz`,
    altKey: "dlMacAlt", altHref: `${REL}/unmessai-v${V}-darwin-amd64.tar.gz`,
  },
  {
    os: "windows", name: "Windows", fmt: "Windows 10/11 · instalador .exe", ctaKey: "dlWin",
    href: `${REL}/unmessai-setup-v${V}.exe`,
    altKey: "dlWinAlt", altHref: `${REL}/unmessai-v${V}-windows-amd64.zip`,
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
