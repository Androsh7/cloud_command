#define VerFile FileOpen(SourcePath + "..\VERSION.txt")
#define AppVersion Trim(FileRead(VerFile))
#expr FileClose(VerFile)

[Setup]
AppId={{7FFEB8AC-7A72-4155-B3C9-E262D0358300}}
AppName=Cloud Command
AppVersion={#AppVersion}
AppPublisher=androsh7
AppPublisherURL=https://github.com/androsh7/cloud_command
DefaultDirName={autopf}\Cloud Command
DefaultGroupName=Cloud Command
SetupIconFile=.\icon.ico
UninstallDisplayIcon={app}\cloud-command.exe
OutputDir=..\dist\windows\
OutputBaseFilename=cloud-command-{#AppVersion}-windows-amd64-setup
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
Compression=lzma2
SolidCompression=yes
WizardStyle=modern

[Files]
Source: "..\dist\windows\cloud-command.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Cloud Command"; Filename: "{app}\cloud-command.exe"
