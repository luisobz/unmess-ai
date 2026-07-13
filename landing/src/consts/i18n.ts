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
      "Protección automática frente a borrados y modificaciones accidentales, incluso los que provoca una IA operando sobre tu equipo.",
    heroCta: "Descargar unmessai",
    heroDocs: "Documentación",
    trust1: "Versiona en segundo plano",
    trust2: "Fuera de tus repos git",
    trust3: "Restaura en segundos",
    artAlt:
      "Ilustración del historial de versiones de un archivo, con versiones anteriores restaurables",
    howEyebrow: "Cómo funciona",
    howTitle: "Tres pasos, y a olvidarse.",
    howLead:
      "Sin flujos que aprender ni copias manuales. Se instala una vez y protege en silencio.",
    step1Title: "Instala",
    step1Text:
      "Descarga el paquete de tu sistema y deja unmessai corriendo en segundo plano. Arranca contigo al iniciar sesión.",
    step2Title: "Trabaja como siempre",
    step2Text:
      "Cada cambio relevante se guarda como versión en tu store local, fuera de tus repos git, sin que tengas que hacer nada.",
    step3Title: "Restaura cuando algo se rompa",
    step3Text:
      "¿Un archivo borrado o pisado por una IA? Recupera cualquier versión anterior. Restaurar nunca es destructivo.",
    dlEyebrow: "Descargas",
    dlTitle: "Elige tu sistema operativo",
    dlLead:
      "Destacamos el que parece ser el tuyo. Puedes descargar cualquiera de los demás.",
    osTag: "Detectado",
    dlLinux: "Descargar .deb",
    dlMac: "Descargar · Apple Silicon",
    dlWin: "Descargar .zip",
    dlLinuxAlt: "Binarios portables (.tar.gz)",
    dlMacAlt: "Mac Intel (x86-64)",
    dlFoot: "Versión actual v0.1.0.",
    dlAll: "Todas las descargas y notas de la versión",
    supTitle: "¿Te resulta útil?",
    supText:
      "unmessai es gratuito. Si quieres apoyar su desarrollo, cualquier ayuda es bienvenida, aunque totalmente opcional.",
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
      "Automatic protection against accidental deletions and edits, including those made by an AI operating on your machine.",
    heroCta: "Download unmessai",
    heroDocs: "Documentation",
    trust1: "Versions in the background",
    trust2: "Outside your git repos",
    trust3: "Restore in seconds",
    artAlt:
      "Illustration of a file's version history, with earlier versions ready to restore",
    howEyebrow: "How it works",
    howTitle: "Three steps, then forget it.",
    howLead:
      "No workflow to learn, no manual copies. Install once and it protects you quietly.",
    step1Title: "Install",
    step1Text:
      "Download the package for your system and let unmessai run in the background. It starts with you at login.",
    step2Title: "Work as usual",
    step2Text:
      "Every meaningful change is saved as a version in your local store, outside your git repos, without you doing a thing.",
    step3Title: "Restore when something breaks",
    step3Text:
      "A file deleted or overwritten by an AI? Recover any earlier version. Restoring is never destructive.",
    dlEyebrow: "Downloads",
    dlTitle: "Choose your operating system",
    dlLead:
      "We highlight the one that looks like yours. You can download any of the others.",
    osTag: "Detected",
    dlLinux: "Download .deb",
    dlMac: "Download · Apple Silicon",
    dlWin: "Download .zip",
    dlLinuxAlt: "Portable binaries (.tar.gz)",
    dlMacAlt: "Mac Intel (x86-64)",
    dlFoot: "Current version v0.1.0.",
    dlAll: "All downloads and release notes",
    supTitle: "Finding it useful?",
    supText:
      "unmessai is free. If you'd like to support its development, any help is welcome, though entirely optional.",
    footDl: "Downloads",
    footHow: "How it works",
    footDocs: "Documentation",
    license: "License: GPLv3.",
    privacy: " Local-first, no cloud.",
  },
};
