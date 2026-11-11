#!/usr/bin/env bash
# Yerel test sshd'sini başlatır: 127.0.0.1:${E2E_PORT:-2222}, kullanıcı 'tester'.
# Şifre rastgele üretilir ve test/e2e/.secrets dosyasına yazılır (git'e girmez).
set -eu
cd "$(dirname "$0")"
PORT="${E2E_PORT:-2222}"
NAME="sshpocket-e2e"

command -v docker >/dev/null || { echo "docker bulunamadı"; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker çalışmıyor (Colima: 'colima start'; Docker Desktop: uygulamayı açın)."; exit 1; }

# Rastgele test şifresi (yalnızca bu yerel konteyner için)
if [ ! -f .secrets ]; then
    umask 077
    printf 'E2E_PASSWORD=%s\n' "$(openssl rand -hex 12)" > .secrets
fi
# shellcheck source=/dev/null
. ./.secrets

# Sabit host key: konteyneri yeniden kurunca known_hosts uyarısı çıkmasın
if [ ! -f .hostkeys/ssh_host_ed25519_key ]; then
    mkdir -p .hostkeys
    ssh-keygen -q -t ed25519 -N '' -C e2e-host -f .hostkeys/ssh_host_ed25519_key
fi

docker build -q -t sshpocket-e2e . >/dev/null
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" \
    -p "127.0.0.1:${PORT}:22" \
    -e TESTER_PASSWORD="$E2E_PASSWORD" \
    -v "$PWD/.hostkeys:/hostkeys-src:ro" \
    sshpocket-e2e >/dev/null

for _ in $(seq 1 30); do
    if nc -z 127.0.0.1 "$PORT" 2>/dev/null; then
        echo "✅ Hazır: 127.0.0.1:${PORT}  kullanıcı=tester  (şifre: test/e2e/.secrets)"
        echo "   Testler: bats test/e2e"
        exit 0
    fi
    sleep 1
done
echo "⚠️  30 sn içinde sshd ayağa kalkmadı. Log: docker logs $NAME"
exit 1
