#!/usr/bin/env bats
load test_helper
setup() {
    load_helpers
    # shellcheck source=/dev/null
    source "$H/SSHKeyCopy.sh"
    PUB="$BATS_TEST_TMPDIR/k.pub"
    FAKEHOME="$BATS_TEST_TMPDIR/home"
    mkdir -p "$FAKEHOME"
}

# Yardım çıktısındaki tek satırlık komutu alır
extract_cmd() { printf '%s\n' "$1" | grep '^mkdir -p ~/.ssh'; }

@test "yardım: public key satırını içeren tek satırlık komut üretir" {
    printf 'ssh-ed25519 AAAAC3Nza TESTKEY\n' > "$PUB"
    run _ssh_manual_key_help "$PUB"
    [[ "$output" == *"ssh-ed25519 AAAAC3Nza TESTKEY"* ]]
    [[ "$output" == *"authorized_keys"* ]]
}
@test "yardım: üretilen komut gerçekten çalışır, izinler doğru" {
    printf 'ssh-ed25519 AAAAC3Nza TESTKEY\n' > "$PUB"
    run _ssh_manual_key_help "$PUB"
    cmd=$(extract_cmd "$output")
    HOME="$FAKEHOME" run bash -c "$cmd"
    [ "$status" -eq 0 ]
    [ "$(cat "$FAKEHOME/.ssh/authorized_keys")" = "ssh-ed25519 AAAAC3Nza TESTKEY" ]
    [ "$(ls -ld "$FAKEHOME/.ssh" | cut -c1-10)" = "drwx------" ]
    [ "$(ls -l "$FAKEHOME/.ssh/authorized_keys" | cut -c1-10)" = "-rw-------" ]
}
@test "yardım: komut tekrar çalıştırılınca satırı çoğaltmaz, mevcut key'ler korunur" {
    mkdir -p "$FAKEHOME/.ssh"
    printf 'ssh-ed25519 OLDKEY eski\n' > "$FAKEHOME/.ssh/authorized_keys"
    printf 'ssh-ed25519 AAAAC3Nza TESTKEY\n' > "$PUB"
    run _ssh_manual_key_help "$PUB"
    cmd=$(extract_cmd "$output")
    HOME="$FAKEHOME" bash -c "$cmd"
    HOME="$FAKEHOME" bash -c "$cmd"
    [ "$(grep -c 'AAAAC3Nza' "$FAKEHOME/.ssh/authorized_keys")" -eq 1 ]
    grep -q 'OLDKEY' "$FAKEHOME/.ssh/authorized_keys"
}
@test "yardım: yorumda tek tırnak ve \$() olsa bile komut enjeksiyonu olmaz" {
    printf "ssh-ed25519 AAAAC3Nza it's\$(touch %s/pwned)\n" "$BATS_TEST_TMPDIR" > "$PUB"
    run _ssh_manual_key_help "$PUB"
    cmd=$(extract_cmd "$output")
    HOME="$FAKEHOME" run bash -c "$cmd"
    [ "$status" -eq 0 ]
    [ ! -e "$BATS_TEST_TMPDIR/pwned" ]
    # dosyaya tam olarak orijinal satır yazılmış olmalı
    [ "$(cat "$FAKEHOME/.ssh/authorized_keys")" = "$(cat "$PUB")" ]
}
