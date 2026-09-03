# Zapret WPF GUI.
# This script starts and stops strategies from the strategies folder.
# This script installs or removes the zapret Windows service.
# Entry from zapret.bat is gui-boot.ps1 so the first mark is the parse of this file.

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

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Zapret.psd1')
[void](Initialize-ZapretUiLanguage)
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

function Import-ZapretXaml {
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

function Show-ZapretOwnedDialog {
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

function Show-GuiDiagnostics {
    $dlg = Import-ZapretXaml 'DiagPick.xaml'
    $dlg.Title = Get-ZapretUiString -Key 'DiagTitle'
    $diagPanel = Get-XamlChild -Root $dlg -Name 'diagPanel'
    $btnOk = Get-XamlChild -Root $dlg -Name 'btnOk'
    $btnOk.Content = Get-ZapretUiString -Key 'BtnOk'
    $btnOk.Add_Click({
        $dlg.DialogResult = $true
    })

    $script:diagPanel = $diagPanel
    $script:diagBusy = $false
    $script:diagRebuild = $null

    $script:diagRebuild = {
        $p = $script:diagPanel
        if (-not $p) {
            return
        }
        $p.Children.Clear()
        $report = Get-ZapretDiagnosticReport
        $items = @($report.Items)
        foreach ($item in $items) {
            $row = New-Object System.Windows.Controls.DockPanel
            $row.LastChildFill = $true
            $row.Height = 30

            $hasAction = -not [string]::IsNullOrWhiteSpace([string]$item.Action)

            $mark = New-Object System.Windows.Controls.TextBlock
            $mark.Width = 22
            $mark.TextAlignment = [System.Windows.TextAlignment]::Center
            $mark.VerticalAlignment = [System.Windows.VerticalAlignment]::Center
            $mark.FontSize = 14
            if ($item.Status -eq 'ok') {
                $mark.Text = [string][char]0x2713
                $mark.Foreground = [System.Windows.Media.Brushes]::ForestGreen
            } elseif ($item.Status -eq 'warn') {
                $mark.Text = '!'
                $mark.Foreground = [System.Windows.Media.Brushes]::DarkOrange
            } else {
                $mark.Text = [string][char]0x2717
                $mark.Foreground = [System.Windows.Media.Brushes]::Firebrick
            }
            [System.Windows.Controls.DockPanel]::SetDock($mark, [System.Windows.Controls.Dock]::Right)
            [void]$row.Children.Add($mark)

            if ($hasAction) {
                $btn = New-Object System.Windows.Controls.Button
                $btn.Content = Get-ZapretUiString -Key 'DiagRun'
                $btn.Width = 84
                $btn.Height = 24
                $btn.Margin = New-Object System.Windows.Thickness(0, 3, 8, 3)
                $btn.VerticalAlignment = [System.Windows.VerticalAlignment]::Center
                $btn.Tag = $item
                $btn.Add_Click({
                    if ($script:diagBusy) {
                        return
                    }
                    $it = $this.Tag
                    if (-not $it) {
                        return
                    }
                    $script:diagBusy = $true
                    try {
                        $this.IsEnabled = $false
                        $msgs = @(Invoke-ZapretDiagnosticAction -Action ([string]$it.Action) -Names @($it.ActionNames))
                        Invoke-GuiPump
                        $fails = @($msgs | Where-Object { $_ -like 'FAIL:*' })
                        if ($fails.Count -gt 0) {
                            Show-ErrorDialog (($fails -join [Environment]::NewLine))
                        }
                        if ($script:diagRebuild) {
                            & $script:diagRebuild
                        }
                    } catch {
                        Show-ErrorDialog $_.Exception.Message
                    } finally {
                        $script:diagBusy = $false
                    }
                })
                [System.Windows.Controls.DockPanel]::SetDock($btn, [System.Windows.Controls.Dock]::Right)
                [void]$row.Children.Add($btn)
            }

            $lbl = New-Object System.Windows.Controls.TextBlock
            $lbl.Text = [string]$item.Text
            $lbl.TextTrimming = [System.Windows.TextTrimming]::CharacterEllipsis
            $lbl.VerticalAlignment = [System.Windows.VerticalAlignment]::Center
            $lbl.Margin = New-Object System.Windows.Thickness(4, 0, 8, 0)
            [void]$row.Children.Add($lbl)
            [void]$p.Children.Add($row)
        }
    }

    $dlg.Add_Loaded({
        if ($script:diagRebuild) {
            & $script:diagRebuild
        }
    })

    [void](Show-ZapretOwnedDialog -Dialog $dlg)
    $script:diagRebuild = $null
    $script:diagPanel = $null
}

function Add-FakeSlotComboItems {
    param(
        [System.Windows.Controls.ComboBox]$Combo,
        [object[]]$Files,
        [string]$CurrentName,
        [string]$State
    )
    $offset = 0
    if ($State -ne 'matched') {
        [void]$Combo.Items.Add((Get-ZapretFakeCurrentText -Name $CurrentName -State $State))
        $offset = 1
    }
    $selected = 0
    foreach ($item in @($Files)) {
        [void]$Combo.Items.Add([string]$item.Name)
        if ($State -eq 'matched' -and $item.Name -eq $CurrentName) {
            $selected = $Combo.Items.Count - 1
        }
    }
    if ($Combo.Items.Count -gt 0) {
        $Combo.SelectedIndex = $selected
    }
    return $offset
}

function Get-FakeSlotPickedFile {
    param(
        [System.Windows.Controls.ComboBox]$Combo,
        [int]$Offset,
        [object[]]$Files,
        [string]$CurrentName,
        [string]$State
    )
    $idx = $Combo.SelectedIndex
    if ($idx -lt 0) {
        return $null
    }
    if ($State -ne 'matched') {
        if ($idx -lt $Offset) {
            return $null
        }
        return @($Files)[$idx - $Offset]
    }
    $picked = @($Files)[$idx]
    if ($CurrentName -and $picked.Name -eq $CurrentName) {
        return $null
    }
    return $picked
}

function Show-FakesDialog {
    $catalog = Get-ZapretFakeCatalog
    $files = @($catalog.Files)
    if (@($files).Count -eq 0) {
        throw (Get-ZapretUiString -Key 'FakesNone')
    }

    $dlg = Import-ZapretXaml 'Fake.xaml'
    $dlg.Title = Get-ZapretUiString -Key 'FakesTitle'
    $lblDiscord = Get-XamlChild -Root $dlg -Name 'lblDiscord'
    $lblDiscord.Text = Get-ZapretUiString -Key 'FakesDiscord'
    $cmbDiscord = Get-XamlChild -Root $dlg -Name 'cmbDiscord'
    $discordOffset = Add-FakeSlotComboItems -Combo $cmbDiscord -Files $files -CurrentName ([string]$catalog.CurrentDiscord) -State ([string]$catalog.DiscordState)
    $lblGame = Get-XamlChild -Root $dlg -Name 'lblGame'
    $lblGame.Text = Get-ZapretUiString -Key 'FakesGame'
    $cmbGameFake = Get-XamlChild -Root $dlg -Name 'cmbGameFake'
    $gameOffset = Add-FakeSlotComboItems -Combo $cmbGameFake -Files $files -CurrentName ([string]$catalog.CurrentGame) -State ([string]$catalog.GameState)
    $btnOk = Get-XamlChild -Root $dlg -Name 'btnOk'
    $btnOk.Content = Get-ZapretUiString -Key 'BtnOk'
    $btnOk.Add_Click({
        $dlg.DialogResult = $true
    })
    $btnCancel = Get-XamlChild -Root $dlg -Name 'btnCancel'
    $btnCancel.Content = Get-ZapretUiString -Key 'BtnCancel'

    $result = Show-ZapretOwnedDialog -Dialog $dlg
    $choice = $null
    if ($result -eq $true) {
        $changes = New-Object System.Collections.ArrayList
        $discordPicked = Get-FakeSlotPickedFile -Combo $cmbDiscord -Offset $discordOffset -Files $files -CurrentName ([string]$catalog.CurrentDiscord) -State ([string]$catalog.DiscordState)
        if ($discordPicked) {
            [void]$changes.Add((New-Object PSObject -Property @{
                Slot = 'discord'
                Path = [string]$discordPicked.FullName
                Name = [string]$discordPicked.Name
            }))
        }
        $gamePicked = Get-FakeSlotPickedFile -Combo $cmbGameFake -Offset $gameOffset -Files $files -CurrentName ([string]$catalog.CurrentGame) -State ([string]$catalog.GameState)
        if ($gamePicked) {
            [void]$changes.Add((New-Object PSObject -Property @{
                Slot = 'game'
                Path = [string]$gamePicked.FullName
                Name = [string]$gamePicked.Name
            }))
        }
        $choice = New-Object PSObject -Property @{
            Changes = @($changes)
        }
    }
    return $choice
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
            Show-InfoDialog (Get-ZapretUiString -Key 'VersionFail')
            Write-GuiLog (Get-ZapretUiString -Key 'VersionFail')
        } else {
            Write-GuiLog (Get-ZapretUiString -Key 'AutoFail')
        }
        return
    }
    $remote = ([string]$RemoteText).Trim()
    $local = Get-ZapretLocalVersion
    if ($remote -eq $local) {
        if ($reportUpToDate) {
            Show-InfoDialog (Get-ZapretUiString -Key 'VersionLatest' -FormatArgs @($local))
            Write-GuiLog (Get-ZapretUiString -Key 'VersionLatest' -FormatArgs @($local))
        }
        return
    }
    if (-not $remote) {
        if ($reportUpToDate) {
            Show-InfoDialog (Get-ZapretUiString -Key 'VersionFail')
            Write-GuiLog (Get-ZapretUiString -Key 'VersionFail')
        }
        return
    }
    $answer = Show-QuestionDialog (Get-ZapretUiString -Key 'VersionNew' -FormatArgs @($remote))
    if ($answer -eq [System.Windows.MessageBoxResult]::Yes) {
        Start-Process (Get-ZapretReleasePageUrl)
    }
    Write-GuiLog (Get-ZapretUiString -Key 'VersionNew' -FormatArgs @($remote))
}

