# Zapret Manager: roots, version, user lists, TLS, TCP timestamps.

Set-StrictMode -Version Latest

# This file is in src\Zapman. The package root is two levels up.
$script:ZapmanRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$script:ZapmanCliDir = Join-Path $script:ZapmanRoot 'src\cli'
$script:ZapmanGuiDir = Join-Path $script:ZapmanRoot 'src\gui'
$script:ZapmanBinDir = Join-Path $script:ZapmanRoot 'bin'
$script:ZapmanListsDir = Join-Path $script:ZapmanRoot 'lists'
$script:ZapmanUserDir = Join-Path $script:ZapmanRoot 'user'
$script:ZapmanStrategiesDir = Join-Path $script:ZapmanRoot 'strategies'
$script:ZapmanConfigPath = Join-Path $script:ZapmanRoot 'config.json'
$script:ZapmanResultsDir = Join-Path $script:ZapmanRoot 'test-results'

function ConvertTo-ZapmanVersion {
    # Product tag is vMAJOR.MINOR.PATCH. Accept a value with or without the v prefix.
    param([string]$Text)
    $t = ([string]$Text).Trim()
    if ([string]::IsNullOrWhiteSpace($t)) {
        return ''
    }
    $t = ($t -split '\r?\n')[0].Trim()
    if ($t.Length -ge 1 -and ($t[0] -eq 'v' -or $t[0] -eq 'V')) {
        $t = 'v' + $t.Substring(1)
    } else {
        $t = 'v' + $t
    }
    return $t
}

function Get-ZapmanVersionNumber {
    param([string]$Tag)
    $t = ConvertTo-ZapmanVersion -Text $Tag
    if ($t -match '^v(\d+)\.(\d+)\.(\d+)$') {
        return New-Object System.Version ([int]$matches[1]), ([int]$matches[2]), ([int]$matches[3])
    }
    return $null
}

function Get-ZapretLayout {
    return New-Object PSObject -Property @{
        Root       = $script:ZapmanRoot
        Bin        = $script:ZapmanBinDir
        Lists      = $script:ZapmanListsDir
        User       = $script:ZapmanUserDir
        Cli        = $script:ZapmanCliDir
        Gui        = $script:ZapmanGuiDir
        Strategies = $script:ZapmanStrategiesDir
        Config     = $script:ZapmanConfigPath
        Results    = $script:ZapmanResultsDir
        Versions   = (Join-Path $script:ZapmanBinDir 'versions.json')
        Module     = $PSScriptRoot
        Version    = (Get-ZapmanLocalVersion)
    }
}

function Get-ZapmanConfigPath {
    return $script:ZapmanConfigPath
}

function Get-ZapmanConfigError {
    [void](Get-ZapmanConfig)
    return [string]$script:ZapmanConfigError
}

