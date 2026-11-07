#!/usr/bin/env bats
# Aşama 2.4a: --components list/sync/check-new ve --update entegrasyonu (sahte paketler, ağ yok)
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

# make_pkg <id> <sürüm> → $T/<id>-<sürüm>.tar.gz(+.sig); sha256'yı $SHA_<…> yerine echo eder
make_pkg() {
    local id="$1" ver="$2" a="$T/$1-$2.tar.gz"
    mkdir -p "$T/pkg/$id-$ver"
    printf '{"id":"%s","version":"%s","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"p","status":"supported"}],"actions":{"a":{"cmd":"run.sh","effects":[]}}}\n' "$id" "$ver" > "$T/pkg/$id-$ver/component.json"
    tar -czf "$a" -C "$T/pkg" "$id-$ver"
    rm -f "$a.sig"
    ssh-keygen -q -Y sign -f "$T/good" -n sshpocket-component "$a"
    comp_sha256_of "$a"
}

# lock_entry <id> <sürüm> <sha>  (ekler)
lock_entry() {
    printf '[%s]\nrepo = syhgorath/%s\nversion = %s\nsha256 = %s\nsigners = sshpocket-component namespaces="sshpocket-component" %s\n\n' \
        "$1" "$1" "$2" "$3" "$PUB" >> "$SSHPOCKET_COMPONENTS_LOCK"
}

fake_download() {   # comp_download <url> <hedef>: dosya adını URL'den çıkarıp $T'den kopyalar
    comp_download() { cp "$T/$(basename "$1")" "$2" 2>/dev/null; }
}

@test "list: kilit boşsa 'paket yok' der, hata vermez" {
    : > "$SSHPOCKET_COMPONENTS_LOCK"
    run components_cli list
    [ "$status" -eq 0 ]
    [[ "$output" == *"paket yok"* ]]
}

@test "list: eksik / kurulu / eski / kilitten yeni / bozuk durumları doğru gösterir" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$sha"
    run components_cli list
    [[ "$output" == *"kurulmamış"* ]]
    comp_install_archive demo 1.0.0 "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig" >/dev/null
    run components_cli list;  [[ "$output" == *"✅ kurulu"* ]]
    # kilit ilerler → eski
    sha2=$(make_pkg demo 1.1.0); : > "$SSHPOCKET_COMPONENTS_LOCK"; lock_entry demo 1.1.0 "$sha2"
    run components_cli list;  [[ "$output" == *"güncellenmeli"* ]]
    # kilit geri gelir (kurulu 1.0.0 > ... aslında kurulu 1.0.0, kilit 0.9.0 → kilitten yeni)
    : > "$SSHPOCKET_COMPONENTS_LOCK"; sha0=$(make_pkg demo 0.9.0); lock_entry demo 0.9.0 "$sha0"
    run components_cli list;  [[ "$output" == *"kilitten yeni"* ]]
    # bozulma
    : > "$SSHPOCKET_COMPONENTS_LOCK"; lock_entry demo 1.0.0 "$sha"
    printf 'x' >> "$T/components/demo/1.0.0/component.json"
    run components_cli list;  [[ "$output" == *"dosyalar değişmiş"* ]]
}

@test "sync: eksik paket onayla kurulur; onay verilmezse kurulmaz" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$sha"; fake_download
    run bash -c 'true'                       # (bats: aşağıdaki çağrılar altkabukta)
    run components_cli sync <<< "n"
    [ "$status" -ne 0 ]
    [ -z "$(comp_installed_version demo)" ]
    run components_cli sync <<< "y"
    [ "$status" -eq 0 ]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "sync --yes: sormadan kurar; ikinci çalıştırma hiçbir şey yapmaz" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$sha"; fake_download
    run components_cli sync --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"✅ demo 1.0.0"* ]]
    run components_cli sync --yes
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "sync: kilit ilerleyince eski sürümden yükseltir, eski sürüm klasörü kalır" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$sha"; fake_download
    components_cli sync --yes >/dev/null
    sha2=$(make_pkg demo 1.1.0); : > "$SSHPOCKET_COMPONENTS_LOCK"; lock_entry demo 1.1.0 "$sha2"
    run components_cli sync --yes
    [ "$status" -eq 0 ]
    [ "$(comp_installed_version demo)" = "1.1.0" ]
    [ -d "$T/components/demo/1.0.0" ]
}

