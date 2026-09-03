# Zapret console manager (diagnostics, tests, hosts, fakes).
# Start: powershell.exe -NoProfile -ExecutionPolicy Bypass -File utils\service.ps1

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

. (Join-Path $PSScriptRoot 'engine.ps1')

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not (Test-IsAdministrator)) {
    $argList = "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`""
    try {
        Start-Process -FilePath 'powershell.exe' -ArgumentList $argList -Verb RunAs | Out-Null
    } catch {
        Write-Host 'Administrator rights are required.' -ForegroundColor Red
        exit 1
    }
    exit 0
}

Initialize-ZapretUserLists
Enable-ZapretTcpTimestamps

function Write-Ok([string]$Message) { Write-Host $Message -ForegroundColor Green }
function Write-Bad([string]$Message) { Write-Host $Message -ForegroundColor Red }
function Write-Warn([string]$Message) { Write-Host $Message -ForegroundColor Yellow }

function Get-InstalledStrategyName {
    try {
        $item = Get-ItemProperty -LiteralPath 'HKLM:\System\CurrentControlSet\Services\zapret' -Name 'zapret-discord-youtube' -ErrorAction Stop
        return [string]$item.'zapret-discord-youtube'
    } catch {
        return ''
    }
}

function Get-IpsetStatus {
    $listFile = Join-Path $script:ZapretListsDir 'ipset-all.txt'
    if (-not (Test-Path -LiteralPath $listFile)) { return 'none' }
    $lines = @(Get-Content -LiteralPath $listFile -ErrorAction SilentlyContinue)
    if ($lines.Count -eq 0) { return 'any' }
    foreach ($line in $lines) {
        if ([string]$line -like '*203.0.113.113/32*') { return 'none' }
    }
    return 'loaded'
}

function Wait-Pause {
    Write-Host ''
    Read-Host 'Press Enter to continue' | Out-Null
}

function Invoke-InstallService {
    $files = @(Get-ZapretStrategyFiles)
    if ($files.Count -eq 0) {
        Write-Bad 'No strategies found in the strategies folder.'
        Wait-Pause
        return
    }
    Write-Host 'Pick a strategy:'
    for ($i = 0; $i -lt $files.Count; $i++) {
        Write-Host ("  {0}. {1}" -f ($i + 1), $files[$i].BaseName)
    }
    Write-Host '  0. Cancel'
    $choice = Read-Host 'Input option'
    if ($choice -eq '0' -or [string]::IsNullOrWhiteSpace($choice)) { return }
    $idx = 0
    if (-not [int]::TryParse($choice, [ref]$idx) -or $idx -lt 1 -or $idx -gt $files.Count) {
        Write-Bad 'Invalid choice.'
        Wait-Pause
        return
    }
    $file = $files[$idx - 1]
    Write-Host "Installing $($file.BaseName)..."
    Get-Process -Name 'winws' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    $svc = Get-Service -Name 'zapret' -ErrorAction SilentlyContinue
    if ($svc) {
        & net.exe stop zapret 2>$null | Out-Null
        & sc.exe delete zapret | Out-Null
        Start-Sleep -Seconds 1
    }
    $env:NO_UPDATE_CHECK = '1'
    Start-ZapretStrategyFile -Path $file.FullName
    $ok = $false
    for ($t = 0; $t -lt 40; $t++) {
        Start-Sleep -Milliseconds 250
        if (@(Get-Process -Name 'winws' -ErrorAction SilentlyContinue).Count -gt 0) { $ok = $true; break }
    }
    if (-not $ok) {
        Write-Bad 'winws.exe did not start. The service was not installed.'
        Wait-Pause
        return
    }
    Start-Sleep -Milliseconds 400
    $proc = Get-CimInstance -ClassName Win32_Process -Filter "Name='winws.exe'" | Select-Object -First 1
    $commandLine = ConvertTo-ZapretServiceImagePath -CommandLine ([string]$proc.CommandLine) -ExecutablePath ([string]$proc.ExecutablePath)
    Get-Process -Name 'winws' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($commandLine)) {
        Write-Bad 'Failed to read the winws.exe command line.'
        Wait-Pause
        return
    }
    & sc.exe create zapret binPath= 'placeholder' DisplayName= 'zapret' start= auto | Out-Null
    $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey('SYSTEM\CurrentControlSet\Services\zapret', $true)
    if ($null -eq $key) {
        Write-Bad 'Cannot open the zapret service registry key for write.'
        Wait-Pause
        return
    }
    try {
        $key.SetValue('ImagePath', $commandLine, [Microsoft.Win32.RegistryValueKind]::ExpandString)
        $key.SetValue('zapret-discord-youtube', $file.BaseName, [Microsoft.Win32.RegistryValueKind]::String)
    } finally {
        $key.Close()
    }
    & sc.exe description zapret 'Zapret DPI bypass software' | Out-Null
    Start-Service -Name 'zapret' -ErrorAction SilentlyContinue
    Write-Ok "Installed service: $($file.BaseName)."
    Wait-Pause
}

