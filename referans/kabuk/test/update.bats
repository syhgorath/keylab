#!/usr/bin/env bats
# İmzalı güncelleme testleri: ağ yok (sahte curl), gerçek GitHub'a bağlanılmaz.
load test_helper

setup() {
    # shellcheck source=/dev/null
    source "$ROOT/sistem/Update.sh"
    T="$BATS_TEST_TMPDIR"
    mkdir -p "$T/bin" "$T/serve"
    FAKE_LOG="$T/curl.log"; : > "$FAKE_LOG"
    export FAKE_LOG FAKE_DIR="$T/serve"

    # Test imza anahtarı (şifresiz, yalnızca bu test için)
    SIGNKEY="$T/signkey"
    ssh-keygen -q -t ed25519 -N "" -f "$SIGNKEY"
    SIGNERS="$T/signers"
    printf 'sshpocket-release namespaces="sshpocket-release" %s\n' "$(cut -d' ' -f1,2 "$SIGNKEY.pub")" > "$SIGNERS"

    # Sahte curl: -sSI -> Location başlığı; indirme -> FAKE_DIR'den kopyala
    cat > "$T/bin/curl" <<'STUB'
#!/bin/bash
echo "$*" >> "$FAKE_LOG"
for a in "$@"; do
    if [ "$a" = "-sSI" ]; then
        [ -n "${FAKE_LATEST_RAW:-}" ] && { printf 'HTTP/2 302\r\nlocation: %s\r\n\r\n' "$FAKE_LATEST_RAW"; exit 0; }
        [ -n "${FAKE_LATEST:-}" ] && printf 'HTTP/2 302\r\nlocation: https://github.com/syhgorath/sshpocket/releases/tag/v%s\r\n\r\n' "$FAKE_LATEST"
        exit 0
    fi
done
out=""; url=""
while [ $# -gt 0 ]; do
    case "$1" in -o) out="$2"; shift 2 ;; -*) shift ;; *) url="$1"; shift ;; esac
done
src="$FAKE_DIR/$(basename "$url")"
[ -f "$src" ] || exit 22
cp "$src" "$out"
STUB
    chmod +x "$T/bin/curl"
    export PATH="$T/bin:$PATH"
}

# make_release VERSION [KEY] [NAMESPACE]  -> $T/serve/sshpocket-VERSION.tar.gz(.sig)
make_release() {
    local v="$1" key="${2:-$SIGNKEY}" ns="${3:-sshpocket-release}"
    local tree="$T/rel/sshpocket-$v"
    mkdir -p "$tree/sistem/Helpers" "$tree/sistem/Modules/example"
    printf '%s\n' "$v" > "$tree/VERSION"
    printf '# Changelog\n\n## [%s] - 2026-01-01\n\n### Dikkat\n- davranış değişti %s\n\n### Eklendi\n- yeni özellik %s\n\n## [0.0.1] - 2020-01-01\n- eski\n' "$v" "$v" "$v" > "$tree/CHANGELOG.md"
    printf '[linux-hardening]\nrepo = a/b\nversion = 0.12.0\n\n[yeni-paket]\nversion = 1.0.0\n' > "$tree/components.lock"
    printf '#!/bin/bash\necho NEW-%s\n' "$v" > "$tree/sistem/Helpers/New.sh"
    printf '#!/bin/bash\necho main-%s\n' "$v" > "$tree/sistem/Main.sh"
    printf '#!/bin/bash\necho update-%s\n' "$v" > "$tree/sistem/Update.sh"
    printf 'EXAMPLE_IP=\n' > "$tree/sistem/Modules/example/.env.example"
    printf 'sshpocket-release namespaces="sshpocket-release" %s\n' "$(cut -d' ' -f1,2 "$SIGNKEY.pub")" > "$tree/sistem/release_signers"
    tar -czf "$T/serve/sshpocket-$v.tar.gz" -C "$T/rel" "sshpocket-$v"
    rm -f "$T/serve/sshpocket-$v.tar.gz.sig"
    ssh-keygen -q -Y sign -f "$key" -n "$ns" "$T/serve/sshpocket-$v.tar.gz"
}

