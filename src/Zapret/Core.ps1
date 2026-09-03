# Zapret module: roots, version, user lists, TLS, TCP timestamps.

Set-StrictMode -Version Latest

$script:ZapretLocalVersion = '1.10.2'
# This file is in src\Zapret. The package root is two levels up.
$script:ZapretRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$script:ZapretUtilsDir = Join-Path $script:ZapretRoot 'src\utils'
$script:ZapretBinDir = Join-Path $script:ZapretRoot 'bin'
$script:ZapretListsDir = Join-Path $script:ZapretRoot 'lists'
$script:ZapretStrategiesDir = Join-Path $script:ZapretRoot 'strategies'
$script:ZapretConfigPath = Join-Path $script:ZapretRoot 'config.json'
$script:ZapretResultsDir = Join-Path $script:ZapretRoot 'test-results'

function Get-ZapretLayout {
    return New-Object PSObject -Property @{
        Root       = $script:ZapretRoot
        Bin        = $script:ZapretBinDir
        Lists      = $script:ZapretListsDir
        Utils      = $script:ZapretUtilsDir
        Strategies = $script:ZapretStrategiesDir
        Config     = $script:ZapretConfigPath
        Results    = $script:ZapretResultsDir
        Versions   = (Join-Path $script:ZapretBinDir 'versions.json')
        Module     = $PSScriptRoot
        Version    = $script:ZapretLocalVersion
    }
}

function Get-ZapretConfigPath {
    return $script:ZapretConfigPath
}

function New-ZapretConfigDefaults {
    return New-Object PSObject -Property @{
        language         = ''
        engine           = 'winws'
        gameFilter       = 'disabled'
        autoUpdateCheck  = $true
        testTargets      = @(
            (New-Object PSObject -Property @{ name = 'DiscordMain'; value = 'https://discord.com' })
            (New-Object PSObject -Property @{ name = 'DiscordGateway'; value = 'https://gateway.discord.gg' })
            (New-Object PSObject -Property @{ name = 'DiscordCDN'; value = 'https://cdn.discordapp.com' })
            (New-Object PSObject -Property @{ name = 'DiscordUpdates'; value = 'https://updates.discord.com' })
            (New-Object PSObject -Property @{ name = 'YouTubeWeb'; value = 'https://www.youtube.com' })
            (New-Object PSObject -Property @{ name = 'YouTubeShort'; value = 'https://youtu.be' })
            (New-Object PSObject -Property @{ name = 'YouTubeImage'; value = 'https://i.ytimg.com' })
            (New-Object PSObject -Property @{ name = 'YouTubeVideoRedirect'; value = 'https://redirector.googlevideo.com' })
            (New-Object PSObject -Property @{ name = 'GoogleMain'; value = 'https://www.google.com' })
            (New-Object PSObject -Property @{ name = 'GoogleGstatic'; value = 'https://www.gstatic.com' })
            (New-Object PSObject -Property @{ name = 'CloudflareWeb'; value = 'https://www.cloudflare.com' })
            (New-Object PSObject -Property @{ name = 'CloudflareCDN'; value = 'https://cdnjs.cloudflare.com' })
            (New-Object PSObject -Property @{ name = 'CloudflareDNS1111'; value = 'PING:1.1.1.1' })
            (New-Object PSObject -Property @{ name = 'CloudflareDNS1001'; value = 'PING:1.0.0.1' })
            (New-Object PSObject -Property @{ name = 'GoogleDNS8888'; value = 'PING:8.8.8.8' })
            (New-Object PSObject -Property @{ name = 'GoogleDNS8844'; value = 'PING:8.8.4.4' })
            (New-Object PSObject -Property @{ name = 'Quad9DNS9999'; value = 'PING:9.9.9.9' })
        )
    }
}

