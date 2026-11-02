#!/usr/bin/env bash
# UI.sh - Ortak ekran öğeleri: başlık (ekmek kırıntısı), ayraç, alt ipucu
# Renk yalnızca hedef terminal ise ve NO_COLOR tanımsızsa kullanılır (boru/test çıktısı düz kalır).

UI_RULE='────────────────────────────────────────────────────────'

# $1 = dosya tanıtıcısı (1 stdout, 2 stderr)
_ui_color() { [ -t "${1:-1}" ] && [ -z "${NO_COLOR:-}" ]; }

# Başlık: boş satır, kalın ekmek kırıntısı, ayraç. Parametre: $1 - "Ana menü › ubuntutest", $2 - fd (ops.)
ui_header() {
    local fd="${2:-1}"
    printf '\n'
    if _ui_color "$fd"; then printf '\033[1m%s\033[0m\n' "$1"; else printf '%s\n' "$1"; fi
    printf '%s\n' "$UI_RULE"
}

# Alt ipucu: ayraç + soluk ipucu satırı. Parametre: $1 - metin, $2 - fd (ops.)
ui_footer() {
    local fd="${2:-1}"
    printf '%s\n' "$UI_RULE"
    if _ui_color "$fd"; then printf '\033[2m%s\033[0m\n' "$1"; else printf '%s\n' "$1"; fi
}

# Grup adının ekranda görünen karşılığı
ui_group_title() {
    case "$1" in
        MainMenu) printf 'Ana menü' ;;
        *) printf '%s' "$1" ;;
    esac
}

# ---------- Ok tuşlu seçim (saf bash, bash 3.2 uyumlu) ----------
# Etkin mi? Ayar UI: numbers → hayır, arrows → evet (zorla), auto → yalnızca gerçek terminalde.
ui_arrows_enabled() {
    local mode=auto
    if declare -F settings_get >/dev/null; then mode=$(settings_get UI); fi
    case "$mode" in
        numbers) return 1 ;;
        arrows) return 0 ;;
        fzf) if ui_fzf_bin >/dev/null 2>&1; then return 0; fi ;;
    esac
    [ -t 0 ] && [ -t 2 ] && [ -n "${TERM:-}" ] && [ "$TERM" != "dumb" ]
}

# fzf'in tam yolu: önce PATH, sonra Homebrew konumları (Finder'dan açılan start.command PATH'inde /opt/homebrew/bin yoktur).
# SSHPOCKET_FZF_DIRS (boşlukla ayrılmış) test/özel kurulum için değiştirir.
ui_fzf_bin() {
    local d p
    if p=$(command -v fzf 2>/dev/null) && [ -n "$p" ]; then printf '%s' "$p"; return 0; fi
    local IFS=' '
    # shellcheck disable=SC2086  # kasıtlı: boşlukla ayrılmış dizin listesi
    for d in ${SSHPOCKET_FZF_DIRS:-/opt/homebrew/bin /usr/local/bin}; do
        if [ -x "$d/fzf" ]; then printf '%s' "$d/fzf"; return 0; fi
    done
    return 1
}

# fzf arka ucu kullanılacak mı? UI=numbers|arrows → hayır; fzf yoksa hayır; auto → yalnızca gerçek terminalde;
# UI=fzf → zorla (fzf kuruluysa).
ui_fzf_enabled() {
    local mode=auto
    if declare -F settings_get >/dev/null; then mode=$(settings_get UI); fi
    case "$mode" in numbers|arrows) return 1 ;; esac
    ui_fzf_bin >/dev/null 2>&1 || return 1
    [ "$mode" = "fzf" ] && return 0
    [ -t 0 ] && [ -t 2 ] && [ -n "${TERM:-}" ] && [ "$TERM" != "dumb" ]
}

# Açıklamalar (isteğe bağlı): çağıran seçici çağrısından ÖNCE doldurur.
#   UI_PICK_DESCS=("1. öğenin açıklaması" "2. öğe..." ...)   UI_PICK_BACK_DESC="geri/çıkış satırının açıklaması"
# Ok tuşlu seçicide alt satırda, fzf'te önizleme penceresinde gösterilir.

