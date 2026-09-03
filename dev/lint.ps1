# Project lint: Zapret 5.1/WinForms checks, then PSScriptAnalyzer and Blinter if installed.
# Usage: powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
# Bat files: pipx install Blinter

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$settings = Join-Path $root 'PSScriptAnalyzerSettings.psd1'

function Get-ZapretLintFiles {
    param([string]$RepoRoot)
    return @(
        Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'dev') -Filter '*.ps1' -File
        Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'src\utils') -Filter '*.ps1' -File
        Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'src\Zapret') -File | Where-Object {
            $_.Extension -in @('.ps1', '.psm1', '.psd1')
        }
        Get-ChildItem -LiteralPath (Join-Path $RepoRoot 'strategies') -Filter '*.ps1' -File
    )
}

function New-ZapretLintIssue {
    param(
        [string]$ScriptName,
        [int]$Line,
        [string]$RuleName,
        [string]$Message
    )
    return New-Object PSObject -Property @{
        ScriptName = $ScriptName
        Line       = $Line
        Severity   = 'Error'
        RuleName   = $RuleName
        Message    = $Message
    }
}

function Test-ZapretUtf8Bom {
    param([string]$Path)
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -lt 3) {
        return $false
    }
    return ($bytes[0] -eq 239 -and $bytes[1] -eq 187 -and $bytes[2] -eq 191)
}

function Test-ZapretFileHasCyrillic {
    param([string]$Path)
    $enc = New-Object System.Text.UTF8Encoding $false
    $text = [System.IO.File]::ReadAllText($Path, $enc)
    return [bool]($text -match '\p{IsCyrillic}')
}

