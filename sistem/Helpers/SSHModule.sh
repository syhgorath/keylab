#!/usr/bin/env bash
# SSHModule.sh - Tüm SSH modülleri için ortak çalıştırıcı
# Her modül yalnızca kendi önekini ve klasörünü verir; mantık burada tek yerde durur.
#
# Modül klasörü beklentisi:
#   .env                 -> <ÖNEK>_IP, <ÖNEK>_USER, <ÖNEK>_PORT (gizli bilgi YOK)
#   id_ed25519_<önek>    -> SSH private key (küçük harf önek)
# Passphrase: macOS Keychain (hesap: USBMonitor_<ÖNEK>_Passphrase)

# Sunucu menüsü bağlantı bloğu: terminal renkliyse başlık camgöbeği, etiket sarı (ui_paint tty; NO_COLOR/boruda düz)
_ssh_paint() { if declare -F ui_paint >/dev/null; then ui_paint tty "$1" "$2"; else printf '%s' "$2"; fi; }
_ssh_info_title() { printf '%s\n' "$(_ssh_paint 1\;36 "$1")"; }
_ssh_info_row() { printf '  %s%s%s\n' "$(_ssh_paint 1\;33 "$1")" "$2" "$3"; }

# _ssh_module_cards <önek> <kullanıcı> <ip> <port> <key> <klasör> <paket-var-mı> → fzf'te sunucu menüsü satırlarının kartları (çağıranın UI_PICK_DESCS dizisine ekler; renk ui_paint kart)
_ssh_module_cards() {
    local prefix="$1" user="$2" ip="$3" port="$4" key="$5" dir="$6" has_pkgs="$7" knock=kapalı pass state pkg
    declare -F knock_enabled >/dev/null && knock_enabled "$prefix" "$dir" 2>/dev/null && knock=etkin
    if ssh_has_passphrase "$prefix"; then pass="$(ui_paint kart 1\;32 '✅ Keychain')"; else pass="$(ui_paint kart 2 'yok (ssh gerekirse sorar)')"; fi
    UI_PICK_DESCS+=("$(ui_card_head '🔌 Bağlan'; ui_card_row Komut "ssh -p $port $user@$ip"
        if [ -f "$key" ]; then ui_card_row Anahtar "$(ui_paint kart 1\;32 '✅ var')"; else ui_card_row Anahtar "$(ui_paint kart 1\;31 '❌ bulunamadı')"; fi
        ui_card_row Knock "$([ "$knock" = etkin ] && echo 'önce knock gönderilir' || echo 'kapalı')"; ui_card_row Passphrase "$pass"
        printf '\nYeni bir Terminal penceresinde bağlanır.\n')")
    UI_PICK_DESCS+=("$(ui_card_head '📤 Public key gönder'; ui_card_row Hedef "$user@$ip:$port"
        printf '\nPublic key sunucunun authorized_keys dosyasına eklenir (ssh-copy-id).\nParola yalnızca ssh'"'"'ın kendi isteminde girilir.\n')")
    UI_PICK_DESCS+=("$(ui_card_head '✏️  Bilgiler'; ui_card_row IP "$ip"; ui_card_row Kullanıcı "$user"; ui_card_row Port "$port"; ui_card_row Key "$key"
        printf '\nEnter: değiştir.\n')")
    UI_PICK_DESCS+=("$(ui_card_head '🔑 Keychain'; ui_card_row Durum "$pass"
        printf '\nPassphrase kaydedilir/güncellenir veya silinir;\nkayıttan sonra key açılarak doğrulanır.\n')")
    UI_PICK_DESCS+=("$(ui_card_head '🚪 Port knocking'; ui_card_row Durum "$knock"
        printf '\nProtokol, bekleme ve dizi ayarlanır (dizi Keychain'"'"'de).\n')")
    if [ "$has_pkgs" = "1" ]; then
        state="${SSHPOCKET_STATE_DIR:-$HOME/.local/share/sshpocket}"
        UI_PICK_DESCS+=("$(ui_card_head '🛡️  Paketler'
            if [ -f "$dir/components.log" ]; then
                while IFS= read -r pkg; do [ -n "$pkg" ] && _server_card_pkg "$dir/components.log" "$state" "$pkg" "$(basename "$dir")"; done < <(awk -F'|' 'NF >= 5 && !seen[$2]++ { print $2 }' "$dir/components.log")
            else printf ' Bu sunucuda henüz paket eylemi çalıştırılmadı.\n'; fi
            printf '\nÖnizleme, uygulama, durum ve denetim eylemleri;\nher biri onay ve ön kontrollerden geçer.\n')")
    fi
}

