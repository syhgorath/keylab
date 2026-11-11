#!/usr/bin/env bats
# Eski paket sürümlerinin temizliği: güncelleme sonrası sorulur (UpdateAction) ve Ayarlar'dan yapılır. Sahte Main.sh/Update.sh, gerçek klasöre dokunulmaz.
load test_helper

setup() {
    T="$BATS_TEST_TMPDIR"
    export SISTEM_PATH="$T/sistem"; mkdir -p "$SISTEM_PATH" "$T/log"
    export CALL_LOG="$T/calls.log"; : > "$CALL_LOG"
    printf '3.6.13\n' > "$T/VERSION"
    # sahte Main.sh: aldığı bağımsız değişkenleri kaydeder (exec ile yeniden başlatma da buraya gelir)
    cat > "$SISTEM_PATH/Main.sh" <<'STUB'
#!/bin/bash
echo "main:$*" >> "$CALL_LOG"
[ "$1" = "--components" ] && [ "$2" = "prune" ] && echo "  linux-hardening: silinecek: 0.7.0"
exit 0
STUB
    # sahte Update.sh: sürümü yükseltir
    cat > "$SISTEM_PATH/Update.sh" <<'STUB'
#!/bin/bash
echo "update" >> "$CALL_LOG"; printf '3.6.14\n' > "$(dirname "$0")/../VERSION"
STUB
    export SSHPOCKET_CONF="$T/c.conf"
}

@test "UpdateAction: sürüm değişince sync'ten SONRA prune çalışır, sonra yeniden başlatılır" {
    run bash -c 'source "'"$H"'/BuiltinMenu.sh"; SCRIPT_VERSION=3.6.13; clear(){ :; }; UpdateAction' <<< $'\n'
    [ "$status" -eq 0 ]
    [[ "$output" == *"Eski paket sürümleri"* && "$output" == *"silinecek: 0.7.0"* ]] || false
    sync_n=$(grep -n "^main:--components sync" "$CALL_LOG" | head -1 | cut -d: -f1)
    prune_n=$(grep -n "^main:--components prune" "$CALL_LOG" | head -1 | cut -d: -f1)
    restart_n=$(grep -nx "main:" "$CALL_LOG" | head -1 | cut -d: -f1)
    [ -n "$sync_n" ] && [ -n "$prune_n" ] && [ -n "$restart_n" ]
    [ "$sync_n" -lt "$prune_n" ]; [ "$prune_n" -lt "$restart_n" ]
}

@test "UpdateAction: sürüm değişmediyse (güncel/iptal) prune sorulmaz" {
    printf '#!/bin/bash\necho "update" >> "$CALL_LOG"\n' > "$SISTEM_PATH/Update.sh"
    run bash -c 'source "'"$H"'/BuiltinMenu.sh"; SCRIPT_VERSION=3.6.13; clear(){ :; }; UpdateAction' <<< $'\n'
    [ "$status" -eq 0 ]
    run grep -c "prune" "$CALL_LOG"; [ "$output" = "0" ]
}

@test "Ayarlar: 4 numaralı seçenek eski paket sürümlerini temizleme komutunu çalıştırır ve menüye döner" {
    run bash -c 'source "'"$H"'/Settings.sh"; clear(){ :; }; SettingsAction' <<< $'4\n\nq'
    [ "$status" -eq 0 ]
    [[ "$output" == *"4) Eski paket sürümlerini temizle"* ]] || false
    [[ "$output" == *"silinecek: 0.7.0"* ]] || false
    grep -qx "main:--components prune" "$CALL_LOG"
}

@test "Ayarlar: temizleme seçilmezse (yalnızca q) prune çalışmaz" {
    run bash -c 'source "'"$H"'/Settings.sh"; clear(){ :; }; SettingsAction' <<< $'q'
    [ "$status" -eq 0 ]
    run grep -c "prune" "$CALL_LOG"; [ "$output" = "0" ]
}
