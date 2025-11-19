; ------------------------------------
; AMMDS Installer Script
; ------------------------------------
[Setup]
AppName=ammds
AppVersion=1.6.32
AppPublisher=新疆萌森软件开发工作室
DefaultDirName={pf}\ammds
DefaultGroupName=ammds
DisableProgramGroupPage=yes
OutputBaseFilename=ammds-setup
SetupIconFile=icon.png
Compression=lzma
SolidCompression=yes
WizardStyle=modern

; 是否需要管理员权限
PrivilegesRequired=admin

; ------------------------------------
; 安装文件
; ------------------------------------
[Files]
Source: "AMMDS-Launcher.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "ammds.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "icon.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "icon.png"; DestDir: "{app}"; Flags: ignoreversion

; ------------------------------------
; 用户可选功能
; ------------------------------------
[Tasks]
Name: "desktopicon"; Description: "创建桌面图标"; Flags: unchecked
Name: "startmenu"; Description: "创建开始菜单项"; Flags: unchecked
Name: "autostart"; Description: "开机自动启动"; Flags: unchecked
Name: "runafterinstall"; Description: "安装完成后立即启动 AMMDS"; Flags: unchecked

; ------------------------------------
; 快捷方式
; ------------------------------------
[Icons]
; 开始菜单
Name: "{group}\AMMDS"; Filename: "{app}\AMMDS-Launcher.exe"; IconFilename: "{app}\icon.ico"; Tasks: startmenu

; 桌面图标
Name: "{commondesktop}\AMMDS"; Filename: "{app}\AMMDS-Launcher.exe"; IconFilename: "{app}\icon.ico"; Tasks: desktopicon

; ------------------------------------
; 安装后执行
; ------------------------------------
[Run]
Filename: "{app}\AMMDS-Launcher.exe"; Description: "启动 AMMDS"; Flags: nowait postinstall skipifsilent; Tasks: runafterinstall

; ------------------------------------
; 注册项（用于开机自启）
; ------------------------------------
[Registry]
; 勾选后写入 Run（开机启动项）
Root: HKLM; Subkey: "SOFTWARE\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "ammds"; ValueData: """{app}\AMMDS-Launcher.exe"""; Tasks: autostart

; 卸载时删除开机启动项
Root: HKLM; Subkey: "SOFTWARE\Microsoft\Windows\CurrentVersion\Run"; ValueName: "ammds"; Flags: deletevalue

; ------------------------------------
; 卸载前停止程序
; ------------------------------------
[UninstallRun]
Filename: "{app}\AMMDS-Launcher.exe"; Parameters: "--stop"; Flags: runhidden skipifdoesntexist

