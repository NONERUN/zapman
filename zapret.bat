@echo off
:: Validate Windows PowerShell and start the GUI.
:: Do not call pwsh. This script needs powershell.exe 3.0 or newer.

setlocal EnableDelayedExpansion
cd /d "%~dp0"

where powershell.exe >nul 2>&1
if errorlevel 1 (
    echo ERROR: powershell.exe is not found in PATH.
    echo Install Windows PowerShell 5.1 ^(WMF 5.1 on Windows 7^).
    pause
    exit /b 1
)

if not exist "%~dp0utils\gui.ps1" (
    echo ERROR: utils\gui.ps1 is not found.
    pause
    exit /b 1
)

if not exist "%~dp0utils\check-env.ps1" (
    echo ERROR: utils\check-env.ps1 is not found.
    pause
    exit /b 1
)

set "PS_MAJOR="
for /f "usebackq delims=" %%V in (`powershell.exe -NoProfile -Command "[int]$PSVersionTable.PSVersion.Major"`) do set "PS_MAJOR=%%V"

if not defined PS_MAJOR (
    echo ERROR: Cannot read the Windows PowerShell version.
    echo Install Windows PowerShell 5.1.
    pause
    exit /b 1
)

if !PS_MAJOR! LSS 3 (
    echo ERROR: Windows PowerShell !PS_MAJOR! is too old.
    echo This program needs Windows PowerShell 3.0 or newer. Target: 5.1.
    echo On Windows 7 install WMF 5.1 and .NET Framework 4.5 or newer.
    pause
    exit /b 1
)

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0utils\check-env.ps1"
if errorlevel 1 (
    pause
    exit /b 1
)

powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File "%~dp0utils\gui.ps1"
if errorlevel 1 (
    echo ERROR: The GUI failed to start.
    pause
    exit /b 1
)

endlocal
exit /b 0
