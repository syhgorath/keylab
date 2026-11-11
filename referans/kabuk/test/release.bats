#!/usr/bin/env bats
# scripts/release.sh: geçici bir git deposunda, test anahtarıyla.
load test_helper

setup() {
    T="$BATS_TEST_TMPDIR"
    R="$T/repo"
    mkdir -p "$R/scripts" "$R/sistem" "$R/test"
    cp "$ROOT/scripts/release.sh" "$R/scripts/"
    printf '9.9.9\n' > "$R/VERSION"
    printf '## [9.9.9] - 2026-01-01\n- x\n' > "$R/CHANGELOG.md"
    printf 'kod\n' > "$R/sistem/Main.sh"
    printf 'test dosyasi\n' > "$R/test/x.bats"
    printf '/test/ export-ignore\n/scripts/ export-ignore\n' > "$R/.gitattributes"
    ssh-keygen -q -t ed25519 -N "" -f "$T/key"
    printf 'sshpocket-release namespaces="sshpocket-release" %s\n' "$(cut -d' ' -f1,2 "$T/key.pub")" > "$R/sistem/release_signers"
    git -C "$R" init -q -b main
    git -C "$R" config user.name t; git -C "$R" config user.email t@t
    git -C "$R" add -A; git -C "$R" commit -q -m init
    git -C "$R" tag -a v9.9.9 -m t
    export SSHPOCKET_SIGNING_KEY="$T/key"
}

@test "imzalı arşiv üretir, doğrulanır, test/scripts dışarıda kalır" {
    run bash "$R/scripts/release.sh"
    [ "$status" -eq 0 ]
    [ -f "$R/dist/sshpocket-9.9.9.tar.gz" ]
    [ -f "$R/dist/sshpocket-9.9.9.tar.gz.sig" ]
    ssh-keygen -Y verify -f "$R/sistem/release_signers" -I sshpocket-release -n sshpocket-release \
        -s "$R/dist/sshpocket-9.9.9.tar.gz.sig" < "$R/dist/sshpocket-9.9.9.tar.gz" >/dev/null
    run tar -tzf "$R/dist/sshpocket-9.9.9.tar.gz"
    [[ "$output" == *"sshpocket-9.9.9/VERSION"* ]]
    [[ "$output" != *"/test/"* ]]
    [[ "$output" != *"/scripts/"* ]]
    [[ "$output" != *"dist/"* ]]
}
@test "üretilen paket, güncelleyicinin kontrollerinden geçer" {
    bash "$R/scripts/release.sh" >/dev/null
    # shellcheck source=/dev/null
    source "$ROOT/sistem/Update.sh"
    update_verify "$R/dist/sshpocket-9.9.9.tar.gz" "$R/dist/sshpocket-9.9.9.tar.gz.sig" "$R/sistem/release_signers"
    update_check_archive "$R/dist/sshpocket-9.9.9.tar.gz" 9.9.9
}
@test "etiket yoksa durur" {
    git -C "$R" tag -d v9.9.9 >/dev/null
    run bash "$R/scripts/release.sh"
    [ "$status" -ne 0 ]; [[ "$output" == *"etiket yok"* ]]
    [ ! -e "$R/dist/sshpocket-9.9.9.tar.gz.sig" ]
}
@test "etiket şu anki commit'i göstermiyorsa durur" {
    printf 'yeni\n' >> "$R/sistem/Main.sh"; git -C "$R" commit -qam sonra
    run bash "$R/scripts/release.sh"
    [ "$status" -ne 0 ]; [[ "$output" == *"şu anki commit"* ]]
}
@test "commit edilmemiş değişiklik varsa durur" {
    printf 'kirli\n' >> "$R/sistem/Main.sh"
    run bash "$R/scripts/release.sh"
    [ "$status" -ne 0 ]; [[ "$output" == *"temiz değil"* ]]
}
@test "CHANGELOG'da sürüm başlığı yoksa durur" {
    printf '# boş\n' > "$R/CHANGELOG.md"; git -C "$R" commit -qam c; git -C "$R" tag -f -a v9.9.9 -m t >/dev/null
    run bash "$R/scripts/release.sh"
    [ "$status" -ne 0 ]; [[ "$output" == *"CHANGELOG"* ]]
}
@test "imza anahtarı yoksa durur" {
    SSHPOCKET_SIGNING_KEY="$T/yok" run bash "$R/scripts/release.sh"
    [ "$status" -ne 0 ]; [[ "$output" == *"imza anahtarı bulunamadı"* ]]
}
@test "repodaki signers dosyasında public key yoksa durur" {
    printf '# boş\n' > "$R/sistem/release_signers"; git -C "$R" commit -qam s; git -C "$R" tag -f -a v9.9.9 -m t >/dev/null
    run bash "$R/scripts/release.sh"
    [ "$status" -ne 0 ]; [[ "$output" == *"public key yok"* ]]
}
@test "yanlış anahtarla imzalamaya çalışılırsa doğrulamada durur (yanlış paket yayınlanmaz)" {
    ssh-keygen -q -t ed25519 -N "" -f "$T/baska"
    SSHPOCKET_SIGNING_KEY="$T/baska" run bash "$R/scripts/release.sh"
    [ "$status" -ne 0 ]; [[ "$output" == *"DOĞRULANAMADI"* ]]
}
