# Zapret Manager console (same actions as the GUI).
# Start: cli.bat service

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapman\Zapman.psd1')

Enable-ZapmanConsoleUtf8
Initialize-ZapmanUiLanguage | Out-Null

# Keep the -File path even when this script is dot-sourced from cli.ps1.
$script:ZapmanServiceEntryPath = Join-Path $PSScriptRoot 'service.ps1'

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Write-Ok([string]$Message) { Write-Host $Message -ForegroundColor Green }
function Write-Bad([string]$Message) { Write-Host $Message -ForegroundColor Red }
function Write-Warn([string]$Message) { Write-Host $Message -ForegroundColor Yellow }

function Wait-Pause {
    Write-Host ''
    Read-Host 'Press Enter to continue' | Out-Null
}

function Write-MenuSep {
    Write-Host '  ----------------------------------------'
}

function Write-MenuItem {
    param(
        [string]$Number,
        [string]$Text,
        [string]$Tag = ''
    )
    if ([string]::IsNullOrWhiteSpace($Tag)) {
        Write-Host ("    {0,2}. {1}" -f $Number, $Text)
        return
    }
    Write-Host ("    {0,2}. {1,-22} [{2}]" -f $Number, $Text, $Tag)
}

function Write-MenuSection {
    param([string]$Key)
    Write-MenuSep
    Write-Host ("  {0}" -f (Get-ZapmanUiString -Key $Key))
}

function Read-YesNo {
    param(
        [string]$Prompt,
        [bool]$DefaultYes = $false
    )
    $hint = ' (Y/N, default: N)'
    if ($DefaultYes) {
        $hint = ' (Y/N, default: Y)'
    }
    $raw = Read-Host ($Prompt + $hint)
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return $DefaultYes
    }
    if ($raw -eq 'Y' -or $raw -eq 'y') {
        return $true
    }
    return $false
}

function Select-ZapretStrategyFile {
    $files = @(Get-ZapretStrategyFiles)
    if ($files.Count -eq 0) {
        Write-Bad (Get-ZapmanUiString -Key 'NoStrategies')
        Wait-Pause
        return $null
    }
    $engine = Get-ZapretEngine
    Write-Host (Get-ZapmanUiString -Key 'PickStrategy')
    for ($i = 0; $i -lt $files.Count; $i++) {
        $ok = Test-ZapretStrategySupportsEngine -Path $files[$i].FullName -Engine $engine
        $line = "  {0}. {1}" -f ($i + 1), $files[$i].BaseName
        if ($ok) {
            Write-Host $line
        } else {
            Write-Host $line -ForegroundColor DarkGray
        }
    }
    Write-Host ("  {0}" -f (Get-ZapmanUiString -Key 'CancelItem'))
    $choice = Read-Host 'Input option'
    if ($choice -eq '0' -or [string]::IsNullOrWhiteSpace($choice)) {
        return $null
    }
    $idx = 0
    if (-not [int]::TryParse($choice, [ref]$idx) -or $idx -lt 1 -or $idx -gt $files.Count) {
        Write-Bad (Get-ZapmanUiString -Key 'InvalidChoice')
        Wait-Pause
        return $null
    }
    return $files[$idx - 1]
}

# Strategy: run, install, tests.

function Invoke-StrategyRun {
    $file = Select-ZapretStrategyFile
    if (-not $file) {
        return
    }
    $busy = $false
    if (Test-ZapretBypassRunning) {
        $busy = $true
    } else {
        $svc = Get-ZapretService
        if ($svc -and $svc.Status -eq 'Running') {
            $busy = $true
        }
    }
    if ($busy) {
        if (-not (Read-YesNo -Prompt (Get-ZapmanUiString -Key 'ConfirmRunOver') -DefaultYes $false)) {
            return
        }
    }
    try {
        Start-ZapretSelectedStrategy -File $file
        Write-Ok (Get-ZapmanUiString -Key 'RunDone' -FormatArgs @($file.BaseName))
    } catch {
        $errText = Get-ZapmanExceptionText $_
        Set-ZapmanLastError -Text $errText
        Write-Bad $errText
    }
    Wait-Pause
}

