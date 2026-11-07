#!/usr/bin/env bash
# start.command - Finder'da çift tıklayınca Terminal'de sshpocket'i başlatır (macOS)
# Not: macOS USB takılınca otomatik çalıştırma yapmaz; bu yalnızca tek tıkla başlatma kolaylığıdır.
# İlk açılışta Gatekeeper onay isteyebilir (sağ tık > Aç).
cd "$(dirname "$0")" || exit 1
exec bash ./start.sh "$@"
