# Build SVG/HTML GUI mockups from src\gui\*.xaml and Ui.ps1 strings.
# Usage:
#   powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1
#   powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1 -Check
# Default language is ru. -Language en is for a local preview. Do not commit EN files.

param(
    [switch]$Check,
    [string]$Language = 'ru'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$guiDir = Join-Path $root 'src\gui'
$outDir = Join-Path $root 'docs\ui'
$xamlNs = 'http://schemas.microsoft.com/winfx/2006/xaml'

$script:ZapmanGuiTitleKey = @{
    'Main.xaml'       = 'AppTitle'
    'Strategy.xaml'   = 'StratTitle'
    'Fake.xaml'       = 'FakesTitle'
    'DiagPick.xaml'   = 'DiagTitle'
    'Download.xaml'   = 'IpsetTitle'
    'Hosts.xaml'      = 'HostsTitle'
    'TestsSetup.xaml' = 'TestsSetupTitle'
    'Tests.xaml'      = 'TestsTitle'
    'Status.xaml'     = 'StatusDlgTitle'
}

$script:ZapmanGuiIdleKey = @{
    'Main.xaml'       = @{
        lblBypass    = 'StatusBypassOff'
        lblService   = 'StatusServiceOff'
        lblInstalled = 'StatusStrategyNone'
        lblDivert    = 'StatusDivertNone'
    }
    'Download.xaml'   = @{ lblStatus = 'DownloadConnecting' }
    'Hosts.xaml'      = @{ lblText = 'HostsNeed' }
    'Tests.xaml'      = @{
        lblStatus     = 'TestsStarting'
        lblStratWait  = 'TestsWaitStrat'
        grpTestHead   = 'TestsGrpHead'
        grpTestStrats = 'TestsGrpStrats'
        grpTestSum    = 'TestsGrpSum'
    }
    'TestsSetup.xaml' = @{ lblPick = 'TestsPick' }
}

$script:ZapmanGuiCaption = @{
    'Main.xaml'       = 'Главное'
    'Strategy.xaml'   = 'Стратегия'
    'Fake.xaml'       = 'Fake'
    'TestsSetup.xaml' = 'Прогнать тесты'
    'Tests.xaml'      = 'Тесты'
    'DiagPick.xaml'   = 'Диагностика'
    'Download.xaml'   = 'Скачать'
    'Hosts.xaml'      = 'Сверить hosts'
    'Status.xaml'     = 'Статус'
}

function Get-ZapmanXamlAttr {
    param($Node, [string]$Name)
    if ($null -eq $Node -or $null -eq $Node.Attributes) {
        return ''
    }
    $a = $Node.Attributes.GetNamedItem($Name)
    if ($null -eq $a) {
        return ''
    }
    return [string]$a.Value
}

function Get-ZapmanXamlName {
    param($Node)
    $n = $Node.GetAttribute('Name', $xamlNs)
    if ([string]::IsNullOrWhiteSpace($n)) {
        $n = Get-ZapmanXamlAttr -Node $Node -Name 'Name'
    }
    return $n
}

function Get-ZapmanXamlNumber {
    param($Node, [string]$Name, [double]$Default = 0)
    $raw = Get-ZapmanXamlAttr -Node $Node -Name $Name
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return $Default
    }
    return [double]$raw
}

function Test-ZapmanXamlCollapsed {
    param($Node)
    $v = Get-ZapmanXamlAttr -Node $Node -Name 'Visibility'
    return ($v -eq 'Collapsed' -or $v -eq 'Hidden')
}

function ConvertTo-ZapmanXmlText {
    param([string]$Text)
    $t = [string]$Text
    $t = $t.Replace('&', '&amp;')
    $t = $t.Replace('<', '&lt;')
    $t = $t.Replace('>', '&gt;')
    $t = $t.Replace('"', '&quot;')
    return $t
}

function Get-ZapmanFileBind {
    param($Store, [string]$FileName)
    if (-not $Store.ContainsKey($FileName)) {
        $Store[$FileName] = @{
            Key = @{}
            Tip = @{}
            Lit = @{}
        }
    }
    return $Store[$FileName]
}

