# Revisión de unmessai — 4 de octubre de 2026

## Resultado

El modo Agente permite recorrer familias, sesiones y archivos agrupados por carpeta.
Cada archivo distingue evidencia de escritura registrada, coincidencia temporal,
disponibilidad de versiones, trabajo compartido y posibilidad de restauración.
Las escrituras registradas sin copia se muestran sin prometer recuperación.

La revisión se ha centrado en los paquetes de almacenamiento, daemon, watcher,
retención, journal, agentes, API, configuración, los puntos de entrada CLI/app,
la interfaz modular y los workflows de pruebas y distribución. No es una garantía
de ausencia de defectos ni una validación de todos los instaladores en sus SO.

## Problemas corregidos

- Inferencia del proyecto por carpeta mayoritaria: se utiliza el directorio del
  transcript cuando existe. Se excluyen datos de aplicación y se identifica el
  ámbito desconocido.
- Sesiones basadas únicamente en horas de backup: lectura de metadatos y eventos
  originales, incluyendo mensajes posteriores a las primeras 500 líneas.
- Atribución falsa de prompts sin timestamp e instrucciones de contexto.
- Operaciones de escritura intentadas o fallidas confundidas con modificaciones:
  se enlazan llamadas a herramientas compatibles con sus resultados correctos.
- Archivos de otras sesiones simultáneas: se muestran enlaces a sus sesiones y
  se omiten de la restauración automática cuando hay evidencia compartida.
- Versiones guardadas más tarde y modificaciones aún pendientes: se conservan.
- «Nuevo» y «borrado» presentados como hechos históricos: se describe la primera
  copia conocida y la ausencia actual, sin deducir autoría ni creación.
- Exportación desde un transcript en vivo diferente al auditado: se descarga la
  copia exacta del store; también se admiten transcripts binarios.
- Respuestas de red antiguas sustituyendo la selección de agente, sesión, fichero
  o diff; la UI descarta peticiones obsoletas y conserva el contexto de los modales.
- Foco fuera de los modales, hashes dañados y rutas con caracteres reservados.
- Desbordamientos de columnas y cabecera en ventanas pequeñas.
- Colisiones de guardados dentro del mismo segundo y entre instancias del Store:
  publicación exclusiva sin sobrescritura y desempate de nanosegundos.
- Pérdida del bit ejecutable al restaurar y destinos con directorios simbólicos.
- Poda de archivos recién borrados con copias antiguas: plazo desde la primera
  ausencia observada, persistida en el store.
- Anuncio de disponibilidad antes de registrar los watches y registros de
  subdirectorios obsoletos tras renombrar/eliminar carpetas.
- Inicio de daemon con configuración incorrecta al usar una configuración propia
  desde CLI/app; se propaga la ruta efectiva.
- Journal cargado entero para obtener su cuenta o últimas entradas: lectura
  incremental y buffer circular; límite de respuesta en la API.
- Configuraciones vacías o retenciones negativas aceptadas y serialización de
  caracteres de control en cadenas TOML.

## Verificación

Las pruebas se ejecutan con archivos sintéticos y directorios temporales.
No se ha instalado ni sustituido el daemon real del usuario ni se han tocado
servicios de hosting o DNS.

- Suite Go: 114 pruebas con detector de carreras, sin fallos; `go vet` correcto.
- Pruebas de regresión de atribución, scopes, transcripts dañados, herramientas
  fallidas, sesiones compartidas, cambios posteriores, exportación binaria,
  colisiones de guardados, permisos y retención de borrados recientes.
- Una prueba Node de enrutado y siete pruebas Playwright con un servidor de API real:
  familias/sesiones/archivos, filtros, diff, ausencia de copias, respuestas tardías,
  idiomas/temas, ventanas de 1024 y 390 px, restaurar/deshacer, foco del modal y
  navegación a otra sesión con un archivo compartido.
- Compilación local de la app Windows, de la app Linux GTK3 y de daemon/CLI
  para macOS ARM64. La ejecución de la app nativa y los instaladores Windows/macOS
  no se han comprobado en escritorios de esos sistemas.
- CI incorpora pruebas con el Chrome instalado en el runner Ubuntu 24.04 y conserva resultados
  y capturas como artefactos. Los workflows de release no se han ejecutado.

## Límites y próximos pasos

La evidencia procede de transcripts editables, no de un registro firmado del SO.
No se infieren modificaciones desde comandos de shell arbitrarios, herramientas
encapsuladas en código de otros runners ni formatos de herramientas no soportados.
No se puede separar automáticamente la contribución de varios agentes a una copia.
La auditoría acota cada transcript a 16 MiB y avisa de lecturas parciales/corruptas.

Para ampliar la cobertura de autoría conviene integrar hooks de los agentes que
registren sesión, operación, ruta y hashes antes/después. Para separar cambios
simultáneos se necesitarían checkpoints por operación o worktrees aislados.

El backup requiere un sistema de archivos con enlaces duros. No conserva todos
los metadatos del archivo (ACL, atributos extendidos). La comprobación de cambios
posteriores es conservadora; no constituye una transacción con procesos externos
que escriban durante la restauración. Las versiones históricas usan la zona horaria
local y no llevan offset en el nombre.

Se conserva como commit independiente el refactor modular que ya estaba preparado
en el índice antes de esta sesión. Las mejoras y las pruebas se aplican sobre esa
base; hay una copia de los diffs iniciales en `.git/` para referencia local.
