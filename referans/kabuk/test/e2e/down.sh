#!/usr/bin/env bash
# Test konteynerini kaldırır. --clean: şifre, host key ve known_hosts kaydını da siler.
set -u
cd "$(dirname "$0")" || exit 1
PORT="${E2E_PORT:-2222}"
docker rm -f sshpocket-e2e >/dev/null 2>&1 && echo "Konteyner kaldırıldı" || echo "Konteyner zaten yok"
if [ "${1:-}" = "--clean" ]; then
    rm -f .secrets
    rm -r .hostkeys 2>/dev/null || true
    ssh-keygen -R "[127.0.0.1]:${PORT}" >/dev/null 2>&1 || true
    ssh-keygen -R "[localhost]:${PORT}" >/dev/null 2>&1 || true
    echo "Test şifresi, host key ve known_hosts kaydı temizlendi"
fi
