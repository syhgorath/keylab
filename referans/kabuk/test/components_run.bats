#!/usr/bin/env bats
# Aşama 3, PR A: değişken doğrulama, inventory ve argüman dizisi üreteçleri (VM'e dokunmaz, ağ yok)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsManifest.sh"
    source "$H/ComponentsRun.sh"
    T="$BATS_TEST_TMPDIR"
    cat > "$T/m.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/harden.yml --check --diff","effects":[],"requires_sudo":true},
  "ask":{"cmd":"ansible-playbook playbooks/x.yml","effects":[],"interactive":true},
  "evil":{"cmd":"ansible-playbook playbooks/x.yml -i /etc/hosts","effects":[]},
  "evil2":{"cmd":"ansible-playbook playbooks/x.yml --limit=all","effects":[]}},
 "vars":[
  {"name":"allow_from","type":"list[cidr]","required":true},
  {"name":"scan_dir","type":"path"},
  {"name":"rb_enabled","type":"bool","default":true,"warn_if":false},
  {"name":"rb_timeout","type":"int","default":300,"min":30,"max":3600},
  {"name":"mode","type":"enum","values":["a","b"]},
  "legacy_name"],
 "invoke":{"append":[
  {"arg":"-i","value":"<inventory>","required":true},
  {"arg":"--limit","value":"<server>","required":true},
  {"arg":"--ask-become-pass","required":true,"interactive":true}]}}
JSON
    INFO=$(comp_manifest_info "$T/m.json" demo 1.0.0)
}

vars() { rm -f "$T/out.json"; comp_run_vars "$T/m.json" "$T/v" "$T/out.json" "$@"; }

# ---------- değişkenler ----------
@test "vars: geçerli değerler yazılır, varsayılanlar uygulanır, dosya izni 600, değer stdout'ta yok" {
    printf '# yorum\nallow_from=10.0.0.0/24, 192.168.1.5\nmode=b\n' > "$T/v"
    run vars
    [ "$status" -eq 0 ]
    [ "$output" = "ok" ]
    [ "$(file_mode "$T/out.json")" = "600" ]
    run python3 -I -c "import json;d=json.load(open('$T/out.json'));assert d['allow_from']==['10.0.0.0/24','192.168.1.5'];assert d['rb_enabled'] is True;assert d['rb_timeout']==300;assert d['mode']=='b';assert 'scan_dir' not in d"
    [ "$status" -eq 0 ]
}

@test "vars: zorunlu değişken eksikse reddedilir" {
    : > "$T/v"
    run vars
    [ "$status" -ne 0 ]
    [[ "$output" == *"allow_from: zorunlu"* ]] || false
    [ ! -e "$T/out.json" ]
}

@test "vars: --no-required zorunlu EKSİKLİĞİNİ hoşgörür (varsayılanlar yazılır) ama verilmiş geçersiz değeri ve bilinmeyen adı yine reddeder" {
    : > "$T/v"
    run vars --no-required
    [ "$status" -eq 0 ]
    run python3 -I -c "import json;d=json.load(open('$T/out.json'));assert 'allow_from' not in d;assert d['rb_enabled'] is True;assert d['rb_timeout']==300"
    [ "$status" -eq 0 ]
    rm -f "$T/out.json"
    printf 'allow_from=bu-cidr-degil\n' > "$T/v"
    run vars --no-required
    [ "$status" -ne 0 ]
    [ ! -e "$T/out.json" ]
    printf 'yok_boyle_degisken=1\n' > "$T/v"
    run vars --no-required
    [ "$status" -ne 0 ]
    [[ "$output" == *"pakette tanımlı bir değişken değil"* ]] || false
}

@test "vars: değerler dosyası yoksa yalnızca varsayılanlar/zorunlu denetimi (dosya yok = boş)" {
    rm -f "$T/v"
    run vars --set allow_from=10.0.0.0/8
    [ "$status" -eq 0 ]
}

