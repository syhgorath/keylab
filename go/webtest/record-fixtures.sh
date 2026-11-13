#!/bin/bash
# Arayüz testlerinin sahte API yanıtlarını GERÇEK Go sunucusundan kaydeder (şekil birebir doğru kalsın).
# Kullanım: cd go/webtest && bash record-fixtures.sh      (yalnız fixture'ları yenilemek gerektiğinde)
# Sabit sahte veriyle (üç sunucu, sabit tarihli kayıtlar, Lynis geçmişi) geçici bir ortam kurar, sunucuyu başlatır,
# yanıtları fixtures/*.json olarak yazar ve geçici ortamı siler. Gerçek sunucu/anahtar/durum klasörlerine DOKUNMAZ.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
T="$(mktemp -d /tmp/keylab-fixtures.XXXXXX)"
case "$T" in /tmp/keylab-fixtures.*) ;; *) echo "geçici klasör beklenmedik: $T"; exit 1;; esac
PID=""
cleanup() { [ -n "$PID" ] && kill "$PID" 2>/dev/null || true; rm -rf "$T"; }
trap cleanup EXIT

mkdir -p "$T/srv" "$T/state" "$T/bin"
for n in web1 db1 vm1; do
  mkdir -p "$T/srv/$n"
  printf 'IP=10.255.0.%s\nUSER=root\nPORT=22\n' "$(echo "$n" | cksum | cut -c1-2)" > "$T/srv/$n/server.env"
done
# sabit tarihli işlem kayıtları (zaman|paket|sürüm|eylem|sonuç)
printf '2026-11-10T08:00:00Z|linux-hardening|0.13.0|preview|ok\n2026-11-10T09:00:00Z|linux-hardening|0.13.0|audit|ok\n' > "$T/srv/web1/components.log"
printf '2026-11-10T10:00:00Z|utility|0.1.0|durum|fail\n' > "$T/srv/db1/components.log"
# Lynis geçmişi (web1)
for d in 20261101:61:3 20261105:68:2 20261110:74:1; do
  IFS=: read -r day score warn <<<"$d"
  dir="$T/state/linux-hardening/web1/out/web1/$day"; mkdir -p "$dir"
  printf '{"hardening_index":%s,"warnings":%s,"suggestions":12,"date":"%s-%s-%s"}\n' "$score" "$warn" "${day:0:4}" "${day:4:2}" "${day:6:2}" > "$dir/summary.json"
done

( cd "$repo/go" && go build -o "$T/keylab" . )
"$T/keylab" -servers "$T/srv" -components "$repo/components" -state "$T/state" -no-open > "$T/log" 2>&1 &
PID=$!
for i in $(seq 1 50); do grep -q 'http://127.0.0.1' "$T/log" 2>/dev/null && break; sleep 0.2; done
url="$(grep -o 'http://127.0.0.1:[0-9]*/?t=[0-9a-f]*' "$T/log" | head -1)"
base="${url%%/?t=*}"
curl -fsS -c "$T/jar" -o /dev/null "$url"

out="$here/fixtures"; mkdir -p "$out"
get() { # ad yol
  curl -fsS -b "$T/jar" -H 'X-Requested-With: sshpocket' "$base$2" > "$out/$1.json"
}
get home /api/home
get status /api/status?detail=1
get jobs /api/jobs
get batch /api/batch
get ui-settings /api/ui-settings
get packages /api/packages
get about /api/about
get profiles /api/profiles
get logs /api/logs
for n in web1 db1 vm1; do get "server-$n" "/api/server?name=$n"; get "lynis-$n" "/api/lynis?server=$n"; done
for n in web1 db1 vm1; do for p in linux-hardening utility kurulum; do get "vars-$n-$p" "/api/vars?server=$n&package=$p"; done; done
# makineye/zamana bağlı alanları sabitle
python3 - "$out" "$T" <<'PY'
import json, sys, glob, os, re
out, tmp = sys.argv[1], sys.argv[2]
for f in sorted(glob.glob(out + '/*.json')):
    s = open(f).read().replace(tmp, '/fixture')
    s = re.sub(r'/Users/[^"\\]+', '/fixture/path', s)
    d = json.loads(s)
    if os.path.basename(f) == 'about.json':  # makineye/sürüme bağlı alanlar sabit
        d.update({'version': '0.0.0-test', 'go': 'go-test', 'platform': 'test/test'})
    open(f, 'w').write(json.dumps(d, indent=1, ensure_ascii=False, sort_keys=True) + '\n')
print('kaydedildi:', len(glob.glob(out + '/*.json')), 'dosya')
PY
