# Zapret Manager module. Load by path. Do not install this module in PSModulePath.
# Engine helpers live in src/Zapret and are dotsourced here.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

. (Join-Path $PSScriptRoot 'Core.ps1')
. (Join-Path $PSScriptRoot 'Ui.ps1')
. (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Bypass.ps1')
. (Join-Path $PSScriptRoot 'Tools.ps1')
. (Join-Path $PSScriptRoot 'Tests.ps1')

function Invoke-ZapmanStrategyTests {
    param(
        [string]$TestType,
        [string[]]$Names,
        [scriptblock]$OnLine,
        [scriptblock]$ShouldStop,
        [switch]$AskType,
        [switch]$AskNames
    )
    Invoke-ZapmanStrategyTestsCore @PSBoundParameters
}

Export-ModuleMember -Function @(
    'Get-ZapretLayout'
    'Get-ZapmanConfig'
    'Get-ZapmanConfigError'
    'Get-ZapmanLocalVersion'
    'Initialize-ZapmanUserLists'
    'Test-ZapmanHostReady'
    'Test-ZapretBinVersions'
    'Show-ZapmanHostReadyReport'
    'Test-ZapmanTcpTimestampsEnabled'
    'Enable-ZapmanTcpTimestamps'
    'Enable-ZapmanTls12'
    'Get-ZapretEngine'
    'Set-ZapretEngine'
    'Get-ZapretEngineExeName'
    'Test-ZapretEngineFiles'
    'Test-ZapretStrategySupportsEngine'
    'Get-ZapretGameFilter'
    'Set-ZapretGameFilterMode'
    'Test-ZapretServiceRunning'
    'Test-ZapretNamedService'
    'Get-ZapretService'
    'Invoke-ZapretStrategyPrep'
    'Get-ZapretStrategyFiles'
    'Start-ZapretWinws'
    'Start-ZapretStrategyFile'
    'ConvertTo-ZapretServiceImagePath'
    'Test-ZapretBypassRunning'
    'Get-ZapretStatusBypassName'
    'Get-ZapretInstalledStrategyName'
    'Get-ZapretRunningStrategyName'
    'Get-ZapretIpsetStatus'
    'Set-ZapretIpsetMode'
    'Stop-ZapretWinwsProcess'
    'Wait-ZapretServiceStopped'
    'Wait-ZapretServiceGone'
    'Stop-ZapretBypass'
    'Wait-ZapretWinws'
    'Get-ZapretWinwsCommandLine'
    'Remove-ZapretServiceRecord'
    'Suspend-ZapretServiceForTests'
    'Restore-ZapretServiceAfterTests'
    'Start-ZapretSelectedStrategy'
    'Start-ZapretServiceIfInstalled'
    'Install-ZapretService'
    'Remove-ZapretServices'
    'Test-ZapmanAutoUpdateEnabled'
    'Set-ZapmanAutoUpdateEnabled'
    'Get-ZapretVersionCheckUrl'
    'Get-ZapretRemoteVersion'
    'Get-ZapretReleasePageUrl'
    'Get-ZapretIpsetListUrl'
    'Get-ZapretHostsSourceUrl'
    'Invoke-ZapmanWebDownload'
    'Update-ZapretIpsetList'
    'Get-ZapretHostsUpdateInfo'
    'Open-ZapretHostsUpdate'
    'Get-ZapretFakeCatalog'
    'Get-ZapretFakeCurrentText'
    'Set-ZapretActiveFake'
    'Invoke-ZapmanStrategyTests'
    'Get-ZapmanStatusLines'
    'Clear-ZapmanDiscordCache'
    'Remove-ZapretNamedServices'
    'Get-ZapmanDiagnosticReport'
    'Invoke-ZapmanDiagnosticAction'
    'Initialize-ZapmanUiLanguage'
    'Get-ZapmanUiLanguage'
    'Set-ZapmanUiLanguage'
    'Get-ZapmanUiString'
    'Enable-ZapmanConsoleUtf8'
)
