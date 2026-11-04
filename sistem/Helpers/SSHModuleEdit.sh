#!/usr/bin/env bash
# SSHModuleEdit.sh - Modül ayarlarını (.env) ve Keychain kaydını güncelleme
# Gerektirir: Validate.sh, SSHModule.sh (_ssh_env_get), SSHHelper.sh (Keychain/agent yardımcıları)

# .env içinde bir anahtarın değerini günceller (yoksa sona ekler); diğer satırlar ve yorumlar korunur.
# Değer kabuğa verilmez, yalnızca printf ile yazılır. Dosya izni 600 yapılır.
# Parametreler: $1 - .env dosyası, $2 - anahtar, $3 - değer
_ssh_env_set() {
    local file="$1" want="$2" value="$3"
    local tmp line trimmed key found=0
    tmp=$(mktemp "${file}.XXXXXX") || return 1
    if [ -f "$file" ]; then
        while IFS= read -r line || [ -n "$line" ]; do
            key=""
            trimmed="${line#"${line%%[![:space:]]*}"}"
            case "$trimmed" in
                ''|\#*) ;;
                *=*) key="${trimmed%%=*}"; key="${key%"${key##*[![:space:]]}"}" ;;
            esac
            if [ "$key" = "$want" ] && [ "$found" -eq 0 ]; then
                printf '%s=%s\n' "$want" "$value" >> "$tmp"
                found=1
            else
                printf '%s\n' "$line" >> "$tmp"
            fi
        done < "$file"
    fi
    if [ "$found" -eq 0 ]; then
        printf '%s=%s\n' "$want" "$value" >> "$tmp"
    fi
    chmod 600 "$tmp"
    mv "$tmp" "$file"
}

# IP/kullanıcı/port/key yolunu soru-cevapla günceller (Enter = değiştirme)
# Parametreler: $1 - öneki, $2 - modül klasörü
ssh_module_edit() {
    local prefix="$1" module_dir="$2"
    local env_file
    env_file=$(srv_env_file "$module_dir")
    local cur_ip cur_user cur_port cur_key ip user port key verr

    if [ ! -f "$env_file" ]; then
        : > "$env_file"
        chmod 600 "$env_file"
    fi
    cur_ip=$(srv_get "$module_dir" "$prefix" IP)
    cur_user=$(srv_get "$module_dir" "$prefix" USER)
    cur_port=$(srv_get "$module_dir" "$prefix" PORT)
    cur_port="${cur_port:-22}"
    cur_key=$(srv_get "$module_dir" "$prefix" KEY)

    echo ""
    echo "Bilgileri güncelle (köşeli parantez = mevcut değer, Enter = değiştirme)"
    read -r -p "IP/host [${cur_ip}]: " ip || return 1
    ip="${ip:-$cur_ip}"
    read -r -p "Kullanıcı [${cur_user}]: " user || return 1
    user="${user:-$cur_user}"
    read -r -p "Port [${cur_port}]: " port || return 1
    port="${port:-$cur_port}"
    read -r -p "Key yolu [${cur_key:-modül klasöründeki varsayılan}] (Enter = aynı, - = varsayılana dön): " key || return 1
    case "$key" in
        '') key="$cur_key" ;;
        -)  key="" ;;
    esac

    if ! verr=$(ssh_validate_target "$ip" "$user" "$port" 2>&1); then
        echo "⚠️  Kaydedilmedi: $verr"
        return 1
    fi

    srv_set "$module_dir" "$prefix" IP "$ip"
    srv_set "$module_dir" "$prefix" USER "$user"
    srv_set "$module_dir" "$prefix" PORT "$port"
    if [ -n "$key" ] || [ -n "$cur_key" ]; then
        srv_set "$module_dir" "$prefix" KEY "$key"
    fi

    local check="$key"
    if [ "${check:0:1}" = "~" ]; then check="$HOME${check:1}"; fi
    if [ -n "$check" ] && [ ! -f "$check" ]; then
        echo "ℹ️  Uyarı: key dosyası şu an yok: $check"
    fi
    echo "✅ Güncellendi: ${user}@${ip}:${port}"
}