function Invoke-StrategyInstall {
    $file = Select-ZapretStrategyFile
    if (-not $file) {
        return
    }
    try {
        Install-ZapretService -File $file
        Write-Ok (Get-ZapmanUiString -Key 'InstallDone' -FormatArgs @($file.BaseName))
    } catch {
        $errText = Get-ZapmanExceptionText $_
        Set-ZapmanLastError -Text $errText
        Write-Bad $errText
    }
    Wait-Pause
}

function Invoke-StrategyTests {
    try {
        $snap = $null
        if (Get-ZapretService) {
            Write-Host (Get-ZapmanUiString -Key 'TestsNeedNoService')
            $snap = Suspend-ZapretServiceForTests
            Write-Ok (Get-ZapmanUiString -Key 'RemoveDone')
        }
        try {
            Invoke-ZapmanStrategyTests -AskType -AskNames
            Write-Host (Get-ZapmanUiString -Key 'TestsTitle')
        } finally {
            if ($snap -and $snap.File) {
                Restore-ZapretServiceAfterTests -Snapshot $snap
                Write-Ok (Get-ZapmanUiString -Key 'InstallDone' -FormatArgs @($snap.File.BaseName))
            }
        }
    } catch {
        Write-Bad $_.Exception.Message
    }
    Wait-Pause
}

function Invoke-StrategyNetworkReset {
    if (-not (Read-YesNo -Prompt (Get-ZapmanUiString -Key 'StratNoneConfirm') -DefaultYes $false)) {
        return
    }
    $res = Invoke-ZapmanNetworkReset
    Write-Host $res.Text
    if (-not $res.Ok) {
        Write-Bad (Get-ZapmanUiString -Key 'StratNoneFail')
        Wait-Pause
        return
    }
    Write-Ok (Get-ZapmanUiString -Key 'StratNoneDone')
    if (Read-YesNo -Prompt (Get-ZapmanUiString -Key 'StratNoneDoneReboot') -DefaultYes $false) {
        Restart-Computer -Force
        return
    }
    Write-Warn (Get-ZapmanUiString -Key 'StratNoneRebootLater')
    Wait-Pause
}

function Invoke-StrategyMenu {
    $files = @(Get-ZapretStrategyFiles)
    if ($files.Count -eq 0) {
        Write-Bad (Get-ZapmanUiString -Key 'NoStrategies')
        Wait-Pause
        return
    }
    while ($true) {
        Clear-Host
        $engine = Get-ZapretEngine
        Write-Host ("  {0}" -f (Get-ZapmanUiString -Key 'StratTitle'))
        Write-Host '  ----------------------------------------'
        Write-Host ("     {0}: {1}" -f (Get-ZapmanUiString -Key 'LblEngine'), $engine)
        Write-Host ("     1. {0}" -f (Get-ZapmanUiString -Key 'BtnInstall'))
        Write-Host ("     2. {0}" -f (Get-ZapmanUiString -Key 'BtnRunSelected'))
        Write-Host ("     3. {0}" -f (Get-ZapmanUiString -Key 'BtnTests'))
        Write-Host ("     4. {0}" -f (Get-ZapmanUiString -Key 'BtnStratNone'))
        Write-Host '     5. winws'
        Write-Host '     6. winws2'
        Write-Host ("     0. {0}" -f (Get-ZapmanUiString -Key 'BtnCancel'))
        $choice = Read-Host '  Select option (0-6)'
        switch ($choice) {
            '1' { Invoke-StrategyInstall; return }
            '2' { Invoke-StrategyRun; return }
            '3' { Invoke-StrategyTests; return }
            '4' { Invoke-StrategyNetworkReset; return }
            '5' { Set-ZapretEngine -Engine 'winws' | Out-Null }
            '6' { Set-ZapretEngine -Engine 'winws2' | Out-Null }
            '0' { return }
            default { }
        }
    }
}

# Service: start, stop, remove, status.

function Invoke-StartService {
    if (-not (Get-ZapretService)) {
        Write-Warn (Get-ZapmanUiString -Key 'BannerNoService')
        Wait-Pause
        return
    }
    try {
        Start-ZapretServiceIfInstalled
        $name = Get-ZapretInstalledStrategyName
        if ($name) {
            Write-Ok (Get-ZapmanUiString -Key 'StartDoneName' -FormatArgs @($name))
        } else {
            Write-Ok (Get-ZapmanUiString -Key 'StartDone')
        }
    } catch {
        $errText = Get-ZapmanExceptionText $_
        Set-ZapmanLastError -Text $errText
        Write-Bad $errText
    }
    Wait-Pause
}

