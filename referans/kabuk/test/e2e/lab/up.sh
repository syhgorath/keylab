#!/usr/bin/env bash
# LAB sunucusunu SIFIRDAN kurar: systemd'li Ubuntu 26.04 konteyneri, 127.0.0.1:${LAB_PORT:-2223}, kullanıcı 'tester' (anahtarla, parolasız sudo).
# sshpocket için hazır bir "sunucu" girdisi, host key'ler ve ayrı bir durum klasörü .state/ altına yazılır (git'e girmez);
# ~/.ssh/known_hosts ve ~/.local/share/sshpocket DOKUNULMAZ.
set -eu
cd "$(dirname "$0")"
PORT="${LAB_PORT:-2223}"; NAME=sshpocket-lab; IMG=sshpocket-lab:ubuntu26; STATE="$PWD/.state"

command -v docker >/dev/null || { echo "docker bulunamadı (örn. brew install colima docker)"; exit 1; }
docker info >/dev/null 2>&1 || { echo "Docker çalışmıyor (Colima: 'colima start'; Docker Desktop: uygulamayı açın)."; exit 1; }
command -v expect >/dev/null || { echo "expect bulunamadı (macOS'ta hazır gelir; Linux: apt install expect)"; exit 1; }

mkdir -p "$STATE/servers/lab" "$STATE/shim" "$STATE/hostkey"
[ -f "$STATE/id_lab" ] || ssh-keygen -q -t ed25519 -N '' -C sshpocket-lab -f "$STATE/id_lab"
[ -f "$STATE/hostkey/ssh_host_ed25519_key" ] || ssh-keygen -q -t ed25519 -N '' -C sshpocket-lab-host -f "$STATE/hostkey/ssh_host_ed25519_key"
cp "$STATE/id_lab" "$STATE/servers/lab/id_ed25519_lab"; chmod 600 "$STATE/servers/lab/id_ed25519_lab"
printf 'IP=127.0.0.1\nUSER=tester\nPORT=%s\n' "$PORT" > "$STATE/servers/lab/server.env"
# Docker köprüsü 172.17.0.0/16: sunucu bizi 172.17.0.1 olarak görür (gerçek ağ değil, LAB'a özgü)
# hardening_rollback_timeout=30 (izin verilen en düşük): geri alma testleri hızlı bitsin
printf 'hardening_ssh_allow_from=172.17.0.0/16\nhardening_rollback_timeout=30\n' > "$STATE/servers/lab/linux-hardening.vars"
# ssh sarmalayıcı: ~/.ssh/known_hosts yerine lab'a özgü dosya (OpenSSH $HOME'u yok sayar, bu yüzden PATH'e koyulur).
# LAB_BREAK_VERIFY=1: paketin "denetleyiciden YENİ bağlantı" doğrulamasını (ControlMaster=no ile tanınır) bilerek başarısız eder (geri alma testi).
cat > "$STATE/shim/ssh" <<SHIM
#!/bin/bash
if [ "\${LAB_BREAK_VERIFY:-0}" = 1 ]; then
    for a in "\$@"; do
        if [ "\$a" = "ControlMaster=no" ]; then echo "ssh: connect to host 127.0.0.1 port $PORT: Connection timed out" >&2; exit 255; fi
    done
fi
exec /usr/bin/ssh -o UserKnownHostsFile=$STATE/kh "\$@"
SHIM
chmod +x "$STATE/shim/ssh"
mkdir -p "$STATE/tmp"
rm -rf "$STATE/userdata"     # sshpocket durumu (sonuç kayıtları, günlükler) sıfırlanır

docker build -q -t "$IMG" . >/dev/null
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" --privileged --cgroupns=host --tmpfs /run --tmpfs /run/lock \
    -p "127.0.0.1:${PORT}:22" "$IMG" >/dev/null
st=""
for _ in $(seq 1 45); do
    st=$(docker exec "$NAME" systemctl is-system-running 2>/dev/null || true)
    case "$st" in running|degraded) break ;; esac
    sleep 2
done
case "$st" in running|degraded) ;; *) echo "⚠️  systemd 90 sn içinde hazır olmadı (durum: ${st:-?}). Log: docker logs $NAME"; exit 1 ;; esac

docker exec -i "$NAME" sh -c 'cat > /home/tester/.ssh/authorized_keys && chown tester:tester /home/tester/.ssh/authorized_keys && chmod 600 /home/tester/.ssh/authorized_keys' < "$STATE/id_lab.pub"
# Sabit host key: konteyner yeniden kurulunca known_hosts uyarısı çıkmaz
docker cp "$STATE/hostkey/ssh_host_ed25519_key" "$NAME:/etc/ssh/ssh_host_ed25519_key"
docker cp "$STATE/hostkey/ssh_host_ed25519_key.pub" "$NAME:/etc/ssh/ssh_host_ed25519_key.pub"
docker exec "$NAME" sh -c 'chown root:root /etc/ssh/ssh_host_ed25519_key* && chmod 600 /etc/ssh/ssh_host_ed25519_key && chmod 644 /etc/ssh/ssh_host_ed25519_key.pub && systemctl restart ssh'
for _ in $(seq 1 30); do nc -z 127.0.0.1 "$PORT" 2>/dev/null && break; sleep 1; done
nc -z 127.0.0.1 "$PORT" 2>/dev/null || { echo "⚠️  sshd 127.0.0.1:$PORT üzerinde yanıt vermiyor"; exit 1; }
{ printf '[127.0.0.1]:%s ' "$PORT"; cut -d' ' -f1,2 "$STATE/hostkey/ssh_host_ed25519_key.pub"; } > "$STATE/kh"
echo "✅ Hazır: 127.0.0.1:${PORT}  kullanıcı=tester  (Ubuntu 26.04, systemd, parolasız sudo)"
echo "   Çalıştır:  test/e2e/lab/run.sh rollback_status   |  Testler: bats test/e2e/components.bats"