function Start-GuiVersionCheck {
    param([bool]$ReportUpToDate = $false)

    if ($script:versionCheckBusy) {
        return
    }
    $script:versionCheckBusy = $true
    $script:versionCheckReportUpToDate = $ReportUpToDate
    Enable-ZapretTls12
    $wc = New-Object System.Net.WebClient
    $wc.Headers.Add('Cache-Control', 'no-cache')
    $wc.Headers.Add('User-Agent', 'zapret')
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

function Invoke-GuiDownload {
    param(
        [string]$Url,
        [string]$Destination,
        [string]$Title
    )

    Enable-ZapretTls12
    $partial = $Destination + '.partial'
    $script:dlDone = $false
    $script:dlError = $null

    $dlg = Import-ZapretXaml 'Download.xaml'
    $dlg.Title = $Title
    $lbl = Get-XamlChild -Root $dlg -Name 'lblStatus'
    $lbl.Text = 'Connecting...'
    $bar = Get-XamlChild -Root $dlg -Name 'barProgress'
    $btnCancel = Get-XamlChild -Root $dlg -Name 'btnCancel'
    $btnCancel.Content = 'Cancel'
    if ($window) {
        $dlg.Owner = $window
        $dlg.WindowStartupLocation = [System.Windows.WindowStartupLocation]::CenterOwner
    }

    $script:dlBar = $bar
    $script:dlLabel = $lbl
    $wc = New-Object System.Net.WebClient
    $wc.Headers.Add('Cache-Control', 'no-cache')
    $wc.Headers.Add('User-Agent', 'zapret')

    $wc.add_DownloadProgressChanged({
        param($source, $e)
        try {
            if ($null -eq $source) { return }
            if (-not $e) { return }
            $script:dlBytes = $e.BytesReceived
            $script:dlTotal = $e.TotalBytesToReceive
            $script:dlPct = $e.ProgressPercentage
            Invoke-GuiOnUi {
                $total = $script:dlTotal
                $pct = $script:dlPct
                if ($total -gt 0 -and $pct -ge 0) {
                    $script:dlBar.IsIndeterminate = $false
                    $script:dlBar.Minimum = 0
                    $script:dlBar.Maximum = 100
                    $script:dlBar.Value = [Math]::Min(100, [Math]::Max(0, $pct))
                    $script:dlLabel.Text = ('Downloaded {0} of {1} KB ({2}%)' -f [int]($script:dlBytes / 1024), [int]($total / 1024), $pct)
                } else {
                    $script:dlLabel.Text = ('Downloaded {0} KB' -f [int]($script:dlBytes / 1024))
                }
            }
        } catch {
            return
        }
    })

    $wc.add_DownloadFileCompleted({
        param($source, $e)
        try {
            if ($null -eq $source) {
                return
            }
            if ($e -and $e.Cancelled) {
                $script:dlError = 'Download cancelled.'
            } elseif ($e -and $e.Error) {
                $script:dlError = $e.Error.Message
            }
        } catch {
            $script:dlError = 'Download failed.'
        }
        $script:dlDone = $true
    })

    $btnCancel.Add_Click({
        $wc.CancelAsync()
    })
    $dlg.Add_Closing({
        if (-not $script:dlDone) {
            $wc.CancelAsync()
        }
    })

    try {
        $dlg.Show()
        $wc.DownloadFileAsync([Uri]$Url, $partial)
        while (-not $script:dlDone) {
            Invoke-GuiPump
            Start-Sleep -Milliseconds 40
        }
    } finally {
        $dlg.Close()
        $wc.Dispose()
    }

    if ($script:dlError) {
        if (Test-Path -LiteralPath $partial) {
            Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
        }
        throw $script:dlError
    }
    if (-not (Test-Path -LiteralPath $partial)) {
        throw 'Download failed.'
    }
    if (Test-Path -LiteralPath $Destination) {
        Remove-Item -LiteralPath $Destination -Force
    }
    Move-Item -LiteralPath $partial -Destination $Destination -Force
}

function Show-GuiTestsSetup {
    $engine = Get-ZapretEngine
    if (-not (Test-ZapretEngineFiles -Engine $engine)) {
        if ($engine -eq 'winws2') {
            throw (Get-ZapretUiString -Key 'EngineNoWinws2')
        }
        throw (Get-ZapretUiString -Key 'EngineNoWinws')
    }
    $files = @(
        @(Get-ZapretStrategyFiles) | Where-Object {
            Test-ZapretStrategySupportsEngine -Path $_.FullName -Engine $engine
        }
    )
    if ($files.Count -eq 0) {
        throw (Get-ZapretUiString -Key 'EngineNoFlags' -FormatArgs @($engine))
    }

    $dlg = Import-ZapretXaml 'TestsSetup.xaml'
    $dlg.Title = 'Run Tests'
    $lblType = Get-XamlChild -Root $dlg -Name 'lblType'
    $lblType.Text = 'Test type'
    $rbStd = Get-XamlChild -Root $dlg -Name 'rbStd'
    $rbStd.Content = 'Standard (HTTP / ping)'
    $rbDpi = Get-XamlChild -Root $dlg -Name 'rbDpi'
    $rbDpi.Content = 'DPI checkers (TCP 16-20)'
    $lblPick = Get-XamlChild -Root $dlg -Name 'lblPick'
    $lblPick.Text = ('Strategies to test ({0})' -f $engine)
    $checks = Get-XamlChild -Root $dlg -Name 'listChecks'
    $selectedName = Get-ZapretRunningStrategyName
    if ([string]::IsNullOrWhiteSpace($selectedName)) {
        $selectedName = Get-ZapretInstalledStrategyName
    }
    foreach ($file in $files) {
        $cb = New-Object System.Windows.Controls.CheckBox
        $cb.Content = $file.BaseName
        $cb.Margin = New-Object System.Windows.Thickness(4, 2, 4, 2)
        if ($file.BaseName -eq $selectedName) {
            $cb.IsChecked = $true
        }
        [void]$checks.Items.Add($cb)
    }
    $checkedCount = @($checks.Items | Where-Object { $_.IsChecked -eq $true }).Count
    if ($checkedCount -eq 0 -and $checks.Items.Count -gt 0) {
        $checks.Items[0].IsChecked = $true
    }
    $btnAll = Get-XamlChild -Root $dlg -Name 'btnAll'
    $btnAll.Content = 'All'
    $btnAll.Add_Click({
        foreach ($cb in @($checks.Items)) {
            $cb.IsChecked = $true
        }
    })
    $btnNone = Get-XamlChild -Root $dlg -Name 'btnNone'
    $btnNone.Content = 'None'
    $btnNone.Add_Click({
        foreach ($cb in @($checks.Items)) {
            $cb.IsChecked = $false
        }
    })
    $btnRun = Get-XamlChild -Root $dlg -Name 'btnRun'
    $btnRun.Content = 'Start tests'
    $btnRun.Add_Click({
        $dlg.DialogResult = $true
    })
    $btnCancel = Get-XamlChild -Root $dlg -Name 'btnCancel'
    $btnCancel.Content = 'Cancel'

    $result = Show-ZapretOwnedDialog -Dialog $dlg
    $choice = $null
    if ($result -eq $true) {
        $names = New-Object System.Collections.Generic.List[string]
        foreach ($cb in @($checks.Items)) {
            if ($cb.IsChecked -eq $true) {
                [void]$names.Add([string]$cb.Content)
            }
        }
        if ($names.Count -eq 0) {
            throw 'Select at least one strategy.'
        }
        $kind = 'standard'
        if ($rbDpi.IsChecked -eq $true) {
            $kind = 'dpi'
        }
        $choice = New-Object PSObject -Property @{
            TestType = $kind
            Names    = @($names)
        }
    }
    return $choice
}

function Add-GuiTestLine {
    param(
        [string]$Text,
        [bool]$NoNewline
    )
    if (-not $script:testBox) {
        return
    }
    if ($NoNewline) {
        $script:testBox.AppendText($Text)
    } else {
        $script:testBox.AppendText($Text + [Environment]::NewLine)
    }
    $script:testBox.ScrollToEnd()
    if ($Text -match '\[(\d+)/(\d+)\]') {
        $total = [int]$matches[2]
        if ($total -lt 1) {
            $total = 1
        }
        if ($script:testBar) {
            $script:testBar.IsIndeterminate = $false
            $script:testBar.Minimum = 0
            $script:testBar.Maximum = $total
            $cur = [int]$matches[1]
            if ($cur -gt $script:testBar.Maximum) { $cur = [int]$script:testBar.Maximum }
            if ($cur -lt 0) { $cur = 0 }
            $script:testBar.Value = $cur
        }
        $script:testLbl.Text = ('Testing {0} / {1}' -f $matches[1], $matches[2])
    }
}

function Start-GuiTestRun {
    param(
        [string]$TestType,
        [string[]]$Names
    )

    $nameList = @($Names)
    $dlg = Import-ZapretXaml 'Tests.xaml'
    $dlg.Title = 'Tests'
    $lbl = Get-XamlChild -Root $dlg -Name 'lblStatus'
    $lbl.Text = 'Starting tests...'
    $bar = Get-XamlChild -Root $dlg -Name 'barProgress'
    $box = Get-XamlChild -Root $dlg -Name 'txtLog'
    $btnStop = Get-XamlChild -Root $dlg -Name 'btnStop'
    $btnStop.Content = 'Cancel'

    $script:testExited = $false
    $script:testCancel = $false
    $script:testShown = $false
    $script:testDlg = $dlg
    $script:testBar = $bar
    $script:testBox = $box
    $script:testLbl = $lbl
    $script:testBtn = $btnStop
    $script:testType = $TestType
    $script:testNames = $nameList

    $btnStop.Add_Click({
        if (-not $script:testExited) {
            $script:testCancel = $true
            Stop-ZapretWinwsProcess
            if ($script:testLbl) {
                $script:testLbl.Text = 'Cancelling...'
            }
            return
        }
        if ($script:testDlg) {
            $script:testDlg.Close()
        }
    })
    $dlg.Add_Closing({
        if (-not $script:testExited) {
            $script:testCancel = $true
            Stop-ZapretWinwsProcess
            $_.Cancel = $true
        }
    })
    $dlg.Add_ContentRendered({
        if ($script:testShown) {
            return
        }
        $script:testShown = $true
        try {
            Invoke-ZapretStrategyTests -TestType $script:testType -Names @($script:testNames) -OnLine {
                param($text, $noNewline)
                Add-GuiTestLine -Text $text -NoNewline ([bool]$noNewline)
                Invoke-GuiPump
            } -ShouldStop { [bool]$script:testCancel }
        } catch {
            Add-GuiTestLine -Text $_.Exception.Message -NoNewline $false
        } finally {
            $script:testExited = $true
            if ($script:testBar) {
                $script:testBar.IsIndeterminate = $false
                if ($script:testBar.Maximum -lt 1) {
                    $script:testBar.Maximum = 1
                }
                $script:testBar.Value = $script:testBar.Maximum
            }
            if ($script:testLbl) {
                if ($script:testCancel) {
                    $script:testLbl.Text = 'Cancelled.'
                } else {
                    $script:testLbl.Text = 'Tests finished. See test-results\ for the saved log.'
                }
            }
            if ($script:testBtn) {
                $script:testBtn.Content = 'Close'
            }
        }
    })

    try {
        [void](Show-ZapretOwnedDialog -Dialog $dlg)
    } finally {
        $script:testDlg = $null
        $script:testBar = $null
        $script:testBox = $null
        $script:testLbl = $null
        $script:testBtn = $null
        $script:testNames = $null
    }
}

function Get-SelectedStrategy {
    param($ListBox)
    $sel = $ListBox.SelectedItem
    if ($null -eq $sel) {
        return $null
    }
    $name = $null
    if ($sel -is [System.Windows.Controls.ListBoxItem]) {
        $name = [string]$sel.Tag
    } else {
        $name = [string]$sel
    }
    if ([string]::IsNullOrWhiteSpace($name)) {
        return $null
    }
    if (-not $script:strategyMap.ContainsKey($name)) {
        return $null
    }
    return $script:strategyMap[$name]
}

function Show-StatusJournalDialog {
    $status = @((@(Get-ZapretStatusLines)) -join [Environment]::NewLine)
    $journal = @((@($script:guiJournal)) -join [Environment]::NewLine)
    $text = $status + [Environment]::NewLine + [Environment]::NewLine + (Get-ZapretUiString -Key 'JournalHeader') + [Environment]::NewLine + $journal
    $dlg = Import-ZapretXaml 'Status.xaml'
    $dlg.Title = Get-ZapretUiString -Key 'StatusDlgTitle'
    $box = Get-XamlChild -Root $dlg -Name 'txtStatus'
    $box.Text = $text
    $box.CaretIndex = 0
    $copy = Get-XamlChild -Root $dlg -Name 'btnCopy'
    $copy.Content = Get-ZapretUiString -Key 'BtnCopy'
    $copy.Add_Click({
        try {
            [System.Windows.Clipboard]::SetText($box.Text)
        } catch {
            Show-ErrorDialog $_.Exception.Message
        }
    })
    $ok = Get-XamlChild -Root $dlg -Name 'btnClose'
    $ok.Content = Get-ZapretUiString -Key 'BtnClose'
    $ok.Add_Click({
        $dlg.DialogResult = $true
    })
    [void](Show-ZapretOwnedDialog -Dialog $dlg)
}

function Update-StrategyListMarks {
    param($ListBox)
    $runningName = Get-ZapretRunningStrategyName
    $installedName = Get-ZapretInstalledStrategyName
    $running = Test-ZapretBypassRunning
    $eng = Get-ZapretEngine
    foreach ($it in @($ListBox.Items)) {
        if (-not ($it -is [System.Windows.Controls.ListBoxItem])) {
            continue
        }
        $name = [string]$it.Tag
        $label = $name
        if ($running -and $name -eq $runningName) {
            $label = $name + (Get-ZapretUiString -Key 'StratMarkRun')
        } elseif ($name -eq $installedName) {
            $label = $name + (Get-ZapretUiString -Key 'StratMarkService')
        }
        $it.Content = $label
        $file = $null
        if ($script:strategyMap.ContainsKey($name)) {
            $file = $script:strategyMap[$name]
        }
        $ok = $false
        if ($file) {
            $ok = Test-ZapretStrategySupportsEngine -Path $file.FullName -Engine $eng
        }
        if ($ok) {
            $it.ClearValue([System.Windows.Controls.Control]::ForegroundProperty)
        } else {
            $it.Foreground = [System.Windows.Media.Brushes]::Gray
        }
    }
}

function Show-StrategyDialog {
    $dlg = Import-ZapretXaml 'Strategy.xaml'
    $dlg.Title = Get-ZapretUiString -Key 'StratTitle'
    $lblEngine = Get-XamlChild -Root $dlg -Name 'lblEngine'
    $lblEngine.Text = Get-ZapretUiString -Key 'LblEngine'
    $rbWinws = Get-XamlChild -Root $dlg -Name 'rbWinws'
    $rbWinws.Content = 'winws'
    $rbWinws2 = Get-XamlChild -Root $dlg -Name 'rbWinws2'
    $rbWinws2.Content = 'winws2'
    if ((Get-ZapretEngine) -eq 'winws2') {
        $rbWinws2.IsChecked = $true
    } else {
        $rbWinws.IsChecked = $true
    }
    $lb = Get-XamlChild -Root $dlg -Name 'listStrategies'
    foreach ($name in @($script:strategyMap.Keys | Sort-Object)) {
        $item = New-Object System.Windows.Controls.ListBoxItem
        $item.Content = $name
        $item.Tag = $name
        [void]$lb.Items.Add($item)
    }
    Select-InstalledOrFirstStrategy -ListBox $lb
    $btnRun = Get-XamlChild -Root $dlg -Name 'btnRun'
    $btnRun.Content = Get-ZapretUiString -Key 'BtnRunSelected'
    $btnInst = Get-XamlChild -Root $dlg -Name 'btnInst'
    $btnInst.Content = Get-ZapretUiString -Key 'BtnInstall'
    $btnT = Get-XamlChild -Root $dlg -Name 'btnTests'
    $btnT.Content = Get-ZapretUiString -Key 'BtnTests'
    $syncEngineUi = {
        $eng = 'winws'
        if ($rbWinws2.IsChecked -eq $true) {
            $eng = 'winws2'
        }
        Set-ZapretEngine -Engine $eng | Out-Null
        Update-StrategyListMarks -ListBox $lb
        $file = Get-SelectedStrategy -ListBox $lb
        $can = $false
        if ($file -and (Test-ZapretStrategySupportsEngine -Path $file.FullName -Engine $eng)) {
            if (Test-ZapretEngineFiles -Engine $eng) {
                $can = $true
            }
        }
        $btnRun.IsEnabled = $can
        $btnInst.IsEnabled = $can
    }
    $rbWinws.Add_Checked({ & $syncEngineUi })
    $rbWinws2.Add_Checked({ & $syncEngineUi })
    $lb.Add_SelectionChanged({ & $syncEngineUi })
    & $syncEngineUi
    $btnRun.Add_Click({
        $file = Get-SelectedStrategy -ListBox $lb
        if (-not $file) {
            Show-ErrorDialog (Get-ZapretUiString -Key 'NoStrategies')
            return
        }
        $live = $false
        if (Test-ZapretBypassRunning) {
            $live = $true
        } else {
            $svcLive = Get-ZapretService
            if ($svcLive -and $svcLive.Status -eq 'Running') {
                $live = $true
            }
        }
        if ($live) {
            $ans = Show-QuestionDialog -Message (Get-ZapretUiString -Key 'ConfirmRunOver') -DefaultYes $false
            if ($ans -ne [System.Windows.MessageBoxResult]::Yes) {
                return
            }
        }
        Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnRunSelected') -FreezeUi -Action {
            Start-ZapretSelectedStrategy -File $file -OnWait { Invoke-GuiPump }
            Write-GuiLog (Get-ZapretUiString -Key 'RunDone' -FormatArgs @($file.BaseName))
        }
        Update-StrategyListMarks -ListBox $lb
    })
    $btnInst.Add_Click({
        $file = Get-SelectedStrategy -ListBox $lb
        if (-not $file) {
            Show-ErrorDialog (Get-ZapretUiString -Key 'NoStrategies')
            return
        }
        $script:installOk = $false
        Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnInstall') -FreezeUi -Action {
            Install-ZapretService -File $file -OnWait { Invoke-GuiPump }
            Write-GuiLog (Get-ZapretUiString -Key 'InstallDone' -FormatArgs @($file.BaseName))
            $script:installOk = $true
        }
        if ($script:installOk) {
            $dlg.Close()
        }
    })
    $btnT.Add_Click({
        $script:strategyPendingTests = $true
        $dlg.Close()
    })
    $script:strategyDialogOpen = $true
    $script:strategyPendingTests = $false
    try {
        [void](Show-ZapretOwnedDialog -Dialog $dlg)
    } finally {
        $script:strategyDialogOpen = $false
        Update-Status
    }
    if ($script:strategyPendingTests) {
        $script:strategyPendingTests = $false
        Invoke-GuiTestsFromMain
    }
}