@test "vars: --set dosyadaki değeri ezer (çalıştırıcının zorladığı değer)" {
    printf 'allow_from=10.0.0.0/8\nscan_dir=/usb/oops\n' > "$T/v"
    run vars --set scan_dir=/home/u/.local/share/x
    [ "$status" -eq 0 ]
    run python3 -I -c "import json;assert json.load(open('$T/out.json'))['scan_dir']=='/home/u/.local/share/x'"
    [ "$status" -eq 0 ]
}

@test "vars: tür/aralık hataları reddedilir (int aralık, bool, cidr, enum, boş liste öğesi)" {
    for bad in "rb_timeout=5" "rb_timeout=99999" "rb_timeout=abc" "rb_enabled=belki" "allow_from=10.0.0.0/99" "allow_from=evil" "allow_from=10.0.0.0/8,,1.2.3.4" "mode=zzz"; do
        printf 'allow_from=10.0.0.0/8\n%s\n' "$bad" > "$T/v"
        # aynı ad iki kez yazılırsa "yinelenen" hatası da bir ret sayılır; amaç tek sorunlu satır
        [ "${bad%%=*}" = "allow_from" ] && printf '%s\n' "$bad" > "$T/v"
        run vars
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
        [ ! -e "$T/out.json" ]
    done
}

@test "vars: pakette tanımsız ad, bozuk satır, yinelenen ad reddedilir" {
    for bad in "yok_boyle=1" "satirda esit yok" "Allow=1"; do
        printf 'allow_from=10.0.0.0/8\n%s\n' "$bad" > "$T/v"
        run vars
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
    done
    printf 'allow_from=10.0.0.0/8\nallow_from=10.0.0.0/8\n' > "$T/v"
    run vars
    [ "$status" -ne 0 ]
}

@test "vars: warn_if eşleşirse 'warn=<ad>' yazılır" {
    printf 'allow_from=10.0.0.0/8\nrb_enabled=false\n' > "$T/v"
    run vars
    [ "$status" -eq 0 ]
    [[ "$output" == *"warn=rb_enabled"* ]] || false
}

@test "vars: eski biçim (yalnızca ad) string sayılır; denetim karakteri reddedilir" {
    printf 'allow_from=10.0.0.0/8\nlegacy_name=merhaba\n' > "$T/v"
    run vars
    [ "$status" -eq 0 ]
    printf 'allow_from=10.0.0.0/8\n' > "$T/v"
    run vars --set "legacy_name=$(printf 'a\001b')"
    [ "$status" -ne 0 ]
}

@test "vars: çıktı dosyası zaten varsa üzerine yazmaz" {
    printf 'allow_from=10.0.0.0/8\n' > "$T/v"
    echo "eski" > "$T/out.json"
    run comp_run_vars "$T/m.json" "$T/v" "$T/out.json"
    [ "$status" -ne 0 ]
    [ "$(cat "$T/out.json")" = "eski" ]
}

# ---------- inventory ----------
@test "inventory: beklenen satırlar, izin 600" {
    comp_run_write_inventory "$T/inv" hardening ubuntutest 10.1.10.193 syhgorath 22 /Volumes/test/servers/ubuntutest/id_ed25519_x
    [ "$(file_mode "$T/inv")" = "600" ]
    [ "$(sed -n 1p "$T/inv")" = "[hardening]" ]
    [ "$(sed -n 2p "$T/inv")" = "ubuntutest ansible_host=10.1.10.193 ansible_user=syhgorath ansible_port=22 ansible_private_key_file='/Volumes/test/servers/ubuntutest/id_ed25519_x'" ]
}

@test "inventory: yol boşluk içerebilir (tırnaklanır)" {
    comp_run_write_inventory "$T/inv" g web1 h.example u 2222 "/Volumes/my usb/k"
    grep -q "ansible_private_key_file='/Volumes/my usb/k'" "$T/inv"
}

@test "inventory: geçersiz girdiler reddedilir ve dosya oluşmaz" {
    # her satır: grup|ad|host|kullanıcı|port|anahtar
    local line g n h u p k
    while IFS='|' read -r g n h u p k; do
        rm -f "$T/inv"
        run comp_run_write_inventory "$T/inv" "$g" "$n" "$h" "$u" "$p" "$k"
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $g|$n|$h|$u|$p|$k"; return 1; }
        [[ "$output" != *"command not found"* ]] || false
        [ ! -e "$T/inv" ]
    done <<'CASES'
bad-grp!|web1|h|u|22|/k
g|web1;x|h|u|22|/k
g|web1|h;rm|u|22|/k
g|web1|h|u v|22|/k
g|web1|h|u|0|/k
g|web1|h|u|70000|/k
g|web1|h|u|22|rel/key
g|web1|h|u|22|/k'x
g|web1|h|u|22|/k"x
g|web1|h|u|22|/k\x
CASES
    run comp_run_write_inventory "$T/inv" g web1 h u 22 "$(printf '/k\nansible_become=yes')"
    [ "$status" -ne 0 ]
    [ ! -e "$T/inv" ]
}

@test "inventory: var olan dosyanın üzerine yazmaz" {
    echo eski > "$T/inv"
    run comp_run_write_inventory "$T/inv" g web1 h u 22 /k
    [ "$status" -ne 0 ]
    [ "$(cat "$T/inv")" = "eski" ]
}

# ---------- argüman dizisi ----------
@test "argv: sıra cmd, -i, --limit, -e @dosya, ek bayraklar; paketteki -i/--limit yinelenmez; interactive işaretlenir" {
    comp_run_build_argv "$INFO" preview /tmp/inv web1 /tmp/vars.json
    [ "${COMP_RUN_ARGV[*]}" = "ansible-playbook playbooks/harden.yml --check --diff -i /tmp/inv --limit web1 -e @/tmp/vars.json --ask-become-pass" ]
    [ "$COMP_RUN_INTERACTIVE" = "1" ]
    [ "${#COMP_RUN_ARGV[@]}" -eq 11 ]
}

@test "argv: boşluklu yollar tek argüman kalır; kabuk özel karakterleri yorumlanmaz" {
    comp_run_build_argv "$INFO" preview "/tmp/my inv" 'web1' '/tmp/v ars.json'
    [ "${COMP_RUN_ARGV[5]}" = "/tmp/my inv" ]
    [ "${COMP_RUN_ARGV[9]}" = "@/tmp/v ars.json" ]
}

@test "argv: cmd içindeki glob genişletilmez" {
    mkdir -p "$T/g" && : > "$T/g/a.yml" && : > "$T/g/b.yml"
    cd "$T/g"
    local i2="$INFO"$'\n'"action.glob.cmd=ansible-playbook *.yml"$'\n'"action=glob"
    comp_run_build_argv "$i2" glob /i web1 /v
    [ "${COMP_RUN_ARGV[1]}" = "*.yml" ]
}

@test "argv: cmd çalıştırıcıya ait bayrak içeriyorsa (-i, --limit=…) reddedilir, dizi boşalır" {
    run comp_run_build_argv "$INFO" evil /i web1 /v
    [ "$status" -ne 0 ]
    comp_run_build_argv "$INFO" evil2 /i web1 /v || true
    [ "${#COMP_RUN_ARGV[@]}" -eq 0 ]
}

@test "argv: bilinmeyen eylem, script runner, bilinmeyen yer tutucu, eksik girdi reddedilir" {
    run comp_run_build_argv "$INFO" yok /i web1 /v;       [ "$status" -ne 0 ]
    run comp_run_build_argv "$INFO" preview "" web1 /v;   [ "$status" -ne 0 ]
    run comp_run_build_argv "${INFO/runner=ansible/runner=script}" preview /i web1 /v; [ "$status" -ne 0 ]
    run comp_run_build_argv "${INFO/--ask-become-pass||yes|yes/--foo|<tanimsiz>|yes|no}" preview /i web1 /v
    [ "$status" -ne 0 ]
    [[ "$output" == *"yer tutucu"* ]] || false
}

@test "argv: eylem interactive=yes ise terminal devri işaretlenir (invoke olmasa da)" {
    comp_run_build_argv "${INFO/invoke=--ask-become-pass||yes|yes/invoke=--check||no|no}" ask /i web1 /v
    [ "$COMP_RUN_INTERACTIVE" = "1" ]
}