function New-ZapmanConfigDefaults {
    return New-Object PSObject -Property @{
        language         = ''
        engine           = 'winws'
        gameFilter       = 'disabled'
        autoUpdateCheck  = $true
        trayWatch        = $true
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

$script:ZapmanConfigCache = $null
$script:ZapmanConfigCacheMtime = $null
$script:ZapmanConfigError = ''

function Get-ZapmanConfig {
    $path = $script:ZapmanConfigPath
    $mtime = [int64]0
    if (Test-Path -LiteralPath $path) {
        $mtime = (Get-Item -LiteralPath $path).LastWriteTimeUtc.Ticks
    }
    if ($script:ZapmanConfigCache -and $script:ZapmanConfigCacheMtime -eq $mtime) {
        return $script:ZapmanConfigCache
    }
    $cfg = New-ZapmanConfigDefaults
    if (-not (Test-Path -LiteralPath $path)) {
        $script:ZapmanConfigError = ''
        $script:ZapmanConfigCache = $cfg
        $script:ZapmanConfigCacheMtime = $mtime
        return $cfg
    }
    try {
        $raw = [System.IO.File]::ReadAllText($path)
        $parsed = $raw | ConvertFrom-Json
    } catch {
        $script:ZapmanConfigError = Get-ZapmanUiString -Key 'ConfigParseFail'
        $script:ZapmanConfigCache = $cfg
        $script:ZapmanConfigCacheMtime = $mtime
        return $cfg
    }
    $script:ZapmanConfigError = ''
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
    if ($parsed.PSObject.Properties['trayWatch']) {
        $cfg.trayWatch = [bool]$parsed.trayWatch
    }
    if ($parsed.PSObject.Properties['testTargets']) {
        $items = @($parsed.testTargets)
        if ($items.Count -gt 0) {
            $cfg.testTargets = @($items)
        }
    }
    $script:ZapmanConfigCache = $cfg
    $script:ZapmanConfigCacheMtime = $mtime
    return $cfg
}

function Save-ZapmanConfig {
    param($Config)
    $payload = New-Object PSObject -Property @{
        language        = [string]$Config.language
        engine          = [string]$Config.engine
        gameFilter      = [string]$Config.gameFilter
        autoUpdateCheck = [bool]$Config.autoUpdateCheck
        trayWatch       = [bool]$Config.trayWatch
        testTargets     = @($Config.testTargets)
    }
    $json = $payload | ConvertTo-Json -Depth 6
    $utf8 = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($script:ZapmanConfigPath, $json, $utf8)
    $script:ZapmanConfigCache = $Config
    $script:ZapmanConfigError = ''
    if (Test-Path -LiteralPath $script:ZapmanConfigPath) {
        $script:ZapmanConfigCacheMtime = (Get-Item -LiteralPath $script:ZapmanConfigPath).LastWriteTimeUtc.Ticks
    }
}

function Update-ZapmanConfig {
    param(
        [string]$Language,
        [string]$Engine,
        [string]$GameFilter,
        [object]$AutoUpdateCheck,
        [object]$TrayWatch
    )
    $cfg = Get-ZapmanConfig
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
    if ($PSBoundParameters.ContainsKey('TrayWatch')) {
        $cfg.trayWatch = [bool]$TrayWatch
    }
    Save-ZapmanConfig -Config $cfg
    return $cfg
}

function Get-ZapretEngine {
    $cfg = Get-ZapmanConfig
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
    [void](Update-ZapmanConfig -Engine $value)
    return $value
}

function Get-ZapmanLocalVersion {
    $psd1 = Join-Path $PSScriptRoot 'Zapman.psd1'
    if (-not (Test-Path -LiteralPath $psd1)) {
        return 'v0.0.0'
    }
    $data = Import-PowerShellDataFile -Path $psd1
    $v = ConvertTo-ZapmanVersion -Text ([string]$data.ModuleVersion)
    if ([string]::IsNullOrWhiteSpace($v)) {
        return 'v0.0.0'
    }
    return $v
}

function Initialize-ZapmanUserLists {
    if (-not (Test-Path -LiteralPath $script:ZapmanUserDir)) {
        New-Item -ItemType Directory -Path $script:ZapmanUserDir | Out-Null
    }
    $ipsetExcludeUser = Join-Path $script:ZapmanUserDir 'ipset-exclude-user.txt'
    if (-not (Test-Path -LiteralPath $ipsetExcludeUser)) {
        Set-Content -LiteralPath $ipsetExcludeUser -Value '203.0.113.113/32' -Encoding ASCII
    }
    $listGeneralUser = Join-Path $script:ZapmanUserDir 'list-general-user.txt'
    if (-not (Test-Path -LiteralPath $listGeneralUser)) {
        Set-Content -LiteralPath $listGeneralUser -Value "# Never leave this file empty`r`ndomain.example.abc" -Encoding ASCII
    }
    $listExcludeUser = Join-Path $script:ZapmanUserDir 'list-exclude-user.txt'
    if (-not (Test-Path -LiteralPath $listExcludeUser)) {
        Set-Content -LiteralPath $listExcludeUser -Value 'domain.example.abc' -Encoding ASCII
    }
    $ipsetAll = Join-Path $script:ZapmanUserDir 'ipset-all.txt'
    if (-not (Test-Path -LiteralPath $ipsetAll)) {
        $seed = Join-Path $script:ZapmanListsDir 'ipset-all.default.txt'
        if (Test-Path -LiteralPath $seed) {
            Copy-Item -LiteralPath $seed -Destination $ipsetAll
        } else {
            Set-Content -LiteralPath $ipsetAll -Value '203.0.113.113/32' -Encoding ASCII
        }
    }
}

function Test-ZapmanTcpTimestampsEnabled {
    param([string]$ShowText = '')
    if ([string]::IsNullOrWhiteSpace($ShowText)) {
        $ShowText = netsh interface tcp show global 2>$null | Out-String
    }
    # Match English "timestamps" and Russian "RFC 1323" netsh lines.
    return [bool]($ShowText -match '(?i)(?:timestamps|RFC\s*1323)[^\r\n]*enabled')
}

function Enable-ZapmanTcpTimestamps {
    if (Test-ZapmanTcpTimestampsEnabled) {
        return
    }
    netsh interface tcp set global timestamps=enabled | Out-Null
}

function Enable-ZapmanTls12 {
    try {
        $tls = [Net.ServicePointManager]::SecurityProtocol
        [Net.ServicePointManager]::SecurityProtocol = $tls -bor [Net.SecurityProtocolType]::Tls12
    } catch {
        return
    }
}

function Test-Zapman64BitOs {
    try {
        return [Environment]::Is64BitOperatingSystem
    } catch {
        return $env:PROCESSOR_ARCHITECTURE -ne 'x86' -or $env:PROCESSOR_ARCHITEW6432
    }
}

function Get-ZapmanNet45Release {
    $path = 'HKLM:\SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full'
    try {
        $item = Get-ItemProperty -LiteralPath $path -Name Release -ErrorAction Stop
        return [int]$item.Release
    } catch {
        return 0
    }
}

function Test-ZapmanWpf {
    try {
        Add-Type -AssemblyName PresentationFramework -ErrorAction Stop
        return $true
    } catch {
        return $false
    }
}

function Get-ZapretBinVersionsPath {
    return (Join-Path $script:ZapmanBinDir 'versions.json')
}

function Get-ZapretPinnedFileSha256 {
    param([string]$Path)
    $name = [System.IO.Path]::GetFileName($Path)
    if ($name -like '*.lua') {
        # Git on Windows may store CRLF in the working tree. The pin is the zip (LF).
        $text = [System.IO.File]::ReadAllText($Path)
        $norm = $text.Replace("`r`n", "`n").Replace("`r", "`n")
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($norm)
        $sha = [System.Security.Cryptography.SHA256]::Create()
        try {
            $hash = $sha.ComputeHash($bytes)
        } finally {
            $sha.Dispose()
        }
        return ([System.BitConverter]::ToString($hash) -replace '-', '').ToLowerInvariant()
    }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Test-ZapretBinVersions {
    param([string]$Engine)
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
        $relNorm = $rel -replace '\\', '/'
        if ($Engine -eq 'winws') {
            if ($relNorm -eq 'winws2.exe' -or $relNorm -like 'lua/*') {
                continue
            }
        } elseif ($Engine -eq 'winws2') {
            if ($relNorm -eq 'winws.exe') {
                continue
            }
        }
        $full = Join-Path $script:ZapmanBinDir ($rel -replace '/', '\')
        if (-not (Test-Path -LiteralPath $full)) {
            [void]$errors.Add(("Pinned file is missing: {0} ({1})." -f $rel, $src))
            continue
        }
        $got = Get-ZapretPinnedFileSha256 -Path $full
        if ($got -ne $want) {
            [void]$errors.Add(("SHA256 mismatch: {0} ({1}). File is not the pinned release." -f $rel, $src))
        }
    }
    return @($errors)
}

# Console probe only (cli.bat env). The GUI does not call this at start.
function Test-ZapmanHostReady {
    $errors = New-Object System.Collections.ArrayList
    if (-not (Test-Zapman64BitOs)) {
        [void]$errors.Add('This program needs a 64-bit Windows system. 32-bit is not supported.')
    }
    if (-not (Test-Path -LiteralPath $script:ZapmanBinDir)) {
        [void]$errors.Add('The bin folder is not found. Extract the full Zapret archive first.')
    }
    $netRelease = Get-ZapmanNet45Release
    # 378389 is .NET Framework 4.5.
    if ($netRelease -lt 378389) {
        [void]$errors.Add('.NET Framework 4.5 or newer is not installed. On Windows 7 install .NET 4.5+ and WMF 5.1.')
    }
    if (-not (Test-ZapmanWpf)) {
        [void]$errors.Add('WPF (PresentationFramework) is not available. Install a full .NET Framework desktop runtime.')
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
    if (Test-Path -LiteralPath $script:ZapmanBinDir) {
        foreach ($pinErr in @(Test-ZapretBinVersions)) {
            [void]$errors.Add($pinErr)
        }
    }
    return @($errors)
}

function Show-ZapmanHostReadyReport {
    param([switch]$ShowVersions)
    if ($ShowVersions) {
        Write-Host ("Zapret Manager {0}" -f (Get-ZapmanLocalVersion))
        Write-Host ("ZapretSpec {0}" -f (Get-ZapmanSpecVersion))
    }
    $cfgErr = Get-ZapmanConfigError
    if (-not [string]::IsNullOrWhiteSpace($cfgErr)) {
        Write-Host $cfgErr -ForegroundColor Yellow
    }
    $errors = @(Test-ZapmanHostReady)
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

$script:ZapmanLastError = ''

function Get-ZapmanLastError {
    return [string]$script:ZapmanLastError
}

function Set-ZapmanLastError {
    param([string]$Text)
    if ($null -eq $Text) {
        $script:ZapmanLastError = ''
        return
    }
    $script:ZapmanLastError = [string]$Text
}

function Get-ZapmanExceptionText {
    param($InputObject)
    if ($null -eq $InputObject) {
        return ''
    }
    $parts = New-Object System.Collections.ArrayList
    $ex = $null
    if ($InputObject -is [System.Management.Automation.ErrorRecord]) {
        $ex = $InputObject.Exception
    } elseif ($InputObject -is [System.Exception]) {
        $ex = $InputObject
    } else {
        return [string]$InputObject
    }
    while ($null -ne $ex) {
        $msg = [string]$ex.Message
        if (-not [string]::IsNullOrWhiteSpace($msg)) {
            if ($parts -notcontains $msg) {
                [void]$parts.Add($msg)
            }
        }
        $ex = $ex.InnerException
    }
    if ($parts.Count -eq 0) {
        return [string]$InputObject
    }
    return (@($parts) -join [Environment]::NewLine)
}