# Hedef kurulum: eski sürüm kodu + kullanıcı verisi
make_install() {
    INST="$T/inst"
    mkdir -p "$INST/sistem/Helpers" "$INST/sistem/Modules/mine" "$INST/sistem/Modules/example"
    printf '2.1.0\n' > "$INST/VERSION"
    printf 'eski-main\n' > "$INST/sistem/Main.sh"
    printf 'eski-helper\n' > "$INST/sistem/Helpers/Old.sh"
    printf 'MINE_IP=192.0.2.1\n' > "$INST/sistem/Modules/mine/.env"
    printf 'GIZLI-KEY\n' > "$INST/sistem/Modules/mine/id_ed25519_mine"
    printf 'EXAMPLE_IP=10.9.9.9\n' > "$INST/sistem/Modules/example/.env"
    cp "$SIGNERS" "$INST/sistem/release_signers"
}

# ---------- sürüm ----------
@test "sürüm biçimi ve karşılaştırma" {
    update_version_valid "2.1.0"
    run ! update_version_valid "2.1"
    run ! update_version_valid "v2.1.0"
    run ! update_version_valid "2.1.0; id"
    [ "$(update_version_cmp 2.2.0 2.1.0)" = "1" ]
    [ "$(update_version_cmp 2.1.0 2.1.0)" = "0" ]
    [ "$(update_version_cmp 2.1.0 2.2.0)" = "-1" ]
    [ "$(update_version_cmp 2.10.0 2.9.0)" = "1" ]
    [ "$(update_version_cmp 3.0.0 2.99.99)" = "1" ]
    [ "$(update_version_cmp 1.0.10 1.0.9)" = "1" ]
}

@test "latest: geçerli etiket çözülür, curl yalnızca HTTPS ile çağrılır" {
    FAKE_LATEST=2.2.0 run update_latest_version
    [ "$status" -eq 0 ]; [ "$output" = "2.2.0" ]
    grep -q -- "--proto =https" "$FAKE_LOG"
    grep -q -- "--tlsv1.2" "$FAKE_LOG"
}
@test "latest: başka repo, geçersiz etiket veya boş yanıt reddedilir" {
    FAKE_LATEST_RAW="https://github.com/saldirgan/sshpocket/releases/tag/v9.9.9" run update_latest_version
    [ "$status" -ne 0 ]
    FAKE_LATEST_RAW="https://github.com/syhgorath/sshpocket/releases/tag/v1.2" run update_latest_version
    [ "$status" -ne 0 ]
    FAKE_LATEST_RAW='https://github.com/syhgorath/sshpocket/releases/tag/v1.2.3;id' run update_latest_version
    [ "$status" -ne 0 ]
    FAKE_LATEST_RAW="http://github.com/syhgorath/sshpocket/releases/tag/v1.2.3" run update_latest_version
    [ "$status" -ne 0 ]
    FAKE_LATEST="" run update_latest_version
    [ "$status" -ne 0 ]
}

# ---------- imza ----------
@test "verify: doğru imza kabul edilir" {
    make_release 2.2.0
    run update_verify "$T/serve/sshpocket-2.2.0.tar.gz" "$T/serve/sshpocket-2.2.0.tar.gz.sig" "$SIGNERS"
    [ "$status" -eq 0 ]
}
@test "verify: arşiv imzadan sonra değiştirilirse reddedilir" {
    make_release 2.2.0
    printf 'x' >> "$T/serve/sshpocket-2.2.0.tar.gz"
    run update_verify "$T/serve/sshpocket-2.2.0.tar.gz" "$T/serve/sshpocket-2.2.0.tar.gz.sig" "$SIGNERS"
    [ "$status" -ne 0 ]
}
@test "verify: başka bir anahtarla imzalanmış paket reddedilir" {
    ssh-keygen -q -t ed25519 -N "" -f "$T/saldirgan"
    make_release 2.2.0 "$T/saldirgan"
    run update_verify "$T/serve/sshpocket-2.2.0.tar.gz" "$T/serve/sshpocket-2.2.0.tar.gz.sig" "$SIGNERS"
    [ "$status" -ne 0 ]
}
@test "verify: yanlış namespace ile imzalanmış paket reddedilir" {
    make_release 2.2.0 "$SIGNKEY" "baska-amac"
    run update_verify "$T/serve/sshpocket-2.2.0.tar.gz" "$T/serve/sshpocket-2.2.0.tar.gz.sig" "$SIGNERS"
    [ "$status" -ne 0 ]
}
@test "verify: imza anahtarı tanımlı değilse (boş/yorum) güncelleme KAPALI" {
    make_release 2.2.0
    printf '# yalnızca yorum\n' > "$T/bos_signers"
    run update_verify "$T/serve/sshpocket-2.2.0.tar.gz" "$T/serve/sshpocket-2.2.0.tar.gz.sig" "$T/bos_signers"
    [ "$status" -ne 0 ]
    [[ "$output" == *"tanımlı değil"* ]]
    run update_verify "$T/serve/sshpocket-2.2.0.tar.gz" "$T/serve/sshpocket-2.2.0.tar.gz.sig" "$T/yok_dosya"
    [ "$status" -ne 0 ]
}

