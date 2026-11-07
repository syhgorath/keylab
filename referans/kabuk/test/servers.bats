#!/usr/bin/env bats
# servers/ veri katmanı: yeni biçim (server.env) + eski biçim uyumluluğu.
load test_helper

setup() {
    load_helpers
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/servers"
    mkdir -p "$SSHPOCKET_SERVERS_DIR"
}

mk_new() { # ad [ip] [user] [port]
    mkdir -p "$SSHPOCKET_SERVERS_DIR/$1"
    printf 'IP=%s\nUSER=%s\nPORT=%s\n' "${2:-192.0.2.7}" "${3:-deploy}" "${4:-2222}" > "$SSHPOCKET_SERVERS_DIR/$1/server.env"
}

# ---------- erişim katmanı ----------
@test "yeni biçim: srv_get genel anahtarları okur" {
    mk_new alpha 192.0.2.7 deploy 2222
    d="$SSHPOCKET_SERVERS_DIR/alpha"
    run srv_get "$d" ALPHA IP;   [ "$output" = "192.0.2.7" ]
    run srv_get "$d" ALPHA USER; [ "$output" = "deploy" ]
    run srv_get "$d" ALPHA PORT; [ "$output" = "2222" ]
}
@test "eski biçim: srv_get <ÖNEK>_ anahtarlarını okur" {
    d="$BATS_TEST_TMPDIR/legacy"; mkdir -p "$d"
    printf 'OLD_IP=198.51.100.4\nOLD_USER=pi\nOLD_PORT=22\n' > "$d/.env"
    run srv_get "$d" OLD IP;   [ "$output" = "198.51.100.4" ]
    run srv_get "$d" OLD USER; [ "$output" = "pi" ]
    run ! srv_is_current "$d"
}
@test "yeni biçim önceliklidir (server.env varsa .env yok sayılır)" {
    mk_new alpha 192.0.2.7
    printf 'ALPHA_IP=203.0.113.9\n' > "$SSHPOCKET_SERVERS_DIR/alpha/.env"
    run srv_get "$SSHPOCKET_SERVERS_DIR/alpha" ALPHA IP
    [ "$output" = "192.0.2.7" ]
}
@test "srv_set biçimi korur: yeni → IP=, eski → ÖNEK_IP=" {
    mk_new alpha
    srv_set "$SSHPOCKET_SERVERS_DIR/alpha" ALPHA IP "192.0.2.50"
    grep -qx 'IP=192.0.2.50' "$SSHPOCKET_SERVERS_DIR/alpha/server.env"
    run ! grep -q 'ALPHA_IP' "$SSHPOCKET_SERVERS_DIR/alpha/server.env"
    d="$BATS_TEST_TMPDIR/legacy"; mkdir -p "$d"; printf 'OLD_IP=1.1.1.1\n' > "$d/.env"
    srv_set "$d" OLD IP "198.51.100.8"
    grep -qx 'OLD_IP=198.51.100.8' "$d/.env"
    [ ! -e "$d/server.env" ]
}
@test "srv_get ayar dosyası yoksa hata döner" {
    mkdir -p "$SSHPOCKET_SERVERS_DIR/bos"
    run srv_get "$SSHPOCKET_SERVERS_DIR/bos" BOS IP
    [ "$status" -ne 0 ]
}
@test "servers_dir: ortam değişkeni, yoksa uygulama kökünün yanındaki servers/" {
    run servers_dir; [ "$output" = "$SSHPOCKET_SERVERS_DIR" ]
    unset SSHPOCKET_SERVERS_DIR
    SISTEM_PATH="$ROOT/sistem" run servers_dir
    [ "$output" = "$ROOT/servers" ]
}

# ---------- listeleme ve kayıt ----------
@test "srv_names: yalnızca geçerli adlı, server.env'i olan klasörler" {
    mk_new alpha; mk_new beta_2
    mkdir -p "$SSHPOCKET_SERVERS_DIR/bos"                       # server.env yok
    mkdir -p "$SSHPOCKET_SERVERS_DIR/.gizli"; printf 'IP=1.1.1.1\n' > "$SSHPOCKET_SERVERS_DIR/.gizli/server.env"
    mkdir -p "$SSHPOCKET_SERVERS_DIR/Buyuk"; printf 'IP=1.1.1.1\n' > "$SSHPOCKET_SERVERS_DIR/Buyuk/server.env"
    mkdir -p "$SSHPOCKET_SERVERS_DIR/1sayi"; printf 'IP=1.1.1.1\n' > "$SSHPOCKET_SERVERS_DIR/1sayi/server.env"
    printf 'x' > "$SSHPOCKET_SERVERS_DIR/dosya"
    run srv_names
    [ "$output" = $'alpha\nbeta_2' ]
}
@test "RegisterServers: 'server:<ad>' girdisi, etiket ÖNEK" {
    MODULE_REGISTRY=""; MODULE_COUNT=0
    mk_new alpha
    RegisterServers
    run GetModulesInGroup MainMenu
    [ "$output" = "server:alpha" ]
    run GetModuleLabel "server:alpha"
    [ "$output" = "ALPHA" ]
}
@test "RegisterModules: aynı etiket+grup yinelenmez, ilk kayıt kazanır" {
    MODULE_REGISTRY=""; MODULE_COUNT=0
    RegisterModules "ALPHA" "MainMenu" "server:alpha"
    RegisterModules "ALPHA" "MainMenu" "AlphaModule"
    run GetModulesInGroup MainMenu
    [ "$output" = "server:alpha" ]
    RegisterModules "ALPHA" "Baska" "X"          # farklı grup: serbest
    run GetModulesInGroup Baska
    [ "$output" = "X" ]
}
@test "EntryExists: server: adı doğrulanır; fonksiyon adı bakılır" {
    EntryExists "server:alpha"
    run ! EntryExists "server:../etc"
    run ! EntryExists "server:Bad"
    run ! EntryExists "YokBoyleBirFonksiyon"
    EntryExists "srv_names"
}

# ---------- durum ----------
@test "server_status: IP yoksa ⚪, kapalı portta 🔴" {
    printf 'USER=u\nPORT=22\n' > /dev/null
    mkdir -p "$SSHPOCKET_SERVERS_DIR/bos"; printf 'USER=u\n' > "$SSHPOCKET_SERVERS_DIR/bos/server.env"
    run server_status bos;  [ "$output" = "⚪" ]
    mk_new kapali 127.0.0.1 u 1
    run server_status kapali; [ "$output" = "🔴" ]
    run server_status "../x"; [ "$output" = "⚪" ]
}
