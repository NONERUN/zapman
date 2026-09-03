# Owned WPF dialogs. Dotsource from gui.ps1 in the same $script: scope.
# Diagnostics, fakes, download, tests, status, and strategy.

function Show-GuiDiagnostics {
    $dlg = Import-ZapmanXaml 'DiagPick.xaml'
    $dlg.Title = Get-ZapmanUiString -Key 'DiagTitle'
    $diagPanel = Get-XamlChild -Root $dlg -Name 'diagPanel'
    $btnOk = Get-XamlChild -Root $dlg -Name 'btnOk'
    $btnOk.Content = Get-ZapmanUiString -Key 'BtnOk'
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
        $report = Get-ZapmanDiagnosticReport
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
                $btn.Content = Get-ZapmanUiString -Key 'DiagRun'
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
                        $msgs = @(Invoke-ZapmanDiagnosticAction -Action ([string]$it.Action) -Names @($it.ActionNames))
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

    [void](Show-ZapmanOwnedDialog -Dialog $dlg)
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
        throw (Get-ZapmanUiString -Key 'FakesNone')
    }

    $dlg = Import-ZapmanXaml 'Fake.xaml'
    $dlg.Title = Get-ZapmanUiString -Key 'FakesTitle'
    $lblDiscord = Get-XamlChild -Root $dlg -Name 'lblDiscord'
    $lblDiscord.Text = Get-ZapmanUiString -Key 'FakesDiscord'
    $cmbDiscord = Get-XamlChild -Root $dlg -Name 'cmbDiscord'
    $discordOffset = Add-FakeSlotComboItems -Combo $cmbDiscord -Files $files -CurrentName ([string]$catalog.CurrentDiscord) -State ([string]$catalog.DiscordState)
    $lblGame = Get-XamlChild -Root $dlg -Name 'lblGame'
    $lblGame.Text = Get-ZapmanUiString -Key 'FakesGame'
    $cmbGameFake = Get-XamlChild -Root $dlg -Name 'cmbGameFake'
    $gameOffset = Add-FakeSlotComboItems -Combo $cmbGameFake -Files $files -CurrentName ([string]$catalog.CurrentGame) -State ([string]$catalog.GameState)
    $btnOk = Get-XamlChild -Root $dlg -Name 'btnOk'
    $btnOk.Content = Get-ZapmanUiString -Key 'BtnOk'
    $btnOk.Add_Click({
        $dlg.DialogResult = $true
    })
    $btnCancel = Get-XamlChild -Root $dlg -Name 'btnCancel'
    $btnCancel.Content = Get-ZapmanUiString -Key 'BtnCancel'

    $result = Show-ZapmanOwnedDialog -Dialog $dlg
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

