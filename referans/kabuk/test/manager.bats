#!/usr/bin/env bats
# ModuleManager: yeni biçim (servers/) + eski biçim uyumluluğu
load test_helper
setup() {
    load_helpers
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/servers"
    export SSHPOCKET_MODULES_DIR="$BATS_TEST_TMPDIR/Modules"
    mkdir -p "$SSHPOCKET_SERVERS_DIR" "$SSHPOCKET_MODULES_DIR"
    server_write_files "$SSHPOCKET_SERVERS_DIR" "web1" "192.0.2.5" "deploy" "2222"
    printf 'KEY' > "$SSHPOCKET_SERVERS_DIR/web1/id_ed25519_web1"
    printf 'PUB' > "$SSHPOCKET_SERVERS_DIR/web1/id_ed25519_web1.pub"
    # Keychain'e dokunulmasın
    mkdir -p "$BATS_TEST_TMPDIR/bin"
    printf '#!/bin/bash\nexit 1\n' > "$BATS_TEST_TMPDIR/bin/security"
    chmod +x "$BATS_TEST_TMPDIR/bin/security"
    export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
    MM="$ROOT/sistem/ModuleManager.sh"
}

@test "list: yeni biçimli sunucuları gösterir" {
    run bash "$MM" list
    [[ "$output" == *"web1"* ]]
    [[ "$output" == *"deploy@192.0.2.5:2222"* ]]
}
@test "list: eski biçimli sunucuyu etiketle gösterir (taşıma ipucu)" {
    mkdir -p "$SSHPOCKET_MODULES_DIR/old1"
    printf 'OLD1_IP=198.51.100.4\nOLD1_USER=pi\nOLD1_PORT=22\n' > "$SSHPOCKET_MODULES_DIR/old1/.env"
    run bash "$MM" list
    [[ "$output" == *"old1"*"pi@198.51.100.4:22"*"eski biçim"* ]]
    [[ "$output" == *"web1"* ]]
}
@test "list: aynı adlı eski kopya 'yeni biçim kazanır' notuyla görünür" {
    mkdir -p "$SSHPOCKET_MODULES_DIR/web1"
    printf 'WEB1_IP=203.0.113.9\nWEB1_USER=eski\nWEB1_PORT=22\n' > "$SSHPOCKET_MODULES_DIR/web1/.env"
    run bash "$MM" list
    [[ "$output" == *"eski kopya"* ]]
}
@test "list: boşsa '(sunucu yok)'" {
    rm -r "$SSHPOCKET_SERVERS_DIR/web1"
    run bash "$MM" list
    [[ "$output" == *"(sunucu yok)"* ]]
}
@test "remove: doğru adla onaylanırsa siler" {
    run bash "$MM" remove web1 <<< "web1"
    [ "$status" -eq 0 ]
    [ ! -e "$SSHPOCKET_SERVERS_DIR/web1" ]
}
@test "remove: yanlış onayda silmez" {
    run bash "$MM" remove web1 <<< "hayir"
    [ "$status" -ne 0 ]
    [ -d "$SSHPOCKET_SERVERS_DIR/web1" ]
}
@test "remove: path traversal reddedilir" {
    mkdir -p "$BATS_TEST_TMPDIR/hedef"
    run bash "$MM" remove "../hedef" <<< "../hedef"
    [ "$status" -ne 0 ]
    [ -d "$BATS_TEST_TMPDIR/hedef" ]
}
@test "remove: eski biçimli sunucuyu da siler" {
    mkdir -p "$SSHPOCKET_MODULES_DIR/old1"; printf 'OLD1_IP=1.1.1.1\n' > "$SSHPOCKET_MODULES_DIR/old1/.env"
    run bash "$MM" remove old1 <<< "old1"
    [ "$status" -eq 0 ]
    [ ! -e "$SSHPOCKET_MODULES_DIR/old1" ]
}
@test "rename: klasörü ve key dosyalarını yeni ada taşır, ayarlar korunur" {
    run bash "$MM" rename web1 web2
    [ "$status" -eq 0 ]
    [ ! -e "$SSHPOCKET_SERVERS_DIR/web1" ]
    [ -f "$SSHPOCKET_SERVERS_DIR/web2/server.env" ]
    [ "$(cat "$SSHPOCKET_SERVERS_DIR/web2/id_ed25519_web2")" = "KEY" ]
    [ "$(cat "$SSHPOCKET_SERVERS_DIR/web2/id_ed25519_web2.pub")" = "PUB" ]
    [ ! -e "$SSHPOCKET_SERVERS_DIR/web2/id_ed25519_web1" ]
    run _ssh_env_get "$SSHPOCKET_SERVERS_DIR/web2/server.env" IP
    [ "$output" = "192.0.2.5" ]
    [[ "$output" != *"WEB1"* ]]
}
@test "rename: Keychain kaydının taşınmadığını söyler (yeni hesap adıyla)" {
    run bash "$MM" rename web1 web2
    [[ "$output" == *"USBMonitor_WEB2_Passphrase"* ]]
}
@test "rename: ad, key adının alt dizesini içerse bile doğru dosyayı değiştirir" {
    rm -r "$SSHPOCKET_SERVERS_DIR/web1"
    server_write_files "$SSHPOCKET_SERVERS_DIR" "ed25519" "192.0.2.5" "u" "22"
    printf 'K' > "$SSHPOCKET_SERVERS_DIR/ed25519/id_ed25519_ed25519"
    run bash "$MM" rename ed25519 yeni
    [ "$status" -eq 0 ]
    [ -f "$SSHPOCKET_SERVERS_DIR/yeni/id_ed25519_yeni" ]
    [ ! -e "$SSHPOCKET_SERVERS_DIR/yeni/id_ed25519_ed25519" ]
}
@test "rename: hedef varsa, ad geçersizse veya sunucu eski biçimdeyse dokunmaz" {
    server_write_files "$SSHPOCKET_SERVERS_DIR" "web2" "h" "u" "22"
    run bash "$MM" rename web1 web2;  [ "$status" -ne 0 ]
    run bash "$MM" rename web1 "Bad"; [ "$status" -ne 0 ]
    [ -d "$SSHPOCKET_SERVERS_DIR/web1" ]
    mkdir -p "$SSHPOCKET_MODULES_DIR/old1"; printf 'OLD1_IP=1.1.1.1\n' > "$SSHPOCKET_MODULES_DIR/old1/.env"
    run bash "$MM" rename old1 yeni
    [ "$status" -ne 0 ]
    [[ "$output" == *"--migrate"* ]]
    [ -d "$SSHPOCKET_MODULES_DIR/old1" ]
}
@test "bilinmeyen komut kullanım gösterir" {
    run bash "$MM" nope
    [ "$status" -ne 0 ]
    [[ "$output" == *"Kullanım"* ]]
}

