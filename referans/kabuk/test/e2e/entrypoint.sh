#!/bin/sh
# Konteyner başlangıcı: test şifresini ortamdan alır, host key'i sabit tutar, sshd'yi başlatır.
set -e

: "${TESTER_PASSWORD:?TESTER_PASSWORD gerekli}"
echo "tester:${TESTER_PASSWORD}" | chpasswd

if [ -f /hostkeys-src/ssh_host_ed25519_key ]; then
    cp /hostkeys-src/ssh_host_ed25519_key /etc/ssh/e2e_host_key
    chown root:root /etc/ssh/e2e_host_key
    chmod 600 /etc/ssh/e2e_host_key
    exec /usr/sbin/sshd -D -e -h /etc/ssh/e2e_host_key
fi

ssh-keygen -A
exec /usr/sbin/sshd -D -e