function Test-ZapretProjectRules {
    param([System.IO.FileInfo[]]$Files)

    $issues = New-Object System.Collections.ArrayList
    # Split the type name so this file does not match its own rule.
    $listObject = 'New-Object\s+(?:System\.)?Collections\.Generic\.List\[' + '(?:object|PSObject)\]'
    $reListObject = New-Object System.Text.RegularExpressions.Regex (
        $listObject,
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reFormEnabled = New-Object System.Text.RegularExpressions.Regex (
        '\$form\.Enabled\s*=\s*\$false',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reFormHide = New-Object System.Text.RegularExpressions.Regex (
        '\$form\.Hide\s*\(',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reFormShowDialog = New-Object System.Text.RegularExpressions.Regex (
        '\$form\.ShowDialog\s*\(',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reNullCoalesce = New-Object System.Text.RegularExpressions.Regex '\?\?'
    $reParallel = New-Object System.Text.RegularExpressions.Regex (
        '\s-Parallel\b',
        [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
    )
    $reAndAnd = New-Object System.Text.RegularExpressions.Regex '\s&&\s'

    foreach ($file in $Files) {
        $rel = $file.FullName.Substring($root.Length).TrimStart('\', '/')
        $isGui = ($file.Name -eq 'gui.ps1')
        $needsBom = ($file.Name -eq 'Ui.ps1')
        if (-not $needsBom) {
            $needsBom = Test-ZapretFileHasCyrillic -Path $file.FullName
        }
        if ($needsBom -and -not (Test-ZapretUtf8Bom -Path $file.FullName)) {
            [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line 1 -RuleName 'ZapretUtf8Bom' -Message 'Cyrillic or Ui.ps1 must be UTF-8 with BOM. PowerShell 5.1 breaks literals without a BOM.'))
        }

        $n = 0
        foreach ($raw in [System.IO.File]::ReadAllLines($file.FullName)) {
            $n++
            $line = [string]$raw
            $trim = $line.Trim()
            if ($trim.StartsWith('#')) {
                continue
            }
            if ($reListObject.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidNewObjectListObject' -Message 'PS 5.1 parses New-Object Generic.List[object] as an open List. Use ArrayList or a quoted type name.'))
            }
            if ($isGui -and $reFormEnabled.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainFormEnabledFalse' -Message 'Do not set $form.Enabled = $false. That ends the main window message loop.'))
            }
            if ($isGui -and $reFormHide.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainFormHide' -Message 'Do not call $form.Hide() on the main window. Hide ends the UI message loop.'))
            }
            if ($isGui -and $reFormShowDialog.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidMainFormShowDialog' -Message 'The main window must use Application.Run, not $form.ShowDialog. A child dialog disables the owner and ends the UI.'))
            }
            if ($reNullCoalesce.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidPwshNullCoalesce' -Message 'The null-coalescing operator is PowerShell 7. Target is Windows PowerShell 5.1.'))
            }
            if ($reParallel.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidPwshParallel' -Message '-Parallel is PowerShell 7. Target is Windows PowerShell 5.1.'))
            }
            if ($reAndAnd.IsMatch($line)) {
                [void]$issues.Add((New-ZapretLintIssue -ScriptName $rel -Line $n -RuleName 'ZapretAvoidPwshAndAnd' -Message '&& is PowerShell 7. Use a separate statement or -and in 5.1.'))
            }
        }
    }
    return @($issues)
}

$files = @(Get-ZapretLintFiles -RepoRoot $root)
if ($files.Count -lt 1) {
    throw 'No PowerShell files found to lint.'
}

$issues = New-Object System.Collections.ArrayList
foreach ($item in @(Test-ZapretProjectRules -Files $files)) {
    [void]$issues.Add($item)
}

function Get-ZapretBlinterPath {
    $cmd = Get-Command -Name 'blinter' -ErrorAction SilentlyContinue
    if ($cmd) {
        return [string]$cmd.Source
    }
    $cmd = Get-Command -Name 'blinter.exe' -ErrorAction SilentlyContinue
    if ($cmd) {
        return [string]$cmd.Source
    }
    return $null
}

function Invoke-ZapretBlinter {
    param(
        [string]$RepoRoot,
        [string]$BlinterPath
    )
    $outFile = Join-Path $env:TEMP ('zapret-blinter-{0}.json' -f [guid]::NewGuid().ToString('N'))
    $argList = @(
        '--no-recursive'
        '--format'
        'json'
        '--output'
        $outFile
        '.'
    )
    $oldEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        Push-Location -LiteralPath $RepoRoot
        & $BlinterPath @argList
    } finally {
        Pop-Location
        $ErrorActionPreference = $oldEap
    }
    if (-not (Test-Path -LiteralPath $outFile)) {
        throw 'Blinter produced no JSON report.'
    }
    try {
        $raw = Get-Content -LiteralPath $outFile -Raw -Encoding UTF8
        $report = $raw | ConvertFrom-Json
    } finally {
        Remove-Item -LiteralPath $outFile -Force -ErrorAction SilentlyContinue
    }
    $found = New-Object System.Collections.ArrayList
    foreach ($item in @($report.issues)) {
        $sev = 'Warning'
        if ($item.severity -eq 'Error' -or $item.severity -eq 'Security') {
            $sev = 'Error'
        }
        [void]$found.Add((New-Object PSObject -Property @{
            ScriptName = [string]$item.file
            Line       = [int]$item.line
            Severity   = $sev
            RuleName   = [string]$item.code
            Message    = [string]$item.name
        }))
    }
    return @($found)
}

$pssaRan = $false
if (Get-Module -ListAvailable -Name PSScriptAnalyzer) {
    if (-not (Test-Path -LiteralPath $settings)) {
        throw 'PSScriptAnalyzerSettings.psd1 is not found.'
    }
    Import-Module PSScriptAnalyzer -ErrorAction Stop
    foreach ($file in $files) {
        $found = @(Invoke-ScriptAnalyzer -Path $file.FullName -Settings $settings)
        foreach ($item in $found) {
            [void]$issues.Add($item)
        }
    }
    $pssaRan = $true
} else {
    Write-Host 'WARN: PSScriptAnalyzer is not installed. Project rules still ran. CI installs the module.'
}

$blinterRan = $false
$blinterPath = Get-ZapretBlinterPath
if ($blinterPath) {
    foreach ($item in @(Invoke-ZapretBlinter -RepoRoot $root -BlinterPath $blinterPath)) {
        [void]$issues.Add($item)
    }
    $blinterRan = $true
} else {
    Write-Host 'WARN: Blinter is not installed. Project rules still ran. Install: pipx install Blinter'
}

if ($issues.Count -eq 0) {
    $extra = ''
    if ($pssaRan) {
        $extra = $extra + ', PSScriptAnalyzer clean'
    }
    if ($blinterRan) {
        $extra = $extra + ', Blinter clean'
    }
    Write-Host ("OK: {0} files, no Zapret lint issues{1}." -f $files.Count, $extra)
    exit 0
}

$issues |
    Sort-Object ScriptName, Line |
    Format-Table -AutoSize ScriptName, Line, Severity, RuleName, Message
Write-Host ("FAIL: {0} issue(s) in {1} files." -f $issues.Count, $files.Count)
exit 1
