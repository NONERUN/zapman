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
    $wc.Headers.Add('User-Agent', (Get-ZapmanWebUserAgent))

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
        Invoke-GuiOnUi {
            if ($script:dlDlg) {
                $script:dlDlg.Close()
            }
        }
    })

    $btnCancel.Add_Click({
        $wc.CancelAsync()
    })
    $dlg.Add_Closing({
        if (-not $script:dlDone) {
            $wc.CancelAsync()
            $_.Cancel = $true
        }
    })

    $script:dlDlg = $dlg
    try {
        $wc.DownloadFileAsync([Uri]$Url, $partial)
        [void]$dlg.ShowDialog()
    } finally {
        $script:dlDlg = $null
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

# Read a note property. Return $null if the property is missing.
function Get-GuiTestInfoProp {
    param($Info, [string]$Name)
    if ($null -eq $Info) {
        return $null
    }
    $prop = $Info.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $null
    }
    return $prop.Value
}

# Map a result token to a color kind: err, warn, ok, or info.
function Get-GuiTestCellKind {
    param([string]$Text)
    $t = ([string]$Text).Trim()
    if ([string]::IsNullOrWhiteSpace($t) -or $t -eq '-' -or $t -eq 'n/a') {
        return 'info'
    }
    $u = $t.ToUpperInvariant()
    if ($u -match 'ERROR|FAIL|SSL') {
        return 'err'
    }
    if ($u -match 'UNSUP|TIMEOUT|BLOCK|NA') {
        return 'warn'
    }
    if ($u -match 'OK' -or $u -match '^\d') {
        return 'ok'
    }
    return 'info'
}

# Return a fill or text brush for a result kind.
function Get-GuiTestKindBrush {
    param([string]$Kind, [switch]$Foreground)
    if ($Kind -eq 'err') {
        if ($Foreground) {
            return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(180, 35, 24))
        }
        return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(253, 232, 230))
    }
    if ($Kind -eq 'ok') {
        if ($Foreground) {
            return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(30, 100, 40))
        }
        return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(220, 237, 200))
    }
    if ($Kind -eq 'warn') {
        if ($Foreground) {
            return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(154, 103, 0))
        }
        return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(255, 244, 214))
    }
    if ($Kind -eq 'head') {
        if ($Foreground) {
            return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(24, 95, 165))
        }
        return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(230, 240, 250))
    }
    if ($Foreground) {
        return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(80, 80, 80))
    }
    return New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(245, 245, 245))
}

# Add one table cell to a result grid.
function Add-GuiTestGridCell {
    param(
        $Grid,
        [int]$Row,
        [int]$Col,
        [string]$Text,
        [string]$Kind,
        [switch]$Left
    )
    $border = New-Object System.Windows.Controls.Border
    $border.BorderBrush = New-Object System.Windows.Media.SolidColorBrush([System.Windows.Media.Color]::FromRgb(200, 200, 200))
    $border.BorderThickness = New-Object System.Windows.Thickness(0.5)
    $border.Background = Get-GuiTestKindBrush -Kind $Kind
    $border.Padding = New-Object System.Windows.Thickness(6, 2, 6, 2)
    $tb = New-Object System.Windows.Controls.TextBlock
    $tb.Text = $Text
    $tb.Foreground = Get-GuiTestKindBrush -Kind $Kind -Foreground
    if ($Left) {
        $tb.TextAlignment = [System.Windows.TextAlignment]::Left
        $tb.TextTrimming = [System.Windows.TextTrimming]::CharacterEllipsis
    } else {
        $tb.TextAlignment = [System.Windows.TextAlignment]::Center
        $tb.FontWeight = [System.Windows.FontWeights]::SemiBold
    }
    $tb.TextWrapping = [System.Windows.TextWrapping]::NoWrap
    $border.Child = $tb
    [System.Windows.Controls.Grid]::SetRow($border, $Row)
    [System.Windows.Controls.Grid]::SetColumn($border, $Col)
    [void]$Grid.Children.Add($border)
}

