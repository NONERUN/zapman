#Requires -Version 5.1
<#
.SYNOPSIS
Writes the ZapretSpec.psm1 argv of each strategies\*.json to testdata\golden.
The Go builder owns testdata\golden now: write it with
go test ./internal/spec -run "Golden|Edge" -update.
Use this script with -Check to compare the PowerShell module with the Go output.
#>
param([switch]$Check)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repo = Split-Path -Parent $PSScriptRoot
Import-Module -Force -DisableNameChecking (Join-Path $repo 'src\ZapretSpec\ZapretSpec.psd1')

# Fixed root, so that the text does not depend on the clone folder.
$root = 'C:\zapman'
$cases = @(
    @{ Name = 'gf-off'; Tcp = '12'; Udp = '12' },
    @{ Name = 'gf-all'; Tcp = '1024-65535'; Udp = '1024-65535' }
)
$outDir = Join-Path $repo 'testdata\golden'
$utf8 = New-Object System.Text.UTF8Encoding($false)
$diff = 0

foreach ($file in @(Get-ChildItem -LiteralPath (Join-Path $repo 'strategies') -Filter '*.json' -File)) {
    $spec = Get-ZapretStrategySpec -Path $file.FullName
    foreach ($eng in @('winws', 'winws2')) {
        foreach ($case in $cases) {
            $argv = ConvertTo-ZapretStrategyArgList -Spec $spec -Engine $eng `
                -Bin "$root\bin" -Lists "$root\lists" -User "$root\user" `
                -GameFilterTcp $case.Tcp -GameFilterUdp $case.Udp
            $text = (($argv -split "`r?`n") -join "`n").TrimEnd() + "`n"
            $dir = Join-Path $outDir (Join-Path $eng $case.Name)
            $path = Join-Path $dir ($file.BaseName + '.txt')
            if ($Check) {
                $old = $null
                if (Test-Path -LiteralPath $path) { $old = [IO.File]::ReadAllText($path) }
                if ($old -ne $text) {
                    Write-Host ('DIFF {0}' -f $path)
                    $diff++
                }
                continue
            }
            [void](New-Item -ItemType Directory -Force -Path $dir)
            [IO.File]::WriteAllText($path, $text, $utf8)
        }
    }
}

if ($Check -and $diff -gt 0) { exit 1 }