function Get-ZapmanBindMap {
    param([string]$RepoRoot)
    $store = @{}
    $reXaml = New-Object System.Text.RegularExpressions.Regex (
        'Import-ZapmanXaml\s+''([^'']+\.xaml)''',
        [System.Text.RegularExpressions.RegexOptions]::Multiline
    )
    $reChild = New-Object System.Text.RegularExpressions.Regex (
        '\$([A-Za-z_][A-Za-z0-9_]*)\s*=\s*Get-XamlChild\s+-Root\s+\$[A-Za-z_][A-Za-z0-9_]*\s+-Name\s+''([^'']+)''',
        [System.Text.RegularExpressions.RegexOptions]::Multiline
    )
    $reKey = New-Object System.Text.RegularExpressions.Regex (
        '\$([A-Za-z_][A-Za-z0-9_]*)\.(Content|Text|Header)\s*=\s*Get-ZapmanUiString\s+-Key\s+''([^'']+)''',
        [System.Text.RegularExpressions.RegexOptions]::Multiline
    )
    $reTip = New-Object System.Text.RegularExpressions.Regex (
        '\$([A-Za-z_][A-Za-z0-9_]*)\.ToolTip\s*=\s*Get-ZapmanUiString\s+-Key\s+''([^'']+)''',
        [System.Text.RegularExpressions.RegexOptions]::Multiline
    )
    $reLit = New-Object System.Text.RegularExpressions.Regex (
        '\$([A-Za-z_][A-Za-z0-9_]*)\.(Content|Text|Header)\s*=\s*''([^'']*)''',
        [System.Text.RegularExpressions.RegexOptions]::Multiline
    )
    foreach ($rel in @('src\gui\gui.ps1', 'src\gui\gui-dialogs.ps1')) {
        $path = Join-Path $RepoRoot $rel
        $raw = [System.IO.File]::ReadAllText($path)
        $current = 'Main.xaml'
        if ($rel -like '*gui-dialogs.ps1') {
            $current = ''
        }
        $varToName = @{}
        $marks = New-Object System.Collections.ArrayList
        foreach ($m in $reXaml.Matches($raw)) {
            [void]$marks.Add((New-Object PSObject -Property @{ Index = $m.Index; Kind = 'xaml'; File = $m.Groups[1].Value; Var = ''; Name = ''; Key = '' }))
        }
        foreach ($m in $reChild.Matches($raw)) {
            [void]$marks.Add((New-Object PSObject -Property @{ Index = $m.Index; Kind = 'child'; File = ''; Var = $m.Groups[1].Value; Name = $m.Groups[2].Value; Key = '' }))
        }
        foreach ($m in $reKey.Matches($raw)) {
            [void]$marks.Add((New-Object PSObject -Property @{ Index = $m.Index; Kind = 'key'; File = ''; Var = $m.Groups[1].Value; Name = ''; Key = $m.Groups[3].Value }))
        }
        foreach ($m in $reTip.Matches($raw)) {
            [void]$marks.Add((New-Object PSObject -Property @{ Index = $m.Index; Kind = 'tip'; File = ''; Var = $m.Groups[1].Value; Name = ''; Key = $m.Groups[2].Value }))
        }
        foreach ($m in $reLit.Matches($raw)) {
            [void]$marks.Add((New-Object PSObject -Property @{ Index = $m.Index; Kind = 'lit'; File = ''; Var = $m.Groups[1].Value; Name = ''; Key = $m.Groups[3].Value }))
        }
        foreach ($item in @($marks | Sort-Object Index)) {
            if ($item.Kind -eq 'xaml') {
                $current = [string]$item.File
                $varToName = @{}
                continue
            }
            if ([string]::IsNullOrWhiteSpace($current)) {
                continue
            }
            $slot = Get-ZapmanFileBind -Store $store -FileName $current
            if ($item.Kind -eq 'child') {
                $varToName[[string]$item.Var] = [string]$item.Name
                continue
            }
            $ctl = [string]$item.Var
            if ($varToName.ContainsKey($ctl)) {
                $ctl = [string]$varToName[$ctl]
            }
            if ($item.Kind -eq 'key') {
                $slot.Key[$ctl] = [string]$item.Key
            } elseif ($item.Kind -eq 'tip') {
                $slot.Tip[$ctl] = [string]$item.Key
            } else {
                $slot.Lit[$ctl] = [string]$item.Key
            }
        }
    }
    return $store
}

function Get-ZapmanFileSlot {
    param($Binds, [string]$FileName)
    if ($Binds -and $Binds.ContainsKey($FileName)) {
        return $Binds[$FileName]
    }
    return @{ Key = @{}; Tip = @{}; Lit = @{} }
}

function Get-ZapmanControlLabel {
    param(
        [string]$FileName,
        [string]$Name,
        $Binds
    )
    if ([string]::IsNullOrWhiteSpace($Name)) {
        return ''
    }
    $idle = $null
    if ($script:ZapmanGuiIdleKey.ContainsKey($FileName)) {
        $idle = $script:ZapmanGuiIdleKey[$FileName]
    }
    if ($idle -and $idle.ContainsKey($Name)) {
        $key = [string]$idle[$Name]
        if ($key -eq 'TestsPick') {
            return (Get-ZapmanUiString -Key $key -FormatArgs @('winws'))
        }
        return (Get-ZapmanUiString -Key $key)
    }
    $slot = Get-ZapmanFileSlot -Binds $Binds -FileName $FileName
    if ($slot.Key.ContainsKey($Name)) {
        $key = [string]$slot.Key[$Name]
        if ($key -eq 'AppTitle') {
            return (Get-ZapmanUiString -Key $key -FormatArgs @(Get-ZapmanLocalVersion))
        }
        if ($key -eq 'TestsPick') {
            return (Get-ZapmanUiString -Key $key -FormatArgs @('winws'))
        }
        return (Get-ZapmanUiString -Key $key)
    }
    if ($slot.Lit.ContainsKey($Name)) {
        return [string]$slot.Lit[$Name]
    }
    return ''
}