# Build a result table. Color error cells red.
function New-GuiTestResultGrid {
    param(
        [string[]]$Headers,
        $Rows
    )
    $grid = New-Object System.Windows.Controls.Grid
    $colCount = @($Headers).Count
    $i = 0
    while ($i -lt $colCount) {
        $col = New-Object System.Windows.Controls.ColumnDefinition
        if ($i -eq 0) {
            $col.Width = New-Object System.Windows.GridLength(1.4, [System.Windows.GridUnitType]::Star)
        } else {
            $col.Width = New-Object System.Windows.GridLength(1, [System.Windows.GridUnitType]::Star)
        }
        [void]$grid.ColumnDefinitions.Add($col)
        $i++
    }
    $rowDef0 = New-Object System.Windows.Controls.RowDefinition
    $rowDef0.Height = [System.Windows.GridLength]::Auto
    [void]$grid.RowDefinitions.Add($rowDef0)
    $c = 0
    foreach ($h in @($Headers)) {
        Add-GuiTestGridCell -Grid $grid -Row 0 -Col $c -Text $h -Kind 'head'
        $c++
    }
    $r = 1
    if ($null -eq $Rows) {
        return $grid
    }
    foreach ($row in $Rows) {
        $rd = New-Object System.Windows.Controls.RowDefinition
        $rd.Height = [System.Windows.GridLength]::Auto
        [void]$grid.RowDefinitions.Add($rd)
        $c = 0
        $cells = $row
        if ($null -eq $cells) {
            $r++
            continue
        }
        foreach ($cell in $cells) {
            $text = [string](Get-GuiTestInfoProp -Info $cell -Name 'Text')
            $kind = [string](Get-GuiTestInfoProp -Info $cell -Name 'Kind')
            $left = $false
            if ($c -eq 0) {
                $left = $true
                if ([string]::IsNullOrWhiteSpace($kind)) {
                    $kind = 'info'
                }
            } elseif ([string]::IsNullOrWhiteSpace($kind)) {
                $kind = Get-GuiTestCellKind -Text $text
            }
            if ($left) {
                Add-GuiTestGridCell -Grid $grid -Row $r -Col $c -Text $text -Kind $kind -Left
            } else {
                Add-GuiTestGridCell -Grid $grid -Row $r -Col $c -Text $text -Kind $kind
            }
            $c++
        }
        $r++
    }
    return $grid
}

# Convert text to an integer. Return 0 if the text is not a number.
function ConvertTo-GuiTestInt {
    param($Value)
    $s = [string]$Value
    if ([string]::IsNullOrWhiteSpace($s)) {
        return 0
    }
    $n = 0
    if ([int]::TryParse($s, [ref]$n)) {
        return $n
    }
    return 0
}