# Tek tuş okur, UI_KEY'e yazar: UP DOWN LEFT RIGHT ENTER BACKSPACE ESC EOF OTHER CHAR:<c>
# (Testlerde bu fonksiyon sahte tuş kuyruğuyla değiştirilir.)
_ui_read_key() {
    local k k2 k3
    UI_KEY=""
    IFS= read -rsn1 k || { UI_KEY=EOF; return 0; }
    case "$k" in
        $'\033')
            IFS= read -rsn1 -t 1 k2 || { UI_KEY=ESC; return 0; }
            if [ "$k2" = "[" ] || [ "$k2" = "O" ]; then
                IFS= read -rsn1 -t 1 k3 || { UI_KEY=ESC; return 0; }
                case "$k3" in
                    A) UI_KEY=UP ;;
                    B) UI_KEY=DOWN ;;
                    C) UI_KEY=RIGHT ;;
                    D) UI_KEY=LEFT ;;
                    *) UI_KEY=OTHER ;;
                esac
            else
                UI_KEY=ESC
            fi
            ;;
        "") UI_KEY=ENTER ;;
        $'\177'|$'\b') UI_KEY=BACKSPACE ;;
        *) UI_KEY="CHAR:$k" ;;
    esac
}

_ui_cursor_show() { printf '\033[?25h\033[?7h' >&2; }

# Liste + alt ipucu çizer (stderr). Çağıranın yerel değişkenlerini kullanır: sel n back buf items
_ui_pick_draw() {
    local redraw="$1" i label num hint desc
    printf '\033[?7l' >&2        # otomatik sarma KAPALI: uzun satır sarılıp yeni satır üretmesin (imleç hesabı kaymasın)
    if [ "$redraw" = "1" ]; then printf '\033[%dA' "$((n + 4))" >&2; fi
    i=1
    while [ "$i" -le "$((n + 1))" ]; do
        if [ "$i" -le "$n" ]; then label="${items[$((i - 1))]}"; num="$i"; else label="$back"; num=0; fi
        printf '\r\033[2K' >&2
        if [ "$i" -eq "$sel" ]; then
            printf '\033[7m ›%2d) %s \033[0m\n' "$num" "$label" >&2
        else
            printf '   %2d) %s\n' "$num" "$label" >&2
        fi
        i=$((i + 1))
    done
    printf '\r\033[2K%s\n' "$UI_RULE" >&2
    if [ -n "$buf" ]; then hint="numara: $buf · Enter = seç · Backspace = sil"
    else hint="↑↓ seç · Enter aç · ←/q/0 geri · numara yazıp Enter"; fi
    printf '\r\033[2K\033[2m%s\033[0m\n' "$hint" >&2
    if [ "$sel" -le "$n" ]; then desc="${UI_PICK_DESCS[$((sel - 1))]:-}"; else desc="${UI_PICK_BACK_DESC:-}"; fi
    desc="${desc//$'\n'/ · }"
    printf '\r\033[2K%s\n' "$desc" >&2
    printf '\033[?7h' >&2        # sarma yeniden AÇIK
}

# fzf sürümü tamsayı olarak: 0.74.5 → 7405 (major*10000 + minor*100 + patch); okunamazsa 0
ui_fzf_version() {
    local v a b c
    v=$("${1:-fzf}" --version </dev/null 2>/dev/null | head -n 1 | awk '{print $1}') || v=""
    IFS=. read -r a b c <<< "$v"
    c="${c//[^0-9]/}"
    [[ "$a" =~ ^[0-9]+$ && "${b:-0}" =~ ^[0-9]+$ ]] || { printf '0'; return 0; }
    printf '%d' "$((10#$a * 10000 + 10#${b:-0} * 100 + 10#${c:-0}))" 2>/dev/null || printf '0'
}

