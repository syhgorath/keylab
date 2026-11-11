#!/usr/bin/env bats
# Aşama 2.0: components.lock ayrıştırma ve imza doğrulama (sahte paketler, ağ yok)
load test_helper

setup() {
    source "$H/Components.sh"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_COMPONENTS_LOCK="$T/components.lock"
    # güvenilen anahtar (bileşen imzacısı)
    ssh-keygen -q -t ed25519 -N "" -f "$T/good" -C test
    ssh-keygen -q -t ed25519 -N "" -f "$T/evil" -C evil
    PUB=$(cut -d' ' -f1,2 "$T/good.pub")
    # sahte paket
    mkdir -p "$T/pkg/demo-1.0.0"; printf 'x\n' > "$T/pkg/demo-1.0.0/component.json"
    tar -czf "$T/demo-1.0.0.tar.gz" -C "$T/pkg" demo-1.0.0
    ssh-keygen -q -Y sign -f "$T/good" -n sshpocket-component "$T/demo-1.0.0.tar.gz"
    SHA=$(comp_sha256_of "$T/demo-1.0.0.tar.gz")
}

write_lock() {   # $1 id, $2 sha, $3 signers-pub (ops.)
    cat > "$SSHPOCKET_COMPONENTS_LOCK" <<LOCK
# yorum
[$1]
repo = syhgorath/demo
version = 1.0.0
sha256 = $2
signers = sshpocket-component namespaces="sshpocket-component" ${3:-$PUB}
LOCK
}

@test "kilit: geçerli dosya ayrıştırılır, alanlar okunur" {
    write_lock demo "$SHA"
    run comp_lock_validate
    [ "$status" -eq 0 ]
    [ "$(comp_ids)" = "demo" ]
    [ "$(comp_get demo repo)" = "syhgorath/demo" ]
    [ "$(comp_get demo version)" = "1.0.0" ]
    [ "$(comp_get demo sha256)" = "$SHA" ]
    comp_known demo
    run ! comp_known other
}

@test "kilit: dosya yok ya da yalnızca yorum ise paket yok" {
    run comp_ids
    [ -z "$output" ]
    printf '# boş\n' > "$SSHPOCKET_COMPONENTS_LOCK"
    run comp_lock_validate
    [ "$status" -eq 0 ]
    run ! comp_known demo
}

@test "kilit: geçersiz değerler reddedilir (id, repo, sürüm, sha, signers)" {
    local bad
    for bad in 'ID=Demo' 'ID=../x' 'ID=1abc'; do
        printf '[%s]\n' "${bad#ID=}" > "$SSHPOCKET_COMPONENTS_LOCK"
        run comp_lock_validate; [ "$status" -ne 0 ]
    done
    for line in 'repo = ../etc/passwd' 'repo = a/b/c' 'repo = a b/c' 'repo = a/..' \
                'version = 1.0' 'version = 1.0.0-rc' 'sha256 = ABC' "sha256 = ${SHA:0:63}" \
                'signers = ssh-ed25519 AAAA' 'bilinmeyen = 1'; do
        printf '[demo]\n%s\n' "$line" > "$SSHPOCKET_COMPONENTS_LOCK"
        run comp_lock_validate
        [ "$status" -ne 0 ]
    done
}

@test "kilit: signers yanlış principal/ad alanı/anahtar türü reddedilir" {
    for s in "sshpocket-release namespaces=\"sshpocket-component\" $PUB" \
             "sshpocket-component namespaces=\"sshpocket-release\" $PUB" \
             "sshpocket-component namespaces=\"sshpocket-component\" ssh-rsa AAAA" \
             "sshpocket-component namespaces=\"sshpocket-component,sshpocket-release\" $PUB"; do
        printf '[demo]\nrepo = a/b\nversion = 1.0.0\nsha256 = %s\nsigners = %s\n' "$SHA" "$s" > "$SSHPOCKET_COMPONENTS_LOCK"
        run comp_lock_validate
        [ "$status" -ne 0 ]
    done
}

@test "kilit: yinelenen paket/anahtar ve başlıksız satır reddedilir" {
    write_lock demo "$SHA"; write_lock demo "$SHA" >> /dev/null
    { cat "$SSHPOCKET_COMPONENTS_LOCK"; printf '[demo]\nrepo = a/b\n'; } > "$T/x" && mv "$T/x" "$SSHPOCKET_COMPONENTS_LOCK"
    run comp_lock_validate; [ "$status" -ne 0 ]
    printf 'repo = a/b\n' > "$SSHPOCKET_COMPONENTS_LOCK"
    run comp_lock_validate; [ "$status" -ne 0 ]
    printf '[demo]\nrepo = a/b\nrepo = c/d\n' > "$SSHPOCKET_COMPONENTS_LOCK"
    run comp_lock_validate; [ "$status" -ne 0 ]
}

