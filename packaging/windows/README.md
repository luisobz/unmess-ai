# Empaquetado para Windows

## Instalador

- Herramienta: Inno Setup 6.3 o superior (`unmessai.iss`). El script usa
  `SaveStringsToUTF8FileWithoutBOM`, disponible a partir de esa versión.
- Compilación: `iscc /DAppVersion=<versión> packaging\windows\unmessai.iss`.
  El resultado queda en `dist/unmessai-setup-v<versión>.exe`. Requiere los binarios
  ya compilados en `dist\windows-amd64\` (ver más abajo).
- La instalación es por usuario y no requiere administrador
  (`PrivilegesRequired=lowest`).

### Flujo del asistente

1. Selección de idioma (español o inglés).
2. Normas de uso (`normas-es.txt` / `normas-en.txt`, en UTF-8 con BOM para que Inno
   Setup detecte la codificación). El usuario debe aceptarlas para continuar.
3. Configuración básica: carpeta a vigilar y carpeta donde guardar las versiones.
4. Tareas opcionales (acceso directo de escritorio, autoarranque) e instalación.

Al finalizar, el instalador escribe `%APPDATA%\unmessai\config.toml` únicamente con
`prefix` e `included_paths`; el daemon completa el resto de opciones con sus valores
por defecto al cargar. En una actualización sobre una instalación existente, la página
de configuración se omite y el fichero no se modifica. Antes de copiar binarios se
detienen los procesos `unmess-app.exe` y `unmessd.exe` que estén en ejecución.

El punto de entrada del usuario es `unmess-app.exe`: icono en la bandeja del sistema,
ventana propia con WebView2 y notificaciones nativas. Los accesos directos del menú
Inicio y del escritorio la abren; el autoarranque es un acceso directo en la carpeta
Inicio que la lanza con `--background`. La app arranca el daemon `unmessd` si no está
en marcha, sin ventana de consola.

## Compilación de los binarios

Los tres binarios cross-compilan desde cualquier SO. `unmessd.exe` y `unmess.exe` son
Go puro; `unmess-app.exe` (Wails v3) tampoco necesita CGO en Windows, porque el backend
WebView2 funciona mediante syscalls. Solo requiere la etiqueta de build `gui`:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
  -o dist/windows-amd64/ ./cmd/unmessd ./cmd/unmess

# Icono y manifiesto embebidos en el .exe con go-winres
# (lee cmd/unmess-app/winres/winres.json, que apunta a build/appicon/icon.ico):
go install github.com/tc-hib/go-winres@latest
(cd cmd/unmess-app && go-winres make)     # genera rsrc_windows_amd64.syso

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags gui \
  -ldflags="-s -w -H windowsgui" -o dist/windows-amd64/unmess-app.exe ./cmd/unmess-app
```

`-H windowsgui` marca el ejecutable como aplicación de ventanas, sin consola asociada.
Compilar `./cmd/...` sin `-tags gui` produce un stub de `unmess-app` que imprime un
aviso y termina; ese stub existe para que la compilación cruzada del resto del proyecto
no dependa de las librerías de WebView.

En CI, el job `build` de `release.yml` compila los tres ejecutables desde Linux y el
job `installer-windows` genera el instalador en un runner de Windows, donde `iscc`
viene preinstalado.

## Compilar el instalador sin Windows

Para pruebas locales desde Linux puede usarse Inno Setup a través de Wine con la
imagen Docker `amake/innosetup`:

```sh
docker run --rm -v "$PWD:/work" amake/innosetup /DAppVersion=<versión> packaging/windows/unmessai.iss
```

## Firma de código y formato MSI

- MSI: descartado en v1. El instalador de Inno Setup cubre el requisito de la
  especificación.
- Firma Authenticode: pendiente. Sin ella, SmartScreen muestra un aviso al ejecutar el
  instalador y, en equipos Windows 11 con Smart App Control en modo estricto, el
  ejecutable puede quedar bloqueado sin opción de continuar. Conviene presupuestar un
  certificado OV o EV antes del primer release público.
