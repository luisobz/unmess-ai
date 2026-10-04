// Package ui contiene el frontend web de unmessai (HTML/CSS/JS modular con ES
// modules nativos, sin build step) embebido en el binario. El servidor de
// internal/api lo sirve en 127.0.0.1:<ui.port>. La directiva go:embed vive aquí
// porque go:embed no puede referenciar rutas con ".." desde internal/api; este
// paquete fino expone los assets como un embed.FS que la API consume.
//
// El código JS vive bajo src/ (core/, features/, components/) y se carga como un
// grafo de módulos desde src/main.js; go:embed incluye el árbol completo.
package ui

import "embed"

// Assets contiene los ficheros estáticos de la UI.
//
//go:embed index.html src styles assets
var Assets embed.FS
