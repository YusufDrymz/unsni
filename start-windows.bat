@echo off
REM Double-click to start unsni with the system proxy on.
REM Close this window (or press Ctrl+C) to stop and restore your settings.
cd /d "%~dp0"
echo unsni starting - routing your traffic (system proxy ON).
echo Open Discord / your browser now. Close this window when done (settings auto-revert).
echo.
unsni.exe run --system-proxy
pause
