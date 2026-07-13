// @ts-check
import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";
import fs from "node:fs";
import path from "node:path";

const versionRaw = fs.readFileSync(path.resolve("../.version"), "utf-8").trim();

export default defineConfig({
  site: "https://unmess.ai",
  integrations: [sitemap()],
  build: { inlineStylesheets: "auto" },
  vite: {
    define: {
      __UNMESSAI_VERSION_RAW__: JSON.stringify(versionRaw),
      __UNMESSAI_VERSION__: JSON.stringify(`v${versionRaw}`),
    },
  },
});
