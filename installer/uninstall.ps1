#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Uninstalls Vylos Family Control after verifying Parent Authorization password.
#>
param(
    [Parameter(Mandatory = $true)]
    [string]$ParentPassword,
    [string]$InstallDir = "$env:ProgramFiles\Vylos"
)

$ErrorActionPreference = "Stop"

$ExePath = "$InstallDir\Vylos.exe"
if (Test-Path $ExePath) {
    Write-Host "[1/3] Verifying Parent Authorization and stopping VylosService ..."
    & $ExePath --uninstall-service --password $ParentPassword
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Parent authorization failed. Uninstallation aborted."
        exit 1
    }
}

Write-Host "[2/3] Removing Windows Firewall rules ..."
netsh advfirewall firewall delete rule name="Vylos LAN Coordination (TLS)" | Out-Null
netsh advfirewall firewall delete rule name="Vylos LAN Discovery (UDP)" | Out-Null

Write-Host "[3/3] Removing application binaries and shortcuts ..."
Remove-Item -Path "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\Vylos Family Control.lnk" -Force -ErrorAction SilentlyContinue
Remove-Item -Path $InstallDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "Vylos Family Control uninstalled successfully." -ForegroundColor Green
