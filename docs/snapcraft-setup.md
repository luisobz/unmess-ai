# Configuración Snapcraft para GitHub Actions

Este documento explica cómo configurar la publicación automática de unmessai en la Snap Store mediante GitHub Actions.

## Requisitos previos

1. **Cuenta en Snapcraft**: Regístrate en [snapcraft.io](https://snapcraft.io) con tu email.
2. **Snap registrado**: La aplicación `unmessai` debe estar registrada en la Snap Store.
   - Ve a [https://snapcraft.io/account/register-snap](https://snapcraft.io/account/register-snap)
   - Completa el formulario con los detalles de la app

## Paso 1: Generar credenciales locales

En tu máquina local:

```bash
# Instalar snapcraft (si no lo has hecho)
sudo snap install snapcraft --classic

# Autenticarte en Snapcraft
snapcraft login

# Exportar credenciales
snapcraft export-login credentials.txt
```

Esto genera un archivo `credentials.txt` con tus credenciales Snapcraft encriptadas.

## Paso 2: Configurar el secreto en GitHub

1. Ve a tu repositorio en GitHub
2. Abre **Settings** → **Secrets and variables** → **Actions**
3. Haz clic en **New repository secret**
4. Crea un secreto con estos datos:
   - **Name**: `SNAPCRAFT_STORE_CREDENTIALS`
   - **Secret**: Copia el contenido completo de `credentials.txt` (incluyendo el `---`)

5. ⚠️ **Importante**: Después de guardar el secreto en GitHub, elimina `credentials.txt` de tu máquina:
   ```bash
   rm credentials.txt
   ```

## Paso 3: Verificar la configuración

1. Haz un push a una rama de desarrollo
2. En GitHub, abre **Actions** para verificar que el workflow `Release (producto)` aparece
3. Si taggueas una versión con `v*`, el workflow debería:
   - Compilar el snap
   - Publicarlo automáticamente en la Snap Store (si `SNAPCRAFT_STORE_CREDENTIALS` está configurado)

## Canales de publicación

El workflow publica en el canal `stable` de la Snap Store. Para cambiar el canal, edita el step "Publicar en Snap Store" en `.github/workflows/release.yml`:

```yaml
- name: Publicar en Snap Store
  uses: snapcore/action-publish@v1
  with:
    snap: dist/unmessai_*.snap
    release: stable  # cambiar a: candidate, beta, edge
    credentials: ${{ secrets.SNAPCRAFT_STORE_CREDENTIALS }}
```

## Troubleshooting

### Las credenciales expiran

Las credenciales de Snapcraft expiran periódicamente. Si la publicación falla:

```bash
# Generar nuevas credenciales localmente
snapcraft export-login --snapshot credentials.txt

# Actualizar el secreto en GitHub:
# Settings → Secrets and variables → Actions → SNAPCRAFT_STORE_CREDENTIALS → Update
```

### El snap no se compila

- Verifica que el `snapcraft.yaml` esté correctamente configurado
- Los binarios deben compilar sin errores localmente (ver `snapcraft` en la raíz del proyecto)
- Las dependencias de compilación (GTK3, WebKit2GTK) deben estar disponibles

### La publicación falla pero el snap se compiló correctamente

- Las credenciales pueden estar expiradas (ver arriba)
- Verifica que el snap no viola las políticas de la Snap Store
- Consulta los logs del workflow en GitHub Actions para más detalles

## Referencias

- [Snapcraft Documentation](https://snapcraft.io/docs)
- [GitHub Actions para Snapcraft](https://github.com/snapcore/action-build)
- [Snap Store](https://snapcraft.io/store)
