#!/bin/sh
# Double-click for FULL mode (WARP tunnel): fixes the Discord DESKTOP app + voice.
# You'll be asked for your Mac password (routing traffic needs admin).
# Keep this window open while using Discord; close it (or Ctrl+C) to stop.
cd "$(dirname "$0")" || exit 1
echo "unsni — TAM tünel modu (Discord masaüstü uygulaması + ses)."
echo "İstenince Mac şifreni gir. Bu pencereyi açık tut; durdurmak için kapat."
echo "unsni — FULL tunnel mode (Discord desktop app + voice)."
echo "Enter your Mac password when asked. Keep this window open; close it to stop."
echo
sudo ./unsni tunnel
