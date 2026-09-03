# Shared Zapret helpers for strategies and the console manager.
# Do not use PowerShell 7 syntax.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

$script:ZapretLocalVersion = '1.10.2'
$script:ZapretRoot = Split-Path -Parent $PSScriptRoot
$script:ZapretBinDir = Join-Path $script:ZapretRoot 'bin'
$script:ZapretListsDir = Join-Path $script:ZapretRoot 'lists'
$script:ZapretUtilsDir = Join-Path $script:ZapretRoot 'utils'
$script:ZapretStrategiesDir = Join-Path $script:ZapretRoot 'strategies'

function Get-ZapretGameFilter {
    $flagFile = Join-Path $script:ZapretUtilsDir 'game_filter.enabled'
    $tcp = '12'
    $udp = '12'
    $status = 'disabled'
    $modeName = 'disabled'
    if (Test-Path -LiteralPath $flagFile) {
        $mode = [string](Get-Content -LiteralPath $flagFile -TotalCount 1 -ErrorAction SilentlyContinue)
        $mode = $mode.Trim().ToLowerInvariant()
        if ($mode -eq 'all') {
            $tcp = '1024-65535'
            $udp = '1024-65535'
            $status = 'enabled (TCP and UDP)'
            $modeName = 'all'
        } elseif ($mode -eq 'tcp') {
            $tcp = '1024-65535'
            $udp = '12'
            $status = 'enabled (TCP)'
            $modeName = 'tcp'
        } else {
            $tcp = '12'
            $udp = '1024-65535'
            $status = 'enabled (UDP)'
            $modeName = 'udp'
        }
    }
    return New-Object PSObject -Property @{
        Tcp    = $tcp
        Udp    = $udp
        Status = $status
        Mode   = $modeName
    }
}

function Initialize-ZapretUserLists {
    if (-not (Test-Path -LiteralPath $script:ZapretListsDir)) {
        New-Item -ItemType Directory -Path $script:ZapretListsDir | Out-Null
    }
    $ipsetExcludeUser = Join-Path $script:ZapretListsDir 'ipset-exclude-user.txt'
    if (-not (Test-Path -LiteralPath $ipsetExcludeUser)) {
        Set-Content -LiteralPath $ipsetExcludeUser -Value '203.0.113.113/32' -Encoding ASCII
    }
    $listGeneralUser = Join-Path $script:ZapretListsDir 'list-general-user.txt'
    if (-not (Test-Path -LiteralPath $listGeneralUser)) {
        Set-Content -LiteralPath $listGeneralUser -Value "# Never leave this file empty`r`ndomain.example.abc" -Encoding ASCII
    }
    $listExcludeUser = Join-Path $script:ZapretListsDir 'list-exclude-user.txt'
    if (-not (Test-Path -LiteralPath $listExcludeUser)) {
        Set-Content -LiteralPath $listExcludeUser -Value 'domain.example.abc' -Encoding ASCII
    }
}

function Enable-ZapretTcpTimestamps {
    $show = netsh interface tcp show global 2>$null | Out-String
    if ($show -match '(?i)timestamps[^\r\n]*enabled') {
        return
    }
    netsh interface tcp set global timestamps=enabled | Out-Null
}

function Test-ZapretServiceRunning {
    $svc = Get-Service -Name 'zapret' -ErrorAction SilentlyContinue
    return [bool]($svc -and $svc.Status -eq 'Running')
}

function Invoke-ZapretStrategyPrep {
    if (Test-ZapretServiceRunning) {
        Write-Host 'The zapret service is already running. Remove the service first if you want to run a strategy.' -ForegroundColor Yellow
        throw 'The zapret service is already running.'
    }
    Enable-ZapretTcpTimestamps
    Initialize-ZapretUserLists
}

function Get-ZapretStrategyFiles {
    if (-not (Test-Path -LiteralPath $script:ZapretStrategiesDir)) {
        return @()
    }
    return @(
        Get-ChildItem -LiteralPath $script:ZapretStrategiesDir -Filter '*.ps1' |
            Sort-Object { [Regex]::Replace($_.Name, '(\d+)', { $args[0].Value.PadLeft(8, '0') }) }
    )
}

function Start-ZapretWinws {
    param([string]$ArgumentList)
    $exe = Join-Path $script:ZapretBinDir 'winws.exe'
    if (-not (Test-Path -LiteralPath $exe)) {
        throw 'winws.exe is not found in the bin folder.'
    }
    Start-Process -FilePath $exe -ArgumentList $ArgumentList -WorkingDirectory $script:ZapretBinDir -WindowStyle Minimized | Out-Null
}

function Start-ZapretStrategyFile {
    param([string]$Path)
    $argList = "-NoProfile -ExecutionPolicy Bypass -File `"$Path`""
    Start-Process -FilePath 'powershell.exe' -ArgumentList $argList -WorkingDirectory $script:ZapretRoot -WindowStyle Minimized | Out-Null
}

function ConvertTo-ZapretServiceImagePath {
    param(
        [string]$CommandLine,
        [string]$ExecutablePath
    )

    if ([string]::IsNullOrWhiteSpace($CommandLine) -and [string]::IsNullOrWhiteSpace($ExecutablePath)) {
        return ''
    }

    $exe = $ExecutablePath
    $arguments = ''
    if ($CommandLine) {
        if ($exe) {
            $quotedExe = '"' + $exe + '"'
            if ($CommandLine.StartsWith($quotedExe)) {
                $arguments = $CommandLine.Substring($quotedExe.Length).Trim()
            } elseif ($CommandLine.StartsWith($exe)) {
                $arguments = $CommandLine.Substring($exe.Length).Trim()
            } elseif ($CommandLine -match '^"([^"]+)"\s*(.*)$') {
                $exe = $matches[1]
                $arguments = $matches[2]
            } else {
                return $CommandLine
            }
        } elseif ($CommandLine -match '^"([^"]+)"\s*(.*)$') {
            $exe = $matches[1]
            $arguments = $matches[2]
        } elseif ($CommandLine -match '^(\S+)\s*(.*)$') {
            $exe = $matches[1]
            $arguments = $matches[2]
        } else {
            return $CommandLine
        }
    }

    if ([string]::IsNullOrWhiteSpace($exe)) {
        return $CommandLine
    }
    if ($arguments) {
        return "`"$exe`" $arguments"
    }
    return "`"$exe`""
}
