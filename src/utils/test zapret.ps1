# Console entry for strategy tests. Start: cli.bat tests. The runner is Invoke-ZapretStrategyTests.
param(
    [ValidateSet('standard', 'dpi')]
    [string]$TestType,
    [string]$Strategies,
    [switch]$NoPause
)

Import-Module -Force -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Zapret.psd1')

$names = @()
if (-not [string]::IsNullOrWhiteSpace($Strategies)) {
    $names = @($Strategies.Split(',') | ForEach-Object { $_.Trim() } | Where-Object { $_ })
}

$askType = [string]::IsNullOrWhiteSpace($TestType)
$askNames = [string]::IsNullOrWhiteSpace($Strategies)

$code = Invoke-ZapretStrategyTests -TestType $TestType -Names $names -AskType:$askType -AskNames:$askNames
if ($null -eq $code) {
    $code = 1
}

if (-not $NoPause) {
    Write-Host "Press any key to close..." -ForegroundColor Yellow
    [void][System.Console]::ReadKey($true)
}

exit ([int]$code)
