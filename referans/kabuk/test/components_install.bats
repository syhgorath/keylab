#!/usr/bin/env bats
# Aşama 2.1: paketin güvenli kurulumu (sahte paketler, ağ yok)
load test_helper

setup() {
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_COMPONENTS_LOCK="$T/components.lock"
    export SSHPOCKET_COMPONENTS_DIR="$T/components"
    ssh-keygen -q -t ed25519 -N "" -f "$T/good" -C test
    ssh-keygen -q -t ed25519 -N "" -f "$T/evil" -C evil
    PUB=$(cut -d' ' -f1,2 "$T/good.pub")
}

# make_pkg <sürüm> [bozma]  → $T/demo-<sürüm>.tar.gz (+ .sig), kilit dosyasını bu sürüme/özetine ayarlar
make_pkg() {
    local ver="$1" mode="${2:-}" a="$T/demo-$1.tar.gz"
    python3 -I - "$T" "$ver" "$mode" <<'PY'
import tarfile, io, sys, json, os
T, ver, mode = sys.argv[1], sys.argv[2], sys.argv[3]
root = "demo-" + ver
manifest = {"id": "demo", "version": ver, "sshpocket_api": 1}
if mode == "badid": manifest["id"] = "other"
if mode == "badver": manifest["version"] = "9.9.9"
def add(tf, name, data, mode_=0o644):
    ti = tarfile.TarInfo(name); ti.size = len(data); ti.mode = mode_
    tf.addfile(ti, io.BytesIO(data))
with tarfile.open(os.path.join(T, "demo-%s.tar.gz" % ver), "w:gz") as tf:
    d = tarfile.TarInfo(root); d.type = tarfile.DIRTYPE; d.mode = 0o755; tf.addfile(d)
    if mode != "nomanifest":
        add(tf, root + "/component.json", json.dumps(manifest).encode())
    add(tf, root + "/playbooks/harden.yml", b"- hosts: all\n")
    if mode == "dotdot": add(tf, root + "/../escape", b"x")
    if mode == "abs": add(tf, "/tmp/abs-escape", b"x")
    if mode == "otherroot": add(tf, "other/file", b"x")
    if mode == "symlink":
        l = tarfile.TarInfo(root + "/ln"); l.type = tarfile.SYMTYPE; l.linkname = "/etc/passwd"; tf.addfile(l)
    if mode == "hardlink":
        l = tarfile.TarInfo(root + "/hl"); l.type = tarfile.LNKTYPE; l.linkname = root + "/component.json"; tf.addfile(l)
    if mode == "fifo":
        f = tarfile.TarInfo(root + "/pipe"); f.type = tarfile.FIFOTYPE; tf.addfile(f)
PY
    rm -f "$a.sig"
    ssh-keygen -q -Y sign -f "$T/good" -n sshpocket-component "$a"
    set_lock demo "$ver" "$(comp_sha256_of "$a")"
}

set_lock() {   # id ver sha
    cat > "$SSHPOCKET_COMPONENTS_LOCK" <<LOCK
[$1]
repo = syhgorath/demo
version = $2
sha256 = $3
signers = sshpocket-component namespaces="sshpocket-component" $PUB
LOCK
}

install_ver() { comp_install_archive demo "$1" "$T/demo-$1.tar.gz" "$T/demo-$1.tar.gz.sig"; }
nothing_installed() { [ ! -e "$SSHPOCKET_COMPONENTS_DIR/demo/current" ] && [ -z "$(ls -A "$SSHPOCKET_COMPONENTS_DIR/demo" 2>/dev/null | grep -v '^\.tmp')" ] ; }

@test "kurulum: geçerli paket sürümlü klasöre kurulur, current yazılır" {
    make_pkg 1.0.0
    run install_ver 1.0.0
    [ "$status" -eq 0 ]
    [ -f "$T/components/demo/1.0.0/component.json" ]
    [ -f "$T/components/demo/1.0.0/playbooks/harden.yml" ]
    [ "$(cat "$T/components/demo/current")" = "1.0.0" ]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
    run comp_verify_installed demo 1.0.0
    [ "$status" -eq 0 ]
    [ -z "$(find "$T/components" -name '.tmp*')" ]
}

@test "kurulum: aynı sürüm tekrar kurulursa 'zaten kurulu' (idempotent)" {
    make_pkg 1.0.0
    install_ver 1.0.0
    run install_ver 1.0.0
    [ "$status" -eq 0 ]
    [[ "$output" == *"zaten kurulu"* ]]
}

