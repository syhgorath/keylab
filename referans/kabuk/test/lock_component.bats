#!/usr/bin/env bats
# Bakımcı aracı: scripts/lock-component.sh (yerel arşivle; ağ yok, doğrulamalar tam)
load test_helper

setup() {
    T="$BATS_TEST_TMPDIR"; S="$ROOT/scripts/lock-component.sh"
    export SSHPOCKET_COMPONENTS_LOCK="$T/components.lock"
    ssh-keygen -q -t ed25519 -N "" -f "$T/good" -C test
    ssh-keygen -q -t ed25519 -N "" -f "$T/evil" -C evil
    PUB=$(cut -d' ' -f1,2 "$T/good.pub")
    SIGNERS="sshpocket-component namespaces=\"sshpocket-component\" $PUB"
    printf '%s sshpocket-release\n' "sshpocket-release namespaces=\"sshpocket-release\" $PUB" > "$T/release_signers"
    export LOCK_RELEASE_SIGNERS="$T/release_signers"
}

# mk <id> <sürüm> [bozma] → $T/<id>-<sürüm>.tar.gz + .sig (good anahtarıyla, sshpocket-component)
mk() {
    local id="$1" ver="$2" mode="${3:-}" a="$T/$1-$2.tar.gz" key=good ns=sshpocket-component
    python3 -I - "$T" "$id" "$ver" "$mode" <<'PY'
import tarfile, io, sys, json, os
T, id_, ver, mode = sys.argv[1:5]
root = "%s-%s" % (id_, ver)
m = {"id": id_, "version": ver, "sshpocket_api": 1, "runner": "ansible",
     "profiles": [{"id": "p", "status": "supported"}], "actions": {"a": {"cmd": "run.sh", "effects": []}}}
if mode == "badid": m["id"] = "baska"
if mode == "badschema": m["sshpocket_api"] = 99
def add(t, n, d):
    i = tarfile.TarInfo(n); i.size = len(d); t.addfile(i, io.BytesIO(d))
with tarfile.open(os.path.join(T, root + ".tar.gz"), "w:gz") as t:
    if mode != "nomanifest": add(t, root + "/component.json", json.dumps(m).encode())
    add(t, root + "/x.yml", b"- hosts: all\n")
    if mode == "symlink":
        l = tarfile.TarInfo(root + "/ln"); l.type = tarfile.SYMTYPE; l.linkname = "/etc/passwd"; t.addfile(l)
PY
    [ "$mode" = "evilsig" ] && key=evil
    [ "$mode" = "wrongns" ] && ns=sshpocket-release
    rm -f "$a.sig"
    ssh-keygen -q -Y sign -f "$T/$key" -n "$ns" "$a"
}

run_lock() {   # $1 id $2 ver, ekstra argümanlar
    local id="$1" ver="$2"; shift 2
    LOCK_COMPONENT_ARCHIVE="$T/$id-$ver.tar.gz" LOCK_COMPONENT_SIG="$T/$id-$ver.tar.gz.sig" \
        run bash "$S" "$id" "$ver" --repo syhgorath/demo "$@"
}

@test "kilitleme: --write yoksa yalnızca gösterir, dosyaya dokunmaz" {
    mk demo 1.0.0
    run_lock demo 1.0.0
    [ "$status" -eq 0 ]
    [[ "$output" == *"[demo]"* ]]
    [[ "$output" == *"sha256 = "* ]]
    [[ "$output" == *"yalnızca gösterildi"* ]]
    [ ! -e "$SSHPOCKET_COMPONENTS_LOCK" ]
}

@test "kilitleme: --write girdiyi yazar; varsayılan imzacı release_signers'tan türetilir; sha256 doğru" {
    mk demo 1.0.0
    run_lock demo 1.0.0 --write
    [ "$status" -eq 0 ]
    grep -qx 'repo = syhgorath/demo' "$SSHPOCKET_COMPONENTS_LOCK"
    grep -qx 'version = 1.0.0' "$SSHPOCKET_COMPONENTS_LOCK"
    grep -qx "sha256 = $(shasum -a 256 "$T/demo-1.0.0.tar.gz" | cut -d' ' -f1)" "$SSHPOCKET_COMPONENTS_LOCK"
    grep -qx "signers = $SIGNERS" "$SSHPOCKET_COMPONENTS_LOCK"
}

