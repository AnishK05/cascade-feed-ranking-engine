@echo off
REM Windows entry point. Bypasses ExecutionPolicy so this works from cmd.exe
REM and from PowerShell without a prior Set-ExecutionPolicy.
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0cascade.ps1" %*
exit /b %ERRORLEVEL%