function Invoke-GuiTestsFromMain {
    if (Get-ZapretService) {
        Write-GuiLog (Get-ZapretUiString -Key 'TestsNeedNoService')
        Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnRemove') -FreezeUi -Action {
            Remove-ZapretServices -OnWait { Invoke-GuiPump }
            Write-GuiLog (Get-ZapretUiString -Key 'RemoveDone')
        }
        if (Get-ZapretService) {
            return
        }
    }
    try {
        $setup = Show-GuiTestsSetup
    } catch {
        Show-ErrorDialog $_.Exception.Message
        return
    }
    if (-not $setup) {
        return
    }
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnTests') -Action {
        Start-GuiTestRun -TestType $setup.TestType -Names @($setup.Names)
        Write-GuiLog (Get-ZapretUiString -Key 'TestsFinished')
    }
}

function Get-GameFilterApplyHint {
    if (Get-ZapretService) {
        return 'Game Filter saved. Run Install Service again to apply.'
    }
    return 'Game Filter saved. Stop and Run selected to apply.'
}

function Get-IpsetApplyHint {
    if (Get-ZapretService) {
        return 'IPSet Filter saved. Stop, then Start service to apply.'
    }
    return 'IPSet Filter saved. Stop and Run selected to apply.'
}

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
Add-GuiStartupMark 'hide-console'