function Get-ZapretConfig {
    $cfg = New-ZapretConfigDefaults
    $path = $script:ZapretConfigPath
    if (-not (Test-Path -LiteralPath $path)) {
        return $cfg
    }
    try {
        $raw = [System.IO.File]::ReadAllText($path)
        $parsed = $raw | ConvertFrom-Json
    } catch {
        return $cfg
    }
    if ($parsed.PSObject.Properties['language']) {
        $cfg.language = [string]$parsed.language
    }
    if ($parsed.PSObject.Properties['engine']) {
        $eng = [string]$parsed.engine
        if ($eng -eq 'winws' -or $eng -eq 'winws2') {
            $cfg.engine = $eng
        }
    }
    if ($parsed.PSObject.Properties['gameFilter']) {
        $mode = [string]$parsed.gameFilter
        if ($mode -eq 'disabled' -or $mode -eq 'all' -or $mode -eq 'tcp' -or $mode -eq 'udp') {
            $cfg.gameFilter = $mode
        }
    }
    if ($parsed.PSObject.Properties['autoUpdateCheck']) {
        $cfg.autoUpdateCheck = [bool]$parsed.autoUpdateCheck
    }
    if ($parsed.PSObject.Properties['testTargets']) {
        $items = @($parsed.testTargets)
        if ($items.Count -gt 0) {
            $cfg.testTargets = @($items)
        }
    }
    return $cfg
}

function Save-ZapretConfig {
    param($Config)
    $payload = New-Object PSObject -Property @{
        language        = [string]$Config.language
        engine          = [string]$Config.engine
        gameFilter      = [string]$Config.gameFilter
        autoUpdateCheck = [bool]$Config.autoUpdateCheck
        testTargets     = @($Config.testTargets)
    }
    $json = $payload | ConvertTo-Json -Depth 6
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($script:ZapretConfigPath, $json, $utf8)
}

function Update-ZapretConfig {
    param(
        [string]$Language,
        [string]$Engine,
        [string]$GameFilter,
        [object]$AutoUpdateCheck
    )
    $cfg = Get-ZapretConfig
    if ($PSBoundParameters.ContainsKey('Language')) {
        $cfg.language = $Language
    }
    if ($PSBoundParameters.ContainsKey('Engine')) {
        if ($Engine -eq 'winws2') {
            $cfg.engine = 'winws2'
        } else {
            $cfg.engine = 'winws'
        }
    }
    if ($PSBoundParameters.ContainsKey('GameFilter')) {
        $cfg.gameFilter = $GameFilter
    }
    if ($PSBoundParameters.ContainsKey('AutoUpdateCheck')) {
        $cfg.autoUpdateCheck = [bool]$AutoUpdateCheck
    }
    Save-ZapretConfig -Config $cfg
    return $cfg
}

function Get-ZapretEngine {
    $cfg = Get-ZapretConfig
    if ([string]$cfg.engine -eq 'winws2') {
        return 'winws2'
    }
    return 'winws'
}

function Set-ZapretEngine {
    param([string]$Engine)
    $value = 'winws'
    if ($Engine -eq 'winws2') {
        $value = 'winws2'
    }
    [void](Update-ZapretConfig -Engine $value)
    return $value
}

function Get-ZapretLocalVersion {
    return $script:ZapretLocalVersion
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
    $ipsetAll = Join-Path $script:ZapretListsDir 'ipset-all.txt'
    if (-not (Test-Path -LiteralPath $ipsetAll)) {
        $seed = Join-Path $script:ZapretListsDir 'ipset-all.default.txt'
        if (Test-Path -LiteralPath $seed) {
            Copy-Item -LiteralPath $seed -Destination $ipsetAll
        } else {
            Set-Content -LiteralPath $ipsetAll -Value '203.0.113.113/32' -Encoding ASCII
        }
    }
}

function Test-ZapretTcpTimestampsEnabled {
    param([string]$ShowText = '')
    if ([string]::IsNullOrWhiteSpace($ShowText)) {
        $ShowText = netsh interface tcp show global 2>$null | Out-String
    }
    # Match English "timestamps" and Russian "RFC 1323" netsh lines.
    return [bool]($ShowText -match '(?i)(?:timestamps|RFC\s*1323)[^\r\n]*enabled')
}

function Enable-ZapretTcpTimestamps {
    if (Test-ZapretTcpTimestampsEnabled) {
        return
    }
    netsh interface tcp set global timestamps=enabled | Out-Null
}

function Enable-ZapretTls12 {
    try {
        $tls = [Net.ServicePointManager]::SecurityProtocol
        [Net.ServicePointManager]::SecurityProtocol = $tls -bor [Net.SecurityProtocolType]::Tls12
    } catch {
        return
    }
}

