# Zapret WPF GUI.
# This script starts and stops strategies from the strategies folder.
# This script installs or removes the zapret Windows service.
# Entry from zapman.bat is gui-boot.ps1. That file shows Main.xaml before this parse.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

if ($PSVersionTable.PSVersion.Major -lt 3) {
    Write-Host 'ERROR: Windows PowerShell is too old. This program needs 3.0 or newer. Target: 5.1.'
    Write-Host 'On Windows 7 install WMF 5.1 and .NET Framework 4.5 or newer.'
    exit 1
}

if (-not (Get-Variable -Name startupClock -Scope Script -ErrorAction SilentlyContinue)) {
    $script:startupClock = [System.Diagnostics.Stopwatch]::StartNew()
    $script:startupLastMs = 0
    $script:startupMarks = New-Object System.Collections.ArrayList
} else {
    $now = [int]$script:startupClock.ElapsedMilliseconds
    $delta = $now - $script:startupLastMs
    $script:startupLastMs = $now
    [void]$script:startupMarks.Add(('parse: {0} ms' -f $delta))
}

function Add-GuiStartupMark {
    param([string]$Name)
    $now = [int]$script:startupClock.ElapsedMilliseconds
    $delta = $now - $script:startupLastMs
    $script:startupLastMs = $now
    [void]$script:startupMarks.Add(('{0}: {1} ms' -f $Name, $delta))
}

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapman\Zapman.psd1')
[void](Initialize-ZapmanUiLanguage)
Add-GuiStartupMark 'import'

