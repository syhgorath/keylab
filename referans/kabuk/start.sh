#!/usr/bin/env bash
# start.sh - Launcher'ı başlatır (macOS USB takınca otomatik çalıştırmaz; elle çalıştırın)
DIR="$(cd "$(dirname "$0")" && pwd)"
MAIN="$DIR/sistem/Main.sh"
[ -f "$MAIN" ] || { echo "HATA: $MAIN bulunamadı" >&2; exit 1; }

if [ -t 0 ]; then
    exec bash "$MAIN" "$@"
fi
# Terminal dışından (Finder vb.) açıldıysa Terminal penceresi aç
cmd=$(printf '%q ' bash "$MAIN" "$@")
cmd="${cmd//\\/\\\\}"; cmd="${cmd//\"/\\\"}"
osascript -e 'tell application "Terminal"' -e 'activate' -e "do script \"$cmd\"" -e 'end tell'
