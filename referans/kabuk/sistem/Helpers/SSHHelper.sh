#!/usr/bin/env bash
# SSHHelper.sh - Keychain tabanlı passphrase yönetimi ve SSH başlatma
# Passphrase hiçbir zaman diske yazılmaz: macOS Keychain'de durur ve
# SSH_ASKPASS (Helpers/askpass.sh) üzerinden ssh-add'e doğrudan aktarılır.

SSH_KEYCHAIN_SERVICE="USBMonitor"

# Keychain'deki passphrase hesabının adı
# Parametre: $1 - modül öneki (büyük harf, örn: RPI)
ssh_keychain_account() {
    echo "USBMonitor_${1}_Passphrase"
}

# Keychain'e kaydetmeden hemen önce ne istendiğini açıklar: macOS'un kendi istemi yalnızca
# "password data for new item:" der ve bunun SSH anahtarının passphrase'i olduğunu söylemez.
ssh_keychain_prompt_note() {
    echo "🔐 macOS şimdi SSH anahtarınızın PASSPHRASE'ini (anahtarı oluştururken verdiğiniz) Keychain'e kaydetmek için soracak."
    echo "   İstem 'password data for new item:' diye görünür; yazarken ekranda bir şey görünmez."
    echo "   Aynı passphrase'i yazın; macOS doğrulama için bir kez daha sorabilir. Sunucunun sudo parolası DEĞİLDİR."
}

