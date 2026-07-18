@echo off
REM Double-click to start unsni with the system proxy on.
REM Close this window (or press Ctrl+C) to stop and restore your settings.
cd /d "%~dp0"
echo unsni baslatiliyor - trafik yonlendiriliyor (sistem proxy ACIK).
echo Windows Guvenlik Duvari sorarsa "Erisime izin ver" de (proxy baglanti alabilsin).
echo Simdi Discord / tarayicini ac. Bitince bu pencereyi kapat (ayarlar geri alinir).
echo.
echo unsni starting - routing your traffic (system proxy ON).
echo If Windows Firewall asks, click "Allow access" so the proxy can accept connections.
echo Open Discord / your browser now. Close this window when done (settings auto-revert).
echo.
unsni.exe run --system-proxy
pause