function Invoke-GuiDownload {
    param(
        [string]$Url,
        [string]$Destination,
        [string]$Title
    )

    Enable-ZapmanTls12
    $partial = $Destination + '.partial'
    $script:dlDone = $false
    $script:dlError = $null

    $dlg = Import-ZapmanXaml 'Download.xaml'
    $dlg.Title = $Title
    $lbl = Get-XamlChild -Root $dlg -Name 'lblStatus'
    $lbl.Text = Get-ZapmanUiString -Key 'DownloadConnecting'
    $bar = Get-XamlChild -Root $dlg -Name 'barProgress'
    $btnCancel = Get-XamlChild -Root $dlg -Name 'btnCancel'
    $btnCancel.Content = Get-ZapmanUiString -Key 'BtnCancel'
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
                    $script:dlLabel.Text = (Get-ZapmanUiString -Key 'DownloadProgress' -FormatArgs @([int]($script:dlBytes / 1024), [int]($total / 1024), $pct))
                } else {
                    $script:dlLabel.Text = (Get-ZapmanUiString -Key 'DownloadProgressKb' -FormatArgs @([int]($script:dlBytes / 1024)))
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
                $script:dlError = Get-ZapmanUiString -Key 'DownloadCancelled'
            } elseif ($e -and $e.Error) {
                $script:dlError = $e.Error.Message
            }
        } catch {
            $script:dlError = Get-ZapmanUiString -Key 'DownloadFailed'
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
        throw (Get-ZapmanUiString -Key 'DownloadFailed')
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
            throw (Get-ZapmanUiString -Key 'EngineNoWinws2')
        }
        throw (Get-ZapmanUiString -Key 'EngineNoWinws')
    }
    $files = @(
        @(Get-ZapretStrategyFiles) | Where-Object {
            Test-ZapretStrategySupportsEngine -Path $_.FullName -Engine $engine
        }
    )
    if ($files.Count -eq 0) {
        throw (Get-ZapmanUiString -Key 'EngineNoFlags' -FormatArgs @($engine))
    }

    $dlg = Import-ZapmanXaml 'TestsSetup.xaml'
    $dlg.Title = Get-ZapmanUiString -Key 'TestsSetupTitle'
    $lblType = Get-XamlChild -Root $dlg -Name 'lblType'
    $lblType.Text = Get-ZapmanUiString -Key 'TestsType'
    $rbStd = Get-XamlChild -Root $dlg -Name 'rbStd'
    $rbStd.Content = Get-ZapmanUiString -Key 'TestsTypeStd'
    $rbDpi = Get-XamlChild -Root $dlg -Name 'rbDpi'
    $rbDpi.Content = Get-ZapmanUiString -Key 'TestsTypeDpi'
    $lblPick = Get-XamlChild -Root $dlg -Name 'lblPick'
    $lblPick.Text = (Get-ZapmanUiString -Key 'TestsPick' -FormatArgs @($engine))
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
    $btnAll.Content = Get-ZapmanUiString -Key 'TestsAll'
    $btnAll.Add_Click({
        foreach ($cb in @($checks.Items)) {
            $cb.IsChecked = $true
        }
    })
    $btnNone = Get-XamlChild -Root $dlg -Name 'btnNone'
    $btnNone.Content = Get-ZapmanUiString -Key 'TestsNone'
    $btnNone.Add_Click({
        foreach ($cb in @($checks.Items)) {
            $cb.IsChecked = $false
        }
    })
    $btnRun = Get-XamlChild -Root $dlg -Name 'btnRun'
    $btnRun.Content = Get-ZapmanUiString -Key 'TestsStart'
    $btnRun.Add_Click({
        $dlg.DialogResult = $true
    })
    $btnCancel = Get-XamlChild -Root $dlg -Name 'btnCancel'
    $btnCancel.Content = Get-ZapmanUiString -Key 'BtnCancel'

    $result = Show-ZapmanOwnedDialog -Dialog $dlg
    $choice = $null
    if ($result -eq $true) {
        $names = New-Object System.Collections.Generic.List[string]
        foreach ($cb in @($checks.Items)) {
            if ($cb.IsChecked -eq $true) {
                [void]$names.Add([string]$cb.Content)
            }
        }
        if ($names.Count -eq 0) {
            throw (Get-ZapmanUiString -Key 'TestsNeedOne')
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
        $script:testLbl.Text = (Get-ZapmanUiString -Key 'TestsProgress' -FormatArgs @($matches[1], $matches[2]))
    }
}

