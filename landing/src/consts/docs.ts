// Contenido de la página de documentación en ES y EN. Cada sección tiene
// un id (usado como ancla y para la tabla de contenidos), un título y el
// cuerpo en HTML simple (se inyecta con `@html` en Astro).
//
// Los comandos de descarga/actualización se generan a partir de la versión
// publicada (.version del repo) para que siempre apunten a la última release.

import {
  RELEASE_VERSION_RAW,
  RELEASE_ASSETS_BASE,
  RELEASES_URL,
  ASSET_FILES as A,
} from "@/consts/site";

export interface DocSection {
  id: string;
  title: string;
  html: string;
}

export interface DocsPage {
  pageTitle: string;
  subtitle: string;
  tocLabel: string;
  sections: DocSection[];
}

const V = RELEASE_VERSION_RAW; // p. ej. "0.2.0"
const B = RELEASE_ASSETS_BASE; // base de descarga de la release actual

export const DOCS: Record<"es" | "en", DocsPage> = {
  es: {
    pageTitle: "Documentación",
    subtitle: "Protección automática y versionado de tus archivos. Local-first.",
    tocLabel: "Contenido",
    sections: [
      {
        id: "what",
        title: "Qué es unmessai",
        html: `<p>unmessai versiona en segundo plano cada cambio relevante en tus archivos —fuera de tus repositorios git— y te deja restaurar cualquier versión anterior. Protege frente a borrados o modificaciones accidentales, incluidos los provocados por un agente de IA operando sobre tu sistema de archivos. Todo se guarda en tu equipo: sin nube.</p>
<p>Se distribuye en tres piezas: el CLI <code>unmess</code>, el daemon <code>unmessd</code> (vigila y versiona) y la app nativa <code>unmess-app</code> (bandeja del sistema y ventana propia, incluida en el paquete de Linux).</p>`,
      },
      {
        id: "how",
        title: "Cómo funciona",
        html: `<ol>
<li><strong>Vigila</strong> tu carpeta personal con la API nativa del sistema de archivos.</li>
<li><strong>Agrupa</strong> los cambios (debounce) para no versionar cada pulsación.</li>
<li><strong>Guarda</strong> cada versión en tu store local (<code>~/UnmessaiBackups</code>), con un journal de actividad.</li>
<li><strong>Poda</strong> por retención (nº de versiones, antigüedad) para no crecer sin límite.</li>
</ol>
<p>Quedan fuera tus repositorios git (que ya tienen su propio historial), el propio store y las rutas/nombres excluidos en la configuración.</p>`,
      },
      {
        id: "install",
        title: "Instalación",
        html: `<p>Descarga el paquete para tu sistema desde la <a href="/#descargas">página principal</a> o desde la terminal. Los comandos usan la última versión publicada, <code>${V}</code>.</p>
<h3>Linux (Debian / Ubuntu)</h3>
<p>El <code>.deb</code> instala el CLI, el daemon y la app nativa (icono en la bandeja y ventana propia), y configura el autoarranque.</p>
<pre><code>curl -LO ${B}/${A.deb}
sudo apt install ./${A.deb}</code></pre>
<p>La app aparece en el lanzador como <strong>unmessai</strong> y se inicia con la sesión, mostrando su icono en la barra superior. También puedes abrirla con <code>unmess ui</code>.</p>
<h3>Linux (otras distros · binarios)</h3>
<p>Descarga el <code>.tar.gz</code>, extráelo y copia los binarios al <code>PATH</code>. La app con ventana solo se compila para el <code>.deb</code>; con el tarball usas el CLI y la interfaz web local.</p>
<pre><code>curl -LO ${B}/${A.linuxTar}
tar -xzf ${A.linuxTar}
sudo install unmess unmessd /usr/local/bin/</code></pre>
<h3>macOS</h3>
<p>Descarga el <code>.tar.gz</code> de tu chip (Apple Silicon o Intel), extráelo e instala los binarios:</p>
<pre><code>curl -LO ${B}/${A.macArmTar}   # Intel: ${A.macIntelTar}
tar -xzf ${A.macArmTar}
sudo install unmess unmessd /usr/local/bin/</code></pre>
<p>Para vigilar carpetas arbitrarias, macOS pide conceder <strong>Acceso Total al Disco</strong> a <code>unmessd</code> en <em>Ajustes del Sistema → Privacidad y Seguridad</em>. Arranca el daemon con <code>unmess service install</code> y abre la interfaz con <code>unmess ui --browser</code>.</p>
<h3>Windows</h3>
<p>Descarga y ejecuta el instalador <a href="${B}/${A.winSetup}"><code>${A.winSetup}</code></a>. No pide administrador: durante la instalación eliges qué carpeta vigilar y dónde guardar las versiones, y deja la app en la bandeja del sistema (con autoarranque al iniciar sesión).</p>
<p>Alternativa portable, sin instalador: descarga el <code>.zip</code>, descomprímelo (Windows 10/11 traen <code>curl</code> y <code>tar</code>) y ejecuta desde la carpeta <code>windows-amd64\\</code>:</p>
<pre><code>curl -LO ${B}/${A.winZip}
tar -xf ${A.winZip}
windows-amd64\\unmess.exe service install
windows-amd64\\unmess.exe ui --browser</code></pre>
<div class="note"><strong>Nota:</strong> la primera vez, unmessai crea su configuración por defecto y empieza a proteger tu carpeta personal (excluyendo Descargas, Vídeos, Música, <code>node_modules</code>, etc.).</div>`,
      },
      {
        id: "restore",
        title: "Restaurar una versión",
        html: `<p>Restaurar <strong>nunca es destructivo</strong>: antes de sobrescribir, unmessai guarda una copia de seguridad del estado actual como una versión nueva.</p>
<h3>Desde la app</h3>
<p>Elige el fichero, selecciona la versión, compárala (diff) si quieres y pulsa <em>Restaurar versión</em>.</p>
<h3>Desde el CLI</h3>
<pre><code>unmess versions ruta/al/fichero.txt      # lista versiones
unmess diff ruta/al/fichero.txt          # cambios frente a la anterior
unmess restore ruta/al/fichero.txt       # restaura la última versión
unmess restore ruta/al/fichero.txt --version v2026-07-12-11-44.txt</code></pre>`,
      },
      {
        id: "cli",
        title: "CLI de referencia",
        html: `<p>Todos los comandos aceptan la opción global <code>--config &lt;ruta&gt;</code> para usar un <code>config.toml</code> alternativo, y <code>unmess --help</code> imprime el resumen de uso.</p>
<table>
<tr><th>Comando</th><th>Qué hace</th></tr>
<tr><td><code>unmess status</code></td><td>Estado del daemon y del store (ficheros, versiones, tamaño, debounce).</td></tr>
<tr><td><code>unmess ls [texto]</code></td><td>Lista los ficheros versionados. Flags: <code>--modified</code>, <code>--deleted</code> (excluyentes); <code>texto</code> filtra por subcadena de la ruta.</td></tr>
<tr><td><code>unmess versions &lt;ruta&gt;</code></td><td>Versiones de un fichero, con fecha y tamaño.</td></tr>
<tr><td><code>unmess diff &lt;ruta&gt;</code></td><td>Diferencias entre versiones. Flags: <code>--from &lt;v&gt;</code>, <code>--to &lt;v|current&gt;</code>. Por defecto compara la última con la anterior; <code>--to current</code> compara con el fichero en disco.</td></tr>
<tr><td><code>unmess restore &lt;ruta&gt;</code></td><td>Restaura una versión (guardando copia de seguridad). Flags: <code>--version &lt;v&gt;</code>, <code>--yes</code>/<code>-y</code> (sin confirmación).</td></tr>
<tr><td><code>unmess prune</code></td><td>Aplica la retención. Flag: <code>--dry-run</code> (simula sin borrar).</td></tr>
<tr><td><code>unmess config</code></td><td>Muestra o edita la config: <code>config</code>, <code>config path</code>, <code>config get &lt;clave&gt;</code>, <code>config set &lt;clave&gt; &lt;valor&gt;</code>.</td></tr>
<tr><td><code>unmess ui [ruta]</code></td><td>Abre la app nativa (o el fichero indicado). Flag: <code>--browser</code> fuerza la interfaz web.</td></tr>
<tr><td><code>unmess service &lt;acción&gt;</code></td><td>Gestiona el daemon como servicio: <code>install</code>, <code>uninstall</code>, <code>start</code>, <code>stop</code>, <code>status</code>.</td></tr>
</table>
<h3>Ejemplos</h3>
<pre><code># Estado y espacio ocupado por el store
unmess status

# Solo ficheros borrados que aún puedes recuperar
unmess ls --deleted

# Historial de un fichero y comparación con el estado actual en disco
unmess versions notas.md
unmess diff notas.md --to current

# Restaurar una versión concreta sin confirmación
unmess restore notas.md --version v2026-07-12-11-44.txt --yes

# Simular la poda por retención antes de aplicarla
unmess prune --dry-run

# Usar una configuración alternativa
unmess --config /ruta/config.toml status</code></pre>`,
      },
      {
        id: "updates",
        title: "Actualizaciones",
        html: `<p>Para actualizar, descarga la nueva versión desde la <a href="/#descargas">página de descargas</a> o desde <a href="${RELEASES_URL}">GitHub releases</a> y reemplaza los binarios. Tu store (<code>~/UnmessaiBackups</code>) y tu <code>config.toml</code> se conservan intactos.</p>
<pre><code>unmess service stop
# descarga y reemplaza los binarios (.tar.gz / .deb / .zip según tu plataforma)
unmess service start
unmess status                 # confirma que el daemon está activo</code></pre>
<div class="note"><strong>Compatibilidad:</strong> las versiones nuevas leen los stores y configuraciones existentes. Actualizar nunca pierde historial.</div>`,
      },
      {
        id: "server",
        title: "Servidores y headless",
        html: `<p>En un servidor sin escritorio no necesitas la app nativa: bastan el CLI (<code>unmess</code>) y el daemon (<code>unmessd</code>). El daemon vigila y versiona; el CLI consulta y restaura.</p>
<h3>1. Instalar los binarios</h3>
<pre><code>curl -LO ${B}/${A.linuxTar}
tar -xzf ${A.linuxTar}
sudo install unmess unmessd /usr/local/bin/</code></pre>
<h3>2. Arrancar el daemon como servicio</h3>
<p><code>unmess service install</code> registra <code>unmessd</code> como servicio de usuario de systemd (<code>~/.config/systemd/user/unmessai.service</code>) y lo arranca:</p>
<pre><code>unmess service install     # instala y arranca
unmess service status      # activo / inactivo
unmess service stop        # detener
unmess service uninstall   # quitar</code></pre>
<div class="note"><strong>Sesiones headless:</strong> para que el servicio de usuario siga corriendo sin sesión iniciada, habilita el <em>lingering</em> con <code>sudo loginctl enable-linger $USER</code>.</div>
<h3>3. Configurar qué se vigila</h3>
<p>Ajusta las carpetas y la retención por CLI, sin abrir un editor (las listas van separadas por comas):</p>
<pre><code>unmess config set included_paths "/srv/datos,/etc"
unmess config set debounce_seconds 10
unmess config set retention.max_versions 50
unmess config                              # ver la config efectiva</code></pre>
<h3>4. Consultar y restaurar</h3>
<p>Usa los comandos habituales desde el propio servidor (<code>unmess status</code>, <code>unmess ls</code>, <code>unmess versions</code>, <code>unmess restore</code>). Si necesitas la interfaz web, <code>unmess ui --browser</code> imprime la URL local; en un servidor remoto ábrela por un túnel SSH (<code>ssh -L 48111:127.0.0.1:48111 servidor</code>) — nunca la expongas directamente a la red.</p>`,
      },
      {
        id: "config",
        title: "Configuración",
        html: `<p>La configuración vive en <code>config.toml</code> (en <code>~/.config/unmessai/</code> en Linux, <code>%APPDATA%\\unmessai\\</code> en Windows). Puedes editarla desde <em>Ajustes</em> en la app o con <code>unmess config set</code>. Ajustes principales:</p>
<table>
<tr><th>Clave</th><th>Descripción</th></tr>
<tr><td><code>debounce_seconds</code></td><td>Tiempo de agrupación antes de guardar una versión.</td></tr>
<tr><td><code>included_paths</code> / <code>excluded_paths</code></td><td>Qué carpetas vigilar o excluir.</td></tr>
<tr><td><code>exclude_names</code></td><td>Nombres a ignorar (<code>.git</code>, <code>node_modules</code>…).</td></tr>
<tr><td><code>gitignore_aware</code></td><td>Respeta los <code>.gitignore</code> al decidir qué versionar.</td></tr>
<tr><td><code>max_file_size_mb</code></td><td>Tamaño máximo por fichero para versionar.</td></tr>
<tr><td><code>retention</code></td><td>Máximo de versiones/días y mínimo a conservar.</td></tr>
<tr><td><code>ui.port</code></td><td>Puerto de la interfaz local en <code>127.0.0.1</code>.</td></tr>
</table>
<h3>Ejemplo de configuración</h3>
<pre><code># ~/.config/unmessai/config.toml
debounce_seconds = 5
included_paths = ["/home/usuario/proyectos"]
excluded_paths = ["/home/usuario/descargas"]
exclude_names = [".git", "node_modules", "__pycache__"]
max_file_size_mb = 50

[retention]
max_versions = 20
max_days = 90
min_versions = 3</code></pre>
<h3>Consultar y modificar desde el CLI</h3>
<pre><code>unmess config                        # ver toda la configuración
unmess config get debounce_seconds   # leer una clave
unmess config set debounce_seconds 10
unmess config set included_paths "/home/usuario/proyectos,/home/usuario/docs"
unmess config set retention.max_versions 30</code></pre>`,
      },
    ],
  },
  en: {
    pageTitle: "Documentation",
    subtitle: "Automatic protection and versioning for your files. Local-first.",
    tocLabel: "Contents",
    sections: [
      {
        id: "what",
        title: "What unmessai is",
        html: `<p>unmessai versions every meaningful change to your files in the background —outside your git repositories— and lets you restore any earlier version. It protects against accidental deletions or edits, including those made by an AI agent operating on your file system. Everything stays on your machine: no cloud.</p>
<p>It ships as three pieces: the <code>unmess</code> CLI, the <code>unmessd</code> daemon (watches and versions), and the <code>unmess-app</code> native app (system tray and its own window, included in the Linux package).</p>`,
      },
      {
        id: "how",
        title: "How it works",
        html: `<ol>
<li><strong>Watches</strong> your home folder using the OS's native file-system API.</li>
<li><strong>Coalesces</strong> changes (debounce) so it doesn't version every keystroke.</li>
<li><strong>Saves</strong> each version to your local store (<code>~/UnmessaiBackups</code>), with an activity journal.</li>
<li><strong>Prunes</strong> by retention (number of versions, age) so it doesn't grow unbounded.</li>
</ol>
<p>Excluded: your git repositories (which already keep their own history), the store itself, and any paths/names excluded in the configuration.</p>`,
      },
      {
        id: "install",
        title: "Installation",
        html: `<p>Download the package for your system from the <a href="/#descargas">home page</a> or from the terminal. The commands use the latest published version, <code>${V}</code>.</p>
<h3>Linux (Debian / Ubuntu)</h3>
<p>The <code>.deb</code> installs the CLI, the daemon and the native app (tray icon and its own window), and sets up autostart.</p>
<pre><code>curl -LO ${B}/${A.deb}
sudo apt install ./${A.deb}</code></pre>
<p>The app appears in the launcher as <strong>unmessai</strong> and starts with your session, showing its icon in the top bar. You can also open it with <code>unmess ui</code>.</p>
<h3>Linux (other distros · binaries)</h3>
<p>Download the <code>.tar.gz</code>, extract it and copy the binaries onto your <code>PATH</code>. The windowed app is only built for the <code>.deb</code>; with the tarball you use the CLI and the local web interface.</p>
<pre><code>curl -LO ${B}/${A.linuxTar}
tar -xzf ${A.linuxTar}
sudo install unmess unmessd /usr/local/bin/</code></pre>
<h3>macOS</h3>
<p>Download the <code>.tar.gz</code> for your chip (Apple Silicon or Intel), extract it and install the binaries:</p>
<pre><code>curl -LO ${B}/${A.macArmTar}   # Intel: ${A.macIntelTar}
tar -xzf ${A.macArmTar}
sudo install unmess unmessd /usr/local/bin/</code></pre>
<p>To watch arbitrary folders, macOS asks you to grant <strong>Full Disk Access</strong> to <code>unmessd</code> in <em>System Settings → Privacy &amp; Security</em>. Start the daemon with <code>unmess service install</code> and open the interface with <code>unmess ui --browser</code>.</p>
<h3>Windows</h3>
<p>Download and run the installer <a href="${B}/${A.winSetup}"><code>${A.winSetup}</code></a>. It doesn't ask for admin rights: during installation you pick which folder to watch and where to store versions, and it leaves the app in the system tray (auto-starting at login).</p>
<p>Portable alternative, no installer: download the <code>.zip</code>, unpack it (Windows 10/11 ship <code>curl</code> and <code>tar</code>) and run from the <code>windows-amd64\\</code> folder:</p>
<pre><code>curl -LO ${B}/${A.winZip}
tar -xf ${A.winZip}
windows-amd64\\unmess.exe service install
windows-amd64\\unmess.exe ui --browser</code></pre>
<div class="note"><strong>Note:</strong> on first run, unmessai creates its default configuration and starts protecting your home folder (excluding Downloads, Videos, Music, <code>node_modules</code>, etc.).</div>`,
      },
      {
        id: "restore",
        title: "Restoring a version",
        html: `<p>Restoring is <strong>never destructive</strong>: before overwriting, unmessai saves a backup of the current state as a new version.</p>
<h3>From the app</h3>
<p>Pick the file, select the version, compare it (diff) if you want, and click <em>Restore version</em>.</p>
<h3>From the CLI</h3>
<pre><code>unmess versions path/to/file.txt      # list versions
unmess diff path/to/file.txt          # changes vs the previous one
unmess restore path/to/file.txt       # restore the latest version
unmess restore path/to/file.txt --version v2026-07-12-11-44.txt</code></pre>`,
      },
      {
        id: "cli",
        title: "CLI reference",
        html: `<p>Every command accepts the global <code>--config &lt;path&gt;</code> option to use an alternative <code>config.toml</code>, and <code>unmess --help</code> prints the usage summary.</p>
<table>
<tr><th>Command</th><th>What it does</th></tr>
<tr><td><code>unmess status</code></td><td>Daemon and store status (files, versions, size, debounce).</td></tr>
<tr><td><code>unmess ls [text]</code></td><td>List versioned files. Flags: <code>--modified</code>, <code>--deleted</code> (mutually exclusive); <code>text</code> filters by path substring.</td></tr>
<tr><td><code>unmess versions &lt;path&gt;</code></td><td>Versions of a file, with date and size.</td></tr>
<tr><td><code>unmess diff &lt;path&gt;</code></td><td>Differences between versions. Flags: <code>--from &lt;v&gt;</code>, <code>--to &lt;v|current&gt;</code>. Defaults to latest vs previous; <code>--to current</code> compares against the file on disk.</td></tr>
<tr><td><code>unmess restore &lt;path&gt;</code></td><td>Restore a version (saving a backup first). Flags: <code>--version &lt;v&gt;</code>, <code>--yes</code>/<code>-y</code> (no confirmation).</td></tr>
<tr><td><code>unmess prune</code></td><td>Apply retention. Flag: <code>--dry-run</code> (simulate without deleting).</td></tr>
<tr><td><code>unmess config</code></td><td>Show or edit the config: <code>config</code>, <code>config path</code>, <code>config get &lt;key&gt;</code>, <code>config set &lt;key&gt; &lt;value&gt;</code>.</td></tr>
<tr><td><code>unmess ui [path]</code></td><td>Open the native app (or the given file). Flag: <code>--browser</code> forces the web interface.</td></tr>
<tr><td><code>unmess service &lt;action&gt;</code></td><td>Manage the daemon as a service: <code>install</code>, <code>uninstall</code>, <code>start</code>, <code>stop</code>, <code>status</code>.</td></tr>
</table>
<h3>Examples</h3>
<pre><code># Status and space used by the store
unmess status

# Only deleted files you can still recover
unmess ls --deleted

# A file's history and a comparison with the current on-disk state
unmess versions notes.md
unmess diff notes.md --to current

# Restore a specific version without confirmation
unmess restore notes.md --version v2026-07-12-11-44.txt --yes

# Simulate retention pruning before applying it
unmess prune --dry-run

# Use an alternative configuration
unmess --config /path/config.toml status</code></pre>`,
      },
      {
        id: "updates",
        title: "Updates",
        html: `<p>To update, download the new version from the <a href="/#descargas">downloads page</a> or from <a href="${RELEASES_URL}">GitHub releases</a> and replace the binaries. Your store (<code>~/UnmessaiBackups</code>) and <code>config.toml</code> stay untouched.</p>
<pre><code>unmess service stop
# download and replace the binaries (.tar.gz / .deb / .zip per your platform)
unmess service start
unmess status                 # confirm the daemon is active</code></pre>
<div class="note"><strong>Compatibility:</strong> newer versions read existing stores and configs. Updating never loses history.</div>`,
      },
      {
        id: "server",
        title: "Servers & headless",
        html: `<p>On a server with no desktop you don't need the native app: the CLI (<code>unmess</code>) and the daemon (<code>unmessd</code>) are enough. The daemon watches and versions; the CLI queries and restores.</p>
<h3>1. Install the binaries</h3>
<pre><code>curl -LO ${B}/${A.linuxTar}
tar -xzf ${A.linuxTar}
sudo install unmess unmessd /usr/local/bin/</code></pre>
<h3>2. Run the daemon as a service</h3>
<p><code>unmess service install</code> registers <code>unmessd</code> as a systemd user service (<code>~/.config/systemd/user/unmessai.service</code>) and starts it:</p>
<pre><code>unmess service install     # install and start
unmess service status      # active / inactive
unmess service stop        # stop
unmess service uninstall   # remove</code></pre>
<div class="note"><strong>Headless sessions:</strong> to keep the user service running without an active login, enable <em>lingering</em> with <code>sudo loginctl enable-linger $USER</code>.</div>
<h3>3. Configure what gets watched</h3>
<p>Set folders and retention from the CLI, no editor needed (lists are comma-separated):</p>
<pre><code>unmess config set included_paths "/srv/data,/etc"
unmess config set debounce_seconds 10
unmess config set retention.max_versions 50
unmess config                              # show the effective config</code></pre>
<h3>4. Query and restore</h3>
<p>Use the usual commands from the server itself (<code>unmess status</code>, <code>unmess ls</code>, <code>unmess versions</code>, <code>unmess restore</code>). If you need the web interface, <code>unmess ui --browser</code> prints the local URL; on a remote server open it over an SSH tunnel (<code>ssh -L 48111:127.0.0.1:48111 server</code>) — never expose it directly to the network.</p>`,
      },
      {
        id: "config",
        title: "Configuration",
        html: `<p>The configuration lives in <code>config.toml</code> (in <code>~/.config/unmessai/</code> on Linux, <code>%APPDATA%\\unmessai\\</code> on Windows). Edit it from <em>Settings</em> in the app or with <code>unmess config set</code>. Main settings:</p>
<table>
<tr><th>Key</th><th>Description</th></tr>
<tr><td><code>debounce_seconds</code></td><td>Coalescing time before a version is saved.</td></tr>
<tr><td><code>included_paths</code> / <code>excluded_paths</code></td><td>Which folders to watch or exclude.</td></tr>
<tr><td><code>exclude_names</code></td><td>Names to ignore (<code>.git</code>, <code>node_modules</code>…).</td></tr>
<tr><td><code>gitignore_aware</code></td><td>Respect <code>.gitignore</code> files when deciding what to version.</td></tr>
<tr><td><code>max_file_size_mb</code></td><td>Maximum per-file size to version.</td></tr>
<tr><td><code>retention</code></td><td>Max versions/days and minimum to keep.</td></tr>
<tr><td><code>ui.port</code></td><td>Port of the local interface on <code>127.0.0.1</code>.</td></tr>
</table>
<h3>Example configuration</h3>
<pre><code># ~/.config/unmessai/config.toml
debounce_seconds = 5
included_paths = ["/home/user/projects"]
excluded_paths = ["/home/user/downloads"]
exclude_names = [".git", "node_modules", "__pycache__"]
max_file_size_mb = 50

[retention]
max_versions = 20
max_days = 90
min_versions = 3</code></pre>
<h3>Query and modify from the CLI</h3>
<pre><code>unmess config                        # show the full configuration
unmess config get debounce_seconds   # read a single key
unmess config set debounce_seconds 10
unmess config set included_paths "/home/user/projects,/home/user/docs"
unmess config set retention.max_versions 30</code></pre>`,
      },
    ],
  },
};
