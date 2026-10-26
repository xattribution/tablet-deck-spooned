; Tablet Deck installer, for Inno Setup 6. The Release workflow builds it:
;   ISCC /DAppVersion=2.0.0 /DFileVersion=2.0.0.0 /DBuildDir=..\build installer\TabletDeck.iss
; BuildDir holds app\TabletDeck.exe, app\voice.exe and icon.ico.

#ifndef AppVersion
  #error Pass /DAppVersion, /DFileVersion and /DBuildDir (see .github/workflows/release.yml).
#endif

#define AppName "Tablet Deck"
#define AppExe "TabletDeck.exe"
#define RepoUrl "https://github.com/xattribution/tablet-deck-spooned"
#define FirewallRule "Tablet Deck"

[Setup]
; Never change AppId: Windows uses it to recognise upgrades of the same app. (A new id
; from Tablet Deck 1.x, which is a different program.)
AppId={{3E9C2F7A-5B1D-4C8E-9A6F-2D7B8E4C1A90}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher=xattribution
AppPublisherURL={#RepoUrl}
AppSupportURL={#RepoUrl}/issues
AppUpdatesURL={#RepoUrl}/releases
VersionInfoVersion={#FileVersion}
VersionInfoProductName={#AppName}
VersionInfoDescription={#AppName} Setup
SourceDir=..
DefaultDirName={autopf}\{#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=admin
UsedUserAreasWarning=no
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir={#BuildDir}
OutputBaseFilename=TabletDeck-Setup-{#AppVersion}
SetupIconFile={#BuildDir}\icon.ico
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName={#AppName}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
CloseApplications=force
RestartApplications=no
#if FileExists(BuildDir + "\sign.iss")
  #include BuildDir + "\sign.iss"
#endif

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Messages]
WelcomeLabel2=This will install [name/ver].%n%nTablet Deck turns tablets and phones into a touch control deck for Star Citizen, with voice commands, crew sharing and Stream Deck and AI agent control.%n%nFan project. Not affiliated with or endorsed by Cloud Imperium Games.

[Tasks]
Name: "startup"; Description: "Start Tablet Deck with Windows (in the tray)"
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "scprofile"; Description: "Install the Tablet Deck keybind profile into Star Citizen"; GroupDescription: "Star Citizen:"

[Files]
Source: "{#BuildDir}\app\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#BuildDir}\app\voice.exe"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist
Source: "keybinds\TabletDeck-StarCitizen.xml"; DestDir: "{app}\keybinds"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion
Source: "THIRD-PARTY-NOTICES.md"; DestDir: "{app}"; DestName: "THIRD-PARTY-NOTICES.txt"; Flags: ignoreversion
; The copy in the game folder stays on uninstall: the player may have it loaded in-game.
Source: "keybinds\TabletDeck-StarCitizen.xml"; DestDir: "{code:MappingsDir}"; Tasks: scprofile; Check: HasStarCitizen; Flags: ignoreversion uninsneveruninstall

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "TabletDeck"; ValueData: """{app}\{#AppExe}"" --tray"; Tasks: startup; Flags: uninsdeletevalue
Root: HKCU; Subkey: "Software\Classes\tabletdeck"; Flags: uninsdeletekey dontcreatekey

[Run]
; Decks reach the app on private (home) networks only. One program rule covers the deck
; port, the guest port and network discovery. Deleting first keeps upgrades to one rule.
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""{#FirewallRule}"""; Flags: runhidden
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall add rule name=""{#FirewallRule}"" dir=in action=allow program=""{app}\{#AppExe}"" enable=yes profile=private"; Flags: runhidden; StatusMsg: "Allowing Tablet Deck through Windows Firewall on private networks..."
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: postinstall nowait skipifsilent runasoriginaluser
Filename: "{app}\{#AppExe}"; Parameters: "--tray"; Flags: nowait runasoriginaluser; Check: WizardSilent

[UninstallRun]
Filename: "{sys}\taskkill.exe"; Parameters: "/f /im {#AppExe}"; Flags: runhidden; RunOnceId: "StopApp"
Filename: "{sys}\netsh.exe"; Parameters: "advfirewall firewall delete rule name=""{#FirewallRule}"""; Flags: runhidden; RunOnceId: "RemoveFirewallRule"

[Code]
var
  StarCitizenDir: String;

function FindStarCitizen(): String;
var
  Drive, I: Integer;
  Roots: array[0..4] of String;
begin
  Result := '';
  Roots[0] := 'Program Files\Roberts Space Industries';
  Roots[1] := 'Roberts Space Industries';
  Roots[2] := 'RSI';
  Roots[3] := 'Games\Roberts Space Industries';
  Roots[4] := 'Games';
  for Drive := Ord('C') to Ord('Z') do
    for I := 0 to 4 do
      if DirExists(Chr(Drive) + ':\' + Roots[I] + '\StarCitizen\LIVE') then
      begin
        Result := Chr(Drive) + ':\' + Roots[I] + '\StarCitizen\LIVE';
        Exit;
      end;
end;

function InitializeSetup(): Boolean;
begin
  StarCitizenDir := FindStarCitizen();
  Result := True;
end;

function HasStarCitizen(): Boolean;
begin
  Result := StarCitizenDir <> '';
end;

function MappingsDir(Param: String): String;
begin
  if StarCitizenDir <> '' then
    Result := StarCitizenDir + '\user\client\0\Controls\Mappings'
  else
    Result := ExpandConstant('{app}\keybinds');
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Settings: String;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    Settings := ExpandConstant('{userappdata}\TabletDeck');
    if DirExists(Settings) and not UninstallSilent and
       (MsgBox('Also delete your Tablet Deck settings (paired devices, share links, views and layouts)?' + #13#10#13#10 + Settings,
               mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES) then
      DelTree(Settings, True, True, True);
  end;
end;
