// @ts-check
import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";

// Sitio de marketing estático: output 'static' (por defecto) y sin adaptador.
// No hay islas de framework ni rutas bajo demanda; toda la interactividad
// (tema, idioma, detección de SO) es un <script> vanilla bundleado.
export default defineConfig({
  site: "https://unmess.ai",
  integrations: [sitemap()],
  build: { inlineStylesheets: "auto" },
});