function Get-ZapmanControlTip {
    param([string]$FileName, [string]$Name, $Binds)
    if ([string]::IsNullOrWhiteSpace($Name)) {
        return ''
    }
    $slot = Get-ZapmanFileSlot -Binds $Binds -FileName $FileName
    if ($slot.Tip.ContainsKey($Name)) {
        return (Get-ZapmanUiString -Key ([string]$slot.Tip[$Name]))
    }
    return ''
}

function Add-ZapmanSvgRect {
    param(
        $Parts,
        [double]$X,
        [double]$Y,
        [double]$W,
        [double]$H,
        [string]$Fill,
        [string]$Stroke,
        [string]$Extra = ''
    )
    $e = ''
    if (-not [string]::IsNullOrWhiteSpace($Extra)) {
        $e = ' ' + $Extra
    }
    [void]$Parts.Add(('  <rect x="{0}" y="{1}" width="{2}" height="{3}" fill="{4}" stroke="{5}"{6}/>' -f `
        ([int][math]::Round($X)), ([int][math]::Round($Y)), ([int][math]::Round($W)), ([int][math]::Round($H)), $Fill, $Stroke, $e))
}

function Add-ZapmanSvgText {
    param(
        $Parts,
        [double]$X,
        [double]$Y,
        [string]$Text,
        [double]$Size = 12,
        [string]$Anchor = 'start',
        [string]$Weight = 'normal',
        [string]$Fill = '#222'
    )
    if ([string]::IsNullOrWhiteSpace($Text)) {
        return
    }
    [void]$Parts.Add(('  <text x="{0}" y="{1}" font-size="{2}" text-anchor="{3}" font-weight="{4}" fill="{5}">{6}</text>' -f `
        ([int][math]::Round($X)), ([int][math]::Round($Y)), $Size, $Anchor, $Weight, $Fill, (ConvertTo-ZapmanXmlText $Text)))
}

function Get-ZapmanGridRowHeights {
    param($DefNode, [double]$TotalH)
    $raw = New-Object System.Collections.ArrayList
    if ($DefNode) {
        foreach ($rd in @($DefNode.ChildNodes)) {
            if ($rd.NodeType -ne [System.Xml.XmlNodeType]::Element) {
                continue
            }
            if ($rd.LocalName -ne 'RowDefinition') {
                continue
            }
            [void]$raw.Add((Get-ZapmanXamlAttr -Node $rd -Name 'Height'))
        }
    }
    if ($raw.Count -lt 1) {
        [void]$raw.Add('*')
    }
    $fixed = 0.0
    $star = 0.0
    foreach ($h in $raw) {
        $hs = [string]$h
        if ($hs -eq '*' -or [string]::IsNullOrWhiteSpace($hs)) {
            $star += 1
        } elseif ($hs -eq 'Auto') {
            $fixed += 28
        } elseif ($hs -match '^([0-9.]+)\*$') {
            $star += [double]$matches[1]
        } else {
            $fixed += [double]$hs
        }
    }
    $rest = $TotalH - $fixed
    if ($rest -lt 0) {
        $rest = 0
    }
    $out = New-Object System.Collections.ArrayList
    foreach ($h in $raw) {
        $hs = [string]$h
        $val = 0.0
        if ($hs -eq '*' -or [string]::IsNullOrWhiteSpace($hs)) {
            if ($star -gt 0) {
                $val = $rest / $star
            }
        } elseif ($hs -eq 'Auto') {
            $val = 28
        } elseif ($hs -match '^([0-9.]+)\*$') {
            if ($star -gt 0) {
                $val = $rest * [double]$matches[1] / $star
            }
        } else {
            $val = [double]$hs
        }
        [void]$out.Add($val)
    }
    return $out
}

