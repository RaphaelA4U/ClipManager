@echo off
cd /d "%~dp0"
echo Starting ClipManager button integration...
echo If you see a security warning, please approve it to allow the script to run.
start /MIN powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "clipmanager.ps1"
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo Failed to start the script. This might be due to PowerShell execution policy restrictions.
    echo Please run this script manually first by double-clicking it in the arduino_button folder,
    echo approve any security warnings, and then copy it to the Startup folder.
    echo Press any key to exit...
    pause >nul
)