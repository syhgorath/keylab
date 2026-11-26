#!/usr/bin/env bash
# keylabtest.sh - yerel DENEME klasörünü depodan günceller ve KeyLab'ı oradan başlatır (bakımcı/geliştirici aracı).
#
#   bash go/scripts/keylabtest.sh                  # derle, paketleri eşitle, başlat
#   bash go/scripts/keylabtest.sh --sadece-derle   # derle ve eşitle, BAŞLATMA
#   bash go/scripts/keylabtest.sh --paketleri-atla # components/ eşitlemesini atla (yalnız programı güncelle)
#   bash go/scripts/keylabtest.sh --chrome         # gömülü pencere (macOS varsayılanı: CGO + -tags webview) yerine Chrome/sistem tarayıcısı derlemesi (CGO'suz, hızlı); --web aynı şey
#   bash go/scripts/keylabtest.sh --keychain       # misafir modu ZORLAMA (varsayılan: KEYLAB_GUEST=1; bu seçenekle Keychain/ssh-agent kararı ~/.keylab/keychain-ok işaretine kalır)
#   bash go/scripts/keylabtest.sh --yes            # izinli listede olmayan sunucu adresi uyarısına rağmen başlat
#   bash go/scripts/keylabtest.sh --main           # GitHub'daki GÜNCEL main'i ayrı bir klonda çekip (git pull) ondan derle, eşitle, başlat
#   bash go/scripts/keylabtest.sh --paket <dizin>  # bir PAKETİN çalışma kopyasını (commit'lenmemiş değişiklikler dahil) YAYINLAMADAN deneme klasörüne kur; birden çok kez verilebilir
#
# --paket (paket geliştirme döngüsü; bkz. aşağıda): ör.  --paket ~/Developer/kurulum --paket ~/Developer/linux-hardening
#   1) arşiv: geçici bir git index'iyle `git add -A` + `git write-tree` + `git archive --prefix=<id>-<sürüm>/` (paket reposunun gerçek index'ine/çalışma ağacına dokunmaz;
#      .gitattributes export-ignore uygulanır: yayın arşiviyle aynı içerik kuralı), 2) `keylab components validate --unsigned`, 3) TEK KULLANIMLIK test anahtarıyla imza
#      ($KT/.testkey; gerçek yayın anahtarına DOKUNULMAZ), 4) $KT/components.lock'ta YALNIZ o paketin girdisi (sürüm, sha256, test imzacısı) yazılır, 5) `components install-local --force`.
#   Kapılar atlanmaz: imza, özet ve kilit denetimleri indirmeyle aynıdır; yalnız güven kökü test anahtarıdır. Bu bir DENEME kopyasıdır, yayımlanmış sürüm değildir.
#   Aynı sürümde içerik değişirse eski kopya `<sürüm>.bozuk-*` olarak kenara alınır (bu script deneme klasöründekileri siler). linux-hardening'de component.json, component.yml'den
#   türetilir: güncel değilse DURUR (PyYAML için repodaki .venv ya da KEYLABTEST_PYTHON kullanılır).
#
# EMNİYETLER (gerçek veriye/sunucuya bulaşmasın):
#   - Sürüm damgası: derleme `-X main.buildVersion=<VERSION>` ile yapılır ve VERSION dosyası deneme klasörüne kopyalanır (yoksa sürüm eski VERSION'dan okunurdu).
#   - Misafir mod: KEYLAB_GUEST=1 verilir (özel ssh-agent, Keychain'e dokunulmaz); "Bu Mac'e güveniyorum" işareti olsa bile. Pencere verisi temizliği yalnız
#     bu deneme ikilisinin klasörünü (~/Library/{WebKit,Caches}/keylab) siler; kurulu KeyLab.app'inkine dokunmaz.
#   - Adres koruması: servers/*/server.env içindeki IP yalnız 127.0.0.1/localhost/::1, KEYLABTEST_IZINLI (boşluk/virgülle) ya da $KT/lab-hosts (satır başına bir adres, # yorum)
#     listesindeyse sessiz geçer; başka adres varsa başlatma DURUR (--yes ile aşılır). Test VM'lerinizi bilerek lab-hosts'a ekleyin.
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
#   .testkey/               --paket test imza anahtarı (tek kullanımlık; gerçek anahtar DEĞİL; depoya girmez)
#   .paket/<id>/            --paket ile üretilen deneme arşivi ve imzası
set -euo pipefail

