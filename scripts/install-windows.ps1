# RMB Desktop Windows installer (optional convenience).
# Right-click install.ps1 in Explorer and choose "Run with PowerShell", or:
#   powershell -ExecutionPolicy Bypass -File install.ps1
#
# Creates a Start Menu shortcut for rmb-app.exe and (with -LaunchAtLogin)
# registers it to start at login via an HKCU Run key. Extracting the zip and
# running rmb-app.exe directly also works — this script is purely optional.
param(
    [switch]$LaunchAtLogin
)

$ErrorActionPreference = "Stop"
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
$app = Join-Path $dir "rmb-app.exe"

if (-not (Test-Path $app)) {
    Write-Error "rmb-app.exe not found next to install.ps1 (extract the full zip first)"
}

# Start Menu shortcut.
$startMenu = Join-Path ([Environment]::GetFolderPath("Programs")) "RMB Desktop"
New-Item -ItemType Directory -Force -Path $startMenu | Out-Null
$shortcut = Join-Path $startMenu "RMB Desktop.lnk"
$shell = New-Object -ComObject WScript.Shell
$link = $shell.CreateShortcut($shortcut)
$link.TargetPath = $app
$link.WorkingDirectory = $dir
$link.Description = "RMB Desktop - local-first memory for AI coding agents"
$link.Save()
Write-Host "Created $shortcut"

# Optional: launch at login (HKCU only, no admin needed).
if ($LaunchAtLogin) {
    New-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" `
        -Name "RMBDesktop" -Value "`"$app`"" -PropertyType String -Force | Out-Null
    Write-Host "Registered RMB Desktop to start at login."
}

Write-Host "Done. Start RMB Desktop from the Start Menu (or run rmb-app.exe)."
