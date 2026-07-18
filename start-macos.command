#!/bin/sh
# Double-click to start unsni with the system proxy on.
# Close this window (or press Ctrl+C) to stop and restore your settings.
cd "$(dirname "$0")" || exit 1
echo "unsni başlatılıyor — trafik yönlendiriliyor (sistem proxy AÇIK)."
echo "Şimdi Discord / tarayıcını aç. Bitince bu pencereyi kapat (ayarlar geri alınır)."
echo "unsni starting — routing your traffic (system proxy ON)."
echo "Open Discord / your browser now. Close this window when done (settings auto-revert)."
echo
./unsni run --system-proxy
