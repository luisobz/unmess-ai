# unmessai: landing

Sitio de marketing construido con **Astro 7** (`output: static`). Genera HTML estático que
se puede servir en cualquier hosting; toda la interactividad (tema claro/oscuro, idioma
ES/EN, detección del SO del visitante) es un `<script>` vanilla bundleado, sin islas de
framework ni peticiones externas.

## Desarrollo

```sh
npm install
npm run dev      # servidor de desarrollo
npm run build    # genera dist/ (estático)
npm run preview  # sirve dist/ en local
npm run check    # astro check (tipos de .astro)
```

El resultado del build en `dist/` es lo que se despliega (GitHub Pages, Netlify,
Cloudflare Pages, un nginx…). `node_modules/`, `dist/` y `.astro/` están en `.gitignore`.

## Estructura

```
src/
├── consts/       i18n.ts (copys ES/EN tipados) y site.ts (descargas, donaciones)
├── styles/       global.css (tokens de diseño + reset + primitivos compartidos)
├── layouts/      Layout.astro (head/SEO, script anti-FOUC de tema, slots)
├── components/   BrandMark, LangToggle, ThemeToggle, VersionHistoryArt
├── sections/     Header, Hero, HowItWorks, Downloads, Support, SiteFooter
├── scripts/      client.ts (tema, idioma en vivo, detección de SO)
└── pages/        index.astro (compone el layout con las secciones)
public/           favicon.svg (servido tal cual)
```

Los textos viven una sola vez en `src/consts/i18n.ts` y se usan tanto en el render del
servidor (idioma por defecto) como en el cambio de idioma en cliente.

## Placeholders pendientes de rellenar

- Enlaces de descarga por SO: `href="#"`, marcados `data-release="TODO"` en `src/consts/site.ts`.
- Enlaces de donación (PayPal / Ko-fi / GitHub Sponsors): `href="#"` en `src/consts/site.ts`.
- Enlace a documentación (`/docs`, marcado `data-todo="docs"`).
- Licencia del proyecto (texto placeholder "Licencia: pendiente de decidir").