$script:layout = Get-ZapretLayout
$script:rootDir = $script:layout.Root
$script:binDir = $script:layout.Bin
$script:updatingSettings = $false
$script:busy = $false
$script:strategyMap = @{}
$script:statusRefreshWarned = $false
$script:versionCheckBusy = $false
$script:versionCheckClient = $null
$script:versionCheckTimer = $null
$script:versionCheckReportUpToDate = $false
$script:guiClosing = $false
$script:updatingLang = $false
$script:strategyDialogOpen = $false
$script:strategyPendingTests = $false
$script:startupIdleTimer = $null
$script:startupReady = $false
$script:startupShown = $false
$script:guiJournal = New-Object System.Collections.Generic.List[string]
$window = $null
if (Get-Variable -Name guiShellWindow -Scope Script -ErrorAction SilentlyContinue) {
    if ($script:guiShellWindow) {
        $window = $script:guiShellWindow
    }
}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Request-Administrator {
    $boot = Join-Path $PSScriptRoot 'gui-boot.ps1'
    $argList = "-NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$boot`""
    $ps = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
    if (-not (Test-Path -LiteralPath $ps)) {
        $ps = 'powershell.exe'
    }
    try {
        Start-Process -FilePath $ps -ArgumentList $argList -Verb RunAs | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Initialize-GuiConsoleType {
    $code = @'
using System;
using System.Runtime.InteropServices;
public static class GuiConsole {
    [DllImport("kernel32.dll")] public static extern IntPtr GetConsoleWindow();
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
}
'@
    if (-not ('GuiConsole' -as [type])) {
        Add-Type -TypeDefinition $code -ErrorAction Stop
    }
}

function Hide-ConsoleWindow {
    try {
        Initialize-GuiConsoleType
        $hwnd = [GuiConsole]::GetConsoleWindow()
        if ($hwnd -ne [IntPtr]::Zero) {
            [void][GuiConsole]::ShowWindow($hwnd, 0)
        }
    } catch {
        return
    }
}

function Show-ConsoleWindow {
    try {
        Initialize-GuiConsoleType
        $hwnd = [GuiConsole]::GetConsoleWindow()
        if ($hwnd -ne [IntPtr]::Zero) {
            [void][GuiConsole]::ShowWindow($hwnd, 5)
        }
    } catch {
        return
    }
}

function Import-ZapmanXaml {
    param([string]$Name)
    $path = Join-Path $PSScriptRoot $Name
    if (-not (Test-Path -LiteralPath $path)) {
        throw ('XAML file is missing: {0}' -f $Name)
    }
    $fs = New-Object System.IO.FileStream($path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read)
    try {
        return [System.Windows.Markup.XamlReader]::Load($fs)
    } finally {
        $fs.Dispose()
    }
}

function Get-XamlChild {
    param(
        $Root,
        [string]$Name
    )
    $el = $Root.FindName($Name)
    if ($null -eq $el) {
        throw ('XAML name {0} is missing.' -f $Name)
    }
    return $el
}

function Test-GuiBootOwnsRun {
    if (Get-Variable -Name guiBootOwnsRun -Scope Script -ErrorAction SilentlyContinue) {
        return [bool]$script:guiBootOwnsRun
    }
    return $false
}

function Update-GuiWindowPaint {
    if (-not $window) {
        return
    }
    try {
        $window.Dispatcher.Invoke(
            [System.Windows.Threading.DispatcherPriority]::Render,
            [System.Action]{ $null }
        )
    } catch {
        return
    }
}

function Show-ZapmanOwnedDialog {
    param($Dialog)
    if ($window) {
        $Dialog.Owner = $window
        $Dialog.WindowStartupLocation = [System.Windows.WindowStartupLocation]::CenterOwner
    }
    return $Dialog.ShowDialog()
}

function Show-ErrorDialog {
    param([string]$Message)
    if ($window) {
        [System.Windows.MessageBox]::Show(
            $window,
            $Message,
            'Zapret',
            [System.Windows.MessageBoxButton]::OK,
            [System.Windows.MessageBoxImage]::Error
        ) | Out-Null
        return
    }
    [System.Windows.MessageBox]::Show(
        $Message,
        'Zapret',
        [System.Windows.MessageBoxButton]::OK,
        [System.Windows.MessageBoxImage]::Error
    ) | Out-Null
}

function Show-InfoDialog {
    param([string]$Message)
    if ($window) {
        [System.Windows.MessageBox]::Show(
            $window,
            $Message,
            'Zapret',
            [System.Windows.MessageBoxButton]::OK,
            [System.Windows.MessageBoxImage]::Information
        ) | Out-Null
        return
    }
    [System.Windows.MessageBox]::Show(
        $Message,
        'Zapret',
        [System.Windows.MessageBoxButton]::OK,
        [System.Windows.MessageBoxImage]::Information
    ) | Out-Null
}

function Show-QuestionDialog {
    param(
        [string]$Message,
        [bool]$DefaultYes = $true
    )
    $def = [System.Windows.MessageBoxResult]::Yes
    if (-not $DefaultYes) {
        $def = [System.Windows.MessageBoxResult]::No
    }
    if ($window) {
        return [System.Windows.MessageBox]::Show(
            $window,
            $Message,
            'Zapret',
            [System.Windows.MessageBoxButton]::YesNo,
            [System.Windows.MessageBoxImage]::Question,
            $def
        )
    }
    return [System.Windows.MessageBox]::Show(
        $Message,
        'Zapret',
        [System.Windows.MessageBoxButton]::YesNo,
        [System.Windows.MessageBoxImage]::Question,
        $def
    )
}


function Invoke-GuiPump {
    $disp = $null
    if ($window) {
        $disp = $window.Dispatcher
    } elseif ([System.Windows.Application]::Current) {
        $disp = [System.Windows.Application]::Current.Dispatcher
    }
    if (-not $disp) {
        return
    }
    [void]$disp.Invoke([System.Windows.Threading.DispatcherPriority]::Background, [action]{})
}

function Invoke-GuiOnUi {
    param([scriptblock]$Action)
    if ($window -and $window.Dispatcher -and -not $window.Dispatcher.CheckAccess()) {
        [void]$window.Dispatcher.Invoke(
            [System.Windows.Threading.DispatcherPriority]::Normal,
            [action]{ & $Action }
        )
        return
    }
    & $Action
}

function Stop-GuiVersionCheck {
    param([bool]$FromComplete = $false)
    if ($script:versionCheckTimer) {
        $script:versionCheckTimer.Stop()
        $script:versionCheckTimer = $null
    }
    $client = $script:versionCheckClient
    $script:versionCheckClient = $null
    if ($client) {
        if ((-not $FromComplete) -and $client.IsBusy) {
            $client.CancelAsync()
        }
        $client.Dispose()
    }
    $script:versionCheckBusy = $false
}

function Complete-GuiVersionCheck {
    param(
        $ErrorObject,
        [string]$RemoteText,
        [bool]$Cancelled
    )
    Stop-GuiVersionCheck -FromComplete $true
    if ($script:guiClosing -or -not $window) {
        return
    }
    $reportUpToDate = $script:versionCheckReportUpToDate
    $script:versionCheckReportUpToDate = $false
    if ($Cancelled -or $ErrorObject) {
        if ($reportUpToDate) {
            Show-InfoDialog (Get-ZapmanUiString -Key 'VersionFail')
            Write-GuiLog (Get-ZapmanUiString -Key 'VersionFail')
        } else {
            Write-GuiLog (Get-ZapmanUiString -Key 'AutoFail')
        }
        return
    }
    $remote = Get-ZapmanVersionFromRemoteBody -Body $RemoteText
    $local = Get-ZapmanLocalVersion
    if ($remote -eq $local) {
        if ($reportUpToDate) {
            Show-InfoDialog (Get-ZapmanUiString -Key 'VersionLatest' -FormatArgs @($local))
            Write-GuiLog (Get-ZapmanUiString -Key 'VersionLatest' -FormatArgs @($local))
        }
        return
    }
    if (-not $remote) {
        if ($reportUpToDate) {
            Show-InfoDialog (Get-ZapmanUiString -Key 'VersionFail')
            Write-GuiLog (Get-ZapmanUiString -Key 'VersionFail')
        }
        return
    }
    $answer = Show-QuestionDialog (Get-ZapmanUiString -Key 'VersionNew' -FormatArgs @($remote))
    if ($answer -eq [System.Windows.MessageBoxResult]::Yes) {
        Start-Process (Get-ZapretReleasePageUrl)
    }
    Write-GuiLog (Get-ZapmanUiString -Key 'VersionNew' -FormatArgs @($remote))
}

function Start-GuiVersionCheck {
    param([bool]$ReportUpToDate = $false)

    if ($script:versionCheckBusy) {
        return
    }
    $script:versionCheckBusy = $true
    $script:versionCheckReportUpToDate = $ReportUpToDate
    Enable-ZapmanTls12
    $wc = New-Object System.Net.WebClient
    $wc.Headers.Add('Cache-Control', 'no-cache')
    $wc.Headers.Add('User-Agent', 'zapman')
    $wc.Headers.Add('Accept', 'application/vnd.github+json')
    $script:versionCheckClient = $wc
    $wc.add_DownloadStringCompleted({
        param($source, $e)
        $err = $null
        $text = ''
        $cancelled = $false
        try {
            if ($null -eq $source) {
                return
            }
            if ($e) {
                $cancelled = [bool]$e.Cancelled
                $err = $e.Error
                if (-not $cancelled -and -not $err) {
                    $text = [string]$e.Result
                }
            }
        } catch {
            $err = $_.Exception
        }
        $script:vcErr = $err
        $script:vcText = $text
        $script:vcCancelled = $cancelled
        Invoke-GuiOnUi {
            Complete-GuiVersionCheck -ErrorObject $script:vcErr -RemoteText $script:vcText -Cancelled $script:vcCancelled
        }
    })
    $to = New-Object System.Windows.Threading.DispatcherTimer
    $to.Interval = [TimeSpan]::FromMilliseconds(8000)
    $script:versionCheckTimer = $to
    $to.Add_Tick({
        $client = $script:versionCheckClient
        if ($client -and $client.IsBusy) {
            $client.CancelAsync()
        }
        if ($script:versionCheckTimer) {
            $script:versionCheckTimer.Stop()
        }
    })
    $to.Start()
    try {
        $wc.DownloadStringAsync([uri](Get-ZapretVersionCheckUrl))
    } catch {
        Complete-GuiVersionCheck -ErrorObject $_.Exception -RemoteText '' -Cancelled $false
    }
}


. (Join-Path $PSScriptRoot 'gui-dialogs.ps1')

try {
    Add-Type -AssemblyName PresentationFramework -ErrorAction Stop
    Add-Type -AssemblyName PresentationCore -ErrorAction Stop
    Add-Type -AssemblyName WindowsBase -ErrorAction Stop
} catch {
    Show-ConsoleWindow
    Write-Host $_.Exception.Message
    exit 1
}
Hide-ConsoleWindow
if (-not $window) {
    Add-GuiStartupMark 'hide-console'
    if (-not (Test-IsAdministrator)) {
        if (-not (Request-Administrator)) {
            Show-ConsoleWindow
            Show-ErrorDialog (Get-ZapmanUiString -Key 'AdminRequired')
            exit 1
        }
        exit 0
    }
}

Initialize-ZapmanUserLists

$strategies = @(Get-ZapretStrategyFiles)
$script:strategyMap = @{}
foreach ($file in $strategies) {
    $script:strategyMap[$file.BaseName] = $file
}

if (-not $window) {
    $window = Import-ZapmanXaml 'Main.xaml'
}
$title = Get-XamlChild -Root $window -Name 'title'
$rbRu = Get-XamlChild -Root $window -Name 'rbRu'
$rbEn = Get-XamlChild -Root $window -Name 'rbEn'
$lblBanner = Get-XamlChild -Root $window -Name 'lblBanner'
$grpStatus = Get-XamlChild -Root $window -Name 'grpStatus'
$lblBypass = Get-XamlChild -Root $window -Name 'lblBypass'
$lblService = Get-XamlChild -Root $window -Name 'lblService'
$lblInstalled = Get-XamlChild -Root $window -Name 'lblInstalled'
$lblDivert = Get-XamlChild -Root $window -Name 'lblDivert'
$btnStart = Get-XamlChild -Root $window -Name 'btnStart'
$btnStop = Get-XamlChild -Root $window -Name 'btnStop'
$btnRemove = Get-XamlChild -Root $window -Name 'btnRemove'
$btnStrategy = Get-XamlChild -Root $window -Name 'btnStrategy'
$grpSettings = Get-XamlChild -Root $window -Name 'grpSettings'
$lblGame = Get-XamlChild -Root $window -Name 'lblGame'
$cmbGame = Get-XamlChild -Root $window -Name 'cmbGame'
$lblIpset = Get-XamlChild -Root $window -Name 'lblIpset'
$cmbIpset = Get-XamlChild -Root $window -Name 'cmbIpset'
$chkAuto = Get-XamlChild -Root $window -Name 'chkAuto'
$btnFakes = Get-XamlChild -Root $window -Name 'btnFakes'
$grpTools = Get-XamlChild -Root $window -Name 'grpTools'
$btnIpsetUpd = Get-XamlChild -Root $window -Name 'btnIpsetUpd'
$btnHosts = Get-XamlChild -Root $window -Name 'btnHosts'
$btnUpdates = Get-XamlChild -Root $window -Name 'btnUpdates'
$btnDiag = Get-XamlChild -Root $window -Name 'btnDiag'

[void]$cmbGame.Items.Add('disabled')
[void]$cmbGame.Items.Add('TCP and UDP')
[void]$cmbGame.Items.Add('TCP only')
[void]$cmbGame.Items.Add('UDP only')
[void]$cmbIpset.Items.Add('none')
[void]$cmbIpset.Items.Add('loaded')
[void]$cmbIpset.Items.Add('any')

function Write-GuiLog {
    param([string]$Message)
    $line = ('{0}  {1}' -f (Get-Date -Format 'HH:mm:ss'), $Message)
    [void]$script:guiJournal.Add($line)
    if ($script:guiJournal.Count -gt 80) {
        $script:guiJournal.RemoveAt(0)
    }
}

function Update-GuiLanguage {
    $ver = Get-ZapmanLocalVersion
    $window.Title = Get-ZapmanUiString -Key 'AppTitle' -FormatArgs @($ver)
    $title.Text = Get-ZapmanUiString -Key 'AppName'
    $grpStatus.Header = Get-ZapmanUiString -Key 'GrpStatus'
    $grpSettings.Header = Get-ZapmanUiString -Key 'GrpSettings'
    $grpTools.Header = Get-ZapmanUiString -Key 'GrpTools'
    $btnStart.Content = Get-ZapmanUiString -Key 'BtnStart'
    $btnStop.Content = Get-ZapmanUiString -Key 'BtnStop'
    $btnRemove.Content = Get-ZapmanUiString -Key 'BtnRemove'
    $btnStrategy.Content = Get-ZapmanUiString -Key 'BtnStrategy'
    $btnFakes.Content = Get-ZapmanUiString -Key 'BtnFakes'
    $btnIpsetUpd.Content = Get-ZapmanUiString -Key 'BtnIpset'
    $btnHosts.Content = Get-ZapmanUiString -Key 'BtnHosts'
    $btnUpdates.Content = Get-ZapmanUiString -Key 'BtnVersion'
    $btnDiag.Content = Get-ZapmanUiString -Key 'BtnDiag'
    $chkAuto.Content = Get-ZapmanUiString -Key 'ChkAuto'
    $lblGame.Text = Get-ZapmanUiString -Key 'LblGame'
    $lblIpset.Text = Get-ZapmanUiString -Key 'LblIpset'
    $rbRu.Content = Get-ZapmanUiString -Key 'LangRu'
    $rbEn.Content = Get-ZapmanUiString -Key 'LangEn'
    $btnStart.ToolTip = Get-ZapmanUiString -Key 'TipStart'
    $btnStop.ToolTip = Get-ZapmanUiString -Key 'TipStop'
    $btnRemove.ToolTip = Get-ZapmanUiString -Key 'TipRemove'
    $btnStrategy.ToolTip = Get-ZapmanUiString -Key 'TipStrategy'
    $btnFakes.ToolTip = Get-ZapmanUiString -Key 'TipFakes'
    $btnIpsetUpd.ToolTip = Get-ZapmanUiString -Key 'TipIpset'
    $btnHosts.ToolTip = Get-ZapmanUiString -Key 'TipHosts'
    $btnUpdates.ToolTip = Get-ZapmanUiString -Key 'TipVersion'
    $btnDiag.ToolTip = Get-ZapmanUiString -Key 'TipDiag'
    $chkAuto.ToolTip = Get-ZapmanUiString -Key 'TipAuto'
    $grpStatus.ToolTip = Get-ZapmanUiString -Key 'TipStatusClick'
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
    $hasService = $false
    if (Get-ZapretService) {
        $hasService = $true
    }
    $hasStrategies = $script:strategyMap.Count -gt 0
    $btnStart.IsEnabled = $Enabled -and $hasService
    $btnStop.IsEnabled = $Enabled
    $btnRemove.IsEnabled = $Enabled
    $btnStrategy.IsEnabled = $Enabled -and $hasStrategies
    $btnFakes.IsEnabled = $Enabled
    $btnIpsetUpd.IsEnabled = $Enabled
    $btnHosts.IsEnabled = $Enabled
    $btnUpdates.IsEnabled = $Enabled
    $btnDiag.IsEnabled = $Enabled
    $chkAuto.IsEnabled = $Enabled
    $cmbGame.IsEnabled = $Enabled
    $cmbIpset.IsEnabled = $Enabled
}

function Update-Status {
    $running = Test-ZapretBypassRunning
    $bypassName = Get-ZapretStatusBypassName
    if ($running) {
        $lblBypass.Text = Get-ZapmanUiString -Key 'StatusBypassOn' -FormatArgs @($bypassName)
    } else {
        $lblBypass.Text = Get-ZapmanUiString -Key 'StatusBypassOff'
    }

    $svc = Get-ZapretService
    if ($svc) {
        $lblService.Text = Get-ZapmanUiString -Key 'StatusServiceOn' -FormatArgs @($svc.Status)
    } else {
        $lblService.Text = Get-ZapmanUiString -Key 'StatusServiceOff'
    }

    $installed = Get-ZapretInstalledStrategyName
    $runningName = Get-ZapretRunningStrategyName
    $hasStrategies = $script:strategyMap.Count -gt 0
    $svcRunning = $false
    if ($svc -and $svc.Status -eq 'Running') {
        $svcRunning = $true
    }
    if ($running -and -not [string]::IsNullOrWhiteSpace($runningName)) {
        if ($svcRunning) {
            $lblInstalled.Text = Get-ZapmanUiString -Key 'StatusStrategy' -FormatArgs @($runningName)
        } else {
            $lblInstalled.Text = Get-ZapmanUiString -Key 'StatusStrategyRun' -FormatArgs @($runningName)
        }
    } elseif ([string]::IsNullOrWhiteSpace($installed)) {
        if ($running -and -not $svc) {
            $lblInstalled.Text = Get-ZapmanUiString -Key 'StatusStrategyManual'
        } else {
            $lblInstalled.Text = Get-ZapmanUiString -Key 'StatusStrategyNone'
        }
    } elseif (-not $hasStrategies) {
        $lblInstalled.Text = Get-ZapmanUiString -Key 'StatusStrategyGone' -FormatArgs @($installed)
    } else {
        $lblInstalled.Text = Get-ZapmanUiString -Key 'StatusStrategy' -FormatArgs @($installed)
    }

    $wd = Get-Service -Name 'WinDivert' -ErrorAction SilentlyContinue
    $pending = $false
    if ($wd -and $wd.Status -eq 'Running') {
        $lblDivert.Text = Get-ZapmanUiString -Key 'StatusDivertOn'
    } elseif ($wd -and ([string]$wd.Status -eq 'StopPending')) {
        $lblDivert.Text = Get-ZapmanUiString -Key 'StatusDivertPending'
        $pending = $true
    } elseif ($wd) {
        $lblDivert.Text = Get-ZapmanUiString -Key 'StatusDivertOther' -FormatArgs @($wd.Status)
    } else {
        $lblDivert.Text = Get-ZapmanUiString -Key 'StatusDivertNone'
    }

    $bannerKey = ''
    $bannerBg = [System.Windows.Media.Color]::FromRgb(230, 240, 250)
    $bannerFg = [System.Windows.Media.Color]::FromRgb(24, 95, 165)
    if ($pending) {
        $bannerKey = 'BannerStopPending'
        $bannerBg = [System.Windows.Media.Color]::FromRgb(253, 232, 230)
        $bannerFg = [System.Windows.Media.Color]::FromRgb(180, 35, 24)
    } elseif (-not $hasStrategies) {
        $bannerKey = 'BannerNoStrategies'
        $bannerBg = [System.Windows.Media.Color]::FromRgb(255, 244, 214)
        $bannerFg = [System.Windows.Media.Color]::FromRgb(154, 103, 0)
    } elseif ($svc -and -not $running) {
        $bannerKey = 'BannerMismatch'
        $bannerBg = [System.Windows.Media.Color]::FromRgb(255, 244, 214)
        $bannerFg = [System.Windows.Media.Color]::FromRgb(154, 103, 0)
    } elseif ($running -and -not $svc) {
        $bannerKey = 'BannerOrphan'
    } elseif (-not $svc) {
        $bannerKey = 'BannerNoService'
    }
    if ($bannerKey) {
        if ($bannerKey -eq 'BannerMismatch') {
            $lblBanner.Text = Get-ZapmanUiString -Key $bannerKey -FormatArgs @($bypassName)
        } else {
            $lblBanner.Text = Get-ZapmanUiString -Key $bannerKey
        }
        $lblBanner.Background = New-Object System.Windows.Media.SolidColorBrush($bannerBg)
        $lblBanner.Foreground = New-Object System.Windows.Media.SolidColorBrush($bannerFg)
        $lblBanner.Visibility = [System.Windows.Visibility]::Visible
    } else {
        $lblBanner.Text = ''
        $lblBanner.Visibility = [System.Windows.Visibility]::Collapsed
    }

    $gameIndex = ConvertTo-GameComboIndex ((Get-ZapretGameFilter).Mode)
    $ipset = Get-ZapretIpsetStatus
    $ipsetIndex = @('none', 'loaded', 'any').IndexOf($ipset)
    if ($ipsetIndex -lt 0) { $ipsetIndex = 0 }

    $autoOn = Test-ZapmanAutoUpdateEnabled
    $script:updatingSettings = $true
    try {
        if ($cmbGame.SelectedIndex -ne $gameIndex) {
            $cmbGame.SelectedIndex = $gameIndex
        }
        if ($cmbIpset.SelectedIndex -ne $ipsetIndex) {
            $cmbIpset.SelectedIndex = $ipsetIndex
        }
        if (($chkAuto.IsChecked -eq $true) -ne $autoOn) {
            $chkAuto.IsChecked = $autoOn
        }
    } finally {
        $script:updatingSettings = $false
    }

    if (-not $script:busy) {
        Set-ActionButtonsEnabled -Enabled $true
    }
}

function Select-InstalledOrFirstStrategy {
    param($ListBox)
    if (-not $ListBox) {
        return
    }
    $pick = ''
    if (Test-ZapretBypassRunning) {
        $pick = Get-ZapretRunningStrategyName
    }
    if ([string]::IsNullOrWhiteSpace($pick)) {
        $pick = Get-ZapretInstalledStrategyName
    }
    $i = 0
    foreach ($it in @($ListBox.Items)) {
        $name = $null
        if ($it -is [System.Windows.Controls.ListBoxItem]) {
            $name = [string]$it.Tag
        } else {
            $name = [string]$it
        }
        if ($pick -and $name -eq $pick) {
            $ListBox.SelectedIndex = $i
            return
        }
        $i++
    }
    if ($ListBox.Items.Count -gt 0 -and $ListBox.SelectedIndex -lt 0) {
        $ListBox.SelectedIndex = 0
    }
}

function Invoke-GuiAction {
    param(
        [string]$BusyText,
        [scriptblock]$Action,
        [switch]$FreezeUi
    )

    if ($script:busy) {
        return
    }
    $script:busy = $true
    if ($FreezeUi) {
        $window.Cursor = [System.Windows.Input.Cursors]::Wait
        Set-ActionButtonsEnabled -Enabled $false
    }
    Write-GuiLog $BusyText
    try {
        & $Action
        Update-Status
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Write-GuiLog $_.Exception.Message
        Update-Status
    } finally {
        if ($FreezeUi) {
            Set-ActionButtonsEnabled -Enabled $true
            $window.Cursor = $null
        }
        $script:busy = $false
    }
}

$btnStart.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnStart') -FreezeUi -Action {
        Start-ZapretServiceIfInstalled -OnWait { Invoke-GuiPump }
        $name = Get-ZapretInstalledStrategyName
        if ($name) {
            Write-GuiLog (Get-ZapmanUiString -Key 'StartDoneName' -FormatArgs @($name))
        } else {
            Write-GuiLog (Get-ZapmanUiString -Key 'StartDone')
        }
    }
})