function Add-ZapmanSvgGrid {
    param(
        $Parts,
        $Node,
        [double]$X,
        [double]$Y,
        [double]$W,
        [double]$H,
        [string]$FileName,
        $Binds
    )
    $defNode = $null
    $visuals = New-Object System.Collections.ArrayList
    foreach ($ch in @($Node.ChildNodes)) {
        if ($ch.NodeType -ne [System.Xml.XmlNodeType]::Element) {
            continue
        }
        if ($ch.LocalName -eq 'Grid.RowDefinitions') {
            $defNode = $ch
            continue
        }
        if ($ch.LocalName -eq 'Grid.ColumnDefinitions') {
            continue
        }
        [void]$visuals.Add($ch)
    }
    $heights = Get-ZapmanGridRowHeights -DefNode $defNode -TotalH $H
    if ($null -eq $defNode) {
        foreach ($ch in $visuals) {
            Add-ZapmanSvgControl -Parts $Parts -Node $ch -Ox $X -Oy $Y -FileName $FileName -Binds $Binds -BoxW $W -BoxH $H
        }
        return
    }
    foreach ($ch in $visuals) {
        $ri = 0
        $rs = Get-ZapmanXamlAttr -Node $ch -Name 'Grid.Row'
        if ($rs -match '^\d+$') {
            $ri = [int]$rs
        }
        if ($ri -ge $heights.Count) {
            $ri = $heights.Count - 1
        }
        if ($ri -lt 0) {
            $ri = 0
        }
        $cellY = $Y
        $k = 0
        while ($k -lt $ri) {
            $cellY += [double]$heights[$k]
            $k++
        }
        $cellH = [double]$heights[$ri]
        $mg = Get-ZapmanXamlAttr -Node $ch -Name 'Margin'
        if ($mg -match ',') {
            $mp = $mg -split ','
            if ($mp.Count -eq 4) {
                $cellY += [double]$mp[1]
                $cellH = $cellH - [double]$mp[1] - [double]$mp[3]
            }
        }
        if ($cellH -lt 16) {
            $cellH = 16
        }
        Add-ZapmanSvgControl -Parts $Parts -Node $ch -Ox $X -Oy $cellY -FileName $FileName -Binds $Binds -BoxW $W -BoxH $cellH
    }
}