if (-not (Test-IsAdministrator)) {
    if (-not (Request-Administrator)) {
        Show-ConsoleWindow
        Show-ErrorDialog (Get-ZapretUiString -Key 'AdminRequired')
        exit 1
    }
    exit 0
}

Initialize-ZapretUserLists

$strategies = @(Get-ZapretStrategyFiles)
$script:strategyMap = @{}
foreach ($file in $strategies) {
    $script:strategyMap[$file.BaseName] = $file
}

$window = Import-ZapretXaml 'Main.xaml'
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
    $ver = Get-ZapretLocalVersion
    $window.Title = Get-ZapretUiString -Key 'AppTitle' -FormatArgs @($ver)
    $title.Text = Get-ZapretUiString -Key 'AppName'
    $grpStatus.Header = Get-ZapretUiString -Key 'GrpStatus'
    $grpSettings.Header = Get-ZapretUiString -Key 'GrpSettings'
    $grpTools.Header = Get-ZapretUiString -Key 'GrpTools'
    $btnStart.Content = Get-ZapretUiString -Key 'BtnStart'
    $btnStop.Content = Get-ZapretUiString -Key 'BtnStop'
    $btnRemove.Content = Get-ZapretUiString -Key 'BtnRemove'
    $btnStrategy.Content = Get-ZapretUiString -Key 'BtnStrategy'
    $btnFakes.Content = Get-ZapretUiString -Key 'BtnFakes'
    $btnIpsetUpd.Content = Get-ZapretUiString -Key 'BtnIpset'
    $btnHosts.Content = Get-ZapretUiString -Key 'BtnHosts'
    $btnUpdates.Content = Get-ZapretUiString -Key 'BtnVersion'
    $btnDiag.Content = Get-ZapretUiString -Key 'BtnDiag'
    $chkAuto.Content = Get-ZapretUiString -Key 'ChkAuto'
    $lblGame.Text = Get-ZapretUiString -Key 'LblGame'
    $lblIpset.Text = Get-ZapretUiString -Key 'LblIpset'
    $rbRu.Content = Get-ZapretUiString -Key 'LangRu'
    $rbEn.Content = Get-ZapretUiString -Key 'LangEn'
    $btnStart.ToolTip = Get-ZapretUiString -Key 'TipStart'
    $btnStop.ToolTip = Get-ZapretUiString -Key 'TipStop'
    $btnRemove.ToolTip = Get-ZapretUiString -Key 'TipRemove'
    $btnStrategy.ToolTip = Get-ZapretUiString -Key 'TipStrategy'
    $btnFakes.ToolTip = Get-ZapretUiString -Key 'TipFakes'
    $btnIpsetUpd.ToolTip = Get-ZapretUiString -Key 'TipIpset'
    $btnHosts.ToolTip = Get-ZapretUiString -Key 'TipHosts'
    $btnUpdates.ToolTip = Get-ZapretUiString -Key 'TipVersion'
    $btnDiag.ToolTip = Get-ZapretUiString -Key 'TipDiag'
    $chkAuto.ToolTip = Get-ZapretUiString -Key 'TipAuto'
    $grpStatus.ToolTip = Get-ZapretUiString -Key 'TipStatusClick'
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
        $lblBypass.Text = Get-ZapretUiString -Key 'StatusBypassOn' -FormatArgs @($bypassName)
    } else {
        $lblBypass.Text = Get-ZapretUiString -Key 'StatusBypassOff'
    }

    $svc = Get-ZapretService
    if ($svc) {
        $lblService.Text = Get-ZapretUiString -Key 'StatusServiceOn' -FormatArgs @($svc.Status)
    } else {
        $lblService.Text = Get-ZapretUiString -Key 'StatusServiceOff'
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
            $lblInstalled.Text = Get-ZapretUiString -Key 'StatusStrategy' -FormatArgs @($runningName)
        } else {
            $lblInstalled.Text = Get-ZapretUiString -Key 'StatusStrategyRun' -FormatArgs @($runningName)
        }
    } elseif ([string]::IsNullOrWhiteSpace($installed)) {
        if ($running -and -not $svc) {
            $lblInstalled.Text = Get-ZapretUiString -Key 'StatusStrategyManual'
        } else {
            $lblInstalled.Text = Get-ZapretUiString -Key 'StatusStrategyNone'
        }
    } elseif (-not $hasStrategies) {
        $lblInstalled.Text = Get-ZapretUiString -Key 'StatusStrategyGone' -FormatArgs @($installed)
    } else {
        $lblInstalled.Text = Get-ZapretUiString -Key 'StatusStrategy' -FormatArgs @($installed)
    }

    $wd = Get-Service -Name 'WinDivert' -ErrorAction SilentlyContinue
    $pending = $false
    if ($wd -and $wd.Status -eq 'Running') {
        $lblDivert.Text = Get-ZapretUiString -Key 'StatusDivertOn'
    } elseif ($wd -and ([string]$wd.Status -eq 'StopPending')) {
        $lblDivert.Text = Get-ZapretUiString -Key 'StatusDivertPending'
        $pending = $true
    } elseif ($wd) {
        $lblDivert.Text = Get-ZapretUiString -Key 'StatusDivertOther' -FormatArgs @($wd.Status)
    } else {
        $lblDivert.Text = Get-ZapretUiString -Key 'StatusDivertNone'
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
            $lblBanner.Text = Get-ZapretUiString -Key $bannerKey -FormatArgs @($bypassName)
        } else {
            $lblBanner.Text = Get-ZapretUiString -Key $bannerKey
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

    $autoOn = Test-ZapretAutoUpdateEnabled
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
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnStart') -FreezeUi -Action {
        Start-ZapretServiceIfInstalled -OnWait { Invoke-GuiPump }
        $name = Get-ZapretInstalledStrategyName
        if ($name) {
            Write-GuiLog (Get-ZapretUiString -Key 'StartDoneName' -FormatArgs @($name))
        } else {
            Write-GuiLog (Get-ZapretUiString -Key 'StartDone')
        }
    }
})