@test "sync: bozuk kurulum onayla kenara alınır (silinmez) ve yeniden kurulur" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$sha"; fake_download
    components_cli sync --yes >/dev/null
    printf 'KURCALANDI' >> "$T/components/demo/1.0.0/component.json"
    run components_cli sync <<< "n"
    [ "$status" -ne 0 ]
    [ "$(comp_status demo)" = "broken" ]
    run components_cli sync <<< "y"
    [ "$status" -eq 0 ]
    [ "$(comp_status demo)" = "ok" ]
    [ -n "$(ls -d "$T"/components/demo/1.0.0.bozuk-* 2>/dev/null)" ]     # kurcalanmış kopya saklandı
    grep -q KURCALANDI "$T"/components/demo/1.0.0.bozuk-*/component.json
}

@test "sync: sahte paket (kilitle uyuşmayan sha256) KURULMAZ, hata bildirilir" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$(printf '0%.0s' $(seq 1 64))"; fake_download
    run components_cli sync --yes
    [ "$status" -ne 0 ]
    [ -z "$(comp_installed_version demo)" ]
    [[ "$output" == *"kurulamadı"* ]]
}

@test "sync: bir paket başarısız olsa da diğerleri kurulur, çıkış kodu hata" {
    shaA=$(make_pkg alpha 1.0.0); shaB=$(make_pkg beta 1.0.0)
    lock_entry alpha 1.0.0 "$(printf '1%.0s' $(seq 1 64))"; lock_entry beta 1.0.0 "$shaB"; fake_download
    run components_cli sync --yes
    [ "$status" -ne 0 ]
    [ -z "$(comp_installed_version alpha)" ]
    [ "$(comp_installed_version beta)" = "1.0.0" ]
}

@test "check-new: daha yeni yayın bilgi olarak gösterilir, KURULMAZ" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$sha"
    comp_http_head() { printf 'https://github.com/syhgorath/demo/releases/tag/v1.2.0'; }
    run components_cli check-new
    [ "$status" -eq 0 ]
    [[ "$output" == *"1.2.0 yayınlanmış"* ]]
    [[ "$output" == *"şimdi kurulmaz"* ]]
    [ -z "$(comp_installed_version demo)" ]
    comp_http_head() { printf 'https://github.com/syhgorath/demo/releases/tag/v1.0.0'; }
    run components_cli check-new
    [[ "$output" == *"güncel"* ]]
}

@test "check-new: beklenmeyen/sahte yönlendirme yok sayılır" {
    sha=$(make_pkg demo 1.0.0); lock_entry demo 1.0.0 "$sha"
    comp_http_head() { printf 'https://evil.example/releases/tag/v9.9.9'; }
    run components_cli check-new
    [[ "$output" == *"alınamadı"* ]]
    comp_http_head() { printf 'https://github.com/syhgorath/demo/releases/tag/latest'; }
    run components_cli check-new
    [[ "$output" == *"alınamadı"* ]]
}

@test "CLI: --components list ve bilinmeyen alt komut; kilit yoksa sessiz" {
    run bash "$ROOT/sistem/Main.sh" --components list
    [ "$status" -eq 0 ]
    run bash "$ROOT/sistem/Main.sh" --components bilinmiyor
    [ "$status" -eq 2 ]
}

# --update zinciri: çekirdek güncellemesi başarılıysa YENİ kodla paketler kilide göre eşitlenir
make_update_sandbox() {
    make_sandbox                                   # $SANDBOX
    printf '#!/bin/bash\nexit %s\n' "$1" > "$SANDBOX/sistem/Update.sh"
    sha=$(make_pkg demo 1.0.0)
    unset SSHPOCKET_COMPONENTS_LOCK SSHPOCKET_COMPONENTS_DIR
    printf '[demo]\nrepo = syhgorath/demo\nversion = 1.0.0\nsha256 = %s\nsigners = sshpocket-component namespaces="sshpocket-component" %s\n' \
        "$sha" "$PUB" > "$SANDBOX/components.lock"
    mkdir -p "$SANDBOX/bin"
    # sahte curl: -o <hedef> ... <url>  → $T/<dosya adı>
    cat > "$SANDBOX/bin/curl" <<CURL
#!/bin/bash
out=""; url=""
while [ \$# -gt 0 ]; do case "\$1" in -o) out="\$2"; shift 2 ;; http*) url="\$1"; shift ;; *) shift ;; esac; done
echo "\$url" >> "$T/curl.log"
cp "$T/\$(basename "\$url")" "\$out"
CURL
    chmod +x "$SANDBOX/bin/curl"
}

@test "--update: çekirdek güncellemesi başarılıysa paketler kilide göre kurulur (onayla)" {
    make_update_sandbox 0
    run bash -c "printf 'y\n' | PATH='$SANDBOX/bin:$PATH' bash '$SANDBOX/sistem/Main.sh' --update"
    [ "$status" -eq 0 ]
    [ "$(cat "$SANDBOX/components/demo/current")" = "1.0.0" ]
    grep -q 'releases/download/v1.0.0/demo-1.0.0.tar.gz$' "$T/curl.log"
    grep -q 'releases/download/v1.0.0/demo-1.0.0.tar.gz.sig$' "$T/curl.log"
}

@test "--update: çekirdek güncellemesi başarısız/iptal ise paketlere dokunulmaz" {
    make_update_sandbox 1
    run bash -c "printf 'y\n' | PATH='$SANDBOX/bin:$PATH' bash '$SANDBOX/sistem/Main.sh' --update"
    [ "$status" -eq 1 ]
    [ ! -e "$SANDBOX/components" ]
    [ ! -e "$T/curl.log" ]
}

@test "--update: onay verilmezse paket kurulmaz ama çekirdek güncellemesi başarılı sayılır" {
    make_update_sandbox 0
    run bash -c "printf 'n\n' | PATH='$SANDBOX/bin:$PATH' bash '$SANDBOX/sistem/Main.sh' --update"
    [ ! -e "$SANDBOX/components/demo/current" ]
    [[ "$output" == *"Atlandı: demo"* ]]
}

@test "menü: Paketler girdisi yalnızca kilitte paket varsa görünür" {
    make_sandbox
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/diskutil"; chmod +x "$SANDBOX/bin/diskutil"
    run bash -c "printf '0\n' | PATH='$SANDBOX/bin:$PATH' TERM=xterm bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" != *"Paketler (durum"* ]]
    sha=$(make_pkg demo 1.0.0)
    printf '[demo]\nrepo = syhgorath/demo\nversion = 1.0.0\nsha256 = %s\nsigners = sshpocket-component namespaces="sshpocket-component" %s\n' \
        "$sha" "$PUB" > "$SANDBOX/components.lock"
    unset SSHPOCKET_COMPONENTS_LOCK
    run bash -c "printf '0\n' | PATH='$SANDBOX/bin:$PATH' TERM=xterm bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"Paketler (durum"* ]]
}

@test "check-new: kararlı 'latest' yoksa (ön sürüm) API'deki en yeni yayına bakar" {
    sha=$(make_pkg demo 0.1.0); lock_entry demo 0.1.0 "$sha"
    comp_http_head() { return 0; }                         # latest yönlendirmesi yok (yalnızca pre-release)
    comp_http_json() { printf '[\n  {\n    "tag_name": "v0.2.0",\n    "prerelease": true\n  }\n]\n'; }
    run components_cli check-new
    [ "$status" -eq 0 ]
    [[ "$output" == *"0.2.0 yayınlanmış"* ]]
}

@test "check-new: API yanıtı bozuk/beklenmedik etiketse yok sayılır" {
    sha=$(make_pkg demo 0.1.0); lock_entry demo 0.1.0 "$sha"
    comp_http_head() { return 0; }
    comp_http_json() { printf '[{"tag_name": "latest; rm -rf /"}]'; }
    run components_cli check-new
    [[ "$output" == *"alınamadı"* ]]
    comp_http_json() { printf '<html>rate limit</html>'; }
    run components_cli check-new
    [[ "$output" == *"alınamadı"* ]]
}