$btnStop.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnStop') -FreezeUi -Action {
        Stop-ZapretBypass -OnWait { Invoke-GuiPump }
        Write-GuiLog (Get-ZapmanUiString -Key 'StopDone')
    }
})

$btnRemove.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnRemove') -FreezeUi -Action {
        Remove-ZapretServices -OnWait { Invoke-GuiPump }
        Write-GuiLog (Get-ZapmanUiString -Key 'RemoveDone')
    }
})

$btnStrategy.Add_Click({
    if ($script:strategyMap.Count -lt 1) {
        Show-ErrorDialog (Get-ZapmanUiString -Key 'NoStrategies')
        return
    }
    Show-StrategyDialog
})

# One handler on the group. Child MouseLeftButtonUp bubbles; extra handlers
# opened a new dialog after each Close.
$grpStatus.Add_MouseLeftButtonUp({
    $_.Handled = $true
    Show-StatusJournalDialog
})

$cmbGame.Add_SelectionChanged({
    if ($script:updatingSettings) { return }
    try {
        Set-ZapretGameFilterMode (ConvertFrom-GameComboIndex $cmbGame.SelectedIndex)
        Write-GuiLog (Get-GameFilterApplyHint)
        if (Get-ZapretService) {
            $ans = Show-QuestionDialog -Message (Get-ZapmanUiString -Key 'FilterNeedInstall')
            if ($ans -eq [System.Windows.MessageBoxResult]::Yes) {
                Show-StrategyDialog
            }
        } else {
            Show-InfoDialog (Get-ZapmanUiString -Key 'FilterNeedRerun')
        }
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
})

$cmbIpset.Add_SelectionChanged({
    if ($script:updatingSettings) { return }
    try {
        Set-ZapretIpsetMode -Mode ([string]$cmbIpset.SelectedItem)
        Write-GuiLog (Get-IpsetApplyHint)
        if (Get-ZapretService) {
            $ans = Show-QuestionDialog -Message (Get-ZapmanUiString -Key 'FilterNeedRestart')
            if ($ans -eq [System.Windows.MessageBoxResult]::Yes) {
                Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnStop') -FreezeUi -Action {
                    Stop-ZapretBypass -OnWait { Invoke-GuiPump }
                    Start-ZapretServiceIfInstalled -OnWait { Invoke-GuiPump }
                    Write-GuiLog (Get-ZapmanUiString -Key 'StartDone')
                }
            }
        } else {
            Show-InfoDialog (Get-ZapmanUiString -Key 'FilterNeedRerun')
        }
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
})

$autoChanged = {
    if ($script:updatingSettings) { return }
    try {
        $on = $chkAuto.IsChecked -eq $true
        Set-ZapmanAutoUpdateEnabled -Enabled $on
        if ($on) {
            Write-GuiLog (Get-ZapmanUiString -Key 'AutoOn')
        } else {
            Write-GuiLog (Get-ZapmanUiString -Key 'AutoOff')
        }
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
}
$chkAuto.Add_Checked($autoChanged)
$chkAuto.Add_Unchecked($autoChanged)

$btnFakes.Add_Click({
    try {
        $choice = Show-FakesDialog
    } catch {
        Show-ErrorDialog $_.Exception.Message
        return
    }
    if (-not $choice) {
        return
    }
    $changes = @($choice.Changes)
    if ($changes.Count -eq 0) {
        return
    }
    try {
        foreach ($item in $changes) {
            Set-ZapretActiveFake -Slot $item.Slot -SourcePath $item.Path
            Write-GuiLog (Get-ZapmanUiString -Key 'FakeDone' -FormatArgs @($item.Slot, $item.Name))
        }
        if (Get-ZapretService) {
            $ans = Show-QuestionDialog -Message (Get-ZapmanUiString -Key 'FakeNeedRestart')
            if ($ans -eq [System.Windows.MessageBoxResult]::Yes) {
                Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnStop') -FreezeUi -Action {
                    Stop-ZapretBypass -OnWait { Invoke-GuiPump }
                    Start-ZapretServiceIfInstalled -OnWait { Invoke-GuiPump }
                    Write-GuiLog (Get-ZapmanUiString -Key 'StartDone')
                }
            }
        } else {
            Show-InfoDialog (Get-ZapmanUiString -Key 'FakeNeedRerun')
        }
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
})

$btnIpsetUpd.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnIpset') -Action {
        $temp = Join-Path $env:TEMP 'zapret-ipset-all.txt'
        try {
            Invoke-GuiDownload -Url (Get-ZapretIpsetListUrl) -Destination $temp -Title (Get-ZapmanUiString -Key 'IpsetTitle')
        } catch {
            if ($_.Exception.Message -eq 'Download cancelled.') {
                Write-GuiLog (Get-ZapmanUiString -Key 'IpsetCancel')
                return
            }
            throw
        }
        Update-ZapretIpsetList -SourceFile $temp
        Write-GuiLog (Get-ZapmanUiString -Key 'IpsetOk')
    }
})