$btnStop.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnStop') -FreezeUi -Action {
        Stop-ZapretBypass -OnWait { Invoke-GuiPump }
        Write-GuiLog (Get-ZapretUiString -Key 'StopDone')
    }
})

$btnRemove.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnRemove') -FreezeUi -Action {
        Remove-ZapretServices -OnWait { Invoke-GuiPump }
        Write-GuiLog (Get-ZapretUiString -Key 'RemoveDone')
    }
})

$btnStrategy.Add_Click({
    if ($script:strategyMap.Count -lt 1) {
        Show-ErrorDialog (Get-ZapretUiString -Key 'NoStrategies')
        return
    }
    Show-StrategyDialog
})

# One handler on the group. Child MouseLeftButtonUp bubbles; extra handlers
# opened a new dialog after each Close (WinForms Click does not bubble).
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
            $ans = Show-QuestionDialog -Message (Get-ZapretUiString -Key 'FilterNeedInstall')
            if ($ans -eq [System.Windows.MessageBoxResult]::Yes) {
                Show-StrategyDialog
            }
        } else {
            Show-InfoDialog (Get-ZapretUiString -Key 'FilterNeedRerun')
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
            $ans = Show-QuestionDialog -Message (Get-ZapretUiString -Key 'FilterNeedRestart')
            if ($ans -eq [System.Windows.MessageBoxResult]::Yes) {
                Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnStop') -FreezeUi -Action {
                    Stop-ZapretBypass -OnWait { Invoke-GuiPump }
                    Start-ZapretServiceIfInstalled -OnWait { Invoke-GuiPump }
                    Write-GuiLog (Get-ZapretUiString -Key 'StartDone')
                }
            }
        } else {
            Show-InfoDialog (Get-ZapretUiString -Key 'FilterNeedRerun')
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
        Set-ZapretAutoUpdateEnabled -Enabled $on
        if ($on) {
            Write-GuiLog (Get-ZapretUiString -Key 'AutoOn')
        } else {
            Write-GuiLog (Get-ZapretUiString -Key 'AutoOff')
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
            Write-GuiLog (Get-ZapretUiString -Key 'FakeDone' -FormatArgs @($item.Slot, $item.Name))
        }
        if (Get-ZapretService) {
            $ans = Show-QuestionDialog -Message (Get-ZapretUiString -Key 'FakeNeedRestart')
            if ($ans -eq [System.Windows.MessageBoxResult]::Yes) {
                Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnStop') -FreezeUi -Action {
                    Stop-ZapretBypass -OnWait { Invoke-GuiPump }
                    Start-ZapretServiceIfInstalled -OnWait { Invoke-GuiPump }
                    Write-GuiLog (Get-ZapretUiString -Key 'StartDone')
                }
            }
        } else {
            Show-InfoDialog (Get-ZapretUiString -Key 'FakeNeedRerun')
        }
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
})

