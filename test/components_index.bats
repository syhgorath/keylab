#!/usr/bin/env bats
# İmzalı paket indeksi istemcisi (docs/package-index.md §6, §12): sahte paket + sahte indeks, gerçek ağ YOK
load test_helper

NOW=1800000000

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsManifest.sh"
    source "$H/ComponentsCLI.sh"
    source "$H/ComponentsIndex.sh"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_COMPONENTS_LOCK="$T/components.lock"
    export SSHPOCKET_COMPONENTS_DIR="$T/components"
    export SSHPOCKET_INDEX_SIGNERS="$T/index_signers"
    export SSHPOCKET_INDEX_NOW=$NOW
    export SCRIPT_VERSION=3.7.0
    mkdir -p "$T/srv"
    ssh-keygen -q -t ed25519 -N "" -f "$T/good" -C pkg      # paket imzacısı (kilitte)
    ssh-keygen -q -t ed25519 -N "" -f "$T/evil" -C evil
    ssh-keygen -q -t ed25519 -N "" -f "$T/idx" -C index     # indeks imzacısı
    PUB=$(cut -d' ' -f1,2 "$T/good.pub")
    printf 'sshpocket-index namespaces="sshpocket-index" %s\n' "$(cut -d' ' -f1,2 "$T/idx.pub")" > "$T/index_signers"
    comp_index_download() { cp "$T/srv/$(basename "$1")" "$2" 2>/dev/null; }
    comp_download() { cp "$T/$(basename "$1")" "$2" 2>/dev/null; }
}

# make_pkg <sürüm> [imzalayan-anahtar] → $T/demo-<sürüm>.tar.gz(+.sig); sha256'yı SHA_<sürüm-noktasız>'e koyar değil, echo eder
make_pkg() {
    local ver="$1" key="${2:-good}" a="$T/demo-$1.tar.gz"
    mkdir -p "$T/pkg/demo-$ver"
    printf '{"id":"demo","version":"%s","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"p","status":"supported"}],"actions":{"a":{"cmd":"run.sh","effects":[]}}}\n' "$ver" > "$T/pkg/demo-$ver/component.json"
    tar -czf "$a" -C "$T/pkg" "demo-$ver"
    rm -f "$a.sig"
    ssh-keygen -q -Y sign -f "$T/$key" -n sshpocket-component "$a"
    comp_sha256_of "$a"
}

set_lock() {   # sürüm sha
    printf '[demo]\nrepo = syhgorath/demo\nversion = %s\nsha256 = %s\nsigners = sshpocket-component namespaces="sshpocket-component" %s\n' "$1" "$2" "$PUB" > "$SSHPOCKET_COMPONENTS_LOCK"
}

# idx <serial> <issued-farkı-sn> <geçerlilik-gün> <demo-girdisi-json|-> [imza-anahtarı] [imza-ad-alanı]
idx() {
    local serial="$1" delta="$2" days="$3" entry="$4" key="${5:-idx}" ns="${6:-sshpocket-index}"
    python3 -I - "$T/srv/index.json" "$serial" "$((NOW + delta))" "$days" "$entry" <<'PY'
import json, sys, time
path, serial, issued, days, entry = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), int(sys.argv[4]), sys.argv[5]
f = lambda t: time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t))
d = {"schema": 1, "serial": serial, "issued_at": f(issued), "valid_until": f(issued + days * 86400),
     "packages": ({} if entry == "-" else {"demo": json.loads(entry)})}
open(path, "w").write(json.dumps(d, sort_keys=True, indent=2) + "\n")
PY
    sign_idx "$key" "$ns"
}

sign_idx() {
    rm -f "$T/srv/index.json.sig"
    ssh-keygen -q -Y sign -f "$T/${1:-idx}" -n "${2:-sshpocket-index}" "$T/srv/index.json"
}

# entry <sürüm> <sha> [ek-json-alanları ", \"yanked\": true"]
entry() { printf '{"repo":"syhgorath/demo","version":"%s","asset":"demo-%s.tar.gz","sha256":"%s"%s}' "$1" "$1" "$2" "${3:-}"; }

# kurulu 1.0.0 (kilit 1.0.0) + 1.1.0 yayımlı
base_setup() {
    SHA10=$(make_pkg 1.0.0); SHA11=$(make_pkg 1.1.0)
    set_lock 1.0.0 "$SHA10"
    comp_install_archive demo 1.0.0 "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig" >/dev/null
}

