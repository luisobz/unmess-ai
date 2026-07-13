# Windows

- Instalador: Inno Setup (`unmessai.iss`). Pasos: compilar los binarios (ver abajo),
  luego `iscc /DAppVersion=<v> packaging\windows\unmessai.iss` → `dist/unmessai-setup-<v>.exe`.
- Instalación **por usuario, sin admin** (`PrivilegesRequired=lowest`).
- El punto de entrada es la app nativa `unmess-app.exe` (icono en la bandeja + ventana
  propia con WebView2 + notificaciones). Los accesos directos de menú Inicio y escritorio
  la abren; el autoarranque es un acceso directo en la carpeta Inicio que la lanza con
  `--background` (la app arranca el daemon `unmessd` si no está en marcha).
- Las exclusiones por defecto las crea el daemon al generar `config.toml` en el primer
  arranque (`%APPDATA%\unmessai\config.toml`).

## Compilar los binarios

`unmessd.exe` y `unmess.exe` son Go puro y se compilan cruzado desde cualquier SO:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
  -o dist/windows-amd64/ ./cmd/unmessd ./cmd/unmess
```

`unmess-app.exe` usa WebView2 (Wails v3) y conviene compilarlo **en Windows** (o un
runner Windows de CI). Antes de compilar, embeber el icono en el `.exe` con
[go-winres](https://github.com/tc-hib/go-winres) (lee `cmd/unmess-app/winres/winres.json`,
que apunta a `build/appicon/icon.ico`):

```sh
go install github.com/tc-hib/go-winres@latest
cd cmd/unmess-app && go-winres make        # genera rsrc_windows_amd64.syso
cd ../.. && go build -tags gui -ldflags="-s -w -H windowsgui" \
  -o dist/windows-amd64/unmess-app.exe ./cmd/unmess-app
```

`-H windowsgui` evita que se abra una consola al lanzar la app.

> Nota de verificación: la app nativa en Windows (icono de bandeja, ventana WebView2,
> notificaciones y el instalador Inno) **no se han podido comprobar** en este entorno
> (desarrollo/CI en Linux). Requiere una máquina o runner Windows.

## Firma y MSI

- MSI: no en v1; el `.exe` de Inno cubre el requisito `.exe/.msi` de la spec.
- Firma Authenticode pendiente — sin ella SmartScreen mostrará aviso; presupuestar
  certificado OV/EV antes del primer release público.
