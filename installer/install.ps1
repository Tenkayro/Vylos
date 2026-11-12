#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Installs Vylos Family Control, configures LocalSubnet Windows Firewall rules, and registers VylosService.
#>
param(
    [string]$InstallDir = "$env:ProgramFiles\Vylos",
    [string]$SourceBinary = "$PSScriptRoot\..\Vylos.exe"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $SourceBinary)) {
    Write-Error "Vylos.exe not found at $SourceBinary. Please build the project first."
    exit 1
}

Write-Host "[1/4] Creating installation directory $InstallDir ..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path $SourceBinary -Destination "$InstallDir\Vylos.exe" -Force

Write-Host "[2/4] Configuring Windows Firewall rules for LocalSubnet only (TLS 41843 & UDP Discovery 41880) ..."
netsh advfirewall firewall delete rule name="Vylos LAN Coordination (TLS)" | Out-Null
netsh advfirewall firewall add rule name="Vylos LAN Coordination (TLS)" dir=in action=allow protocol=TCP localport=41843 remoteip=localsubnet program="$InstallDir\Vylos.exe" profile=private | Out-Null

netsh advfirewall firewall delete rule name="Vylos LAN Discovery (UDP)" | Out-Null
netsh advfirewall firewall add rule name="Vylos LAN Discovery (UDP)" dir=in action=allow protocol=UDP localport=41880 remoteip=localsubnet program="$InstallDir\Vylos.exe" profile=private | Out-Null

Write-Host "[3/4] Registering and hardening VylosService ..."
& "$InstallDir\Vylos.exe" --install-service

Write-Host "[4/4] Creating Start Menu shortcut ..."
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut("$env:ProgramData\Microsoft\Windows\Start Menu\Programs\Vylos Family Control.lnk")
$Shortcut.TargetPath = "$InstallDir\Vylos.exe"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "Vylos Family Control — Local Parental Control"
$Shortcut.Save()

Write-Host "Vylos Family Control installed successfully." -ForegroundColor Green