# Build the table or error banner for one strategy tab.
function New-GuiTestStrategyTable {
    param($Info)
    $kind = [string](Get-GuiTestInfoProp -Info $Info -Name 'Kind')
    $err = [string](Get-GuiTestInfoProp -Info $Info -Name 'Error')
    if ($kind -eq 'fail' -or -not [string]::IsNullOrWhiteSpace($err)) {
        $tb = New-Object System.Windows.Controls.TextBox
        $tb.Text = $err
        $tb.IsReadOnly = $true
        $tb.AcceptsReturn = $true
        $tb.TextWrapping = [System.Windows.TextWrapping]::Wrap
        $tb.BorderThickness = New-Object System.Windows.Thickness(0)
        $tb.Background = Get-GuiTestKindBrush -Kind 'err'
        $tb.Foreground = Get-GuiTestKindBrush -Kind 'err' -Foreground
        $tb.FontFamily = New-Object System.Windows.Media.FontFamily('Consolas')
        $tb.FontSize = 12
        $tb.Padding = New-Object System.Windows.Thickness(8)
        $tb.VerticalScrollBarVisibility = [System.Windows.Controls.ScrollBarVisibility]::Auto
        $tb.HorizontalScrollBarVisibility = [System.Windows.Controls.ScrollBarVisibility]::Auto
        $tb.IsReadOnlyCaretVisible = $true
        return @{ Control = $tb; HasError = $true }
    }
    $rows = New-Object System.Collections.ArrayList
    $hasError = $false
    if ($kind -eq 'dpi') {
        $headers = @(
            (Get-ZapmanUiString -Key 'TestsColTarget')
            'HTTP'
            'TLS1.2'
            'TLS1.3'
        )
        foreach ($target in @(Get-GuiTestInfoProp -Info $Info -Name 'Results')) {
            $name = [string](Get-GuiTestInfoProp -Info $target -Name 'TargetId')
            $cells = New-Object System.Collections.ArrayList
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $name; Kind = 'info' }))
            $byLabel = @{}
            foreach ($line in @(Get-GuiTestInfoProp -Info $target -Name 'Lines')) {
                $lab = [string](Get-GuiTestInfoProp -Info $line -Name 'TestLabel')
                $st = [string](Get-GuiTestInfoProp -Info $line -Name 'Status')
                $byLabel[$lab] = $st
            }
            foreach ($lab in @('HTTP', 'TLS1.2', 'TLS1.3')) {
                $st = '-'
                if ($byLabel.ContainsKey($lab)) {
                    $st = [string]$byLabel[$lab]
                }
                $ck = Get-GuiTestCellKind -Text $st
                if ($ck -eq 'err') {
                    $hasError = $true
                }
                [void]$cells.Add((New-Object PSObject -Property @{ Text = $st; Kind = $ck }))
            }
            [void]$rows.Add($cells.ToArray())
        }
    } else {
        $headers = @(
            (Get-ZapmanUiString -Key 'TestsColTarget')
            'HTTP'
            'TLS1.2'
            'TLS1.3'
            (Get-ZapmanUiString -Key 'TestsColPing')
        )
        foreach ($target in @(Get-GuiTestInfoProp -Info $Info -Name 'Results')) {
            $name = [string](Get-GuiTestInfoProp -Info $target -Name 'Name')
            $cells = New-Object System.Collections.ArrayList
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $name; Kind = 'info' }))
            $byLabel = @{}
            foreach ($tok in @(Get-GuiTestInfoProp -Info $target -Name 'HttpTokens')) {
                $raw = [string]$tok
                $lab = $raw
                $st = $raw
                if ($raw -match '^(HTTP|TLS1\.2|TLS1\.3):(\S+)') {
                    $lab = $matches[1]
                    $st = $matches[2]
                }
                $byLabel[$lab] = $st
            }
            foreach ($lab in @('HTTP', 'TLS1.2', 'TLS1.3')) {
                $st = '-'
                if ($byLabel.ContainsKey($lab)) {
                    $st = [string]$byLabel[$lab]
                }
                $ck = Get-GuiTestCellKind -Text $st
                if ($ck -eq 'err') {
                    $hasError = $true
                }
                [void]$cells.Add((New-Object PSObject -Property @{ Text = $st; Kind = $ck }))
            }
            $ping = [string](Get-GuiTestInfoProp -Info $target -Name 'PingResult')
            $pk = Get-GuiTestCellKind -Text $ping
            if ($pk -eq 'err') {
                $hasError = $true
            }
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $ping; Kind = $pk }))
            [void]$rows.Add($cells.ToArray())
        }
    }
    $grid = New-GuiTestResultGrid -Headers $headers -Rows $rows
    $sv = New-Object System.Windows.Controls.ScrollViewer
    $sv.VerticalScrollBarVisibility = [System.Windows.Controls.ScrollBarVisibility]::Auto
    $sv.HorizontalScrollBarVisibility = [System.Windows.Controls.ScrollBarVisibility]::Auto
    $sv.Content = $grid
    return @{ Control = $sv; HasError = $hasError }
}

# Add a strategy tab after that strategy finishes.
function Add-GuiTestStrategyTab {
    param($Info)
    if (-not $script:testTabs) {
        return
    }
    $name = [string](Get-GuiTestInfoProp -Info $Info -Name 'Name')
    if ([string]::IsNullOrWhiteSpace($name)) {
        $name = '?'
    }
    $built = New-GuiTestStrategyTable -Info $Info
    $item = New-Object System.Windows.Controls.TabItem
    $hdr = New-Object System.Windows.Controls.TextBlock
    $hdr.Text = $name
    $hdr.TextWrapping = [System.Windows.TextWrapping]::Wrap
    $hdr.MaxWidth = 160
    if ($built.HasError) {
        $hdr.Foreground = Get-GuiTestKindBrush -Kind 'err' -Foreground
        $hdr.FontWeight = [System.Windows.FontWeights]::SemiBold
    }
    $item.Header = $hdr
    $item.Content = $built.Control
    [void]$script:testTabs.Items.Add($item)
    $script:testTabs.SelectedIndex = $script:testTabs.Items.Count - 1
    if ($script:testWaitLbl) {
        $script:testWaitLbl.Visibility = [System.Windows.Visibility]::Collapsed
    }
    $script:testUiPhase = 'mid'
}