@test "kurulum: yükseltmede eski sürüm klasörü kalır, current yenisini gösterir" {
    make_pkg 1.0.0; install_ver 1.0.0
    make_pkg 1.1.0
    run install_ver 1.1.0
    [ "$status" -eq 0 ]
    [ -d "$T/components/demo/1.0.0" ]
    [ -d "$T/components/demo/1.1.0" ]
    [ "$(cat "$T/components/demo/current")" = "1.1.0" ]
}

@test "kurulum: geri sürüme düşme reddedilir" {
    make_pkg 1.1.0; install_ver 1.1.0
    make_pkg 1.0.0
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    [[ "$output" == *"geri sürüme"* ]]
    [ "$(cat "$T/components/demo/current")" = "1.1.0" ]
    [ ! -e "$T/components/demo/1.0.0" ]
}

@test "kurulum: istenen sürüm kilitteki sürümle farklıysa reddedilir" {
    make_pkg 1.0.0
    set_lock demo 1.2.0 "$(comp_sha256_of "$T/demo-1.0.0.tar.gz")"
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    nothing_installed
}

@test "kurulum: imzadan sonra değiştirilen arşiv reddedilir, hiçbir şey kurulmaz" {
    make_pkg 1.0.0
    printf 'x' >> "$T/demo-1.0.0.tar.gz"
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    nothing_installed
}

@test "kurulum: başka anahtarla imzalı paket reddedilir" {
    make_pkg 1.0.0
    rm -f "$T/demo-1.0.0.tar.gz.sig"
    ssh-keygen -q -Y sign -f "$T/evil" -n sshpocket-component "$T/demo-1.0.0.tar.gz"
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    nothing_installed
}

@test "kurulum: GÜVENİLEN anahtarla ama yanlış ad alanıyla (sshpocket-release) imzalı paket reddedilir" {
    make_pkg 1.0.0
    rm -f "$T/demo-1.0.0.tar.gz.sig"
    ssh-keygen -q -Y sign -f "$T/good" -n sshpocket-release "$T/demo-1.0.0.tar.gz"
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    nothing_installed
}

@test "kurulum: kilitte signers satırı boş ya da bozuksa hiçbir şey kurulmaz" {
    make_pkg 1.0.0
    sed -i.bak 's/^signers = .*/signers = /' "$SSHPOCKET_COMPONENTS_LOCK"
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    nothing_installed
    sed -i.bak 's/^signers = .*/signers = anahtar-degil/' "$SSHPOCKET_COMPONENTS_LOCK"
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    nothing_installed
}

@test "arşiv saldırıları reddedilir: .., mutlak yol, farklı kök, symlink, hardlink, fifo" {
    local m
    for m in dotdot abs otherroot symlink hardlink fifo; do
        rm -rf "$T/components"
        make_pkg 1.0.0 "$m"
        run install_ver 1.0.0
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $m"; return 1; }
        nothing_installed || { echo "KALINTI: $m"; return 1; }
    done
    [ ! -e /tmp/abs-escape ]
}

@test "arşiv denetimi (tek başına): .., mutlak yol, farklı kök, symlink, hardlink, fifo reddedilir; temiz arşiv kabul edilir" {
    local m
    make_pkg 1.0.0
    run comp_check_archive "$T/demo-1.0.0.tar.gz" demo-1.0.0
    [ "$status" -eq 0 ]
    for m in dotdot abs otherroot symlink hardlink fifo; do
        make_pkg 1.0.0 "$m"
        run comp_check_archive "$T/demo-1.0.0.tar.gz" demo-1.0.0
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $m"; return 1; }
    done
}

@test "bildirim: eksik, kimliği veya sürümü uyuşmayan component.json reddedilir" {
    local m
    for m in nomanifest badid badver; do
        rm -rf "$T/components"
        make_pkg 1.0.0 "$m"
        run install_ver 1.0.0
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $m"; return 1; }
        nothing_installed || { echo "KALINTI: $m"; return 1; }
    done
}

