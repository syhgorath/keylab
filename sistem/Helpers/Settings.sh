#!/usr/bin/env bash
# Settings.sh - Küçük ayar katmanı: sshpocket.conf (ANAHTAR=değer)
# Öncelik: ortam değişkeni (SSHPOCKET_<ANAHTAR>) > dosya > varsayılan.
# Dosya ASLA source edilmez; yalnızca bilinen anahtarlar okunur, değerler doğrulanır.
# Gerektirir: (SISTEM_PATH ya da SSHPOCKET_CONF)

settings_file() {
    if [ -n "${SSHPOCKET_CONF:-}" ]; then
        printf '%s' "$SSHPOCKET_CONF"
    else
        printf '%s' "${SISTEM_PATH:-.}/../sshpocket.conf"
    fi
}

settings_default() {
    case "$1" in
        STATUS) printf 'on' ;;
        STATUS_TIMEOUT) printf '1' ;;
        UI) printf 'auto' ;;
        *) return 1 ;;
    esac
}

# Değeri normalleştirir (STATUS: 1/true→on, 0/false→off); geçersizse boş yazar, 1 döner
settings_normalize() {
    local key="$1" val="$2"
    case "$key" in
        STATUS)
            case "$val" in
                on|1|true|yes) printf 'on' ;;
                off|0|false|no) printf 'off' ;;
                *) return 1 ;;
            esac ;;
        STATUS_TIMEOUT)
            case "$val" in
                [1-5]) printf '%s' "$val" ;;
                *) return 1 ;;
            esac ;;
        UI)
            case "$val" in
                auto|arrows|numbers|fzf) printf '%s' "$val" ;;
                *) return 1 ;;
            esac ;;
        *) return 1 ;;
    esac
}

# IFS'ten bağımsız (çağıran bağlamda IFS değişmiş olabilir)
settings_known() {
    case "$1" in
        STATUS|STATUS_TIMEOUT|UI) return 0 ;;
        *) return 1 ;;
    esac
}

# Dosyadaki ham değer (son tanım kazanır); yoksa boş
_settings_file_raw() {
    local f
    f=$(settings_file)
    [ -f "$f" ] || return 0
    grep -E "^$1=" "$f" 2>/dev/null | tail -n 1 | cut -d= -f2- | tr -d '\r' \
        | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

_settings_env_raw() {
    local name="SSHPOCKET_$1"
    printf '%s' "${!name:-}"
}

# Geçerli değer
settings_get() {
    local key="$1" raw val
    settings_known "$key" || return 1
    raw=$(_settings_env_raw "$key")
    if [ -n "$raw" ] && val=$(settings_normalize "$key" "$raw"); then printf '%s' "$val"; return 0; fi
    raw=$(_settings_file_raw "$key")
    if [ -n "$raw" ] && val=$(settings_normalize "$key" "$raw"); then printf '%s' "$val"; return 0; fi
    settings_default "$key"
}

# Değerin kaynağı: env | file | default
settings_source() {
    local key="$1" raw
    raw=$(_settings_env_raw "$key")
    if [ -n "$raw" ] && settings_normalize "$key" "$raw" >/dev/null; then echo env; return 0; fi
    raw=$(_settings_file_raw "$key")
    if [ -n "$raw" ] && settings_normalize "$key" "$raw" >/dev/null; then echo file; return 0; fi
    echo default
}

# Dosyaya yazar (diğer satırlar korunur); geçersiz değer/anahtar reddedilir
settings_set() {
    local key="$1" val f tmp line done_=0
    settings_known "$key" || { echo "Bilinmeyen ayar: $key" >&2; return 1; }
    val=$(settings_normalize "$key" "$2") || { echo "Geçersiz değer: $2" >&2; return 1; }
    f=$(settings_file)
    tmp=$(mktemp "${f}.XXXXXX" 2>/dev/null) || { echo "Ayar dosyası yazılamıyor: $f" >&2; return 1; }
    if [ -f "$f" ]; then
        while IFS= read -r line || [ -n "$line" ]; do
            case "$line" in
                "$key="*) if [ "$done_" -eq 0 ]; then printf '%s=%s\n' "$key" "$val" >> "$tmp"; done_=1; fi ;;
                *) printf '%s\n' "$line" >> "$tmp" ;;
            esac
        done < "$f"
    else
        printf '# sshpocket ayarları (gizli bilgi YAZMAYIN)\n' > "$tmp"
    fi
    if [ "$done_" -eq 0 ]; then printf '%s=%s\n' "$key" "$val" >> "$tmp"; fi
    chmod 644 "$tmp" 2>/dev/null
    mv -f "$tmp" "$f" || { rm -f "$tmp"; echo "Ayar dosyası yazılamıyor: $f" >&2; return 1; }
}

_settings_label() {
    case "$1" in
        STATUS) printf 'Durum göstergesi (🟢/🔴)' ;;
        STATUS_TIMEOUT) printf 'Durum kontrol süresi (sn)' ;;
        UI) printf 'Menü: auto/fzf/arrows/numbers' ;;
    esac
}

_settings_note() {
    case "$(settings_source "$1")" in
        env) printf '  (ortam değişkeni SSHPOCKET_%s geçerli; dosya yok sayılır)' "$1" ;;
        default) printf '  (varsayılan)' ;;
    esac
}

