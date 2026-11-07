#!/usr/bin/env bash
# LAB konteynerini kaldırır. --clean: anahtarlar, host key ve sshpocket durumu (.state/) ile ek Docker ağını da siler.
set -u
cd "$(dirname "$0")" || exit 1
docker rm -f sshpocket-lab >/dev/null 2>&1 && echo "Konteyner kaldırıldı" || echo "Konteyner zaten yok"
docker network rm sshpocket-other >/dev/null 2>&1 || true
if [ "${1:-}" = "--clean" ]; then
    rm -rf .state
    docker rmi sshpocket-lab:ubuntu26 >/dev/null 2>&1 || true
    echo ".state/ ve imaj temizlendi"
fi