# .env dosyasından tek bir anahtarı güvenli okur (eval/source kullanmaz)
# Parametreler: $1 - dosya, $2 - anahtar adı
_ssh_env_get() {
    local file="$1" want="$2" line key value
    while IFS= read -r line || [ -n "$line" ]; do
        local trimmed="${line#"${line%%[![:space:]]*}"}"
        case "$trimmed" in ''|\#*) continue ;; esac
        case "$line" in *=*) ;; *) continue ;; esac
        key="${line%%=*}"
        value="${line#*=}"
        key="${key#"${key%%[![:space:]]*}"}"; key="${key%"${key##*[![:space:]]}"}"
        value="${value#"${value%%[![:space:]]*}"}"; value="${value%"${value##*[![:space:]]}"}"
        # Çevreleyen tırnakları kaldır
        case "$value" in
            \"*\") value="${value#\"}"; value="${value%\"}" ;;
            \'*\') value="${value#\'}"; value="${value%\'}" ;;
            *) value="${value%%[[:space:]]#*}"   # tırnaksız değerde satır sonu yorumunu at
               value="${value%"${value##*[![:space:]]}"}" ;;
        esac
        if [ "$key" = "$want" ]; then
            printf '%s' "$value"
            return 0
        fi
    done < "$file"
    return 1
}

# Port açık mı? $1 ip, $2 port, $3 en çok saniye.
# macOS'ta `nc -w` bağlantı beklemesini sınırlamaz (kapalı/erişilemeyen sunucuda ~75 sn sürebilir);
# bu yüzden nc arka planda çalıştırılır ve süre dolunca öldürülür.
ssh_port_open() {
    local ip="$1" port="$2" secs="${3:-1}" pid killer rc
    command -v nc >/dev/null 2>&1 || return 2
    nc -z "$ip" "$port" >/dev/null 2>&1 &
    pid=$!
    ( sleep "$secs"; kill "$pid" 2>/dev/null ) >/dev/null 2>&1 &
    killer=$!
    wait "$pid" 2>/dev/null
    rc=$?
    kill "$killer" 2>/dev/null
    wait "$killer" 2>/dev/null
    return "$rc"
}

# Port kontrolü yapıp bağlanır
# Parametreler: $1 - önek, $2 - kullanıcı, $3 - ip, $4 - port, $5 - key yolu
_ssh_module_connect() {
    local prefix="$1" user="$2" ip="$3" port="$4" ssh_key="$5"
    echo ""
    echo "🔍 Port kontrolü..."
    if command -v nc >/dev/null 2>&1; then
        if ssh_port_open "$ip" "$port" 5; then
            echo "✅ Port $port açık"
        else
            echo "⚠️  Port $port kapalı veya erişilemiyor"
            local answer
            read -r -p "Yine de bağlanılsın mı? (y/n) " answer
            case "$answer" in y|Y) ;; *) echo "İptal edildi."; return 1 ;; esac
        fi
    else
        echo "ℹ️  'nc' yok, port kontrolü atlandı"
    fi
    echo ""

    Log "INFO" "$prefix SSH bağlantısı: ${user}@${ip}:${port}"
    ssh_connect "$user" "$ip" "$port" "$ssh_key"
}