$btnIpsetUpd.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnIpset') -Action {
        $temp = Join-Path $env:TEMP 'zapret-ipset-all.txt'
        try {
            Invoke-GuiDownload -Url (Get-ZapretIpsetListUrl) -Destination $temp -Title (Get-ZapretUiString -Key 'IpsetTitle')
        } catch {
            if ($_.Exception.Message -eq 'Download cancelled.') {
                Write-GuiLog (Get-ZapretUiString -Key 'IpsetCancel')
                return
            }
            throw
        }
        Update-ZapretIpsetList -SourceFile $temp
        Write-GuiLog (Get-ZapretUiString -Key 'IpsetOk')
    }
})

$btnHosts.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnHosts') -Action {
        $temp = Join-Path $env:TEMP 'zapret_hosts.txt'
        try {
            Invoke-GuiDownload -Url ((Get-ZapretHostsSourceUrl) + '?t=' + [guid]::NewGuid().ToString()) -Destination $temp -Title (Get-ZapretUiString -Key 'HostsTitle')
            $info = Get-ZapretHostsUpdateInfo -TempFile $temp
        } catch {
            if ($_.Exception.Message -eq 'Download cancelled.') {
                Write-GuiLog (Get-ZapretUiString -Key 'HostsCancel')
                return
            }
            throw (Get-ZapretUiString -Key 'HostsFail')
        }
        if ($info.NeedsUpdate) {
            Open-ZapretHostsUpdate -Info $info
            Show-InfoDialog (Get-ZapretUiString -Key 'HostsNeed')
            Write-GuiLog (Get-ZapretUiString -Key 'HostsNeed')
        } else {
            Remove-Item -LiteralPath $info.TempFile -Force -ErrorAction SilentlyContinue
            Show-InfoDialog (Get-ZapretUiString -Key 'HostsOk')
            Write-GuiLog (Get-ZapretUiString -Key 'HostsOk')
        }
    }
})