function Add-ZapmanSvgControl {
    param(
        $Parts,
        $Node,
        [double]$Ox,
        [double]$Oy,
        [string]$FileName,
        $Binds,
        [double]$BoxW = 0,
        [double]$BoxH = 0
    )
    if ($Node.NodeType -ne [System.Xml.XmlNodeType]::Element) {
        return
    }
    if (Test-ZapmanXamlCollapsed -Node $Node) {
        return
    }
    $kind = $Node.LocalName
    $name = Get-ZapmanXamlName -Node $Node
    $x = $Ox + (Get-ZapmanXamlNumber -Node $Node -Name 'Canvas.Left' -Default 0)
    $y = $Oy + (Get-ZapmanXamlNumber -Node $Node -Name 'Canvas.Top' -Default 0)
    $w = Get-ZapmanXamlNumber -Node $Node -Name 'Width' -Default 0
    $h = Get-ZapmanXamlNumber -Node $Node -Name 'Height' -Default 0
    if ($BoxW -gt 0) {
        $w = $BoxW
    }
    if ($BoxH -gt 0) {
        $h = $BoxH
    }
    $label = Get-ZapmanControlLabel -FileName $FileName -Name $name -Binds $Binds
    $tip = Get-ZapmanControlTip -FileName $FileName -Name $name -Binds $Binds
    if ($kind -eq 'Grid') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 80 }
        Add-ZapmanSvgGrid -Parts $Parts -Node $Node -X $x -Y $y -W $w -H $h -FileName $FileName -Binds $Binds
        return
    }
    if ($kind -eq 'GroupBox') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 80 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#f7f7f7' -Stroke '#b5b5b5'
        Add-ZapmanSvgText -Parts $Parts -X ($x + 10) -Y ($y + 14) -Text $label -Size 11 -Weight 'bold'
        $innerW = $w - 8
        $innerH = $h - 22
        if ($innerW -lt 20) { $innerW = 20 }
        if ($innerH -lt 20) { $innerH = 20 }
        foreach ($ch in @($Node.ChildNodes)) {
            if ($ch.NodeType -ne [System.Xml.XmlNodeType]::Element) {
                continue
            }
            Add-ZapmanSvgControl -Parts $Parts -Node $ch -Ox ($x + 4) -Oy ($y + 18) -FileName $FileName -Binds $Binds -BoxW $innerW -BoxH $innerH
        }
        return
    }
    if ($kind -eq 'Canvas') {
        foreach ($ch in @($Node.ChildNodes)) {
            Add-ZapmanSvgControl -Parts $Parts -Node $ch -Ox $x -Oy $y -FileName $FileName -Binds $Binds
        }
        return
    }
    if ($kind -eq 'TabControl') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 80 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#fff' -Stroke '#a0a0a0'
        $strip = 96
        if ($strip -gt ($w * 0.4)) {
            $strip = [Math]::Max(40, $w * 0.28)
        }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $strip -H $h -Fill '#ececec' -Stroke '#a0a0a0'
        return
    }
    if ($kind -eq 'Border') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 40 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#fff' -Stroke '#c8c8c8'
        foreach ($ch in @($Node.ChildNodes)) {
            if ($ch.NodeType -ne [System.Xml.XmlNodeType]::Element) {
                continue
            }
            Add-ZapmanSvgControl -Parts $Parts -Node $ch -Ox ($x + 2) -Oy ($y + 2) -FileName $FileName -Binds $Binds -BoxW ($w - 4) -BoxH ($h - 4)
        }
        return
    }
    if ($kind -eq 'ScrollViewer') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 80 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#fff' -Stroke '#a0a0a0'
        foreach ($ch in @($Node.ChildNodes)) {
            if ($ch.NodeType -ne [System.Xml.XmlNodeType]::Element) {
                continue
            }
            Add-ZapmanSvgControl -Parts $Parts -Node $ch -Ox ($x + 2) -Oy ($y + 2) -FileName $FileName -Binds $Binds -BoxW ($w - 4) -BoxH ($h - 4)
        }
        return
    }
    if ($kind -eq 'StackPanel') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 80 }
        $kids = New-Object System.Collections.ArrayList
        foreach ($ch in @($Node.ChildNodes)) {
            if ($ch.NodeType -ne [System.Xml.XmlNodeType]::Element) {
                continue
            }
            [void]$kids.Add($ch)
        }
        $cy = $y
        $i = 0
        foreach ($ch in $kids) {
            $i++
            $left = ($y + $h) - $cy
            if ($left -lt 12) {
                $left = 12
            }
            $chh = 22
            if ($i -eq $kids.Count) {
                $chh = $left
            }
            Add-ZapmanSvgControl -Parts $Parts -Node $ch -Ox $x -Oy $cy -FileName $FileName -Binds $Binds -BoxW $w -BoxH $chh
            $cy += $chh + 4
        }
        return
    }
    if ($kind -eq 'ListBox' -or $kind -eq 'ItemsControl') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 80 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#fff' -Stroke '#a0a0a0'
        return
    }
    if ($kind -eq 'Button') {
        if ($w -lt 1) { $w = 80 }
        if ($h -lt 1) { $h = 28 }
        [void]$Parts.Add('  <g>')
        if (-not [string]::IsNullOrWhiteSpace($tip)) {
            [void]$Parts.Add(('    <title>{0}</title>' -f (ConvertTo-ZapmanXmlText $tip)))
        }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#e1e1e1' -Stroke '#adadad'
        Add-ZapmanSvgText -Parts $Parts -X ($x + ($w / 2)) -Y ($y + ($h / 2) + 4) -Text $label -Size 11 -Anchor 'middle'
        [void]$Parts.Add('  </g>')
        return
    }
    if ($kind -eq 'RadioButton') {
        if ($w -lt 1) { $w = 90 }
        if ($h -lt 1) { $h = 24 }
        $cy = $y + ($h / 2)
        [void]$Parts.Add(('  <circle cx="{0}" cy="{1}" r="6" fill="#fff" stroke="#666"/>' -f ([int]($x + 8)), ([int]$cy)))
        Add-ZapmanSvgText -Parts $Parts -X ($x + 20) -Y ($cy + 4) -Text $label -Size 12
        return
    }
    if ($kind -eq 'CheckBox') {
        if ($w -lt 1) { $w = 220 }
        if ($h -lt 1) { $h = 22 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y ($y + 3) -W 13 -H 13 -Fill '#fff' -Stroke '#666'
        Add-ZapmanSvgText -Parts $Parts -X ($x + 20) -Y ($y + 15) -Text $label -Size 12
        return
    }
    if ($kind -eq 'ComboBox') {
        if ($w -lt 1) { $w = 120 }
        if ($h -lt 1) { $h = 24 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#fff' -Stroke '#7a7a7a'
        Add-ZapmanSvgRect -Parts $Parts -X ($x + $w - 18) -Y $y -W 18 -H $h -Fill '#e1e1e1' -Stroke '#7a7a7a'
        Add-ZapmanSvgText -Parts $Parts -X ($x + $w - 9) -Y ($y + 16) -Text 'v' -Size 10 -Anchor 'middle' -Fill '#444'
        return
    }
    if ($kind -eq 'ProgressBar') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 18 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#e6e6e6' -Stroke '#a0a0a0'
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W ([math]::Max(24, $w * 0.35)) -H $h -Fill '#cce4f7' -Stroke 'none'
        return
    }
    if ($kind -eq 'TextBox') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 24 }
        Add-ZapmanSvgRect -Parts $Parts -X $x -Y $y -W $w -H $h -Fill '#fff' -Stroke '#7a7a7a'
        if (-not [string]::IsNullOrWhiteSpace($label)) {
            Add-ZapmanSvgText -Parts $Parts -X ($x + 6) -Y ($y + 16) -Text $label -Size 11 -Fill '#666'
        }
        return
    }
    if ($kind -eq 'TextBlock') {
        if ($w -lt 1) { $w = 200 }
        if ($h -lt 1) { $h = 18 }
        $size = 12
        $fw = Get-ZapmanXamlAttr -Node $Node -Name 'FontWeight'
        $fs = Get-ZapmanXamlAttr -Node $Node -Name 'FontSize'
        $weight = 'normal'
        if ($fw -eq 'Bold') {
            $weight = 'bold'
        }
        if (-not [string]::IsNullOrWhiteSpace($fs)) {
            $size = [double]$fs
        }
        $tx = $x
        $ty = $y + $size
        $anchor = 'start'
        $ha = Get-ZapmanXamlAttr -Node $Node -Name 'HorizontalAlignment'
        $va = Get-ZapmanXamlAttr -Node $Node -Name 'VerticalAlignment'
        if ($ha -eq 'Center' -and $w -gt 0) {
            $tx = $x + ($w / 2)
            $anchor = 'middle'
        }
        if ($va -eq 'Center' -and $h -gt 0) {
            $ty = $y + ($h / 2) + ($size / 3)
        }
        Add-ZapmanSvgText -Parts $Parts -X $tx -Y $ty -Text $label -Size $size -Weight $weight -Anchor $anchor
        return
    }
}