# ---------- arşiv güvenliği ----------
mk_evil() { # python ile tarfile üret (tar komutları '..' ve '/' yollarını temizler)
    command -v python3 >/dev/null || skip "python3 yok"
    python3 - "$@" <<'PY'
import sys, tarfile, io
out, kind = sys.argv[1], sys.argv[2]
with tarfile.open(out, "w:gz") as t:
    def add(name, data=b"x", typ=None, link=None):
        i = tarfile.TarInfo(name); i.size = len(data)
        if typ: i.type = typ; i.linkname = link or ""; i.size = 0; t.addfile(i)
        else: t.addfile(i, io.BytesIO(data))
    if kind == "dotdot":   add("sshpocket-2.2.0/VERSION", b"2.2.0\n"); add("sshpocket-2.2.0/../../evil", b"x")
    elif kind == "abs":    add("sshpocket-2.2.0/VERSION", b"2.2.0\n"); add("/tmp/evil_abs", b"x")
    elif kind == "symlink":add("sshpocket-2.2.0/VERSION", b"2.2.0\n"); add("sshpocket-2.2.0/l", typ=tarfile.SYMTYPE, link="/etc/passwd")
    elif kind == "hardlink":add("sshpocket-2.2.0/VERSION", b"2.2.0\n"); add("sshpocket-2.2.0/h", typ=tarfile.LNKTYPE, link="sshpocket-2.2.0/VERSION")
    elif kind == "prefix": add("baska-dizin/VERSION", b"2.2.0\n")
    elif kind == "ok":     add("sshpocket-2.2.0/VERSION", b"2.2.0\n")
PY
}
@test "arşiv: geçerli arşiv kabul edilir" {
    mk_evil "$T/a.tgz" ok
    run update_check_archive "$T/a.tgz" 2.2.0
    [ "$status" -eq 0 ]
}
@test "arşiv: '..' içeren yol reddedilir" {
    mk_evil "$T/a.tgz" dotdot
    run update_check_archive "$T/a.tgz" 2.2.0
    [ "$status" -ne 0 ]
}
@test "arşiv: mutlak yol reddedilir" {
    mk_evil "$T/a.tgz" abs
    run update_check_archive "$T/a.tgz" 2.2.0
    [ "$status" -ne 0 ]
}
@test "arşiv: symlink ve hardlink reddedilir" {
    mk_evil "$T/a.tgz" symlink
    run update_check_archive "$T/a.tgz" 2.2.0
    [ "$status" -ne 0 ]
    mk_evil "$T/b.tgz" hardlink
    run update_check_archive "$T/b.tgz" 2.2.0
    [ "$status" -ne 0 ]
}
@test "arşiv: beklenen üst dizin dışındaki içerik reddedilir" {
    mk_evil "$T/a.tgz" prefix
    run update_check_archive "$T/a.tgz" 2.2.0
    [ "$status" -ne 0 ]
}
@test "arşiv: bozuk/olmayan dosya reddedilir" {
    printf 'bu bir tar degil' > "$T/a.tgz"
    run update_check_archive "$T/a.tgz" 2.2.0
    [ "$status" -ne 0 ]
}

