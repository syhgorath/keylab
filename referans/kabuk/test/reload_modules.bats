#!/usr/bin/env bats
# ReloadModules: eski sistem/Modules klasörü yoksa (yeni kurulum) stderr'e uyarı basmaz.
load test_helper

setup() {
    load_helpers
    source "$H/../Autoload.sh"
    source "$H/BuiltinMenu.sh"
    export SISTEM_PATH="$BATS_TEST_TMPDIR/sistem"
    mkdir -p "$SISTEM_PATH"
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/srv"
    mkdir -p "$SSHPOCKET_SERVERS_DIR"
}

@test "ReloadModules: Modules klasörü yoksa uyarı yok, çıkış 0" {
    run --separate-stderr ReloadModules
    [ "$status" -eq 0 ]
    [[ "$stderr" != *"Autoload path does not exist"* ]]
}

@test "ReloadModules: Modules klasörü varsa yine yüklenir (eski biçim bozulmaz)" {
    mkdir -p "$SISTEM_PATH/Modules"
    run --separate-stderr ReloadModules
    [ "$status" -eq 0 ]
    [[ "$stderr" != *"Autoload path does not exist"* ]]
}