function Invoke-RemoveServices {
    $svc = Get-Service -Name 'zapret' -ErrorAction SilentlyContinue
    if ($svc) {
        & net.exe stop zapret 2>$null | Out-Null
        & sc.exe delete zapret | Out-Null
    } else {
        Write-Host 'Service zapret is not installed.'
    }
    Get-Process -Name 'winws' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 400
    if (Get-Service -Name 'WinDivert' -ErrorAction SilentlyContinue) {
        & net.exe stop WinDivert 2>$null | Out-Null
        & sc.exe delete WinDivert | Out-Null
    }
    & net.exe stop WinDivert14 2>$null | Out-Null
    & sc.exe delete WinDivert14 2>$null | Out-Null
    Write-Ok 'Remove finished.'
    Wait-Pause
}

function Invoke-CheckStatus {
    $name = Get-InstalledStrategyName
    if ($name) { Write-Host "Service strategy: $name" }
    foreach ($svcName in @('zapret', 'WinDivert')) {
        $svc = Get-Service -Name $svcName -ErrorAction SilentlyContinue
        if ($svc -and $svc.Status -eq 'Running') {
            Write-Ok "$svcName service is RUNNING."
        } else {
            Write-Host "$svcName service is NOT running."
        }
    }
    $sys = Get-ChildItem -LiteralPath $script:ZapretBinDir -Filter '*.sys' -ErrorAction SilentlyContinue
    if (-not $sys) { Write-Bad 'WinDivert64.sys file NOT found.' }
    $winws = @(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)
    if ($winws.Count -gt 0) { Write-Ok 'Bypass (winws.exe) is RUNNING.' } else { Write-Bad 'Bypass (winws.exe) is NOT running.' }
    Wait-Pause
}

function Invoke-GameFilterMenu {
    Write-Host 'Select game filter mode:'
    Write-Host '  0. Disable'
    Write-Host '  1. TCP and UDP'
    Write-Host '  2. TCP only'
    Write-Host '  3. UDP only'
    $c = Read-Host 'Select option (0-3, default: 0)'
    if ([string]::IsNullOrWhiteSpace($c)) { $c = '0' }
    $flag = Join-Path $script:ZapretUtilsDir 'game_filter.enabled'
    switch ($c) {
        '0' { if (Test-Path -LiteralPath $flag) { Remove-Item -LiteralPath $flag -Force } }
        '1' { Set-Content -LiteralPath $flag -Value 'all' -Encoding ASCII }
        '2' { Set-Content -LiteralPath $flag -Value 'tcp' -Encoding ASCII }
        '3' { Set-Content -LiteralPath $flag -Value 'udp' -Encoding ASCII }
        default { Write-Bad 'Invalid choice.'; Wait-Pause; return }
    }
    Write-Warn 'Restart zapret to apply Game Filter. If a service is installed, run Install Service again.'
    Wait-Pause
}

function Invoke-IpsetMenu {
    $listFile = Join-Path $script:ZapretListsDir 'ipset-all.txt'
    $backupName = 'ipset-all.txt.backup'
    $backupFile = Join-Path $script:ZapretListsDir $backupName
    $current = Get-IpsetStatus
    $order = @('loaded', 'none', 'any')
    $next = $order[(($order.IndexOf($current) + 1) % 3)]
    if ($current -eq 'loaded') {
        if (Test-Path -LiteralPath $backupFile) { Remove-Item -LiteralPath $backupFile -Force }
        Rename-Item -LiteralPath $listFile -NewName $backupName
    }
    switch ($next) {
        'none' { [System.IO.File]::WriteAllText($listFile, "203.0.113.113/32`r`n") }
        'any' { [System.IO.File]::WriteAllText($listFile, '') }
        'loaded' {
            if (-not (Test-Path -LiteralPath $backupFile)) {
                Write-Bad 'No IPSet backup found. Update the list first.'
                Wait-Pause
                return
            }
            if (Test-Path -LiteralPath $listFile) { Remove-Item -LiteralPath $listFile -Force }
            Rename-Item -LiteralPath $backupFile -NewName 'ipset-all.txt'
        }
    }
    Write-Host "IPSet Filter is now: $next"
    Wait-Pause
}

