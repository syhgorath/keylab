#!/usr/bin/env bash
set -uo pipefail
IFS=$'\n\t'

# USB path'i belirle
USB_PATH="$(cd "$(dirname "$0")/.." && pwd)"
SISTEM_PATH="$USB_PATH/sistem"

SCRIPT_NAME="${SCRIPT_NAME:-sshpocket}"
# Sürüm tek yerde: kökteki VERSION dosyası (ortamdan SCRIPT_VERSION ile ezilebilir)
if [ -z "${SCRIPT_VERSION:-}" ]; then
    SCRIPT_VERSION="$(head -n 1 "$USB_PATH/VERSION" 2>/dev/null | tr -d '[:space:]')"
    SCRIPT_VERSION="${SCRIPT_VERSION:-unknown}"
fi

case "${1:-}" in
    --version|-V) echo "sshpocket $SCRIPT_VERSION"; exit 0 ;;
    # Güncelleme ayrı bir süreçte çalışır (kendi dosyalarını değiştirebileceği için exec ile devredilir)
    --update)
        # Çekirdek güncellemesi ayrı süreçte; başarılıysa YENİ kodla paketler kilide göre eşitlenir
        bash "$SISTEM_PATH/Update.sh"; update_rc=$?
        if [ "$update_rc" -eq 0 ]; then exec bash "$SISTEM_PATH/Main.sh" --components sync; fi
        exit "$update_rc" ;;
    --check-update) exec bash "$SISTEM_PATH/Update.sh" --check ;;
esac

USB_NAME=$(basename "$USB_PATH")
USB_UUID="N/A"
if command -v diskutil >/dev/null 2>&1; then
    USB_UUID=$(diskutil info "$USB_PATH" 2>/dev/null | awk -F': *' '/Volume UUID/ {print $2; exit}')
    USB_UUID="${USB_UUID:-N/A}"
fi

# Log dosyası
USB_LOG_FILE="$USB_PATH/usb_logs/system_$(date +%Y%m%d).log"
mkdir -p "$(dirname "$USB_LOG_FILE")"

# Helper'lar (sıra önemli)
# shellcheck source=Helpers/Log.sh
source "$SISTEM_PATH/Helpers/Log.sh"
# shellcheck source=Helpers/RegisterModules.sh
source "$SISTEM_PATH/Helpers/RegisterModules.sh"
# shellcheck source=Helpers/UI.sh
source "$SISTEM_PATH/Helpers/UI.sh"
# shellcheck source=Helpers/Menu.sh
source "$SISTEM_PATH/Helpers/Menu.sh"
# shellcheck source=Helpers/CheckRoot.sh
source "$SISTEM_PATH/Helpers/CheckRoot.sh"
# shellcheck source=Helpers/Eject.sh
source "$SISTEM_PATH/Helpers/Eject.sh"
# shellcheck source=Helpers/SSHHelper.sh
source "$SISTEM_PATH/Helpers/SSHHelper.sh"
# shellcheck source=Helpers/Validate.sh
source "$SISTEM_PATH/Helpers/Validate.sh"
# shellcheck source=Helpers/ModuleFiles.sh
source "$SISTEM_PATH/Helpers/ModuleFiles.sh"
# shellcheck source=Helpers/SSHKeyCopy.sh
source "$SISTEM_PATH/Helpers/SSHKeyCopy.sh"
# shellcheck source=Helpers/SSHModule.sh
source "$SISTEM_PATH/Helpers/SSHModule.sh"
# shellcheck source=Helpers/SSHModuleEdit.sh
source "$SISTEM_PATH/Helpers/SSHModuleEdit.sh"
# shellcheck source=Helpers/Servers.sh
source "$SISTEM_PATH/Helpers/Servers.sh"
# shellcheck source=Helpers/Knock.sh
source "$SISTEM_PATH/Helpers/Knock.sh"
# shellcheck source=Helpers/Migrate.sh
source "$SISTEM_PATH/Helpers/Migrate.sh"
# shellcheck source=Helpers/Settings.sh
source "$SISTEM_PATH/Helpers/Settings.sh"
# shellcheck source=Helpers/Components.sh
source "$SISTEM_PATH/Helpers/Components.sh"
# shellcheck source=Helpers/ComponentsInstall.sh
source "$SISTEM_PATH/Helpers/ComponentsInstall.sh"
# shellcheck source=Helpers/ComponentsManifest.sh
source "$SISTEM_PATH/Helpers/ComponentsManifest.sh"
# shellcheck source=Helpers/ComponentsVenv.sh
source "$SISTEM_PATH/Helpers/ComponentsVenv.sh"
# shellcheck source=Helpers/ComponentsRun.sh
source "$SISTEM_PATH/Helpers/ComponentsRun.sh"
# shellcheck source=Helpers/ComponentsVars.sh
source "$SISTEM_PATH/Helpers/ComponentsVars.sh"
# shellcheck source=Helpers/ComponentsGates.sh
source "$SISTEM_PATH/Helpers/ComponentsGates.sh"
# shellcheck source=Helpers/ComponentsExec.sh
source "$SISTEM_PATH/Helpers/ComponentsExec.sh"
# shellcheck source=Helpers/ComponentsRunCLI.sh
source "$SISTEM_PATH/Helpers/ComponentsRunCLI.sh"
# shellcheck source=Helpers/ComponentsMenu.sh
source "$SISTEM_PATH/Helpers/ComponentsMenu.sh"
# shellcheck source=Helpers/ComponentsOnboard.sh
source "$SISTEM_PATH/Helpers/ComponentsOnboard.sh"
# shellcheck source=Helpers/ComponentsCLI.sh
source "$SISTEM_PATH/Helpers/ComponentsCLI.sh"
# shellcheck source=Helpers/Doctor.sh
source "$SISTEM_PATH/Helpers/Doctor.sh"
# shellcheck source=Autoload.sh
source "$SISTEM_PATH/Autoload.sh"
# shellcheck source=Helpers/BuiltinMenu.sh
source "$SISTEM_PATH/Helpers/BuiltinMenu.sh"

