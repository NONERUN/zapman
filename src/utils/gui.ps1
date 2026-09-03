# Zapret WinForms GUI.
# This script starts and stops strategies from the strategies folder.
# This script installs or removes the zapret Windows service.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

if ($PSVersionTable.PSVersion.Major -lt 3) {
    Write-Host 'ERROR: Windows PowerShell is too old. This program needs 3.0 or newer. Target: 5.1.'
    Write-Host 'On Windows 7 install WMF 5.1 and .NET Framework 4.5 or newer.'
    exit 1
}

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Zapret.psd1')

[void](Initialize-ZapretUiLanguage)

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
$script:guiJournal = New-Object System.Collections.Generic.List[string]

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
        return
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

function Show-InfoDialog {
    param([string]$Message)
    [System.Windows.Forms.MessageBox]::Show(
        $Message,
        'Zapret',
        [System.Windows.Forms.MessageBoxButtons]::OK,
        [System.Windows.Forms.MessageBoxIcon]::Information
    ) | Out-Null
}

function Show-QuestionDialog {
    param(
        [string]$Message,
        [bool]$DefaultYes = $true
    )
    $def = [System.Windows.Forms.MessageBoxDefaultButton]::Button1
    if (-not $DefaultYes) {
        $def = [System.Windows.Forms.MessageBoxDefaultButton]::Button2
    }
    return [System.Windows.Forms.MessageBox]::Show(
        $Message,
        'Zapret',
        [System.Windows.Forms.MessageBoxButtons]::YesNo,
        [System.Windows.Forms.MessageBoxIcon]::Question,
        $def
    )
}

function Show-TextDialog {
    param(
        [string]$Title,
        [string]$Text
    )
    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = $Title
    $dlg.StartPosition = 'CenterParent'
    $dlg.FormBorderStyle = 'FixedDialog'
    $dlg.MaximizeBox = $false
    $dlg.MinimizeBox = $false
    $dlg.ClientSize = New-Object System.Drawing.Size(520, 420)
    $dlg.Font = New-Object System.Drawing.Font('Consolas', 9)

    $box = New-Object System.Windows.Forms.TextBox
    $box.Multiline = $true
    $box.ReadOnly = $true
    $box.ScrollBars = 'Vertical'
    $box.WordWrap = $true
    $box.Location = New-Object System.Drawing.Point(12, 12)
    $box.Size = New-Object System.Drawing.Size(496, 360)
    $box.Text = $Text
    $box.Select(0, 0)
    $dlg.Controls.Add($box)

    $ok = New-Object System.Windows.Forms.Button
    $ok.Text = 'OK'
    $ok.Location = New-Object System.Drawing.Point(433, 380)
    $ok.Size = New-Object System.Drawing.Size(75, 28)
    $ok.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $dlg.Controls.Add($ok)
    $dlg.AcceptButton = $ok
    if ($form) {
        $dlg.Owner = $form
        $dlg.StartPosition = 'CenterParent'
    }
    [void]$dlg.ShowDialog()
    $dlg.Dispose()
}

function Show-GuiDiagnostics {
    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = Get-ZapretUiString -Key 'DiagTitle'
    $dlg.StartPosition = 'CenterParent'
    $dlg.FormBorderStyle = 'Sizable'
    $dlg.MaximizeBox = $true
    $dlg.MinimizeBox = $false
    $dlg.ShowInTaskbar = $false
    $dlg.ClientSize = New-Object System.Drawing.Size(640, 720)
    $dlg.MinimumSize = New-Object System.Drawing.Size(480, 480)
    $dlg.Font = New-Object System.Drawing.Font('Segoe UI', 9)
    if ($form) {
        $dlg.Owner = $form
        $dlg.StartPosition = 'CenterParent'
    }

    $scroll = New-Object System.Windows.Forms.Panel
    $scroll.Location = New-Object System.Drawing.Point(12, 12)
    $scroll.Size = New-Object System.Drawing.Size(616, 662)
    $scroll.Anchor = 'Top,Bottom,Left,Right'
    $scroll.AutoScroll = $true
    $dlg.Controls.Add($scroll)

    $ok = New-Object System.Windows.Forms.Button
    $ok.Text = Get-ZapretUiString -Key 'BtnOk'
    $ok.Location = New-Object System.Drawing.Point(553, 682)
    $ok.Size = New-Object System.Drawing.Size(75, 28)
    $ok.Anchor = 'Bottom,Right'
    $ok.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $dlg.Controls.Add($ok)
    $dlg.AcceptButton = $ok
    $dlg.CancelButton = $ok

    $script:diagScroll = $scroll
    $script:diagBusy = $false
    $script:diagRebuild = $null

    $script:diagRebuild = {
        $p = $script:diagScroll
        if (-not $p -or $p.IsDisposed) {
            return
        }
        $p.SuspendLayout()
        $p.Controls.Clear()
        $report = Get-ZapretDiagnosticReport
        $items = @($report.Items)
        $y = 4
        $innerW = $p.ClientSize.Width - 8
        if ($p.VerticalScroll.Visible) {
            $innerW = $p.ClientSize.Width - 24
        }
        if ($innerW -lt 280) {
            $innerW = 280
        }
        foreach ($item in $items) {
            $row = New-Object System.Windows.Forms.Panel
            $row.Location = New-Object System.Drawing.Point(0, $y)
            $row.Size = New-Object System.Drawing.Size($innerW, 30)
            $row.Anchor = 'Top,Left,Right'

            $hasAction = -not [string]::IsNullOrWhiteSpace([string]$item.Action)
            $markW = 22
            $btnW = 84
            $right = $innerW - 4

            $mark = New-Object System.Windows.Forms.Label
            $mark.Size = New-Object System.Drawing.Size($markW, 22)
            $mark.Location = New-Object System.Drawing.Point(($right - $markW), 4)
            $mark.Anchor = 'Top,Right'
            $mark.TextAlign = 'MiddleCenter'
            $mark.Font = New-Object System.Drawing.Font('Segoe UI', 11)
            if ($item.Status -eq 'ok') {
                $mark.Text = [string][char]0x2713
                $mark.ForeColor = [System.Drawing.Color]::ForestGreen
            } elseif ($item.Status -eq 'warn') {
                $mark.Text = '!'
                $mark.ForeColor = [System.Drawing.Color]::DarkOrange
            } else {
                $mark.Text = [string][char]0x2717
                $mark.ForeColor = [System.Drawing.Color]::Firebrick
            }
            $row.Controls.Add($mark)

            $lblW = $right - $markW - 12
            if ($hasAction) {
                $lblW = $right - $markW - $btnW - 20
            }
            $lbl = New-Object System.Windows.Forms.Label
            $lbl.Location = New-Object System.Drawing.Point(4, 6)
            $lbl.Size = New-Object System.Drawing.Size($lblW, 20)
            $lbl.AutoEllipsis = $true
            $lbl.Text = [string]$item.Text
            $row.Controls.Add($lbl)

            if ($hasAction) {
                $btn = New-Object System.Windows.Forms.Button
                $btn.Text = Get-ZapretUiString -Key 'DiagRun'
                $btn.Size = New-Object System.Drawing.Size($btnW, 24)
                $btn.Location = New-Object System.Drawing.Point(($right - $markW - 8 - $btnW), 3)
                $btn.Anchor = 'Top,Right'
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
                        $this.Enabled = $false
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
                $row.Controls.Add($btn)
            }

            $p.Controls.Add($row)
            $y += 30
        }
        $p.ResumeLayout()
    }

    $dlg.Add_Shown({
        if ($script:diagRebuild) {
            & $script:diagRebuild
        }
    })

    [void]$dlg.ShowDialog()
    $script:diagRebuild = $null
    $script:diagScroll = $null
    $dlg.Dispose()
}

