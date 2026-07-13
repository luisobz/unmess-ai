# macOS

## Bundle `.app` (app nativa)

La app nativa (`unmess-app`: icono en la barra de menú + ventana WKWebView +
notificaciones) se distribuye como bundle `unmessai.app`.

1. Compilar el binario **en macOS** (usa WKWebView vía CGO; no se compila cruzado):
   ```sh
   CGO_ENABLED=1 go build -tags gui -o unmess-app ./cmd/unmess-app
   # universal (opcional): compilar arm64 + amd64 y unir con lipo.
   ```
2. Ensamblar el bundle con el icono `.icns` (de `build/appicon/generate.sh`):
   ```sh
   packaging/macos/make-app.sh <version> ./unmess-app ./unmessd ./unmess
   ```
   Genera `dist/unmessai.app` con `Info.plist`, `Contents/MacOS/unmess-app` y
   `Contents/Resources/icon.icns`.
3. Firmar y notarizar el bundle (ver más abajo) antes de distribuir.

> Nota de verificación: el aspecto real del bundle, el icono en el Launchpad y el
> comportamiento de la barra de menú **no se han podido comprobar visualmente**
> (el desarrollo/CI corre en Linux). Requiere revisión en un Mac.

## Distribución (fuera de la App Store)

El App Sandbox de la Mac App Store es incompatible con vigilar rutas arbitrarias del
usuario → distribución con **Developer ID + notarización**, como el resto de herramientas
de backup de terceros.

Pasos de release (requieren un Mac o runner macOS de CI con Xcode CLT):

1. Cross-compilar o compilar nativo (`packaging/README.md`); binarios universales opcionales
   con `lipo -create darwin-amd64/unmessd darwin-arm64/unmessd -output unmessd`.
2. Firmar: `codesign --sign "Developer ID Application: <equipo>" --options runtime --timestamp unmessd unmess`
3. Empaquetar el `.dmg` (p. ej. con `create-dmg`, contenido: los binarios + un instalador
   `install.command` que los copie a `/usr/local/bin` y ejecute `unmess service install`).
4. Notarizar: `xcrun notarytool submit unmessai-<v>.dmg --keychain-profile <perfil> --wait`
   y grapar: `xcrun stapler staple unmessai-<v>.dmg`.

## Autoarranque

LaunchAgent por usuario (`ai.unmess.unmessd.plist`, plantilla en este directorio),
instalado por `unmess service install` en `~/Library/LaunchAgents` — sin root.

## Full Disk Access (obligatorio)

Para vigilar rutas arbitrarias, macOS exige conceder **Acceso Total al Disco** a `unmessd`
(Ajustes del Sistema → Privacidad y Seguridad → Acceso total al disco). El onboarding
guiado en la UI está en el roadmap; mientras tanto, documentarlo en la guía de instalación. Sin FDA el daemon funciona pero solo ve las carpetas con acceso
automático.

## Pendiente / roadmap

- Bundle `.app` con icono en la barra de menú — v1 distribuye binarios CLI
  + UI web; el bundle llegará con la pieza de menu bar.
- Sparkle para autoactualización.
- FSEvents nativo en el watcher (hoy kqueue vía fsnotify; funcional, menos eficiente en
  árboles grandes).
