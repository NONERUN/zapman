# Zapret module: status, diagnostics, fakes, list download, version check.

Set-StrictMode -Version Latest

function Test-ZapretAutoUpdateEnabled {
    return [bool]((Get-ZapretConfig).autoUpdateCheck)
}

function Set-ZapretAutoUpdateEnabled {
    param([bool]$Enabled)
    [void](Update-ZapretConfig -AutoUpdateCheck $Enabled)
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

function Invoke-ZapretWebDownload {
    param(
        [string]$Url,
        [string]$Destination
    )
    Enable-ZapretTls12
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
    $listFile = Join-Path $script:ZapretListsDir 'ipset-all.txt'
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
    Invoke-ZapretWebDownload -Url (Get-ZapretIpsetListUrl) -Destination $temp
    Copy-Item -LiteralPath $temp -Destination $listFile -Force
}

function Get-ZapretHostsUpdateInfo {
    param([string]$TempFile = '')
    $hostsFile = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
    $hostsUrl = Get-ZapretHostsSourceUrl
    if (-not $TempFile) {
        $TempFile = Join-Path $env:TEMP 'zapret_hosts.txt'
        Invoke-ZapretWebDownload -Url ($hostsUrl + '?t=' + [guid]::NewGuid().ToString()) -Destination $TempFile
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
    $bin = $script:ZapretBinDir
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
    $currentDiscord = '(not found)'
    $currentGame = '(not found)'
    foreach ($file in $files) {
        $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash
        [void]$items.Add((New-Object PSObject -Property @{
            Name     = $file.BaseName
            FullName = $file.FullName
            Hash     = $hash
        }))
        if ($discordHash -and ($hash -eq $discordHash)) { $currentDiscord = $file.BaseName }
        if ($gameHash -and ($hash -eq $gameHash)) { $currentGame = $file.BaseName }
    }
    return New-Object PSObject -Property @{
        Files          = @($items)
        CurrentDiscord = $currentDiscord
        CurrentGame    = $currentGame
        DiscordActive  = $discordActive
        GameActive     = $gameActive
    }
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
    $dest = Join-Path $script:ZapretBinDir 'ACTIVE_GAME_UDP.bin'
    if ($Slot -eq 'discord') {
        $dest = Join-Path $script:ZapretBinDir 'ACTIVE_DISCORD_UDP.bin'
    }
    if (Test-Path -LiteralPath $dest) {
        Remove-Item -LiteralPath $dest -Force
    }
    Copy-Item -LiteralPath $SourcePath -Destination $dest -Force
}

function Start-ZapretConfigTests {
    $test = Join-Path $script:ZapretUtilsDir 'test zapret.ps1'
    if (-not (Test-Path -LiteralPath $test)) {
        throw 'src\utils\test zapret.ps1 is not found.'
    }
    Start-Process -FilePath 'powershell.exe' -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$test`"" | Out-Null
}

function Get-ZapretStatusLines {
    $lines = New-Object System.Collections.Generic.List[string]
    $runningName = Get-ZapretRunningStrategyName
    if ($runningName) {
        [void]$lines.Add("Running strategy: $runningName")
    }
    $name = Get-ZapretInstalledStrategyName
    if ($name) {
        [void]$lines.Add("Service strategy: $name")
    }
    foreach ($svcName in @('zapret', 'WinDivert')) {
        $svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
        if ($svc -and $svc.Status -eq 'Running') {
            [void]$lines.Add("$svcName service is RUNNING.")
        } elseif ($svc -and ([string]$svc.Status -eq 'StopPending')) {
            [void]$lines.Add("$svcName is STOP_PENDING. Run Diagnostics to look for a conflict.")
        } else {
            [void]$lines.Add("$svcName service is NOT running.")
        }
    }
    $sys = Get-ChildItem -LiteralPath $script:ZapretBinDir -Filter '*.sys' -ErrorAction SilentlyContinue
    if (-not $sys) {
        [void]$lines.Add('WinDivert64.sys file NOT found.')
    }
    $winws = @(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)
    $winws2 = @(Get-Process -Name 'winws2' -ErrorAction SilentlyContinue)
    if ($winws.Count -gt 0) {
        [void]$lines.Add('Bypass (winws.exe) is RUNNING.')
    }
    if ($winws2.Count -gt 0) {
        [void]$lines.Add('Bypass (winws2.exe) is RUNNING.')
    }
    if (($winws.Count + $winws2.Count) -eq 0) {
        [void]$lines.Add('Bypass (winws / winws2) is NOT running.')
    }
    return @($lines)
}

function Get-ZapretDiscordCacheApps {
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

function Clear-ZapretDiscordCache {
    $lines = New-Object System.Collections.Generic.List[string]
    $apps = @(Get-ZapretDiscordCacheApps)
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

function New-ZapretDiagItem {
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

function Invoke-ZapretDiagnosticAction {
    param(
        [string]$Action,
        [string[]]$Names
    )
    if ($Action -eq 'discord-cache') {
        return @(Clear-ZapretDiscordCache)
    }
    if ($Action -eq 'remove-services') {
        return @(Remove-ZapretNamedServices -Names @($Names))
    }
    if ($Action -eq 'remove-windivert') {
        return @(Remove-ZapretNamedServices -Names @('WinDivert', 'WinDivert14'))
    }
    return @()
}

function Get-ZapretDiagnosticReport {
    $items = New-Object System.Collections.ArrayList
    $conflicts = New-Object System.Collections.ArrayList

    [void]$items.Add((New-ZapretDiagItem -Id 'path' -Status 'ok' -Text ("Zapret is installed in: '{0}'" -f $script:ZapretRoot)))

    $bfe = Get-Service -Name 'BFE' -ErrorAction SilentlyContinue
    if ($bfe -and $bfe.Status -eq 'Running') {
        [void]$items.Add((New-ZapretDiagItem -Id 'bfe' -Status 'ok' -Text 'Base Filtering Engine is running'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'bfe' -Status 'fail' -Text 'Base Filtering Engine is not running. This service is required'))
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
        [void]$items.Add((New-ZapretDiagItem -Id 'proxy' -Status 'warn' -Text ("System proxy is enabled: {0}. Disable it if you do not use a proxy" -f $server)))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'proxy' -Status 'ok' -Text 'System proxy is off'))
    }

    Enable-ZapretTcpTimestamps
    if (Test-ZapretTcpTimestampsEnabled) {
        [void]$items.Add((New-ZapretDiagItem -Id 'tcp' -Status 'ok' -Text 'TCP timestamps are enabled'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'tcp' -Status 'fail' -Text 'Failed to enable TCP timestamps'))
    }

    if (@(Get-Process -Name 'AdguardSvc' -ErrorAction SilentlyContinue).Count -gt 0) {
        [void]$items.Add((New-ZapretDiagItem -Id 'adguard' -Status 'fail' -Text 'Adguard process found. Adguard may block Discord'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'adguard' -Status 'ok' -Text 'Adguard is not running'))
    }

    $allSvc = @(Get-Service -ErrorAction SilentlyContinue)
    if ($allSvc | Where-Object { $_.DisplayName -match 'Killer' -or $_.Name -match 'Killer' }) {
        [void]$items.Add((New-ZapretDiagItem -Id 'killer' -Status 'fail' -Text 'Killer services found. Killer conflicts with zapret'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'killer' -Status 'ok' -Text 'No Killer services'))
    }

    $intel = $allSvc | Where-Object {
        ($_.DisplayName -match 'Intel') -and ($_.DisplayName -match 'Connectivity') -and ($_.DisplayName -match 'Network')
    }
    if ($intel) {
        [void]$items.Add((New-ZapretDiagItem -Id 'intel' -Status 'fail' -Text 'Intel Connectivity Network Service found. It conflicts with zapret'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'intel' -Status 'ok' -Text 'No Intel Connectivity Network Service'))
    }

    $checkpoint = $allSvc | Where-Object { $_.Name -match 'TracSrvWrapper' -or $_.Name -match 'EPWD' }
    if ($checkpoint) {
        [void]$items.Add((New-ZapretDiagItem -Id 'checkpoint' -Status 'fail' -Text 'Check Point services found. Uninstall Check Point'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'checkpoint' -Status 'ok' -Text 'No Check Point services'))
    }

    if ($allSvc | Where-Object { $_.DisplayName -match 'SmartByte' -or $_.Name -match 'SmartByte' }) {
        [void]$items.Add((New-ZapretDiagItem -Id 'smartbyte' -Status 'fail' -Text 'SmartByte services found. Disable SmartByte in services.msc'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'smartbyte' -Status 'ok' -Text 'No SmartByte services'))
    }

    if ($script:ZapretRoot -match '[\u0400-\u04FF]') {
        [void]$items.Add((New-ZapretDiagItem -Id 'cyrillic' -Status 'warn' -Text 'The path contains Cyrillic characters. Move Zapret to C:\zapret if bypass fails'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'cyrillic' -Status 'ok' -Text 'Path has no Cyrillic characters'))
    }

    $oneDrive = [string]$env:OneDrive
    if ($oneDrive -and ($script:ZapretRoot -like ($oneDrive.TrimEnd('\') + '\*'))) {
        [void]$items.Add((New-ZapretDiagItem -Id 'onedrive' -Status 'fail' -Text 'Zapret is in a OneDrive folder. Move it to C:\zapret'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'onedrive' -Status 'ok' -Text 'Zapret is not in OneDrive'))
    }

    if (Get-ChildItem -LiteralPath $script:ZapretBinDir -Filter '*.sys' -ErrorAction SilentlyContinue) {
        [void]$items.Add((New-ZapretDiagItem -Id 'sys' -Status 'ok' -Text 'WinDivert64.sys is present'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'sys' -Status 'fail' -Text 'WinDivert64.sys file NOT found'))
    }

    $vpn = @($allSvc | Where-Object { $_.DisplayName -match 'VPN' -or $_.Name -match 'VPN' } | ForEach-Object { $_.Name })
    if ($vpn.Count -gt 0) {
        [void]$items.Add((New-ZapretDiagItem -Id 'vpn' -Status 'warn' -Text ('VPN services found: {0}. Disable VPN if bypass fails' -f ($vpn -join ', '))))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'vpn' -Status 'ok' -Text 'No VPN services'))
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
        [void]$items.Add((New-ZapretDiagItem -Id 'doh' -Status 'ok' -Text 'Secure DNS is configured'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'doh' -Status 'warn' -Text 'Set Secure DNS in the browser (non-default provider) or Windows 11 Settings'))
    }

    $hostsFile = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
    if (Test-Path -LiteralPath $hostsFile) {
        $hostsText = [System.IO.File]::ReadAllText($hostsFile)
        if ($hostsText -match '(?i)youtube\.com' -or $hostsText -match '(?i)youtu\.be') {
            [void]$items.Add((New-ZapretDiagItem -Id 'hosts' -Status 'warn' -Text 'hosts contains youtube.com or youtu.be. This may break YouTube'))
        } else {
            [void]$items.Add((New-ZapretDiagItem -Id 'hosts' -Status 'ok' -Text 'hosts has no YouTube overrides'))
        }
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'hosts' -Status 'ok' -Text 'hosts file not found'))
    }

    $winwsRunning = (Test-ZapretBypassRunning)
    $wd = Get-Service -Name 'WinDivert' -ErrorAction SilentlyContinue
    $wdBusy = $false
    if ($wd) {
        $st = [string]$wd.Status
        if ($st -eq 'Running' -or $st -eq 'StopPending') { $wdBusy = $true }
    }
    if ((-not $winwsRunning) -and $wdBusy) {
        [void]$items.Add((New-ZapretDiagItem -Id 'windivert' -Status 'warn' -Text 'winws.exe is not running but WinDivert is active' -Action 'remove-windivert'))
        if (Get-Service -Name 'GoodbyeDPI' -ErrorAction SilentlyContinue) {
            [void]$conflicts.Add('GoodbyeDPI')
        }
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'windivert' -Status 'ok' -Text 'No leftover WinDivert without winws'))
    }

    foreach ($svcName in @('GoodbyeDPI', 'discordfix_zapret', 'winws1', 'winws2')) {
        if (Get-Service -Name $svcName -ErrorAction SilentlyContinue) {
            if (-not ($conflicts -contains $svcName)) {
                [void]$conflicts.Add($svcName)
            }
        }
    }
    if ($conflicts.Count -gt 0) {
        [void]$items.Add((New-ZapretDiagItem -Id 'conflicts' -Status 'fail' -Text ('Conflicting bypass services: {0}' -f ($conflicts -join ', ')) -Action 'remove-services' -ActionNames @($conflicts)))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'conflicts' -Status 'ok' -Text 'No conflicting bypass services'))
    }

    $discordApps = @(Get-ZapretDiscordCacheApps)
    if ($discordApps.Count -gt 0) {
        $labels = @($discordApps | ForEach-Object { $_.Label })
        [void]$items.Add((New-ZapretDiagItem -Id 'discord' -Status 'warn' -Text ('Discord cache found: {0}' -f ($labels -join ', ')) -Action 'discord-cache'))
    } else {
        [void]$items.Add((New-ZapretDiagItem -Id 'discord' -Status 'ok' -Text 'No Discord cache folders to clear'))
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