@test "update: geçerli indeks kabul edilir, yükseltilebilir listelenir, durum yazılır, kurulum yapılmaz" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_update
    [ "$status" -eq 0 ]
    [[ "$output" == *"seri 5"* ]]
    [[ "$output" == *"yükseltilebilir → 1.1.0"* ]]
    [ "$(comp_index_state_get serial)" = "5" ]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "indeks: yanlış anahtarla imzalı indeks reddedilir, durum yazılmaz" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")" evil
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"imzası geçersiz"* ]]
    [ ! -e "$T/components/.index/state" ]
}

@test "indeks: paket ad alanıyla (sshpocket-component) atılmış imza indeksi geçmez" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")" idx sshpocket-component
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"imzası geçersiz"* ]]
}

@test "indeks: imza sonrası değiştirilmiş içerik reddedilir" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    printf ' ' >> "$T/srv/index.json"
    run comp_cli_update
    [ "$status" -ne 0 ]
}

@test "indeks: eski seri reddedilir (geri alma); aynı indeks yeniden alınabilir" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    comp_cli_update >/dev/null
    run comp_cli_update
    [ "$status" -eq 0 ]
    idx 4 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"eski indeks"* ]]
    [ "$(comp_index_state_get serial)" = "5" ]
}

@test "indeks: aynı seri, farklı içerik reddedilir" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    comp_cli_update >/dev/null
    idx 5 0 60 "$(entry 1.1.0 "$SHA11" ', "yanked": true')"
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"iki farklı içerik"* ]]
}

@test "indeks: seri ilerleyince durum güncellenir" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"; comp_cli_update >/dev/null
    idx 6 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_update
    [ "$status" -eq 0 ]
    [ "$(comp_index_state_get serial)" = "6" ]
}

@test "indeks: süresi dolmuş indeks reddedilir" {
    base_setup; idx 5 -$((70 * 86400)) 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"süresi dolmuş"* ]]
}

@test "indeks: valid_until 90 günden uzunsa reddedilir" {
    base_setup; idx 5 0 120 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"90 gün"* ]]
}

@test "indeks: gelecekte imzalanmış (issued_at > şimdi+1 gün) reddedilir" {
    base_setup; idx 5 $((3 * 86400)) 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"gelecekte"* ]]
}

@test "indeks: yinelenen JSON anahtarı reddedilir" {
    base_setup
    python3 -I - "$T/srv/index.json" <<'PY'
import sys, time
f = lambda t: time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t))
open(sys.argv[1], "w").write('{"schema":1,"schema":1,"serial":5,"issued_at":"%s","valid_until":"%s","packages":{}}\n' % (f(1800000000), f(1800000000 + 86400)))
PY
    sign_idx
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"reddedildi"* ]]
}

@test "indeks: bilinmeyen schema reddedilir" {
    base_setup; idx 5 0 60 -
    sed -i.bak 's/"schema": 1/"schema": 2/' "$T/srv/index.json"; sign_idx
    run comp_cli_update
    [ "$status" -ne 0 ]
    [[ "$output" == *"schema 2"* ]]
}

@test "indeks: asset adı <id>-<sürüm>.tar.gz değilse reddedilir" {
    base_setup; idx 5 0 60 '{"repo":"syhgorath/demo","version":"1.1.0","asset":"evil.tar.gz","sha256":"'"$SHA11"'"}'
    run comp_cli_update
    [ "$status" -ne 0 ]
}

@test "indeks: kilit tabanının altındaki sürüm yok sayılır ve uyarılır" {
    base_setup; idx 5 0 60 "$(entry 0.9.0 "$SHA10")"
    run comp_cli_update
    [ "$status" -eq 0 ]
    [[ "$output" == *"kilit tabanının altında"* ]]
    [[ "$output" != *"yükseltilebilir"* ]]
}

@test "indeks: kilit sürümünde özet çelişirse paket yükseltilmez" {
    base_setup; idx 5 0 60 "$(entry 1.0.0 "$SHA11")"
    run comp_cli_update
    [[ "$output" == *"çelişiyor"* ]]
    run comp_cli_upgrade --yes
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "indeks: yanked sürüm önerilmez, kurulmaz" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11" ', "yanked": true')"
    run comp_cli_upgrade --yes
    [[ "$output" == *"yanked"* ]]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "indeks: requires_sshpocket çalışan sürümden büyükse önerilmez" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11" ', "requires_sshpocket": "9.0.0"')"
    run comp_cli_update
    [[ "$output" == *"sshpocket 9.0.0 gerekli"* ]]
    [[ "$output" != *"yükseltilebilir"* ]]
}