# Modül menüsü: bağlan / key gönder / bilgileri güncelle / Keychain
# Ayar bozuk veya key eksik olsa bile güncelleme seçenekleri kullanılabilir.
# Parametreler:
#   $1 - öneki (büyük harf, örn: RPI)
#   $2 - modül klasörü
ssh_module_run() {
    local prefix="$1"
    local module_dir="$2"
    local lower env_file default_key
    lower=$(printf '%s' "$prefix" | tr '[:upper:]' '[:lower:]')
    env_file=$(srv_env_file "$module_dir")
    default_key="$module_dir/id_ed25519_${lower}"

    while true; do
        local ip="" user="" port="22" ssh_key="$default_key" custom_key problem="" verr choice has_pkgs=0
        if declare -F comp_menu_available >/dev/null && comp_menu_available "$module_dir"; then has_pkgs=1; fi

        clear
        if declare -F ui_header >/dev/null; then
            ui_header "Ana menü › $prefix"
        else
            echo "====================================="
            echo "$prefix SSH Bağlantı"
            echo "====================================="
        fi
        echo ""

        if [ ! -f "$env_file" ]; then
            problem="Ayar dosyası bulunamadı. 3) ile bilgileri girin."
        else
            ip=$(srv_get "$module_dir" "$prefix" IP)
            user=$(srv_get "$module_dir" "$prefix" USER)
            port=$(srv_get "$module_dir" "$prefix" PORT)
            port="${port:-22}"
            custom_key=$(srv_get "$module_dir" "$prefix" KEY)
            if [ -n "$custom_key" ]; then
                if [ "${custom_key:0:1}" = "~" ]; then custom_key="$HOME${custom_key:1}"; fi
                ssh_key="$custom_key"
            fi
            if [ -z "$ip" ] || [ -z "$user" ]; then
                problem="Ayar dosyasında IP veya kullanıcı eksik. 3) ile tamamlayın."
            elif ! verr=$(ssh_validate_target "$ip" "$user" "$port" 2>&1); then
                problem="Ayar dosyası: $verr. 3) ile düzeltin."
            elif [ ! -f "$ssh_key" ]; then
                problem="SSH key bulunamadı: $ssh_key (3 ile key yolunu düzeltin)"
            fi
        fi

        _ssh_info_title "📡 Bağlantı Bilgileri:"
        _ssh_info_row "IP:" "   " "${ip:-(yok)}"
        _ssh_info_row "User:" " " "${user:-(yok)}"
        _ssh_info_row "Port:" " " "$port"
        _ssh_info_row "Key:" "  " "$ssh_key"
        if ssh_has_passphrase "$prefix"; then
            _ssh_info_row "Passphrase:" " " "$(_ssh_paint 1\;32 '✅ Keychain')"
        else
            _ssh_info_row "Passphrase:" " " "$(_ssh_paint 2 "ℹ️  Keychain'de yok (ssh gerekirse soracak)")"
        fi
        if [ -n "$problem" ]; then
            echo ""
            echo "⚠️  $problem"
        else
            chmod 600 "$ssh_key" 2>/dev/null
            if ssh_has_passphrase "$prefix"; then ssh_agent_add_retry "$prefix" "$ssh_key"; fi
        fi
        echo ""

        if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled; then
            # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
            local -a UI_PICK_DESCS=(
                "Yeni bir Terminal penceresinde ssh ile bağlanır (varsa önce knock gönderilir; passphrase Keychain'den)."
                "Public key'i sunucunun authorized_keys dosyasına ekler (ssh-copy-id); parola yalnızca ssh'ın kendi isteminde girilir."
                "IP, kullanıcı, port ve key yolunu değiştirir."
                "Key passphrase'ini Keychain'e kaydeder/günceller veya siler; kayıttan sonra key'i açarak doğrular."
                "Port knocking: protokol, bekleme ve diziyi ayarlar (dizi Keychain'de saklanır)."
            )
            # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
            local UI_PICK_BACK_DESC="Ana menüye döner."
            local -a pick_items=("Bağlan" "Public key'i sunucuya gönder" "Bilgileri güncelle (IP / kullanıcı / port / key yolu)" \
                "Keychain passphrase'ini güncelle / sil" "Knock ayarları (port knocking)")
            if [ "$has_pkgs" = "1" ]; then
                pick_items+=("🛡️  Paketler (sertleştirme vb.)")
                UI_PICK_DESCS+=("Kurulu paketlerin eylemlerini bu sunucuda çalıştırır (önizleme, uygulama, durum...); her eylem onay ve ön kontrollerden geçer.")
            fi
            if declare -F ui_card_on >/dev/null && ui_card_on; then
                # fzf: satır başına renkli kart, geniş düzen (ok tuşlu/numaralı modda yukarıdaki tek satırlık açıklamalar kalır)
                UI_PICK_DESCS=()
                _ssh_module_cards "$prefix" "$user" "$ip" "$port" "$ssh_key" "$module_dir" "$has_pkgs"
                # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
                local UI_PICK_WIDE=1 UI_PICK_TITLE="" UI_PICK_HEIGHT=20
                # shellcheck disable=SC2034
                UI_PICK_TITLE="Ana menü › $(basename "$module_dir")"
            fi
            choice=$(ui_pick_arrows 1 "Geri" "${pick_items[@]}")
            if [ "$choice" = "0" ]; then ui_no_pause; return 0; fi
        else
            echo "  1) Bağlan"
            echo "  2) Public key'i sunucuya gönder"
            echo "  3) Bilgileri güncelle (IP / kullanıcı / port / key yolu)"
            echo "  4) Keychain passphrase'ini güncelle / sil"
            echo "  5) Knock ayarları (port knocking)"
            if [ "$has_pkgs" = "1" ]; then echo "  6) 🛡️  Paketler (sertleştirme vb.)"; fi
            echo "  0) Geri (q)"
            if declare -F ui_footer >/dev/null; then ui_footer "numara yaz + Enter (boş = 1 Bağlan) · 0 veya q = geri"; fi
            if ! read -r -p "Seçim [1]: " choice; then return 0; fi
        fi
        case "${choice:-1}" in
            1|2)
                if [ -n "$problem" ]; then
                    echo "⚠️  Önce sorunu giderin (3 veya 4)."
                    read -r -p "Devam için Enter..." _ || return 0
                elif declare -F knock_pre_connect >/dev/null && ! knock_pre_connect "$prefix" "$module_dir" "$ip"; then
                    read -r -p "Devam için Enter..." _ || return 0
                elif [ "${choice:-1}" = "1" ]; then
                    _ssh_module_connect "$prefix" "$user" "$ip" "$port" "$ssh_key"; return $?
                else
                    ssh_copy_key "$user" "$ip" "$port" "$ssh_key"; return $?
                fi
                ;;
            3) ssh_module_edit "$prefix" "$module_dir"; read -r -p "Devam için Enter..." _ || return 0 ;;
            4) ssh_keychain_manage "$prefix" "$ssh_key"; read -r -p "Devam için Enter..." _ || return 0 ;;
            5) knock_manage "$prefix" "$module_dir"; read -r -p "Devam için Enter..." _ || return 0 ;;
            6)
                if [ "$has_pkgs" = "1" ]; then comp_menu_server "$prefix" "$module_dir"; else ui_no_pause; return 0; fi
                ;;
            *) ui_no_pause; return 0 ;;
        esac
    done
}

# Menüde gösterilecek durum simgesi: 🟢 port açık, 🔴 kapalı/erişilemiyor, ⚪ ayar yok
# Parametreler: $1 - öneki, $2 - modül klasörü
ssh_module_status() {
    local prefix="$1" module_dir="$2"
    local env_file ip port
    env_file=$(srv_env_file "$module_dir")
    if [ ! -f "$env_file" ]; then printf '⚪'; return 0; fi
    ip=$(srv_get "$module_dir" "$prefix" IP)
    port=$(srv_get "$module_dir" "$prefix" PORT)
    port="${port:-22}"
    if [ -z "$ip" ] || ! ssh_validate_target "$ip" "x" "$port" 2>/dev/null; then
        printf '⚪'; return 0
    fi
    local tmo=1
    if declare -F settings_get >/dev/null; then tmo=$(settings_get STATUS_TIMEOUT); fi
    if ssh_port_open "$ip" "$port" "${tmo:-1}"; then
        printf '🟢'
    elif declare -F knock_enabled >/dev/null && knock_enabled "$prefix" "$module_dir"; then
        printf '🔒'      # knock gerekir; durum kontrolü knock GÖNDERMEZ
    else
        printf '🔴'
    fi
}