# _settings_card <anahtar> <açıklama> → fzf'te ayar satırının kartı: değer, kaynak (ortam/dosya/varsayılan) ve varsayılan; renk ui_paint kart
_settings_card() {
    local key="$1" kaynak
    case "$(settings_source "$key")" in
        env) kaynak="ortam değişkeni SSHPOCKET_$key (dosya yok sayılır)" ;;
        default) kaynak="varsayılan" ;;
        *) kaynak="sshpocket.conf" ;;
    esac
    ui_card_head "$(_settings_label "$key")"
    ui_card_row Değer "$(settings_get "$key")"
    ui_card_row Kaynak "$kaynak"
    ui_card_row Varsayılan "$(settings_default "$key")"
    printf '\n%s\n' "$2"
}

# Menü eylemi: ⚙️ Ayarlar
# Eski paket sürümlerini listeler ve silinmesini sorar (güncelleme sonrası soru çıkmadıysa ya da silinemediyse buradan yapılır)
_settings_prune_packages() {
    clear
    echo "🧹 Eski paket sürümleri"
    echo ""
    bash "$SISTEM_PATH/Main.sh" --components prune || true
    echo ""
    read -r -p "Devam için Enter..." _ || true
}

SettingsAction() {
    local choice cur new
    while true; do
        clear
        if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled; then
            ui_screen "Ana menü › Ayarlar"
            if ! { declare -F ui_card_on >/dev/null && ui_card_on; }; then echo ""; fi
            # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
            local -a UI_PICK_DESCS=(
                "Menüde sunucuların yanında 🟢 açık / 🔴 erişilemiyor / 🔒 knock gerekir / ⚪ ayar yok işaretini gösterir. Enter: aç/kapat."
                "Durum kontrolünün bir sunucu için en çok kaç saniye bekleyeceği (1-5). Enter: değiştir."
                "Menü stili: auto (fzf varsa fzf, yoksa ok tuşları) · fzf · arrows (ok tuşları) · numbers (numaralı). Enter: sıradakine geç."
                "Güncellemelerde yan yana biriken eski paket sürümlerini listeler ve silinmesini sorar (etkin + önceki + kilitteki sürüm kalır)."
            )
            # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
            local UI_PICK_BACK_DESC="Ana menüye döner."
            if declare -F ui_card_on >/dev/null && ui_card_on; then
                # fzf: değer/kaynak/varsayılan kartı, geniş düzen
                # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
                UI_PICK_DESCS=("$(_settings_card STATUS 'Menüde sunucuların yanında 🟢 açık / 🔴 erişilemiyor / 🔒 knock gerekir / ⚪ ayar yok işaretini gösterir. Enter: aç/kapat.')"
                    "$(_settings_card STATUS_TIMEOUT 'Durum kontrolünün bir sunucu için en çok kaç saniye bekleyeceği (1-5). Enter: değiştir.')"
                    "$(_settings_card UI 'auto (fzf varsa fzf, yoksa ok tuşları) · fzf · arrows · numbers. Enter: sıradakine geç.')"
                    "$(ui_card_head '🧹 Eski paket sürümleri'; printf '\nGüncellemelerde yan yana biriken eski sürümleri listeler ve silinmesini sorar\n(etkin + önceki + kilitteki sürüm kalır).\n')")
                # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
                local UI_PICK_WIDE=1 UI_PICK_TITLE="Ana menü › Ayarlar" UI_PICK_HEIGHT=18
            fi
            choice=$(ui_pick_arrows 1 "Geri" \
                "$(printf '%-28s %s%s' "$(_settings_label STATUS)" "$(settings_get STATUS)" "$(_settings_note STATUS)")" \
                "$(printf '%-28s %s%s' "$(_settings_label STATUS_TIMEOUT)" "$(settings_get STATUS_TIMEOUT)" "$(_settings_note STATUS_TIMEOUT)")" \
                "$(printf '%-28s %s%s' "$(_settings_label UI)" "$(settings_get UI)" "$(_settings_note UI)")" \
                "Eski paket sürümlerini temizle")
        else
            echo "=== ⚙️  Ayarlar ==="
            echo ""
            printf '  1) %-28s %s%s\n' "$(_settings_label STATUS)" "$(settings_get STATUS)" "$(_settings_note STATUS)"
            printf '  2) %-28s %s%s\n' "$(_settings_label STATUS_TIMEOUT)" "$(settings_get STATUS_TIMEOUT)" "$(_settings_note STATUS_TIMEOUT)"
            printf '  3) %-28s %s%s\n' "$(_settings_label UI)" "$(settings_get UI)" "$(_settings_note UI)"
            echo "  4) Eski paket sürümlerini temizle"
            echo "  0) Geri (q)"
            echo ""
            if ! read -r -p "Değiştirilecek ayar [0]: " choice; then ui_no_pause; return 0; fi
        fi
        case "${choice:-0}" in
            0|q|Q) ui_no_pause; return 0 ;;
            1)
                cur=$(settings_get STATUS)
                if [ "$cur" = "on" ]; then new=off; else new=on; fi
                settings_set STATUS "$new" || read -r -p "Enter..." _ || true
                ;;
            3)
                case "$(settings_get UI)" in
                    auto) new=numbers ;;
                    numbers) new=arrows ;;
                    arrows) new=fzf ;;
                    *) new=auto ;;
                esac
                settings_set UI "$new" || read -r -p "Enter..." _ || true
                ;;
            4) _settings_prune_packages ;;
            2)
                if ! read -r -p "Süre 1-5 sn [$(settings_get STATUS_TIMEOUT)]: " new; then return 0; fi
                if [ -n "$new" ]; then
                    settings_set STATUS_TIMEOUT "$new" || read -r -p "Enter..." _ || true
                fi
                ;;
        esac
    done
}