$btnUpdates.Add_Click({
    if ($script:versionCheckBusy) {
        return
    }
    Write-GuiLog (Get-ZapretUiString -Key 'VersionChecking')
    Start-GuiVersionCheck -ReportUpToDate $true
})

$btnDiag.Add_Click({
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnDiag') -Action {
        Show-GuiDiagnostics
        Write-GuiLog (Get-ZapretUiString -Key 'DiagDone')
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

$window.Add_ContentRendered({
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
                $timer.Start()
                if (Test-ZapretAutoUpdateEnabled) {
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
})

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
        Set-ZapretUiLanguage -Language 'ru'
        Update-GuiLanguage
        Update-Status
    }
})
$rbEn.Add_Checked({
    if ($script:updatingLang) { return }
    if ($rbEn.IsChecked -eq $true) {
        Set-ZapretUiLanguage -Language 'en'
        Update-GuiLanguage
        Update-Status
    }
})

$script:updatingLang = $true
if ((Get-ZapretUiLanguage) -eq 'ru') {
    $rbRu.IsChecked = $true
} else {
    $rbEn.IsChecked = $true
}
$script:updatingLang = $false
Update-GuiLanguage
Add-GuiStartupMark 'build-form'

try {
    $app = New-Object System.Windows.Application
    $app.ShutdownMode = [System.Windows.ShutdownMode]::OnMainWindowClose
    # Application.Run is the main loop. ShowDialog on $window ends when a child dialog disables the owner.
    [void]$app.Run($window)
} catch {
    Show-ErrorDialog $_.Exception.Message
    exit 1
}
