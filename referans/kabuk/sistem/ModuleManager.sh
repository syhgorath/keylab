#!/usr/bin/env bash
# ModuleManager.sh - Sunucuları listele / sil / yeniden adlandır
# Kullanım:
#   bash sistem/ModuleManager.sh list
#   bash sistem/ModuleManager.sh remove <ad>
#   bash sistem/ModuleManager.sh rename <eski_ad> <yeni_ad>     (yalnızca yeni biçim; eskiler için önce --migrate)
# Yeni biçim: servers/<ad>/server.env.  Eski biçim: sistem/Modules/<ad>/.env (listelenir/silinir, yeniden adlandırılmaz).
# SSHPOCKET_SERVERS_DIR / SSHPOCKET_MODULES_DIR ile klasörler değiştirilebilir (testler için).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SISTEM_PATH="$SCRIPT_DIR"
LEGACY_DIR="${SSHPOCKET_MODULES_DIR:-$SCRIPT_DIR/Modules}"

# shellcheck source=Helpers/Validate.sh
source "$SCRIPT_DIR/Helpers/Validate.sh"
# shellcheck source=Helpers/ModuleFiles.sh
source "$SCRIPT_DIR/Helpers/ModuleFiles.sh"
# shellcheck source=Helpers/SSHModule.sh
source "$SCRIPT_DIR/Helpers/SSHModule.sh"
# shellcheck source=Helpers/Servers.sh
source "$SCRIPT_DIR/Helpers/Servers.sh"
SERVERS_DIR="$(servers_dir)"

# Bir satır yazdırır. Parametreler: $1 - klasör, $2 - ad, $3 - etiket (ops.)
_list_line() {
    local d="$1" n="$2" tag="${3:-}" ip user port prefix
    prefix=$(module_prefix "$n")
    ip=$(srv_get "$d" "$prefix" IP 2>/dev/null)
    user=$(srv_get "$d" "$prefix" USER 2>/dev/null)
    port=$(srv_get "$d" "$prefix" PORT 2>/dev/null)
    printf '  %-18s %s%s%s%s\n' "$n" "${user:+$user@}" "${ip:-(ayar yok)}" "${port:+:$port}" "${tag:+   $tag}"
}