# fzf arka ucu: "<no>\t<etiket>" satırları; açıklama --preview ile (dosyadan, kabuğa veri verilmez).
# Parametreler ui_pick_arrows ile aynı. Çıktı: seçilen numara (geri/Esc/hata = 0)
_ui_pick_fzf() {
    local sel="${1:-1}" back="$2"; shift 2
    local -a items=("$@")
    local n=$# tmp i label out rc num fzf_bin key
    fzf_bin=$(ui_fzf_bin) || return 2
    tmp=$(mktemp -d "${TMPDIR:-/tmp}/sshpocket-fzf.XXXXXX") || return 1
    if ! [[ "$tmp" =~ ^[A-Za-z0-9_./-]+$ ]]; then rm -r -- "${tmp:?}"; return 2; fi   # güvenli olmayan yol: kendi seçicimiz
    {
        i=1
        for label in "${items[@]}"; do
            label="${label//$'\t'/ }"; label="${label//$'\n'/ }"
            printf '%d\t%s\n' "$i" "$label"
            i=$((i + 1))
        done
        label="${back//$'\t'/ }"
        printf '0\t%s\n' "$label"
    } > "$tmp/list"
    {
        # çok satırlı açıklama (kart): satır sonu \037 olarak saklanır, önizlemede geri çevrilir
        label="${UI_PICK_BACK_DESC:-}"; printf '%s\n' "${label//$'\n'/$'\037'}"
        i=0
        while [ "$i" -lt "$n" ]; do label="${UI_PICK_DESCS[$i]:-}"; printf '%s\n' "${label//$'\n'/$'\037'}"; i=$((i + 1)); done
    } > "$tmp/desc"
    local -a opts
    opts=(--delimiter=$'\t' --with-nth=2.. --layout=reverse --height='~60%' --min-height=8
          --border --no-sort --cycle --prompt='› '
          --header='Enter: aç · ←/Esc: geri · yazarak ara'
          --preview "awk -v n={1} 'NR==n+1 {gsub(/\037/,\"\\n\"); print}' '$tmp/desc'" --preview-window='down,3,wrap,border-top'
          --bind 'left:abort' --bind "start:pos($sel)")
    if [ -n "${UI_PICK_WIDE:-}" ]; then
        # geniş düzen: sağda önizleme, etiketli çerçeveler (yol çubuğu = kenarlık etiketi); eski fzf'te yalnız sağ önizleme
        local hdr="${UI_PICK_HEADER:-}" ver
        ver=$(ui_fzf_version "$fzf_bin")
        opts=(--delimiter=$'\t' --with-nth=2.. --layout=reverse --height='85%' --min-height=12
              --no-sort --cycle --prompt='› '
              --preview "awk -v n={1} 'NR==n+1 {gsub(/\037/,\"\\n\"); print}' '$tmp/desc'" --preview-window='right,48%,wrap'
              --bind 'left:abort' --bind "start:pos($sel)")
        hdr="${hdr:+$hdr$'\n'}Enter: aç · ←/Esc: geri · yazarak ara${UI_PICK_EXPECT_HINT:+ · $UI_PICK_EXPECT_HINT}"
        opts+=(--header="$hdr")
        if [ "${ver:-0}" -ge 3500 ]; then
            opts+=(--border=rounded --preview-label=' Açıklama ')
            [ -z "${UI_PICK_TITLE:-}" ] || opts+=(--border-label=" $UI_PICK_TITLE ")
        else
            opts+=(--border)
        fi
        if [ "${ver:-0}" -ge 5800 ]; then
            opts+=(--list-border=rounded --input-border=rounded --header-border=rounded --input-label=' Ara ')
        fi
    fi
    [ -z "${UI_PICK_EXPECT:-}" ] || opts+=(--expect="$UI_PICK_EXPECT")
    out=$("$fzf_bin" "${opts[@]}" < "$tmp/list")
    rc=$?
    rm -r -- "${tmp:?}"
    if [ -n "${UI_PICK_EXPECT:-}" ] && [ "$rc" -eq 0 ]; then
        # --expect: ilk satır basılan kısayol (Enter'da boş), sonra seçilen satır
        key="${out%%$'\n'*}"; out="${out#*$'\n'}"
        if [ -n "$key" ]; then printf 'key:%s' "$key"; return 0; fi
    fi
    num="${out%%$'\t'*}"
    if [ "$rc" -eq 0 ] && [[ "$num" =~ ^[0-9]+$ ]] && [ "$num" -le "$n" ]; then printf '%s' "$num"; else printf '0'; fi
    return 0
}