@test "kilitleme: mevcut girdi YENİ sürümle değişir (yinelenmez), diğer paketler korunur" {
    mk demo 1.0.0; run_lock demo 1.0.0 --write
    mk other 2.0.0; LOCK_COMPONENT_ARCHIVE="$T/other-2.0.0.tar.gz" LOCK_COMPONENT_SIG="$T/other-2.0.0.tar.gz.sig" \
        bash "$S" other 2.0.0 --repo syhgorath/other --write >/dev/null
    mk demo 1.1.0; run_lock demo 1.1.0 --write
    [ "$status" -eq 0 ]
    [ "$(grep -c '^\[demo\]' "$SSHPOCKET_COMPONENTS_LOCK")" -eq 1 ]
    grep -qx 'version = 1.1.0' "$SSHPOCKET_COMPONENTS_LOCK"
    run ! grep -q 'version = 1.0.0' "$SSHPOCKET_COMPONENTS_LOCK"
    grep -qx '\[other\]' "$SSHPOCKET_COMPONENTS_LOCK"
    grep -qx 'version = 2.0.0' "$SSHPOCKET_COMPONENTS_LOCK"
}

@test "kilitleme: kötü imza/ad alanı/anahtar → reddedilir, kilit dosyası DEĞİŞMEZ" {
    mk demo 1.0.0; run_lock demo 1.0.0 --write
    before=$(cat "$SSHPOCKET_COMPONENTS_LOCK")
    local m
    for m in evilsig wrongns; do
        mk demo 1.1.0 "$m"
        run_lock demo 1.1.0 --write
        [ "$status" -ne 0 ] || { echo "KABUL: $m"; return 1; }
        [ "$(cat "$SSHPOCKET_COMPONENTS_LOCK")" = "$before" ] || { echo "DEĞİŞTİ: $m"; return 1; }
    done
}

@test "kilitleme: kurulamayacak paket (component.json yok/geçersiz/uyuşmuyor, symlink) kilide YAZILMAZ" {
    local m
    for m in nomanifest badid badschema symlink; do
        mk demo 1.0.0 "$m"
        run_lock demo 1.0.0 --write
        [ "$status" -ne 0 ] || { echo "KABUL: $m"; return 1; }
        [ ! -e "$SSHPOCKET_COMPONENTS_LOCK" ] || { echo "YAZILDI: $m"; return 1; }
    done
    mk demo 1.0.0 nomanifest
    run_lock demo 1.0.0
    [[ "$output" == *"component.json içermiyor"* ]]
}

@test "kilitleme: üretilen kilit gerçekten kurulabilir paketi doğrular (uçtan uca: lock → install)" {
    mk demo 1.0.0; run_lock demo 1.0.0 --write
    run env SISTEM_PATH="$ROOT/sistem" SSHPOCKET_COMPONENTS_DIR="$T/components" bash -c "
        source '$ROOT/sistem/Helpers/Components.sh'; source '$ROOT/sistem/Helpers/ComponentsInstall.sh'
        comp_install_archive demo 1.0.0 '$T/demo-1.0.0.tar.gz' '$T/demo-1.0.0.tar.gz.sig'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"kuruldu: demo 1.0.0"* ]]
}

@test "kilitleme: geçersiz id/sürüm/depo ve eksik argüman reddedilir" {
    run bash "$S"; [ "$status" -ne 0 ]
    run bash "$S" "Demo" 1.0.0 --repo a/b; [ "$status" -ne 0 ]
    run bash "$S" demo 1.0 --repo a/b; [ "$status" -ne 0 ]
    run bash "$S" demo 1.0.0 --repo "../etc"; [ "$status" -ne 0 ]
    run bash "$S" demo 1.0.0; [ "$status" -ne 0 ]              # depo yok
}