# ---------- kurulum ----------
@test "apply: kod değişir, kullanıcı verisi korunur, eski kod temizlenir, yedek alınır" {
    make_install
    make_release 2.2.0
    mkdir "$T/x"; tar -xzf "$T/serve/sshpocket-2.2.0.tar.gz" -C "$T/x"
    run update_apply "$T/x/sshpocket-2.2.0" "$INST" 2.1.0
    [ "$status" -eq 0 ]
    [ "$(cat "$INST/VERSION")" = "2.2.0" ]
    [ -f "$INST/sistem/Helpers/New.sh" ]
    [ ! -e "$INST/sistem/Helpers/Old.sh" ]
    [[ "$(cat "$INST/sistem/Main.sh")" == *"main-2.2.0"* ]]
    # kullanıcı verisi
    [ "$(cat "$INST/sistem/Modules/mine/.env")" = "MINE_IP=192.0.2.1" ]
    [ "$(cat "$INST/sistem/Modules/mine/id_ed25519_mine")" = "GIZLI-KEY" ]
    # example klasöründeki kullanıcı .env'i silinmez, arşivdeki dosya yazılır
    [ "$(cat "$INST/sistem/Modules/example/.env")" = "EXAMPLE_IP=10.9.9.9" ]
    [ -f "$INST/sistem/Modules/example/.env.example" ]
    # yedek: var, eski kodu içerir, gizli dosya İÇERMEZ
    [ -f "$INST/.update-backup/code-2.1.0.tar.gz" ]
    tar -tzf "$INST/.update-backup/code-2.1.0.tar.gz" | grep -q 'Helpers/Old.sh'
    [ -z "$(tar -tzf "$INST/.update-backup/code-2.1.0.tar.gz" | grep -E '\.env$|id_ed25519')" ]
}
@test "apply: kurulum yarıda başarısız olursa yedekten geri yüklenir" {
    make_install
    make_release 2.2.0
    mkdir "$T/x"; tar -xzf "$T/serve/sshpocket-2.2.0.tar.gz" -C "$T/x"
    # kurulumu kısmen yapıp başarısız olan sahte adım
    update_install_tree() { rm -r "$2/sistem/Helpers"; printf 'YARIM\n' > "$2/sistem/Main.sh"; return 1; }
    run update_apply "$T/x/sshpocket-2.2.0" "$INST" 2.1.0
    [ "$status" -ne 0 ]
    [ "$(cat "$INST/sistem/Main.sh")" = "eski-main" ]
    [ "$(cat "$INST/sistem/Helpers/Old.sh")" = "eski-helper" ]
    [ "$(cat "$INST/VERSION")" = "2.1.0" ]
    [ "$(cat "$INST/sistem/Modules/mine/.env")" = "MINE_IP=192.0.2.1" ]
}
@test "apply: yazma korumalı hedefte hiçbir şey yapmadan hata verir" {
    make_install
    make_release 2.2.0
    mkdir "$T/x"; tar -xzf "$T/serve/sshpocket-2.2.0.tar.gz" -C "$T/x"
    chmod a-w "$INST"
    run update_apply "$T/x/sshpocket-2.2.0" "$INST" 2.1.0
    chmod u+w "$INST"
    [ "$status" -ne 0 ]
    [[ "$output" == *"yazılabilir değil"* ]]
    [ "$(cat "$INST/VERSION")" = "2.1.0" ]
}