# ui_pick_arrows <başlangıç 1..n> <geri etiketi> <öğe>...
# Seçilen numarayı stdout'a yazar (geri/çıkış = 0). Ekran stderr'e çizilir.
# Arka uç: fzf (kuruluysa, UI=auto|fzf) → kendi ok tuşlu seçicimiz.
ui_pick_arrows() {
    local rc
    if ui_fzf_enabled; then
        _ui_pick_fzf "$@"; rc=$?
        [ "$rc" -ne 2 ] && return "$rc"
    fi
    _ui_pick_builtin "$@"
}

_ui_pick_builtin() {
    local sel="${1:-1}" back="$2"; shift 2
    local items=("$@") n=$# buf="" result=""
    [ "$sel" -ge 1 ] && [ "$sel" -le "$n" ] 2>/dev/null || sel=1
    trap '_ui_cursor_show' INT TERM
    printf '\033[?25l' >&2
    _ui_pick_draw 0
    while true; do
        _ui_read_key
        case "$UI_KEY" in
            UP|CHAR:k)
                buf=""; sel=$((sel - 1)); [ "$sel" -lt 1 ] && sel=$((n + 1)) ;;
            DOWN|CHAR:j)
                buf=""; sel=$((sel + 1)); [ "$sel" -gt "$((n + 1))" ] && sel=1 ;;
            LEFT|CHAR:q|CHAR:Q|EOF|ESC)
                result=0; break ;;
            CHAR:[0-9])
                if [ "${#buf}" -lt 3 ]; then buf="$buf${UI_KEY#CHAR:}"; fi
                if [ "$buf" = "0" ]; then sel=$((n + 1))
                elif [ "$buf" -ge 1 ] 2>/dev/null && [ "$buf" -le "$n" ]; then sel="$buf"; fi
                ;;
            BACKSPACE)
                buf="${buf%?}" ;;
            ENTER)
                if [ -n "$buf" ]; then
                    if [ "$buf" -ge 0 ] 2>/dev/null && [ "$buf" -le "$n" ]; then result="$buf"; break; fi
                    buf=""
                elif [ "$sel" -gt "$n" ]; then result=0; break
                else result="$sel"; break; fi
                ;;
        esac
        _ui_pick_draw 1
    done
    trap - INT TERM
    _ui_cursor_show
    printf '%s' "$result"
}

# Ana menü girdileri için açıklama. Parametre: işlev adı (server:<ad> dahil)
ui_describe_entry() {
    case "$1" in
        server:*)              printf 'Sunucu menüsünü açar: bağlan, key gönder, bilgileri güncelle, Keychain passphrase, knock ayarları.' ;;
        AddServerAction)       printf "Yeni sunucu ekler: IP/kullanıcı/port sorar, isterseniz ed25519 key üretir, passphrase'i Keychain'e kaydeder." ;;
        ImportSSHConfigAction) printf '%s' 'SSH config dosyasındaki (~/.ssh/config) sunucuları buraya aktarır; private key dosyaları kopyalanmaz, yolları kaydedilir.' ;;
        ManageServersAction)   printf 'Sunucuları listeler, siler veya yeniden adlandırır (silmede sunucu adını yazarak onaylarsınız).' ;;
        UpdateAction)          printf 'Yeni sürüm var mı bakar; imzalı paketi doğrulayıp onayınızla kurar. Sunucularınıza ve key'"'"'lerinize dokunmaz.' ;;
        SettingsAction)        printf 'Durum göstergesi, kontrol süresi ve menü stilini (fzf/ok tuşu/numara) ayarlar.' ;;
        ComponentsAction)      printf 'Kurulu paketlerin durumunu gösterir; eksik/eski olanı imzayı doğrulayarak, onayınızla kurar.' ;;
        *)                     printf '' ;;
    esac
}

# Ana menü döngüsüne "bu eylemden sonra duraklatma" der (menüden geri dönüldü; okunacak çıktı yok).
# shellcheck disable=SC2034  # Main.sh döngüsü okur
ui_no_pause() { SSHPOCKET_NO_PAUSE=1; }