$btnHosts.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnHosts') -Action {
        $temp = Join-Path $env:TEMP 'zapret_hosts.txt'
        try {
            Invoke-GuiDownload -Url ((Get-ZapretHostsSourceUrl) + '?t=' + [guid]::NewGuid().ToString()) -Destination $temp -Title (Get-ZapmanUiString -Key 'HostsTitle')
            $info = Get-ZapretHostsUpdateInfo -TempFile $temp
        } catch {
            if ($_.Exception.Message -eq 'Download cancelled.') {
                Write-GuiLog (Get-ZapmanUiString -Key 'HostsCancel')
                return
            }
            throw (Get-ZapmanUiString -Key 'HostsFail')
        }
        if ($info.NeedsUpdate) {
            Show-HostsUpdateDialog -Info $info
            Write-GuiLog (Get-ZapmanUiString -Key 'HostsNeed')
        } else {
            Remove-Item -LiteralPath $info.TempFile -Force -ErrorAction SilentlyContinue
            Show-InfoDialog (Get-ZapmanUiString -Key 'HostsOk')
            Write-GuiLog (Get-ZapmanUiString -Key 'HostsOk')
        }
    }
})

$btnUpdates.Add_Click({
    if ($script:versionCheckBusy) {
        return
    }
    Write-GuiLog (Get-ZapmanUiString -Key 'VersionChecking')
    Start-GuiVersionCheck -ReportUpToDate $true
})