@test "remove: sunucuyla birlikte Keychain'deki knock dizisi de silinir (yetim kalmaz)" {
    SECLOG="$BATS_TEST_TMPDIR/sec.log"
    printf '#!/bin/bash\necho "$*" >> "%s"\nexit 0\n' "$SECLOG" > "$BATS_TEST_TMPDIR/bin/security"
    run bash -c "printf 'web1\nn\n' | bash '$MM' remove web1"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Knock dizisi Keychain'den silindi"* ]]
    grep -q 'delete-generic-password -a USBMonitor_WEB1_Knock -s USBMonitor' "$SECLOG"
    [ ! -d "$SSHPOCKET_SERVERS_DIR/web1" ]
}

@test "remove: ad doğrulanmazsa knock kaydına dokunulmaz" {
    SECLOG="$BATS_TEST_TMPDIR/sec.log"
    printf '#!/bin/bash\necho "$*" >> "%s"\nexit 0\n' "$SECLOG" > "$BATS_TEST_TMPDIR/bin/security"
    run bash -c "printf 'yanlis\n' | bash '$MM' remove web1"
    [ "$status" -ne 0 ]
    [ ! -e "$SECLOG" ]
    [ -d "$SSHPOCKET_SERVERS_DIR/web1" ]
}

@test "rename: eski adla duran knock dizisi için yeniden girme uyarısı verir" {
    printf '#!/bin/bash\nexit 0\n' > "$BATS_TEST_TMPDIR/bin/security"
    run bash "$MM" rename web1 web2
    [ "$status" -eq 0 ]
    [[ "$output" == *"Knock dizisi de eski adla"* ]]
    [[ "$output" == *"Knock ayarları"* ]]
}
