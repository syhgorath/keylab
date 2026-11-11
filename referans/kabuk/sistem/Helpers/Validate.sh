#!/usr/bin/env bash
# Validate.sh - Girdi doğrulama (komut/seçenek enjeksiyonuna karşı)

# Modül adı: küçük harfle başlar; küçük harf, rakam, _ içerir
# Parametre: $1 - ad
module_name_valid() {
    [[ "$1" =~ ^[a-z][a-z0-9_]*$ ]]
}

# IP/host, kullanıcı ve portu doğrular; hata mesajını stderr'e yazar
# Kullanıcı adı '-' ile başlayamaz (ssh onu seçenek sanmasın)
# Parametreler: $1 - ip/host, $2 - kullanıcı, $3 - port
ssh_validate_target() {
    local ip="$1" user="$2" port="$3"
    if ! [[ "$ip" =~ ^[A-Za-z0-9._:-]+$ ]]; then
        echo "geçersiz IP/host: '$ip'" >&2; return 1
    fi
    if ! [[ "$user" =~ ^[A-Za-z0-9_][A-Za-z0-9._-]*$ ]]; then
        echo "geçersiz kullanıcı: '$user'" >&2; return 1
    fi
    if ! [[ "$port" =~ ^[0-9]+$ ]]; then
        echo "geçersiz port: '$port'" >&2; return 1
    fi
    if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
        echo "port aralık dışı: '$port'" >&2; return 1
    fi
    return 0
}
