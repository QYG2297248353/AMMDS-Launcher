; ------------------------------------
; AMMDS Installer Script (FINAL FIXED)
; ------------------------------------
[Setup]
AppName=AMMDS
AppVersion=1.6.32
AppPublisher=新疆萌森软件开发工作室
DefaultDirName={autopf}\ammds
DefaultGroupName=ammds
DisableProgramGroupPage=yes
OutputBaseFilename=ammds-setup
SetupIconFile=icon.ico
Compression=lzma
SolidCompression=yes
WizardStyle=modern
OutputDir=dist
UninstallDisplayName=AMMDS
UninstallDisplayIcon={app}\icon.ico
AppSupportURL=https://ammds.lifebus.top
AppUpdatesURL=https://ammds.lifebus.top
AppPublisherURL=https://ammds.lifebus.top

; 以管理员方式安装
PrivilegesRequired=admin

ArchitecturesInstallIn64BitMode=x64compatible

; ------------------------------------
; 安装前删除
; ------------------------------------
[InstallDelete]
Type: files; Name: "{localappdata}\IconCache.db"

; ------------------------------------
; 安装文件
; ------------------------------------
[Files]
Source: "dist\AMMDS-Launcher.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\ammds.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\icon.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist\logo.ico"; DestDir: "{app}"; Flags: ignoreversion

; ------------------------------------
; 用户可选功能
; ------------------------------------
[Tasks]
Name: "desktopicon"; Description: "创建桌面图标"; Flags: unchecked
Name: "startmenu"; Description: "创建开始菜单项"; Flags: unchecked


; ------------------------------------
; 快捷方式
; ------------------------------------
[Icons]
Name: "{group}\AMMDS"; Filename: "{app}\AMMDS-Launcher.exe"; IconFilename: "{app}\icon.ico"; Tasks: startmenu
Name: "{commondesktop}\AMMDS"; Filename: "{app}\AMMDS-Launcher.exe"; IconFilename: "{app}\logo.ico"; Tasks: desktopicon

; ------------------------------------
; 安装完成界面执行
; ------------------------------------
[Run]
Filename: "{app}\AMMDS-Launcher.exe"; Description: "启动 AMMDS"; Flags: nowait postinstall skipifsilent
Filename: "https://ammds.lifebus.top/"; Description: "打开官方文档"; Flags: shellexec postinstall skipifsilent


; ------------------------------------
; 卸载前强制停止
; ------------------------------------
[Code]
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then begin
    RegDeleteValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'AMMDS Launcher');
  end;
end;

[UninstallRun]
Filename: "{app}\AMMDS-Launcher.exe"; Parameters: "--uninstall"; Flags: runhidden waituntilterminated skipifdoesntexist; RunOnceId: stopApp
Filename: "cmd.exe"; Parameters: "/C taskkill /F /IM AMMDS-Launcher.exe"; Flags: runhidden; RunOnceId: killLauncher
Filename: "cmd.exe"; Parameters: "/C taskkill /F /IM ammds.exe"; Flags: runhidden; RunOnceId: killAMMDS
