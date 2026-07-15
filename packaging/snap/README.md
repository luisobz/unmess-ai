# Snap

Esta carpeta contiene la configuración para empaquetar unmessai como snap y distribuirlo a través de la [Snap Store](https://snapcraft.io/store).

## Configuración inicial

Para publicar unmessai en la Snap Store, es necesario:

### 1. Crear cuenta en Snapcraft

1. Regístrate en [snapcraft.io](https://snapcraft.io) con tu email
2. Valida tu email
3. Dirígete a [https://snapcraft.io/account](https://snapcraft.io/account) para obtener las credenciales

### 2. Solicitar la aplicación en la tienda

1. Ve a [https://snapcraft.io/account/register-snap](https://snapcraft.io/account/register-snap)
2. Rellena el formulario para registrar `unmessai`:
   - **Name**: `unmessai`
   - **Description**: Automatic background file versioning
   - **Website**: https://unmessai.com
   - Acepta las condiciones de distribución

### 3. Exportar credenciales para CI/CD

En tu máquina local:

```bash
snapcraft login
snapcraft export-login credentials.txt
```

Esto genera `credentials.txt` con tus credenciales Snapcraft.

### 4. Configurar el secreto en GitHub

1. Ve a tu repositorio en GitHub
2. Abre **Settings → Secrets and variables → Actions**
3. Crea un nuevo secreto llamado `SNAPCRAFT_STORE_CREDENTIALS`
4. Copia el contenido de `credentials.txt` y pégalo como valor
5. ⚠️ **Importante**: Elimina `credentials.txt` del sistema después de guardarlo en GitHub

## Compilación local (opcional)

Para probar la construcción del snap localmente:

```bash
# Instalar snapcraft (requiere snapd)
sudo snap install snapcraft --classic

# Construir el snap
snapcraft

# El archivo .snap se generará en el directorio actual
# Para probarlo:
sudo snap install --dangerous unmessai_*.snap
```

## Estructura del snapcraft.yaml

El `snapcraft.yaml` en la raíz del proyecto define:

- **Binarios**: unmessd (daemon), unmess (CLI), unmess-app (GUI)
- **Dependencias de compilación**: Go, GTK3, WebKit2GTK, etc.
- **Plugs** (permisos): home, network, system-tray, X11/Wayland, etc.
- **Apps**: configuración de lanzadores y daemon

## CI/CD

El workflow `.github/workflows/release.yml` incluye un job `build-snap` que:

1. Detecta los tags `v*` (e.g., `v0.3.2`)
2. Compila el snap usando `snapcore/action-build`
3. Publica automáticamente en la Snap Store usando `snapcore/action-publish`
   (si `SNAPCRAFT_STORE_CREDENTIALS` está configurado)

### Publicación manual

Si necesitas publicar manualmente un snap:

```bash
snapcraft login
snapcraft push unmessai_*.snap --release=stable
```

## Versiones y canales

El workflow publica en el canal `stable` de la Snap Store. Para otros canales:

- `stable`: versiones de producción
- `candidate`: candidatas a producción
- `beta`: versiones beta
- `edge`: desarrollo (no recomendado para usuarios finales)

Puedes cambiar el canal en el workflow editando el parámetro `release` en el step "Publicar en Snap Store".

## Permisos necesarios

El snap declara los siguientes permisos (plugs):

- `home`: acceso a carpetas del usuario
- `network` / `network-bind`: conexiones de red
- `system-tray`: soporte para ícono en bandeja del sistema
- `x11` / `wayland`: interfaces gráficas
- `unity7` / `desktop`: integración con el escritorio
- `gsettings`: acceso a configuración del sistema

Estos permisos se instalan automáticamente al instalar el snap.

## Troubleshooting

### El snap no se compila localmente

Asegúrate de que:

- Tienes instalado `snapcraft`: `sudo snap install snapcraft --classic`
- Ejecutas desde la raíz del repositorio
- Las dependencias de compilación están disponibles: `sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev`

### La publicación falla en CI/CD

- Verifica que `SNAPCRAFT_STORE_CREDENTIALS` está correctamente configurado en GitHub Secrets
- Las credenciales pueden expirar; si eso ocurre, exporta nuevas credenciales y actualiza el secreto

### El snap compilado es muy grande

El snap empaqueta todas las dependencias de runtime (GTK, WebKit, etc.). Esto es normal y necesario para garantizar compatibilidad entre distribuciones Linux.