@test "bütünlük: kurulumdan sonra değişen/eklenen/silinen dosya yakalanır" {
    make_pkg 1.0.0; install_ver 1.0.0
    run comp_verify_installed demo 1.0.0; [ "$status" -eq 0 ]
    printf 'EVIL' >> "$T/components/demo/1.0.0/playbooks/harden.yml"
    run comp_verify_installed demo 1.0.0; [ "$status" -ne 0 ]
    rm -rf "$T/components"; make_pkg 1.0.0; install_ver 1.0.0
    printf 'x' > "$T/components/demo/1.0.0/extra.yml"
    run comp_verify_installed demo 1.0.0; [ "$status" -ne 0 ]
    rm -rf "$T/components"; make_pkg 1.0.0; install_ver 1.0.0
    rm "$T/components/demo/1.0.0/playbooks/harden.yml"
    run comp_verify_installed demo 1.0.0; [ "$status" -ne 0 ]
}

@test "indirme: kilitteki repo/sürümden HTTPS GitHub Release adresi kullanılır, imza ayrı indirilir" {
    make_pkg 1.0.0
    URLS="$T/urls"; : > "$URLS"
    comp_download() { echo "$1" >> "$URLS"; case "$1" in *.sig) cp "$T/demo-1.0.0.tar.gz.sig" "$2" ;; *) cp "$T/demo-1.0.0.tar.gz" "$2" ;; esac; }
    run comp_install demo
    [ "$status" -eq 0 ]
    grep -qx 'https://github.com/syhgorath/demo/releases/download/v1.0.0/demo-1.0.0.tar.gz' "$URLS"
    grep -qx 'https://github.com/syhgorath/demo/releases/download/v1.0.0/demo-1.0.0.tar.gz.sig' "$URLS"
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "indirme: başarısız indirme hiçbir şey kurmaz; kilitte olmayan paket hiç indirilmez" {
    make_pkg 1.0.0
    comp_download() { return 22; }
    run comp_install demo
    [ "$status" -ne 0 ]
    nothing_installed
    called="$T/called"; comp_download() { echo x > "$called"; }
    run comp_install baska
    [ "$status" -ne 0 ]
    [ ! -e "$called" ]
}

@test "indirme: gerçek indirici yalnızca HTTPS ister ve boyut sınırlıdır" {
    run grep -c -- "--proto '=https'" "$H/ComponentsInstall.sh"
    [ "$output" -ge 1 ]
    run grep -c -- '--max-filesize' "$H/ComponentsInstall.sh"
    [ "$output" -ge 1 ]
}


@test "kilit: canlı bir kurulum kilit tutuyorsa bekler ve sonunda reddeder; hiçbir şey kurulmaz" {
    make_pkg 1.0.0
    mkdir -p "$T/components/demo/.install.lock"
    sleep 30 & local holder=$!
    echo "$holder" > "$T/components/demo/.install.lock/pid"
    SSHPOCKET_INSTALL_LOCK_TRIES=2 run install_ver 1.0.0
    kill "$holder" 2>/dev/null; wait "$holder" 2>/dev/null || true
    [ "$status" -ne 0 ]
    [[ "$output" == *"başka bir kurulum sürüyor"* ]] || false
    [ ! -e "$T/components/demo/1.0.0" ]
}

@test "kilit: sahibi ölmüş (yarım kalmış) kilit temizlenir ve kurulum sürer" {
    make_pkg 1.0.0
    mkdir -p "$T/components/demo/.install.lock"
    echo 99999999 > "$T/components/demo/.install.lock/pid"
    run install_ver 1.0.0
    [ "$status" -eq 0 ]
    [ -f "$T/components/demo/1.0.0/component.json" ]
    [ ! -e "$T/components/demo/.install.lock" ]
}

@test "kilit: aynı anda iki kurulum iç içe klasör üretmez; biri kurar, diğeri 'zaten kurulu' der" {
    make_pkg 1.0.0
    install_ver 1.0.0 > "$T/o1" 2>&1 & local p1=$!
    install_ver 1.0.0 > "$T/o2" 2>&1 & local p2=$!
    wait "$p1"; r1=$?; wait "$p2"; r2=$?
    [ "$r1" -eq 0 ] && [ "$r2" -eq 0 ]
    [ ! -e "$T/components/demo/1.0.0/demo-1.0.0" ]
    [ -f "$T/components/demo/1.0.0/component.json" ]
    run comp_verify_installed demo 1.0.0
    [ "$status" -eq 0 ]
    [ -z "$(find "$T/components" -name '.tmp*' -o -name '.install.lock')" ]
    grep -qh "kuruldu\|zaten kurulu" "$T/o1" "$T/o2"
}

@test "kilit: başarısız kurulum da kilidi bırakır" {
    make_pkg 1.0.0 badid
    run install_ver 1.0.0
    [ "$status" -ne 0 ]
    [ ! -e "$T/components/demo/.install.lock" ]
}
