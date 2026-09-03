# Zapret WinForms GUI.
# This script starts and stops strategies from the strategies folder.
# This script installs or removes the zapret Windows service.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

$script:rootDir = Split-Path -Parent $PSScriptRoot
$script:listsDir = Join-Path $script:rootDir 'lists'
$script:utilsDir = Join-Path $script:rootDir 'utils'
$script:binDir = Join-Path $script:rootDir 'bin'
$script:strategiesDir = Join-Path $script:rootDir 'strategies'
$script:updatingSettings = $false
$script:busy = $false
$script:strategyMap = @{}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Request-Administrator {
    $argList = "-NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$PSCommandPath`""
    try {
        Start-Process -FilePath 'powershell.exe' -ArgumentList $argList -Verb RunAs | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Hide-ConsoleWindow {
    $code = @'
using System;
using System.Runtime.InteropServices;
public static class GuiConsole {
    [DllImport("kernel32.dll")] public static extern IntPtr GetConsoleWindow();
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
}
'@
    try {
        if (-not ('GuiConsole' -as [type])) {
            Add-Type -TypeDefinition $code -ErrorAction Stop
        }
        $hwnd = [GuiConsole]::GetConsoleWindow()
        if ($hwnd -ne [IntPtr]::Zero) {
            [void][GuiConsole]::ShowWindow($hwnd, 0)
        }
    } catch {
        $null = $_
    }
}

function Show-ErrorDialog {
    param([string]$Message)
    [System.Windows.Forms.MessageBox]::Show(
        $Message,
        'Zapret',
        [System.Windows.Forms.MessageBoxButtons]::OK,
        [System.Windows.Forms.MessageBoxIcon]::Error
    ) | Out-Null
}

function Show-QuestionDialog {
    param([string]$Message)
    return [System.Windows.Forms.MessageBox]::Show(
        $Message,
        'Zapret',
        [System.Windows.Forms.MessageBoxButtons]::YesNo,
        [System.Windows.Forms.MessageBoxIcon]::Question
    )
}

function Invoke-GuiPump {
    [System.Windows.Forms.Application]::DoEvents()
}

function Get-LocalVersion {
    $engine = Join-Path $script:utilsDir 'engine.ps1'
    if (-not (Test-Path -LiteralPath $engine)) {
        return 'unknown'
    }
    $match = Select-String -LiteralPath $engine -Pattern "ZapretLocalVersion\s*=\s*'([^']+)'" | Select-Object -First 1
    if ($match) {
        return $match.Matches[0].Groups[1].Value
    }
    return 'unknown'
}

function Initialize-UserLists {
    if (-not (Test-Path -LiteralPath $script:listsDir)) {
        New-Item -ItemType Directory -Path $script:listsDir | Out-Null
    }

    $ipsetExcludeUser = Join-Path $script:listsDir 'ipset-exclude-user.txt'
    if (-not (Test-Path -LiteralPath $ipsetExcludeUser)) {
        Set-Content -LiteralPath $ipsetExcludeUser -Value '203.0.113.113/32' -Encoding ASCII
    }

    $listGeneralUser = Join-Path $script:listsDir 'list-general-user.txt'
    if (-not (Test-Path -LiteralPath $listGeneralUser)) {
        Set-Content -LiteralPath $listGeneralUser -Value "# Never leave this file empty`r`ndomain.example.abc" -Encoding ASCII
    }

    $listExcludeUser = Join-Path $script:listsDir 'list-exclude-user.txt'
    if (-not (Test-Path -LiteralPath $listExcludeUser)) {
        Set-Content -LiteralPath $listExcludeUser -Value 'domain.example.abc' -Encoding ASCII
    }
}

function Enable-TcpTimestamps {
    $show = netsh interface tcp show global 2>$null | Out-String
    if ($show -match '(?i)timestamps[^\r\n]*enabled') {
        return
    }
    netsh interface tcp set global timestamps=enabled | Out-Null
}

function Get-StrategyFiles {
    if (-not (Test-Path -LiteralPath $script:strategiesDir)) {
        return @()
    }
    return @(
        Get-ChildItem -LiteralPath $script:strategiesDir -Filter '*.ps1' |
            Sort-Object { [Regex]::Replace($_.Name, '(\d+)', { $args[0].Value.PadLeft(8, '0') }) }
    )
}

function Test-BypassRunning {
    return (@(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)).Count -gt 0
}

function Get-ZapretService {
    return Get-Service -Name 'zapret' -ErrorAction SilentlyContinue
}

function Test-ServiceExists {
    param([string]$Name)
    $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
    return [bool]$svc
}

function Get-InstalledStrategyName {
    $path = 'HKLM:\System\CurrentControlSet\Services\zapret'
    try {
        $item = Get-ItemProperty -LiteralPath $path -Name 'zapret-discord-youtube' -ErrorAction Stop
        return [string]$item.'zapret-discord-youtube'
    } catch {
        return ''
    }
}

function Get-GameFilterMode {
    $flagFile = Join-Path $script:utilsDir 'game_filter.enabled'
    if (-not (Test-Path -LiteralPath $flagFile)) {
        return 'disabled'
    }
    $mode = [string](Get-Content -LiteralPath $flagFile -TotalCount 1 -ErrorAction SilentlyContinue)
    if ([string]::IsNullOrWhiteSpace($mode)) {
        return 'udp'
    }
    $mode = $mode.Trim().ToLowerInvariant()
    if ($mode -eq 'all') { return 'all' }
    if ($mode -eq 'tcp') { return 'tcp' }
    return 'udp'
}

function Set-GameFilterMode {
    param([string]$Mode)
    $flagFile = Join-Path $script:utilsDir 'game_filter.enabled'
    if ($Mode -eq 'disabled') {
        if (Test-Path -LiteralPath $flagFile) {
            Remove-Item -LiteralPath $flagFile -Force
        }
        return
    }
    $value = switch ($Mode) {
        'all' { 'all' }
        'tcp' { 'tcp' }
        default { 'udp' }
    }
    Set-Content -LiteralPath $flagFile -Value $value -Encoding ASCII
}

function Get-IpsetStatus {
    $listFile = Join-Path $script:listsDir 'ipset-all.txt'
    if (-not (Test-Path -LiteralPath $listFile)) {
        return 'none'
    }
    # Empty file = any. Dummy IP = none. Other content = loaded.
    $lines = @(Get-Content -LiteralPath $listFile -ErrorAction SilentlyContinue)
    if ($lines.Count -eq 0) {
        return 'any'
    }
    foreach ($line in $lines) {
        if ([string]$line -like '*203.0.113.113/32*') {
            return 'none'
        }
    }
    return 'loaded'
}

function Set-IpsetMode {
    param([string]$Mode)

    $listFile = Join-Path $script:listsDir 'ipset-all.txt'
    $backupName = 'ipset-all.txt.backup'
    $backupFile = Join-Path $script:listsDir $backupName
    $current = Get-IpsetStatus
    if ($current -eq $Mode) {
        return
    }

    # Move the loaded list to the backup file.
    if ($current -eq 'loaded') {
        if (Test-Path -LiteralPath $backupFile) {
            Remove-Item -LiteralPath $backupFile -Force
        }
        Rename-Item -LiteralPath $listFile -NewName $backupName
    }

    switch ($Mode) {
        'none' {
            [System.IO.File]::WriteAllText($listFile, "203.0.113.113/32`r`n")
        }
        'any' {
            [System.IO.File]::WriteAllText($listFile, '')
        }
        'loaded' {
            if (-not (Test-Path -LiteralPath $backupFile)) {
                throw 'No IPSet backup found. Use service.ps1 -> Update IPSet List first.'
            }
            if (Test-Path -LiteralPath $listFile) {
                Remove-Item -LiteralPath $listFile -Force
            }
            Rename-Item -LiteralPath $backupFile -NewName 'ipset-all.txt'
        }
        default {
            throw "Unknown IPSet mode: $Mode"
        }
    }
}

function Stop-WinwsProcess {
    $procs = @(Get-Process -Name 'winws' -ErrorAction SilentlyContinue)
    if ($procs.Count -eq 0) {
        return
    }
    $procs | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 400
}

function Wait-UntilServiceStopped {
    param(
        [string]$Name,
        [int]$TimeoutSeconds = 8
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        Invoke-GuiPump
        $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
        if (-not $svc -or $svc.Status -eq 'Stopped') {
            return $true
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Wait-UntilServiceGone {
    param(
        [string]$Name,
        [int]$TimeoutSeconds = 8
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        Invoke-GuiPump
        if (-not (Test-ServiceExists -Name $Name)) {
            return $true
        }
        Start-Sleep -Milliseconds 400
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Stop-Bypass {
    $svc = Get-ZapretService
    if ($svc -and $svc.Status -ne 'Stopped') {
        try {
            Stop-Service -Name 'zapret' -Force -ErrorAction SilentlyContinue
        } catch {
            $null = $_
        }
        & net.exe stop zapret 2>$null | Out-Null
        [void](Wait-UntilServiceStopped -Name 'zapret' -TimeoutSeconds 8)
    }

    Stop-WinwsProcess
}

function Wait-ForWinws {
    param(
        [int]$TimeoutSeconds = 12,
        [switch]$RequireCommandLine
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        Invoke-GuiPump
        Start-Sleep -Milliseconds 250
        if (-not (Test-BypassRunning)) {
            continue
        }
        Start-Sleep -Milliseconds 300
        Invoke-GuiPump
        if (-not (Test-BypassRunning)) {
            continue
        }
        if ($RequireCommandLine) {
            $cmd = Get-WinwsCommandLine
            if ([string]::IsNullOrWhiteSpace($cmd)) {
                continue
            }
        }
        return $true
    } while ((Get-Date) -lt $deadline)
    return $false
}

function ConvertTo-ServiceImagePath {
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

function Get-WinwsCommandLine {
    try {
        $proc = Get-CimInstance -ClassName Win32_Process -Filter "Name='winws.exe'" -ErrorAction Stop |
            Select-Object -First 1
        if (-not $proc) {
            return ''
        }
        return ConvertTo-ServiceImagePath -CommandLine ([string]$proc.CommandLine) -ExecutablePath ([string]$proc.ExecutablePath)
    } catch {
        $null = $_
    }
    return ''
}

function Remove-ZapretServiceRecord {
    if (-not (Test-ServiceExists -Name 'zapret')) {
        return
    }
    & net.exe stop zapret 2>$null | Out-Null
    $deleteOut = & sc.exe delete zapret 2>&1
    if (-not (Wait-UntilServiceGone -Name 'zapret' -TimeoutSeconds 8)) {
        throw "Failed to delete the existing zapret service. $deleteOut"
    }
}

function Start-SelectedStrategy {
    param([System.IO.FileInfo]$File)

    if (-not $File) {
        throw 'Select a strategy first.'
    }

    $svc = Get-ZapretService
    if ($svc) {
        if ($svc.Status -eq 'Running') {
            throw 'The zapret service is already running. Stop it or remove the service first.'
        }
        throw 'The zapret service is installed. Start the service, or remove it to run a strategy.'
    }

    if (Test-BypassRunning) {
        Stop-Bypass
    }

    Enable-TcpTimestamps
    $env:NO_UPDATE_CHECK = '1'
    Start-Process -FilePath 'powershell.exe' -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$($File.FullName)`"" -WorkingDirectory $script:rootDir -WindowStyle Minimized | Out-Null

    if (-not (Wait-ForWinws -TimeoutSeconds 12 -RequireCommandLine)) {
        throw 'winws.exe did not start. Check the bin folder and antivirus exclusions.'
    }
}

function Start-ZapretServiceIfInstalled {
    $svc = Get-ZapretService
    if (-not $svc) {
        throw 'The zapret service is not installed.'
    }
    if ($svc.Status -eq 'Running') {
        return
    }
    Enable-TcpTimestamps
    Start-Service -Name 'zapret' -ErrorAction Stop
    if (-not (Wait-ForWinws -TimeoutSeconds 12 -RequireCommandLine)) {
        throw 'The service started, but winws.exe is not running.'
    }
}

function Install-ZapretService {
    param([System.IO.FileInfo]$File)

    if (-not $File) {
        throw 'Select a strategy first.'
    }

    Stop-Bypass
    Remove-ZapretServiceRecord

    Enable-TcpTimestamps
    $env:NO_UPDATE_CHECK = '1'
    Start-Process -FilePath 'powershell.exe' -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$($File.FullName)`"" -WorkingDirectory $script:rootDir -WindowStyle Minimized | Out-Null

    if (-not (Wait-ForWinws -TimeoutSeconds 15 -RequireCommandLine)) {
        throw 'winws.exe did not start. The service was not installed.'
    }

    $commandLine = Get-WinwsCommandLine
    Stop-Bypass

    if ([string]::IsNullOrWhiteSpace($commandLine)) {
        throw 'Failed to read the winws.exe command line.'
    }

    $created = $false
    try {
        $createOut = $null
        for ($try = 1; $try -le 8; $try++) {
            $createOut = & sc.exe create zapret binPath= 'placeholder' DisplayName= 'zapret' start= auto 2>&1
            if ($LASTEXITCODE -eq 0) {
                $created = $true
                break
            }
            Start-Sleep -Milliseconds 400
            Invoke-GuiPump
        }
        if (-not $created) {
            throw "Failed to create the service. $createOut"
        }

        $writableKey = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
            'SYSTEM\CurrentControlSet\Services\zapret',
            $true
        )
        if ($null -eq $writableKey) {
            throw 'Cannot open the zapret service registry key for write.'
        }
        try {
            $writableKey.SetValue('ImagePath', $commandLine, [Microsoft.Win32.RegistryValueKind]::ExpandString)
            $writableKey.SetValue('zapret-discord-youtube', $File.BaseName, [Microsoft.Win32.RegistryValueKind]::String)
        } finally {
            $writableKey.Close()
        }

        $verifyKey = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
            'SYSTEM\CurrentControlSet\Services\zapret',
            $false
        )
        if ($null -eq $verifyKey) {
            throw 'Cannot read the zapret service registry key.'
        }
        try {
            $written = [string]$verifyKey.GetValue(
                'ImagePath',
                '',
                [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames
            )
            $writtenName = [string]$verifyKey.GetValue('zapret-discord-youtube')
        } finally {
            $verifyKey.Close()
        }
        if ($written -ne $commandLine) {
            throw 'Failed to write the service ImagePath.'
        }
        if ($writtenName -ne $File.BaseName) {
            throw 'Failed to write the installed strategy name.'
        }

        & sc.exe description zapret 'Zapret DPI bypass software' | Out-Null

        Start-Service -Name 'zapret' -ErrorAction Stop
        if (-not (Wait-ForWinws -TimeoutSeconds 12 -RequireCommandLine)) {
            throw 'The service was created, but winws.exe did not start.'
        }
    } catch {
        if ($created -or (Test-ServiceExists -Name 'zapret')) {
            & net.exe stop zapret 2>$null | Out-Null
            & sc.exe delete zapret 2>$null | Out-Null
        }
        throw
    }
}

function Remove-ZapretServices {
    # Stop zapret, stop winws, then stop WinDivert. Do not stop the driver first.
    if (Test-ServiceExists -Name 'zapret') {
        & net.exe stop zapret 2>$null | Out-Null
        & sc.exe delete zapret | Out-Null
        [void](Wait-UntilServiceGone -Name 'zapret' -TimeoutSeconds 8)
    }

    Stop-WinwsProcess

    if (Test-ServiceExists -Name 'WinDivert') {
        & net.exe stop WinDivert 2>$null | Out-Null
        if (Test-ServiceExists -Name 'WinDivert') {
            & sc.exe delete WinDivert | Out-Null
        }
    }

    & net.exe stop WinDivert14 2>$null | Out-Null
    & sc.exe delete WinDivert14 2>$null | Out-Null

    [void](Wait-UntilServiceGone -Name 'WinDivert' -TimeoutSeconds 5)
    [void](Wait-UntilServiceGone -Name 'WinDivert14' -TimeoutSeconds 5)

    $left = New-Object System.Collections.Generic.List[string]
    foreach ($name in @('zapret', 'WinDivert', 'WinDivert14')) {
        if (Test-ServiceExists -Name $name) {
            [void]$left.Add($name)
        }
    }
    if (Test-BypassRunning) {
        [void]$left.Add('winws.exe')
    }
    if ($left.Count -gt 0) {
        throw "Failed to remove: $($left -join ', ')"
    }
}

function Get-SelectedStrategy {
    param($ListBox)
    $name = [string]$ListBox.SelectedItem
    if ([string]::IsNullOrWhiteSpace($name)) {
        return $null
    }
    if (-not $script:strategyMap.ContainsKey($name)) {
        return $null
    }
    return $script:strategyMap[$name]
}

function Get-GameFilterApplyHint {
    if (Get-ZapretService) {
        return 'Game Filter saved. Run Install Service again to apply.'
    }
    return 'Game Filter saved. Stop and Start to apply.'
}

function Get-IpsetApplyHint {
    return 'IPSet Filter saved. Stop and Start to apply.'
}

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

if (-not (Test-IsAdministrator)) {
    if (-not (Request-Administrator)) {
        Add-Type -AssemblyName System.Windows.Forms
        Show-ErrorDialog 'Administrator rights are required.'
        exit 1
    }
    exit 0
}

Hide-ConsoleWindow

if (-not (Test-Path -LiteralPath $script:binDir)) {
    Show-ErrorDialog 'The bin folder is not found. Extract the full Zapret archive first.'
    exit 1
}

Initialize-UserLists

$version = Get-LocalVersion
$strategies = @(Get-StrategyFiles)
$script:strategyMap = @{}

$form = New-Object System.Windows.Forms.Form
$form.Text = "Zapret Manager v$version"
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedSingle'
$form.MaximizeBox = $false
$form.MinimizeBox = $true
$form.ClientSize = New-Object System.Drawing.Size(460, 528)
$form.Font = New-Object System.Drawing.Font('Segoe UI', 9)

$title = New-Object System.Windows.Forms.Label
$title.Text = 'Zapret'
$title.Font = New-Object System.Drawing.Font('Segoe UI', 14, [System.Drawing.FontStyle]::Bold)
$title.Location = New-Object System.Drawing.Point(16, 12)
$title.AutoSize = $true
$form.Controls.Add($title)

$lblStrategy = New-Object System.Windows.Forms.Label
$lblStrategy.Text = 'Strategy'
$lblStrategy.Location = New-Object System.Drawing.Point(16, 48)
$lblStrategy.AutoSize = $true
$form.Controls.Add($lblStrategy)

$list = New-Object System.Windows.Forms.ListBox
$list.Location = New-Object System.Drawing.Point(16, 70)
$list.Size = New-Object System.Drawing.Size(428, 176)
foreach ($file in $strategies) {
    [void]$list.Items.Add($file.BaseName)
    $script:strategyMap[$file.BaseName] = $file
}
$form.Controls.Add($list)

$grpStatus = New-Object System.Windows.Forms.GroupBox
$grpStatus.Text = 'Status'
$grpStatus.Location = New-Object System.Drawing.Point(16, 254)
$grpStatus.Size = New-Object System.Drawing.Size(428, 88)
$form.Controls.Add($grpStatus)

$lblBypass = New-Object System.Windows.Forms.Label
$lblBypass.Location = New-Object System.Drawing.Point(12, 22)
$lblBypass.Size = New-Object System.Drawing.Size(400, 20)
$lblBypass.Text = 'Bypass: ...'
$grpStatus.Controls.Add($lblBypass)

$lblService = New-Object System.Windows.Forms.Label
$lblService.Location = New-Object System.Drawing.Point(12, 44)
$lblService.Size = New-Object System.Drawing.Size(400, 20)
$lblService.Text = 'Service: ...'
$grpStatus.Controls.Add($lblService)

$lblInstalled = New-Object System.Windows.Forms.Label
$lblInstalled.Location = New-Object System.Drawing.Point(12, 66)
$lblInstalled.Size = New-Object System.Drawing.Size(400, 18)
$lblInstalled.Text = 'Installed strategy: ...'
$grpStatus.Controls.Add($lblInstalled)

$grpSettings = New-Object System.Windows.Forms.GroupBox
$grpSettings.Text = 'Settings'
$grpSettings.Location = New-Object System.Drawing.Point(16, 348)
$grpSettings.Size = New-Object System.Drawing.Size(428, 58)
$form.Controls.Add($grpSettings)

$lblGame = New-Object System.Windows.Forms.Label
$lblGame.Text = 'Game Filter'
$lblGame.Location = New-Object System.Drawing.Point(12, 24)
$lblGame.AutoSize = $true
$grpSettings.Controls.Add($lblGame)

$cmbGame = New-Object System.Windows.Forms.ComboBox
$cmbGame.DropDownStyle = 'DropDownList'
$cmbGame.Location = New-Object System.Drawing.Point(92, 20)
$cmbGame.Size = New-Object System.Drawing.Size(118, 24)
[void]$cmbGame.Items.AddRange(@('disabled', 'TCP and UDP', 'TCP only', 'UDP only'))
$grpSettings.Controls.Add($cmbGame)

$lblIpset = New-Object System.Windows.Forms.Label
$lblIpset.Text = 'IPSet Filter'
$lblIpset.Location = New-Object System.Drawing.Point(224, 24)
$lblIpset.AutoSize = $true
$grpSettings.Controls.Add($lblIpset)

$cmbIpset = New-Object System.Windows.Forms.ComboBox
$cmbIpset.DropDownStyle = 'DropDownList'
$cmbIpset.Location = New-Object System.Drawing.Point(304, 20)
$cmbIpset.Size = New-Object System.Drawing.Size(108, 24)
[void]$cmbIpset.Items.AddRange(@('none', 'loaded', 'any'))
$grpSettings.Controls.Add($cmbIpset)

$btnStart = New-Object System.Windows.Forms.Button
$btnStart.Text = 'Start'
$btnStart.Location = New-Object System.Drawing.Point(16, 414)
$btnStart.Size = New-Object System.Drawing.Size(206, 32)
$form.Controls.Add($btnStart)

$btnStop = New-Object System.Windows.Forms.Button
$btnStop.Text = 'Stop'
$btnStop.Location = New-Object System.Drawing.Point(238, 414)
$btnStop.Size = New-Object System.Drawing.Size(206, 32)
$form.Controls.Add($btnStop)

$btnInstall = New-Object System.Windows.Forms.Button
$btnInstall.Text = 'Install Service'
$btnInstall.Location = New-Object System.Drawing.Point(16, 452)
$btnInstall.Size = New-Object System.Drawing.Size(206, 32)
$form.Controls.Add($btnInstall)

$btnRemove = New-Object System.Windows.Forms.Button
$btnRemove.Text = 'Remove Services'
$btnRemove.Location = New-Object System.Drawing.Point(238, 452)
$btnRemove.Size = New-Object System.Drawing.Size(206, 32)
$form.Controls.Add($btnRemove)

$lblLog = New-Object System.Windows.Forms.Label
$lblLog.Location = New-Object System.Drawing.Point(16, 492)
$lblLog.Size = New-Object System.Drawing.Size(428, 28)
$lblLog.ForeColor = [System.Drawing.Color]::DimGray
if ($list.Items.Count -eq 0) {
    $lblLog.Text = 'No strategy files found in the strategies folder.'
} else {
    $lblLog.Text = 'Select a strategy, then Start. Install Service enables autostart.'
}
$form.Controls.Add($lblLog)

$tips = New-Object System.Windows.Forms.ToolTip
$tips.SetToolTip($btnStart, 'Start the selected strategy. If the zapret service is installed, start that service.')
$tips.SetToolTip($btnStop, 'Stop winws.exe and the zapret service if it is running.')
$tips.SetToolTip($btnInstall, 'Install the selected strategy as a Windows service (autostart).')
$tips.SetToolTip($btnRemove, 'Remove zapret and WinDivert services.')
$tips.SetToolTip($cmbGame, 'Stop and Start to apply. If a service is installed, run Install Service again.')
$tips.SetToolTip($cmbIpset, 'Stop and Start to apply.')

function Write-GuiLog {
    param([string]$Message)
    $lblLog.Text = $Message
    $lblLog.Refresh()
}

function ConvertTo-GameComboIndex {
    param([string]$Mode)
    switch ($Mode) {
        'all' { return 1 }
        'tcp' { return 2 }
        'udp' { return 3 }
        default { return 0 }
    }
}

function ConvertFrom-GameComboIndex {
    param([int]$Index)
    switch ($Index) {
        1 { return 'all' }
        2 { return 'tcp' }
        3 { return 'udp' }
        default { return 'disabled' }
    }
}

function Set-ActionButtonsEnabled {
    param([bool]$Enabled)
    $hasStrategies = $list.Items.Count -gt 0
    $btnStart.Enabled = $Enabled -and $hasStrategies
    $btnInstall.Enabled = $Enabled -and $hasStrategies
    $btnStop.Enabled = $Enabled
    $btnRemove.Enabled = $Enabled
}

function Update-SettingHints {
    if (Get-ZapretService) {
        $grpSettings.Text = 'Settings (Game Filter: Install Service)'
        $tips.SetToolTip($cmbGame, 'Run Install Service again to apply Game Filter.')
    } else {
        $grpSettings.Text = 'Settings (Stop and Start to apply)'
        $tips.SetToolTip($cmbGame, 'Stop and Start to apply Game Filter.')
    }
    $tips.SetToolTip($cmbIpset, 'Stop and Start to apply IPSet Filter.')
}

function Update-Status {
    $running = Test-BypassRunning
    if ($running) {
        $lblBypass.Text = 'Bypass: running (winws.exe)'
        $lblBypass.ForeColor = [System.Drawing.Color]::ForestGreen
    } else {
        $lblBypass.Text = 'Bypass: not running'
        $lblBypass.ForeColor = [System.Drawing.Color]::Firebrick
    }

    $svc = Get-ZapretService
    if ($svc) {
        $lblService.Text = "Service: installed ($($svc.Status))"
        $lblService.ForeColor = [System.Drawing.Color]::DarkBlue
    } else {
        $lblService.Text = 'Service: not installed'
        $lblService.ForeColor = [System.Drawing.Color]::DimGray
    }

    $installed = Get-InstalledStrategyName
    if ([string]::IsNullOrWhiteSpace($installed)) {
        $lblInstalled.Text = 'Installed strategy: -'
    } else {
        $lblInstalled.Text = "Installed strategy: $installed"
    }

    $gameIndex = ConvertTo-GameComboIndex (Get-GameFilterMode)
    $ipset = Get-IpsetStatus
    $ipsetIndex = @('none', 'loaded', 'any').IndexOf($ipset)
    if ($ipsetIndex -lt 0) { $ipsetIndex = 0 }

    $script:updatingSettings = $true
    try {
        if ($cmbGame.SelectedIndex -ne $gameIndex) {
            $cmbGame.SelectedIndex = $gameIndex
        }
        if ($cmbIpset.SelectedIndex -ne $ipsetIndex) {
            $cmbIpset.SelectedIndex = $ipsetIndex
        }
    } finally {
        $script:updatingSettings = $false
    }

    Update-SettingHints
}

function Select-InstalledOrFirstStrategy {
    $installed = Get-InstalledStrategyName
    if ($installed -and $list.Items.Contains($installed)) {
        $list.SelectedItem = $installed
        return
    }
    if ($list.Items.Count -gt 0 -and $list.SelectedIndex -lt 0) {
        $list.SelectedIndex = 0
    }
}

function Invoke-GuiAction {
    param(
        [string]$BusyText,
        [scriptblock]$Action
    )

    if ($script:busy) {
        return
    }
    $script:busy = $true
    $form.UseWaitCursor = $true
    Set-ActionButtonsEnabled -Enabled $false
    Write-GuiLog $BusyText
    try {
        & $Action
        Update-Status
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Write-GuiLog $_.Exception.Message
        Update-Status
    } finally {
        Set-ActionButtonsEnabled -Enabled $true
        $form.UseWaitCursor = $false
        $script:busy = $false
    }
}

$btnStart.Add_Click({
    $svc = Get-ZapretService
    if ($svc) {
        $installed = Get-InstalledStrategyName
        $selected = [string]$list.SelectedItem
        if ($installed -and $selected -and ($installed -ne $selected)) {
            $answer = Show-QuestionDialog (
                "The service uses '$installed'. Start that service?" +
                [Environment]::NewLine +
                "To use '$selected', click Install Service."
            )
            if ($answer -ne [System.Windows.Forms.DialogResult]::Yes) {
                return
            }
        }
    }

    Invoke-GuiAction -BusyText 'Starting...' -Action {
        $svcInner = Get-ZapretService
        if ($svcInner) {
            Start-ZapretServiceIfInstalled
            $name = Get-InstalledStrategyName
            if ($name) {
                Write-GuiLog "Started service ($name). Use Install Service to change strategy."
            } else {
                Write-GuiLog 'Service started.'
            }
            return
        }
        $file = Get-SelectedStrategy -ListBox $list
        Start-SelectedStrategy -File $file
        Write-GuiLog "Started $($file.BaseName)."
    }
})

$btnStop.Add_Click({
    Invoke-GuiAction -BusyText 'Stopping...' -Action {
        Stop-Bypass
        Write-GuiLog 'Stopped.'
    }
})

$btnInstall.Add_Click({
    $file = Get-SelectedStrategy -ListBox $list
    Invoke-GuiAction -BusyText 'Installing service...' -Action {
        Install-ZapretService -File $file
        Write-GuiLog "Installed service: $($file.BaseName)."
    }
})

$btnRemove.Add_Click({
    $answer = Show-QuestionDialog 'Remove the zapret and WinDivert services?'
    if ($answer -ne [System.Windows.Forms.DialogResult]::Yes) {
        return
    }
    Invoke-GuiAction -BusyText 'Removing services...' -Action {
        Remove-ZapretServices
        Write-GuiLog 'Services removed.'
    }
})

$list.Add_DoubleClick({
    $btnStart.PerformClick()
})

$cmbGame.Add_SelectedIndexChanged({
    if ($script:updatingSettings) { return }
    try {
        Set-GameFilterMode (ConvertFrom-GameComboIndex $cmbGame.SelectedIndex)
        Write-GuiLog (Get-GameFilterApplyHint)
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
})

$cmbIpset.Add_SelectedIndexChanged({
    if ($script:updatingSettings) { return }
    try {
        Set-IpsetMode -Mode ([string]$cmbIpset.SelectedItem)
        Write-GuiLog (Get-IpsetApplyHint)
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
})

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 2000
$timer.Add_Tick({
    if (-not $script:busy) {
        try {
            Update-Status
        } catch {
            $null = $_
        }
    }
})

$form.Add_Shown({
    try {
        Select-InstalledOrFirstStrategy
        Set-ActionButtonsEnabled -Enabled $true
        Update-Status
        $timer.Start()
    } catch {
        Show-ErrorDialog $_.Exception.Message
    }
})

$form.Add_FormClosed({
    $timer.Stop()
    $timer.Dispose()
})

try {
    [void]$form.ShowDialog()
} catch {
    Show-ErrorDialog $_.Exception.Message
    exit 1
}
