#!/usr/bin/env bash
# ModuleFiles.sh - Modül dosyalarını üretir (generator, içe aktarma ve yeniden adlandırma ortak kullanır)
# Gerektirir: Validate.sh

# Parametre: $1 - modül adı (küçük harf)
module_prefix() {
    printf '%s' "$1" | tr '[:lower:]' '[:upper:]'
}

# Parametre: $1 - modül adı
module_func_name() {
    local first rest
    first=$(printf '%s' "${1:0:1}" | tr '[:lower:]' '[:upper:]')
    rest="${1:1}"
    printf '%s%sModule' "$first" "$rest"
}

# Modül klasörünü, .env, .env.example ve ince modül dosyasını yazar. Key üretmez.
# Parametreler: $1 - Modules klasörü, $2 - ad, $3 - ip, $4 - kullanıcı, $5 - port,
#               $6 - (ops.) mevcut private key yolu (varsayılan: modül klasöründeki id_ed25519_<ad>)
module_write_files() {
    local modules_dir="$1" name="$2" ip="$3" user="$4" port="$5" key="${6:-}"

    if ! module_name_valid "$name"; then
        echo "geçersiz sunucu adı: '$name'" >&2; return 1
    fi
    ssh_validate_target "$ip" "$user" "$port" || return 1
    case "$key" in *$'\n'*) echo "geçersiz key yolu" >&2; return 1 ;; esac

    local dir="$modules_dir/$name"
    local upper func
    upper=$(module_prefix "$name")
    func=$(module_func_name "$name")
    mkdir -p "$dir" || return 1

    {
        echo "# SSH Bağlantı Bilgileri (gizli bilgi buraya YAZILMAZ)"
        echo "${upper}_IP="
        echo "${upper}_USER="
        echo "${upper}_PORT=22"
        echo "# ${upper}_KEY=/yol/mevcut_private_key   # (ops.) modül klasörü yerine başka bir key"
    } > "$dir/.env.example"

    {
        echo "# SSH Bağlantı Bilgileri (gizli bilgi buraya YAZILMAZ)"
        echo "${upper}_IP=${ip}"
        echo "${upper}_USER=${user}"
        echo "${upper}_PORT=${port}"
        if [ -n "$key" ]; then echo "${upper}_KEY=${key}"; fi
    } > "$dir/.env"
    chmod 600 "$dir/.env"

    local tpl
    tpl=$(cat <<'TPL'
#!/usr/bin/env bash
# @FUNC@.sh - @NAME@ SSH bağlantı modülü
# Ayarlar: .env (@UPPER@_IP, @UPPER@_USER, @UPPER@_PORT, ops. @UPPER@_KEY)
# Key: id_ed25519_@NAME@ | Passphrase: Keychain (USBMonitor_@UPPER@_Passphrase)

RegisterModules "@UPPER@" "MainMenu" "@FUNC@"

_@UPPER@_MODULE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

@FUNC@() {
    ssh_module_run "@UPPER@" "$_@UPPER@_MODULE_DIR"
}

@FUNC@_status() {
    ssh_module_status "@UPPER@" "$_@UPPER@_MODULE_DIR"
}
TPL
)
    tpl="${tpl//@UPPER@/$upper}"
    tpl="${tpl//@FUNC@/$func}"
    tpl="${tpl//@NAME@/$name}"
    printf '%s\n' "$tpl" > "$dir/${func}.sh"
    chmod +x "$dir/${func}.sh"
}

# YENİ BİÇİM: servers/<ad>/server.env yazar (sunucu başına üretilmiş bash dosyası YOK). Key üretmez.
# Hedef zaten varsa 3 döner ve hiçbir şeyi değiştirmez.
# Parametreler: $1 - servers klasörü, $2 - ad, $3 - ip, $4 - kullanıcı, $5 - port,
#               $6 - (ops.) mevcut private key yolu, $7 - (ops.) OS profili
server_write_files() {
    local sdir="$1" name="$2" ip="$3" user="$4" port="$5" key="${6:-}" os="${7:-}"

    if ! module_name_valid "$name"; then
        echo "geçersiz sunucu adı: '$name'" >&2; return 1
    fi
    ssh_validate_target "$ip" "$user" "$port" || return 1
    case "$key$os" in *$'\n'*) echo "geçersiz değer (satır sonu)" >&2; return 1 ;; esac

    local dir="$sdir/$name"
    if [ -e "$dir" ]; then
        return 3
    fi
    mkdir -p "$sdir" && chmod 700 "$sdir" 2>/dev/null
    mkdir "$dir" || return 1
    chmod 700 "$dir"
    {
        echo "# SSH bağlantı bilgileri (gizli bilgi buraya YAZILMAZ)"
        echo "IP=${ip}"
        echo "USER=${user}"
        echo "PORT=${port}"
        if [ -n "$key" ]; then echo "KEY=${key}"; fi
        if [ -n "$os" ]; then echo "OS=${os}"; fi
    } > "$dir/server.env"
    chmod 600 "$dir/server.env"
}