@test "indeks: indeksin gösterdiği depo kilitteki depodan farklıysa yok sayılır" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11" | sed 's#syhgorath/demo#evil/demo#')"
    run comp_cli_update
    [[ "$output" == *"başka depo"* ]]
    [[ "$output" != *"yükseltilebilir"* ]]
}

@test "indeks: kilitte olmayan paket yok sayılır (yeni paket tanıtılamaz)" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11" | sed 's#"repo"#"x":1,"repo"#')"
    python3 -I - "$T/srv/index.json" <<'PY'
import json, sys
p = sys.argv[1]; d = json.load(open(p)); d["packages"]["yeni"] = dict(d["packages"]["demo"], version="1.0.0", asset="yeni-1.0.0.tar.gz")
open(p, "w").write(json.dumps(d, sort_keys=True, indent=2) + "\n")
PY
    sign_idx
    run comp_cli_update
    [ "$status" -eq 0 ]
    [[ "$output" != *"yeni "* ]]
    run comp_cli_upgrade --yes
    [ ! -e "$T/components/yeni" ]
}

@test "upgrade: --yes ile kurulur; kilit 1.0.0 iken kurulu 1.1.0 paketi çalıştırma kapısından geçer" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_upgrade --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"yükseltilen: 1, atlanan: 0, hata: 0"* ]]
    [ "$(comp_installed_version demo)" = "1.1.0" ]
    [ -d "$T/components/demo/1.0.0" ]
    run comp_prepare demo
    [ "$status" -eq 0 ]
    [ "$(comp_status demo)" = "ahead" ]
    run comp_cli_upgrade --yes
    [[ "$output" == *"Yükseltilebilir paket yok"* ]]
}

@test "upgrade: onay verilmezse kurulmaz (varsayılan sessiz kurulum yok)" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_upgrade <<< "n"
    [ "$status" -ne 0 ]
    [[ "$output" == *"Atlandı: demo"* ]]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "upgrade: onay verilirse kurulur" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_upgrade <<< "y"
    [ "$status" -eq 0 ]
    [ "$(comp_installed_version demo)" = "1.1.0" ]
}

@test "upgrade: arşiv özeti indeksle uyuşmazsa kurulmaz, current değişmez" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$(printf '%064d' 7)")"
    run comp_cli_upgrade --yes
    [ "$status" -ne 0 ]
    [[ "$output" == *"hata: 1"* ]]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
    [ ! -e "$T/components/demo/1.1.0" ]
}

@test "upgrade: paket imzası kilitteki imzacıyla doğrulanmazsa kurulmaz (indeks imzacı getiremez)" {
    base_setup
    SHA11=$(make_pkg 1.1.0 evil)
    idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_upgrade --yes
    [ "$status" -ne 0 ]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "upgrade: geri sürüme düşülmez (kurulu 1.1.0, indeks 1.0.0)" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    comp_cli_upgrade --yes >/dev/null
    idx 6 0 60 "$(entry 1.0.0 "$SHA10")"
    run comp_cli_upgrade --yes
    [ "$(comp_installed_version demo)" = "1.1.0" ]
    run comp_install_archive demo 1.0.0 "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig" "$SHA10"
    [ "$status" -ne 0 ]
    [[ "$output" == *"geri sürüme"* ]]
}

@test "upgrade: indirme başarısızsa eski sürüm durur, seri yine de kaydedilir" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    rm -f "$T/demo-1.1.0.tar.gz"
    run comp_cli_upgrade --yes
    [ "$status" -ne 0 ]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
    [ "$(comp_index_state_get serial)" = "5" ]
}

@test "upgrade: yalnızca istenen paket yükseltilir; geçersiz kimlik/seçenek reddedilir" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    run comp_cli_upgrade --bogus
    [ "$status" -eq 2 ]
    run comp_cli_upgrade --yes "../x"
    [ "$status" -eq 2 ]
    run comp_cli_upgrade --yes baska
    [[ "$output" == *"yükseltilen: 0"* ]]
    [ "$(comp_installed_version demo)" = "1.0.0" ]
}

@test "kurulum: beklenen özet olmadan kilitten yeni sürüm reddedilir" {
    base_setup
    run comp_install_archive demo 1.1.0 "$T/demo-1.1.0.tar.gz" "$T/demo-1.1.0.tar.gz.sig"
    [ "$status" -ne 0 ]
    [[ "$output" == *"sürüm kilitle uyuşmuyor"* ]]
}

