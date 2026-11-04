#!/usr/bin/env bats
load test_helper
setup() {
    # shellcheck source=/dev/null
    source "$H/SSHConfigImport.sh"
    CFG="$BATS_TEST_TMPDIR/config"
    cat > "$CFG" <<'CONF'
# yorum
Host *
    ServerAliveInterval 30

Host web1 web2
    HostName 192.0.2.10
    User deploy
    Port 2200
    IdentityFile ~/.ssh/id_web

Host=db
    HostName=db.example.com
    user = admin

Host bare

Host *.internal gate?
    User nobody

Match host foo
    User matched

Host after-match
    HostName 192.0.2.99
CONF
}

@test "joker içeren Host kayıtları atlanır" {
    run ssh_config_hosts "$CFG"
    [[ "$output" != *"*"* ]]
    [[ "$output" != *"nobody"* ]]
}
@test "birden çok alias ayrı satır olur" {
    run ssh_config_hosts "$CFG"
    [[ "$output" == *"web1|192.0.2.10|deploy|2200|~/.ssh/id_web"* ]]
    [[ "$output" == *"web2|192.0.2.10|deploy|2200|~/.ssh/id_web"* ]]
}
@test "'=' sözdizimi ve büyük/küçük harf" {
    run ssh_config_hosts "$CFG"
    [[ "$output" == *"db|db.example.com|admin||"* ]]
}
@test "HostName yoksa alias kullanılır" {
    run ssh_config_hosts "$CFG"
    [[ "$output" == *"bare|bare|||"* ]]
}
@test "Match bloğunun ayarları sızmaz" {
    run ssh_config_hosts "$CFG"
    [[ "$output" != *"matched"* ]]
    [[ "$output" == *"after-match|192.0.2.99|||"* ]]
}
@test "modül adı üretimi" {
    run ssh_config_module_name "Web-1";   [ "$output" = "web_1" ]
    run ssh_config_module_name "1host";   [ "$output" = "h_1host" ]
    run ssh_config_module_name "my.srv";  [ "$output" = "my_srv" ]
}
@test "CLI: seçilenleri servers/ altına yeni biçimde içe aktarır, mevcut olanı atlar" {
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/servers"
    export SSHPOCKET_MODULES_DIR="$BATS_TEST_TMPDIR/Modules"
    mkdir -p "$SSHPOCKET_SERVERS_DIR/db"
    run bash "$ROOT/sistem/ImportSSHConfig.sh" "$CFG" <<< "a"
    [ "$status" -eq 0 ]
    [ -f "$SSHPOCKET_SERVERS_DIR/web1/server.env" ]
    [ -f "$SSHPOCKET_SERVERS_DIR/web2/server.env" ]
    [[ "$output" == *"db zaten var"* ]]
    # üretilmiş modül dosyası YOK; genel anahtarlar; private key kopyalanmaz, yol KEY olarak yazılır
    [ ! -e "$SSHPOCKET_SERVERS_DIR/web1/Web1Module.sh" ]
    grep -qx 'KEY=~/.ssh/id_web' "$SSHPOCKET_SERVERS_DIR/web1/server.env"
    grep -qx 'IP=192.0.2.10' "$SSHPOCKET_SERVERS_DIR/web1/server.env"
    [ -z "$(ls "$SSHPOCKET_SERVERS_DIR/web1" | grep id_ed25519 || true)" ]
}
@test "CLI: eski biçimde aynı ad varsa da atlanır" {
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/servers"
    export SSHPOCKET_MODULES_DIR="$BATS_TEST_TMPDIR/Modules"
    mkdir -p "$SSHPOCKET_MODULES_DIR/web1"; printf 'WEB1_IP=1.1.1.1\n' > "$SSHPOCKET_MODULES_DIR/web1/.env"
    run bash "$ROOT/sistem/ImportSSHConfig.sh" "$CFG" <<< "a"
    [[ "$output" == *"web1 zaten var"* ]]
    [ ! -e "$SSHPOCKET_SERVERS_DIR/web1" ]
}
@test "CLI: Enter iptal eder" {
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/servers"
    mkdir -p "$SSHPOCKET_SERVERS_DIR"
    run bash "$ROOT/sistem/ImportSSHConfig.sh" "$CFG" <<< ""
    [[ "$output" == *"İptal"* ]]
    [ -z "$(ls -A "$SSHPOCKET_SERVERS_DIR")" ]
}
@test "CLI: geçersiz hostname içeren kayıt atlanır" {
    printf 'Host evil\n    HostName a$(id)\n' > "$CFG"
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/servers"
    mkdir -p "$SSHPOCKET_SERVERS_DIR"
    run bash "$ROOT/sistem/ImportSSHConfig.sh" "$CFG" <<< "a"
    [[ "$output" == *"içe aktarılamadı"* ]]
    [ ! -e "$SSHPOCKET_SERVERS_DIR/evil" ]
}

# ---------- ok tuşlu / fzf çoklu seçim (UI=arrows; tuşlar stdin'den: numara + Enter) ----------
two_hosts() {
    printf 'Host a1\n    HostName 192.0.2.1\n    User u\nHost a2\n    HostName 192.0.2.2\n    User u\n' > "$CFG"
    export SSHPOCKET_SERVERS_DIR="$BATS_TEST_TMPDIR/servers" SSHPOCKET_MODULES_DIR="$BATS_TEST_TMPDIR/Modules" SSHPOCKET_UI=arrows TERM=dumb
    mkdir -p "$SSHPOCKET_SERVERS_DIR"
}

@test "ok tuşlu: işaretlenen sunucu içe aktarılır, işaretlenmeyen aktarılmaz" {
    two_hosts
    run bash "$ROOT/sistem/ImportSSHConfig.sh" "$CFG" < <(printf '1\n4\n')
    [ "$status" -eq 0 ]
    [[ "$output" == *"[x] a1"* && "$output" == *"1 modül oluşturuldu"* ]] || { echo "$output"; false; }
    [ -f "$SSHPOCKET_SERVERS_DIR/a1/server.env" ]
    [ ! -e "$SSHPOCKET_SERVERS_DIR/a2" ]
}

@test "ok tuşlu: 'Tümünü işaretle' hepsini aktarır" {
    two_hosts
    run bash "$ROOT/sistem/ImportSSHConfig.sh" "$CFG" < <(printf '3\n4\n')
    [ -f "$SSHPOCKET_SERVERS_DIR/a1/server.env" ] && [ -f "$SSHPOCKET_SERVERS_DIR/a2/server.env" ]
}

@test "ok tuşlu: hiçbiri işaretli değilken içe aktar uyarır, 0 iptal eder ve hiçbir şey yazılmaz" {
    two_hosts
    run bash "$ROOT/sistem/ImportSSHConfig.sh" "$CFG" < <(printf '4\n0\n')
    [ "$status" -eq 0 ]
    [[ "$output" == *"Önce en az bir sunucu işaretleyin"* && "$output" == *"İptal edildi"* ]] || { echo "$output"; false; }
    [ -z "$(ls "$SSHPOCKET_SERVERS_DIR")" ]
}
