# Run PSScriptAnalyzer with PSScriptAnalyzerSettings.psd1.
# Usage: powershell.exe -File utils\lint.ps1

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$settings = Join-Path $root 'PSScriptAnalyzerSettings.psd1'
if (-not (Test-Path -LiteralPath $settings)) {
    throw 'PSScriptAnalyzerSettings.psd1 is not found.'
}

if (-not (Get-Module -ListAvailable -Name PSScriptAnalyzer)) {
    throw 'Install PSScriptAnalyzer: Install-Module PSScriptAnalyzer -Scope CurrentUser'
}

Import-Module PSScriptAnalyzer -ErrorAction Stop

$files = @(
    Get-ChildItem -LiteralPath (Join-Path $root 'utils') -Filter '*.ps1' -File
    Get-ChildItem -LiteralPath (Join-Path $root 'strategies') -Filter '*.ps1' -File
)

$issues = New-Object System.Collections.Generic.List[object]
foreach ($file in $files) {
    $found = @(Invoke-ScriptAnalyzer -Path $file.FullName -Settings $settings)
    foreach ($item in $found) {
        [void]$issues.Add($item)
    }
}

if ($issues.Count -eq 0) {
    Write-Host ("OK: {0} files, no PSScriptAnalyzer issues." -f $files.Count)
    exit 0
}

$issues |
    Sort-Object ScriptName, Line |
    Format-Table -AutoSize ScriptName, Line, Severity, RuleName, Message
Write-Host ("FAIL: {0} issue(s) in {1} files." -f $issues.Count, $files.Count)
exit 1
