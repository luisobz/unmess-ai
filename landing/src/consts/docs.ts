// Contenido de la página de documentación en ES y EN. Cada sección tiene
// un id (usado como ancla y para la tabla de contenidos), un título y el
// cuerpo en HTML simple (se inyecta con `@html` en Astro).

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

export const DOCS: Record<"es" | "en", DocsPage> = {
  es: {
    pageTitle: "Documentación",
    subtitle: "Protección automática y versionado de tus archivos. Local-first.",
    tocLabel: "Contenido",
    sections: [
      {
        id: "que-es",
        title: "Qué es unmessai",
        html: `<p>unmessai versiona en segundo plano cada cambio relevante en tus archivos —fuera de tus repositorios git— y te deja restaurar cualquier versión anterior. Protege frente a borrados o modificaciones accidentales, incluidos los provocados por un agente de IA operando sobre tu sistema de archivos. Todo se guarda en tu equipo: sin nube.</p>`,
      },
      {
        id: "instalacion",
        title: "Instalación",
        html: `<p>Descarga el paquete para tu sistema desde la <a href="/#descargas">página principal</a> y sigue estos pasos.</p>
<h3>Linux (Debian / Ubuntu)</h3>
<pre><code>sudo apt install ./unmessai_0.1.0_amd64.deb</code></pre>
<p>La app aparece en el lanzador como <strong>unmessai</strong> y se inicia con la sesión, mostrando su icono en la barra superior. También puedes abrirla con <code>unmess ui</code>.</p>
<h3>macOS</h3>
<p>Descomprime el <code>.tar.gz</code> (Apple Silicon o Intel según tu Mac) y ejecuta la app. Para vigilar carpetas arbitrarias, macOS pide conceder <strong>Acceso Total al Disco</strong> a unmessai en <em>Ajustes del Sistema → Privacidad y Seguridad</em>.</p>
<h3>Windows</h3>
<p>Descomprime el <code>.zip</code> y ejecuta <code>unmess-app.exe</code>. La app se coloca en la bandeja del sistema y puede iniciarse al arrancar sesión.</p>
<div class="note"><strong>Nota:</strong> la primera vez, unmessai crea su configuración por defecto y empieza a proteger tu carpeta personal (excluyendo Descargas, Vídeos, Música, <code>node_modules</code>, etc.).</div>`,
      },
      {
        id: "como-funciona",
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
        id: "app",
        title: "La app nativa",
        html: `<p>unmessai se integra como una aplicación nativa de tu sistema:</p>
<div class="cards">
<div class="card"><h3>Icono en la bandeja</h3><p>Menú para abrir la ventana, pausar/reanudar la protección y salir. El icono cambia a ámbar cuando está en pausa.</p></div>
<div class="card"><h3>Ventana propia</h3><p>La interfaz se abre en su propia ventana (no en el navegador), con búsqueda, historial, diff y restauración.</p></div>
<div class="card"><h3>Notificaciones</h3><p>Avisos del sistema cuando se guarda una versión, se completa una restauración o hay un error.</p></div>
</div>
<p><strong>Pausar</strong> detiene el versionado (por ejemplo, durante una operación masiva de archivos); mientras está en pausa, los cambios no se guardan. Reanudar vuelve a proteger.</p>`,
      },
      {
        id: "restaurar",
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
        html: `<table>
<tr><th>Comando</th><th>Qué hace</th></tr>
<tr><td><code>unmess status</code></td><td>Estado del daemon y del store.</td></tr>
<tr><td><code>unmess ls</code></td><td>Lista los ficheros versionados (<code>--modified</code>, <code>--deleted</code>).</td></tr>
<tr><td><code>unmess versions &lt;ruta&gt;</code></td><td>Versiones de un fichero.</td></tr>
<tr><td><code>unmess diff &lt;ruta&gt;</code></td><td>Diferencias entre versiones.</td></tr>
<tr><td><code>unmess restore &lt;ruta&gt;</code></td><td>Restaura una versión (guardando copia de seguridad).</td></tr>
<tr><td><code>unmess prune</code></td><td>Aplica la retención (<code>--dry-run</code> para simular).</td></tr>
<tr><td><code>unmess config</code></td><td>Muestra o edita la configuración.</td></tr>
<tr><td><code>unmess ui [ruta]</code></td><td>Abre la app nativa (<code>--browser</code> para el navegador).</td></tr>
<tr><td><code>unmess service install</code></td><td>Autoarranque del daemon sin interfaz (servidores).</td></tr>
</table>`,
      },
      {
        id: "config",
        title: "Configuración",
        html: `<p>La configuración vive en <code>config.toml</code> (en <code>~/.config/unmessai/</code> en Linux, <code>%APPDATA%\\unmessai\\</code> en Windows). Puedes editarla desde <em>Ajustes</em> en la app o con <code>unmess config</code>. Ajustes principales:</p>
<table>
<tr><th>Clave</th><th>Descripción</th></tr>
<tr><td><code>debounce_seconds</code></td><td>Tiempo de agrupación antes de guardar una versión.</td></tr>
<tr><td><code>included_paths</code> / <code>excluded_paths</code></td><td>Qué carpetas vigilar o excluir.</td></tr>
<tr><td><code>exclude_names</code></td><td>Nombres a ignorar (<code>.git</code>, <code>node_modules</code>…).</td></tr>
<tr><td><code>retention</code></td><td>Máximo de versiones/días y mínimo a conservar.</td></tr>
</table>`,
      },
      {
        id: "privacidad",
        title: "Privacidad",
        html: `<p>Local-first: tus datos <strong>no salen de tu equipo</strong>. Sin nube salvo que tú lo configures explícitamente. La interfaz se sirve solo en <code>127.0.0.1</code> y nunca se expone a la red.</p>`,
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
        html: `<p>unmessai versions every meaningful change to your files in the background —outside your git repositories— and lets you restore any earlier version. It protects against accidental deletions or edits, including those made by an AI agent operating on your file system. Everything stays on your machine: no cloud.</p>`,
      },
      {
        id: "install",
        title: "Installation",
        html: `<p>Download the package for your system from the <a href="/#descargas">home page</a> and follow these steps.</p>
<h3>Linux (Debian / Ubuntu)</h3>
<pre><code>sudo apt install ./unmessai_0.1.0_amd64.deb</code></pre>
<p>The app appears in the launcher as <strong>unmessai</strong> and starts with your session, showing its icon in the top bar. You can also open it with <code>unmess ui</code>.</p>
<h3>macOS</h3>
<p>Unpack the <code>.tar.gz</code> (Apple Silicon or Intel, matching your Mac) and run the app. To watch arbitrary folders, macOS asks you to grant <strong>Full Disk Access</strong> to unmessai in <em>System Settings → Privacy &amp; Security</em>.</p>
<h3>Windows</h3>
<p>Unzip the <code>.zip</code> and run <code>unmess-app.exe</code>. The app sits in the system tray and can start at login.</p>
<div class="note"><strong>Note:</strong> on first run, unmessai creates its default configuration and starts protecting your home folder (excluding Downloads, Videos, Music, <code>node_modules</code>, etc.).</div>`,
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
        id: "app",
        title: "The native app",
        html: `<p>unmessai integrates as a native application on your system:</p>
<div class="cards">
<div class="card"><h3>Tray icon</h3><p>A menu to open the window, pause/resume protection and quit. The icon turns amber while paused.</p></div>
<div class="card"><h3>Its own window</h3><p>The interface opens in its own window (not the browser), with search, history, diff and restore.</p></div>
<div class="card"><h3>Notifications</h3><p>System alerts when a version is saved, a restore completes, or an error occurs.</p></div>
</div>
<p><strong>Pausing</strong> stops versioning (e.g. during a bulk file operation); while paused, changes are not saved. Resuming protects again.</p>`,
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
        html: `<table>
<tr><th>Command</th><th>What it does</th></tr>
<tr><td><code>unmess status</code></td><td>Daemon and store status.</td></tr>
<tr><td><code>unmess ls</code></td><td>List versioned files (<code>--modified</code>, <code>--deleted</code>).</td></tr>
<tr><td><code>unmess versions &lt;path&gt;</code></td><td>Versions of a file.</td></tr>
<tr><td><code>unmess diff &lt;path&gt;</code></td><td>Differences between versions.</td></tr>
<tr><td><code>unmess restore &lt;path&gt;</code></td><td>Restore a version (saving a backup first).</td></tr>
<tr><td><code>unmess prune</code></td><td>Apply retention (<code>--dry-run</code> to simulate).</td></tr>
<tr><td><code>unmess config</code></td><td>Show or edit the configuration.</td></tr>
<tr><td><code>unmess ui [path]</code></td><td>Open the native app (<code>--browser</code> for the browser).</td></tr>
<tr><td><code>unmess service install</code></td><td>Autostart the headless daemon (servers).</td></tr>
</table>`,
      },
      {
        id: "config",
        title: "Configuration",
        html: `<p>The configuration lives in <code>config.toml</code> (in <code>~/.config/unmessai/</code> on Linux, <code>%APPDATA%\\unmessai\\</code> on Windows). Edit it from <em>Settings</em> in the app or with <code>unmess config</code>. Main settings:</p>
<table>
<tr><th>Key</th><th>Description</th></tr>
<tr><td><code>debounce_seconds</code></td><td>Coalescing time before a version is saved.</td></tr>
<tr><td><code>included_paths</code> / <code>excluded_paths</code></td><td>Which folders to watch or exclude.</td></tr>
<tr><td><code>exclude_names</code></td><td>Names to ignore (<code>.git</code>, <code>node_modules</code>…).</td></tr>
<tr><td><code>retention</code></td><td>Max versions/days and minimum to keep.</td></tr>
</table>`,
      },
      {
        id: "privacy",
        title: "Privacy",
        html: `<p>Local-first: your data <strong>never leaves your machine</strong>. No cloud unless you configure it explicitly. The interface is served only on <code>127.0.0.1</code> and is never exposed to the network.</p>`,
      },
    ],
  },
};
