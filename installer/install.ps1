#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Installs FamilyControl, configures LocalSubnet Windows Firewall rules, and registers FamilyControlService.
#>
param(
    [string]$InstallDir = "$env:ProgramFiles\FamilyControl",
    [string]$SourceBinary = "$PSScriptRoot\..\FamilyControl.exe"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $SourceBinary)) {
    Write-Error "FamilyControl.exe not found at $SourceBinary. Please build the project first."
    exit 1
}

Write-Host "[1/4] Creating installation directory $InstallDir ..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path $SourceBinary -Destination "$InstallDir\FamilyControl.exe" -Force

Write-Host "[2/4] Configuring Windows Firewall rules for LocalSubnet only (TLS 41843 & UDP Discovery 41880) ..."
netsh advfirewall firewall delete rule name="FamilyControl LAN Coordination (TLS)" | Out-Null
netsh advfirewall firewall add rule name="FamilyControl LAN Coordination (TLS)" dir=in action=allow protocol=TCP localport=41843 remoteip=localsubnet program="$InstallDir\FamilyControl.exe" profile=private | Out-Null

netsh advfirewall firewall delete rule name="FamilyControl LAN Discovery (UDP)" | Out-Null
netsh advfirewall firewall add rule name="FamilyControl LAN Discovery (UDP)" dir=in action=allow protocol=UDP localport=41880 remoteip=localsubnet program="$InstallDir\FamilyControl.exe" profile=private | Out-Null

Write-Host "[3/4] Registering and hardening FamilyControlService ..."
& "$InstallDir\FamilyControl.exe" --install-service

Write-Host "[4/4] Creating Start Menu shortcut ..."
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut("$env:ProgramData\Microsoft\Windows\Start Menu\Programs\FamilyControl.lnk")
$Shortcut.TargetPath = "$InstallDir\FamilyControl.exe"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "FamilyControl — Local Parental Control"
$Shortcut.Save()

Write-Host "FamilyControl installed successfully." -ForegroundColor Green
