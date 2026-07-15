; Inno Setup — instalador de unmessai para Windows.
; Compilar con: iscc /DAppVersion=0.2.0 packaging\windows\unmessai.iss
; Requiere Inno Setup >= 6.3 (usa SaveStringsToUTF8FileWithoutBOM) y los
; binarios ya compilados en dist\windows-amd64\ (unmessd.exe, unmess.exe y
; unmess-app.exe, este último con -tags gui; ver README.md de esta carpeta).
;
; Flujo del asistente: normas de uso (aceptar para continuar) → configuración
; básica (carpeta a vigilar y carpeta de versiones) → tareas → instalar.
; Al terminar escribe %APPDATA%\unmessai\config.toml con esas rutas, de modo
; que el primer arranque del daemon ya vigila lo que el usuario eligió. Si ya
; existe un config.toml (actualización), la página se omite y no se toca nada.

#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif

[Setup]
AppId={{7E1B2C77-1B7E-4A5C-9A57-unmessai0001}
AppName=unmessai
AppVersion={#AppVersion}
; Título de la ventana del instalador (sin "versión X.Y.Z").
AppVerName=unmessai
AppPublisher=unmessai
; Red de seguridad adicional al taskkill de PrepareToInstall: el Restart
; Manager cierra procesos que tengan en uso los ficheros a reemplazar.
CloseApplications=force
RestartApplications=no
DefaultDirName={autopf}\unmessai
DefaultGroupName=unmessai
; Sin esto, "Aplicaciones instaladas" y el desinstalador muestran
; "unmessai versión 0.2.0"; la versión ya va en su columna propia.
UninstallDisplayName=unmessai
DisableProgramGroupPage=yes
; Los binarios van a una ruta fija por usuario; elegir carpeta de instalación
; no aporta nada al flujo (la configuración útil se pide en la página propia).
DisableDirPage=yes
OutputDir=..\..\dist
OutputBaseFilename=unmessai-setup-v{#AppVersion}
SetupIconFile=..\..\build\appicon\icon.ico
UninstallDisplayIcon={app}\unmess-app.exe
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
; Instalación por usuario: sin privilegios de administrador
; (el watcher corre como el propio usuario).
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible

[Languages]
Name: "es"; MessagesFile: "compiler:Languages\Spanish.isl"; LicenseFile: "normas-es.txt"
Name: "en"; MessagesFile: "compiler:Default.isl"; LicenseFile: "normas-en.txt"

[CustomMessages]
es.DirsTitle=Configuración básica
es.DirsSubtitle=Elige qué proteger y dónde guardar las versiones
es.DirsHint=unmessai vigilará la primera carpeta y guardará una copia de cada cambio en la segunda. Podrás afinarlo después en la propia app.
es.WatchDirLabel=Carpeta a vigilar:
es.StoreDirLabel=Carpeta donde guardar las versiones:
es.WatchDirMissing=La carpeta a vigilar no existe. Elige una carpeta existente.
en.DirsTitle=Basic setup
en.DirsSubtitle=Choose what to protect and where to keep versions
en.DirsHint=unmessai will watch the first folder and keep a copy of every change in the second one. You can fine-tune this later in the app.
en.WatchDirLabel=Folder to watch:
en.StoreDirLabel=Folder to store versions in:
en.WatchDirMissing=The folder to watch does not exist. Please choose an existing folder.

[Files]
Source: "..\..\dist\windows-amd64\unmessd.exe";   DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\dist\windows-amd64\unmess.exe";    DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\dist\windows-amd64\unmess-app.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; La app nativa (bandeja + ventana propia) es el punto de entrada.
Name: "{group}\unmessai"; Filename: "{app}\unmess-app.exe"
Name: "{autodesktop}\unmessai"; Filename: "{app}\unmess-app.exe"; Tasks: desktopicon
; Autoarranque idiomático en Windows: acceso directo en la carpeta Inicio que
; lanza la app oculta en la bandeja (ésta arranca el daemon si hace falta).
Name: "{userstartup}\unmessai"; Filename: "{app}\unmess-app.exe"; Parameters: "--background"; Tasks: autostart

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: unchecked
Name: "autostart"; Description: "{cm:AutoStartProgram,unmessai}"

[Run]
; Al terminar la instalación, ofrecer abrir la app.
Filename: "{app}\unmess-app.exe"; Description: "{cm:LaunchProgram,unmessai}"; Flags: nowait postinstall skipifsilent

[Code]
var
  DirsPage: TInputDirWizardPage;

function ConfigPath: String;
begin
  Result := ExpandConstant('{userappdata}\unmessai\config.toml');
end;

procedure InitializeWizard;
begin
  DirsPage := CreateInputDirPage(wpLicense,
    CustomMessage('DirsTitle'), CustomMessage('DirsSubtitle'),
    CustomMessage('DirsHint'), False, '');
  DirsPage.Add(CustomMessage('WatchDirLabel'));
  DirsPage.Add(CustomMessage('StoreDirLabel'));
  DirsPage.Values[0] := ExpandConstant('{%USERPROFILE}');
  DirsPage.Values[1] := ExpandConstant('{%USERPROFILE}\UnmessaiBackups');
end;

// En una actualización ya hay config.toml: no preguntamos ni lo pisamos.
function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := (PageID = DirsPage.ID) and FileExists(ConfigPath);
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if (CurPageID = DirsPage.ID) and not DirExists(DirsPage.Values[0]) then
  begin
    MsgBox(CustomMessage('WatchDirMissing'), mbError, MB_OK);
    Result := False;
  end;
end;

// Escapado de cadenas TOML: primero las barras, luego las comillas.
function TomlEscape(const S: String): String;
begin
  Result := S;
  StringChangeEx(Result, '\', '\\', True);
  StringChangeEx(Result, '"', '\"', True);
end;

// Escribe la configuración inicial con las rutas elegidas. Solo claves que
// difieren de la elección del usuario: el daemon completa el resto con sus
// valores por defecto al cargar (internal/config).
procedure WriteInitialConfig;
var
  Lines: TArrayOfString;
begin
  if FileExists(ConfigPath) then
    Exit;
  if not ForceDirectories(ExtractFileDir(ConfigPath)) then
    Exit;
  SetArrayLength(Lines, 5);
  Lines[0] := '# Configuración de unmessai generada por el instalador.';
  Lines[1] := '# El resto de opciones usan los valores por defecto del daemon';
  Lines[2] := '# (se completan en este fichero al primer arranque).';
  Lines[3] := 'prefix = "' + TomlEscape(DirsPage.Values[1]) + '"';
  Lines[4] := 'included_paths = ["' + TomlEscape(DirsPage.Values[0]) + '"]';
  SaveStringsToUTF8FileWithoutBOM(ConfigPath, Lines, False);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    WriteInitialConfig;
end;

// En actualizaciones, ningún binario puede estar en uso al copiarlos, o el
// reemplazo falla con "Acceso denegado". taskkill admite una sola imagen por
// invocación y, además, vuelve antes de que el proceso muera del todo: por eso
// se repite hasta que devuelve 128 ("no encontrado"), que es la única señal
// fiable de que el proceso ya no está en la tabla y ha liberado sus ficheros.
//
// Códigos de taskkill: 0 = terminado ahora; 128 = no había proceso.
function KillUntilGone(const ImageName: String): Boolean;
var
  R, Tries: Integer;
begin
  Result := True;
  for Tries := 0 to 39 do          // hasta ~10 s (40 × 250 ms)
  begin
    if not Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /T /IM ' + ImageName,
       '', SW_HIDE, ewWaitUntilTerminated, R) then
    begin
      Result := False;             // no se pudo lanzar taskkill
      Exit;
    end;
    if R <> 0 then
      Exit;                        // 128: el proceso ya no existe
    Sleep(250);                    // 0: se acaba de matar; confirmar en la vuelta
  end;
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  Result := '';
  // La app primero y hasta confirmar que murió: si quedara viva, relanzaría el
  // daemon justo después de matarlo y el reemplazo volvería a fallar.
  KillUntilGone('unmess-app.exe');
  KillUntilGone('unmessd.exe');
  // Margen final para que el SO cierre los handles heredados (WebView2, etc.).
  Sleep(700);
end;
