# Zapret Manager: status, diagnostics, fakes, list download, version check.

Set-StrictMode -Version Latest

function Test-ZapmanAutoUpdateEnabled {
    return [bool]((Get-ZapmanConfig).autoUpdateCheck)
}

function Set-ZapmanAutoUpdateEnabled {
    param([bool]$Enabled)
    [void](Update-ZapmanConfig -AutoUpdateCheck $Enabled)
}

function Get-ZapretVersionCheckUrl {
    return 'https://raw.githubusercontent.com/Flowseal/zapret-discord-youtube/main/.service/version.txt'
}

function Get-ZapretRemoteVersion {
    $res = Invoke-WebRequest -Uri (Get-ZapretVersionCheckUrl) -UseBasicParsing -TimeoutSec 5 -Headers @{ 'Cache-Control' = 'no-cache' }
    return ([string]$res.Content).Trim()
}

function Get-ZapretReleasePageUrl {
    return 'https://github.com/Flowseal/zapret-discord-youtube/releases/latest'
}

function Get-ZapretIpsetListUrl {
    return 'https://raw.githubusercontent.com/Flowseal/zapret-discord-youtube/refs/heads/main/.service/ipset-service.txt'
}

function Get-ZapretHostsSourceUrl {
    return 'https://raw.githubusercontent.com/Flowseal/zapret-discord-youtube/refs/heads/main/.service/hosts'
}

function Invoke-ZapmanWebDownload {
    param(
        [string]$Url,
        [string]$Destination
    )
    Enable-ZapmanTls12
    $dir = Split-Path -Parent $Destination
    if ($dir -and -not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
    }
    $wc = New-Object System.Net.WebClient
    try {
        $wc.Headers.Add('Cache-Control', 'no-cache')
        $wc.Headers.Add('User-Agent', 'zapret')
        $wc.DownloadFile($Url, $Destination)
    } finally {
        $wc.Dispose()
    }
    if (-not (Test-Path -LiteralPath $Destination)) {
        throw 'Download failed.'
    }
}

function Update-ZapretIpsetList {
    param([string]$SourceFile = '')
    $listFile = Join-Path $script:ZapmanListsDir 'ipset-all.txt'
    $dir = Split-Path -Parent $listFile
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
    }
    if ($SourceFile) {
        if (-not (Test-Path -LiteralPath $SourceFile)) {
            throw 'The downloaded IPSet file is not found.'
        }
        Copy-Item -LiteralPath $SourceFile -Destination $listFile -Force
        return
    }
    $temp = Join-Path $env:TEMP 'zapret-ipset-all.txt'
    Invoke-ZapmanWebDownload -Url (Get-ZapretIpsetListUrl) -Destination $temp
    Copy-Item -LiteralPath $temp -Destination $listFile -Force
}

function Get-ZapretHostsUpdateInfo {
    param([string]$TempFile = '')
    $hostsFile = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
    $hostsUrl = Get-ZapretHostsSourceUrl
    if (-not $TempFile) {
        $TempFile = Join-Path $env:TEMP 'zapret_hosts.txt'
        Invoke-ZapmanWebDownload -Url ($hostsUrl + '?t=' + [guid]::NewGuid().ToString()) -Destination $TempFile
    }
    $lines = @(Get-Content -LiteralPath $TempFile | Where-Object { $_ -ne '' })
    $first = ''
    $last = ''
    if ($lines.Count -gt 0) {
        $first = [string]$lines[0]
        $last = [string]$lines[$lines.Count - 1]
    }
    $hostsText = ''
    if (Test-Path -LiteralPath $hostsFile) {
        $hostsText = [System.IO.File]::ReadAllText($hostsFile)
    }
    $needs = ($first -ne '') -and (($hostsText -notlike "*$first*") -or ($hostsText -notlike "*$last*"))
    return New-Object PSObject -Property @{
        NeedsUpdate = $needs
        TempFile    = $TempFile
        HostsFile   = $hostsFile
        SourceUrl   = $hostsUrl
    }
}