$btnDiag.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnDiag') -Action {
        Show-GuiDiagnostics
        Write-GuiLog (Get-ZapmanUiString -Key 'DiagDone')
    }
})

$timer = New-Object System.Windows.Threading.DispatcherTimer
$timer.Interval = [TimeSpan]::FromMilliseconds(2000)
$timer.Add_Tick({
    if (-not $script:busy) {
        try {
            Update-Status
        } catch {
            if (-not $script:statusRefreshWarned) {
                $script:statusRefreshWarned = $true
                Write-GuiLog 'Status refresh failed.'
            }
            return
        }
    }
})

function Start-GuiFirstPaint {
    if ($script:startupShown) {
        return
    }
    $script:startupShown = $true
    try {
        Add-GuiStartupMark 'shown'
        $total = [int]$script:startupClock.ElapsedMilliseconds
        Write-GuiLog ('Startup {0}; total {1} ms' -f ($script:startupMarks -join '; '), $total)
        $script:startupIdleTimer = New-Object System.Windows.Threading.DispatcherTimer
        $script:startupIdleTimer.Interval = [TimeSpan]::FromMilliseconds(1)
        $script:startupIdleTimer.Add_Tick({
            $idle = $script:startupIdleTimer
            $script:startupIdleTimer = $null
            if ($idle) {
                $idle.Stop()
            }
            if ($script:guiClosing -or -not $window) {
                return
            }
            $script:startupReady = $true
            try {
                $winwsExe = Join-Path $script:binDir 'winws.exe'
                if (Test-Path -LiteralPath $winwsExe) {
                    try {
                        Add-Type -AssemblyName System.Drawing -ErrorAction Stop
                        $extracted = [System.Drawing.Icon]::ExtractAssociatedIcon($winwsExe)
                        if ($extracted) {
                            $src = [System.Windows.Interop.Imaging]::CreateBitmapSourceFromHIcon(
                                $extracted.Handle,
                                [System.Windows.Int32Rect]::Empty,
                                [System.Windows.Media.Imaging.BitmapSizeOptions]::FromEmptyOptions()
                            )
                            $src.Freeze()
                            $window.Icon = $src
                            $extracted.Dispose()
                        }
                    } catch {
                        $null = $_.Exception
                    }
                }
                Set-ActionButtonsEnabled -Enabled $true
                Update-Status
                $cfgErr = Get-ZapmanConfigError
                if (-not [string]::IsNullOrWhiteSpace($cfgErr)) {
                    Write-GuiLog $cfgErr
                }
                $timer.Start()
                if (Test-ZapmanAutoUpdateEnabled) {
                    Start-GuiVersionCheck
                }
                Add-GuiStartupMark 'idle'
                Write-GuiLog ('Startup idle; total {0} ms' -f ([int]$script:startupClock.ElapsedMilliseconds))
            } catch {
                Show-ErrorDialog $_.Exception.Message
            }
        })
        $script:startupIdleTimer.Start()
    } catch {
        Show-ErrorDialog $_.Exception.Message
    }
}

