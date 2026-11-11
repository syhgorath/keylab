#!/usr/bin/env bash
# ComponentsVars.sh - Paket değişkenlerini menüden yönetmek için kütüphane (Aşama 3, PR E2a; menü yok, VM'e dokunmaz*)
# *comp_net_seen_by_server isteğe bağlı olarak sabit, salt okunur bir ssh çalıştırır; yalnızca çağrılırsa.
# Değerler servers/<ad>/<paket>.vars içinde düz "ad=değer" satırlarıdır (gizli bilgi yok); hiçbir şey source/eval edilmez.
# Doğrulama ComponentsRun.sh/component_run.py ile AYNIDIR: menüden geçen değer çalıştırmada reddedilmez.
# Gerektirir: ComponentsRun.sh (COMP_RUN_PY)

COMP_RUN_PY="${COMP_RUN_PY:-${SISTEM_PATH:-.}/Helpers/component_run.py}"

_comp_vars_py() { python3 -I "$COMP_RUN_PY" "$@"; }

# comp_vars_file <sunucu-klasörü> <paket> → değerler dosyasının yolu
comp_vars_file() { printf '%s/%s.vars' "$1" "$2"; }

# comp_vars_describe <component.json> [<vars-dosyası>] → (vars-dosyası verilirse platforma göre süzülmüş) değişken başına TSV: ad, tür, zorunlu, varsayılan, min, max, warn_if, açıklama
comp_vars_describe() { _comp_vars_py describe "$1" ${2:+"$2"}; }

# comp_vars_platform <component.json> [<vars-dosyası>] → platform_var'ın geçerli değeri (belirlenemezse boş)
comp_vars_platform() { _comp_vars_py platform "$1" ${2:+"$2"}; }

# comp_vars_check <component.json> <ad> <değer> → 0 geçerli ("warn=<ad>" yazılabilir) · 1 geçersiz (neden stderr'de)
comp_vars_check() { _comp_vars_py check "$1" "$2" "$3"; }

_comp_vars_name_ok() { [[ "$1" =~ ^[a-z][a-z0-9_]{0,63}$ ]]; }

# comp_vars_get <dosya> <ad> → değer (yoksa boş, çıkış 1)
comp_vars_get() {
    local f="$1" n="$2" v
    _comp_vars_name_ok "$n" || return 1
    [ -f "$f" ] || return 1
    v=$(awk -v k="$n" '{ s=$0; sub(/^[ \t]+/, "", s) } s ~ /^#/ { next } index(s, k "=") == 1 { print substr(s, length(k) + 2); found=1; exit } END { exit !found }' "$f") || return 1
    printf '%s' "$v" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'
}

# comp_vars_set <dosya> <ad> <değer> → satırı değiştirir/ekler (diğer satırlar ve yorumlar korunur); izin 600, atomik yazım
comp_vars_set() {
    local f="$1" n="$2" v="$3" tmp
    _comp_vars_name_ok "$n" || { echo "geçersiz değişken adı" >&2; return 1; }
    case "$v" in *$'\n'*|*$'\r'*|*$'\t'*) echo "değer satır sonu/sekme içeremez" >&2; return 1 ;; esac
    [[ "$v" =~ [[:cntrl:]] ]] && { echo "değer denetim karakteri içeremez" >&2; return 1; }
    tmp=$(mktemp "${f}.XXXXXX") || { echo "geçici dosya oluşturulamadı" >&2; return 1; }
    chmod 600 "$tmp"
    if [ -f "$f" ]; then
        awk -v k="$n" -v val="$v" 'BEGIN { done=0 } { s=$0; sub(/^[ \t]+/, "", s) } s !~ /^#/ && index(s, k "=") == 1 { if (!done) { print k "=" val; done=1 } next } { print } END { if (!done) print k "=" val }' "$f" > "$tmp" || { rm -f -- "$tmp"; return 1; }
    else
        printf '%s=%s\n' "$n" "$v" > "$tmp"
    fi
    mv -f -- "$tmp" "$f" || { rm -f -- "$tmp"; return 1; }
    chmod 600 "$f" 2>/dev/null || true
}

# comp_vars_unset <dosya> <ad> → satırı siler (varsayılana döner)
comp_vars_unset() {
    local f="$1" n="$2" tmp
    _comp_vars_name_ok "$n" || return 1
    [ -f "$f" ] || return 0
    tmp=$(mktemp "${f}.XXXXXX") || return 1
    chmod 600 "$tmp"
    awk -v k="$n" '{ s=$0; sub(/^[ \t]+/, "", s) } s !~ /^#/ && index(s, k "=") == 1 { next } { print }' "$f" > "$tmp" || { rm -f -- "$tmp"; return 1; }
    mv -f -- "$tmp" "$f" || { rm -f -- "$tmp"; return 1; }
    chmod 600 "$f" 2>/dev/null || true
}