# ui_pick <geri-etiketi> <öğe>... → seçilen numara (geri/hata = 0). Etkileşimli modda (UI.sh seçicileri) ok tuşlu/fzf, değilse numaralı menü.
# Bağlam (çağıranın dinamik kapsamı, hepsi isteğe bağlı): bkz. docs/ui-design.md "Seçici bağlam değişkenleri" (UI_PICK_*).
ui_pick() {
    local back="$1" choice i=1 label; shift
    if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled; then
        choice=$(ui_pick_arrows "${UI_PICK_START:-1}" "$back" "$@")
        echo "" >&2
        # fzf kısayolu (UI_PICK_EXPECT): çağıran işler; yalnızca seçicinin kendi çıktısı, elle yazılan değil
        if [ -n "${UI_PICK_EXPECT:-}" ] && [ "$choice" = "key:$UI_PICK_EXPECT" ]; then printf '%s' "$choice"; return 0; fi
    else
        for label in "$@"; do printf '  %d) %s\n' "$i" "$label" >&2; i=$((i + 1)); done
        printf '  0) %s (q)\n' "$back" >&2
        IFS= read -r -p "Seçim [0]: " choice || { printf '0'; return 0; }
    fi
    case "${choice:-0}" in
        q|Q) choice=0 ;;
    esac
    if [[ "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -le "$#" ]; then printf '%s' "$choice"; else printf '0'; fi
}

# ---- Genel liste editörü: öğeler liste olarak görünür; Enter = sil/değiştir, ➕ Ekle, (ek satırlar), 🧹 Temizle, 💾 Kaydet, Vazgeç ----
# Girdi (çağıranın kapsamı):
#   UI_LIST_ITEMS (dizi)   başlangıç öğeleri; çıkışta son hali
#   UI_LIST_TITLE          yol çubuğu (üst başlık/kenarlık etiketi)        UI_LIST_NAME  liste adı
#   UI_LIST_META           üst bilgi satırı (tür, zorunlu, yazılı, varsayılan)   UI_LIST_DESC  açıklama (sağ bölme)
#   UI_LIST_ADD_HINT       "Ekle…" açıklamasına ek metin                   UI_LIST_PROMPT  ekleme istemi (varsayılan "Yeni öğe: ")
#   UI_LIST_REQUIRED       yes → boş liste kaydedilemez
#   UI_LIST_VALIDATE       işlev: <öğe> → 0 geçerli; geçersizse nedenini stdout'a yazar
#   UI_LIST_ADD            işlev (isteğe bağlı): <öğe> → UI_LIST_ITEMS'a kendi kuralıyla ekler, bilgiyi stdout'a yazar (varsayılan: yineleneni atla, sona ekle)
#   UI_LIST_EXTRAS         işlev (isteğe bağlı): UI_LIST_X_LABELS / UI_LIST_X_DESCS / UI_LIST_X_ACTS dizilerini doldurur (ek satırlar)
#   UI_LIST_EXTRA_RUN      işlev (isteğe bağlı): <eylem> → ek satır seçilince çalışır; iletiyi UI_LIST_MSG'ye yazabilir
#   UI_LIST_SAVE_CHECK     işlev (isteğe bağlı): 0 = kaydetmeye devam · 1 = düzenlemeye dön
# Çıkış kodu: 0 Kaydet (son hali UI_LIST_ITEMS) · 1 Vazgeç · 2 girdi bitti (EOF). UI_LIST_DIRTY=1: kaydedilmemiş değişiklik vardı.
ui_edit_list() {
    local i c x v k pick act out msg="" tmpm
    local -a labels acts
    UI_LIST_DIRTY=0; UI_LIST_MSG=""
    tmpm=$(mktemp "${TMPDIR:-/tmp}/sshpocket-liste.XXXXXX") || return 2
    while true; do
        labels=(); acts=(); UI_LIST_X_LABELS=(); UI_LIST_X_DESCS=(); UI_LIST_X_ACTS=()
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local -a UI_PICK_DESCS=()
        i=0
        for c in ${UI_LIST_ITEMS[@]+"${UI_LIST_ITEMS[@]}"}; do
            labels+=("• $c"); acts+=("item:$i"); UI_PICK_DESCS+=("Enter: bu öğeyi sil ya da değiştir. ${UI_LIST_DESC:-}"); i=$((i + 1))
        done
        labels+=("➕ Ekle…"); acts+=(add)
        UI_PICK_DESCS+=("Yeni öğeyi yazarsınız; tür doğrulanır. ${UI_LIST_ADD_HINT:-}${UI_LIST_DESC:-}")
        if [ -n "${UI_LIST_EXTRAS:-}" ]; then
            "$UI_LIST_EXTRAS"
            i=0
            for c in ${UI_LIST_X_LABELS[@]+"${UI_LIST_X_LABELS[@]}"}; do
                labels+=("$c"); acts+=("${UI_LIST_X_ACTS[$i]}"); UI_PICK_DESCS+=("${UI_LIST_X_DESCS[$i]:-}"); i=$((i + 1))
            done
        fi
        if [ "${#UI_LIST_ITEMS[@]}" -gt 0 ]; then labels+=("🧹 Hepsini temizle"); acts+=(clear); UI_PICK_DESCS+=("Listedeki tüm öğeleri kaldırır (Kaydet'e kadar dosyaya yazılmaz)."); fi
        labels+=("💾 Kaydet (${#UI_LIST_ITEMS[@]} öğe)"); acts+=(save)
        UI_PICK_DESCS+=("Listeyi kaydeder.")
        clear
        if declare -F ui_header >/dev/null; then ui_header "${UI_LIST_TITLE:-Liste} › ${UI_LIST_NAME:-}"; fi
        [ -z "$msg" ] || { echo "$msg"; echo ""; }
        # shellcheck disable=SC2034  # ui_pick_arrows / _ui_pick_fzf (dinamik kapsam) okur
        local UI_PICK_WIDE=1 UI_PICK_EXPECT="" UI_PICK_EXPECT_HINT="" UI_PICK_START=1 UI_PICK_BACK_DESC="Kaydetmeden döner."
        # shellcheck disable=SC2034
        local UI_PICK_HEADER=""
        UI_PICK_HEADER="${msg:+$msg$'\n'}${UI_LIST_META:-}$([ "$UI_LIST_DIRTY" = 1 ] && echo ' · ✏️ kaydedilmedi')"
        msg=""
        pick=$(ui_pick "Vazgeç (kaydetme)" "${labels[@]}")
        if ! [ "$pick" -ge 1 ] 2>/dev/null; then rm -f -- "$tmpm"; return 1; fi
        act="${acts[$((pick - 1))]}"
        case "$act" in
            add)
                IFS= read -r -p "${UI_LIST_PROMPT:-Yeni öğe: }" c || { rm -f -- "$tmpm"; return 2; }
                [ -n "$c" ] || continue
                if out=$("$UI_LIST_VALIDATE" "$c" 2>&1); then
                    if [ -n "${UI_LIST_ADD:-}" ]; then "$UI_LIST_ADD" "$c" > "$tmpm"; msg=$(cat "$tmpm")
                    else
                        x=0; for v in ${UI_LIST_ITEMS[@]+"${UI_LIST_ITEMS[@]}"}; do [ "$v" = "$c" ] && x=1; done
                        if [ "$x" = 1 ]; then msg="Zaten listede: $c"; else UI_LIST_ITEMS+=("$c"); fi
                    fi
                    UI_LIST_DIRTY=1
                else msg="⚠️  $out"; fi ;;
            clear) UI_LIST_ITEMS=(); UI_LIST_DIRTY=1 ;;
            item:*)
                k="${act#item:}"
                # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
                local -a UI_PICK_DESCS=("Bu öğeyi listeden çıkarır." "Öğeyi yeni bir değerle değiştirir (tür doğrulanır).")
                # shellcheck disable=SC2034
                local UI_PICK_WIDE=1 UI_PICK_START=1 UI_PICK_EXPECT="" UI_PICK_HEADER="Öğe: ${UI_LIST_ITEMS[$k]}" UI_PICK_BACK_DESC="Öğeye dokunmadan döner."
                pick=$(ui_pick "Geri" "🗑 Sil" "✏️  Değiştir")
                case "$pick" in
                    1) local -a keep=(); i=0
                       for v in "${UI_LIST_ITEMS[@]}"; do [ "$i" -eq "$k" ] || keep+=("$v"); i=$((i + 1)); done
                       UI_LIST_ITEMS=(); for v in ${keep[@]+"${keep[@]}"}; do UI_LIST_ITEMS+=("$v"); done; UI_LIST_DIRTY=1 ;;
                    2) IFS= read -r -p "Yeni değer (şimdi: ${UI_LIST_ITEMS[$k]}): " c || { rm -f -- "$tmpm"; return 2; }
                       if [ -z "$c" ]; then :
                       elif out=$("$UI_LIST_VALIDATE" "$c" 2>&1); then UI_LIST_ITEMS[k]="$c"; UI_LIST_DIRTY=1
                       else msg="⚠️  $out"; fi ;;
                esac ;;
            save)
                if [ "${#UI_LIST_ITEMS[@]}" -eq 0 ] && [ "${UI_LIST_REQUIRED:-no}" = yes ]; then msg="⚠️  Liste boş olamaz (zorunlu)."; continue; fi
                if [ -n "${UI_LIST_SAVE_CHECK:-}" ] && ! "$UI_LIST_SAVE_CHECK"; then continue; fi
                rm -f -- "$tmpm"; return 0 ;;
            *)
                if [ -n "${UI_LIST_EXTRA_RUN:-}" ]; then UI_LIST_MSG=""; "$UI_LIST_EXTRA_RUN" "$act"; msg="$UI_LIST_MSG"; fi ;;
        esac
    done
}