# Draw the summary table and the best strategy line.
function Add-GuiTestSummary {
    param($Info)
    $script:testUiPhase = 'sum'
    if (-not $script:testSumHost) {
        return
    }
    $rowsIn = @(Get-GuiTestInfoProp -Info $Info -Name 'Rows')
    $kind = [string](Get-GuiTestInfoProp -Info $Info -Name 'Kind')
    $best = [string](Get-GuiTestInfoProp -Info $Info -Name 'Best')
    if ($script:testBestLbl) {
        if (-not [string]::IsNullOrWhiteSpace($best)) {
            $script:testBestLbl.Text = (Get-ZapmanUiString -Key 'TestsBest' -FormatArgs @($best))
        } else {
            $script:testBestLbl.Text = ''
        }
    }
    if (@($rowsIn).Count -lt 1) {
        $script:testSumHost.Child = $null
        return
    }
    $headers = New-Object System.Collections.ArrayList
    [void]$headers.Add((Get-ZapmanUiString -Key 'TestsGrpStrats'))
    $isStd = ($kind -eq 'standard')
    if ($isStd) {
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumOk'))
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumErr'))
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumUnsup'))
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumPingOk'))
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumPingFail'))
    } else {
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumOk'))
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumFail'))
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumUnsup'))
        [void]$headers.Add((Get-ZapmanUiString -Key 'TestsSumBlocked'))
    }
    $rows = New-Object System.Collections.ArrayList
    foreach ($row in $rowsIn) {
        $name = [string](Get-GuiTestInfoProp -Info $row -Name 'Name')
        $cells = New-Object System.Collections.ArrayList
        [void]$cells.Add((New-Object PSObject -Property @{ Text = $name; Kind = 'info' }))
        if ($isStd) {
            $ok = [string](Get-GuiTestInfoProp -Info $row -Name 'OK')
            $er = [string](Get-GuiTestInfoProp -Info $row -Name 'ERROR')
            $un = [string](Get-GuiTestInfoProp -Info $row -Name 'UNSUP')
            $po = [string](Get-GuiTestInfoProp -Info $row -Name 'PingOK')
            $pf = [string](Get-GuiTestInfoProp -Info $row -Name 'PingFail')
            $ek = 'ok'
            if ((ConvertTo-GuiTestInt $er) -gt 0) { $ek = 'err' }
            $uk = 'ok'
            if ((ConvertTo-GuiTestInt $un) -gt 0) { $uk = 'warn' }
            $pk = 'ok'
            if ((ConvertTo-GuiTestInt $pf) -gt 0) { $pk = 'warn' }
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $ok; Kind = 'ok' }))
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $er; Kind = $ek }))
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $un; Kind = $uk }))
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $po; Kind = 'ok' }))
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $pf; Kind = $pk }))
        } else {
            $ok = [string](Get-GuiTestInfoProp -Info $row -Name 'OK')
            $fl = [string](Get-GuiTestInfoProp -Info $row -Name 'FAIL')
            $un = [string](Get-GuiTestInfoProp -Info $row -Name 'UNSUP')
            $bl = [string](Get-GuiTestInfoProp -Info $row -Name 'BLOCKED')
            $fk = 'ok'
            if ((ConvertTo-GuiTestInt $fl) -gt 0) { $fk = 'err' }
            $uk = 'ok'
            if ((ConvertTo-GuiTestInt $un) -gt 0) { $uk = 'warn' }
            $bk = 'ok'
            if ((ConvertTo-GuiTestInt $bl) -gt 0) { $bk = 'warn' }
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $ok; Kind = 'ok' }))
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $fl; Kind = $fk }))
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $un; Kind = $uk }))
            [void]$cells.Add((New-Object PSObject -Property @{ Text = $bl; Kind = $bk }))
        }
        [void]$rows.Add($cells.ToArray())
    }
    $headerArr = [string[]]($headers.ToArray())
    $script:testSumHost.Child = (New-GuiTestResultGrid -Headers $headerArr -Rows $rows)
}

