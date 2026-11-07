#!/usr/bin/env bash
# Eject.sh - USB'yi güvenli çıkarma

# Parametre: $1 - USB mount yolu
EjectUSB() {
    local usb_path="$1"
    [ -n "$usb_path" ] && [ -d "$usb_path" ] || return 0

    if ! command -v diskutil >/dev/null 2>&1; then
        echo "⚠️  diskutil yok, USB'yi elle çıkarın"
        return 1
    fi

    # USB'deki anahtarlar çıkarmadan ÖNCE ajandan silinir (çıkarılınca .pub dosyaları okunamaz); anahtar bilgisayarda kalmasın
    if declare -F ssh_agent_remove_usb_keys >/dev/null; then
        local removed
        removed=$(ssh_agent_remove_usb_keys "$usb_path")
        if [ "${removed:-0}" -gt 0 ] 2>/dev/null; then echo "🔑 $removed anahtar ssh-agent'tan çıkarıldı"; fi
    fi

    echo ""
    echo "📤 USB çıkarılıyor: $(basename "$usb_path")"
    # Script USB üzerinde çalışıyor olabilir; çalışma dizinini taşı
    cd "$HOME" 2>/dev/null || cd / || true

    local disk_id
    disk_id=$(diskutil info "$usb_path" 2>/dev/null | awk -F': *' '/Device Identifier/ {print $2; exit}')

    if [ -n "$disk_id" ] && diskutil eject "$disk_id" >/dev/null 2>&1; then
        echo "✅ USB çıkarıldı"; Log "INFO" "USB ejected: $disk_id"; return 0
    fi
    if diskutil unmount force "$usb_path" >/dev/null 2>&1; then
        echo "✅ USB unmount edildi (force)"; Log "INFO" "USB force unmounted: $usb_path"
        [ -n "$disk_id" ] && diskutil eject "$disk_id" >/dev/null 2>&1
        return 0
    fi
    echo "⚠️  USB çıkarılamadı (kullanımda olabilir), elle çıkarabilirsiniz"
    Log "WARN" "Failed to eject USB: $usb_path"
    return 1
}