function Invoke-Stop {
    try {
        Stop-ZapretBypass
        Write-Ok (Get-ZapmanUiString -Key 'StopDone')
    } catch {
        Write-Bad $_.Exception.Message
    }
    Wait-Pause
}

function Invoke-RemoveServices {
    try {
        Remove-ZapretServices
        Write-Ok (Get-ZapmanUiString -Key 'RemoveDone')
    } catch {
        Write-Bad $_.Exception.Message
    }
    Wait-Pause
}

function Invoke-CheckStatus {
    foreach ($line in @(Get-ZapmanStatusLines)) {
        if ($line -like '*NOT found*' -or $line -like '*is NOT running*') {
            Write-Bad $line
        } elseif ($line -like '*RUNNING*') {
            Write-Ok $line
        } else {
            Write-Host $line
        }
    }
    Wait-Pause
}

# Settings: Game Filter, IPSet, Auto-Update Check, tray icon, fakes.

function Invoke-GameFilterMenu {
    Write-Host (Get-ZapmanUiString -Key 'MenuGame')
    Write-Host '  0. disabled'
    Write-Host '  1. all'
    Write-Host '  2. tcp'
    Write-Host '  3. udp'
    $c = Read-Host 'Select option (0-3, default: 0)'
    if ([string]::IsNullOrWhiteSpace($c)) {
        $c = '0'
    }
    $mode = $null
    switch ($c) {
        '0' { $mode = 'disabled' }
        '1' { $mode = 'all' }
        '2' { $mode = 'tcp' }
        '3' { $mode = 'udp' }
        default {
            Write-Bad (Get-ZapmanUiString -Key 'InvalidChoice')
            Wait-Pause
            return
        }
    }
    Set-ZapretGameFilterMode -Mode $mode
    if (Get-ZapretService) {
        Write-Warn (Get-ZapmanUiString -Key 'FilterNeedInstall')
    } else {
        Write-Warn (Get-ZapmanUiString -Key 'FilterNeedRerun')
    }
    Wait-Pause
}

function Invoke-IpsetMenu {
    Write-Host (Get-ZapmanUiString -Key 'MenuIpset')
    Write-Host ("  Current: {0}" -f (Get-ZapretIpsetStatus))
    Write-Host '  1. none'
    Write-Host '  2. loaded'
    Write-Host '  3. any'
    Write-Host ("  {0}" -f (Get-ZapmanUiString -Key 'CancelItem'))
    $c = Read-Host 'Select option (1-3)'
    $mode = $null
    switch ($c) {
        '1' { $mode = 'none' }
        '2' { $mode = 'loaded' }
        '3' { $mode = 'any' }
        '0' { return }
        default {
            if ([string]::IsNullOrWhiteSpace($c)) {
                return
            }
            Write-Bad (Get-ZapmanUiString -Key 'InvalidChoice')
            Wait-Pause
            return
        }
    }
    try {
        Set-ZapretIpsetMode -Mode $mode
        Write-Host (Get-ZapmanUiString -Key 'IpsetNow' -FormatArgs @($mode))
        if (Get-ZapretService) {
            Write-Warn (Get-ZapmanUiString -Key 'FilterNeedRestart')
        } else {
            Write-Warn (Get-ZapmanUiString -Key 'FilterNeedRerun')
        }
    } catch {
        Write-Bad $_.Exception.Message
    }
    Wait-Pause
}

function Invoke-AutoUpdateToggle {
    $on = -not (Test-ZapmanAutoUpdateEnabled)
    Set-ZapmanAutoUpdateEnabled -Enabled $on
    if ($on) {
        Write-Host (Get-ZapmanUiString -Key 'AutoOn')
    } else {
        Write-Host (Get-ZapmanUiString -Key 'AutoOff')
    }
    Wait-Pause
}

function Invoke-TrayWatchToggle {
    $on = -not (Test-ZapmanTrayWatchEnabled)
    try {
        Set-ZapmanTrayWatchEnabled -Enabled $on
        if ($on) {
            Write-Host (Get-ZapmanUiString -Key 'TrayOn')
        } else {
            Write-Host (Get-ZapmanUiString -Key 'TrayOff')
        }
    } catch {
        Write-Bad $_.Exception.Message
    }
    Wait-Pause
}

# Tools: version, ipset download, hosts, diagnostics.

