@echo off
rem Thin shim so double-click runs the PowerShell build; .ps1 alone opens in an editor.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\build.ps1" %*
pause
