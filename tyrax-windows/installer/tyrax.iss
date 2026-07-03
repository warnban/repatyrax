; -----------------------------------------------------------------------------
;  TYRAX - Windows installer (Inno Setup 6)
;
;  Installs the UI + privileged service (machine-wide, Program Files), registers
;  TyraxProtocol as an auto-start LocalSystem service, wires shortcuts and cleans
;  everything up on uninstall (stops engines + removes the service).
;
;  Build:  publish first (build\publish.ps1), then:  iscc installer\tyrax.iss
;  See installer\README.md for signing.
; -----------------------------------------------------------------------------

#define AppName "TYRAX"
#define AppVersion "1.0.17"
#define AppPublisher "TYRAX"
#define AppExe "TYRAX.exe"
#define ServiceExe "TyraxService.exe"
#define ServiceName "TyraxProtocol"
#define ServiceDisplay "TYRAX PROTOCOL"

[Setup]
AppId={{7E5B2C4A-1F3D-4B8E-9A2C-6D0A1B2C3D4E}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName={#AppName}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\src\Tyrax.App\Assets\tyrax.ico
; Close apps holding files in {app} (shared .NET runtime DLLs during upgrade).
CloseApplications=force
CloseApplicationsFilter=*.exe,*.dll
; A privileged service can only be registered by an elevated installer.
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=Output
OutputBaseFilename=TYRAX-Setup-{#AppVersion}

[Languages]
Name: "en"; MessagesFile: "compiler:Default.isl"
Name: "ru"; MessagesFile: "compiler:Languages\Russian.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "autostart"; Description: "Launch TYRAX at Windows startup"; GroupDescription: "Startup:"

[Files]
; Everything the publish step staged (UI + service + shared runtime + engines\).
Source: "..\dist\*"; DestDir: "{app}"; Flags: recursesubdirs createallsubdirs ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExe}"; IconFilename: "{app}\{#AppExe}"
Name: "{group}\Uninstall {#AppName}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; IconFilename: "{app}\{#AppExe}"; Tasks: desktopicon

[Registry]
; Machine-wide autostart of the UNPRIVILEGED UI at user logon (the service starts
; on its own as an auto service). Runs with the logged-in user's token.
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; \
    ValueName: "{#AppName}"; ValueData: """{app}\{#AppExe}"""; Flags: uninsdeletevalue; Tasks: autostart

[Run]
; Launch the UI after install as the ORIGINAL (non-elevated) user, not as admin.
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; \
    Flags: nowait postinstall skipifsilent runasoriginaluser

[Code]
const
  SERVICE_NAME = '{#ServiceName}';

// Runs a command hidden and waits; ignores the exit code (best-effort service ops).
procedure RunHidden(const FileName, Params: string);
var
  ResultCode: Integer;
begin
  Exec(ExpandConstant(FileName), Params, '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;

// Stop UI, engines, and the Windows service BEFORE files are replaced (upgrade path).
procedure StopTyraxProcesses();
var
  I: Integer;
begin
  RunHidden('{sys}\taskkill.exe', '/f /im {#AppExe}');
  RunHidden('{sys}\taskkill.exe', '/f /im xray.exe');
  RunHidden('{sys}\taskkill.exe', '/f /im tun2socks.exe');
  RunHidden('{sys}\sc.exe', 'stop ' + SERVICE_NAME);
  Sleep(1000);
  RunHidden('{sys}\taskkill.exe', '/f /im {#ServiceExe}');
  // .NET self-contained DLLs (clrjit.dll, etc.) need a moment to be released.
  for I := 1 to 4 do
  begin
    Sleep(750);
    RunHidden('{sys}\sc.exe', 'query ' + SERVICE_NAME);
  end;
end;

procedure StopAndDeleteService();
begin
  RunHidden('{sys}\sc.exe', 'stop ' + SERVICE_NAME);
  // Give it a moment to stop before deleting.
  Sleep(1500);
  RunHidden('{sys}\sc.exe', 'delete ' + SERVICE_NAME);
  Sleep(500);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  Result := '';
  NeedsRestart := False;
  StopTyraxProcesses();
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  BinPath: string;
begin
  if CurStep = ssPostInstall then
  begin
    // Clean any prior registration (upgrade / repair), then (re)create + start.
    StopAndDeleteService();

    BinPath := ExpandConstant('{app}\{#ServiceExe}');
    // Note the required spaces after '=' in sc arguments.
    RunHidden('{sys}\sc.exe',
      'create ' + SERVICE_NAME + ' binPath= "' + BinPath + '" start= auto DisplayName= "{#ServiceDisplay}"');
    RunHidden('{sys}\sc.exe',
      'description ' + SERVICE_NAME + ' "TYRAX PROTOCOL - privileged tunnel engine."');
    // Recover automatically if the service ever crashes.
    RunHidden('{sys}\sc.exe',
      'failure ' + SERVICE_NAME + ' reset= 60 actions= restart/5000/restart/5000/restart/5000');
    RunHidden('{sys}\sc.exe', 'start ' + SERVICE_NAME);
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
  begin
    // Stop the UI + engines and remove the service BEFORE files are deleted.
    StopTyraxProcesses();
    StopAndDeleteService();
  end;
end;
