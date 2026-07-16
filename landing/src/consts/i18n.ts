// Copys de la landing en los dos idiomas. Única fuente de verdad: se usa tanto
// en el render del servidor (idioma por defecto) como en el <script> de cliente
// que cambia de idioma sin recargar (importa este mismo objeto).

export const LANGS = ["es", "en"] as const;
export type Lang = (typeof LANGS)[number];
export const DEFAULT_LANG: Lang = "es";

export interface Copy {
  title: string;
  metaDescription: string;
  skip: string;
  langLabel: string;
  themeLabel: string;
  heroKicker: string;
  heroTitle: string;
  heroLead: string;
  heroCta: string;
  heroDocs: string;
  trust1: string;
  trust2: string;
  trust3: string;
  artAlt: string;
  howEyebrow: string;
  howTitle: string;
  howLead: string;
  step1Title: string;
  step1Text: string;
  step2Title: string;
  step2Text: string;
  step3Title: string;
  step3Text: string;
  dlEyebrow: string;
  dlTitle: string;
  dlLead: string;
  osTag: string;
  dlLinux: string;
  dlMac: string;
  dlWin: string;
  dlLinuxAlt: string;
  dlMacAlt: string;
  dlWinAlt: string;
  dlFoot: string;
  dlAll: string;
  supTitle: string;
  supText: string;
  footDl: string;
  footHow: string;
  footDocs: string;
  license: string;
  privacy: string;
}

export const DICT: Record<Lang, Copy> = {
  es: {
    title: "unmessai: protección automática de tus archivos",
    metaDescription:
      "unmessai: protección automática de tus archivos frente a borrados o modificaciones accidentales, incluso los causados por una IA. Local-first, sin nube.",
    skip: "Saltar al contenido",
    langLabel: "Idioma",
    themeLabel: "Cambiar tema",
    heroKicker: "Local-first · Sin nube",
    heroTitle: "Tus archivos, a prueba de errores.",
    heroLead:
      "Protección automática frente a modificaciones y borrados accidentales, manuales o provocados por la IA.",
    heroCta: "Descargar unmessai",
    heroDocs: "Documentación",
    trust1: "Versiona en segundo plano",
    trust2: "Todo en local",
    trust3: "Busca y restaura en segundos",
    artAlt:
      "Ilustración del historial de versiones de un archivo, con versiones anteriores restaurables",
    howEyebrow: "Cómo funciona",
    howTitle: "Instala, olvídate y deja que proteja en silencio.",
    howLead:
      "Sin flujos que aprender ni copias manuales. Se instala una vez y trabaja en segundo plano.",
    step1Title: "Instala",
    step1Text:
      "Descarga el paquete de tu sistema y deja unmessai corriendo en segundo plano. Arranca contigo al iniciar sesión.",
    step2Title: "Trabaja como siempre",
    step2Text:
      "Cada cambio relevante se guarda como versión en tu store local, sin subir nada a la nube, sin que tengas que hacer nada.",
    step3Title: "Restaura cuando algo se rompa",
    step3Text:
      "¿Un archivo borrado, con conflictos o con cambios pisados por la IA? Recupera cualquier versión anterior. Restaurar nunca es destructivo.",
    dlEyebrow: "Descargas",
    dlTitle: "Elige tu sistema operativo",
    dlLead:
      "Soporte para varios sistemas operativos. Descarga el tuyo.",
    osTag: "Detectado",
    dlLinux: "Descargar .deb",
    dlMac: "Descargar · Apple Silicon",
    dlWin: "Descargar instalador",
    dlLinuxAlt: "Binarios portables (.tar.gz)",
    dlMacAlt: "Mac Intel (x86-64)",
    dlWinAlt: "Binarios portables (.zip)",
    dlFoot: "Versión actual {version}.",
    dlAll: "Todas las descargas y notas de la versión",
    supTitle: "¿Quieres colaborar?",
    supText:
      "unmessai es un servicio gratuito. Si te aporta valor y lo consideras, puedes dejarnos una propina.",
    footDl: "Descargas",
    footHow: "Cómo funciona",
    footDocs: "Documentación",
    license: "Licencia: GPLv3.",
    privacy: " Local-first, sin nube.",
  },
  en: {
    title: "unmessai: automatic protection for your files",
    metaDescription:
      "unmessai: automatic protection for your files against accidental deletions or edits, even those caused by an AI. Local-first, no cloud.",
    skip: "Skip to content",
    langLabel: "Language",
    themeLabel: "Toggle theme",
    heroKicker: "Local-first · No cloud",
    heroTitle: "Your files, mistake-proof.",
    heroLead:
      "Automatic protection against accidental modifications and deletions — whether manual or caused by an AI.",
    heroCta: "Download unmessai",
    heroDocs: "Documentation",
    trust1: "Versions in the background",
    trust2: "All local",
    trust3: "Search and restore in seconds",
    artAlt:
      "Illustration of a file's version history, with earlier versions ready to restore",
    howEyebrow: "How it works",
    howTitle: "Install, forget it, and let it protect you quietly.",
    howLead:
      "No workflows to learn, no manual copies. Install once and it works in the background.",
    step1Title: "Install",
    step1Text:
      "Download the package for your system and let unmessai run in the background. It starts with you at login.",
    step2Title: "Work as usual",
    step2Text:
      "Every meaningful change is saved as a version in your local store, without uploading anything to the cloud, without you doing a thing.",
    step3Title: "Restore when something breaks",
    step3Text:
      "A file deleted, conflicted, or overwritten by an AI? Recover any earlier version. Restoring is never destructive.",
    dlEyebrow: "Downloads",
    dlTitle: "Choose your operating system",
    dlLead:
      "Multi-platform support. Download yours.",
    osTag: "Detected",
    dlLinux: "Download .deb",
    dlMac: "Download · Apple Silicon",
    dlWin: "Download installer",
    dlLinuxAlt: "Portable binaries (.tar.gz)",
    dlMacAlt: "Mac Intel (x86-64)",
    dlWinAlt: "Portable binaries (.zip)",
    dlFoot: "Current version {version}.",
    dlAll: "All downloads and release notes",
    supTitle: "Want to contribute?",
    supText:
      "unmessai is a free service. If it brings you value and you'd like to, you can leave us a tip.",
    footDl: "Downloads",
    footHow: "How it works",
    footDocs: "Documentation",
    license: "License: GPLv3.",
    privacy: " Local-first, no cloud.",
  },
};
