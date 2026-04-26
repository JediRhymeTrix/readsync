; ReadSync Inno Setup script (Phase 6).
;
; Builds: bin\readsync-service.exe and bin\readsync-tray.exe must exist
; before running ISCC. The Makefile target `make installer` chains the
; Go build then invokes ISCC.
;
; Output: dist\ReadSync-<version>-setup.exe

#define MyAppName        "ReadSync"
#define MyAppVersion     "0.6.0"
#define MyAppPublisher   "ReadSync Project"
#define MyAppExeName     "readsync-tray.exe"
#define ServiceExeName   "readsync-service.exe"
#define ServiceName      "ReadSync"

[Setup]
AppId={{4E91A7C8-4C3C-4C3C-9F1F-READSYNC0001}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={pf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=ReadSync-{#MyAppVersion}-setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut for {#MyAppName}"; GroupDescription: "Additional icons:"; Flags: unchecked
Name: "openfirewall"; Description: "Allow LAN reader endpoints (KOReader 7200, Moon+ 8765, LAN-only)"; GroupDescription: "Network:"; Flags: unchecked

[Files]
Source: "..\bin\{#ServiceExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\bin\{#MyAppExeName}";   DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md";             DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}";           Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{commondesktop}\{#MyAppName}";   Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
; Self-install the Windows service via kardianos/service.
Filename: "{app}\{#ServiceExeName}"; Parameters: "install"; \
  Flags: runhidden waituntilterminated; \
  StatusMsg: "Registering ReadSync service..."

; Auto-start on boot, restart on failure.
Filename: "{sys}\sc.exe"; Parameters: "config {#ServiceName} start= auto"; \
  Flags: runhidden waituntilterminated
Filename: "{sys}\sc.exe"; \
  Parameters: "failure {#ServiceName} reset= 86400 actions= restart/60000/restart/60000/restart/60000"; \
  Flags: runhidden waituntilterminated

; Start the service.
Filename: "{sys}\sc.exe"; Parameters: "start {#ServiceName}"; \
  Flags: runhidden waituntilterminated; \
  StatusMsg: "Starting ReadSync service..."

; Optional firewall rules (LAN-only, off by default).
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall add rule name=""ReadSync KOReader"" dir=in action=allow protocol=TCP localport=7200 profile=private remoteip=LocalSubnet"; \
  Flags: runhidden waituntilterminated; Tasks: openfirewall
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall add rule name=""ReadSync Moon WebDAV"" dir=in action=allow protocol=TCP localport=8765 profile=private remoteip=LocalSubnet"; \
  Flags: runhidden waituntilterminated; Tasks: openfirewall

; Launch tray app post-install.
Filename: "{app}\{#MyAppExeName}"; Description: "Launch {#MyAppName} tray"; \
  Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{sys}\sc.exe"; Parameters: "stop {#ServiceName}"; Flags: runhidden waituntilterminated
Filename: "{app}\{#ServiceExeName}"; Parameters: "uninstall"; Flags: runhidden waituntilterminated
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall delete rule name=""ReadSync KOReader"""; \
  Flags: runhidden waituntilterminated
Filename: "{sys}\netsh.exe"; \
  Parameters: "advfirewall firewall delete rule name=""ReadSync Moon WebDAV"""; \
  Flags: runhidden waituntilterminated

[Code]
var
  RemoveDataPage: TInputOptionWizardPage;

procedure InitializeWizard();
begin
  RemoveDataPage := CreateInputOptionPage(wpSelectTasks,
    'User data',
    'What should we do with ReadSync user data on uninstall?',
    'ReadSync stores adapter credentials, the SQLite database, and logs in your AppData folder. By default we keep them so you can reinstall without losing state.',
    True, False);
  RemoveDataPage.Add('Keep user data (recommended)');
  RemoveDataPage.Add('Remove all data on uninstall');
  RemoveDataPage.SelectedValueIndex := 0;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  DataDir: string;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    DataDir := ExpandConstant('{userappdata}\ReadSync');
    if (RemoveDataPage <> nil) and (RemoveDataPage.SelectedValueIndex = 1) then
    begin
      if DirExists(DataDir) then
        DelTree(DataDir, True, True, True);
    end;
  end;
end;