function Get-ZapmanDockSide {
    param($Node)
    $d = Get-ZapmanXamlAttr -Node $Node -Name 'DockPanel.Dock'
    if ([string]::IsNullOrWhiteSpace($d)) {
        return 'Fill'
    }
    return $d
}

function Add-ZapmanSvgDockChild {
    param(
        $Parts,
        $Node,
        [double]$X,
        [double]$Y,
        [double]$W,
        [double]$H,
        [string]$FileName,
        $Binds
    )
    if ($Node.NodeType -ne [System.Xml.XmlNodeType]::Element) {
        return
    }
    $kind = $Node.LocalName
    if ($kind -eq 'Grid') {
        Add-ZapmanSvgControl -Parts $Parts -Node $Node -Ox $X -Oy $Y -FileName $FileName -Binds $Binds -BoxW $W -BoxH $H
        return
    }
    $cw = Get-ZapmanXamlNumber -Node $Node -Name 'Width' -Default $W
    $ch = Get-ZapmanXamlNumber -Node $Node -Name 'Height' -Default $H
    $align = Get-ZapmanXamlAttr -Node $Node -Name 'HorizontalAlignment'
    $px = $X
    if ($align -eq 'Right' -and $cw -gt 0 -and $cw -lt $W) {
        $px = $X + $W - $cw
    }
    $tmp = New-Object System.Xml.XmlDocument
    $clone = $tmp.ImportNode($Node, $false)
    if ($clone.Attributes.GetNamedItem('Canvas.Left')) {
        $clone.Attributes.RemoveNamedItem('Canvas.Left') | Out-Null
    }
    $left = $tmp.CreateAttribute('Canvas.Left')
    $left.Value = ([string][int]$px)
    [void]$clone.Attributes.Append($left)
    $top = $tmp.CreateAttribute('Canvas.Top')
    $top.Value = ([string][int]$Y)
    [void]$clone.Attributes.Append($top)
    if ($cw -gt 0 -and -not $clone.Attributes.GetNamedItem('Width')) {
        $aw = $tmp.CreateAttribute('Width')
        $aw.Value = ([string][int]$cw)
        [void]$clone.Attributes.Append($aw)
    }
    if ($ch -gt 0 -and -not $clone.Attributes.GetNamedItem('Height')) {
        $ah = $tmp.CreateAttribute('Height')
        $ah.Value = ([string][int]$ch)
        [void]$clone.Attributes.Append($ah)
    }
    if ($kind -eq 'ScrollViewer' -or $kind -eq 'DockPanel') {
        Add-ZapmanSvgRect -Parts $Parts -X $X -Y $Y -W $W -H $H -Fill '#fff' -Stroke '#a0a0a0'
        return
    }
    Add-ZapmanSvgControl -Parts $Parts -Node $clone -Ox 0 -Oy 0 -FileName $FileName -Binds $Binds
}