# ---------- uçtan uca (CLI, sahte curl) ----------
make_cli_sandbox() {
    SB="$T/sb"; mkdir -p "$SB"
    # -L: imza dosyaları (release_signers/index_signers) depo kökünü gösteren göreli sembolik bağdır; sandbox'ta kopuk kalırlar ve
    # GNU cp kopuk bağın üstüne yazmaz ("not writing through dangling symlink"). Gerçek dosya olarak kopyala.
    cp -RL "$ROOT/sistem" "$SB/"; cp "$ROOT/start.sh" "$SB/"
    printf '2.1.0\n' > "$SB/VERSION"
    printf '# yorum\n# [yorum-paket]\n\n[linux-hardening]\nrepo = a/b\nversion = 0.11.0\n\n[eski-paket]\nversion = 2.0.0\n' > "$SB/components.lock"
    printf 'EjectUSB(){ :; }\n' > "$SB/sistem/Helpers/Eject.sh"
    cp "$SIGNERS" "$SB/sistem/release_signers"
    mkdir -p "$SB/sistem/Modules/mine"
    printf 'MINE_IP=192.0.2.1\n' > "$SB/sistem/Modules/mine/.env"
}
@test "CLI --check-update: yeni sürüm varsa çıkış kodu 10, hiçbir şey değişmez" {
    make_cli_sandbox
    FAKE_LATEST=2.2.0 run bash "$SB/sistem/Main.sh" --check-update
    [ "$status" -eq 10 ]
    [[ "$output" == *"Yeni sürüm var: 2.2.0"* ]]
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
    run ! grep -q -- "-o " "$FAKE_LOG"
}
@test "CLI --check-update: güncelse 0; uzak sürüm eskiyse geri sürüme düşmez" {
    make_cli_sandbox
    FAKE_LATEST=2.1.0 run bash "$SB/sistem/Main.sh" --check-update
    [ "$status" -eq 0 ]; [[ "$output" == *"Güncelsiniz"* ]]
    FAKE_LATEST=2.0.0 run bash "$SB/sistem/Main.sh" --check-update
    [ "$status" -eq 0 ]; [[ "$output" == *"geri sürüme düşülmez"* ]]
}
@test "CLI --update: imzalı paket kurulur, kullanıcı modülü korunur" {
    make_cli_sandbox; make_release 2.2.0
    FAKE_LATEST=2.2.0 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -eq 0 ]
    [[ "$output" == *"İmza geçerli"* ]]
    [[ "$output" == *"yeni özellik 2.2.0"* ]]
    [[ "$output" == *"2.1.0 → 2.2.0"* ]]
    # özet tablosu (kurulumdan önce): sürüm, imza, paket farkları, Dikkat
    [[ "$output" == *"Güncelleme özeti"* && "$output" == *"linux-hardening 0.11.0 → 0.12.0"* ]] || { echo "$output"; false; }
    [[ "$output" == *"yeni-paket 1.0.0 (yeni)"* && "$output" == *"eski-paket 2.0.0 (kilitten çıkıyor)"* ]] || { echo "$output"; false; }
    [[ "$output" == *"Dikkat:"* && "$output" == *"davranış değişti 2.2.0"* ]] || { echo "$output"; false; }
    pre="${output%%Kurulsun mu*}"; [[ "$pre" == *"Güncelleme özeti"* ]]
    [ "$(cat "$SB/VERSION")" = "2.2.0" ]
    [ "$(cat "$SB/sistem/Modules/mine/.env")" = "MINE_IP=192.0.2.1" ]
    grep -q -- "--proto-redir =https" "$FAKE_LOG"
}
@test "CLI --update: sahte (başka anahtarla imzalı) paket KURULMAZ" {
    make_cli_sandbox
    ssh-keygen -q -t ed25519 -N "" -f "$T/saldirgan"
    make_release 2.2.0 "$T/saldirgan"
    FAKE_LATEST=2.2.0 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    [[ "$output" == *"İMZA DOĞRULANAMADI"* ]]
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
    [ ! -e "$SB/sistem/Helpers/New.sh" ]
}
@test "CLI --update: indirme sonrası bozulan paket KURULMAZ" {
    make_cli_sandbox; make_release 2.2.0
    printf 'x' >> "$T/serve/sshpocket-2.2.0.tar.gz"
    FAKE_LATEST=2.2.0 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    [[ "$output" == *"İMZA DOĞRULANAMADI"* ]]
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
}
@test "CLI --update: eski imzalı sürüm 'latest' diye sunulursa (replay) reddedilir" {
    make_cli_sandbox
    make_release 2.1.5                       # gerçekten imzalı, ama eski sürüm
    cp "$T/serve/sshpocket-2.1.5.tar.gz"     "$T/serve/sshpocket-3.0.0.tar.gz"
    cp "$T/serve/sshpocket-2.1.5.tar.gz.sig" "$T/serve/sshpocket-3.0.0.tar.gz.sig"
    FAKE_LATEST=3.0.0 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
}
@test "CLI --update: imza anahtarı tanımlı değilse hiçbir şey kurulmaz (varsayılan KAPALI)" {
    make_cli_sandbox; make_release 2.2.0
    printf '# boş\n' > "$SB/sistem/release_signers"
    FAKE_LATEST=2.2.0 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
}
@test "CLI --update: ilk soruya hayır denirse indirme bile yapılmaz" {
    make_cli_sandbox; make_release 2.2.0
    FAKE_LATEST=2.2.0 run bash -c "printf 'n\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    run ! grep -q -- "-o " "$FAKE_LOG"
}
@test "CLI --update: kurulumu onaylamazsanız dosyalar değişmez" {
    make_cli_sandbox; make_release 2.2.0
    FAKE_LATEST=2.2.0 run bash -c "printf 'y\nn\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
    [ ! -e "$SB/sistem/Helpers/New.sh" ]
}
@test "CLI --update: dizin adı doğru ama içindeki VERSION farklıysa (imzalı olsa bile) KURULMAZ" {
    make_cli_sandbox
    # Geçerli imzalı, üst dizin sshpocket-3.0.0/ ama VERSION dosyası 2.1.5 diyor
    tree="$T/rel2/sshpocket-3.0.0"; mkdir -p "$tree/sistem/Helpers"
    printf '2.1.5\n' > "$tree/VERSION"
    printf '## [3.0.0] - 2026-01-01\n- x\n' > "$tree/CHANGELOG.md"
    printf 'echo x\n' > "$tree/sistem/Helpers/New.sh"
    tar -czf "$T/serve/sshpocket-3.0.0.tar.gz" -C "$T/rel2" sshpocket-3.0.0
    ssh-keygen -q -Y sign -f "$SIGNKEY" -n sshpocket-release "$T/serve/sshpocket-3.0.0.tar.gz"
    FAKE_LATEST=3.0.0 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    [[ "$output" == *"uyuşmuyor"* ]]
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
    [ ! -e "$SB/sistem/Helpers/New.sh" ]
}