# USB'deki anahtarları ssh-agent'tan çıkarır (USB çıkarılmadan ÖNCE çağrılır; çıkarılınca .pub dosyalarına erişilemez).
# Yalnızca USB'deki servers/*/*.pub dosyalarına karşılık gelen anahtarlar silinir; ajandaki başka anahtarlara (GitHub vb.) dokunulmaz.
# ".pub" kullanılır: özel anahtara parola sorulmaz. Ajan yoksa ya da anahtar zaten ajanda değilse sessizce geçer.
# Parametre: $1 - USB mount yolu → çıkarılan anahtar sayısı stdout'a yazılır
ssh_agent_remove_usb_keys() {
    local usb="$1" pub n=0
    [ -n "$usb" ] && [ -d "$usb/servers" ] || { echo 0; return 0; }
    command -v ssh-add >/dev/null 2>&1 || { echo 0; return 0; }
    for pub in "$usb"/servers/*/*.pub; do
        [ -f "$pub" ] || continue
        if ssh-add -d "$pub" >/dev/null 2>&1; then n=$((n + 1)); fi
    done
    echo "$n"
    return 0
}

# Passphrase Keychain'de kayıtlı mı?
# Parametre: $1 - modül öneki
ssh_has_passphrase() {
    security find-generic-password -a "$(ssh_keychain_account "$1")" \
        -s "$SSH_KEYCHAIN_SERVICE" >/dev/null 2>&1
}

# SSH key'i (gerekirse Keychain'deki passphrase ile) agent'a ekle
# Parametreler: $1 - modül öneki, $2 - key dosyası
ssh_agent_add() {
    local prefix="$1"
    local ssh_key="$2"
    local fingerprint
    fingerprint=$(ssh-keygen -lf "$ssh_key" 2>/dev/null | awk '{print $2}')

    if [ -n "$fingerprint" ] && ssh-add -l 2>/dev/null | grep -q "$fingerprint"; then
        echo "✅ SSH key zaten agent'ta" >&2
        return 0
    fi

    echo "🔐 SSH key agent'a ekleniyor (Keychain passphrase'i ile)..." >&2
    # ssh-add yanlış passphrase'te sonsuz döner: askpass yalnızca bir kez cevap verir
    local state
    state="$(mktemp -u "${TMPDIR:-/tmp}/sshpocket-askpass.XXXXXX")"
    SSH_ASKPASS="$SISTEM_PATH/Helpers/askpass.sh" \
    ASKPASS_STATE="$state" \
    SSH_ASKPASS_REQUIRE=force \
    DISPLAY="${DISPLAY:-:0}" \
    ASKPASS_ACCOUNT="$(ssh_keychain_account "$prefix")" \
    ASKPASS_SERVICE="$SSH_KEYCHAIN_SERVICE" \
        ssh-add "$ssh_key" </dev/null >/dev/null 2>&1
    rm -f "$state"

    if [ -n "$fingerprint" ] && ssh-add -l 2>/dev/null | grep -q "$fingerprint"; then
        echo "✅ SSH key agent'a eklendi" >&2
        return 0
    fi
    echo "⚠️  Key agent'a eklenemedi, ssh passphrase'i kendisi soracak" >&2
    return 1
}

# ssh_agent_add başarısız olursa (Keychain'deki passphrase yanlış olabilir)
# yeni passphrase'i sorar, Keychain'e yazar ve yeniden dener (en çok 3 kez).
# Agent'a hiç ulaşılamıyorsa (ssh-add -l çıkış 2) passphrase suçlanmaz.
# Parametreler: $1 - modül öneki, $2 - key dosyası
ssh_agent_add_retry() {
    local prefix="$1" ssh_key="$2" attempt=0 answer rc
    ssh_agent_add "$prefix" "$ssh_key" && return 0
    while [ "$attempt" -lt 3 ]; do
        ssh-add -l >/dev/null 2>&1; rc=$?
        [ "$rc" -eq 2 ] && return 1
        read -r -p "Kayıtlı passphrase yanlış olabilir. Yeniden girip Keychain'e kaydedeyim mi? [e/H]: " answer || return 1
        case "$answer" in e|E|evet|Evet) ;; *) return 1 ;; esac
        # -w son argüman: passphrase komut satırında görünmez, güvenli sorulur
        ssh_keychain_prompt_note
        security add-generic-password -U -a "$(ssh_keychain_account "$prefix")" \
            -s "$SSH_KEYCHAIN_SERVICE" -w || return 1
        attempt=$((attempt + 1))
        ssh_agent_add "$prefix" "$ssh_key" && return 0
    done
    return 1
}

# Yeni bir Terminal penceresinde ssh çalıştırır
# Parametreler: $1 - kullanıcı, $2 - ip, $3 - port, $4 - key dosyası
# Ortam: SSH_VERBOSE=1 ise -v ekler
ssh_connect() {
    local user="$1" ip="$2" port="${3:-22}" ssh_key="$4"

    local -a args=(ssh)
    [ "${SSH_VERBOSE:-0}" = "1" ] && args+=(-v)
    args+=(-p "$port")
    [ -f "$ssh_key" ] && args+=(-i "$ssh_key" -o IdentitiesOnly=yes)
    # Host key ilk bağlantıda kullanıcıya sorulur (otomatik kabul yok)
    args+=(-o "PreferredAuthentications=publickey,keyboard-interactive")
    args+=(-o PasswordAuthentication=no)
    args+=("${user}@${ip}")

    # Boşluklu yollar (iCloud, USB adı) için kabuk kaçışı
    local cmd
    cmd=$(printf '%q ' "${args[@]}")

    echo "🔌 SSH bağlantısı kuruluyor: ${user}@${ip}:${port}"

    if command -v osascript >/dev/null 2>&1; then
        # AppleScript string kaçışı
        local as_cmd="${cmd//\\/\\\\}"
        as_cmd="${as_cmd//\"/\\\"}"
        if osascript -e 'tell application "Terminal"' \
                     -e 'activate' \
                     -e "do script \"${as_cmd}\"" \
                     -e 'end tell' >/dev/null 2>&1; then
            echo "✅ Terminal penceresi açıldı"
            return 0
        fi
        echo "⚠️  Terminal açılamadı, bu pencerede bağlanılıyor" >&2
    fi
    "${args[@]}"
}