# ---- Bölgeli ekran: üst bölüm sabit çizilir, alt bölüm terminalin KAYDIRMA BÖLGESİ (DECSTBM) olur ----
# Alt bölgedeki her şey (sorular, parola istemleri, çıktı) kendi içinde kaydırılır; üst bölüm yerinde kalır.
# ui_region_begin <başlık> <gövde-metni> <alt-çerçeve-etiketi> → 0: kuruldu · 1: terminal küçük (<24 satır / <60 sütun) ya da bilinmiyor
# ui_region_end <etiket> → alt çerçevenin başlığını günceller · ui_region_reset → bölgeyi sıfırlar, imleci en alta alır
# Ctrl-C de bölgeyi sıfırlar. Test için SSHPOCKET_ROWS / SSHPOCKET_COLS terminal boyutunu geçersiz kılar.
UI_REGION_ROWS=0
_ui_region_int() { printf '\033[r' >&2; trap - INT; kill -INT $$; }

# ui_rule_label <satır> <etiket> <sütun> → verilen satıra "── etiket ─────" çizer (kalın)
ui_rule_label() {
    local label="$2" cols="$3" n
    n=$((cols - ${#label} - 4)); [ "$n" -gt 0 ] || n=1
    printf '\033[%d;1H\033[2K\033[1m── %s %s\033[0m' "$1" "$label" "$(printf '─%.0s' $(seq 1 "$n"))"
}

ui_region_begin() {
    local title="$1" body="$2" label="$3" rows cols top i=0 line
    rows="${SSHPOCKET_ROWS:-$(tput lines 2>/dev/null)}"; cols="${SSHPOCKET_COLS:-$(tput cols 2>/dev/null)}"
    [[ "$rows" =~ ^[0-9]+$ && "$cols" =~ ^[0-9]+$ ]] && [ "$rows" -ge 24 ] && [ "$cols" -ge 60 ] || return 1
    top=$((rows * 4 / 10)); [ "$top" -ge 11 ] || top=11; [ "$top" -le 16 ] || top=16
    clear
    printf '\033[1;1H\033[2K\033[1m  %s\033[0m' "$title"
    printf '\033[2;1H\033[2K%s' "$(printf '─%.0s' $(seq 1 "$cols"))"
    while IFS= read -r line; do
        i=$((i + 1)); [ "$i" -le $((top - 3)) ] || break
        printf '\033[%d;1H\033[2K  %s' "$((i + 2))" "${line:0:$((cols - 4))}"
    done <<< "$body"
    ui_rule_label "$top" "$label" "$cols"
    UI_REGION_ROWS="$rows"; UI_REGION_COLS="$cols"; UI_REGION_TOP="$top"
    printf '\033[%d;%dr\033[%d;1H' "$((top + 1))" "$rows" "$((top + 1))"
    trap _ui_region_int INT
    return 0
}

# ui_wrap_text <genişlik> <metin> → her satırı sözcük sınırından sarar (devam satırları 3 boşluk girintili; genişliği aşan sözcük kesilir)
ui_wrap_text() {
    local w="$1" text="$2" raw line word out
    while IFS= read -r raw; do
        line=""; out=""
        set -f
        for word in $raw; do
            while [ "${#word}" -gt "$w" ]; do out+="${line:+$line$'\n'}${word:0:$w}"$'\n'; line=""; word="   ${word:$w}"; done
            if [ -n "$line" ] && [ $((${#line} + ${#word} + 1)) -gt "$w" ]; then out+="$line"$'\n'; line="   $word"
            else line="${line:+$line }$word"; fi
        done
        set +f
        printf '%s\n' "$out$line"
    done <<< "$text"
}

# ui_region_begin2 <başlık> <sol-metin> <sağ-metin> <sol-etiket> <sağ-etiket> <alt-etiket> → ui_region_begin gibi, ama üst bölüm
# iki sütunlu (sol ~%45, ortada │, sağda kalan) ve kenarlık etiketli; sütunlar $COLUMNS genişliğinde, yükseklik içeriğe göre
# (en az 9 satır, en çok terminalin %40'ı). Sonrası ui_region_end / ui_region_reset ile aynı. 0: kuruldu · 1: terminal küçük/bilinmiyor
ui_region_begin2() {
    local title="$1" left="$2" right="$3" ll="$4" rl="$5" label="$6" rows cols top lw rw n i lines_l lines_r a b max
    rows="${SSHPOCKET_ROWS:-$(tput lines 2>/dev/null)}"; cols="${SSHPOCKET_COLS:-$(tput cols 2>/dev/null)}"
    [[ "$rows" =~ ^[0-9]+$ && "$cols" =~ ^[0-9]+$ ]] && [ "$rows" -ge 24 ] && [ "$cols" -ge 60 ] || return 1
    lw=$((cols * 45 / 100)); [ "$lw" -ge 28 ] || lw=28
    rw=$((cols - lw - 3))                                           # " │ " ayırıcı
    lines_l=$(ui_wrap_text "$((lw - 2))" "$left"); lines_r=$(ui_wrap_text "$((rw - 2))" "$right")
    a=$(printf '%s\n' "$lines_l" | wc -l | tr -d ' '); b=$(printf '%s\n' "$lines_r" | wc -l | tr -d ' ')
    max=$a; [ "$b" -le "$max" ] || max=$b
    top=$((max + 4)); [ "$top" -ge 9 ] || top=9
    n=$((rows * 4 / 10)); [ "$n" -ge 9 ] || n=9; [ "$top" -le "$n" ] || top="$n"
    clear
    printf '\033[1;1H\033[2K\033[1m  %s\033[0m' "$title"
    printf '\033[2;1H\033[2K%s' "$(printf '─%.0s' $(seq 1 "$cols"))"
    printf '\033[3;1H\033[2K\033[1m  %s\033[0m\033[3;%dH\033[1m│ %s\033[0m' "${ll:0:$((lw - 2))}" "$((lw + 2))" "${rl:0:$((rw - 2))}"
    for ((i = 1; i <= top - 4; i++)); do
        a=$(printf '%s\n' "$lines_l" | sed -n "${i}p"); b=$(printf '%s\n' "$lines_r" | sed -n "${i}p")
        printf '\033[%d;1H\033[2K  %s\033[%d;%dH│ %s' "$((i + 3))" "${a:0:$((lw - 2))}" "$((i + 3))" "$((lw + 2))" "${b:0:$((rw - 2))}"
    done
    ui_rule_label "$top" "$label" "$cols"
    UI_REGION_ROWS="$rows"; UI_REGION_COLS="$cols"; UI_REGION_TOP="$top"
    printf '\033[%d;%dr\033[%d;1H' "$((top + 1))" "$rows" "$((top + 1))"
    trap _ui_region_int INT
    return 0
}

ui_region_end() {
    [ "$UI_REGION_ROWS" -gt 0 ] || return 0
    printf '\0337'; ui_rule_label "$UI_REGION_TOP" "$1" "$UI_REGION_COLS"; printf '\0338'
}

ui_region_reset() {
    [ "$UI_REGION_ROWS" -gt 0 ] || return 0
    printf '\033[r\033[%d;1H' "$UI_REGION_ROWS"
    UI_REGION_ROWS=0; trap - INT
}
