#!/usr/bin/env bats
# Sunucu eklenirken platform seçimi ve zorunlu allow_from sorusu (PR 3): sahte paket (fixture manifest); ağ/ssh yok.
# Paket manifestinde platform_choices yoksa hiçbir şey sorulmaz (eski paketler değişmez).
load test_helper

setup() {
    load_helpers
    export SISTEM_PATH="$ROOT/sistem"
    for f in UI Servers Components ComponentsInstall ComponentsManifest ComponentsRun ComponentsVars ComponentsMenu ComponentsOnboard; do source "$H/$f.sh"; done
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_COMPONENTS_DIR="$T/comps" SSHPOCKET_SERVERS_DIR="$T/srv"
    D="$T/srv/web1"; mkdir -p "$D" "$T/comps/demo/1.0.0"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$D/server.env"
    VF="$D/demo.vars"
    comp_ids() { echo demo; }
    comp_installed_version() { [ "$1" = demo ] && printf '1.0.0'; }
    comp_net_local() { return 1; }          # ağ sorgusu yok
}

# mf <ek-üst-düzey-json (virgülle başlar)>
mf() {
    cat > "$T/comps/demo/1.0.0/component.json" <<JSON
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{"preview":{"cmd":"ansible-playbook a.yml --check","effects":[]}},
 "vars":[{"name":"allow_from","type":"list[cidr]","description":"SSH'a izin verilen ağlar"},
         {"name":"demo_platform","type":"string","default":"generic"},
         {"name":"pve_apply","type":"bool","default":false},
         {"name":"mode","type":"enum","values":["a","b"],"default":"a"}],
 "platform_var":"demo_platform"$1}
JSON
}
CHOICES=',"platform_choices":[{"id":"generic","label":"Debian / Ubuntu"},{"id":"proxmox-host","label":"Proxmox VE hostu","description":"PVE","defaults":{"mode":"b"},"apply_vars":{"pve_apply":true},"apply_warnings":["gerçek koşu açılır"]}],"onboarding_vars":["allow_from"]'

@test "platform_choices yoksa hiçbir şey sorulmaz ve yazılmaz" {
    mf ""
    run comp_onboard_all "$D" < /dev/null
    [ "$status" -eq 0 ]; [ -z "$output" ]
    [ ! -e "$VF" ]
    [ -z "$(comp_onboard_ids)" ]
}

@test "platform seçilir, defaults yazılır, allow_from editörle girilir; apply_vars YAZILMAZ" {
    mf "$CHOICES"
    [ "$(comp_onboard_ids)" = demo ]
    run comp_onboard_all "$D" <<< $'2\n\ne\n10.1.10.0/24\nk\n\n\n'
    [ "$status" -eq 0 ]
    [[ "$output" == *"Proxmox VE hostu — PVE"* && "$output" == *"demo_platform=proxmox-host"* && "$output" == *"mode=b"* ]] || { echo "$output"; false; }
    [ "$(comp_vars_get "$VF" demo_platform)" = proxmox-host ]
    [ "$(comp_vars_get "$VF" mode)" = b ]
    [ "$(comp_vars_get "$VF" allow_from)" = 10.1.10.0/24 ]
    run grep -q pve_apply "$VF"; [ "$status" -ne 0 ]
    [ "$(stat -c %a "$VF" 2>/dev/null || stat -f %Lp "$VF")" = 600 ]
}

@test "'Bilmiyorum' generic yazar; 'Şimdi değil' hiçbir platform satırı yazmaz (allow_from yine sorulur)" {
    mf "$CHOICES"
    run comp_onboard_all "$D" <<< $'b\n\n0\nh\n'
    [ "$(comp_vars_get "$VF" demo_platform)" = generic ]
    rm -f "$VF"
    run comp_onboard_all "$D" <<< $'s\n0\nh\n'
    [[ "$output" == *"Platform seçilmedi"* ]] || { echo "$output"; false; }
    [ ! -e "$VF" ] || ! grep -q demo_platform "$VF"
}

