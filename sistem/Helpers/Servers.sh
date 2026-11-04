#!/usr/bin/env bash
# Servers.sh - Sunucu veri katmanı
#
# Yeni biçim:  servers/<ad>/server.env   (IP, USER, PORT, KEY, OS)   + key dosyaları
# Eski biçim:  sistem/Modules/<ad>/.env  (<ÖNEK>_IP, <ÖNEK>_USER, ...) + üretilmiş <Ad>Module.sh
# Her iki biçim okunur; yazarken bulunan biçim korunur. Taşıma (migration) ayrı bir adımdır.
#
# GÜVENLİK: servers/ altından hiçbir dosya `source` edilmez; yalnızca server.env güvenli ayrıştırıcıyla okunur.
# Gerektirir: Validate.sh (module_name_valid), ModuleFiles.sh (module_prefix), SSHModule.sh (_ssh_env_get),
#             SSHModuleEdit.sh (_ssh_env_set), RegisterModules.sh

# servers/ klasörü: SSHPOCKET_SERVERS_DIR ile ya da uygulama kökünün yanında
servers_dir() {
    if [ -n "${SSHPOCKET_SERVERS_DIR:-}" ]; then
        printf '%s' "$SSHPOCKET_SERVERS_DIR"
        return 0
    fi
    local root
    root=$(cd "${SISTEM_PATH:?SISTEM_PATH tanımlı değil}/.." && pwd)
    printf '%s' "$root/servers"
}

# Ayar dosyasının yolu (yeni biçim varsa server.env, yoksa eski .env)
# Parametre: $1 - sunucu klasörü
srv_env_file() {
    if [ -f "$1/server.env" ]; then
        printf '%s' "$1/server.env"
    else
        printf '%s' "$1/.env"
    fi
}

# Yeni biçim mi?
srv_is_current() {
    [ -f "$1/server.env" ]
}

# Biçime göre anahtar adı: yeni → IP, eski → <ÖNEK>_IP
# Parametreler: $1 - önek (büyük harf), $2 - anahtar (IP|USER|PORT|KEY|OS), $3 - sunucu klasörü
srv_key_name() {
    if srv_is_current "$3"; then
        printf '%s' "$2"
    else
        printf '%s_%s' "$1" "$2"
    fi
}

# Değer oku. Parametreler: $1 - klasör, $2 - önek, $3 - anahtar
srv_get() {
    local f
    f=$(srv_env_file "$1")
    [ -f "$f" ] || return 1
    _ssh_env_get "$f" "$(srv_key_name "$2" "$3" "$1")"
}

# Değer yaz. Parametreler: $1 - klasör, $2 - önek, $3 - anahtar, $4 - değer
srv_set() {
    local f
    f=$(srv_env_file "$1")
    _ssh_env_set "$f" "$(srv_key_name "$2" "$3" "$1")" "$4"
}