function ConvertTo-ZapmanWindowSvg {
    param(
        [System.IO.FileInfo]$File,
        $Binds
    )
    $xml = New-Object System.Xml.XmlDocument
    $xml.PreserveWhitespace = $false
    $xml.Load($File.FullName)
    $win = $xml.DocumentElement
    if ($null -eq $win -or $win.LocalName -ne 'Window') {
        throw ("{0}: root must be Window." -f $File.Name)
    }
    $kids = @($win.ChildNodes | Where-Object { $_.NodeType -eq [System.Xml.XmlNodeType]::Element })
    if ($kids.Count -ne 1) {
        throw ("{0}: Window must have one child panel." -f $File.Name)
    }
    $rootEl = $kids[0]
    $winW = Get-ZapmanXamlNumber -Node $win -Name 'Width' -Default 400
    $winH = Get-ZapmanXamlNumber -Node $win -Name 'Height' -Default 300
    $titleKey = ''
    if ($script:ZapmanGuiTitleKey.ContainsKey($File.Name)) {
        $titleKey = [string]$script:ZapmanGuiTitleKey[$File.Name]
    }
    $title = $File.BaseName
    if (-not [string]::IsNullOrWhiteSpace($titleKey)) {
        if ($titleKey -eq 'AppTitle') {
            $title = Get-ZapmanUiString -Key $titleKey -FormatArgs @(Get-ZapmanLocalVersion)
        } else {
            $title = Get-ZapmanUiString -Key $titleKey
        }
    }
    $bar = 28
    $parts = New-Object System.Collections.ArrayList
    [void]$parts.Add(('<?xml version="1.0" encoding="UTF-8"?>'))
    [void]$parts.Add(('<svg xmlns="http://www.w3.org/2000/svg" width="{0}" height="{1}" viewBox="0 0 {0} {1}" font-family="Segoe UI, Arial, sans-serif">' -f ([int]$winW), ([int]$winH)))
    Add-ZapmanSvgRect -Parts $parts -X 0 -Y 0 -W $winW -H $winH -Fill '#f0f0f0' -Stroke '#6d6d6d'
    Add-ZapmanSvgRect -Parts $parts -X 0 -Y 0 -W $winW -H $bar -Fill '#fff' -Stroke '#c8c8c8'
    Add-ZapmanSvgText -Parts $parts -X 10 -Y 18 -Text $title -Size 12
    if ($rootEl.LocalName -eq 'Canvas') {
        $ox = Get-ZapmanXamlNumber -Node $rootEl -Name 'Canvas.Left' -Default 8
        $oy = $bar + 4
        foreach ($ch in @($rootEl.ChildNodes)) {
            Add-ZapmanSvgControl -Parts $parts -Node $ch -Ox $ox -Oy $oy -FileName $File.Name -Binds $Binds
        }
    } elseif ($rootEl.LocalName -eq 'DockPanel') {
        $margin = Get-ZapmanXamlNumber -Node $rootEl -Name 'Margin' -Default 12
        $rawMargin = Get-ZapmanXamlAttr -Node $rootEl -Name 'Margin'
        if ($rawMargin -match ',') {
            $margin = [double](($rawMargin -split ',')[0])
        }
        $l = $margin
        $t = $bar + $margin
        $r = $winW - $margin
        $b = $winH - $margin
        $dockKids = @($rootEl.ChildNodes | Where-Object { $_.NodeType -eq [System.Xml.XmlNodeType]::Element })
        $i = 0
        foreach ($ch in $dockKids) {
            $i++
            $isLast = ($i -eq $dockKids.Count)
            $side = Get-ZapmanDockSide -Node $ch
            if ($isLast) {
                $side = 'Fill'
            }
            $chh = Get-ZapmanXamlNumber -Node $ch -Name 'Height' -Default 28
            $gap = 8
            if ($side -eq 'Bottom') {
                Add-ZapmanSvgDockChild -Parts $parts -Node $ch -X $l -Y ($b - $chh) -W ($r - $l) -H $chh -FileName $File.Name -Binds $Binds
                $b = $b - $chh - $gap
            } elseif ($side -eq 'Top') {
                Add-ZapmanSvgDockChild -Parts $parts -Node $ch -X $l -Y $t -W ($r - $l) -H $chh -FileName $File.Name -Binds $Binds
                $t = $t + $chh + $gap
            } else {
                Add-ZapmanSvgDockChild -Parts $parts -Node $ch -X $l -Y $t -W ($r - $l) -H ($b - $t) -FileName $File.Name -Binds $Binds
            }
        }
    } else {
        throw ("{0}: root panel must be Canvas or DockPanel, not {1}." -f $File.Name, $rootEl.LocalName)
    }
    [void]$parts.Add('</svg>')
    return (($parts -join "`n") + "`n")
}

function ConvertTo-ZapmanLf {
    param([string]$Text)
    return (([string]$Text) -replace "`r`n", "`n" -replace "`r", "`n")
}

function New-ZapmanGuiIndexMarkdown {
    param($Files)
    $lines = New-Object System.Collections.ArrayList
    [void]$lines.Add('# Окна GUI')
    [void]$lines.Add('')
    [void]$lines.Add('Сгенерировано [`dev/export-gui-docs.ps1`](../../dev/export-gui-docs.ps1). Файлы в этой папке не править руками.')
    [void]$lines.Add('')
    [void]$lines.Add('Макет в **покое** (служба не установлена). Списки стратегий и пункты диагностики заполняет код — на макете пустые рамки.')
    [void]$lines.Add('')
    [void]$lines.Add('Локально: откройте [`index.html`](index.html).')
    [void]$lines.Add('')
    foreach ($f in $Files) {
        $cap = $f.BaseName
        if ($script:ZapmanGuiCaption.ContainsKey($f.Name)) {
            $cap = [string]$script:ZapmanGuiCaption[$f.Name]
        }
        [void]$lines.Add(('## {0}' -f $cap))
        [void]$lines.Add('')
        [void]$lines.Add(('![{0}]({1}.svg)' -f $cap, $f.BaseName))
        [void]$lines.Add('')
    }
    return (($lines -join "`n") + "`n")
}

