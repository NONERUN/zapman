# Tiny -File entry. The clock starts here. Then this script dotsources gui.ps1.
# The first mark is the parse time of gui.ps1.

$script:startupClock = [System.Diagnostics.Stopwatch]::StartNew()
$script:startupLastMs = 0
$script:startupMarks = New-Object System.Collections.ArrayList

. (Join-Path $PSScriptRoot 'gui.ps1')