# Append setup log lines. Switch to the strategy phase on [n/m].
function Add-GuiTestLine {
    param(
        [string]$Text,
        [bool]$NoNewline
    )
    if ($Text -match '\[(\d+)/(\d+)\]') {
        $script:testUiPhase = 'mid'
    }
    $phase = [string]$script:testUiPhase
    $box = $null
    if ($phase -eq 'head') {
        $box = $script:testHead
    }
    if ($box) {
        if ($NoNewline) {
            $box.AppendText($Text)
        } else {
            $box.AppendText($Text + [Environment]::NewLine)
        }
        $box.ScrollToEnd()
    }
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
        if ($script:testLbl) {
            $script:testLbl.Text = (Get-ZapmanUiString -Key 'TestsProgress' -FormatArgs @($matches[1], $matches[2]))
        }
    }
}

function Start-GuiTestRun {
    param(
        [string]$TestType,
        [string[]]$Names
    )

    $nameList = @($Names)
    $dlg = Import-ZapmanXaml 'Tests.xaml'
    # Limit the window height to the work area.
    $workH = [System.Windows.SystemParameters]::WorkArea.Height
    if ($dlg.Height -gt $workH) {
        $dlg.Height = $workH
    }
    $dlg.Title = Get-ZapmanUiString -Key 'TestsTitle'
    $lbl = Get-XamlChild -Root $dlg -Name 'lblStatus'
    $lbl.Text = Get-ZapmanUiString -Key 'TestsStarting'
    $bar = Get-XamlChild -Root $dlg -Name 'barProgress'
    $txtHead = Get-XamlChild -Root $dlg -Name 'txtHead'
    $tabs = Get-XamlChild -Root $dlg -Name 'tabStrategies'
    $waitLbl = Get-XamlChild -Root $dlg -Name 'lblStratWait'
    $waitLbl.Text = Get-ZapmanUiString -Key 'TestsWaitStrat'
    $grpHead = Get-XamlChild -Root $dlg -Name 'grpTestHead'
    $grpHead.Header = Get-ZapmanUiString -Key 'TestsGrpHead'
    $grpStrats = Get-XamlChild -Root $dlg -Name 'grpTestStrats'
    $grpStrats.Header = Get-ZapmanUiString -Key 'TestsGrpStrats'
    $grpSum = Get-XamlChild -Root $dlg -Name 'grpTestSum'
    $grpSum.Header = Get-ZapmanUiString -Key 'TestsGrpSum'
    $sumHost = Get-XamlChild -Root $dlg -Name 'hostSummary'
    $bestLbl = Get-XamlChild -Root $dlg -Name 'lblBest'
    $btnStop = Get-XamlChild -Root $dlg -Name 'btnStop'
    $btnStop.Content = Get-ZapmanUiString -Key 'BtnCancel'

    $script:testExited = $false
    $script:testCancel = $false
    $script:testShown = $false
    $script:testUiPhase = 'head'
    $script:testDlg = $dlg
    $script:testBar = $bar
    $script:testHead = $txtHead
    $script:testTabs = $tabs
    $script:testWaitLbl = $waitLbl
    $script:testSumHost = $sumHost
    $script:testBestLbl = $bestLbl
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
            } -ShouldStop { [bool]$script:testCancel } -OnWait { Invoke-GuiPump } -OnStrategy {
                param($info)
                Add-GuiTestStrategyTab -Info $info
                Invoke-GuiPump
            } -OnSummary {
                param($info)
                Add-GuiTestSummary -Info $info
                Invoke-GuiPump
            }
        } catch {
            if ($script:testHead) {
                $script:testHead.AppendText($_.Exception.Message + [Environment]::NewLine)
                $script:testHead.ScrollToEnd()
            }
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
        $script:testHead = $null
        $script:testTabs = $null
        $script:testWaitLbl = $null
        $script:testSumHost = $null
        $script:testBestLbl = $null
        $script:testLbl = $null
        $script:testBtn = $null
        $script:testNames = $null
        $script:testUiPhase = 'head'
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

function Show-HostsUpdateDialog {
    param($Info)
    $dlg = Import-ZapmanXaml 'Hosts.xaml'
    $dlg.Title = Get-ZapmanUiString -Key 'HostsTitle'
    $lblText = Get-XamlChild -Root $dlg -Name 'lblText'
    $lblText.Text = Get-ZapmanUiString -Key 'HostsNeed'
    $lblCopied = Get-XamlChild -Root $dlg -Name 'lblCopied'
    $lblCopied.Tag = Get-ZapmanUiString -Key 'HostsCopied'
    $btnCopy = Get-XamlChild -Root $dlg -Name 'btnCopy'
    $btnCopy.Content = Get-ZapmanUiString -Key 'BtnHostsCopy'
    $btnCopy.ToolTip = Get-ZapmanUiString -Key 'TipHostsCopy'
    $btnCopy.Tag = $Info
    $btnCopy.Add_Click({
        try {
            Copy-ZapretHostsTemplate -Info $this.Tag
            $lblCopied.Text = [string]$lblCopied.Tag
        } catch {
            Show-ErrorDialog $_.Exception.Message
        }
    })
    $btnOpen = Get-XamlChild -Root $dlg -Name 'btnOpen'
    $btnOpen.Content = Get-ZapmanUiString -Key 'BtnHostsOpen'
    $btnOpen.ToolTip = Get-ZapmanUiString -Key 'TipHostsOpen'
    $btnOpen.Tag = $Info
    $btnOpen.Add_Click({
        try {
            Open-ZapretSystemHosts -Info $this.Tag
        } catch {
            Show-ErrorDialog $_.Exception.Message
        }
    })
    $btnClose = Get-XamlChild -Root $dlg -Name 'btnClose'
    $btnClose.Content = Get-ZapmanUiString -Key 'BtnClose'
    $btnClose.Add_Click({
        $dlg.DialogResult = $true
    })
    [void](Show-ZapmanOwnedDialog -Dialog $dlg)
}

function Update-StrategyListMarks {
    param(
        $ListBox,
        [string]$Engine
    )
    $runningName = Get-ZapretRunningStrategyName
    $installedName = Get-ZapretInstalledStrategyName
    $running = Test-ZapretBypassRunning
    $eng = $Engine
    if ([string]::IsNullOrWhiteSpace($eng)) {
        $eng = Get-ZapretEngine
    }
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
    $btnNone = Get-XamlChild -Root $dlg -Name 'btnNoneWork'
    $btnNone.Content = Get-ZapmanUiString -Key 'BtnStratNone'
    $btnNone.ToolTip = Get-ZapmanUiString -Key 'TipStratNone'
    $syncEngineUi = {
        $eng = 'winws'
        if ($rbWinws2.IsChecked -eq $true) {
            $eng = 'winws2'
        }
        $script:strategyDialogEngine = $eng
        Update-StrategyListMarks -ListBox $lb -Engine $eng
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
            Set-ZapretEngine -Engine $script:strategyDialogEngine | Out-Null
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
            Set-ZapretEngine -Engine $script:strategyDialogEngine | Out-Null
            Install-ZapretService -File $file -OnWait { Invoke-GuiPump }
            Write-GuiLog (Get-ZapmanUiString -Key 'InstallDone' -FormatArgs @($file.BaseName))
            $script:installOk = $true
        }
        if ($script:installOk) {
            $dlg.Close()
        }
    })
    $btnT.Add_Click({
        Set-ZapretEngine -Engine $script:strategyDialogEngine | Out-Null
        $script:strategyPendingTests = $true
        $dlg.Close()
    })
    $btnNone.Add_Click({
        $ans = Show-QuestionDialog -Message (Get-ZapmanUiString -Key 'StratNoneConfirm') -DefaultYes $false
        if ($ans -ne [System.Windows.MessageBoxResult]::Yes) {
            return
        }
        $script:netReset = $null
        Invoke-GuiAction -BusyText (Get-ZapmanUiString -Key 'BtnStratNone') -FreezeUi -Action {
            $script:netReset = Invoke-ZapmanNetworkReset
        }
        $res = $script:netReset
        $script:netReset = $null
        if (-not $res) {
            return
        }
        Write-GuiLog $res.Text
        if (-not $res.Ok) {
            Show-ErrorDialog ((Get-ZapmanUiString -Key 'StratNoneFail') + [Environment]::NewLine + [Environment]::NewLine + $res.Text)
            return
        }
        $reboot = Show-QuestionDialog -Message (Get-ZapmanUiString -Key 'StratNoneDoneReboot') -DefaultYes $false
        if ($reboot -eq [System.Windows.MessageBoxResult]::Yes) {
            Restart-Computer -Force
            return
        }
        Show-InfoDialog (Get-ZapmanUiString -Key 'StratNoneRebootLater')
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