function Invoke-AutoUpdateToggle {
    $flag = Join-Path $script:ZapretUtilsDir 'check_updates.enabled'
    if (Test-Path -LiteralPath $flag) {
        Remove-Item -LiteralPath $flag -Force
        Write-Host 'Auto-Update Check disabled.'
    } else {
        Set-Content -LiteralPath $flag -Value 'ENABLED' -Encoding ASCII
        Write-Host 'Auto-Update Check enabled.'
    }
    Wait-Pause
}

function Invoke-CheckUpdates {
    $url = 'https://raw.githubusercontent.com/Flowseal/zapret-discord-youtube/main/.service/version.txt'
    try {
        $remote = (Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 5).Content.Trim()
    } catch {
        Write-Warn 'Warning: failed to fetch the latest version.'
        Wait-Pause
        return
    }
    if ($remote -eq $script:ZapretLocalVersion) {
        Write-Ok "Latest version installed: $($script:ZapretLocalVersion)"
    } else {
        Write-Host "New version available: $remote"
        Start-Process "https://github.com/Flowseal/zapret-discord-youtube/releases/latest"
    }
    Wait-Pause
}

function Invoke-UpdateIpset {
    $listFile = Join-Path $script:ZapretListsDir 'ipset-all.txt'
    $url = 'https://raw.githubusercontent.com/Flowseal/zapret-discord-youtube/refs/heads/main/.service/ipset-service.txt'
    Write-Host 'Updating ipset-all...'
    try {
        $res = Invoke-WebRequest -Uri $url -TimeoutSec 10 -UseBasicParsing
        if ($res.StatusCode -eq 200) {
            [System.IO.File]::WriteAllText($listFile, $res.Content)
            Write-Ok 'Finished'
        } else {
            Write-Bad 'Update failed.'
        }
    } catch {
        Write-Bad $_.Exception.Message
    }
    Wait-Pause
}

function Invoke-UpdateHosts {
    $hostsFile = Join-Path $env:SystemRoot 'System32\drivers\etc\hosts'
    $hostsUrl = 'https://raw.githubusercontent.com/Flowseal/zapret-discord-youtube/refs/heads/main/.service/hosts'
    $tempFile = Join-Path $env:TEMP 'zapret_hosts.txt'
    try {
        $res = Invoke-WebRequest -Uri ($hostsUrl + '?t=' + [guid]::NewGuid().ToString()) -TimeoutSec 10 -UseBasicParsing
        [System.IO.File]::WriteAllText($tempFile, $res.Content)
    } catch {
        Write-Bad 'Failed to download hosts file from repository'
        Wait-Pause
        return
    }
    $lines = @(Get-Content -LiteralPath $tempFile | Where-Object { $_ -ne '' })
    $first = $lines[0]
    $last = $lines[$lines.Count - 1]
    $hostsText = [System.IO.File]::ReadAllText($hostsFile)
    $needs = ($hostsText -notlike "*$first*") -or ($hostsText -notlike "*$last*")
    if ($needs) {
        Write-Warn 'Hosts file needs to be updated. Copy content from the notepad window.'
        Start-Process notepad.exe $tempFile
        Start-Process explorer.exe -ArgumentList "/select,`"$hostsFile`""
    } else {
        Write-Ok 'Hosts file is up to date'
        Remove-Item -LiteralPath $tempFile -Force -ErrorAction SilentlyContinue
    }
    Wait-Pause
}

function Invoke-RunTests {
    $test = Join-Path $script:ZapretUtilsDir 'test zapret.ps1'
    Start-Process -FilePath 'powershell.exe' -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$test`""
    Wait-Pause
}

function Invoke-Diagnostics {
    Write-Ok ("Zapret is installed in: '{0}'" -f $script:ZapretRoot)
    $bfe = Get-Service -Name 'BFE' -ErrorAction SilentlyContinue
    if ($bfe -and $bfe.Status -eq 'Running') { Write-Ok 'Base Filtering Engine check passed' } else { Write-Bad '[X] Base Filtering Engine is not running.' }

    $proxyOn = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyEnable -ErrorAction SilentlyContinue).ProxyEnable
    if ($proxyOn -eq 1) {
        $server = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -Name ProxyServer -ErrorAction SilentlyContinue).ProxyServer
        Write-Warn "[?] System proxy is enabled: $server"
    } else {
        Write-Ok 'Proxy check passed'
    }

    Enable-ZapretTcpTimestamps
    Write-Ok 'TCP timestamps check passed'

    if (@(Get-Process -Name 'AdguardSvc' -ErrorAction SilentlyContinue).Count -gt 0) {
        Write-Bad '[X] Adguard process found.'
    } else { Write-Ok 'Adguard check passed' }

    $allSvc = @(Get-Service -ErrorAction SilentlyContinue)
    if ($allSvc | Where-Object { $_.DisplayName -match 'Killer' -or $_.Name -match 'Killer' }) {
        Write-Bad '[X] Killer services found.'
    } else { Write-Ok 'Killer check passed' }

    if (-not (Get-ChildItem -LiteralPath $script:ZapretBinDir -Filter '*.sys' -ErrorAction SilentlyContinue)) {
        Write-Bad 'WinDivert64.sys file NOT found.'
    }

    if ($script:ZapretRoot -match '[\u0400-\u04FF]') {
        Write-Warn '[?] The path contains Cyrillic characters. Move Zapret to C:\zapret if bypass fails.'
    } else { Write-Ok 'Cyrillic path check passed' }

    Write-Host ''
    $clear = Read-Host 'Clear Discord cache (Stable, PTB, Canary, Development)? (Y/N) (default: Y)'
    if ([string]::IsNullOrWhiteSpace($clear)) { $clear = 'Y' }
    if ($clear -eq 'Y' -or $clear -eq 'y') {
        Write-Host 'Close Discord and delete Cache / Code Cache / GPUCache under %APPDATA%\discord* if needed.'
        Write-Warn 'Automatic cache delete runs only for folders that exist.'
        foreach ($pair in @(
            @{ Exe = 'Discord.exe'; Dir = Join-Path $env:APPDATA 'discord' },
            @{ Exe = 'DiscordPTB.exe'; Dir = Join-Path $env:APPDATA 'discordptb' },
            @{ Exe = 'DiscordCanary.exe'; Dir = Join-Path $env:APPDATA 'discordcanary' },
            @{ Exe = 'DiscordDevelopment.exe'; Dir = Join-Path $env:APPDATA 'discorddevelopment' }
        )) {
            if (Test-Path -LiteralPath $pair.Dir) {
                Get-Process -Name ([IO.Path]::GetFileNameWithoutExtension($pair.Exe)) -ErrorAction SilentlyContinue |
                    Stop-Process -Force -ErrorAction SilentlyContinue
                foreach ($sub in @('Cache', 'Code Cache', 'GPUCache')) {
                    $p = Join-Path $pair.Dir $sub
                    if (Test-Path -LiteralPath $p) {
                        Remove-Item -LiteralPath $p -Recurse -Force -ErrorAction SilentlyContinue
                    }
                }
                Write-Ok ("Cleared cache folders in {0}" -f $pair.Dir)
            }
        }
    }
    Wait-Pause
}