# Taşıma: elle (--migrate [--dry-run] [--yes]) ya da etkileşimli açılışta teklif (onaysız asla)
case "${1:-}" in
    --migrate) shift; run_migration "$@"; exit $? ;;
    --doctor)  shift; doctor_run "$@"; exit $? ;;
    --components) shift; components_cli "$@"; exit $? ;;
esac
if [ -t 0 ] && [ -t 1 ] && [ -z "${1:-}" ] && [ -n "$(migrate_legacy_names)" ]; then
    run_migration || true
    echo ""
    read -r -p "Devam etmek için Enter..." _ || true
fi

# Eski sürümlerden kalan boş 'example' şablonunu (içinde kullanıcı verisi yoksa) temizle
migrate_cleanup_stale_example

# servers/ (yeni biçim) önce, sonra eski modüller (aynı adlı eski modül yinelenmez)
RegisterServers

# Eski modülleri yükle (yalnızca henüz taşınmamış eski biçimliler; klasör yoksa sorun değil)
if [ -d "$SISTEM_PATH/Modules" ]; then
    Autoload "$SISTEM_PATH/Modules"
fi
RegisterBuiltinMenu

Log "INFO" "=== Registered Modules ($MODULE_COUNT) ==="
if [ -n "$MODULE_REGISTRY" ]; then
    echo "$MODULE_REGISTRY" | while IFS='|' read -r func label group; do
        Log "INFO" "Function: $func | Label: $label | Group: $group"
    done
fi
echo ""

# --auto / --one-click / -y: MainMenu'deki ilk modülü direkt çalıştır
case "${1:-}" in
    --auto|--one-click|-y)
        first_module=""
        # IFS satır/tab olduğundan, boşlukla ayrılmış listeyi satırlara çevirip oku
        while IFS= read -r m; do
            if [ -n "$m" ] && ! IsBuiltinAction "$m"; then first_module="$m"; break; fi
        done < <(GetModulesInGroup "MainMenu" | tr ' ' '\n')
        if [ -n "$first_module" ] && EntryExists "$first_module"; then
            Log "INFO" "Auto-executing: $first_module"
            RunEntry "$first_module"
            exit $?
        fi
        ;;
esac

# Ana menü döngüsü
while true; do
    clear
    echo -e "\033[0;32m========= ${SCRIPT_NAME} v${SCRIPT_VERSION} =========\033[0m"
    echo "USB: $USB_NAME | UUID: $USB_UUID"
    echo "Path: $USB_PATH"
    echo ""

    selected=$(Menu "MainMenu")
    selected=$(echo -n "$selected" | tr -d '\n\r' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

    if [ -z "$selected" ] || [ "$selected" = "exit" ]; then
        Log "INFO" "Exiting..."
        EjectUSB "$USB_PATH"
        exit 0
    fi

    if EntryExists "$selected"; then
        Log "INFO" "Executing module: $selected"
        # Menüden "geri" ile dönüldüyse okunacak bir çıktı yoktur: duraklatma (eylemler SSHPOCKET_NO_PAUSE=1 verir)
        SSHPOCKET_NO_PAUSE=0
        RunEntry "$selected"
    else
        Log "ERROR" "Function not found: '$selected'"
    fi
    if [ "${SSHPOCKET_NO_PAUSE:-0}" != "1" ]; then
        echo ""
        read -r -p "Devam etmek için Enter..."
    fi
done
