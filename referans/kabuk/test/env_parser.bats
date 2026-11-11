#!/usr/bin/env bats
load test_helper
setup() { load_helpers; ENVF="$BATS_TEST_TMPDIR/.env"; }

@test "basit KEY=VALUE" {
    printf 'A_IP=10.0.0.5\nA_USER=pi\n' > "$ENVF"
    run _ssh_env_get "$ENVF" A_IP; [ "$output" = "10.0.0.5" ]
    run _ssh_env_get "$ENVF" A_USER; [ "$output" = "pi" ]
}
@test "boşluklar ve tırnaklar temizlenir" {
    printf '  A_IP = "10.0.0.5" \nA_USER='"'"'pi'"'"'\n' > "$ENVF"
    run _ssh_env_get "$ENVF" A_IP; [ "$output" = "10.0.0.5" ]
    run _ssh_env_get "$ENVF" A_USER; [ "$output" = "pi" ]
}
@test "yorum satırları ve satır sonu yorumları" {
    printf '# A_IP=1.1.1.1\n   # A_IP=2.2.2.2\nA_IP=10.0.0.5\nA_PORT=2222 # not\n' > "$ENVF"
    run _ssh_env_get "$ENVF" A_IP;   [ "$output" = "10.0.0.5" ]
    run _ssh_env_get "$ENVF" A_PORT; [ "$output" = "2222" ]
}
@test "değerde = karakteri korunur" {
    printf 'A_NOTE=a=b=c\n' > "$ENVF"
    run _ssh_env_get "$ENVF" A_NOTE; [ "$output" = "a=b=c" ]
}
@test "olmayan anahtar boş ve hata döner" {
    printf 'A_IP=1\n' > "$ENVF"
    run _ssh_env_get "$ENVF" YOK
    [ "$status" -ne 0 ]; [ -z "$output" ]
}
@test "son satır newline'sız olsa da okunur" {
    printf 'A_IP=10.0.0.9' > "$ENVF"
    run _ssh_env_get "$ENVF" A_IP; [ "$output" = "10.0.0.9" ]
}
@test "değer komut olarak çalıştırılmaz (eval yok)" {
    printf 'A_IP=$(touch %s/pwned)\n' "$BATS_TEST_TMPDIR" > "$ENVF"
    run _ssh_env_get "$ENVF" A_IP
    [ ! -e "$BATS_TEST_TMPDIR/pwned" ]
}
