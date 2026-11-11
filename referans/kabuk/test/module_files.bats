#!/usr/bin/env bats
load test_helper
setup() { load_helpers; MD="$BATS_TEST_TMPDIR/Modules"; mkdir -p "$MD"; }

@test "modül dosyaları üretilir" {
    run module_write_files "$MD" "web1" "192.0.2.5" "deploy" "2222"
    [ "$status" -eq 0 ]
    [ -f "$MD/web1/.env" ]
    [ -f "$MD/web1/.env.example" ]
    [ -x "$MD/web1/Web1Module.sh" ]
}
@test ".env içeriği ve izinleri (600)" {
    module_write_files "$MD" "web1" "192.0.2.5" "deploy" "2222"
    run _ssh_env_get "$MD/web1/.env" WEB1_IP;   [ "$output" = "192.0.2.5" ]
    run _ssh_env_get "$MD/web1/.env" WEB1_USER; [ "$output" = "deploy" ]
    run _ssh_env_get "$MD/web1/.env" WEB1_PORT; [ "$output" = "2222" ]
    # platformdan bağımsız: ls -l çıktısının ilk sütunu (macOS ve Linux'ta aynı biçim)
    perms=$(ls -l "$MD/web1/.env" | cut -c1-10)
    [ "$perms" = "-rw-------" ]
}
@test ".env.example gizli bilgi/gerçek değer içermez" {
    module_write_files "$MD" "web1" "192.0.2.5" "deploy" "2222"
    run ! grep -q '192.0.2.5' "$MD/web1/.env.example"
    run ! grep -q 'deploy' "$MD/web1/.env.example"
}
@test "özel key yolu .env'e yazılır" {
    module_write_files "$MD" "web1" "h" "u" "22" "/home/x/.ssh/id_ed25519"
    run _ssh_env_get "$MD/web1/.env" WEB1_KEY
    [ "$output" = "/home/x/.ssh/id_ed25519" ]
}
@test "üretilen modül geçerli bash ve doğru fonksiyonları tanımlar" {
    module_write_files "$MD" "web1" "h" "u" "22"
    run bash -n "$MD/web1/Web1Module.sh"; [ "$status" -eq 0 ]
    grep -q 'RegisterModules "WEB1" "MainMenu" "Web1Module"' "$MD/web1/Web1Module.sh"
    grep -q '^Web1Module_status()' "$MD/web1/Web1Module.sh"
}
@test "geçersiz girdilerde hiçbir şey yazılmaz" {
    run module_write_files "$MD" "Web1" "h" "u" "22";        [ "$status" -ne 0 ]
    run module_write_files "$MD" "web1" "h" "-oEvil" "22";   [ "$status" -ne 0 ]
    run module_write_files "$MD" "web1" "h" "u" "99999";     [ "$status" -ne 0 ]
    [ ! -e "$MD/web1" ]; [ ! -e "$MD/Web1" ]
}
@test "yardımcı fonksiyonlar" {
    run module_prefix "my_srv"; [ "$output" = "MY_SRV" ]
    run module_func_name "my_srv"; [ "$output" = "My_srvModule" ]
}
@test "ssh_module_status: ayar yoksa ⚪" {
    run ssh_module_status "WEB1" "$MD/yok"
    [ "$output" = "⚪" ]
}

# ---------- server_write_files (yeni biçim) ----------
@test "server_write_files: servers/<ad>/server.env, genel anahtarlar, izinler" {
    SD="$BATS_TEST_TMPDIR/servers"
    run server_write_files "$SD" "web1" "192.0.2.5" "deploy" "2222"
    [ "$status" -eq 0 ]
    grep -qx 'IP=192.0.2.5' "$SD/web1/server.env"
    grep -qx 'USER=deploy' "$SD/web1/server.env"
    grep -qx 'PORT=2222' "$SD/web1/server.env"
    run ! grep -q 'WEB1_' "$SD/web1/server.env"
    [ ! -e "$SD/web1/Web1Module.sh" ]
    [ "$(ls -l "$SD/web1/server.env" | cut -c1-10)" = "-rw-------" ]
    [ "$(ls -ld "$SD" | cut -c1-10)" = "drwx------" ]
    [ "$(ls -ld "$SD/web1" | cut -c1-10)" = "drwx------" ]
}
@test "server_write_files: KEY ve OS yazılır; hedef varsa dokunmaz (3)" {
    SD="$BATS_TEST_TMPDIR/servers"
    server_write_files "$SD" "web1" "h" "u" "22" "/x/key" "ubuntu-26.04"
    grep -qx 'KEY=/x/key' "$SD/web1/server.env"
    grep -qx 'OS=ubuntu-26.04' "$SD/web1/server.env"
    cp "$SD/web1/server.env" "$BATS_TEST_TMPDIR/once"
    run server_write_files "$SD" "web1" "BASKA" "u" "22"
    [ "$status" -eq 3 ]
    cmp "$SD/web1/server.env" "$BATS_TEST_TMPDIR/once"
}
@test "server_write_files: geçersiz girdide hiçbir şey yazılmaz" {
    SD="$BATS_TEST_TMPDIR/servers"
    run server_write_files "$SD" "Web1" "h" "u" "22";       [ "$status" -ne 0 ]
    run server_write_files "$SD" "web1" "h" "-oEvil" "22";  [ "$status" -ne 0 ]
    run server_write_files "$SD" "web1" "h" "u" "99999";    [ "$status" -ne 0 ]
    [ ! -e "$SD/web1" ]; [ ! -e "$SD/Web1" ]
}
