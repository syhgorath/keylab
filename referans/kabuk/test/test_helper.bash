# Ortak test yardımcıları
bats_require_minimum_version 1.5.0
# Kök dizin: bu dosyanın konumundan (test/ ve test/e2e/ altındaki testler için de doğru)
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
H="$ROOT/sistem/Helpers"

load_helpers() {
    # shellcheck source=/dev/null
    source "$H/Validate.sh"
    # shellcheck source=/dev/null
    source "$H/ModuleFiles.sh"
    # shellcheck source=/dev/null
    source "$H/SSHModule.sh"
    # shellcheck source=/dev/null
    source "$H/SSHHelper.sh"
    # shellcheck source=/dev/null
    source "$H/SSHModuleEdit.sh"
    # shellcheck source=/dev/null
    source "$H/RegisterModules.sh"
    # shellcheck source=/dev/null
    source "$H/Servers.sh"
    # shellcheck source=/dev/null
    source "$H/Settings.sh"
    # shellcheck source=/dev/null
    source "$H/UI.sh"
    # shellcheck source=/dev/null
    source "$H/Knock.sh"
}

# İçinde sistem/ kopyası olan izole bir çalışma alanı (eject kapalı, sahte komutlar PATH başında)
make_sandbox() {
    SANDBOX="$BATS_TEST_TMPDIR/sb"
    mkdir -p "$SANDBOX/bin"
    # Durum göstergesi varsayılan açık; testler ağa çıkmasın (testler gerekirse =1 verir)
    export SSHPOCKET_STATUS=off
    export SSHPOCKET_CONF="$BATS_TEST_TMPDIR/sshpocket.conf"
    cp -R "$ROOT/sistem" "$ROOT/start.sh" "$SANDBOX/"
    printf 'EjectUSB(){ :; }\n' > "$SANDBOX/sistem/Helpers/Eject.sh"
}

# file_mode <yol> → sekizlik izin (örn. 600). GNU stat önce: Linux'ta "stat -f" dosya sistemi bilgisi verir (hata kodu dönmez).
file_mode() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }
