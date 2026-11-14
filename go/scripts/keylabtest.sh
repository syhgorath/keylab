#!/usr/bin/env bash
# keylabtest.sh - yerel DENEME klasörünü depodan günceller ve KeyLab'ı oradan başlatır (bakımcı/geliştirici aracı).
#
#   bash go/scripts/keylabtest.sh                  # derle, paketleri eşitle, başlat
#   bash go/scripts/keylabtest.sh --sadece-derle   # derle ve eşitle, BAŞLATMA
#   bash go/scripts/keylabtest.sh --paketleri-atla # components/ eşitlemesini atla (yalnız programı güncelle)
#   bash go/scripts/keylabtest.sh --main           # GitHub'daki GÜNCEL main'i ayrı bir klonda çekip (git pull) ondan derle, eşitle, başlat
#
# --main: çalışma klasörünüzdeki dal/değişikliklerden BAĞIMSIZDIR. Ayrı klon ~/Developer/keylab-main (KEYLAB_MAIN_CLONE ile değişir; yoksa
# bu deponun origin adresinden klonlanır, varsa yalnız `git merge --ff-only` yapılır; elle değişiklik yapmayın). Seçenekler birleştirilebilir:
#   bash go/scripts/keylabtest.sh --main --sadece-derle
#
# Deneme klasörü (varsayılan ~/Developer/keylabtest; KEYLABTEST ortam değişkeniyle değişir):
#   keylab                  derlenen program                    ← her çalıştırmada GÜNCELLENİR
#   components/ + kök dosyaları (components.lock, index_signers, release_signers, VERSION)   ← depodan eşitlenir (components/ içinde fazlalar silinir)
#   servers/                SİZİN sunucu verileriniz (server.env, anahtarlar)  ← ASLA değiştirilmez
#   state/                  deneme durum/günlük klasörü (~/.local/share'e dokunmaz) ← ASLA değiştirilmez
set -euo pipefail

SRC="$(cd "$(dirname "$0")/../.." && pwd)"
KT="${KEYLABTEST:-$HOME/Developer/keylabtest}"
START=1; PKG=1; MAIN=0
for a in "$@"; do
    case "$a" in
        --sadece-derle) START=0 ;;
        --paketleri-atla) PKG=0 ;;
        --main) MAIN=1 ;;
        *) echo "bilinmeyen seçenek: $a" >&2; exit 2 ;;
    esac
done

command -v go >/dev/null || { echo "HATA: go bulunamadı" >&2; exit 1; }
if [ "$MAIN" = 1 ]; then
    command -v git >/dev/null || { echo "HATA: git bulunamadı" >&2; exit 1; }
    CLONE="${KEYLAB_MAIN_CLONE:-$HOME/Developer/keylab-main}"
    if [ -d "$CLONE/.git" ]; then
        echo "→ main güncelleniyor: $CLONE"
        if ! { git -C "$CLONE" fetch -q origin main && git -C "$CLONE" checkout -q main && git -C "$CLONE" merge --ff-only -q origin/main; }; then
            echo "HATA: $CLONE güncellenemedi (içinde elle değişiklik/dal mı var? silip yeniden çalıştırabilirsiniz)" >&2; exit 1
        fi
    else
        URL="$(git -C "$SRC" remote get-url origin 2>/dev/null)" || { echo "HATA: origin adresi bulunamadı" >&2; exit 1; }
        echo "→ main klonlanıyor: $CLONE"
        git clone -q "$URL" "$CLONE" || { echo "HATA: klonlanamadı" >&2; exit 1; }
    fi
    SRC="$CLONE"
    # depoda components/ klasörü yoktur (paketler imzalı indeksten kurulur): mevcut paketlerinize dokunulmaz; yoksa Paketler sayfasından kurun
    PKG=0
    echo "   main @ $(git -C "$CLONE" rev-parse --short HEAD): $(git -C "$CLONE" log -1 --format=%s | cut -c1-70)"
fi
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