# servers/ altındaki geçerli sunucu adları (server.env'i olan, adı kurala uyan; gizli klasörler hariç)
srv_names() {
    local d p n
    d=$(servers_dir)
    [ -d "$d" ] || return 0
    for p in "$d"/*/; do
        [ -d "$p" ] || continue
        n=$(basename "$p")
        module_name_valid "$n" || continue
        [ -f "${p}server.env" ] || continue
        printf '%s\n' "$n"
    done
}

# servers/ içindekileri ana menüye kaydeder (eski modüllerden ÖNCE çağrılır)
RegisterServers() {
    local n
    while IFS= read -r n; do
        [ -n "$n" ] || continue
        RegisterModules "$(module_prefix "$n")" "MainMenu" "server:$n"
    done < <(srv_names)
}

# Menü girdisi çalıştırılabilir mi? ("server:<ad>" ya da fonksiyon adı)
EntryExists() {
    case "$1" in
        server:*) module_name_valid "${1#server:}" ;;
        *)        declare -F "$1" >/dev/null 2>&1 ;;
    esac
}

# Menü girdisini çalıştırır
RunEntry() {
    case "$1" in
        server:*) server_run "${1#server:}" ;;
        *)        "$1" ;;
    esac
}

# Sunucu menüsü
server_run() {
    local n="$1"
    module_name_valid "$n" || { echo "⚠️  Geçersiz sunucu adı: $n" >&2; return 1; }
    ssh_module_run "$(module_prefix "$n")" "$(servers_dir)/$n"
}

# Durum simgesi (Menu.sh paralel çağırır)
server_status() {
    local n="$1"
    module_name_valid "$n" || { printf '⚪'; return 0; }
    ssh_module_status "$(module_prefix "$n")" "$(servers_dir)/$n"
}

# Menü satırı için kısa özet: kullanıcı@ip[:port] (bilgi eksik/geçersizse boş)
server_summary() {
    local n="$1" d ip user port
    module_name_valid "$n" || return 0
    d="$(servers_dir)/$n"
    ip=$(srv_get "$d" "$(module_prefix "$n")" IP) || return 0
    user=$(srv_get "$d" "$(module_prefix "$n")" USER)
    port=$(srv_get "$d" "$(module_prefix "$n")" PORT)
    [ -n "$ip" ] && [ -n "$user" ] || return 0
    ssh_validate_target "$ip" "$user" "${port:-22}" 2>/dev/null || return 0
    if [ -n "$port" ] && [ "$port" != "22" ]; then
        printf '%s@%s:%s' "$user" "$ip" "$port"
    else
        printf '%s@%s' "$user" "$ip"
    fi
}

# Ana menüde sunucu satırının kartı (fzf sağ bölmesi; ok tuşlu seçicide satırlar " · " ile birleşir). YALNIZCA yerel veri okur: ssh yok.
# server_card <ad> [durum-simgesi] → çok satırlı metin; fzf etkinse renkli (bkz. ui_paint kart). Durum simgesi Menu.sh'ın zaten yaptığı yoklamadan gelir.
server_card() {
    local n="$1" mark="${2:-}" d prefix ip user port key knock state pkg st
    module_name_valid "$n" || return 0
    d="$(servers_dir)/$n"; prefix=$(module_prefix "$n")
    ip=$(srv_get "$d" "$prefix" IP) || return 0
    user=$(srv_get "$d" "$prefix" USER); port=$(srv_get "$d" "$prefix" PORT); port="${port:-22}"
    key=$(srv_get "$d" "$prefix" KEY)
    [ -n "$key" ] || key="$d/id_ed25519_${n}"
    if [ "${key:0:1}" = "~" ]; then key="$HOME${key:1}"; fi
    ui_card_head "🖥️  SUNUCU"
    case "$mark" in
        🟢) st="$(ui_paint kart 1\;32 '🟢 açık')" ;;
        🔴) st="$(ui_paint kart 1\;31 '🔴 erişilemiyor')" ;;
        🔒) st="$(ui_paint kart 1\;33 '🔒 knock gerekir')" ;;
        ⚪) st="$(ui_paint kart 1\;33 '⚪ ayar eksik')" ;;
        *)  st="" ;;
    esac
    [ -z "$st" ] || ui_card_row "Durum" "$st"
    ui_card_row "Hedef" "$user@$ip:$port"
    knock=kapalı
    if declare -F knock_enabled >/dev/null && knock_enabled "$prefix" "$d" 2>/dev/null; then knock=etkin; fi
    if [ -f "$key" ]; then ui_card_row "Anahtar" "$(ui_paint kart 1\;32 '✅ var')"; else ui_card_row "Anahtar" "$(ui_paint kart 1\;31 '❌ bulunamadı')"; fi
    ui_card_row "Knock" "$knock"
    state="${SSHPOCKET_STATE_DIR:-$HOME/.local/share/sshpocket}"
    if [ -f "$d/components.log" ]; then
        while IFS= read -r pkg; do
            [ -n "$pkg" ] || continue
            _server_card_pkg "$d/components.log" "$state" "$pkg" "$n"
        done < <(awk -F'|' 'NF >= 5 && !seen[$2]++ { print $2 }' "$d/components.log")
    fi
    printf '\n%s\n' "$(ui_paint kart 2 "\$ ssh -p $port $user@$ip")"
}

# _server_card_pkg <components.log> <durum-dizini> <paket> <sunucu> → "📦 paket sürüm" bölümü: son preview/apply/audit (ayrı satırlar) + son Lynis skoru
_server_card_pkg() {
    local log="$1" state="$2" pkg="$3" n="$4" ver sum score act row
    ver=$(awk -F'|' -v p="$pkg" '$2 == p && NF >= 5 { v = $3 } END { print v }' "$log")
    [ -n "$ver" ] || return 0
    printf '\n'; ui_card_head "📦 $pkg $ver"
    for act in preview apply audit; do
        row=$(awk -F'|' -v p="$pkg" -v a="$act" '$2 == p && $4 == a && NF >= 5 { t = substr($1, 6, 5) " " substr($1, 12, 5); r = $5 } END { if (t != "") print t "|" r }' "$log")
        [ -n "$row" ] || continue
        if [ "${row#*|}" = ok ]; then ui_card_row "$act" "${row%|*} $(ui_paint kart 1\;32 '✅')"; else ui_card_row "$act" "${row%|*} $(ui_paint kart 1\;31 '❌')"; fi
    done
    sum=$(find "$state/$pkg/$n/out/$n" -mindepth 2 -maxdepth 2 -name summary.json 2>/dev/null | sort | tail -n 1)
    if [ -n "$sum" ]; then
        score=$(sed -n 's/.*"hardening_index": *\([0-9][0-9]*\).*/\1/p' "$sum" | head -n 1)
        [ -z "$score" ] || ui_card_row "Lynis" "$score (son denetim)"
    fi
}
