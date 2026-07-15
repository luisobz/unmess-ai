# Configuración GitHub

## Secrets

| Nombre | Para qué |
|---|---|
| `SSH_HOST` | Hostname/IP del servidor |
| `SSH_USER` | Usuario SSH |
| `SSH_KEY` | Clave privada SSH (sin passphrase) |
| `SNAPCRAFT_STORE_CREDENTIALS` | Credenciales para publicar en Snap Store (opcional) |

## Variables

| Nombre | Default | Para qué |
|---|---|---|
| `SSH_PORT` | `22` | Puerto SSH |
| `LANDING_PATH` | — | Ruta absoluta al docroot (ej: `/home/user/unmessai.com/`) |
| `DOWNLOADS_PATH` | — | Ruta donde replicar binarios/.deb (opcional, ej: `/home/user/unmessai.com/dl/`) |

## Workflows

| Workflow | Dispara con | Qué hace |
|---|---|---|
| `ci.yml` | Push / PR | `go vet`, `go test`, cross-compile |
| `deploy-landing.yml` | Tag `landing-v*` / manual | Build Astro → despliega landing al servidor |
| `release.yml` | Tag `v*` / manual | Binarios → GitHub Release + servidor |