# comp_vars_save <component.json> <dosya> <ad> <değer> → önce DOĞRULAR, geçerliyse yazar. Çıkış 0 yazıldı · 1 geçersiz/yazılamadı.
# stdout: doğrulayıcının "warn=<ad>" satırı (varsa) — çağıran ayrıca onay isteyebilir.
comp_vars_save() {
    local mf="$1" f="$2" n="$3" v="$4" out
    out=$(comp_vars_check "$mf" "$n" "$v") || return 1
    comp_vars_set "$f" "$n" "$v" || return 1
    [ -z "$out" ] || printf '%s\n' "$out"
}

# comp_vars_missing <component.json> <dosya> → değeri de varsayılanı da olmayan ZORUNLU değişken adları (satır satır)
comp_vars_missing() {
    local mf="$1" f="$2" line name rest req dflt cur
    while IFS= read -r line; do
        name="${line%%$'\t'*}"; rest="${line#*$'\t'}"      # ad | tür ...
        rest="${rest#*$'\t'}"; req="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"; dflt="${rest%%$'\t'*}"
        [ "$req" = "yes" ] || continue
        [ -z "$dflt" ] || continue
        cur=$(comp_vars_get "$f" "$name" 2>/dev/null) || cur=""
        [ -n "$cur" ] || printf '%s\n' "$name"
    done < <(comp_vars_describe "$mf")
    return 0
}

# ---------- erişim ağı önerisi (yalnızca öneri: hiçbir şey otomatik yazılmaz) ----------

# comp_net_local <hedef-ip> → bu Mac'in hedefe giderken kullandığı arayüzün "adres/önek" değeri (BSD route/ifconfig)
comp_net_local() {
    local host="$1" iface line addr mask
    [[ "$host" =~ ^[A-Za-z0-9._:-]+$ ]] || return 1
    iface=$(route -n get "$host" 2>/dev/null | awk '/interface:/ { print $2; exit }')
    [[ "$iface" =~ ^[A-Za-z][A-Za-z0-9]*$ ]] || return 1
    line=$(ifconfig "$iface" 2>/dev/null | awk '/^[[:space:]]+inet / { a=$2; m=""; for (i = 1; i <= NF; i++) if ($i == "netmask") m=$(i + 1); print a " " m; exit }')
    addr="${line%% *}"; mask="${line##* }"
    [ -n "$addr" ] && [ -n "$mask" ] && [ "$addr" != "$mask" ] || return 1
    _comp_vars_py net-prefix "$addr" "$mask"
}

# comp_net_seen_by_server <ip> <port> <kullanıcı> <anahtar> → sunucunun bağlantıyı gördüğü adres (SABİT, salt okunur ssh; host key kapatılmaz)
comp_net_seen_by_server() {
    local host="$1" port="$2" user="$3" key="$4" out
    out=$(ssh -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 \
              -o PreferredAuthentications=publickey -o PasswordAuthentication=no \
              -p "$port" -i "$key" "$user@$host" 'echo $SSH_CONNECTION' 2>/dev/null) || return 1
    out="${out%% *}"
    _comp_vars_py net-valid "$out"
}

# comp_net_suggest <adres/önek | adres> → "CIDR<TAB>açıklama" satırları (ağ, tahmini /24, yalnızca bu adres)
comp_net_suggest() {
    local a="$1" addr plen
    addr="${a%%/*}"
    if [[ "$a" == */* ]]; then plen="${a##*/}"; else
        case "$addr" in *:*) plen=128 ;; *) plen=32 ;; esac
    fi
    [[ "$plen" =~ ^[0-9]{1,3}$ ]] || return 1
    _comp_vars_py net-suggest "$addr" "$plen"
}

# comp_net_add <virgüllü-liste> <yeni-cidr> → 1. satır durum (added | duplicate | covered:<kapsayan> | replaced:<kaldırılanlar>), 2. satır sonuç listesi
# Yeni ağ mevcut bir girdinin İÇİNDEYSE eklenmez; mevcut girdileri KAPSIYORSA onların yerine geçer.
comp_net_add() { _comp_vars_py net-add "$1" "$2"; }