function Show-FakesDialog {
    $catalog = Get-ZapretFakeCatalog
    $files = @($catalog.Files)
    if (@($files).Count -eq 0) {
        throw (Get-ZapretUiString -Key 'FakesNone')
    }

    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = Get-ZapretUiString -Key 'FakesTitle'
    $dlg.StartPosition = 'CenterParent'
    $dlg.FormBorderStyle = 'FixedDialog'
    $dlg.MaximizeBox = $false
    $dlg.MinimizeBox = $false
    $dlg.ClientSize = New-Object System.Drawing.Size(420, 320)
    $dlg.Font = New-Object System.Drawing.Font('Segoe UI', 9)

    $lblSlot = New-Object System.Windows.Forms.Label
    $lblSlot.Text = Get-ZapretUiString -Key 'FakesSlot'
    $lblSlot.Location = New-Object System.Drawing.Point(12, 16)
    $lblSlot.AutoSize = $true
    $dlg.Controls.Add($lblSlot)

    $cmbSlot = New-Object System.Windows.Forms.ComboBox
    $cmbSlot.DropDownStyle = 'DropDownList'
    $cmbSlot.Location = New-Object System.Drawing.Point(56, 12)
    $cmbSlot.Size = New-Object System.Drawing.Size(352, 24)
    [void]$cmbSlot.Items.Add(('Discord UDP (current: {0})' -f [string]$catalog.CurrentDiscord))
    [void]$cmbSlot.Items.Add(('GameFilter UDP (current: {0})' -f [string]$catalog.CurrentGame))
    $cmbSlot.SelectedIndex = 0
    $dlg.Controls.Add($cmbSlot)

    $lblFile = New-Object System.Windows.Forms.Label
    $lblFile.Text = Get-ZapretUiString -Key 'FakesFile'
    $lblFile.Location = New-Object System.Drawing.Point(12, 48)
    $lblFile.AutoSize = $true
    $dlg.Controls.Add($lblFile)

    $lbFiles = New-Object System.Windows.Forms.ListBox
    $lbFiles.Location = New-Object System.Drawing.Point(12, 68)
    $lbFiles.Size = New-Object System.Drawing.Size(396, 180)
    foreach ($item in $files) {
        [void]$lbFiles.Items.Add([string]$item.Name)
    }
    if ($lbFiles.Items.Count -gt 0) {
        $lbFiles.SelectedIndex = 0
    }
    $dlg.Controls.Add($lbFiles)

    $btnOk = New-Object System.Windows.Forms.Button
    $btnOk.Text = Get-ZapretUiString -Key 'BtnReplace'
    $btnOk.Location = New-Object System.Drawing.Point(232, 280)
    $btnOk.Size = New-Object System.Drawing.Size(86, 28)
    $btnOk.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $dlg.Controls.Add($btnOk)

    $btnCancel = New-Object System.Windows.Forms.Button
    $btnCancel.Text = Get-ZapretUiString -Key 'BtnCancel'
    $btnCancel.Location = New-Object System.Drawing.Point(322, 280)
    $btnCancel.Size = New-Object System.Drawing.Size(86, 28)
    $btnCancel.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
    $dlg.Controls.Add($btnCancel)
    $dlg.AcceptButton = $btnOk
    $dlg.CancelButton = $btnCancel

    $result = $null
    if ($form) {
        $dlg.Owner = $form
        $dlg.StartPosition = 'CenterParent'
    }
    $result = $dlg.ShowDialog()
    $choice = $null
    if ($result -eq [System.Windows.Forms.DialogResult]::OK -and $lbFiles.SelectedIndex -ge 0) {
        $slot = 'discord'
        if ($cmbSlot.SelectedIndex -eq 1) {
            $slot = 'game'
        }
        $picked = @($files)[$lbFiles.SelectedIndex]
        $choice = New-Object PSObject -Property @{
            Slot = [string]$slot
            Path = [string]$picked.FullName
            Name = [string]$picked.Name
        }
    }
    $dlg.Dispose()
    return $choice
}

function Invoke-GuiPump {
    [System.Windows.Forms.Application]::DoEvents()
}