SRC="$(cd "$(dirname "$0")/../.." && pwd)"
KT="${KEYLABTEST:-$HOME/Developer/keylabtest}"
START=1; PKG=1; MAIN=0; WIN=0; GUEST=1; YES=0
PAKETLER=()
[ "$(uname -s)" = Darwin ] && WIN=1   # macOS: uygulamanın kendi penceresi (yayın derlemesiyle aynı)
while [ $# -gt 0 ]; do
    case "$1" in
        --sadece-derle) START=0 ;;
        --paketleri-atla) PKG=0 ;;
        --chrome|--web) WIN=0 ;;
        --keychain) GUEST=0 ;;
        --yes) YES=1 ;;
        --main) MAIN=1 ;;
        --paket) [ $# -ge 2 ] || { echo "HATA: --paket bir paket dizini ister" >&2; exit 2; }; PAKETLER+=("$2"); shift ;;
        *) echo "bilinmeyen seçenek: $1" >&2; exit 2 ;;
    esac
    shift
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

# --paket: bir paketin çalışma kopyasını YAYINLAMADAN deneme klasörüne kurar (bkz. başlıktaki açıklama)
pdie() { echo "HATA: $*" >&2; exit 1; }
paket_kur() {
    local dir id ver py work idx tree arch key pub sha repo roots
    dir="$(cd "$1" 2>/dev/null && pwd)" || pdie "paket dizini bulunamadı: $1"
    git -C "$dir" rev-parse --git-dir >/dev/null 2>&1 || pdie "git deposu değil: $dir"
    [ -f "$dir/component.json" ] || pdie "component.json yok: $dir"
    id="$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["id"])' "$dir/component.json")" || pdie "component.json okunamadı: $dir"
    ver="$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$dir/component.json")" || pdie "component.json okunamadı: $dir"
    [[ "$id" =~ ^[a-z][a-z0-9-]*$ ]] || pdie "component.json id geçersiz: '$id'"
    [[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || pdie "component.json sürümü geçersiz: '$ver'"
    echo "→ paket: $id $ver  ($dir; dal: $(git -C "$dir" branch --show-current), kaydedilmemiş değişiklik: $(git -C "$dir" status --porcelain | wc -l | tr -d ' '))"
    if [ -f "$dir/VERSION" ] && [ "$(tr -d '[:space:]' < "$dir/VERSION")" != "$ver" ]; then
        echo "  UYARI: VERSION ($(tr -d '[:space:]' < "$dir/VERSION")) component.json sürümüyle ($ver) aynı değil" >&2
    fi

    # ön denetimler (yayın betiklerindekilerin deneme sürümü): component.json eski ise DUR, tutarlılık sorunu yalnız uyarı
    py="${KEYLABTEST_PYTHON:-}"
    if [ -z "$py" ]; then if [ -x "$dir/.venv/bin/python3" ]; then py="$dir/.venv/bin/python3"; else py="python3"; fi; fi
    if [ -f "$dir/scripts/make-component-json.py" ] && [ -f "$dir/component.yml" ]; then
        if "$py" -c 'import yaml' >/dev/null 2>&1; then
            (cd "$dir" && "$py" -I scripts/make-component-json.py --check >/dev/null 2>&1) \
                || pdie "component.json, component.yml'den üretilmiş güncel hâliyle aynı değil: (cd $dir && $py -I scripts/make-component-json.py) ile üretin"
        else
            echo "  UYARI: PyYAML bulunamadı ($py): component.json güncelliği denetlenemedi (repodaki .venv ya da KEYLABTEST_PYTHON)" >&2
        fi
    fi
    if [ -f "$dir/scripts/check-consistency.py" ]; then
        (cd "$dir" && "$py" -I scripts/check-consistency.py >/dev/null 2>&1) \
            || echo "  UYARI: scripts/check-consistency.py başarısız ya da çalıştırılamadı (yayından önce düzeltin: cd $dir && $py -I scripts/check-consistency.py)" >&2
    fi

    # arşiv: geçici index + write-tree + git archive (yayın arşiviyle aynı içerik kuralı; gerçek index/çalışma ağacı değişmez)
    work="$KT/.paket/$id"; mkdir -p "$work"
    idx="$work/index.tmp"; rm -f "$idx"
    GIT_INDEX_FILE="$idx" git -C "$dir" add -A >/dev/null 2>&1 || pdie "geçici index oluşturulamadı: $dir"
    tree="$(GIT_INDEX_FILE="$idx" git -C "$dir" write-tree)" || pdie "git write-tree başarısız: $dir"
    rm -f "$idx"
    arch="$work/$id-$ver.tar.gz"; rm -f "$arch" "$arch.sig"
    git -C "$dir" archive --format=tar.gz --prefix="$id-$ver/" -o "$arch" "$tree" || pdie "arşiv üretilemedi: $dir"
    tar -tzf "$arch" > "$work/liste.txt"
    roots="$(cut -d/ -f1 "$work/liste.txt" | sort -u)"
    [ "$roots" = "$id-$ver" ] || pdie "arşivde beklenmeyen kök dizin(ler): $roots"
    if grep -qE "^$id-$ver/(\.github/|tools/|scripts/|test/|scans/|ornekler/|dist/|\.gitattributes)" "$work/liste.txt"; then
        pdie "arşivde geliştirme dosyaları var ($dir/.gitattributes export-ignore kurallarını kontrol edin)"
    fi
    "$KT/keylab" components -root "$KT" validate "$arch" --unsigned > "$work/validate.txt" 2>&1 \
        || { cat "$work/validate.txt" >&2; pdie "sözleşme uyum denetimi başarısız (yukarıdaki ❌ satırları)"; }
    echo "  arşiv: $(wc -l < "$work/liste.txt" | tr -d ' ') girdi; $(tail -n1 "$work/validate.txt")"

    # test imzası (gerçek yayın anahtarı DEĞİL; deneme klasöründe, depoya girmez)
    key="$KT/.testkey/id_ed25519"
    if [ ! -f "$key" ]; then
        mkdir -p "$KT/.testkey" && chmod 700 "$KT/.testkey"
        ssh-keygen -q -t ed25519 -N "" -C "keylabtest-test-anahtari" -f "$key" || pdie "test anahtarı üretilemedi"
    fi
    ssh-keygen -Y sign -f "$key" -n sshpocket-component "$arch" >/dev/null 2>&1 || pdie "arşiv imzalanamadı"
    pub="$(ssh-keygen -y -f "$key" | cut -d' ' -f1,2)"
    if command -v sha256sum >/dev/null 2>&1; then sha="$(sha256sum "$arch" | cut -d' ' -f1)"; else sha="$(shasum -a 256 "$arch" | cut -d' ' -f1)"; fi

    # kilit: YALNIZ bu paketin girdisi (diğerleri gerçek kilitte kalır); depo adresi mevcut girdiden, yoksa origin'den
    repo="$(git -C "$dir" remote get-url origin 2>/dev/null | sed -E 's#^(https://github.com/|git@github.com:)##; s#\.git$##')" || repo=""
    [[ "$repo" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || repo="lokal/$id"
    [ -f "$KT/components.lock" ] || { [ -f "$SRC/components.lock" ] && cp -p "$SRC/components.lock" "$KT/components.lock"; } || true
    python3 -I - "$KT/components.lock" "$id" "$repo" "$ver" "$sha" "sshpocket-component namespaces=\"sshpocket-component\" $pub" <<'PYEOF' || pdie "components.lock güncellenemedi"
import os, re, sys
path, pid, repo, ver, sha, signers = sys.argv[1:7]
head, secs, order, cur = [], {}, [], None
for line in (open(path).read().splitlines() if os.path.exists(path) else []):
    m = re.match(r'^\[([^\]]+)\]\s*$', line)
    if m:
        cur = m.group(1); order.append(cur); secs[cur] = []
    elif cur is None:
        head.append(line)
    else:
        secs[cur].append(line)
if pid in secs:
    old = dict(l.split(" = ", 1) for l in secs[pid] if " = " in l)
    repo = old.get("repo", repo)
else:
    order.append(pid)
secs[pid] = ["repo = " + repo, "version = " + ver, "sha256 = " + sha, "signers = " + signers]
out = "\n".join(head).rstrip("\n")
out = (out + "\n\n") if out else ""
out += "\n\n".join("[" + k + "]\n" + "\n".join(l for l in secs[k] if l.strip()) for k in order) + "\n"
open(path, "w").write(out)
PYEOF

    "$KT/keylab" components -root "$KT" install-local --force "$id" "$arch" "$arch.sig" || pdie "$id kurulamadı (kurulu sürüm yenisinden büyük olabilir: $KT/components/$id)"
    find "$KT/components/$id" -maxdepth 1 -name '*.bozuk-*' -exec rm -rf {} + 2>/dev/null || true
    echo "  ✅ $id $ver test imzasıyla kuruldu (DENEME kopyası, yayımlanmış sürüm değil)"
}
if [ "${#PAKETLER[@]}" -gt 0 ]; then
    for c in python3 git ssh-keygen tar; do command -v "$c" >/dev/null || pdie "--paket için $c gerekli"; done
    for p in "${PAKETLER[@]}"; do paket_kur "$p"; done
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