function Invoke-ReplaceFakes {
    Write-Host 'Replace active fakes in the GUI is not required for daily use.'
    Write-Host 'Copy a .bin file from bin\ over ACTIVE_DISCORD_UDP.bin or ACTIVE_GAME_UDP.bin, then restart zapret.'
    Wait-Pause
}

while ($true) {
    $gf = Get-ZapretGameFilter
    $ipset = Get-IpsetStatus
    $upd = if (Test-Path -LiteralPath (Join-Path $script:ZapretUtilsDir 'check_updates.enabled')) { 'enabled' } else { 'disabled' }
    $installed = Get-InstalledStrategyName
    Clear-Host
    Write-Host "  ZAPRET SERVICE MANAGER v$($script:ZapretLocalVersion)"
    if ($installed) { Write-Host "  Strategy: $installed" }
    Write-Host '  ----------------------------------------'
    Write-Host '     1. Install Service'
    Write-Host '     2. Remove Services'
    Write-Host '     3. Check Status'
    Write-Host "     4. Game Filter         [$($gf.Status)]"
    Write-Host "     5. IPSet Filter        [$ipset]"
    Write-Host "     6. Auto-Update Check   [$upd]"
    Write-Host '     7. Replace active fakes'
    Write-Host '     8. Update IPSet List'
    Write-Host '     9. Update Hosts File'
    Write-Host '    10. Check for Updates'
    Write-Host '    11. Run Diagnostics'
    Write-Host '    12. Run Tests'
    Write-Host '     0. Exit'
    $choice = Read-Host '  Select option (0-12)'
    switch ($choice) {
        '1' { Invoke-InstallService }
        '2' { Invoke-RemoveServices }
        '3' { Invoke-CheckStatus }
        '4' { Invoke-GameFilterMenu }
        '5' { Invoke-IpsetMenu }
        '6' { Invoke-AutoUpdateToggle }
        '7' { Invoke-ReplaceFakes }
        '8' { Invoke-UpdateIpset }
        '9' { Invoke-UpdateHosts }
        '10' { Invoke-CheckUpdates }
        '11' { Invoke-Diagnostics }
        '12' { Invoke-RunTests }
        '0' { exit 0 }
        default { }
    }
}