function Invoke-CheckVersion {
    try {
        $remote = Get-ZapretRemoteVersion
    } catch {
        Write-Warn (Get-ZapmanUiString -Key 'VersionFail')
        Wait-Pause
        return
    }
    $local = Get-ZapmanLocalVersion
    if ([string]::IsNullOrWhiteSpace($remote)) {
        Write-Warn (Get-ZapmanUiString -Key 'VersionFail')
        Wait-Pause
        return
    }
    if ($remote -eq $local) {
        Write-Ok (Get-ZapmanUiString -Key 'VersionLatest' -FormatArgs @($local))
    } else {
        $ask = Get-ZapmanUiString -Key 'VersionNew' -FormatArgs @($remote)
        if (Read-YesNo -Prompt $ask -DefaultYes $false) {
            Start-Process (Get-ZapretReleasePageUrl)
        }
    }
    Wait-Pause
}

function Invoke-UpdateIpset {
    Write-Host (Get-ZapmanUiString -Key 'IpsetTitle')
    try {
        Update-ZapretIpsetList
        Write-Ok (Get-ZapmanUiString -Key 'IpsetOk')
    } catch {
        Write-Bad $_.Exception.Message
    }
    Wait-Pause
}

function Invoke-CompareHosts {
    try {
        $info = Get-ZapretHostsUpdateInfo
    } catch {
        Write-Bad (Get-ZapmanUiString -Key 'HostsFail')
        Wait-Pause
        return
    }
    if ($info.NeedsUpdate) {
        Write-Warn (Get-ZapmanUiString -Key 'HostsNeed')
        while ($true) {
            Write-MenuItem -Number '1' -Text (Get-ZapmanUiString -Key 'BtnHostsCopy')
            Write-MenuItem -Number '2' -Text (Get-ZapmanUiString -Key 'BtnHostsOpen')
            Write-MenuItem -Number '0' -Text (Get-ZapmanUiString -Key 'BtnClose')
            $pick = Read-Host '  Select option (0-2)'
            if ($pick -eq '1') {
                try {
                    Copy-ZapretHostsTemplate -Info $info
                    Write-Ok (Get-ZapmanUiString -Key 'HostsCopied')
                } catch {
                    Write-Bad $_.Exception.Message
                }
            } elseif ($pick -eq '2') {
                try {
                    Open-ZapretSystemHosts -Info $info
                } catch {
                    Write-Bad $_.Exception.Message
                }
            } elseif ($pick -eq '0') {
                break
            }
        }
    } else {
        Write-Ok (Get-ZapmanUiString -Key 'HostsOk')
        Remove-Item -LiteralPath $info.TempFile -Force -ErrorAction SilentlyContinue
    }
    Wait-Pause
}

function Invoke-Diagnostics {
    $report = Get-ZapmanDiagnosticReport
    foreach ($item in @($report.Items)) {
        $mark = '[OK]'
        if ($item.Status -eq 'fail') {
            $mark = '[X]'
        } elseif ($item.Status -eq 'warn') {
            $mark = '[!]'
        }
        $line = '{0} {1}' -f $mark, $item.Text
        if ($item.Status -eq 'ok') {
            Write-Ok $line
        } elseif ($item.Status -eq 'fail') {
            Write-Bad $line
        } else {
            Write-Warn $line
        }
    }
    foreach ($item in @($report.Items)) {
        if ([string]::IsNullOrWhiteSpace([string]$item.Action)) {
            continue
        }
        $ask = $item.Text
        $yes = ($item.Action -eq 'discord-cache')
        if ($item.Action -eq 'remove-services') {
            $ask = Get-ZapmanUiString -Key 'DiagConflicts'
        } elseif ($item.Action -eq 'discord-cache') {
            $ask = Get-ZapmanUiString -Key 'DiagCache'
        }
        if (-not (Read-YesNo -Prompt $ask -DefaultYes $yes)) {
            continue
        }
        foreach ($msg in @(Invoke-ZapmanDiagnosticAction -Action ([string]$item.Action) -Names @($item.ActionNames))) {
            if ($msg -like 'OK:*') {
                Write-Ok $msg
            } elseif ($msg -like 'FAIL:*') {
                Write-Bad $msg
            } else {
                Write-Warn $msg
            }
        }
    }
    Write-Host (Get-ZapmanUiString -Key 'DiagDone')
    Wait-Pause
}

