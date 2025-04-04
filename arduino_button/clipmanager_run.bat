@echo off
cd /d "%~dp0"
start "" /min powershell.exe -WindowStyle Hidden -ExecutionPolicy Bypass -File "clipmanager.ps1"