function Start-GuiTestRun {
    param(
        [string]$TestType,
        [string[]]$Names
    )

    $nameList = @($Names)
    $dlg = Import-ZapmanXaml 'Tests.xaml'
    $dlg.Title = Get-ZapmanUiString -Key 'TestsTitle'
    $lbl = Get-XamlChild -Root $dlg -Name 'lblStatus'
    $lbl.Text = Get-ZapmanUiString -Key 'TestsStarting'
    $bar = Get-XamlChild -Root $dlg -Name 'barProgress'
    $box = Get-XamlChild -Root $dlg -Name 'txtLog'
    $btnStop = Get-XamlChild -Root $dlg -Name 'btnStop'
    $btnStop.Content = Get-ZapmanUiString -Key 'BtnCancel'

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
                $script:testLbl.Text = Get-ZapmanUiString -Key 'TestsCancelling'
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
            Invoke-ZapmanStrategyTests -TestType $script:testType -Names @($script:testNames) -OnLine {
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
                    $script:testLbl.Text = Get-ZapmanUiString -Key 'TestsCancelledShort'
                } else {
                    $script:testLbl.Text = Get-ZapmanUiString -Key 'TestsFinishedLog'
                }
            }
            if ($script:testBtn) {
                $script:testBtn.Content = Get-ZapmanUiString -Key 'BtnClose'
            }
        }
    })

    try {
        [void](Show-ZapmanOwnedDialog -Dialog $dlg)
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
    $status = @((@(Get-ZapmanStatusLines)) -join [Environment]::NewLine)
    $journal = @((@($script:guiJournal)) -join [Environment]::NewLine)
    $text = $status + [Environment]::NewLine + [Environment]::NewLine + (Get-ZapmanUiString -Key 'JournalHeader') + [Environment]::NewLine + $journal
    $dlg = Import-ZapmanXaml 'Status.xaml'
    $dlg.Title = Get-ZapmanUiString -Key 'StatusDlgTitle'
    $box = Get-XamlChild -Root $dlg -Name 'txtStatus'
    $box.Text = $text
    $box.CaretIndex = 0
    $copy = Get-XamlChild -Root $dlg -Name 'btnCopy'
    $copy.Content = Get-ZapmanUiString -Key 'BtnCopy'
    $copy.Add_Click({
        try {
            [System.Windows.Clipboard]::SetText($box.Text)
        } catch {
            Show-ErrorDialog $_.Exception.Message
        }
    })
    $ok = Get-XamlChild -Root $dlg -Name 'btnClose'
    $ok.Content = Get-ZapmanUiString -Key 'BtnClose'
    $ok.Add_Click({
        $dlg.DialogResult = $true
    })
    [void](Show-ZapmanOwnedDialog -Dialog $dlg)
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
            $label = $name + (Get-ZapmanUiString -Key 'StratMarkRun')
        } elseif ($name -eq $installedName) {
            $label = $name + (Get-ZapmanUiString -Key 'StratMarkService')
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
    $dlg = Import-ZapmanXaml 'Strategy.xaml'
    $dlg.Title = Get-ZapmanUiString -Key 'StratTitle'
    $lblEngine = Get-XamlChild -Root $dlg -Name 'lblEngine'
    $lblEngine.Text = Get-ZapmanUiString -Key 'LblEngine'
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
    $btnRun.Content = Get-ZapmanUiString -Key 'BtnRunSelected'
    $btnInst = Get-XamlChild -Root $dlg -Name 'btnInst'
    $btnInst.Content = Get-ZapmanUiString -Key 'BtnInstall'
    $btnT = Get-XamlChild -Root $dlg -Name 'btnTests'
    $btnT.Content = Get-ZapmanUiString -Key 'BtnTests'
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
            Show-ErrorDialog (Get-ZapmanUiString -Key 'NoStrategies')
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
            $ans = Show-QuestionDialog -Message (Get-ZapmanUiString -Key 'ConfirmRunOver') -DefaultYes $false
            if ($ans -ne [System.Windows.MessageBoxResult]::Yes) {
                return
            }
        }
        Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnRunSelected') -FreezeUi -Action {
            Start-ZapretSelectedStrategy -File $file -OnWait { Invoke-GuiPump }
            Write-GuiLog (Get-ZapmanUiString -Key 'RunDone' -FormatArgs @($file.BaseName))
        }
        Update-StrategyListMarks -ListBox $lb
    })
    $btnInst.Add_Click({
        $file = Get-SelectedStrategy -ListBox $lb
        if (-not $file) {
            Show-ErrorDialog (Get-ZapmanUiString -Key 'NoStrategies')
            return
        }
        $script:installOk = $false
        Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnInstall') -FreezeUi -Action {
            Install-ZapretService -File $file -OnWait { Invoke-GuiPump }
            Write-GuiLog (Get-ZapmanUiString -Key 'InstallDone' -FormatArgs @($file.BaseName))
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
        [void](Show-ZapmanOwnedDialog -Dialog $dlg)
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
    try {
        $setup = Show-GuiTestsSetup
    } catch {
        Show-ErrorDialog $_.Exception.Message
        return
    }
    if (-not $setup) {
        return
    }
    $script:testServiceSnap = $null
    try {
        if (Get-ZapretService) {
            Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'TestsNeedNoService') -FreezeUi -Action {
                $script:testServiceSnap = Suspend-ZapretServiceForTests -OnWait { Invoke-GuiPump }
            }
        }
        Start-GuiTestRun -TestType $setup.TestType -Names @($setup.Names)
        Write-GuiLog (Get-ZapmanUiString -Key 'TestsFinished')
    } finally {
        $snap = $script:testServiceSnap
        $script:testServiceSnap = $null
        if ($snap -and $snap.File) {
            $script:testServiceSnap = $snap
            Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnInstall') -FreezeUi -Action {
                $toRestore = $script:testServiceSnap
                $script:testServiceSnap = $null
                Restore-ZapretServiceAfterTests -Snapshot $toRestore -OnWait { Invoke-GuiPump }
                Write-GuiLog (Get-ZapmanUiString -Key 'InstallDone' -FormatArgs @($toRestore.File.BaseName))
            }
        }
    }
}

function Get-GameFilterApplyHint {
    if (Get-ZapretService) {
        return Get-ZapmanUiString -Key 'GameHintInstall'
    }
    return Get-ZapmanUiString -Key 'GameHintRerun'
}

function Get-IpsetApplyHint {
    if (Get-ZapretService) {
        return Get-ZapmanUiString -Key 'IpsetHintRestart'
    }
    return Get-ZapmanUiString -Key 'IpsetHintRerun'
}