function Read-FakeSlotFileIndex {
    param(
        [string]$SlotLabel,
        [int]$FileCount
    )
    $raw = Read-Host (Get-ZapmanUiString -Key 'FakesPick' -FormatArgs @($SlotLabel))
    if ([string]::IsNullOrWhiteSpace($raw) -or $raw -eq '0') {
        return 0
    }
    $idx = 0
    if (-not [int]::TryParse($raw.Trim(), [ref]$idx) -or $idx -lt 1 -or $idx -gt $FileCount) {
        Write-Bad (Get-ZapmanUiString -Key 'InvalidChoice')
        return $null
    }
    return $idx
}

function Invoke-ReplaceFakes {
    try {
        $catalog = Get-ZapretFakeCatalog
    } catch {
        Write-Bad $_.Exception.Message
        Wait-Pause
        return
    }
    $files = @($catalog.Files)
    if ($files.Count -eq 0) {
        Write-Bad (Get-ZapmanUiString -Key 'FakesNone')
        Wait-Pause
        return
    }
    $discordLabel = Get-ZapmanUiString -Key 'FakesDiscord'
    $gameLabel = Get-ZapmanUiString -Key 'FakesGame'
    Write-Host (Get-ZapmanUiString -Key 'FakesTitle')
    Write-Host ("  {0}: {1}" -f $discordLabel, (Get-ZapretFakeCurrentText -Name ([string]$catalog.CurrentDiscord) -State ([string]$catalog.DiscordState)))
    Write-Host ("  {0}: {1}" -f $gameLabel, (Get-ZapretFakeCurrentText -Name ([string]$catalog.CurrentGame) -State ([string]$catalog.GameState)))
    for ($i = 0; $i -lt $files.Count; $i++) {
        Write-Host ("  {0}. {1}" -f ($i + 1), $files[$i].Name)
    }
    $discordIdx = Read-FakeSlotFileIndex -SlotLabel $discordLabel -FileCount $files.Count
    if ($null -eq $discordIdx) {
        Wait-Pause
        return
    }
    $gameIdx = Read-FakeSlotFileIndex -SlotLabel $gameLabel -FileCount $files.Count
    if ($null -eq $gameIdx) {
        Wait-Pause
        return
    }
    $picks = New-Object System.Collections.ArrayList
    if ($discordIdx -gt 0) {
        $picked = $files[$discordIdx - 1]
        if ($catalog.DiscordState -ne 'matched' -or $picked.Name -ne [string]$catalog.CurrentDiscord) {
            [void]$picks.Add((New-Object PSObject -Property @{ Slot = 'discord'; File = $picked }))
        }
    }
    if ($gameIdx -gt 0) {
        $picked = $files[$gameIdx - 1]
        if ($catalog.GameState -ne 'matched' -or $picked.Name -ne [string]$catalog.CurrentGame) {
            [void]$picks.Add((New-Object PSObject -Property @{ Slot = 'game'; File = $picked }))
        }
    }
    $applied = 0
    foreach ($item in @($picks)) {
        try {
            Set-ZapretActiveFake -Slot $item.Slot -SourcePath $item.File.FullName
            Write-Ok (Get-ZapmanUiString -Key 'FakeDone' -FormatArgs @($item.Slot, $item.File.Name))
            $applied++
        } catch {
            Write-Bad $_.Exception.Message
        }
    }
    if ($applied -gt 0) {
        if (Get-ZapretService) {
            Write-Warn (Get-ZapmanUiString -Key 'FakeNeedRestart')
        } else {
            Write-Warn (Get-ZapmanUiString -Key 'FakeNeedRerun')
        }
    }
    Wait-Pause
}