$window.Add_ContentRendered({ Start-GuiFirstPaint })

$window.Add_Closed({
    $script:guiClosing = $true
    if ($script:startupIdleTimer) {
        $script:startupIdleTimer.Stop()
        $script:startupIdleTimer = $null
    }
    Stop-GuiVersionCheck
    $timer.Stop()
})

$rbRu.Add_Checked({
    if ($script:updatingLang) { return }
    if ($rbRu.IsChecked -eq $true) {
        Set-ZapmanUiLanguage -Language 'ru'
        Update-GuiLanguage
        Update-Status
    }
})
$rbEn.Add_Checked({
    if ($script:updatingLang) { return }
    if ($rbEn.IsChecked -eq $true) {
        Set-ZapmanUiLanguage -Language 'en'
        Update-GuiLanguage
        Update-Status
    }
})

$script:updatingLang = $true
if ((Get-ZapmanUiLanguage) -eq 'ru') {
    $rbRu.IsChecked = $true
} else {
    $rbEn.IsChecked = $true
}
$script:updatingLang = $false
Update-GuiLanguage
Add-GuiStartupMark 'build-form'
Update-GuiWindowPaint
if ($window.IsLoaded) {
    [void]$window.Dispatcher.BeginInvoke(
        [System.Windows.Threading.DispatcherPriority]::Normal,
        [System.Action]{ Start-GuiFirstPaint }
    )
}

if (-not (Test-GuiBootOwnsRun)) {
    try {
        $app = [System.Windows.Application]::Current
        if (-not $app) {
            $app = New-Object System.Windows.Application
        }
        $app.ShutdownMode = [System.Windows.ShutdownMode]::OnMainWindowClose
        # Application.Run is the main loop. ShowDialog on $window ends when a child dialog disables the owner.
        [void]$app.Run($window)
    } catch {
        Show-ErrorDialog $_.Exception.Message
        exit 1
    }
}
