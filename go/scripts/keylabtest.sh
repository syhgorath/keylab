#!/usr/bin/env bash
# keylabtest.sh - yerel DENEME klasörünü depodan günceller ve KeyLab'ı oradan başlatır (bakımcı/geliştirici aracı).
#
#   bash go/scripts/keylabtest.sh                  # derle, paketleri eşitle, başlat
#   bash go/scripts/keylabtest.sh --sadece-derle   # derle ve eşitle, BAŞLATMA
#   bash go/scripts/keylabtest.sh --paketleri-atla # components/ eşitlemesini atla (yalnız programı güncelle)
#   bash go/scripts/keylabtest.sh --chrome         # gömülü pencere (macOS varsayılanı: CGO + -tags webview) yerine Chrome/sistem tarayıcısı derlemesi (CGO'suz, hızlı); --web aynı şey
#   bash go/scripts/keylabtest.sh --keychain       # misafir modu ZORLAMA (varsayılan: KEYLAB_GUEST=1; bu seçenekle Keychain/ssh-agent kararı ~/.keylab/keychain-ok işaretine kalır)
#   bash go/scripts/keylabtest.sh --yes            # izinli listede olmayan sunucu adresi uyarısına rağmen başlat
#
# EMNİYETLER (gerçek veriye/sunucuya bulaşmasın):
#   - Sürüm damgası: derleme `-X main.buildVersion=<VERSION>` ile yapılır ve VERSION dosyası deneme klasörüne kopyalanır (yoksa sürüm eski VERSION'dan okunurdu).
#   - Misafir mod: KEYLAB_GUEST=1 verilir (özel ssh-agent, Keychain'e dokunulmaz); "Bu Mac'e güveniyorum" işareti olsa bile.
#   - Adres koruması: servers/*/server.env içindeki IP yalnız 127.0.0.1/localhost/::1, KEYLABTEST_IZINLI (boşluk/virgülle) ya da $KT/lab-hosts (satır başına bir adres, # yorum)
#     listesindeyse sessiz geçer; başka adres varsa başlatma DURUR (--yes ile aşılır). Test VM'lerinizi bilerek lab-hosts'a ekleyin.
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
START=1; PKG=1; MAIN=0; WIN=0; GUEST=1; YES=0
[ "$(uname -s)" = Darwin ] && WIN=1   # macOS: uygulamanın kendi penceresi (yayın derlemesiyle aynı)
for a in "$@"; do
    case "$a" in
        --sadece-derle) START=0 ;;
        --paketleri-atla) PKG=0 ;;
        --chrome|--web) WIN=0 ;;
        --keychain) GUEST=0 ;;
        --yes) YES=1 ;;
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
# sürüm damgası: yayın derlemeleri gibi ikiliye gömülür (yoksa deneme klasöründeki eski VERSION dosyası okunurdu); VERSION dosyası da eşitlenir
VER=""
if [ -r "$SRC/VERSION" ]; then VER="$(head -n1 "$SRC/VERSION" | tr -d '[:space:]')"; cp -p "$SRC/VERSION" "$KT/VERSION"; fi
LDF=""
[ -n "$VER" ] && LDF="-X main.buildVersion=$VER"
echo "→ derleniyor: $KT/keylab${VER:+ (sürüm $VER)}"
if [ "$WIN" = 1 ]; then
    (cd "$SRC/go" && CGO_ENABLED=1 go build -tags webview -ldflags "$LDF" -o "$KT/keylab" .) || { echo "HATA: pencere derlemesi başarısız (Xcode Command Line Tools gerekir; CGO'suz denemek için --chrome)" >&2; exit 1; }
else
    (cd "$SRC/go" && CGO_ENABLED=0 go build -ldflags "$LDF" -o "$KT/keylab" .)
fi

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

# adres koruması: deneme klasörü yalnız LAB/test hedefleri içindir (bkz. başlıktaki EMNİYETLER)
izinli_adres() {
    local a l
    a="$1"
    case "$a" in 127.0.0.1|localhost|::1|"[::1]") return 0 ;; esac
    # shellcheck disable=SC2086 # bilerek: boşlukla/virgülle ayrılmış liste kelimelere bölünür
    for l in ${KEYLABTEST_IZINLI//,/ }; do [ "$l" = "$a" ] && return 0; done
    [ -f "$KT/lab-hosts" ] && sed 's/#.*//; s/[[:space:]]//g' "$KT/lab-hosts" | grep -qxF "$a" && return 0
    return 1
}
BAD=""; NSRV=0
for f in "$KT"/servers/*/server.env; do
    [ -f "$f" ] || continue
    NSRV=$((NSRV + 1))
    ip="$(awk -F= '$1=="IP"{print $2; exit}' "$f" | tr -d "\"' \r")"
    [ -n "$ip" ] || continue
    izinli_adres "$ip" || BAD="$BAD    $(basename "$(dirname "$f")") → $ip"$'\n'
done
if [ -n "$BAD" ]; then
    echo "UYARI: izinli listede olmayan sunucu adresleri var (gerçek sunucu olabilir):" >&2
    printf '%s' "$BAD" >&2
    echo "  Test VM'iyse bilerek ekleyin: echo <adres> >> $KT/lab-hosts   (ya da KEYLABTEST_IZINLI=\"adres1 adres2\")" >&2
    if [ "$START" = 1 ] && [ "$YES" = 0 ]; then echo "  Başlatma DURDURULDU (bilerek geçmek için --yes)." >&2; exit 1; fi
elif [ "$NSRV" -gt 0 ]; then
    echo "  sunucu adresleri izinli ($NSRV)"
fi

if [ "$START" = 0 ]; then exit 0; fi
if [ "$GUEST" = 1 ]; then export KEYLAB_GUEST=1; echo "  misafir mod zorlandı (KEYLAB_GUEST=1: özel ssh-agent, Keychain yok; Keychain'i denemek için --keychain)"; fi
exec "$KT/keylab" -servers "$KT/servers" -components "$KT/components" -state "$KT/state"
