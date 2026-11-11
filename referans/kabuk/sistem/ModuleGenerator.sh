#!/usr/bin/env bash
# ModuleGenerator.sh - Yeni SSH modülü oluşturur
# Kullanım: bash sistem/ModuleGenerator.sh
# Passphrase asla dosyaya yazılmaz; macOS Keychain'e kaydedilir.
# Sunucu servers/<ad>/server.env olarak yazılır. SSHPOCKET_SERVERS_DIR ile klasör değiştirilebilir (testler için).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SISTEM_PATH="$SCRIPT_DIR"
LEGACY_DIR="${SSHPOCKET_MODULES_DIR:-$SCRIPT_DIR/Modules}"

# shellcheck source=Helpers/Validate.sh
source "$SCRIPT_DIR/Helpers/Validate.sh"
# shellcheck source=Helpers/ModuleFiles.sh
source "$SCRIPT_DIR/Helpers/ModuleFiles.sh"
# shellcheck source=Helpers/Servers.sh
source "$SCRIPT_DIR/Helpers/Servers.sh"
# shellcheck source=Helpers/SSHHelper.sh
source "$SCRIPT_DIR/Helpers/SSHHelper.sh"
# shellcheck source=Helpers/SSHModule.sh
source "$SCRIPT_DIR/Helpers/SSHModule.sh"
# Paket platformu / zorunlu ayar soruları (manifestinde platform_choices olan paket yoksa hiçbir şey sorulmaz)
for _f in Components ComponentsInstall ComponentsManifest ComponentsRun ComponentsVars ComponentsMenu ComponentsOnboard; do
    # shellcheck source=/dev/null
    source "$SCRIPT_DIR/Helpers/$_f.sh"
done
SERVERS_DIR="$(servers_dir)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

echo -e "${BLUE}=== Yeni sunucu ===${NC}"
echo ""

read -r -p "Sunucu adı (küçük harf/rakam/_, harfle başlar, örn: myserver): " module_name
module_name=$(echo "$module_name" | tr '[:upper:]' '[:lower:]' | tr ' ' '_' | tr -cd '[:alnum:]_')
if ! module_name_valid "$module_name"; then
    echo -e "${RED}❌ Geçersiz ad (harfle başlamalı)${NC}"; exit 1
fi

module_dir="$SERVERS_DIR/$module_name"
if [ -e "$module_dir" ] || [ -e "$LEGACY_DIR/$module_name" ]; then
    echo -e "${RED}❌ Bu adda bir sunucu zaten var: $module_name${NC}"
    echo "   (Üzerine yazılmaz; önce 'ModuleManager.sh remove $module_name' kullanın.)"
    exit 1
fi

upper=$(module_prefix "$module_name")

read -r -p "SSH IP/host: " ssh_ip
read -r -p "SSH kullanıcı: " ssh_user
read -r -p "SSH port [22]: " ssh_port
ssh_port="${ssh_port:-22}"

if ! verr=$(ssh_validate_target "$ssh_ip" "$ssh_user" "$ssh_port" 2>&1); then
    echo -e "${RED}❌ $verr${NC}"; exit 1
fi

server_write_files "$SERVERS_DIR" "$module_name" "$ssh_ip" "$ssh_user" "$ssh_port" || exit 1
key="$module_dir/id_ed25519_${module_name}"

echo ""
read -r -p "Yeni ed25519 SSH key oluşturulsun mu? (y/n) " create_key
if [ "$create_key" = "y" ] || [ "$create_key" = "Y" ]; then
    echo -e "${YELLOW}Passphrase belirleyin (ssh-keygen soracak; boş bırakmanız önerilmez):${NC}"
    ssh-keygen -t ed25519 -a 100 -C "$module_name" -f "$key" || { echo -e "${RED}❌ Key oluşturulamadı${NC}"; exit 1; }
    chmod 600 "$key"
    echo ""
    echo -e "${GREEN}Public key (sunucudaki ~/.ssh/authorized_keys'e ekleyin):${NC}"
    cat "${key}.pub"
    echo ""
    read -r -p "Passphrase Keychain'e kaydedilsin mi? (y/n) " save_kc
    if [ "$save_kc" = "y" ] || [ "$save_kc" = "Y" ]; then
        # -w son argüman: passphrase komut satırında görünmez, güvenli sorulur
        ssh_keychain_prompt_note
        security add-generic-password -U -a "USBMonitor_${upper}_Passphrase" -s "USBMonitor" -w \
            && echo -e "${GREEN}✅ Keychain'e kaydedildi${NC}" \
            || echo -e "${RED}❌ Keychain kaydı başarısız${NC}"
    fi
else
    echo -e "${YELLOW}💡 Mevcut key'i şuraya koyun: $key${NC}"
    echo "   (ya da server.env'e KEY=/yol/key ekleyin)"
    echo "   Passphrase'i Keychain'e eklemek için:"
    echo "   security add-generic-password -U -a USBMonitor_${upper}_Passphrase -s USBMonitor -w"
fi

# Key varsa hedef sunucuya göndermeyi öner
if [ -f "$key" ]; then
    echo ""
    read -r -p "Public key şimdi sunucuya gönderilsin mi? (y/n) " do_copy
    if [ "$do_copy" = "y" ] || [ "$do_copy" = "Y" ]; then
        # shellcheck source=Helpers/SSHKeyCopy.sh
        source "$SCRIPT_DIR/Helpers/SSHKeyCopy.sh"
        ssh_copy_key "$ssh_user" "$ssh_ip" "$ssh_port" "$key" || true
    fi
fi

# Kurulu paketlerin platform listesi ve zorunlu ayarları (anahtar kopyalandıktan sonra: "sunucunun gördüğü adres" sorgusu çalışabilsin)
comp_onboard_all "$module_dir"

echo ""
echo -e "${GREEN}✅ Sunucu oluşturuldu: $module_dir${NC}"
echo "   Menüde '${upper}' olarak görünür (start.sh ile başlatın)."