@test "gerçek paket: bildirim + örnek değerlerle argv üretilir (kuruluysa)" {
    mf="$ROOT/components/linux-hardening/0.2.0/component.json"
    [ -f "$mf" ] || skip "linux-hardening kurulu değil"
    info=$(comp_manifest_info "$mf" linux-hardening 0.2.0)
    printf 'hardening_ssh_allow_from=10.1.10.0/24\n' > "$T/v"
    run comp_run_vars "$mf" "$T/v" "$T/real.json" --set hardening_scan_dir="$T/scans"
    [ "$status" -eq 0 ]
    comp_run_build_argv "$info" apply /i ubuntutest "$T/real.json"
    [ "${COMP_RUN_ARGV[0]}" = "ansible-playbook" ]
    [ "${COMP_RUN_ARGV[1]}" = "playbooks/harden.yml" ]
    [ "${COMP_RUN_ARGV[*]: -1}" = "--ask-become-pass" ]
}

@test "inventory: become-exe yalnızca tam /usr/bin/sudo.ws kabul edilir; boşsa envantere hiçbir şey eklenmez" {
    comp_run_write_inventory "$T/inv1" g web1 h u 22 /k
    run ! grep -q "become" "$T/inv1"
    comp_run_write_inventory "$T/inv2" g web1 h u 22 /k /usr/bin/sudo.ws
    grep -q "^web1 ansible_host=h ansible_user=u ansible_port=22 ansible_private_key_file='/k' ansible_become_exe=/usr/bin/sudo.ws$" "$T/inv2"
    local bad
    for bad in /bin/sh /usr/bin/sudo /usr/bin/sudo.ws.evil "/usr/bin/sudo.ws -S" $'/usr/bin/sudo.ws\nansible_become=yes' sudo.ws; do
        rm -f "$T/inv3"
        run comp_run_write_inventory "$T/inv3" g web1 h u 22 /k "$bad"
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
        [ ! -e "$T/inv3" ]
    done
}

# ---------- requires_sudo → sudo parolası istemi (paket invoke yazmasa da) ----------
sudo_manifest() {
    cat > "$T/ms.json" <<JSON
{"id":"util","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"util",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "durum":{"cmd":"ansible-playbook playbooks/durum.yml","effects":[],"requires_sudo":false},
  "apply":{"cmd":"ansible-playbook playbooks/apply.yml","effects":["changes-config"],"requires_sudo":true,"requires":["durum"]}},
 "vars":[{"name":"util_x","type":"string"}]$1}
JSON
    SINFO=$(comp_manifest_info "$T/ms.json" util 1.0.0)
}

@test "requires_sudo: paket invoke yazmasa da --ask-become-pass eklenir ve eylem etkileşimli olur; sudo gerektirmeyen eyleme eklenmez" {
    sudo_manifest ""
    comp_run_build_argv "$SINFO" apply /tmp/inv web1 /tmp/v.json
    [[ " ${COMP_RUN_ARGV[*]} " == *" --ask-become-pass "* ]] || { echo "${COMP_RUN_ARGV[*]}"; false; }
    [ "$COMP_RUN_INTERACTIVE" -eq 1 ]
    comp_run_build_argv "$SINFO" durum /tmp/inv web1 /tmp/v.json
    [[ " ${COMP_RUN_ARGV[*]} " != *"--ask-become-pass"* ]] || { echo "${COMP_RUN_ARGV[*]}"; false; }
    [ "$COMP_RUN_INTERACTIVE" -eq 0 ]
}

@test "requires_sudo: paket zaten --ask-become-pass yazdıysa ikilenmez" {
    sudo_manifest ',"invoke":{"append":[{"arg":"--ask-become-pass","required":true,"interactive":true}]}'
    comp_run_build_argv "$SINFO" apply /tmp/inv web1 /tmp/v.json
    [ "$(printf '%s\n' "${COMP_RUN_ARGV[@]}" | grep -c -- '--ask-become-pass')" -eq 1 ]
}