# Keychain'deki passphrase kaydını kaydet/güncelle/sil
# Parametreler: $1 - öneki, $2 - private key yolu (doğrulama için)
ssh_keychain_manage() {
    local prefix="$1" ssh_key="$2" choice account durum fzf_card=0
    account=$(ssh_keychain_account "$prefix")
    if declare -F ui_card_on >/dev/null && ui_card_on; then fzf_card=1; fi
    if ssh_has_passphrase "$prefix"; then
        durum="Keychain kaydı: ✅ var ($account)"
    else
        durum="Keychain kaydı: ❌ yok"
    fi
    if [ "$fzf_card" = 1 ]; then
        clear
        ui_screen "Ana menü › ${prefix} › Keychain passphrase"   # fzf: başlık kenarlıkta, durum başlık kutusunda
    else
        echo ""; echo "$durum"
    fi
    if declare -F ui_arrows_enabled >/dev/null && ui_arrows_enabled; then
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local -a UI_PICK_DESCS=("Passphrase'i Keychain'e kaydeder veya günceller; kayıttan sonra key'i açarak doğrular." "Keychain'deki passphrase kaydını siler (key dosyası silinmez).")
        if [ "$fzf_card" = 1 ]; then
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            UI_PICK_DESCS=("$(ui_card_head '🔑 Kaydet / güncelle'; ui_card_row Hesap "$account"; printf '\nPassphrase güvenli sorulur (ekranda görünmez);\nkayıttan sonra key agent'"'"'tan çıkarılıp yeniden eklenerek doğrulanır.\n')"
                "$(ui_card_head '🗑️  Sil'; ui_card_row Hesap "$account"; printf '\nKeychain'"'"'deki passphrase kaydı silinir.\n⚠ Key dosyası silinmez.\n')")
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            local UI_PICK_WIDE=1 UI_PICK_HEIGHT=14 UI_PICK_HEADER=""
            # shellcheck disable=SC2034  # _ui_pick_fzf (dinamik kapsam) okur
            UI_PICK_HEADER="$durum"
            ui_fit_wide 3 3
        fi
        # shellcheck disable=SC2034  # ui_pick_arrows (dinamik kapsam) okur
        local UI_PICK_BACK_DESC="Sunucu menüsüne döner."
        choice=$(ui_pick_arrows 1 "Geri" "Kaydet / güncelle" "Sil")
        echo ""
    else
        echo "  1) Kaydet / güncelle"
        echo "  2) Sil"
        echo "  0) Geri (q)"
        read -r -p "Seçim [0]: " choice || return 0
    fi
    case "${choice:-0}" in
        1)
            # -w son argüman: passphrase komut satırında görünmez, güvenli sorulur
            ssh_keychain_prompt_note
            if security add-generic-password -U -a "$account" -s "$SSH_KEYCHAIN_SERVICE" -w; then
                echo "✅ Keychain'e kaydedildi"
                if [ -f "$ssh_key" ]; then
                    echo "🔎 Doğrulanıyor (key agent'tan çıkarılıp passphrase ile yeniden ekleniyor)..."
                    ssh-add -d "$ssh_key" >/dev/null 2>&1
                    if ssh_agent_add "$prefix" "$ssh_key"; then
                        echo "✅ Key açıldı, passphrase doğru."
                    else
                        echo "⚠️  Doğrulanamadı: passphrase yanlış olabilir. Seçenek 1 ile tekrar deneyin."
                    fi
                fi
            else
                echo "⚠️  Kaydedilemedi"
            fi
            ;;
        2)
            if security delete-generic-password -a "$account" -s "$SSH_KEYCHAIN_SERVICE" >/dev/null 2>&1; then
                echo "✅ Keychain kaydı silindi"
            else
                echo "ℹ️  Silinecek kayıt yok"
            fi
            ;;
    esac
}