@test "kilit: eksik zorunlu anahtar reddedilir" {
    printf '[demo]\nrepo = a/b\nversion = 1.0.0\n' > "$SSHPOCKET_COMPONENTS_LOCK"
    run comp_lock_validate
    [ "$status" -ne 0 ]
}

@test "kilit: değerler komut olarak çalışmaz (source edilmez)" {
    printf '[demo]\nrepo = a/b$(touch %s/pwned)\n' "$T" > "$SSHPOCKET_COMPONENTS_LOCK"
    run comp_lock_validate
    [ "$status" -ne 0 ]
    [ ! -e "$T/pwned" ]
}

@test "verify: doğru özet + doğru imza kabul edilir" {
    write_lock demo "$SHA"
    run comp_verify demo "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig"
    [ "$status" -eq 0 ]
}

@test "verify: kilitte olmayan paket reddedilir" {
    write_lock demo "$SHA"
    run comp_verify baska "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig"
    [ "$status" -ne 0 ]
    [[ "$output" == *"kilit dosyasında yok"* ]]
}

@test "verify: arşiv imzadan sonra değiştirilirse reddedilir (özet uyuşmaz)" {
    write_lock demo "$SHA"
    printf 'oops' >> "$T/demo-1.0.0.tar.gz"
    run comp_verify demo "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig"
    [ "$status" -ne 0 ]
    [[ "$output" == *"özet"* ]]
}

@test "verify: özet doğru ama imza başka anahtarla atılmışsa reddedilir" {
    cp "$T/demo-1.0.0.tar.gz" "$T/evil.tar.gz"
    ssh-keygen -q -Y sign -f "$T/evil" -n sshpocket-component "$T/evil.tar.gz"
    write_lock demo "$SHA"                       # kilit good anahtarını bekler
    run comp_verify demo "$T/evil.tar.gz" "$T/evil.tar.gz.sig"
    [ "$status" -ne 0 ]
    [[ "$output" == *"imza doğrulanamadı"* ]]
}

@test "verify: çekirdek ad alanıyla (sshpocket-release) atılmış imza bileşen olarak kabul EDİLMEZ (replay)" {
    cp "$T/demo-1.0.0.tar.gz" "$T/r.tar.gz"
    ssh-keygen -q -Y sign -f "$T/good" -n sshpocket-release "$T/r.tar.gz"
    write_lock demo "$SHA"
    run comp_verify demo "$T/r.tar.gz" "$T/r.tar.gz.sig"
    [ "$status" -ne 0 ]
}

@test "verify: özet kilitle farklıysa (yanlış sürüm paketi) imza geçerli olsa bile reddedilir" {
    write_lock demo "$(printf '0%.0s' $(seq 1 64))"
    run comp_verify demo "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig"
    [ "$status" -ne 0 ]
}

@test "verify: imza dosyası yok ya da boşsa reddedilir" {
    write_lock demo "$SHA"
    : > "$T/empty.sig"
    run comp_verify demo "$T/demo-1.0.0.tar.gz" "$T/empty.sig"
    [ "$status" -ne 0 ]
    run comp_verify demo "$T/demo-1.0.0.tar.gz" "$T/yok.sig"
    [ "$status" -ne 0 ]
}

@test "comp_get: pipefail altında da çalışır (erken çıkış SIGPIPE/141 üretmez)" {
    write_lock demo "$SHA"
    # birden çok girdi: eşleşme ilk girdide, üretici geri kalanı yazmaya çalışır
    for i in 1 2 3 4 5 6 7 8; do
        printf '\n[pkg%s]\nrepo = a/b\nversion = 1.0.0\nsha256 = %s\nsigners = sshpocket-component namespaces="sshpocket-component" %s\n' "$i" "$SHA" "$PUB" >> "$SSHPOCKET_COMPONENTS_LOCK"
    done
    set -o pipefail
    run comp_get demo version
    [ "$status" -eq 0 ]
    [ "$output" = "1.0.0" ]
    run comp_get yok version
    [ -z "$output" ]
}
