#!/usr/bin/env bash
# keylabtest.sh - yerel DENEME klasörünü depodan günceller ve KeyLab'ı oradan başlatır (bakımcı/geliştirici aracı).
#
#   bash go/scripts/keylabtest.sh                  # derle, paketleri eşitle, başlat
#   bash go/scripts/keylabtest.sh --sadece-derle   # derle ve eşitle, BAŞLATMA
#   bash go/scripts/keylabtest.sh --paketleri-atla # components/ eşitlemesini atla (yalnız programı güncelle)
#
# Deneme klasörü (varsayılan ~/Developer/keylabtest; KEYLABTEST ortam değişkeniyle değişir):
#   keylab                  derlenen program                    ← her çalıştırmada GÜNCELLENİR
#   components/ + kök dosyaları (components.lock, index_signers, release_signers, VERSION)   ← depodan eşitlenir (components/ içinde fazlalar silinir)
#   servers/                SİZİN sunucu verileriniz (server.env, anahtarlar)  ← ASLA değiştirilmez
#   state/                  deneme durum/günlük klasörü (~/.local/share'e dokunmaz) ← ASLA değiştirilmez
set -euo pipefail

SRC="$(cd "$(dirname "$0")/../.." && pwd)"
KT="${KEYLABTEST:-$HOME/Developer/keylabtest}"
START=1; PKG=1
for a in "$@"; do
    case "$a" in
        --sadece-derle) START=0 ;;
        --paketleri-atla) PKG=0 ;;
        *) echo "bilinmeyen seçenek: $a" >&2; exit 2 ;;
    esac
done

command -v go >/dev/null || { echo "HATA: go bulunamadı" >&2; exit 1; }
case "$KT" in ""|"/"|"$HOME"|"$SRC") echo "HATA: geçersiz deneme klasörü: '$KT'" >&2; exit 1 ;; esac

mkdir -p "$KT/servers" "$KT/state"
echo "→ derleniyor: $KT/keylab"
(cd "$SRC/go" && go build -o "$KT/keylab" .)

if [ "$PKG" = 1 ]; then
    if [ -d "$SRC/components" ]; then
        echo "→ components/ eşitleniyor"
        mkdir -p "$KT/components"
        rsync -a --delete "$SRC/components/" "$KT/components/"
    else
        echo "UYARI: $SRC/components yok; paketler eşitlenmedi"
    fi
    for f in components.lock index_signers release_signers VERSION; do
        if [ -e "$SRC/$f" ]; then cp -p "$SRC/$f" "$KT/$f"; fi
    done
fi

echo "→ hazır: $KT"
echo "  sunucu verilerinizi şuraya koyun: $KT/servers/<ad>/server.env (+ id_ed25519_<ad>)"
if [ "$START" = 0 ]; then exit 0; fi
exec "$KT/keylab" -servers "$KT/servers" -components "$KT/components" -state "$KT/state"