function Stop-GuiVersionCheck {
    param([bool]$FromComplete = $false)
    if ($script:versionCheckTimer) {
        $script:versionCheckTimer.Stop()
        $script:versionCheckTimer.Dispose()
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
    if ($script:guiClosing -or $form.IsDisposed) {
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
    if ($answer -eq [System.Windows.Forms.DialogResult]::Yes) {
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
        Complete-GuiVersionCheck -ErrorObject $err -RemoteText $text -Cancelled $cancelled
    })
    $to = New-Object System.Windows.Forms.Timer
    $to.Interval = 8000
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

    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = $Title
    $dlg.FormBorderStyle = 'FixedDialog'
    $dlg.MaximizeBox = $false
    $dlg.MinimizeBox = $false
    $dlg.ControlBox = $true
    $dlg.ClientSize = New-Object System.Drawing.Size(420, 118)
    $dlg.StartPosition = 'CenterScreen'
    $dlg.Font = New-Object System.Drawing.Font('Segoe UI', 9)
    if ($form) {
        $dlg.Owner = $form
        $dlg.StartPosition = 'CenterParent'
    }

    $lbl = New-Object System.Windows.Forms.Label
    $lbl.Text = 'Connecting...'
    $lbl.Location = New-Object System.Drawing.Point(12, 12)
    $lbl.Size = New-Object System.Drawing.Size(396, 20)
    $dlg.Controls.Add($lbl)

    $bar = New-Object System.Windows.Forms.ProgressBar
    $bar.Location = New-Object System.Drawing.Point(12, 40)
    $bar.Size = New-Object System.Drawing.Size(396, 22)
    $bar.Style = 'Marquee'
    $bar.MarqueeAnimationSpeed = 40
    $dlg.Controls.Add($bar)

    $btnCancel = New-Object System.Windows.Forms.Button
    $btnCancel.Text = 'Cancel'
    $btnCancel.Location = New-Object System.Drawing.Point(325, 78)
    $btnCancel.Size = New-Object System.Drawing.Size(83, 28)
    $dlg.Controls.Add($btnCancel)

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
            $total = $e.TotalBytesToReceive
            $pct = $e.ProgressPercentage
            if ($total -gt 0 -and $pct -ge 0) {
                $script:dlBar.Style = 'Continuous'
                $script:dlBar.Minimum = 0
                $script:dlBar.Maximum = 100
                $script:dlBar.Value = [Math]::Min(100, [Math]::Max(0, $pct))
                $script:dlLabel.Text = ('Downloaded {0} of {1} KB ({2}%)' -f [int]($e.BytesReceived / 1024), [int]($total / 1024), $pct)
            } else {
                $script:dlLabel.Text = ('Downloaded {0} KB' -f [int]($e.BytesReceived / 1024))
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
    $dlg.Add_FormClosing({
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
        if (-not $dlg.IsDisposed) {
            $dlg.Close()
            $dlg.Dispose()
        }
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

    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = 'Run Tests'
    $dlg.FormBorderStyle = 'FixedDialog'
    $dlg.MaximizeBox = $false
    $dlg.MinimizeBox = $false
    $dlg.ClientSize = New-Object System.Drawing.Size(440, 420)
    $dlg.StartPosition = 'CenterScreen'
    $dlg.Font = New-Object System.Drawing.Font('Segoe UI', 9)
    if ($form) {
        $dlg.Owner = $form
        $dlg.StartPosition = 'CenterParent'
    }

    $lblType = New-Object System.Windows.Forms.Label
    $lblType.Text = 'Test type'
    $lblType.Location = New-Object System.Drawing.Point(12, 12)
    $lblType.AutoSize = $true
    $dlg.Controls.Add($lblType)

    $rbStd = New-Object System.Windows.Forms.RadioButton
    $rbStd.Text = 'Standard (HTTP / ping)'
    $rbStd.Location = New-Object System.Drawing.Point(12, 32)
    $rbStd.AutoSize = $true
    $rbStd.Checked = $true
    $dlg.Controls.Add($rbStd)

    $rbDpi = New-Object System.Windows.Forms.RadioButton
    $rbDpi.Text = 'DPI checkers (TCP 16-20)'
    $rbDpi.Location = New-Object System.Drawing.Point(220, 32)
    $rbDpi.AutoSize = $true
    $dlg.Controls.Add($rbDpi)

    $lblPick = New-Object System.Windows.Forms.Label
    $lblPick.Text = ('Strategies to test ({0})' -f $engine)
    $lblPick.Location = New-Object System.Drawing.Point(12, 62)
    $lblPick.AutoSize = $true
    $dlg.Controls.Add($lblPick)

    $checks = New-Object System.Windows.Forms.CheckedListBox
    $checks.Location = New-Object System.Drawing.Point(12, 82)
    $checks.Size = New-Object System.Drawing.Size(416, 250)
    $checks.CheckOnClick = $true
    $selectedName = Get-ZapretRunningStrategyName
    if ([string]::IsNullOrWhiteSpace($selectedName)) {
        $selectedName = Get-ZapretInstalledStrategyName
    }
    foreach ($file in $files) {
        $idx = $checks.Items.Add($file.BaseName)
        if ($file.BaseName -eq $selectedName) {
            $checks.SetItemChecked($idx, $true)
        }
    }
    if ($checks.CheckedItems.Count -eq 0 -and $checks.Items.Count -gt 0) {
        $checks.SetItemChecked(0, $true)
    }
    $dlg.Controls.Add($checks)

    $btnAll = New-Object System.Windows.Forms.Button
    $btnAll.Text = 'All'
    $btnAll.Location = New-Object System.Drawing.Point(12, 344)
    $btnAll.Size = New-Object System.Drawing.Size(80, 28)
    $btnAll.Add_Click({
        for ($i = 0; $i -lt $checks.Items.Count; $i++) {
            $checks.SetItemChecked($i, $true)
        }
    })
    $dlg.Controls.Add($btnAll)

    $btnNone = New-Object System.Windows.Forms.Button
    $btnNone.Text = 'None'
    $btnNone.Location = New-Object System.Drawing.Point(98, 344)
    $btnNone.Size = New-Object System.Drawing.Size(80, 28)
    $btnNone.Add_Click({
        for ($i = 0; $i -lt $checks.Items.Count; $i++) {
            $checks.SetItemChecked($i, $false)
        }
    })
    $dlg.Controls.Add($btnNone)

    $btnRun = New-Object System.Windows.Forms.Button
    $btnRun.Text = 'Start tests'
    $btnRun.Location = New-Object System.Drawing.Point(248, 378)
    $btnRun.Size = New-Object System.Drawing.Size(90, 28)
    $btnRun.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $dlg.Controls.Add($btnRun)

    $btnCancel = New-Object System.Windows.Forms.Button
    $btnCancel.Text = 'Cancel'
    $btnCancel.Location = New-Object System.Drawing.Point(344, 378)
    $btnCancel.Size = New-Object System.Drawing.Size(84, 28)
    $btnCancel.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
    $dlg.Controls.Add($btnCancel)
    $dlg.AcceptButton = $btnRun
    $dlg.CancelButton = $btnCancel

    $result = $dlg.ShowDialog()
    $choice = $null
    if ($result -eq [System.Windows.Forms.DialogResult]::OK) {
        $names = New-Object System.Collections.Generic.List[string]
        foreach ($item in @($checks.CheckedItems)) {
            [void]$names.Add([string]$item)
        }
        if ($names.Count -eq 0) {
            $dlg.Dispose()
            throw 'Select at least one strategy.'
        }
        $kind = 'standard'
        if ($rbDpi.Checked) {
            $kind = 'dpi'
        }
        $choice = New-Object PSObject -Property @{
            TestType = $kind
            Names    = @($names)
        }
    }
    $dlg.Dispose()
    return $choice
}

function Add-GuiTestLine {
    param(
        [string]$Text,
        [bool]$NoNewline
    )
    if (-not $script:testBox -or $script:testBox.IsDisposed) {
        return
    }
    if ($NoNewline) {
        $script:testBox.AppendText($Text)
    } else {
        $script:testBox.AppendText($Text + [Environment]::NewLine)
    }
    if ($Text -match '\[(\d+)/(\d+)\]') {
        $total = [int]$matches[2]
        if ($total -lt 1) {
            $total = 1
        }
        if ($script:testBar -and -not $script:testBar.IsDisposed) {
            $script:testBar.Style = 'Continuous'
            $script:testBar.Minimum = 0
            $script:testBar.Maximum = $total
            $cur = [int]$matches[1]
            if ($cur -gt $script:testBar.Maximum) { $cur = $script:testBar.Maximum }
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
    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = 'Tests'
    $dlg.FormBorderStyle = 'Sizable'
    $dlg.MaximizeBox = $true
    $dlg.MinimizeBox = $false
    $dlg.ShowInTaskbar = $false
    $dlg.ClientSize = New-Object System.Drawing.Size(640, 460)
    $dlg.MinimumSize = New-Object System.Drawing.Size(480, 320)
    $dlg.StartPosition = 'CenterScreen'
    $dlg.Font = New-Object System.Drawing.Font('Consolas', 9)
    if ($form) {
        $dlg.Owner = $form
        $dlg.StartPosition = 'CenterParent'
    }

    $lbl = New-Object System.Windows.Forms.Label
    $lbl.Text = 'Starting tests...'
    $lbl.Location = New-Object System.Drawing.Point(12, 12)
    $lbl.Size = New-Object System.Drawing.Size(616, 20)
    $lbl.Anchor = 'Top,Left,Right'
    $dlg.Controls.Add($lbl)

    $bar = New-Object System.Windows.Forms.ProgressBar
    $bar.Location = New-Object System.Drawing.Point(12, 36)
    $bar.Size = New-Object System.Drawing.Size(616, 18)
    $bar.Anchor = 'Top,Left,Right'
    $bar.Style = 'Marquee'
    $bar.MarqueeAnimationSpeed = 40
    $dlg.Controls.Add($bar)

    $box = New-Object System.Windows.Forms.TextBox
    $box.Multiline = $true
    $box.ReadOnly = $true
    $box.ScrollBars = 'Both'
    $box.WordWrap = $false
    $box.Location = New-Object System.Drawing.Point(12, 62)
    $box.Size = New-Object System.Drawing.Size(616, 350)
    $box.Anchor = 'Top,Bottom,Left,Right'
    $dlg.Controls.Add($box)

    $btnStop = New-Object System.Windows.Forms.Button
    $btnStop.Text = 'Cancel'
    $btnStop.Location = New-Object System.Drawing.Point(533, 422)
    $btnStop.Size = New-Object System.Drawing.Size(95, 28)
    $btnStop.Anchor = 'Bottom,Right'
    $dlg.Controls.Add($btnStop)

    $script:testExited = $false
    $script:testCancel = $false
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
    $dlg.Add_FormClosing({
        if (-not $script:testExited) {
            $script:testCancel = $true
            Stop-ZapretWinwsProcess
            $_.Cancel = $true
        }
    })
    $dlg.Add_Shown({
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
            if ($script:testBar -and -not $script:testBar.IsDisposed) {
                $script:testBar.Style = 'Continuous'
                if ($script:testBar.Maximum -lt 1) {
                    $script:testBar.Maximum = 1
                }
                $script:testBar.Value = $script:testBar.Maximum
            }
            if ($script:testLbl -and -not $script:testLbl.IsDisposed) {
                if ($script:testCancel) {
                    $script:testLbl.Text = 'Cancelled.'
                } else {
                    $script:testLbl.Text = 'Tests finished. See test-results\ for the saved log.'
                }
            }
            if ($script:testBtn -and -not $script:testBtn.IsDisposed) {
                $script:testBtn.Text = 'Close'
            }
        }
    })

    try {
        [void]$dlg.ShowDialog()
    } finally {
        if (-not $dlg.IsDisposed) {
            $dlg.Dispose()
        }
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
    $name = [string]$ListBox.SelectedItem
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
    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = Get-ZapretUiString -Key 'StatusDlgTitle'
    $dlg.StartPosition = 'CenterParent'
    $dlg.FormBorderStyle = 'FixedDialog'
    $dlg.MaximizeBox = $false
    $dlg.MinimizeBox = $false
    $dlg.ClientSize = New-Object System.Drawing.Size(520, 420)
    $dlg.Font = New-Object System.Drawing.Font('Consolas', 9)
    $box = New-Object System.Windows.Forms.TextBox
    $box.Multiline = $true
    $box.ReadOnly = $true
    $box.ScrollBars = 'Vertical'
    $box.WordWrap = $true
    $box.Location = New-Object System.Drawing.Point(12, 12)
    $box.Size = New-Object System.Drawing.Size(496, 360)
    $box.Text = $text
    $box.Select(0, 0)
    $dlg.Controls.Add($box)
    $copy = New-Object System.Windows.Forms.Button
    $copy.Text = Get-ZapretUiString -Key 'BtnCopy'
    $copy.Location = New-Object System.Drawing.Point(344, 380)
    $copy.Size = New-Object System.Drawing.Size(86, 28)
    $copy.Add_Click({
        try {
            [System.Windows.Forms.Clipboard]::SetText($box.Text)
        } catch {
            Show-ErrorDialog $_.Exception.Message
        }
    })
    $dlg.Controls.Add($copy)
    $ok = New-Object System.Windows.Forms.Button
    $ok.Text = Get-ZapretUiString -Key 'BtnClose'
    $ok.Location = New-Object System.Drawing.Point(434, 380)
    $ok.Size = New-Object System.Drawing.Size(75, 28)
    $ok.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $dlg.Controls.Add($ok)
    $dlg.AcceptButton = $ok
    $dlg.Owner = $form
    [void]$dlg.ShowDialog()
    $dlg.Dispose()
}

function Show-StrategyDialog {
    $dlg = New-Object System.Windows.Forms.Form
    $dlg.Text = Get-ZapretUiString -Key 'StratTitle'
    $dlg.StartPosition = 'CenterParent'
    $dlg.FormBorderStyle = 'FixedDialog'
    $dlg.MaximizeBox = $false
    $dlg.MinimizeBox = $false
    $dlg.ClientSize = New-Object System.Drawing.Size(420, 356)
    $dlg.Font = New-Object System.Drawing.Font('Segoe UI', 9)
    $lblEngine = New-Object System.Windows.Forms.Label
    $lblEngine.Text = Get-ZapretUiString -Key 'LblEngine'
    $lblEngine.Location = New-Object System.Drawing.Point(12, 14)
    $lblEngine.Size = New-Object System.Drawing.Size(70, 20)
    $dlg.Controls.Add($lblEngine)
    $rbWinws = New-Object System.Windows.Forms.RadioButton
    $rbWinws.Text = 'winws'
    $rbWinws.Location = New-Object System.Drawing.Point(88, 12)
    $rbWinws.Size = New-Object System.Drawing.Size(80, 24)
    $dlg.Controls.Add($rbWinws)
    $rbWinws2 = New-Object System.Windows.Forms.RadioButton
    $rbWinws2.Text = 'winws2'
    $rbWinws2.Location = New-Object System.Drawing.Point(174, 12)
    $rbWinws2.Size = New-Object System.Drawing.Size(90, 24)
    $dlg.Controls.Add($rbWinws2)
    if ((Get-ZapretEngine) -eq 'winws2') {
        $rbWinws2.Checked = $true
    } else {
        $rbWinws.Checked = $true
    }
    $lb = New-Object System.Windows.Forms.ListBox
    $lb.Location = New-Object System.Drawing.Point(12, 42)
    $lb.Size = New-Object System.Drawing.Size(396, 180)
    $lb.DrawMode = [System.Windows.Forms.DrawMode]::OwnerDrawFixed
    $lb.ItemHeight = 18
    $lb.Add_DrawItem({
        param($box, $e)
        if ($e.Index -lt 0) {
            return
        }
        $e.DrawBackground()
        $name = [string]$box.Items[$e.Index]
        $file = $null
        if ($script:strategyMap.ContainsKey($name)) {
            $file = $script:strategyMap[$name]
        }
        $ok = $false
        if ($file) {
            $ok = Test-ZapretStrategySupportsEngine -Path $file.FullName -Engine (Get-ZapretEngine)
        }
        $selected = (([int]$e.State) -band ([int][System.Windows.Forms.DrawItemState]::Selected)) -ne 0
        $color = [System.Drawing.SystemColors]::GrayText
        if ($ok) {
            if ($selected) {
                $color = [System.Drawing.SystemColors]::HighlightText
            } else {
                $color = [System.Drawing.SystemColors]::WindowText
            }
        }
        $flags = [System.Windows.Forms.TextFormatFlags]::Left -bor [System.Windows.Forms.TextFormatFlags]::VerticalCenter -bor [System.Windows.Forms.TextFormatFlags]::NoPrefix -bor [System.Windows.Forms.TextFormatFlags]::EndEllipsis
        [void][System.Windows.Forms.TextRenderer]::DrawText($e.Graphics, $name, $e.Font, $e.Bounds, $color, $flags)
        $e.DrawFocusRectangle()
    })
    foreach ($name in @($script:strategyMap.Keys | Sort-Object)) {
        [void]$lb.Items.Add($name)
    }
    Select-InstalledOrFirstStrategy -ListBox $lb
    $dlg.Controls.Add($lb)
    $btnRun = New-Object System.Windows.Forms.Button
    $btnRun.Text = Get-ZapretUiString -Key 'BtnRunSelected'
    $btnRun.Location = New-Object System.Drawing.Point(12, 260)
    $btnRun.Size = New-Object System.Drawing.Size(196, 28)
    $dlg.Controls.Add($btnRun)
    $btnInst = New-Object System.Windows.Forms.Button
    $btnInst.Text = Get-ZapretUiString -Key 'BtnInstall'
    $btnInst.Location = New-Object System.Drawing.Point(212, 260)
    $btnInst.Size = New-Object System.Drawing.Size(196, 28)
    $dlg.Controls.Add($btnInst)
    $btnT = New-Object System.Windows.Forms.Button
    $btnT.Text = Get-ZapretUiString -Key 'BtnTests'
    $btnT.Location = New-Object System.Drawing.Point(12, 296)
    $btnT.Size = New-Object System.Drawing.Size(396, 28)
    $dlg.Controls.Add($btnT)
    $syncEngineUi = {
        $eng = 'winws'
        if ($rbWinws2.Checked) {
            $eng = 'winws2'
        }
        Set-ZapretEngine -Engine $eng | Out-Null
        $file = Get-SelectedStrategy -ListBox $lb
        $can = $false
        if ($file -and (Test-ZapretStrategySupportsEngine -Path $file.FullName -Engine $eng)) {
            if (Test-ZapretEngineFiles -Engine $eng) {
                $can = $true
            }
        }
        $btnRun.Enabled = $can
        $btnInst.Enabled = $can
        $lb.Invalidate()
    }
    $rbWinws.Add_CheckedChanged({ & $syncEngineUi })
    $rbWinws2.Add_CheckedChanged({ & $syncEngineUi })
    $lb.Add_SelectedIndexChanged({ & $syncEngineUi })
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
            if ($ans -ne [System.Windows.Forms.DialogResult]::Yes) {
                return
            }
        }
        Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnRunSelected') -FreezeUi -Action {
            Start-ZapretSelectedStrategy -File $file -OnWait { Invoke-GuiPump }
            Write-GuiLog (Get-ZapretUiString -Key 'RunDone' -FormatArgs @($file.BaseName))
        }
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
    $dlg.Owner = $form
    $script:strategyDialogOpen = $true
    $script:strategyPendingTests = $false
    try {
        [void]$dlg.ShowDialog()
    } finally {
        $script:strategyDialogOpen = $false
        $dlg.Dispose()
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

$hostErrors = @(Test-ZapretHostReady)
if (@($hostErrors).Count -gt 0) {
    $hostMsg = (@($hostErrors) -join [Environment]::NewLine)
    if (Test-ZapretWinForms) {
        Add-Type -AssemblyName System.Windows.Forms
        Show-ErrorDialog $hostMsg
    } else {
        Write-Host $hostMsg
    }
    exit 1
}

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

if (-not (Test-IsAdministrator)) {
    if (-not (Request-Administrator)) {
        Show-ErrorDialog (Get-ZapretUiString -Key 'AdminRequired')
        exit 1
    }
    exit 0
}

Hide-ConsoleWindow

Initialize-ZapretUserLists

$version = Get-ZapretLocalVersion
$strategies = @(Get-ZapretStrategyFiles)
$script:strategyMap = @{}
foreach ($file in $strategies) {
    $script:strategyMap[$file.BaseName] = $file
}

$form = New-Object System.Windows.Forms.Form
$form.Text = (Get-ZapretUiString -Key 'AppTitle' -FormatArgs @($version))
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedSingle'
$form.MaximizeBox = $false
$form.MinimizeBox = $true
$form.ClientSize = New-Object System.Drawing.Size(400, 548)
$form.Font = New-Object System.Drawing.Font('Segoe UI', 9)
$script:trayExit = $false
$script:ownIcon = $false
$appIcon = $null
$winwsExe = Join-Path $script:binDir 'winws.exe'
if (Test-Path -LiteralPath $winwsExe) {
    try {
        $appIcon = [System.Drawing.Icon]::ExtractAssociatedIcon($winwsExe)
        $script:ownIcon = $true
    } catch {
        $appIcon = $null
    }
}
if (-not $appIcon) {
    $appIcon = [System.Drawing.SystemIcons]::Application
    $script:ownIcon = $false
}
$form.Icon = $appIcon

function Show-ZapretMainWindow {
    $form.ShowInTaskbar = $true
    $form.WindowState = 'Normal'
    if (-not $form.Visible) {
        $form.Show()
    }
    $form.Activate()
}

function Update-TrayIcon {
    if (-not $script:tray) {
        return
    }
    $script:tray.Visible = $true
    $bypassName = Get-ZapretStatusBypassName
    if (Test-ZapretBypassRunning) {
        $script:tray.Text = Get-ZapretUiString -Key 'TrayRunning' -FormatArgs @($bypassName)
    } else {
        $script:tray.Text = Get-ZapretUiString -Key 'TrayStopped' -FormatArgs @($bypassName)
    }
}

$title = New-Object System.Windows.Forms.Label
$title.Font = New-Object System.Drawing.Font('Segoe UI', 14, [System.Drawing.FontStyle]::Bold)
$title.Location = New-Object System.Drawing.Point(16, 10)
$title.AutoSize = $true
$form.Controls.Add($title)

$rbRu = New-Object System.Windows.Forms.RadioButton
$rbRu.Location = New-Object System.Drawing.Point(292, 14)
$rbRu.Size = New-Object System.Drawing.Size(48, 20)
$rbRu.Text = 'RU'
$form.Controls.Add($rbRu)

$rbEn = New-Object System.Windows.Forms.RadioButton
$rbEn.Location = New-Object System.Drawing.Point(340, 14)
$rbEn.Size = New-Object System.Drawing.Size(48, 20)
$rbEn.Text = 'EN'
$form.Controls.Add($rbEn)

$lblBanner = New-Object System.Windows.Forms.Label
$lblBanner.Location = New-Object System.Drawing.Point(16, 42)
$lblBanner.Size = New-Object System.Drawing.Size(368, 40)
$lblBanner.Visible = $false
$form.Controls.Add($lblBanner)

$grpStatus = New-Object System.Windows.Forms.GroupBox
$grpStatus.Location = New-Object System.Drawing.Point(16, 88)
$grpStatus.Size = New-Object System.Drawing.Size(368, 102)
$grpStatus.Cursor = [System.Windows.Forms.Cursors]::Hand
$form.Controls.Add($grpStatus)

$lblBypass = New-Object System.Windows.Forms.Label
$lblBypass.Location = New-Object System.Drawing.Point(12, 20)
$lblBypass.Size = New-Object System.Drawing.Size(344, 18)
$lblBypass.ForeColor = [System.Drawing.Color]::DimGray
$grpStatus.Controls.Add($lblBypass)

$lblService = New-Object System.Windows.Forms.Label
$lblService.Location = New-Object System.Drawing.Point(12, 40)
$lblService.Size = New-Object System.Drawing.Size(344, 18)
$lblService.ForeColor = [System.Drawing.Color]::DimGray
$grpStatus.Controls.Add($lblService)

$lblInstalled = New-Object System.Windows.Forms.Label
$lblInstalled.Location = New-Object System.Drawing.Point(12, 60)
$lblInstalled.Size = New-Object System.Drawing.Size(344, 16)
$lblInstalled.ForeColor = [System.Drawing.Color]::DimGray
$grpStatus.Controls.Add($lblInstalled)

$lblDivert = New-Object System.Windows.Forms.Label
$lblDivert.Location = New-Object System.Drawing.Point(12, 78)
$lblDivert.Size = New-Object System.Drawing.Size(344, 16)
$lblDivert.ForeColor = [System.Drawing.Color]::DimGray
$grpStatus.Controls.Add($lblDivert)

$btnStart = New-Object System.Windows.Forms.Button
$btnStart.Location = New-Object System.Drawing.Point(16, 198)
$btnStart.Size = New-Object System.Drawing.Size(178, 28)
$form.Controls.Add($btnStart)

$btnStop = New-Object System.Windows.Forms.Button
$btnStop.Location = New-Object System.Drawing.Point(206, 198)
$btnStop.Size = New-Object System.Drawing.Size(178, 28)
$form.Controls.Add($btnStop)

$btnRemove = New-Object System.Windows.Forms.Button
$btnRemove.Location = New-Object System.Drawing.Point(16, 230)
$btnRemove.Size = New-Object System.Drawing.Size(178, 28)
$form.Controls.Add($btnRemove)

$btnStrategy = New-Object System.Windows.Forms.Button
$btnStrategy.Location = New-Object System.Drawing.Point(206, 230)
$btnStrategy.Size = New-Object System.Drawing.Size(178, 28)
$form.Controls.Add($btnStrategy)

$grpSettings = New-Object System.Windows.Forms.GroupBox
$grpSettings.Location = New-Object System.Drawing.Point(16, 266)
$grpSettings.Size = New-Object System.Drawing.Size(368, 122)
$form.Controls.Add($grpSettings)

$lblGame = New-Object System.Windows.Forms.Label
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
$lblIpset.Location = New-Object System.Drawing.Point(216, 24)
$lblIpset.AutoSize = $true
$grpSettings.Controls.Add($lblIpset)

$cmbIpset = New-Object System.Windows.Forms.ComboBox
$cmbIpset.DropDownStyle = 'DropDownList'
$cmbIpset.Location = New-Object System.Drawing.Point(276, 20)
$cmbIpset.Size = New-Object System.Drawing.Size(80, 24)
[void]$cmbIpset.Items.AddRange(@('none', 'loaded', 'any'))
$grpSettings.Controls.Add($cmbIpset)

$chkAuto = New-Object System.Windows.Forms.CheckBox
$chkAuto.Location = New-Object System.Drawing.Point(12, 50)
$chkAuto.AutoSize = $true
$grpSettings.Controls.Add($chkAuto)

$btnFakes = New-Object System.Windows.Forms.Button
$btnFakes.Location = New-Object System.Drawing.Point(12, 82)
$btnFakes.Size = New-Object System.Drawing.Size(344, 28)
$grpSettings.Controls.Add($btnFakes)

$grpTools = New-Object System.Windows.Forms.GroupBox
$grpTools.Location = New-Object System.Drawing.Point(16, 396)
$grpTools.Size = New-Object System.Drawing.Size(368, 92)
$form.Controls.Add($grpTools)

$btnIpsetUpd = New-Object System.Windows.Forms.Button
$btnIpsetUpd.Location = New-Object System.Drawing.Point(12, 20)
$btnIpsetUpd.Size = New-Object System.Drawing.Size(168, 28)
$grpTools.Controls.Add($btnIpsetUpd)

$btnHosts = New-Object System.Windows.Forms.Button
$btnHosts.Location = New-Object System.Drawing.Point(188, 20)
$btnHosts.Size = New-Object System.Drawing.Size(168, 28)
$grpTools.Controls.Add($btnHosts)

$btnUpdates = New-Object System.Windows.Forms.Button
$btnUpdates.Location = New-Object System.Drawing.Point(12, 54)
$btnUpdates.Size = New-Object System.Drawing.Size(168, 28)
$grpTools.Controls.Add($btnUpdates)

$btnDiag = New-Object System.Windows.Forms.Button
$btnDiag.Location = New-Object System.Drawing.Point(188, 54)
$btnDiag.Size = New-Object System.Drawing.Size(168, 28)
$grpTools.Controls.Add($btnDiag)

$tips = New-Object System.Windows.Forms.ToolTip

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
    $form.Text = Get-ZapretUiString -Key 'AppTitle' -FormatArgs @($ver)
    $title.Text = Get-ZapretUiString -Key 'AppName'
    $grpStatus.Text = Get-ZapretUiString -Key 'GrpStatus'
    $grpSettings.Text = Get-ZapretUiString -Key 'GrpSettings'
    $grpTools.Text = Get-ZapretUiString -Key 'GrpTools'
    $btnStart.Text = Get-ZapretUiString -Key 'BtnStart'
    $btnStop.Text = Get-ZapretUiString -Key 'BtnStop'
    $btnRemove.Text = Get-ZapretUiString -Key 'BtnRemove'
    $btnStrategy.Text = Get-ZapretUiString -Key 'BtnStrategy'
    $btnFakes.Text = Get-ZapretUiString -Key 'BtnFakes'
    $btnIpsetUpd.Text = Get-ZapretUiString -Key 'BtnIpset'
    $btnHosts.Text = Get-ZapretUiString -Key 'BtnHosts'
    $btnUpdates.Text = Get-ZapretUiString -Key 'BtnVersion'
    $btnDiag.Text = Get-ZapretUiString -Key 'BtnDiag'
    $chkAuto.Text = Get-ZapretUiString -Key 'ChkAuto'
    $lblGame.Text = Get-ZapretUiString -Key 'LblGame'
    $lblIpset.Text = Get-ZapretUiString -Key 'LblIpset'
    $rbRu.Text = Get-ZapretUiString -Key 'LangRu'
    $rbEn.Text = Get-ZapretUiString -Key 'LangEn'
    $tips.SetToolTip($btnStart, (Get-ZapretUiString -Key 'TipStart'))
    $tips.SetToolTip($btnStop, (Get-ZapretUiString -Key 'TipStop'))
    $tips.SetToolTip($btnRemove, (Get-ZapretUiString -Key 'TipRemove'))
    $tips.SetToolTip($btnStrategy, (Get-ZapretUiString -Key 'TipStrategy'))
    $tips.SetToolTip($btnFakes, (Get-ZapretUiString -Key 'TipFakes'))
    $tips.SetToolTip($btnIpsetUpd, (Get-ZapretUiString -Key 'TipIpset'))
    $tips.SetToolTip($btnHosts, (Get-ZapretUiString -Key 'TipHosts'))
    $tips.SetToolTip($btnUpdates, (Get-ZapretUiString -Key 'TipVersion'))
    $tips.SetToolTip($btnDiag, (Get-ZapretUiString -Key 'TipDiag'))
    $tips.SetToolTip($chkAuto, (Get-ZapretUiString -Key 'TipAuto'))
    $tips.SetToolTip($grpStatus, (Get-ZapretUiString -Key 'TipStatusClick'))
    if ($script:trayMenu) {
        $script:trayMenu.Items[0].Text = Get-ZapretUiString -Key 'TrayOpen'
        $script:trayMenu.Items[1].Text = Get-ZapretUiString -Key 'TrayExit'
    }
    Update-TrayIcon
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
    $btnStart.Enabled = $Enabled -and $hasService
    $btnStop.Enabled = $Enabled
    $btnRemove.Enabled = $Enabled
    $btnStrategy.Enabled = $Enabled -and $hasStrategies
    $btnFakes.Enabled = $Enabled
    $btnIpsetUpd.Enabled = $Enabled
    $btnHosts.Enabled = $Enabled
    $btnUpdates.Enabled = $Enabled
    $btnDiag.Enabled = $Enabled
    $chkAuto.Enabled = $Enabled
    $cmbGame.Enabled = $Enabled
    $cmbIpset.Enabled = $Enabled
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
    $bannerColor = [System.Drawing.Color]::FromArgb(230, 240, 250)
    $bannerFg = [System.Drawing.Color]::FromArgb(24, 95, 165)
    if ($pending) {
        $bannerKey = 'BannerStopPending'
        $bannerColor = [System.Drawing.Color]::FromArgb(253, 232, 230)
        $bannerFg = [System.Drawing.Color]::FromArgb(180, 35, 24)
    } elseif (-not $hasStrategies) {
        $bannerKey = 'BannerNoStrategies'
        $bannerColor = [System.Drawing.Color]::FromArgb(255, 244, 214)
        $bannerFg = [System.Drawing.Color]::FromArgb(154, 103, 0)
    } elseif ($svc -and -not $running) {
        $bannerKey = 'BannerMismatch'
        $bannerColor = [System.Drawing.Color]::FromArgb(255, 244, 214)
        $bannerFg = [System.Drawing.Color]::FromArgb(154, 103, 0)
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
        $lblBanner.BackColor = $bannerColor
        $lblBanner.ForeColor = $bannerFg
        $lblBanner.Visible = $true
    } else {
        $lblBanner.Text = ''
        $lblBanner.Visible = $false
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
        if ($chkAuto.Checked -ne $autoOn) {
            $chkAuto.Checked = $autoOn
        }
    } finally {
        $script:updatingSettings = $false
    }

    Update-TrayIcon
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
    if ($pick -and $ListBox.Items.Contains($pick)) {
        $ListBox.SelectedItem = $pick
        return
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
        $form.UseWaitCursor = $true
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
            $form.UseWaitCursor = $false
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

$openStatus = {
    Show-StatusJournalDialog
}
$grpStatus.Add_Click($openStatus)
$lblBypass.Add_Click($openStatus)
$lblService.Add_Click($openStatus)
$lblInstalled.Add_Click($openStatus)
$lblDivert.Add_Click($openStatus)

$cmbGame.Add_SelectedIndexChanged({
    if ($script:updatingSettings) { return }
    try {
        Set-ZapretGameFilterMode (ConvertFrom-GameComboIndex $cmbGame.SelectedIndex)
        Write-GuiLog (Get-GameFilterApplyHint)
        if (Get-ZapretService) {
            $ans = Show-QuestionDialog -Message (Get-ZapretUiString -Key 'FilterNeedInstall')
            if ($ans -eq [System.Windows.Forms.DialogResult]::Yes) {
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

$cmbIpset.Add_SelectedIndexChanged({
    if ($script:updatingSettings) { return }
    try {
        Set-ZapretIpsetMode -Mode ([string]$cmbIpset.SelectedItem)
        Write-GuiLog (Get-IpsetApplyHint)
        if (Get-ZapretService) {
            $ans = Show-QuestionDialog -Message (Get-ZapretUiString -Key 'FilterNeedRestart')
            if ($ans -eq [System.Windows.Forms.DialogResult]::Yes) {
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

$chkAuto.Add_CheckedChanged({
    if ($script:updatingSettings) { return }
    try {
        Set-ZapretAutoUpdateEnabled -Enabled $chkAuto.Checked
        if ($chkAuto.Checked) {
            Write-GuiLog (Get-ZapretUiString -Key 'AutoOn')
        } else {
            Write-GuiLog (Get-ZapretUiString -Key 'AutoOff')
        }
    } catch {
        Show-ErrorDialog $_.Exception.Message
        Update-Status
    }
})

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
    Invoke-GuiAction -BusyText (Get-ZapretUiString -Key 'BtnFakes') -Action {
        Set-ZapretActiveFake -Slot $choice.Slot -SourcePath $choice.Path
        Write-GuiLog (Get-ZapretUiString -Key 'FakeDone' -FormatArgs @($choice.Slot, $choice.Name))
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

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 2000
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

$form.Add_Shown({
    try {
        Set-ActionButtonsEnabled -Enabled $true
        Update-Status
        $timer.Start()
        if (Test-ZapretAutoUpdateEnabled) {
            Start-GuiVersionCheck
        }
    } catch {
        Show-ErrorDialog $_.Exception.Message
    }
})

$script:trayMenu = New-Object System.Windows.Forms.ContextMenuStrip
[void]$script:trayMenu.Items.Add((Get-ZapretUiString -Key 'TrayOpen')).Add_Click({ Show-ZapretMainWindow })
[void]$script:trayMenu.Items.Add((Get-ZapretUiString -Key 'TrayExit')).Add_Click({
    $script:trayExit = $true
    $form.Close()
})

$script:tray = New-Object System.Windows.Forms.NotifyIcon
$script:tray.Icon = $appIcon
$script:tray.Text = Get-ZapretUiString -Key 'TrayStopped' -FormatArgs @(Get-ZapretStatusBypassName)
$script:tray.Visible = $true
$script:tray.ContextMenuStrip = $script:trayMenu
$script:tray.Add_DoubleClick({ Show-ZapretMainWindow })

$form.Add_FormClosed({
    $script:guiClosing = $true
    Stop-GuiVersionCheck
    $timer.Stop()
    $timer.Dispose()
    if ($script:tray) {
        $script:tray.Visible = $false
        $script:tray.Dispose()
    }
    $script:trayMenu.Dispose()
    if ($script:ownIcon -and $appIcon) {
        $appIcon.Dispose()
    }
})

$rbRu.Add_CheckedChanged({
    if ($script:updatingLang) { return }
    if ($rbRu.Checked) {
        Set-ZapretUiLanguage -Language 'ru'
        Update-GuiLanguage
        Update-Status
    }
})
$rbEn.Add_CheckedChanged({
    if ($script:updatingLang) { return }
    if ($rbEn.Checked) {
        Set-ZapretUiLanguage -Language 'en'
        Update-GuiLanguage
        Update-Status
    }
})

$script:updatingLang = $true
if ((Get-ZapretUiLanguage) -eq 'ru') {
    $rbRu.Checked = $true
} else {
    $rbEn.Checked = $true
}
$script:updatingLang = $false
Update-GuiLanguage

try {
    # Application.Run is the main loop. ShowDialog here ends when a child dialog disables this form.
    [System.Windows.Forms.Application]::Run($form)
} catch {
    Show-ErrorDialog $_.Exception.Message
    exit 1
}