cmd_list() {
    local d n found=0
    if [ -d "$SERVERS_DIR" ]; then
        while IFS= read -r n; do
            [ -n "$n" ] || continue
            _list_line "$SERVERS_DIR/$n" "$n"
            found=1
        done < <(srv_names)
    fi
    if [ -d "$LEGACY_DIR" ]; then
        for d in "$LEGACY_DIR"/*/; do
            [ -d "$d" ] || continue
            n=$(basename "$d")
            module_name_valid "$n" || continue
            [ -f "${d}.env" ] || continue
            if [ -d "$SERVERS_DIR/$n" ]; then
                _list_line "$d" "$n" "[eski kopya; yeni biçim kazanır]"
            else
                _list_line "$d" "$n" "[eski biçim: ./start.sh --migrate]"
            fi
            found=1
        done
    fi
    [ "$found" -eq 1 ] || echo "  (sunucu yok)"
}

# Adı doğrular ve sunucu klasörünü yazdırır (önce yeni biçim)
_locate() {
    if ! module_name_valid "$1"; then echo "⚠️  Geçersiz sunucu adı: '$1'" >&2; return 1; fi
    if [ -d "$SERVERS_DIR/$1" ]; then printf '%s' "$SERVERS_DIR/$1"; return 0; fi
    if [ -d "$LEGACY_DIR/$1" ]; then printf '%s' "$LEGACY_DIR/$1"; return 0; fi
    echo "⚠️  Sunucu yok: $1" >&2
    return 1
}

cmd_remove() {
    local name="$1" dir upper confirm
    dir=$(_locate "$name") || return 1
    upper=$(module_prefix "$name")
    echo "Silinecek: $dir"
    ls -A "$dir"
    read -r -p "Onaylamak için sunucu adını yazın ($name): " confirm
    if [ "$confirm" != "$name" ]; then echo "İptal edildi."; return 1; fi
    rm -r -- "${dir:?}"
    echo "✅ Silindi: $name"
    # Knock dizisi sunucuya bağlı bir sırdır; geride yetim kalmasın
    if command -v security >/dev/null 2>&1 \
        && security delete-generic-password -a "USBMonitor_${upper}_Knock" -s USBMonitor >/dev/null 2>&1; then
        echo "✅ Knock dizisi Keychain'den silindi"
    fi
    if command -v security >/dev/null 2>&1 \
        && security find-generic-password -a "USBMonitor_${upper}_Passphrase" -s USBMonitor >/dev/null 2>&1; then
        local del
        read -r -p "Keychain'deki passphrase kaydı da silinsin mi? (y/n) " del
        if [ "$del" = "y" ] || [ "$del" = "Y" ]; then
            security delete-generic-password -a "USBMonitor_${upper}_Passphrase" -s USBMonitor >/dev/null \
                && echo "✅ Keychain kaydı silindi"
        fi
    fi
}

cmd_rename() {
    local old="$1" new="$2" unew
    if ! module_name_valid "$old"; then echo "⚠️  Geçersiz sunucu adı: '$old'" >&2; return 1; fi
    if [ ! -d "$SERVERS_DIR/$old" ]; then
        if [ -d "$LEGACY_DIR/$old" ]; then
            echo "⚠️  '$old' eski biçimde. Önce taşıyın: ./start.sh --migrate" >&2
        else
            echo "⚠️  Sunucu yok: $old" >&2
        fi
        return 1
    fi
    if ! module_name_valid "$new"; then echo "⚠️  Geçersiz yeni ad: '$new'" >&2; return 1; fi
    if [ -e "$SERVERS_DIR/$new" ] || [ -e "$LEGACY_DIR/$new" ]; then
        echo "⚠️  Hedef zaten var: $new" >&2; return 1
    fi

    # Klasörü tek hamlede (rename) taşı; sonra key dosyalarını yeni ada çevir. Hata olursa geri al.
    if ! mv "$SERVERS_DIR/$old" "$SERVERS_DIR/$new"; then
        echo "⚠️  Yeniden adlandırılamadı" >&2; return 1
    fi
    local ext
    for ext in "" ".pub"; do
        if [ -f "$SERVERS_DIR/$new/id_ed25519_${old}${ext}" ]; then
            if ! mv "$SERVERS_DIR/$new/id_ed25519_${old}${ext}" "$SERVERS_DIR/$new/id_ed25519_${new}${ext}"; then
                echo "⚠️  Key dosyası yeniden adlandırılamadı; geri alınıyor" >&2
                mv "$SERVERS_DIR/$new" "$SERVERS_DIR/$old"
                return 1
            fi
        fi
    done
    unew=$(module_prefix "$new")
    echo "✅ $old → $new"
    echo "ℹ️  Keychain kaydı otomatik taşınmaz. Passphrase kullanıyorsanız yeniden ekleyin:"
    echo "   security add-generic-password -U -a USBMonitor_${unew}_Passphrase -s USBMonitor -w"
    if command -v security >/dev/null 2>&1 \
        && security find-generic-password -a "USBMonitor_$(module_prefix "$old")_Knock" -s USBMonitor >/dev/null 2>&1; then
        echo "ℹ️  Knock dizisi de eski adla Keychain'de duruyor; yeni adda sunucu menüsü → Knock ayarları ile yeniden girin."
    fi
}

case "${1:-}" in
    list)   cmd_list ;;
    remove) [ -n "${2:-}" ] || { echo "Kullanım: remove <ad>" >&2; exit 1; }; cmd_remove "$2" ;;
    rename) [ -n "${3:-}" ] || { echo "Kullanım: rename <eski> <yeni>" >&2; exit 1; }; cmd_rename "$2" "$3" ;;
    *) echo "Kullanım: $0 list | remove <ad> | rename <eski> <yeni>" >&2; exit 1 ;;
esac
