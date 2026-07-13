; Inno Setup — instalador de unmessai para Windows.
; Compilar con: iscc /DAppVersion=0.1.0 packaging\windows\unmessai.iss
; Requiere los binarios ya cross-compilados en dist\windows-amd64\.

#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif

[Setup]
AppId={{7E1B2C77-1B7E-4A5C-9A57-unmessai0001}
AppName=unmessai
AppVersion={#AppVersion}
AppPublisher=unmessai
DefaultDirName={autopf}\unmessai
DefaultGroupName=unmessai
DisableProgramGroupPage=yes
OutputDir=..\..\dist
OutputBaseFilename=unmessai-setup-{#AppVersion}
SetupIconFile=..\..\build\appicon\icon.ico
Compression=lzma2
SolidCompression=yes
; Instalación por usuario: sin privilegios de administrador
; (el watcher corre como el propio usuario).
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible

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
Name: "desktopicon"; Description: "Crear acceso directo en el escritorio"; Flags: unchecked
Name: "autostart"; Description: "Iniciar unmessai al iniciar sesión (recomendado)"

[Run]
; Al terminar la instalación, ofrecer abrir la app.
Filename: "{app}\unmess-app.exe"; Description: "Abrir unmessai"; Flags: nowait postinstall skipifsilent