@test "kurulum: kilit sürümüne eşit sürümde beklenen özet kilitteki ile aynı olmalı" {
    base_setup
    run comp_install_archive demo 1.0.0 "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig" "$SHA11"
    [ "$status" -ne 0 ]
}

@test "kurulum: kilit tabanının altındaki sürüm beklenen özetle de kurulamaz" {
    SHA10=$(make_pkg 1.0.0); SHA09=$(make_pkg 0.9.0)
    set_lock 1.0.0 "$SHA10"
    run comp_install_archive demo 0.9.0 "$T/demo-0.9.0.tar.gz" "$T/demo-0.9.0.tar.gz.sig" "$SHA09"
    [ "$status" -ne 0 ]
    [[ "$output" == *"kilit tabanının altında"* ]]
}

@test "çalıştırma kapısı: kurulu sürüm kilit tabanının altındaysa reddedilir" {
    base_setup
    set_lock 1.1.0 "$SHA11"
    run comp_prepare demo
    [ "$status" -ne 0 ]
    [[ "$output" == *"kilit tabanının altında"* ]]
}

@test "kapalı: index_signers yalnızca yorumsa özellik kapalı, ağa çıkılmaz" {
    base_setup
    printf '# boş\n' > "$T/index_signers"
    comp_index_download() { echo "AĞ ÇAĞRILDI" >&2; return 1; }
    run comp_cli_update
    [ "$status" -eq 0 ]
    [[ "$output" == *"indeksi kapalı"* || "$output" == *"indeksi"*"kapalı"* ]]
    [[ "$output" != *"AĞ ÇAĞRILDI"* ]]
}

@test "ağ hatası: indeks alınamazsa çıkış 1, kurulu paket çalışmaya devam eder" {
    base_setup
    rm -f "$T/srv/index.json" "$T/srv/index.json.sig"
    run comp_cli_update
    [ "$status" -eq 1 ]
    [[ "$output" == *"indeks alınamadı"* ]]
    run comp_prepare demo
    [ "$status" -eq 0 ]
}

@test "list: önbellekteki indeks yeniden doğrulanıp gösterilir; ağ yok" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    comp_cli_update >/dev/null
    comp_index_download() { echo "AĞ ÇAĞRILDI" >&2; return 1; }
    run comp_cli_list
    [ "$status" -eq 0 ]
    [[ "$output" == *"indeks: ⬆️  yükseltilebilir → 1.1.0"* ]]
    [[ "$output" != *"AĞ ÇAĞRILDI"* ]]
}

@test "list: önbellekteki indeks süresi dolduysa nedeni söyler, yükseltme önermez" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    comp_cli_update >/dev/null
    export SSHPOCKET_INDEX_NOW=$((NOW + 100 * 86400))
    run comp_cli_list
    [ "$status" -eq 0 ]
    [[ "$output" == *"önbelleği kullanılamıyor"* ]]
    [[ "$output" != *"yükseltilebilir"* ]]
}

@test "list: önbelleğin imzası bozulmuşsa kullanılmaz" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    comp_cli_update >/dev/null
    printf ' ' >> "$T/components/.index/index.json"
    run comp_cli_list
    [[ "$output" == *"imzası geçersiz"* ]]
    [[ "$output" != *"yükseltilebilir"* ]]
}

@test "list: indeks önbelleği yoksa çıktı bugünküyle aynı (indeks satırı yok)" {
    base_setup
    run comp_cli_list
    [ "$status" -eq 0 ]
    [[ "$output" != *"indeks"* ]]
}

@test "durum dosyası bozuksa ilk kullanım sayılır ve yeniden yazılır" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    mkdir -p "$T/components/.index"; printf 'serial=abc\n' > "$T/components/.index/state"
    run comp_cli_update
    [ "$status" -eq 0 ]
    [ "$(comp_index_state_get serial)" = "5" ]
}

@test "prune: .index klasörü paket sayılmaz, silinmez" {
    base_setup; idx 5 0 60 "$(entry 1.1.0 "$SHA11")"
    comp_cli_update >/dev/null
    run comp_cli_prune --yes
    [ "$status" -eq 0 ]
    [ -f "$T/components/.index/state" ]
}

@test "ana komut: --components update/upgrade kullanımda yazılı" {
    run bash -c "source '$H/Components.sh'; source '$H/ComponentsInstall.sh'; source '$H/ComponentsManifest.sh'; source '$H/ComponentsCLI.sh'; source '$H/ComponentsIndex.sh'; components_cli bogus"
    [ "$status" -eq 2 ]
    [[ "$output" == *"update"* && "$output" == *"upgrade"* ]]
}