function Invoke-ZapmanServiceMenu {
    while ($true) {
        $gf = Get-ZapretGameFilter
        $ipset = Get-ZapretIpsetStatus
        $upd = 'off'
        if (Test-ZapmanAutoUpdateEnabled) {
            $upd = 'on'
        }
        $tray = 'off'
        if (Test-ZapmanTrayWatchEnabled) {
            $tray = 'on'
        }
        $installed = Get-ZapretInstalledStrategyName
        $runningName = Get-ZapretRunningStrategyName
        $svc = Get-ZapretService
        $svcRunning = $false
        if ($svc -and $svc.Status -eq 'Running') {
            $svcRunning = $true
        }
        Clear-Host
        Write-Host ("  {0}" -f (Get-ZapmanUiString -Key 'AppTitle' -FormatArgs @(Get-ZapmanLocalVersion)))
        Write-MenuSection -Key 'GrpStatus'
        if (Test-ZapretBypassRunning) {
            Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusBypassOn' -FormatArgs @(Get-ZapretStatusBypassName)))
        } else {
            Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusBypassOff'))
        }
        if ($svc) {
            Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusServiceOn' -FormatArgs @($svc.Status)))
        } else {
            Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusServiceOff'))
        }
        if ($runningName) {
            if ($svcRunning) {
                Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusStrategy' -FormatArgs @($runningName)))
            } else {
                Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusStrategyRun' -FormatArgs @($runningName)))
            }
        } elseif ($installed) {
            Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusStrategy' -FormatArgs @($installed)))
        } else {
            Write-Host ("    {0}" -f (Get-ZapmanUiString -Key 'StatusStrategyNone'))
        }
        Write-MenuSep
        Write-MenuItem -Number '1' -Text (Get-ZapmanUiString -Key 'MenuStrategy')
        Write-MenuItem -Number '2' -Text (Get-ZapmanUiString -Key 'MenuStart')
        Write-MenuItem -Number '3' -Text (Get-ZapmanUiString -Key 'MenuStop')
        Write-MenuItem -Number '4' -Text (Get-ZapmanUiString -Key 'MenuRemove')
        Write-MenuItem -Number '5' -Text (Get-ZapmanUiString -Key 'MenuStatus')
        Write-MenuSection -Key 'GrpSettings'
        Write-MenuItem -Number '6' -Text (Get-ZapmanUiString -Key 'MenuGame') -Tag $gf.Status
        Write-MenuItem -Number '7' -Text (Get-ZapmanUiString -Key 'MenuIpset') -Tag $ipset
        Write-MenuItem -Number '8' -Text (Get-ZapmanUiString -Key 'MenuAuto') -Tag $upd
        Write-MenuItem -Number '9' -Text (Get-ZapmanUiString -Key 'MenuTray') -Tag $tray
        Write-MenuItem -Number '10' -Text (Get-ZapmanUiString -Key 'MenuFakes')
        Write-MenuSection -Key 'GrpTools'
        Write-MenuItem -Number '11' -Text (Get-ZapmanUiString -Key 'MenuIpsetDl')
        Write-MenuItem -Number '12' -Text (Get-ZapmanUiString -Key 'MenuHosts')
        Write-MenuItem -Number '13' -Text (Get-ZapmanUiString -Key 'MenuVersion')
        Write-MenuItem -Number '14' -Text (Get-ZapmanUiString -Key 'MenuDiag')
        Write-MenuSep
        Write-MenuItem -Number '0' -Text (Get-ZapmanUiString -Key 'MenuExit')
        $choice = Read-Host '  Select option (0-14)'
        switch ($choice) {
            '1' { Invoke-StrategyMenu }
            '2' { Invoke-StartService }
            '3' { Invoke-Stop }
            '4' { Invoke-RemoveServices }
            '5' { Invoke-CheckStatus }
            '6' { Invoke-GameFilterMenu }
            '7' { Invoke-IpsetMenu }
            '8' { Invoke-AutoUpdateToggle }
            '9' { Invoke-TrayWatchToggle }
            '10' { Invoke-ReplaceFakes }
            '11' { Invoke-UpdateIpset }
            '12' { Invoke-CompareHosts }
            '13' { Invoke-CheckVersion }
            '14' { Invoke-Diagnostics }
            '0' { return 0 }
            default { }
        }
    }
}

function Start-ZapmanServiceConsole {
    if (-not (Test-IsAdministrator)) {
        $argList = "-NoProfile -NoLogo -ExecutionPolicy Bypass -File `"$script:ZapmanServiceEntryPath`""
        try {
            Start-Process -FilePath 'powershell.exe' -ArgumentList $argList -Verb RunAs | Out-Null
        } catch {
            Write-Host (Get-ZapmanUiString -Key 'AdminRequired') -ForegroundColor Red
            return 1
        }
        return 0
    }

    Initialize-ZapmanUserLists
    Enable-ZapmanTcpTimestamps
    return (Invoke-ZapmanServiceMenu)
}

if ($MyInvocation.InvocationName -eq '.') {
    return
}

exit (Start-ZapmanServiceConsole)

