@echo off
:: Start the Zapret GUI.
:: The GUI script must exist in the utils folder.

cd /d "%~dp0"

if not exist "%~dp0utils\gui.ps1" (
    echo ERROR: utils\gui.ps1 is not found.
    pause
    exit /b 1
)

powershell.exe -NoProfile -STA -ExecutionPolicy Bypass -WindowStyle Hidden -File "%~dp0utils\gui.ps1"
if errorlevel 1 (
    echo ERROR: The GUI failed to start.
    pause
    exit /b 1
)
