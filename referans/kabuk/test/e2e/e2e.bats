#!/usr/bin/env bats
# Gerçek bir sshd'ye karşı uçtan uca testler (Docker gerekir).
#   ./test/e2e/up.sh && bats test/e2e && ./test/e2e/down.sh
# Konteyner yoksa testler atlanır. Varsayılan `bats test` bu klasörü ÇALIŞTIRMAZ.
#
# Not: ssh, kullanıcının ~/.ssh/known_hosts dosyasına [127.0.0.1]:PORT kaydı ekler
# (host key sabit olduğundan yeniden kurulumda uyarı çıkmaz). Temizlik: down.sh --clean
load ../test_helper

setup() {
    load_helpers
    # shellcheck source=/dev/null
    source "$H/SSHKeyCopy.sh"
    E2E_DIR="$ROOT/test/e2e"
    [ -f "$E2E_DIR/.secrets" ] || skip "önce ./test/e2e/up.sh çalıştırın"
    # shellcheck source=/dev/null
    . "$E2E_DIR/.secrets"
    PORT="${E2E_PORT:-2222}"
    nc -z 127.0.0.1 "$PORT" 2>/dev/null || skip "sshd 127.0.0.1:$PORT üzerinde çalışmıyor"

    # Şifre ve host key onayı için SSH_ASKPASS (şifre testin geçici klasöründe değil, ortamda durur)
    ASKPASS="$BATS_TEST_TMPDIR/askpass"
    cat > "$ASKPASS" <<'ASK'
#!/bin/bash
case "$1" in
  *"continue connecting"*) echo yes ;;
  *) printf '%s\n' "${E2E_ASKPASS_PW:-$E2E_PASSWORD}" ;;
esac
ASK
    chmod +x "$ASKPASS"
    export E2E_PASSWORD
    export SSH_ASKPASS="$ASKPASS" SSH_ASKPASS_REQUIRE=force DISPLAY=:0
    # Kullanıcının agent'ındaki key'ler "Too many authentication failures"a yol açmasın
    export SSH_AUTH_SOCK=""

    # Konteyner testler arasında kalıcı: her teste özgü etiket, sayımlar karışmasın
    TAG="e2e-$$-${BATS_TEST_NUMBER}-${RANDOM}"
    KEY="$BATS_TEST_TMPDIR/id_ed25519_e2e"
    ssh-keygen -q -t ed25519 -N "" -C "$TAG it's" -f "$KEY"
}

# key ile (şifresiz, etkileşimsiz) giriş yapılabiliyor mu
key_login() {
    ssh -o BatchMode=yes -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no \
        -o IdentitiesOnly=yes -o ConnectTimeout=8 -i "$KEY" -p "$PORT" tester@127.0.0.1 "$@"
}

@test "başlangıçta key ile giriş yapılamaz" {
    run key_login true
    [ "$status" -ne 0 ]
}

@test "ssh_copy_key: şifreyle key eklenir ve key ile giriş çalışır" {
    run ssh_copy_key tester 127.0.0.1 "$PORT" "$KEY" <<< $'y\n\n'
    [ "$status" -eq 0 ]
    [[ "$output" == *"Key gönderildi"* ]]
    run key_login echo MERHABA
    [ "$status" -eq 0 ]
    [ "$output" = "MERHABA" ]
}

@test "ssh_copy_key: ikinci çalıştırmada 'zaten yetkili' der ve tekrar eklemez" {
    ssh_copy_key tester 127.0.0.1 "$PORT" "$KEY" <<< $'y\n\n' >/dev/null
    run ssh_copy_key tester 127.0.0.1 "$PORT" "$KEY" <<< $'y\n\n'
    [ "$status" -eq 0 ]
    [[ "$output" == *"zaten yetkili"* ]]
    run key_login "grep -cF $TAG ~/.ssh/authorized_keys"
    [ "$output" = "1" ]
}

@test "ssh_copy_key: yanlış şifrede başarısız olur, nedenleri ve elle ekleme komutunu gösterir" {
    E2E_ASKPASS_PW="kesinlikle-yanlis" run ssh_copy_key tester 127.0.0.1 "$PORT" "$KEY" <<< $'y\n\n'
    [ "$status" -ne 0 ]
    [[ "$output" == *"ssh-copy-id başarısız"* ]]
    [[ "$output" == *"Elle ekleme"* ]]
    run key_login true
    [ "$status" -ne 0 ]
}

@test "root: şifreyle girilemez (Ubuntu varsayılanı), root uyarısı ve elle ekleme gösterilir" {
    run ssh_copy_key root 127.0.0.1 "$PORT" "$KEY" <<< $'y\n\n'
    [ "$status" -ne 0 ]
    [[ "$output" == *"Kullanıcı 'root'"* ]]
    [[ "$output" == *"Elle ekleme"* ]]
}

@test "elle ekleme komutu gerçek sunucuda çalışır ve tekrar çalıştırılınca çoğaltmaz" {
    run _ssh_manual_key_help "${KEY}.pub"
    cmd=$(printf '%s\n' "$output" | grep '^mkdir -p ~/.ssh')
    [ -n "$cmd" ]
    # sunucuya şifreyle girip komutu çalıştır (iki kez)
    run ssh -o PreferredAuthentications=password -o PubkeyAuthentication=no -p "$PORT" tester@127.0.0.1 "$cmd"
    [ "$status" -eq 0 ]
    run ssh -o PreferredAuthentications=password -o PubkeyAuthentication=no -p "$PORT" tester@127.0.0.1 "$cmd"
    [ "$status" -eq 0 ]
    run key_login echo TAMAM
    [ "$output" = "TAMAM" ]
    run key_login "grep -cF $TAG ~/.ssh/authorized_keys"
    [ "$output" = "1" ]
    # izinler
    run key_login "stat -c '%a' ~/.ssh ~/.ssh/authorized_keys"
    [ "${lines[0]}" = "700" ]
    [ "${lines[1]}" = "600" ]
}

@test "ssh_module_status: açık port 🟢, kapalı port 🔴" {
    MD="$BATS_TEST_TMPDIR/e2emod"; mkdir -p "$MD"
    printf 'E2E_IP=127.0.0.1\nE2E_USER=tester\nE2E_PORT=%s\n' "$PORT" > "$MD/.env"
    run ssh_module_status E2E "$MD"
    [ "$output" = "🟢" ]
    printf 'E2E_IP=127.0.0.1\nE2E_USER=tester\nE2E_PORT=1\n' > "$MD/.env"
    run ssh_module_status E2E "$MD"
    [ "$output" = "🔴" ]
}