function Open-ZapretHostsUpdate {
    param($Info)
    Start-Process notepad.exe $Info.TempFile
    Start-Process explorer.exe -ArgumentList "/select,`"$($Info.HostsFile)`""
}

function Get-ZapretFakeCatalog {
    $bin = $script:ZapmanBinDir
    if (-not (Test-Path -LiteralPath $bin)) {
        throw 'The bin folder is not found.'
    }
    $discordHash = $null
    $gameHash = $null
    $discordActive = Join-Path $bin 'ACTIVE_DISCORD_UDP.bin'
    $gameActive = Join-Path $bin 'ACTIVE_GAME_UDP.bin'
    if (Test-Path -LiteralPath $discordActive) {
        $discordHash = (Get-FileHash -LiteralPath $discordActive -Algorithm SHA256).Hash
    }
    if (Test-Path -LiteralPath $gameActive) {
        $gameHash = (Get-FileHash -LiteralPath $gameActive -Algorithm SHA256).Hash
    }
    $files = @(Get-ChildItem -LiteralPath $bin -File -Filter '*.bin' | Where-Object { $_.BaseName -notlike 'ACTIVE_*' })
    # PS 5.1 parses List[object] as New-Object List, then [object]. Use ArrayList.
    $items = New-Object System.Collections.ArrayList
    $currentDiscord = $null
    $currentGame = $null
    $discordState = 'missing'
    $gameState = 'missing'
    if ($discordHash) { $discordState = 'unlisted' }
    if ($gameHash) { $gameState = 'unlisted' }
    foreach ($file in $files) {
        $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash
        [void]$items.Add((New-Object PSObject -Property @{
            Name     = $file.BaseName
            FullName = $file.FullName
            Hash     = $hash
        }))
        if ($discordHash -and ($hash -eq $discordHash) -and (-not $currentDiscord)) {
            $currentDiscord = $file.BaseName
            $discordState = 'matched'
        }
        if ($gameHash -and ($hash -eq $gameHash) -and (-not $currentGame)) {
            $currentGame = $file.BaseName
            $gameState = 'matched'
        }
    }
    return New-Object PSObject -Property @{
        Files          = @($items)
        CurrentDiscord = $currentDiscord
        CurrentGame    = $currentGame
        DiscordState   = $discordState
        GameState      = $gameState
        DiscordActive  = $discordActive
        GameActive     = $gameActive
    }
}

function Get-ZapretFakeCurrentText {
    param(
        [string]$Name,
        [string]$State
    )
    if ($State -eq 'matched' -and -not [string]::IsNullOrWhiteSpace($Name)) {
        return $Name
    }
    if ($State -eq 'unlisted') {
        return (Get-ZapmanUiString -Key 'FakesUnlisted')
    }
    return (Get-ZapmanUiString -Key 'FakesMissing')
}

function Set-ZapretActiveFake {
    param(
        [ValidateSet('discord', 'game')]
        [string]$Slot,
        [string]$SourcePath
    )
    if (-not (Test-Path -LiteralPath $SourcePath)) {
        throw 'The fake file is not found.'
    }
    $dest = Join-Path $script:ZapmanBinDir 'ACTIVE_GAME_UDP.bin'
    if ($Slot -eq 'discord') {
        $dest = Join-Path $script:ZapmanBinDir 'ACTIVE_DISCORD_UDP.bin'
    }
    if (Test-Path -LiteralPath $dest) {
        Remove-Item -LiteralPath $dest -Force
    }
    Copy-Item -LiteralPath $SourcePath -Destination $dest -Force
}

function Get-ZapmanStatusLines {
    $lines = New-Object System.Collections.Generic.List[string]
    $cfgErr = Get-ZapmanConfigError
    if (-not [string]::IsNullOrWhiteSpace($cfgErr)) {
        [void]$lines.Add($cfgErr)
    }
    $runningName = Get-ZapretRunningStrategyName
    if ($runningName) {
        [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineRunning' -FormatArgs @($runningName)))
    }
    $name = Get-ZapretInstalledStrategyName
    if ($name) {
        [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineService' -FormatArgs @($name)))
    }
    foreach ($svcName in @('zapret', 'WinDivert')) {
        $svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
        if ($svc -and $svc.Status -eq 'Running') {
            [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineSvcRun' -FormatArgs @($svcName)))
        } elseif ($svc -and ([string]$svc.Status -eq 'StopPending')) {
            [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineSvcPending' -FormatArgs @($svcName)))
        } else {
            [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineSvcOff' -FormatArgs @($svcName)))
        }
    }
    $sys = Get-ChildItem -LiteralPath $script:ZapmanBinDir -Filter '*.sys' -ErrorAction SilentlyContinue
    if (-not $sys) {
        [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineSysMissing'))
    }
    $winws = @(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)
    $winws2 = @(Get-Process -Name 'winws2' -ErrorAction SilentlyContinue)
    if ($winws.Count -gt 0) {
        [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineBypassOn' -FormatArgs @('winws.exe')))
    }
    if ($winws2.Count -gt 0) {
        [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineBypassOn' -FormatArgs @('winws2.exe')))
    }
    if (($winws.Count + $winws2.Count) -eq 0) {
        [void]$lines.Add((Get-ZapmanUiString -Key 'StatusLineBypassOff'))
    }
    return @($lines)
}

function Get-ZapmanDiscordCacheApps {
    $pairs = @(
        @{ Exe = 'Discord.exe'; Dir = Join-Path $env:APPDATA 'discord'; Label = 'Discord' },
        @{ Exe = 'DiscordPTB.exe'; Dir = Join-Path $env:APPDATA 'discordptb'; Label = 'Discord PTB' },
        @{ Exe = 'DiscordCanary.exe'; Dir = Join-Path $env:APPDATA 'discordcanary'; Label = 'Discord Canary' },
        @{ Exe = 'DiscordDevelopment.exe'; Dir = Join-Path $env:APPDATA 'discorddevelopment'; Label = 'Discord Development' }
    )
    $found = New-Object System.Collections.ArrayList
    foreach ($pair in $pairs) {
        if (-not (Test-Path -LiteralPath $pair.Dir)) {
            continue
        }
        $hasCache = $false
        foreach ($sub in @('Cache', 'Code Cache', 'GPUCache')) {
            if (Test-Path -LiteralPath (Join-Path $pair.Dir $sub)) {
                $hasCache = $true
                break
            }
        }
        if ($hasCache) {
            [void]$found.Add((New-Object PSObject -Property @{
                Exe   = [string]$pair.Exe
                Dir   = [string]$pair.Dir
                Label = [string]$pair.Label
            }))
        }
    }
    return @($found)
}

function Clear-ZapmanDiscordCache {
    $lines = New-Object System.Collections.Generic.List[string]
    $apps = @(Get-ZapmanDiscordCacheApps)
    if ($apps.Count -lt 1) {
        [void]$lines.Add('WARN: No Discord cache folders found.')
        return @($lines)
    }
    foreach ($pair in $apps) {
        Get-Process -Name ([IO.Path]::GetFileNameWithoutExtension($pair.Exe)) -ErrorAction SilentlyContinue |
            Stop-Process -Force -ErrorAction SilentlyContinue
        foreach ($sub in @('Cache', 'Code Cache', 'GPUCache')) {
            $p = Join-Path $pair.Dir $sub
            if (Test-Path -LiteralPath $p) {
                Remove-Item -LiteralPath $p -Recurse -Force -ErrorAction SilentlyContinue
                if (Test-Path -LiteralPath $p) {
                    [void]$lines.Add("FAIL: Failed to delete $p")
                } else {
                    [void]$lines.Add("OK: Deleted $p")
                }
            }
        }
    }
    return @($lines)
}

function Remove-ZapretNamedServices {
    param([string[]]$Names)
    $lines = New-Object System.Collections.Generic.List[string]
    foreach ($svcName in @($Names)) {
        if ([string]::IsNullOrWhiteSpace($svcName)) { continue }
        & net.exe stop $svcName 2>$null | Out-Null
        & sc.exe delete $svcName 2>$null | Out-Null
        Start-Sleep -Milliseconds 300
        if (Get-Service -Name $svcName -ErrorAction SilentlyContinue) {
            [void]$lines.Add("FAIL: Failed to remove service: $svcName")
        } else {
            [void]$lines.Add("OK: Removed service: $svcName")
        }
    }
    foreach ($drv in @('WinDivert', 'WinDivert14')) {
        & net.exe stop $drv 2>$null | Out-Null
        & sc.exe delete $drv 2>$null | Out-Null
    }
    return @($lines)
}

function New-ZapmanDiagItem {
    param(
        [string]$Id,
        [string]$Status,
        [string]$Text,
        [string]$Action = '',
        [string[]]$ActionNames = @()
    )
    return New-Object PSObject -Property @{
        Id          = $Id
        Status      = $Status
        Text        = $Text
        Action      = $Action
        ActionNames = @($ActionNames)
    }
}

function Invoke-ZapmanDiagnosticAction {
    param(
        [string]$Action,
        [string[]]$Names
    )
    if ($Action -eq 'discord-cache') {
        return @(Clear-ZapmanDiscordCache)
    }
    if ($Action -eq 'remove-services') {
        return @(Remove-ZapretNamedServices -Names @($Names))
    }
    if ($Action -eq 'remove-windivert') {
        return @(Remove-ZapretNamedServices -Names @('WinDivert', 'WinDivert14'))
    }
    return @()
}

function Get-ZapmanDiagnosticReport {
    $items = New-Object System.Collections.ArrayList
    $conflicts = New-Object System.Collections.ArrayList

    [void]$items.Add((New-ZapmanDiagItem -Id 'path' -Status 'ok' -Text ("Zapret Manager is installed in: '{0}'" -f $script:ZapmanRoot)))

    $bfe = Get-Service -Name 'BFE' -ErrorAction SilentlyContinue
    if ($bfe -and $bfe.Status -eq 'Running') {
        [void]$items.Add((New-ZapmanDiagItem -Id 'bfe' -Status 'ok' -Text 'Base Filtering Engine is running'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'bfe' -Status 'fail' -Text 'Base Filtering Engine is not running. This service is required'))
    }

    $proxyOn = $null
    try {
        $proxyOn = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyEnable -ErrorAction Stop).ProxyEnable
    } catch {
        $proxyOn = 0
    }
    if ($proxyOn -eq 1) {
        $server = ''
        try {
            $server = [string](Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyServer -ErrorAction Stop).ProxyServer
        } catch {
            $server = ''
        }
        [void]$items.Add((New-ZapmanDiagItem -Id 'proxy' -Status 'warn' -Text ("System proxy is enabled: {0}. Disable it if you do not use a proxy" -f $server)))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'proxy' -Status 'ok' -Text 'System proxy is off'))
    }

    Enable-ZapmanTcpTimestamps
    if (Test-ZapmanTcpTimestampsEnabled) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'tcp' -Status 'ok' -Text 'TCP timestamps are enabled'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'tcp' -Status 'fail' -Text 'Failed to enable TCP timestamps'))
    }

    if (@(Get-Process -Name 'AdguardSvc' -ErrorAction SilentlyContinue).Count -gt 0) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'adguard' -Status 'fail' -Text 'Adguard process found. Adguard may block Discord'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'adguard' -Status 'ok' -Text 'Adguard is not running'))
    }

    $allSvc = @(Get-Service -ErrorAction SilentlyContinue)
    if ($allSvc | Where-Object { $_.DisplayName -match 'Killer' -or $_.Name -match 'Killer' }) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'killer' -Status 'fail' -Text 'Killer services found. Killer conflicts with zapret'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'killer' -Status 'ok' -Text 'No Killer services'))
    }

    $intel = $allSvc | Where-Object {
        ($_.DisplayName -match 'Intel') -and ($_.DisplayName -match 'Connectivity') -and ($_.DisplayName -match 'Network')
    }
    if ($intel) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'intel' -Status 'fail' -Text 'Intel Connectivity Network Service found. It conflicts with zapret'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'intel' -Status 'ok' -Text 'No Intel Connectivity Network Service'))
    }

    $checkpoint = $allSvc | Where-Object { $_.Name -match 'TracSrvWrapper' -or $_.Name -match 'EPWD' }
    if ($checkpoint) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'checkpoint' -Status 'fail' -Text 'Check Point services found. Uninstall Check Point'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'checkpoint' -Status 'ok' -Text 'No Check Point services'))
    }

    if ($allSvc | Where-Object { $_.DisplayName -match 'SmartByte' -or $_.Name -match 'SmartByte' }) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'smartbyte' -Status 'fail' -Text 'SmartByte services found. Disable SmartByte in services.msc'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'smartbyte' -Status 'ok' -Text 'No SmartByte services'))
    }

    if ($script:ZapmanRoot -match '[\u0400-\u04FF]') {
        [void]$items.Add((New-ZapmanDiagItem -Id 'cyrillic' -Status 'warn' -Text 'The path contains Cyrillic characters. Move Zapret to C:\zapret if bypass fails'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'cyrillic' -Status 'ok' -Text 'Path has no Cyrillic characters'))
    }

    $oneDrive = [string]$env:OneDrive
    if ($oneDrive -and ($script:ZapmanRoot -like ($oneDrive.TrimEnd('\') + '\*'))) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'onedrive' -Status 'fail' -Text 'Zapret is in a OneDrive folder. Move it to C:\zapret'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'onedrive' -Status 'ok' -Text 'Zapret is not in OneDrive'))
    }

    if (Get-ChildItem -LiteralPath $script:ZapmanBinDir -Filter '*.sys' -ErrorAction SilentlyContinue) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'sys' -Status 'ok' -Text 'WinDivert64.sys is present'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'sys' -Status 'fail' -Text 'WinDivert64.sys file NOT found'))
    }

    $vpn = @($allSvc | Where-Object { $_.DisplayName -match 'VPN' -or $_.Name -match 'VPN' } | ForEach-Object { $_.Name })
    if ($vpn.Count -gt 0) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'vpn' -Status 'warn' -Text ('VPN services found: {0}. Disable VPN if bypass fails' -f ($vpn -join ', '))))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'vpn' -Status 'ok' -Text 'No VPN services'))
    }

    $dohCount = 0
    try {
        $dohRoot = 'HKLM:\SYSTEM\CurrentControlSet\Services\Dnscache\InterfaceSpecificParameters'
        if (Test-Path -LiteralPath $dohRoot) {
            $dohCount = @(
                Get-ChildItem -LiteralPath $dohRoot -Recurse -ErrorAction SilentlyContinue |
                    Get-ItemProperty -ErrorAction SilentlyContinue |
                    Where-Object { $_.DohFlags -gt 0 }
            ).Count
        }
    } catch {
        $dohCount = 0
    }
    if ($dohCount -gt 0) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'doh' -Status 'ok' -Text 'Secure DNS is configured'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'doh' -Status 'warn' -Text 'Set Secure DNS in the browser (non-default provider) or Windows 11 Settings'))
    }

    $hostsFile = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
    if (Test-Path -LiteralPath $hostsFile) {
        $hostsText = [System.IO.File]::ReadAllText($hostsFile)
        if ($hostsText -match '(?i)youtube\.com' -or $hostsText -match '(?i)youtu\.be') {
            [void]$items.Add((New-ZapmanDiagItem -Id 'hosts' -Status 'warn' -Text 'hosts contains youtube.com or youtu.be. This may break YouTube'))
        } else {
            [void]$items.Add((New-ZapmanDiagItem -Id 'hosts' -Status 'ok' -Text 'hosts has no YouTube overrides'))
        }
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'hosts' -Status 'ok' -Text 'hosts file not found'))
    }

    $winwsRunning = (Test-ZapretBypassRunning)
    $wd = Get-Service -Name 'WinDivert' -ErrorAction SilentlyContinue
    $wdBusy = $false
    if ($wd) {
        $st = [string]$wd.Status
        if ($st -eq 'Running' -or $st -eq 'StopPending') { $wdBusy = $true }
    }
    if ((-not $winwsRunning) -and $wdBusy) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'windivert' -Status 'warn' -Text 'winws.exe is not running but WinDivert is active' -Action 'remove-windivert'))
        if (Get-Service -Name 'GoodbyeDPI' -ErrorAction SilentlyContinue) {
            [void]$conflicts.Add('GoodbyeDPI')
        }
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'windivert' -Status 'ok' -Text 'No leftover WinDivert without winws'))
    }

    foreach ($svcName in @('GoodbyeDPI', 'discordfix_zapret', 'winws1', 'winws2')) {
        if (Get-Service -Name $svcName -ErrorAction SilentlyContinue) {
            if (-not ($conflicts -contains $svcName)) {
                [void]$conflicts.Add($svcName)
            }
        }
    }
    if ($conflicts.Count -gt 0) {
        [void]$items.Add((New-ZapmanDiagItem -Id 'conflicts' -Status 'fail' -Text ('Conflicting bypass services: {0}' -f ($conflicts -join ', ')) -Action 'remove-services' -ActionNames @($conflicts)))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'conflicts' -Status 'ok' -Text 'No conflicting bypass services'))
    }

    $discordApps = @(Get-ZapmanDiscordCacheApps)
    if ($discordApps.Count -gt 0) {
        $labels = @($discordApps | ForEach-Object { $_.Label })
        [void]$items.Add((New-ZapmanDiagItem -Id 'discord' -Status 'warn' -Text ('Discord cache found: {0}' -f ($labels -join ', ')) -Action 'discord-cache'))
    } else {
        [void]$items.Add((New-ZapmanDiagItem -Id 'discord' -Status 'ok' -Text 'No Discord cache folders to clear'))
    }

    $lines = New-Object System.Collections.Generic.List[string]
    foreach ($item in @($items)) {
        $prefix = 'OK'
        if ($item.Status -eq 'fail') {
            $prefix = 'FAIL'
        } elseif ($item.Status -eq 'warn') {
            $prefix = 'WARN'
        }
        [void]$lines.Add(('{0}: {1}' -f $prefix, $item.Text))
    }

    return New-Object PSObject -Property @{
        Items     = @($items)
        Lines     = @($lines)
        Conflicts = @($conflicts)
    }
}
