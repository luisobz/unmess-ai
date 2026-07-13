// Package ui contiene el frontend web de unmessai (HTML/CSS/JS vanilla, sin
// build step) embebido en el binario. El servidor de internal/api lo sirve en
// 127.0.0.1:<ui.port>. La directiva go:embed vive aquí porque go:embed no puede
// referenciar rutas con ".." desde internal/api; este paquete fino expone los
// assets como un embed.FS que la API consume.
package ui

import "embed"

// Assets contiene los ficheros estáticos de la UI.
//
//go:embed index.html app.js i18n.js styles.css icon.svg
var Assets embed.FS