function New-ZapmanGuiIndexHtml {
    param($Files)
    $lines = New-Object System.Collections.ArrayList
    [void]$lines.Add('<!DOCTYPE html>')
    [void]$lines.Add('<html lang="ru"><head><meta charset="utf-8"><title>Zapret Manager GUI</title>')
    [void]$lines.Add('<style>body{font-family:Segoe UI,Arial,sans-serif;background:#f4f4f4;margin:24px} img{background:#fff;box-shadow:0 1px 4px #0002;margin:8px 0 24px}</style>')
    [void]$lines.Add('</head><body>')
    [void]$lines.Add('<!-- Generated by dev/export-gui-docs.ps1. Do not edit. -->')
    [void]$lines.Add('<h1>Окна GUI</h1>')
    [void]$lines.Add('<p>Сгенерировано из XAML. Не править руками.</p>')
    foreach ($f in $Files) {
        $cap = $f.BaseName
        if ($script:ZapmanGuiCaption.ContainsKey($f.Name)) {
            $cap = [string]$script:ZapmanGuiCaption[$f.Name]
        }
        [void]$lines.Add(('<h2>{0}</h2>' -f (ConvertTo-ZapmanXmlText $cap)))
        [void]$lines.Add(('<p><img src="{0}.svg" alt="{1}"></p>' -f $f.BaseName, (ConvertTo-ZapmanXmlText $cap)))
    }
    [void]$lines.Add('</body></html>')
    return (($lines -join "`n") + "`n")
}

$lang = ([string]$Language).Trim().ToLowerInvariant()
if ($lang -ne 'ru' -and $lang -ne 'en') {
    throw 'Language must be ru or en.'
}

Import-Module -Force (Join-Path $root 'src\Zapman\Zapman.psd1')
$mod = Get-Module -Name Zapman
& $mod { param($c) $script:ZapmanUiLang = $c } $lang

$binds = Get-ZapmanBindMap -RepoRoot $root
$xamlFiles = New-Object System.Collections.ArrayList
$seen = @{}
foreach ($name in @('Main.xaml', 'Strategy.xaml', 'Fake.xaml', 'TestsSetup.xaml', 'Tests.xaml', 'DiagPick.xaml', 'Download.xaml', 'Hosts.xaml', 'Status.xaml')) {
    $p = Join-Path $guiDir $name
    if (Test-Path -LiteralPath $p) {
        [void]$xamlFiles.Add((Get-Item -LiteralPath $p))
        $seen[$name] = $true
    }
}
foreach ($item in @(Get-ChildItem -LiteralPath $guiDir -Filter '*.xaml' -File | Sort-Object Name)) {
    if (-not $seen.ContainsKey($item.Name)) {
        [void]$xamlFiles.Add($item)
    }
}
$xamlFiles = @($xamlFiles)
if ($xamlFiles.Count -lt 1) {
    throw 'No XAML files in src\gui.'
}

$wanted = @{}
foreach ($f in $xamlFiles) {
    $wanted[($f.BaseName + '.svg')] = (ConvertTo-ZapmanWindowSvg -File $f -Binds $binds)
}
$wanted['index.md'] = (New-ZapmanGuiIndexMarkdown -Files $xamlFiles)
$wanted['index.html'] = (New-ZapmanGuiIndexHtml -Files $xamlFiles)

$utf8 = New-Object System.Text.UTF8Encoding $false

if ($Check) {
    $bad = New-Object System.Collections.ArrayList
    if (-not (Test-Path -LiteralPath $outDir)) {
        throw 'docs\ui is missing. Run dev\export-gui-docs.ps1'
    }
    foreach ($name in @($wanted.Keys | Sort-Object)) {
        $path = Join-Path $outDir $name
        if (-not (Test-Path -LiteralPath $path)) {
            [void]$bad.Add($name + ': missing')
            continue
        }
        $onDisk = ConvertTo-ZapmanLf ([System.IO.File]::ReadAllText($path))
        $expect = ConvertTo-ZapmanLf ([string]$wanted[$name])
        if ($onDisk -ne $expect) {
            [void]$bad.Add($name + ': stale')
        }
    }
    foreach ($item in @(Get-ChildItem -LiteralPath $outDir -File)) {
        if (-not $wanted.ContainsKey($item.Name)) {
            [void]$bad.Add($item.Name + ': extra')
        }
    }
    if ($bad.Count -gt 0) {
        throw ('docs/ui is stale ({0}). Run: powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1' -f (($bad -join '; ')))
    }
    Write-Host ('OK: docs/ui matches {0} XAML window(s).' -f $xamlFiles.Count)
    return
}

if (-not (Test-Path -LiteralPath $outDir)) {
    New-Item -ItemType Directory -Path $outDir | Out-Null
}
foreach ($item in @(Get-ChildItem -LiteralPath $outDir -File)) {
    if (-not $wanted.ContainsKey($item.Name)) {
        Remove-Item -LiteralPath $item.FullName -Force
    }
}
foreach ($name in @($wanted.Keys)) {
    [System.IO.File]::WriteAllText((Join-Path $outDir $name), [string]$wanted[$name], $utf8)
}
Write-Host ('Wrote {0} file(s) under docs\ui.' -f $wanted.Count)