@test "apply: arşivde servers/ olsa bile kullanıcının servers/ klasörü silinmez ve ezilmez" {
    make_install
    mkdir -p "$INST/servers/alpha"
    printf 'IP=192.0.2.7\n' > "$INST/servers/alpha/server.env"
    printf 'GIZLI-KEY\n' > "$INST/servers/alpha/id_ed25519_alpha"
    make_release 2.2.0
    mkdir "$T/x"; tar -xzf "$T/serve/sshpocket-2.2.0.tar.gz" -C "$T/x"
    # yanlışlıkla arşive servers/ girmiş olsun
    mkdir -p "$T/x/sshpocket-2.2.0/servers"
    printf 'SAHTE\n' > "$T/x/sshpocket-2.2.0/servers/README.md"
    run update_apply "$T/x/sshpocket-2.2.0" "$INST" 2.1.0
    [ "$status" -eq 0 ]
    [ "$(cat "$INST/servers/alpha/server.env")" = "IP=192.0.2.7" ]
    [ "$(cat "$INST/servers/alpha/id_ed25519_alpha")" = "GIZLI-KEY" ]
    [ ! -e "$INST/servers/README.md" ]
}

# ---------- 1.4: sadeleşen kurulum ----------
@test "apply: sistem/templates ve kod değişir, eski templates silinir; Modules (eski sunucular) ve servers/ korunur" {
    make_install
    mkdir -p "$INST/sistem/templates" "$INST/servers/alpha"
    printf 'ESKI\n' > "$INST/sistem/templates/eski.example"
    printf 'IP=192.0.2.7\n' > "$INST/servers/alpha/server.env"
    make_release 2.2.0
    mkdir "$T/x"; tar -xzf "$T/serve/sshpocket-2.2.0.tar.gz" -C "$T/x"
    mkdir -p "$T/x/sshpocket-2.2.0/sistem/templates"
    printf 'YENI\n' > "$T/x/sshpocket-2.2.0/sistem/templates/server.env.example"
    run update_apply "$T/x/sshpocket-2.2.0" "$INST" 2.1.0
    [ "$status" -eq 0 ]
    [ ! -e "$INST/sistem/templates/eski.example" ]
    [ "$(cat "$INST/sistem/templates/server.env.example")" = "YENI" ]
    [ "$(cat "$INST/sistem/Modules/mine/.env")" = "MINE_IP=192.0.2.1" ]
    [ "$(cat "$INST/servers/alpha/server.env")" = "IP=192.0.2.7" ]
}
@test "apply: sshpocket.conf (kullanıcı ayarları) güncellemede korunur" {
    make_install
    printf 'STATUS=off\n' > "$INST/sshpocket.conf"
    make_release 2.2.0
    mkdir "$T/x"; tar -xzf "$T/serve/sshpocket-2.2.0.tar.gz" -C "$T/x"
    run update_apply "$T/x/sshpocket-2.2.0" "$INST" 2.1.0
    [ "$status" -eq 0 ]
    [ "$(cat "$INST/sshpocket.conf")" = "STATUS=off" ]
}
@test "eski, yapılandırılmamış example şablonu güncellemede temizlenir" {
    make_install
    rm -f "$INST/sistem/Modules/example/.env"                       # kullanıcı hiç yapılandırmadı
    printf '#\n' > "$INST/sistem/Modules/example/ExampleModule.sh"
    printf 'EXAMPLE_IP=\n' > "$INST/sistem/Modules/example/.env.example"
    printf 'finder' > "$INST/sistem/Modules/example/.DS_Store"
    run update_cleanup_stale_example "$INST"
    [ "$status" -eq 0 ]
    [ ! -e "$INST/sistem/Modules/example" ]
    [ -d "$INST/sistem/Modules/mine" ]                              # diğer sunucu dokunulmadı
}
@test "yapılandırılmış (.env'li) example güncellemede SİLİNMEZ" {
    make_install                                                    # example/.env mevcut
    run update_cleanup_stale_example "$INST"
    [ -f "$INST/sistem/Modules/example/.env" ]
}
@test "example içinde key veya başka kullanıcı dosyası varsa SİLİNMEZ" {
    make_install
    rm -f "$INST/sistem/Modules/example/.env"
    printf 'KEY' > "$INST/sistem/Modules/example/id_ed25519_example"
    run update_cleanup_stale_example "$INST"
    [ -f "$INST/sistem/Modules/example/id_ed25519_example" ]
    rm -f "$INST/sistem/Modules/example/id_ed25519_example"
    printf 'not' > "$INST/sistem/Modules/example/notlarim.txt"
    run update_cleanup_stale_example "$INST"
    [ -f "$INST/sistem/Modules/example/notlarim.txt" ]
}

