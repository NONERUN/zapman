# Console probe for the host. cli.bat env calls Test-ZapretHostReady. The GUI does not.
# Usage: cli.bat env

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

Import-Module -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'Zapret\Zapret.psd1')

exit (Show-ZapretHostReadyReport)
