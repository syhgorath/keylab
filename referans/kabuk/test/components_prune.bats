#!/usr/bin/env bats
# --components prune: eski sürüm klasörlerini temizler (etkin + bir önceki + kilitteki kalır). Sahte klasörler, ağ yok.
load test_helper

setup() {
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsManifest.sh"
    source "$H/ComponentsCLI.sh"
    export SISTEM_PATH="$ROOT/sistem"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_COMPONENTS_LOCK="$T/components.lock"
    export SSHPOCKET_COMPONENTS_DIR="$T/components"
    ssh-keygen -q -t ed25519 -N "" -f "$T/good" -C test
    PUB=$(cut -d' ' -f1,2 "$T/good.pub")
}

# set_lock <sürüm>
set_lock() {
    printf '[demo]\nrepo = syhgorath/demo\nversion = %s\nsha256 = %s\nsigners = sshpocket-component namespaces="sshpocket-component" %s\n' \
        "$1" "$(printf 'a%.0s' $(seq 1 64))" "$PUB" > "$SSHPOCKET_COMPONENTS_LOCK"
}

# mkvers <sürüm>...   her biri için klasör + dosya + .files; current = sonuncu DEĞİL, CUR değişkeni
mkvers() {
    local v
    for v in "$@"; do
        mkdir -p "$T/components/demo/$v/playbooks"
        printf 'x' > "$T/components/demo/$v/component.json"
        printf 'özet\n' > "$T/components/demo/$v.files"
    done
}

present() { [ -d "$T/components/demo/$1" ] && [ -f "$T/components/demo/$1.files" ]; }
gone() { [ ! -e "$T/components/demo/$1" ] && [ ! -e "$T/components/demo/$1.files" ]; }

@test "prune: etkin + bir önceki + kilitteki kalır, gerisi (klasör ve .files) silinir" {
    mkvers 0.9.0 1.0.0 1.1.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 1.2.0
    run comp_cli_prune --yes
    [ "$status" -eq 0 ]
    gone 0.9.0
    gone 1.0.0
    present 1.1.0
    present 1.2.0
    [ "$(cat "$T/components/demo/current")" = "1.2.0" ]
    [[ "$output" == *"silindi: demo 0.9.0"* ]] || false
    [[ "$output" == *"silindi: demo 1.0.0"* ]] || false
}

@test "prune: --dry-run yalnızca listeler, hiçbir şeyi silmez" {
    mkvers 0.9.0 1.0.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 1.2.0
    run comp_cli_prune --dry-run
    [ "$status" -eq 0 ]
    [[ "$output" == *"silinecek: 0.9.0"* ]] || false
    [[ "$output" == *"hiçbir şey silinmedi"* ]] || false
    present 0.9.0
    present 1.0.0
    present 1.2.0
}

@test "prune: onay istenir; 'h' ya da EOF silmez, 'e' siler" {
    mkvers 0.9.0 1.0.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 1.2.0
    run comp_cli_prune <<<"h"; [ "$status" -ne 0 ]; present 0.9.0
    run comp_cli_prune </dev/null; [ "$status" -ne 0 ]; present 0.9.0
    run comp_cli_prune <<<"e"; [ "$status" -eq 0 ]; gone 0.9.0
    present 1.0.0
    present 1.2.0
}

@test "prune: kilitteki sürüm etkinden eskiyse (geri alınmış kilit) o da KALIR" {
    mkvers 0.8.0 0.9.0 1.0.0 1.1.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 0.9.0
    run comp_cli_prune --yes
    [ "$status" -eq 0 ]
    gone 0.8.0
    gone 1.0.0
    present 0.9.0
    present 1.1.0
    present 1.2.0
}

@test "prune: temizse 'silinecek yok' der; current olmayan pakete dokunmaz" {
    mkvers 1.1.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 1.2.0
    run comp_cli_prune --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"Silinecek eski sürüm yok"* ]] || false
    rm -f "$T/components/demo/current"
    mkvers 0.5.0
    run comp_cli_prune --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"etkin sürüm (current) yok"* ]] || false
    present 0.5.0
}

@test "prune: .bozuk-* kenara alınmış kurulum, sembolik bağ ve geçersiz adlı klasörler DOKUNULMAZ" {
    mkvers 0.9.0 1.0.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 1.2.0
    mkdir -p "$T/components/demo/0.5.0.bozuk-20260101-000000" "$T/components/demo/evil" "$T/outside"
    printf 'dur' > "$T/outside/dosya"
    ln -s "$T/outside" "$T/components/demo/0.4.0"
    printf 'özet\n' > "$T/components/demo/0.4.0.files"
    run comp_cli_prune --yes
    [ "$status" -eq 0 ]
    gone 0.9.0
    [ -d "$T/components/demo/0.5.0.bozuk-20260101-000000" ]
    [ -d "$T/components/demo/evil" ]
    [ -L "$T/components/demo/0.4.0" ]
    [ -f "$T/outside/dosya" ]
}

@test "prune: bilinmeyen seçenek 2 ile reddedilir, hiçbir şey silinmez" {
    mkvers 0.9.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 1.2.0
    run comp_cli_prune --hepsini-sil
    [ "$status" -eq 2 ]
    present 0.9.0
}

@test "prune: components klasörü yoksa ya da boşsa hata vermez" {
    run comp_cli_prune --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"Kurulu paket yok"* ]] || false
}

@test "components_cli: 'prune' alt komutu yönlendirilir ve kullanım metninde geçer" {
    mkvers 0.8.0 0.9.0 1.2.0
    printf '1.2.0\n' > "$T/components/demo/current"
    set_lock 1.2.0
    run components_cli prune --dry-run
    [ "$status" -eq 0 ]
    [[ "$output" == *"silinecek: 0.8.0"* ]] || false
    run components_cli bilinmeyen
    [ "$status" -eq 2 ]
    [[ "$output" == *"prune [--yes] [--dry-run]"* ]] || false
}