# ---------- özet tablosu yardımcıları ----------
@test "kilit farkı: eklenen, kalkan, değişen ve aynı paketler; yorum satırları atlanır" {
    printf '# [yorum]\n[a]\nversion = 1.0.0\n\n[b]\nversion = 2.0.0\n\n[c]\nversion = 3.0.0\n' > "$T/old.lock"
    printf '[a]\nversion = 1.0.0\n\n[b]\nversion = 2.1.0\n\n[d]\nversion = 4.0.0\n' > "$T/new.lock"
    run update_lock_diff "$T/old.lock" "$T/new.lock"
    [ "$output" = $'a 1.0.0 1.0.0\nb 2.0.0 2.1.0\nc 3.0.0 -\nd - 4.0.0' ]
    run update_lock_diff "$T/yok.lock" "$T/new.lock"            # eski kilit yoksa hepsi yeni
    [ "$output" = $'a - 1.0.0\nb - 2.1.0\nd - 4.0.0' ]
}

@test "Dikkat bölümü: yalnızca ilgili sürümün '### Dikkat' satırları; denetim karakteri atılır; yoksa boş" {
    printf '## [Unreleased]\n\n### Dikkat\n- yayımlanmamış\n\n## [2.2.0] - x\n\n### Dikkat\n- önemli \033[31mkırmızı\n- ikinci\n\n### Eklendi\n- şey\n\n## [2.1.0] - y\n\n### Dikkat\n- eski\n' > "$T/cl.md"
    run update_attention "$T/cl.md" 2.2.0
    [ "$output" = $'- önemli [31mkırmızı\n- ikinci' ]
    run update_attention "$T/cl.md" 9.9.9
    [ -z "$output" ]
}

