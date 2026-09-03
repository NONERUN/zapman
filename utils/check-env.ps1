# Check the host before the GUI starts.
# Call this script after the launcher confirms PowerShell 3.0 or newer.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

$rootDir = Split-Path -Parent $PSScriptRoot
$errors = New-Object System.Collections.Generic.List[string]

function Test-Is64BitOs {
    try {
        return [Environment]::Is64BitOperatingSystem
    } catch {
        return $env:PROCESSOR_ARCHITECTURE -ne 'x86' -or $env:PROCESSOR_ARCHITEW6432
    }
}

function Get-Net45Release {
    $path = 'HKLM:\SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full'
    try {
        $item = Get-ItemProperty -LiteralPath $path -Name Release -ErrorAction Stop
        return [int]$item.Release
    } catch {
        return 0
    }
}

function Test-WinForms {
    try {
        Add-Type -AssemblyName System.Windows.Forms -ErrorAction Stop
        return $true
    } catch {
        return $false
    }
}

function Get-OsCaption {
    try {
        $os = Get-CimInstance -ClassName Win32_OperatingSystem -ErrorAction Stop
        return [string]$os.Caption
    } catch {
        try {
            $os = Get-WmiObject -Class Win32_OperatingSystem -ErrorAction Stop
            return [string]$os.Caption
        } catch {
            return ''
        }
    }
}

if (-not (Test-Is64BitOs)) {
    [void]$errors.Add('This program needs a 64-bit Windows system. 32-bit is not supported.')
}

$binDir = Join-Path $rootDir 'bin'
if (-not (Test-Path -LiteralPath $binDir)) {
    [void]$errors.Add('The bin folder is not found. Extract the full Zapret archive first.')
}

$netRelease = Get-Net45Release
# 378389 is .NET Framework 4.5.
if ($netRelease -lt 378389) {
    [void]$errors.Add('.NET Framework 4.5 or newer is not installed. On Windows 7 install .NET 4.5+ and WMF 5.1.')
}

if (-not (Test-WinForms)) {
    [void]$errors.Add('System.Windows.Forms is not available. Install a full .NET Framework desktop runtime.')
}

$installType = ''
try {
    $installType = [string](Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion' -Name InstallationType -ErrorAction Stop).InstallationType
} catch {
    $installType = ''
}
if ($installType -eq 'Server Core') {
    [void]$errors.Add('Windows Server Core is not supported. Use a desktop Windows edition.')
}

if ($errors.Count -gt 0) {
    $caption = Get-OsCaption
    Write-Host 'ERROR: This computer is not ready to run Zapret.' -ForegroundColor Red
    if ($caption) {
        Write-Host ("OS: " + $caption)
    }
    Write-Host ("Windows PowerShell " + $PSVersionTable.PSVersion.ToString())
    Write-Host ''
    foreach ($item in $errors) {
        Write-Host ("- " + $item) -ForegroundColor Yellow
    }
    Write-Host ''
    Write-Host 'Windows 10 LTSC and newer desktop editions need no extra setup.'
    Write-Host 'Windows 7 SP1 x64 needs WMF 5.1 and .NET Framework 4.5 or newer.'
    exit 1
}

exit 0
