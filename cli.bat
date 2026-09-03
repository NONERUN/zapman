@echo off
:: After host checks, this console becomes PowerShell. Do not stay in cmd.

setlocal
cd /d "%~dp0"

where powershell.exe >nul 2>&1
if errorlevel 1 (
    echo ERROR: powershell.exe is not found in PATH.
    echo Install Windows PowerShell 5.1 ^(WMF 5.1 on Windows 7^).
    pause
    exit /b 1
)

if not exist "%~dp0src\cli\cli.ps1" (
    echo ERROR: src\cli\cli.ps1 is not found.
    pause
    exit /b 1
)

set "PS=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"
if not exist "%PS%" set "PS=powershell.exe"
set "CLI=%~dp0src\cli\cli.ps1"

if "%~1"=="" goto :noargs

:: Commands: stay attached so CI and scripts get the exit code.
"%PS%" -NoProfile -NoLogo -ExecutionPolicy Bypass -File "%CLI%" %*
exit /b %ERRORLEVEL%

:noargs
:: Explorer starts this file as cmd /c. Open a PowerShell window and close cmd.
echo(%cmdcmdline%) | find /I /C "/c" >nul
if errorlevel 1 goto :sameconsole
start "Zapret Manager CLI" /D "%~dp0" "%PS%" -NoProfile -NoLogo -ExecutionPolicy Bypass -File "%CLI%"
exit 0

:sameconsole
:: Already in a console: hand this window to PowerShell. Do not return to cmd.
endlocal
"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -NoLogo -NoExit -ExecutionPolicy Bypass -File "%~dp0src\cli\cli.ps1"