# ---------- bölgeli panel (ortak UI) ----------
@test "CLI --update (30x80): üst bölüm sabit çizilir, notlar/sorular/kurulum kaydırma bölgesinde akar, bölge sonda sıfırlanır" {
    make_cli_sandbox; make_release 2.2.0
    FAKE_LATEST=2.2.0 SSHPOCKET_ROWS=30 SSHPOCKET_COLS=80 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -eq 0 ]
    [[ "$output" == *$'\033[3;1H'* && "$output" == *"Güncelleme · sshpocket 2.1.0 → 2.2.0"* ]] || { echo "$output" | cat -v | head -20; false; }
    [[ "$output" == *"Özet"* && "$output" == *"Dikkat"* && "$output" == *"davranış değişti 2.2.0"* ]] || { echo "$output"; false; }
    [[ "$output" == *"0.11.0 →"* && "$output" == *"yeni özellik 2.2.0"* && "$output" == *"2.1.0 → 2.2.0"* ]] || { echo "$output"; false; }
    [[ "$output" == *"0.12.0"* && "$output" == *"yeni-paket 1.0.0 (yeni)"* && "$output" == *"kilitten"* ]] || { echo "$output" | cat -v; false; }
    [[ "$output" == *$'\033[r'* ]]                         # bölge sıfırlandı
    [ "$(cat "$SB/VERSION")" = "2.2.0" ]
}

@test "CLI --update (kısa terminal 20x80): bölge çizilmez, düz akış ve özet aynen çıkar" {
    make_cli_sandbox; make_release 2.2.0
    FAKE_LATEST=2.2.0 SSHPOCKET_ROWS=20 SSHPOCKET_COLS=80 run bash -c "printf 'y\ny\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Güncelleme özeti"* && "$output" != *$'\033[3;1H'* ]] || { echo "$output" | cat -v | head -20; false; }
    [ "$(cat "$SB/VERSION")" = "2.2.0" ]
}

@test "CLI --update (panel): kurulumu reddetmek bölgeyi yine de sıfırlar, dosyalar değişmez" {
    make_cli_sandbox; make_release 2.2.0
    FAKE_LATEST=2.2.0 SSHPOCKET_ROWS=30 SSHPOCKET_COLS=80 run bash -c "printf 'y\nn\n' | bash '$SB/sistem/Main.sh' --update"
    [ "$status" -ne 0 ]
    [[ "$output" == *"İptal edildi."* && "$output" == *$'\033[r'* ]] || { echo "$output" | cat -v | tail; false; }
    [ "$(cat "$SB/VERSION")" = "2.1.0" ]
}

@test "panel: özet üst bölüme sığmazsa hiçbir bilgi kaybolmaz, tamamı kaydırma bölgesine düz yazılır" {
    tree="$T/tr"; mkdir -p "$tree" "$T/root"
    printf '## [2.2.0] - x\n\n### Dikkat\n- önemli\n' > "$tree/CHANGELOG.md"
    for i in $(seq 1 15); do printf '[paket-%s]\nversion = 1.0.%s\n\n' "$i" "$i"; done > "$tree/components.lock"
    SSHPOCKET_ROWS=30 SSHPOCKET_COLS=80 run update_panel_begin "$tree" "$T/root" 2.1.0 2.2.0
    [ "$status" -eq 0 ]
    [[ "$output" == *"Güncelleme özeti"* && "$output" == *"paket-15 1.0.15 (yeni)"* && "$output" == *"önemli"* ]] || { echo "$output" | cat -v | tail; false; }
}