function Test-Zapret64BitOs {
    try {
        return [Environment]::Is64BitOperatingSystem
    } catch {
        return $env:PROCESSOR_ARCHITECTURE -ne 'x86' -or $env:PROCESSOR_ARCHITEW6432
    }
}

function Get-ZapretNet45Release {
    $path = 'HKLM:\SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full'
    try {
        $item = Get-ItemProperty -LiteralPath $path -Name Release -ErrorAction Stop
        return [int]$item.Release
    } catch {
        return 0
    }
}

function Test-ZapretWinForms {
    try {
        Add-Type -AssemblyName System.Windows.Forms -ErrorAction Stop
        return $true
    } catch {
        return $false
    }
}

function Get-ZapretBinVersionsPath {
    return (Join-Path $script:ZapretBinDir 'versions.json')
}

function Test-ZapretBinVersions {
    $errors = New-Object System.Collections.ArrayList
    $path = Get-ZapretBinVersionsPath
    if (-not (Test-Path -LiteralPath $path)) {
        [void]$errors.Add('bin/versions.json is not found. Pinned release hashes cannot be checked.')
        return @($errors)
    }
    try {
        $raw = [System.IO.File]::ReadAllText($path)
        $manifest = $raw | ConvertFrom-Json
    } catch {
        [void]$errors.Add('bin/versions.json is not valid JSON.')
        return @($errors)
    }
    $items = @()
    if ($manifest.PSObject.Properties['files']) {
        $items = @($manifest.files)
    }
    if ($items.Count -lt 1) {
        [void]$errors.Add('bin/versions.json has no files list.')
        return @($errors)
    }
    foreach ($item in $items) {
        $rel = [string]$item.path
        $want = ([string]$item.sha256).Trim().ToLowerInvariant()
        $src = [string]$item.source
        if ([string]::IsNullOrWhiteSpace($rel) -or [string]::IsNullOrWhiteSpace($want)) {
            [void]$errors.Add('bin/versions.json has a file entry without path or sha256.')
            continue
        }
        $full = Join-Path $script:ZapretBinDir ($rel -replace '/', '\')
        if (-not (Test-Path -LiteralPath $full)) {
            [void]$errors.Add(("Pinned file is missing: {0} ({1})." -f $rel, $src))
            continue
        }
        $got = (Get-FileHash -LiteralPath $full -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($got -ne $want) {
            [void]$errors.Add(("SHA256 mismatch: {0} ({1}). File is not the pinned release." -f $rel, $src))
        }
    }
    return @($errors)
}

# Registry, Add-Type, and SHA256 of pinned bin files. Do not call CIM/WMI here: that delays the GUI.
function Test-ZapretHostReady {
    $errors = New-Object System.Collections.ArrayList
    if (-not (Test-Zapret64BitOs)) {
        [void]$errors.Add('This program needs a 64-bit Windows system. 32-bit is not supported.')
    }
    if (-not (Test-Path -LiteralPath $script:ZapretBinDir)) {
        [void]$errors.Add('The bin folder is not found. Extract the full Zapret archive first.')
    }
    $netRelease = Get-ZapretNet45Release
    # 378389 is .NET Framework 4.5.
    if ($netRelease -lt 378389) {
        [void]$errors.Add('.NET Framework 4.5 or newer is not installed. On Windows 7 install .NET 4.5+ and WMF 5.1.')
    }
    if (-not (Test-ZapretWinForms)) {
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
    if (Test-Path -LiteralPath $script:ZapretBinDir) {
        foreach ($pinErr in @(Test-ZapretBinVersions)) {
            [void]$errors.Add($pinErr)
        }
    }
    return @($errors)
}

function Show-ZapretHostReadyReport {
    $errors = @(Test-ZapretHostReady)
    if (@($errors).Count -lt 1) {
        return 0
    }

    $caption = ''
    try {
        $os = Get-CimInstance -ClassName Win32_OperatingSystem -ErrorAction Stop
        $caption = [string]$os.Caption
    } catch {
        try {
            $os = Get-WmiObject -Class Win32_OperatingSystem -ErrorAction Stop
            $caption = [string]$os.Caption
        } catch {
            $caption = ''
        }
    }

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
    return 1
}
