# Console entry for strategy tests. Start: cli.bat tests. The runner is Invoke-ZapmanStrategyTests.
param(
    [ValidateSet('standard', 'dpi')]
    [string]$TestType,
    [string]$Strategies,
    [switch]$NoPause
)

Import-Module -Force -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapman\Zapman.psd1')
[void](Initialize-ZapmanUiLanguage)

$names = @()
if (-not [string]::IsNullOrWhiteSpace($Strategies)) {
    $names = @($Strategies.Split(',') | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

$askType = [string]::IsNullOrWhiteSpace($TestType)
$askNames = [string]::IsNullOrWhiteSpace($Strategies)

$snap = $null
$code = 1
if (Get-ZapretService) {
    Write-Host (Get-ZapmanUiString -Key 'TestsNeedNoService')
    $snap = Suspend-ZapretServiceForTests
}

try {
    $code = Invoke-ZapmanStrategyTests -TestType $TestType -Names $names -AskType:$askType -AskNames:$askNames
    if ($null -eq $code) {
        $code = 1
    }
} finally {
    if ($snap -and $snap.File) {
        Restore-ZapretServiceAfterTests -Snapshot $snap
        Write-Host (Get-ZapmanUiString -Key 'InstallDone' -FormatArgs @($snap.File.BaseName))
    }
}

if (-not $NoPause) {
    Write-Host "Press any key to close..." -ForegroundColor Yellow
    [void][System.Console]::ReadKey($true)
}

exit ([int]$code)
