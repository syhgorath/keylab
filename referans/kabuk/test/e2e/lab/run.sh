#!/usr/bin/env bash
# run.sh <eylem> [paket] [sunucu] — LAB sunucusunda sshpocket eylemi çalıştırır (önce up.sh).
# Ayrı known_hosts, ayrı sshpocket durum klasörü ve geçici ssh-agent kullanır; kullanıcının ayarlarına dokunmaz.
set -u
L="$(cd "$(dirname "$0")" && pwd)"; ROOT="$(cd "$L/../../.." && pwd)"; STATE="$L/.state"
[ -f "$STATE/kh" ] || { echo "LAB hazır değil: önce test/e2e/lab/up.sh"; exit 1; }
export SSHPOCKET_SERVERS_DIR="$STATE/servers" SSHPOCKET_KNOWN_HOSTS="$STATE/kh" SSHPOCKET_STATE_DIR="$STATE/userdata"
export PATH="$STATE/shim:$PATH" TERM=dumb TMPDIR="$STATE/tmp" E2E_LAB_TMPDIR="$STATE/tmp"
eval "$(ssh-agent -s 3>&-)" >/dev/null
trap 'kill "$SSH_AGENT_PID" 2>/dev/null' EXIT
ssh-add -q "$STATE/servers/lab/id_ed25519_lab"
cd "$ROOT" && "$L/drive.exp" "$@"