@test "onay verilmezse platform yazılmaz" {
    mf "$CHOICES"
    run comp_onboard_all "$D" <<< $'2\nh\n0\nh\n'
    [[ "$output" == *"İptal edildi"* ]] || { echo "$output"; false; }
    [ ! -e "$VF" ]
}

@test "allow_from boş bırakılırsa yazılmaz ve ⛔ uyarısı çıkar (sessiz değer yok)" {
    mf "$CHOICES"
    run comp_onboard_all "$D" <<< $'1\n\n0\nh\n'
    [[ "$output" == *"allow_from boş"* && "$output" == *"⛔"* ]] || { echo "$output"; false; }
    [ -z "$(comp_vars_get "$VF" allow_from 2>/dev/null)" ]
    [ "$(comp_vars_get "$VF" demo_platform)" = generic ]
}

@test "dosyadaki değerler ezilmez: kayıtlı platform tekrar sorulmaz, dolu allow_from atlanır, mevcut default korunur" {
    mf "$CHOICES"
    printf 'demo_platform=proxmox-host\nallow_from=192.0.2.0/24\nmode=a\n' > "$VF"
    run comp_onboard_all "$D" < /dev/null
    [[ "$output" == *"platform zaten kayıtlı"* ]] || { echo "$output"; false; }
    [ "$(comp_vars_get "$VF" mode)" = a ]
    [ "$(comp_vars_get "$VF" allow_from)" = 192.0.2.0/24 ]
    # platform satırı yok ama mode elle yazılmış: default ezilmez
    printf 'mode=a\n' > "$VF"
    run comp_onboard_all "$D" <<< $'2\n\n0\nh\n'
    [ "$(comp_vars_get "$VF" mode)" = a ]
    [ "$(comp_vars_get "$VF" demo_platform)" = proxmox-host ]
}

@test "ModuleGenerator: sunucu eklenince kurulu paketin platform sorusu çıkar, vars yazılır; paket yoksa sorulmaz" {
    mf "$CHOICES"
    printf '1.0.0\n' > "$T/comps/demo/current"
    sed 's/^\[linux-hardening\]/[demo]/' "$ROOT/components.lock" > "$T/components.lock"
    run env SSHPOCKET_SERVERS_DIR="$T/srv2" SSHPOCKET_COMPONENTS_DIR="$T/comps" SSHPOCKET_COMPONENTS_LOCK="$T/components.lock" \
        bash "$ROOT/sistem/ModuleGenerator.sh" <<< $'web9\n192.0.2.9\ndeploy\n\nn\n2\n\ne\n10.1.10.0/24\nk\n\n\n'
    [ "$status" -eq 0 ]
    [[ "$output" == *"sunucu türü ve zorunlu ayarlar"* && "$output" == *"Sunucu oluşturuldu"* ]] || { echo "$output"; false; }
    [[ "$output" != *"command not found"* && "$output" != *Traceback* ]] || { echo "$output"; false; }
    [ "$(comp_vars_get "$T/srv2/web9/demo.vars" demo_platform)" = proxmox-host ]
    [ "$(comp_vars_get "$T/srv2/web9/demo.vars" allow_from)" = 10.1.10.0/24 ]
    # paket kurulu değil: soru çıkmaz, sunucu yine oluşur
    rm -f "$T/comps/demo/current"
    run env SSHPOCKET_SERVERS_DIR="$T/srv3" SSHPOCKET_COMPONENTS_DIR="$T/comps" SSHPOCKET_COMPONENTS_LOCK="$T/components.lock" \
        bash "$ROOT/sistem/ModuleGenerator.sh" <<< $'web8\n192.0.2.8\ndeploy\n\nn\n'
    [ "$status" -eq 0 ]
    [[ "$output" != *"sunucu türü"* ]] || { echo "$output"; false; }
    [ -f "$T/srv3/web8/server.env" ] && [ ! -e "$T/srv3/web8/demo.vars" ]
}
