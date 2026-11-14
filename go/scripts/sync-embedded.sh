#!/usr/bin/env bash
# sync-embedded.sh - depo kökündeki güven dosyalarını (release_signers, index_signers, components.lock) go/embedded/ altına kopyalar.
# Bu kopyalar ikiliye gömülür (go/trustdefaults.go). Köktekileri değiştirdiğinizde (ör. components.lock'a yeni paket sürümü, anahtar değişimi)
# bunu çalıştırıp sonucu commit'leyin; `go test` ve build-release.sh kopyaların kökle aynı olduğunu denetler.
#
#   bash go/scripts/sync-embedded.sh          # kopyala
#   bash go/scripts/sync-embedded.sh --check  # yalnız denetle (fark varsa hata kodu 1)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DST="$ROOT/go/embedded"
CHECK=0; [ "${1:-}" = "--check" ] && CHECK=1
rc=0
mkdir -p "$DST"
for f in release_signers index_signers components.lock; do
    [ -f "$ROOT/$f" ] || { echo "HATA: $ROOT/$f yok" >&2; exit 2; }
    if [ "$CHECK" = 1 ]; then
        cmp -s "$ROOT/$f" "$DST/$f" || { echo "FARKLI: go/embedded/$f (kök: $f)" >&2; rc=1; }
    else
        cp -p "$ROOT/$f" "$DST/$f"
    fi
done
if [ "$CHECK" = 1 ]; then
    if [ "$rc" = 0 ]; then echo "gömülü kopyalar kökle aynı"; else echo "çalıştırın: bash go/scripts/sync-embedded.sh" >&2; exit 1; fi
else
    echo "go/embedded/ güncellendi; commit'lemeyi unutmayın"
fi
