#!/usr/bin/env bash
# ImportSSHConfig.sh - ~/.ssh/config'teki sunucuları modül olarak içe aktarır
# Kullanım: bash sistem/ImportSSHConfig.sh [config_dosyası]
# Private key'ler KOPYALANMAZ; IdentityFile yolu server.env'e KEY olarak yazılır.
# Sunucular servers/<ad>/server.env olarak yazılır. SSHPOCKET_SERVERS_DIR ile klasör değiştirilebilir (testler için).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SISTEM_PATH="$SCRIPT_DIR"
LEGACY_DIR="${SSHPOCKET_MODULES_DIR:-$SCRIPT_DIR/Modules}"
CONFIG="${1:-$HOME/.ssh/config}"

# shellcheck source=Helpers/Validate.sh
source "$SCRIPT_DIR/Helpers/Validate.sh"
# shellcheck source=Helpers/ModuleFiles.sh
source "$SCRIPT_DIR/Helpers/ModuleFiles.sh"
# shellcheck source=Helpers/SSHConfigImport.sh
source "$SCRIPT_DIR/Helpers/SSHConfigImport.sh"
# shellcheck source=Helpers/Servers.sh
source "$SCRIPT_DIR/Helpers/Servers.sh"
# shellcheck source=Helpers/UI.sh
source "$SCRIPT_DIR/Helpers/UI.sh"
# shellcheck source=Helpers/Settings.sh
source "$SCRIPT_DIR/Helpers/Settings.sh"
SERVERS_DIR="$(servers_dir)"

if [ ! -f "$CONFIG" ]; then
    echo "⚠️  Config bulunamadı: $CONFIG"; exit 1
fi

aliases=(); hosts=(); users=(); ports=(); keys=()
while IFS='|' read -r a h u p k; do
    [ -n "$a" ] || continue
    aliases+=("$a"); hosts+=("$h"); users+=("${u:-$USER}"); ports+=("${p:-22}"); keys+=("$k")
done < <(ssh_config_hosts "$CONFIG")

if [ "${#aliases[@]}" -eq 0 ]; then
    echo "Joker içermeyen Host kaydı bulunamadı."; exit 0
fi

selected=()
if ui_arrows_enabled; then
    # ok tuşlu / fzf: Enter bir sunucuyu işaretler/kaldırır; "İçe aktar" satırı onaylar (Esc/geri = iptal)
    marks=(); for i in "${!aliases[@]}"; do marks+=(0); done
    cursor=1
    while true; do
        labels=(); nsel=0
        # shellcheck disable=SC2034  # ui_pick (dinamik kapsam) okur
        UI_PICK_DESCS=()
        for i in "${!aliases[@]}"; do
            if [ "${marks[$i]}" = 1 ]; then box="[x]"; nsel=$((nsel + 1)); else box="[ ]"; fi
            labels+=("$box $(printf '%-20s %s@%s:%s' "${aliases[$i]}" "${users[$i]}" "${hosts[$i]}" "${ports[$i]}")")
            UI_PICK_DESCS+=("Enter: işaretle / işareti kaldır.${keys[$i]:+ Key: ${keys[$i]}}")
        done
        labels+=("☑ Tümünü işaretle / temizle"); UI_PICK_DESCS+=("Hepsini işaretler; hepsi işaretliyse işaretleri kaldırır.")
        labels+=("✅ İçe aktar ($nsel sunucu)"); UI_PICK_DESCS+=("İşaretli sunucuları servers/ altına yazar. Private key dosyaları kopyalanmaz.")
        clear
        ui_screen "Ana menü › ~/.ssh/config'ten içe aktar"
        if ! { declare -F ui_card_on >/dev/null && ui_card_on; }; then echo ""; fi
        # shellcheck disable=SC2034
        UI_PICK_BACK_DESC="Hiçbir şey içe aktarılmadan döner."
        pick=$(UI_PICK_START="$cursor" ui_pick "İptal" "${labels[@]}")
        if [ "${pick:-0}" = 0 ]; then echo "İptal edildi."; exit 0; fi
        if [ "$pick" -le "${#aliases[@]}" ]; then
            i=$((pick - 1)); if [ "${marks[$i]}" = 1 ]; then marks[i]=0; else marks[i]=1; fi
            cursor="$pick"
        elif [ "$pick" -eq "$(( ${#aliases[@]} + 1 ))" ]; then
            if [ "$nsel" -eq "${#aliases[@]}" ]; then v=0; else v=1; fi
            for i in "${!marks[@]}"; do marks[i]=$v; done
            cursor="$pick"
        else
            [ "$nsel" -gt 0 ] || { echo "Önce en az bir sunucu işaretleyin."; sleep 1; cursor="$pick"; continue; }
            for i in "${!marks[@]}"; do [ "${marks[$i]}" = 1 ] && selected+=("$i"); done
            break
        fi
    done
    echo ""
else
    echo "Bulunan sunucular ($CONFIG):"
    for i in "${!aliases[@]}"; do
        printf '  %2d) %-20s %s@%s:%s\n' "$((i + 1))" "${aliases[$i]}" "${users[$i]}" "${hosts[$i]}" "${ports[$i]}"
    done
    echo ""
    read -r -p "İçe aktarılacaklar (örn: 1,3  |  a = hepsi  |  Enter = iptal): " pick
    [ -n "$pick" ] || { echo "İptal edildi."; exit 0; }

    if [ "$pick" = "a" ] || [ "$pick" = "A" ]; then
        for i in "${!aliases[@]}"; do selected+=("$i"); done
    else
        IFS=', ' read -r -a nums <<< "$pick"
        for n in "${nums[@]}"; do
            if [[ "$n" =~ ^[0-9]+$ ]] && [ "$n" -ge 1 ] && [ "$n" -le "${#aliases[@]}" ]; then
                selected+=("$((n - 1))")
            else
                echo "⚠️  Geçersiz seçim atlandı: $n"
            fi
        done
    fi
fi

imported=0
for i in ${selected[@]+"${selected[@]}"}; do
    name=$(ssh_config_module_name "${aliases[$i]}")
    if [ -e "$SERVERS_DIR/$name" ] || [ -e "$LEGACY_DIR/$name" ]; then
        echo "⏭️  $name zaten var, atlandı"; continue
    fi
    key="${keys[$i]}"
    if server_write_files "$SERVERS_DIR" "$name" "${hosts[$i]}" "${users[$i]}" "${ports[$i]}" "$key"; then
        echo "✅ $name ($(module_prefix "$name"))${key:+  key: $key}"
        imported=$((imported + 1))
    else
        echo "⚠️  $name içe aktarılamadı (geçersiz değer)"
    fi
done

echo ""
echo "$imported sunucu eklendi. Passphrase'i Keychain'e eklemek için (varsa):"
echo "  security add-generic-password -U -a USBMonitor_<ÖNEK>_Passphrase -s USBMonitor -w"
