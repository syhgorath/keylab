#!/usr/bin/env bats
# Aşama 2.2: bildirim okuma ve sözleşme doğrulama (ağ yok)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsManifest.sh"
    T="$BATS_TEST_TMPDIR"
}

# mf <json> → $T/m.json yazar
mf() { printf '%s\n' "$1" > "$T/m.json"; }
BASE='"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"ubuntu-26.04","status":"supported"}]'
ok_act='"actions":{"audit":{"cmd":"ansible-playbook playbooks/audit.yml","effects":[]}}'

valid() { run comp_manifest_info "$T/m.json" "${1:-}" "${2:-}"; [ "$status" -eq 0 ]; }
invalid() { run comp_manifest_info "$T/m.json"; [ "$status" -ne 0 ]; }

@test "bildirim: en küçük geçerli bildirim kabul edilir" {
    mf "{$BASE,$ok_act}"
    run comp_manifest_info "$T/m.json" demo 1.0.0
    [ "$status" -eq 0 ]
    [ "$(comp_info_get "$output" runner)" = "ansible" ]
    [ "$(comp_info_get "$output" version)" = "1.0.0" ]
    [ "$(comp_info_all "$output" action)" = "audit" ]
    [ "$(comp_info_all "$output" profile)" = "ubuntu-26.04:supported" ]
}

@test "bildirim: linux-hardening biçimli (effects, requires, invoke, vars) okunur ve etkin confirm hesaplanır" {
    mf '{"id":"linux-hardening","version":"0.1.0","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
     "profiles":[{"id":"ubuntu-26.04","status":"supported"},{"id":"rpi","status":"planned"}],
     "actions":{
       "audit":{"cmd":"ansible-playbook playbooks/audit.yml","effects":["installs-package"],"requires_sudo":true,"confirm":"yes-no",
                "warnings":["audit hedefe lynis kurar"]},
       "preview":{"cmd":"ansible-playbook playbooks/harden.yml --check --diff","effects":[],"requires_sudo":true},
       "apply":{"cmd":"ansible-playbook playbooks/harden.yml","effects":["installs-package","changes-config"],
                "requires_sudo":true,"requires":["preview"],"confirm":"none"},
       "rollback_now":{"cmd":"ansible-playbook playbooks/rollback.yml --tags now","effects":["destructive"],"confirm":"yes-no"}},
     "vars":[{"name":"hardening_ssh_allow_from","type":"list[cidr]","required":true},
             {"name":"hardening_rollback_timeout","type":"int","default":300,"min":30,"max":3600},
             "hardening_scan_dir"],
     "invoke":{"append":[{"arg":"-i","value":"<inventory>","required":true},{"arg":"--ask-become-pass","interactive":true}]}}'
    run comp_manifest_info "$T/m.json" linux-hardening 0.1.0
    [ "$status" -eq 0 ]
    info="$output"
    [ "$(comp_info_get "$info" inventory_group)" = "hardening" ]
    [ "$(comp_action_get "$info" apply requires)" = "preview" ]
    [ "$(comp_action_get "$info" apply effects)" = "installs-package,changes-config" ]
    [ "$(comp_action_get "$info" apply sudo)" = "yes" ]
    # etkin confirm: paket gevşetemez
    [ "$(comp_action_get "$info" preview confirm)" = "none" ]
    [ "$(comp_action_get "$info" audit confirm)" = "yes-no" ]
    [ "$(comp_action_get "$info" apply confirm)" = "yes-no" ]
    [ "$(comp_action_get "$info" apply confirm_raised)" = "yes" ]
    [ "$(comp_action_get "$info" rollback_now confirm)" = "type-server-name" ]
    [ "$(comp_action_get "$info" rollback_now confirm_raised)" = "yes" ]
    [ "$(comp_action_get "$info" audit warnings)" = "1" ]
    comp_info_all "$info" var | grep -qx 'hardening_ssh_allow_from:list\[cidr\]:yes:1'
    comp_info_all "$info" var | grep -qx 'hardening_scan_dir:string:no:0'      # eski biçim: yalnızca ad
    comp_info_all "$info" invoke | grep -qx -- '-i|<inventory>|yes|no'
    comp_info_all "$info" invoke | grep -qx -- '--ask-become-pass||no|yes'
}

@test "bildirim: requires.control.python aralığı okunur, geçersiz aralık reddedilir" {
    mf "{$BASE,$ok_act,\"requires\":{\"control\":{\"python\":\">=3.12\"}}}"
    run comp_manifest_info "$T/m.json"
    [ "$status" -eq 0 ]
    [ "$(comp_info_get "$output" requires.python)" = ">=3.12" ]
    local r
    for r in '3.12' '>3.12' '>=3' '>=3.12;rm' '*' ''; do
        mf "{$BASE,$ok_act,\"requires\":{\"control\":{\"python\":\"$r\"}}}"
        invalid || { echo "KABUL: [$r]"; return 1; }
    done
}

@test "bildirim: paket daha sıkı confirm isteyebilir (type-server-name korunur, yükseltme sayılmaz)" {
    mf "{$BASE,\"actions\":{\"x\":{\"cmd\":\"run.sh\",\"effects\":[],\"confirm\":\"type-server-name\"}}}"
    run comp_manifest_info "$T/m.json"
    [ "$status" -eq 0 ]
    [ "$(comp_action_get "$output" x confirm)" = "type-server-name" ]
    [ "$(comp_action_get "$output" x confirm_raised)" = "no" ]
}

@test "bildirim: kimlik/sürüm beklenenden farklıysa reddedilir" {
    mf "{$BASE,$ok_act}"
    run comp_manifest_info "$T/m.json" baska 1.0.0; [ "$status" -ne 0 ]
    run comp_manifest_info "$T/m.json" demo 2.0.0;  [ "$status" -ne 0 ]
}

@test "bildirim: sshpocket_api desteklenmiyorsa (daha yeni/eski/tür) reddedilir" {
    local a
    for a in 2 0 '"1"' true null; do
        mf "{\"id\":\"demo\",\"version\":\"1.0.0\",\"sshpocket_api\":$a,\"runner\":\"ansible\",\"profiles\":[{\"id\":\"p\",\"status\":\"supported\"}],$ok_act}"
        invalid || { echo "KABUL: api=$a"; return 1; }
    done
    mf "{\"id\":\"demo\",\"version\":\"1.0.0\",\"sshpocket_api\":2,\"runner\":\"ansible\",\"profiles\":[{\"id\":\"p\",\"status\":\"supported\"}],$ok_act}"
    run comp_manifest_info "$T/m.json"
    [[ "$output" == *"daha yeni"* ]]
}

@test "bildirim: bilinmeyen zorunlu özellik (required_features) reddedilir, bilinmeyen isteğe bağlı alan yok sayılır" {
    mf "{$BASE,$ok_act,\"required_features\":[\"yeni-sey\"]}"
    invalid
    mf "{$BASE,$ok_act,\"gelecek_alan\":{\"x\":1}}"
    valid
}

@test "bildirim: geçersiz id/sürüm/runner/profil reddedilir" {
    local c
    for c in '"id":"Demo"' '"id":"../x"' '"version":"1.0"' '"version":"1.0.0-rc"' '"runner":"bash"' '"runner":5'; do
        mf "{$BASE,$ok_act,$c}"
        invalid || { echo "KABUL: $c"; return 1; }
    done
    for c in '"profiles":[]' '"profiles":"x"' '"profiles":[{"id":"p","status":"live"}]' \
             '"profiles":[{"id":"p","status":"supported"},{"id":"p","status":"planned"}]' '"profiles":[{"id":"../p","status":"supported"}]'; do
        mf "{\"id\":\"demo\",\"version\":\"1.0.0\",\"sshpocket_api\":1,\"runner\":\"ansible\",$c,$ok_act}"
        invalid || { echo "KABUL: $c"; return 1; }
    done
}

@test "bildirim: cmd'de kabuk meta karakteri / satır sonu / boş değer reddedilir" {
    local c
    for c in 'a; rm -rf /' 'a | b' 'a && b' 'a $(id)' 'a `id`' 'a > f' 'a\nb' '' '   '; do
        mf "{$BASE,\"actions\":{\"x\":{\"cmd\":\"$c\",\"effects\":[]}}}"
        invalid || { echo "KABUL: [$c]"; return 1; }
    done
}

@test "bildirim: effects/confirm/bool/requires hataları reddedilir" {
    local c
    for c in '"effects":["root-erisimi"]' '"effects":"changes-config"' '"effects":["changes-config","changes-config"]' \
             '"effects":[],"confirm":"belki"' '"effects":[],"requires_sudo":"yes"' '"effects":[],"requires":["yok"]' \
             '"effects":[],"requires":["x"]' '"effects":[],"warnings":"tek"' '"effects":[],"interactive":1'; do
        mf "{$BASE,\"actions\":{\"x\":{\"cmd\":\"run.sh\",$c}}}"
        invalid || { echo "KABUL: $c"; return 1; }
    done
    mf "{$BASE,\"actions\":{\"a\":{\"cmd\":\"r\",\"effects\":[],\"requires\":[\"b\"]},\"b\":{\"cmd\":\"r\",\"effects\":[],\"requires\":[\"a\"]}}}"
    invalid
    mf "{$BASE,\"actions\":{}}"
    invalid
    mf "{$BASE,\"actions\":{\"Bad Name\":{\"cmd\":\"r\",\"effects\":[]}}}"
    invalid
}

@test "bildirim: değişken hataları reddedilir (tür, enum, aralık, yineleme)" {
    local c
    for c in '{"name":"x","type":"dosya"}' '{"name":"X","type":"int"}' '{"name":"x","type":"enum"}' \
             '{"name":"x","type":"int","min":10,"max":5}' '{"name":"x","type":"int","default":"abc"}' \
             '{"name":"x","type":"int","min":1,"max":3,"default":9}' '{"name":"x","type":"bool","default":"yes"}' \
             '{"name":"x","type":"list[enum]"}' '{"name":"x","required":"evet"}' '5'; do
        mf "{$BASE,$ok_act,\"vars\":[$c]}"
        invalid || { echo "KABUL: $c"; return 1; }
    done
    mf "{$BASE,$ok_act,\"vars\":[{\"name\":\"x\",\"type\":\"int\"},{\"name\":\"x\",\"type\":\"int\"}]}"
    invalid
}

@test "bildirim: invoke.append hataları reddedilir" {
    local c
    for c in '{"arg":"ls"}' '{"arg":"-i x"}' '{"arg":"-i","value":"a b; c"}' '{"arg":"-i","value":"$(id)"}' '"-i"'; do
        mf "{$BASE,$ok_act,\"invoke\":{\"append\":[$c]}}"
        invalid || { echo "KABUL: $c"; return 1; }
    done
}

@test "bildirim: geçersiz JSON, nesne olmayan, çok büyük ve yok olan dosya reddedilir" {
    printf 'not json' > "$T/m.json"; invalid
    printf '[1,2]' > "$T/m.json"; invalid
    # yalnızca boyut yüzünden reddedilmeli: geçerli bildirim + yok sayılan büyük alan
    python3 -I -c "
import json
d={'id':'demo','version':'1.0.0','sshpocket_api':1,'runner':'ansible',
   'profiles':[{'id':'p','status':'supported'}],
   'actions':{'a':{'cmd':'run.sh','effects':[]}},'gelecek_alan':'x'*300000}
print(json.dumps(d))" > "$T/m.json"
    invalid
    [[ "$output" == *"çok büyük"* ]]
    run comp_manifest_info "$T/yok.json"
    [ "$status" -ne 0 ]
}

@test "hazırlık: kurulu paketi bütünlükle doğrular ve bilgiyi verir; kurcalanmışsa reddeder" {
    export SSHPOCKET_COMPONENTS_LOCK="$T/components.lock" SSHPOCKET_COMPONENTS_DIR="$T/components"
    ssh-keygen -q -t ed25519 -N "" -f "$T/good" -C test
    PUB=$(cut -d' ' -f1,2 "$T/good.pub")
    mkdir -p "$T/pkg/demo-1.0.0"
    printf '{%s,%s}\n' "$BASE" "$ok_act" > "$T/pkg/demo-1.0.0/component.json"
    tar -czf "$T/demo-1.0.0.tar.gz" -C "$T/pkg" demo-1.0.0
    ssh-keygen -q -Y sign -f "$T/good" -n sshpocket-component "$T/demo-1.0.0.tar.gz"
    printf '[demo]\nrepo = a/demo\nversion = 1.0.0\nsha256 = %s\nsigners = sshpocket-component namespaces="sshpocket-component" %s\n' \
        "$(comp_sha256_of "$T/demo-1.0.0.tar.gz")" "$PUB" > "$T/components.lock"
    run comp_prepare demo                                    # kurulu değil
    [ "$status" -ne 0 ]
    comp_install_archive demo 1.0.0 "$T/demo-1.0.0.tar.gz" "$T/demo-1.0.0.tar.gz.sig" >/dev/null
    run comp_prepare demo
    [ "$status" -eq 0 ]
    [ "$(comp_info_get "$output" id)" = "demo" ]
    printf '{"id":"demo"}' > "$T/components/demo/1.0.0/component.json"   # kurulumdan sonra kurcalama
    run comp_prepare demo
    [ "$status" -ne 0 ]
}

@test "değişken açıklaması en çok 2000 karakter (Proxmox web_exclusive gibi uzun açıklamalar); aşan ve kontrol karakterli reddedilir; uyarı metni sınırı 500 kalır" {
    local d500 d1000 d1001 d2000 d2001
    d500=$(python3 -c "print('a'*500)"); d1000=$(python3 -c "print('a'*1000)"); d1001=$(python3 -c "print('a'*1001)")
    d2000=$(python3 -c "print('a'*2000)"); d2001=$(python3 -c "print('a'*2001)")
    mf "{$BASE,$ok_act,\"vars\":[{\"name\":\"x\",\"type\":\"string\",\"description\":\"$d1000\"}]}"
    valid
    mf "{$BASE,$ok_act,\"vars\":[{\"name\":\"x\",\"type\":\"string\",\"description\":\"$d2000\"}]}"
    valid
    mf "{$BASE,$ok_act,\"vars\":[{\"name\":\"x\",\"type\":\"string\",\"description\":\"$d2001\"}]}"
    invalid
    [[ "$output" == *"description geçersiz metin"* ]] || false
    mf "{$BASE,$ok_act,\"vars\":[{\"name\":\"x\",\"type\":\"string\",\"description\":\"a\\u0007b\"}]}"
    invalid
    # eylem uyarı metni 500'ü aşamaz (sınır değişmedi)
    mf "{$BASE,\"actions\":{\"audit\":{\"cmd\":\"ansible-playbook playbooks/audit.yml\",\"effects\":[],\"warnings\":[\"$d500\"]}}}"
    valid
    mf "{$BASE,\"actions\":{\"audit\":{\"cmd\":\"ansible-playbook playbooks/audit.yml\",\"effects\":[],\"warnings\":[\"${d500}a\"]}}}"
    invalid
}


# set: eylem düzeyi sabit değişkenler
setj() {   # <eylem-set-json>  → bool, int(30-3600), enum, string, path, list değişkenleri olan bildirim
    mf "{$BASE,\"actions\":{\"run\":{\"cmd\":\"ansible-playbook a.yml\",\"effects\":[],\"set\":$1,\"outputs\":{\"dir_var\":\"d\"}}},
 \"vars\":[{\"name\":\"b\",\"type\":\"bool\"},{\"name\":\"n\",\"type\":\"int\",\"min\":30,\"max\":3600},{\"name\":\"e\",\"type\":\"enum\",\"values\":[\"x\",\"y\"]},
  {\"name\":\"s\",\"type\":\"string\"},{\"name\":\"d\",\"type\":\"path\"},{\"name\":\"l\",\"type\":\"list[string]\"}]}"
}

@test "set: geçerli bool/int/enum/string değerleri kabul edilir ve action.<ad>.set satırları üretilir" {
    setj '{"b":true,"n":60,"e":"y","s":"merhaba"}'
    run comp_manifest_info "$T/m.json"
    [ "$status" -eq 0 ]
    [ "$(comp_info_all "$output" action.run.set | sort | tr '\n' ' ')" = "b=true e=y n=60 s=merhaba " ]
    setj '{"b":false}'
    run comp_manifest_info "$T/m.json"; [ "$status" -eq 0 ]
    [ "$(comp_info_get "$output" action.run.set)" = "b=false" ]
}

@test "set: bilinmeyen değişken, yanlış tür, aralık dışı, geçersiz enum, liste, dir_var ve boş/çok büyük set reddedilir" {
    local bad
    for bad in '{"yok":true}' '{"b":"true"}' '{"b":1}' '{"n":5}' '{"n":99999}' '{"n":true}' '{"e":"z"}' '{"s":""}' '{"l":["a"]}' '{"d":"/tmp/x"}' '{}' '[]' '"x"' \
        '{"b":true,"n":60,"e":"y","s":"a","d_":1,"x1":1,"x2":1,"x3":1,"x4":1}'; do
        setj "$bad"
        run comp_manifest_info "$T/m.json"
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
    done
}

# ---------- platform_var / platforms / group ----------
@test "manifest: geçerli platform_var, platforms ve group kabul edilir" {
    cat > "$T/pf.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","platform_var":"plat",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{"preview":{"cmd":"ansible-playbook a.yml --check","effects":[]}},
 "vars":[{"name":"plat","type":"string","default":"generic"},{"name":"x","type":"bool","platforms":["proxmox-host","raspberry-pi"],"group":"Proxmox"}]}
JSON
    run comp_manifest_info "$T/pf.json" demo 1.0.0; [ "$status" -eq 0 ]
}

@test "manifest: geçersiz platform_var, platforms ve group reddedilir" {
    base='{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"ubuntu-26.04","status":"supported"}],"actions":{"preview":{"cmd":"ansible-playbook a.yml --check","effects":[]}}'
    printf '%s,"platform_var":"yok","vars":[{"name":"plat","type":"string"}]}' "$base" > "$T/a.json"
    run comp_manifest_info "$T/a.json" demo 1.0.0; [ "$status" -ne 0 ]; [[ "$output" == *"platform_var"* ]] || false
    printf '%s,"vars":[{"name":"x","type":"bool","platforms":"proxmox-host"}]}' "$base" > "$T/b.json"
    run comp_manifest_info "$T/b.json" demo 1.0.0; [ "$status" -ne 0 ]; [[ "$output" == *"platforms"* ]] || false
    printf '%s,"vars":[{"name":"x","type":"bool","platforms":["Proxmox Host"]}]}' "$base" > "$T/c.json"
    run comp_manifest_info "$T/c.json" demo 1.0.0; [ "$status" -ne 0 ]
    printf '%s,"vars":[{"name":"x","type":"bool","group":""}]}' "$base" > "$T/d.json"
    run comp_manifest_info "$T/d.json" demo 1.0.0; [ "$status" -ne 0 ]; [[ "$output" == *"group"* ]] || false
}

# ---------- eylem platforms ----------
@test "manifest: eylem platforms kabul edilir ve action.<ad>.platforms olarak yazılır; yoksa satır çıkmaz" {
    cat > "$T/ap.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{"preview":{"cmd":"ansible-playbook a.yml --check","effects":[]},
            "pve_preview":{"cmd":"ansible-playbook p.yml --check","effects":[],"platforms":["proxmox-host"]},
            "pve_apply":{"cmd":"ansible-playbook p.yml","effects":["changes-config"],"requires":["pve_preview"],"platforms":["proxmox-host"]}}}
JSON
    run comp_manifest_info "$T/ap.json" demo 1.0.0; [ "$status" -eq 0 ]
    [[ "$output" == *"action.pve_preview.platforms=proxmox-host"* ]] || false
    [[ "$output" != *"action.preview.platforms"* ]] || false
}

@test "manifest: geçersiz eylem platforms reddedilir" {
    base='{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"ubuntu-26.04","status":"supported"}]'
    for bad in '"proxmox-host"' '[]' '["Proxmox Host"]' '["a","a"]' '[1]'; do
        printf '%s,"actions":{"preview":{"cmd":"ansible-playbook a.yml --check","effects":[],"platforms":%s}}}' "$base" "$bad" > "$T/b.json"
        run comp_manifest_info "$T/b.json" demo 1.0.0
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
        [[ "$output" == *"platforms"* ]] || { echo "$bad: $output"; return 1; }
    done
}

@test "manifest: platformlu eylemin ön koşulu, onun platformlarının hepsinde görünmüyorsa reddedilir" {
    base='{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"ubuntu-26.04","status":"supported"}]'
    printf '%s,"actions":{"a":{"cmd":"x a","effects":[],"platforms":["p1"]},"b":{"cmd":"x b","effects":["changes-config"],"requires":["a"]}}}' "$base" > "$T/r1.json"
    run comp_manifest_info "$T/r1.json" demo 1.0.0; [ "$status" -ne 0 ]; [[ "$output" == *"requires a"* ]] || false
    printf '%s,"actions":{"a":{"cmd":"x a","effects":[],"platforms":["p1"]},"b":{"cmd":"x b","effects":["changes-config"],"requires":["a"],"platforms":["p1","p2"]}}}' "$base" > "$T/r2.json"
    run comp_manifest_info "$T/r2.json" demo 1.0.0; [ "$status" -ne 0 ]
    printf '%s,"actions":{"a":{"cmd":"x a","effects":[],"platforms":["p1","p2"]},"b":{"cmd":"x b","effects":["changes-config"],"requires":["a"],"platforms":["p1"]}}}' "$base" > "$T/r3.json"
    run comp_manifest_info "$T/r3.json" demo 1.0.0; [ "$status" -eq 0 ]
    printf '%s,"actions":{"a":{"cmd":"x a","effects":[]},"b":{"cmd":"x b","effects":["changes-config"],"requires":["a"],"platforms":["p1"]}}}' "$base" > "$T/r4.json"
    run comp_manifest_info "$T/r4.json" demo 1.0.0; [ "$status" -eq 0 ]
}

# ---------- runner: script ----------
SCRIPT_BASE='{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"script","profiles":[{"id":"ubuntu-26.04","status":"supported"}]'

@test "manifest (script): geçerli 'scripts/<ad>.sh [argümanlar]' kabul edilir, cmd olduğu gibi yazılır" {
    printf '%s,"actions":{"saglik":{"cmd":"scripts/saglik.sh --dry-run x=1","effects":[]}}}' "$SCRIPT_BASE" > "$T/s1.json"
    run comp_manifest_info "$T/s1.json" demo 1.0.0; [ "$status" -eq 0 ]
    [[ "$output" == *"runner=script"* && "$output" == *"action.saglik.cmd=scripts/saglik.sh --dry-run x=1"* ]] || false
}

@test "manifest (script): ansible-playbook, kök betik, '..', mutlak yol, başka uzantı ve kötü argümanlar reddedilir" {
    for bad in 'ansible-playbook playbooks/a.yml' 'saglik.sh' 'scripts/../etc/x.sh' '/tmp/x.sh' 'scripts/a.py' 'scripts/a.sh --x;y' 'scripts/a.sh $(id)' 'scripts/a.sh a b c d e f g h i'; do
        printf '%s,"actions":{"a":{"cmd":"%s","effects":[]}}}' "$SCRIPT_BASE" "$bad" > "$T/s2.json"
        run comp_manifest_info "$T/s2.json" demo 1.0.0
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
    done
}

@test "manifest (script): invoke, inventory_group ve outputs.dir_var desteklenmez; ansible'da hâlâ serbest" {
    printf '%s,"invoke":{"append":[{"arg":"--x"}]},"actions":{"a":{"cmd":"scripts/a.sh","effects":[]}}}' "$SCRIPT_BASE" > "$T/s3.json"
    run comp_manifest_info "$T/s3.json" demo 1.0.0; [ "$status" -ne 0 ]; [[ "$output" == *"yalnızca ansible"* ]] || false
    printf '%s,"inventory_group":"x","actions":{"a":{"cmd":"scripts/a.sh","effects":[]}}}' "$SCRIPT_BASE" > "$T/s4.json"
    run comp_manifest_info "$T/s4.json" demo 1.0.0; [ "$status" -ne 0 ]
    printf '%s,"actions":{"a":{"cmd":"scripts/a.sh","effects":[],"outputs":{"dir_var":"d"}}},"vars":[{"name":"d","type":"path"}]}' "$SCRIPT_BASE" > "$T/s5.json"
    run comp_manifest_info "$T/s5.json" demo 1.0.0; [ "$status" -ne 0 ]; [[ "$output" == *"dir_var script runner"* ]] || false
}

# ---------- menu: kategori / başlık / sıra (S1) ----------
@test "menu: kategori, başlık ve sıra okunur; yoksa satır üretilmez" {
    mf "{$BASE,$ok_act,\"menu\":{\"category\":\"kurulum\",\"title\":\"Kurulum merkezi\",\"order\":20}}"
    valid
    [ "$(comp_info_get "$output" menu.category)" = "kurulum" ]
    [ "$(comp_info_get "$output" menu.title)" = "Kurulum merkezi" ]
    [ "$(comp_info_get "$output" menu.order)" = "20" ]
    mf "{$BASE,$ok_act}"
    valid
    [ -z "$(comp_info_get "$output" menu.category)" ]
}

@test "menu: sabit liste dışı kategori, bilinmeyen alan, kötü sıra ve nesne olmayan değer reddedilir" {
    mf "{$BASE,$ok_act,\"menu\":{\"category\":\"oyunlar\"}}"; invalid
    mf "{$BASE,$ok_act,\"menu\":{\"category\":\"kurulum\",\"renk\":1}}"; invalid
    mf "{$BASE,$ok_act,\"menu\":{\"order\":1000}}"; invalid
    mf "{$BASE,$ok_act,\"menu\":{\"order\":true}}"; invalid
    mf "{$BASE,$ok_act,\"menu\":\"kurulum\"}"; invalid
}

# ---------- items[]: yazılım kataloğu (S2) ----------
ITEMVARS='"vars":[{"name":"yazilim","type":"enum","values":["nginx","redis"]},{"name":"mod","type":"enum","values":["native","docker"],"default":"native"},{"name":"port","type":"int"}]'
ITEMACTS='"actions":{"durum":{"cmd":"ansible-playbook d.yml","effects":[]},"preview":{"cmd":"ansible-playbook p.yml --check","effects":[]},"kur":{"cmd":"ansible-playbook k.yml","effects":["installs-package"],"requires":["preview"]}}'
items_mf() { mf "{$BASE,$ITEMACTS,$ITEMVARS,\"item_var\":\"yazilim\",\"mode_var\":\"mod\",\"items\":[$1]}"; }

@test "items: katalog okunur (etiket, açıklama, mod, eylem, parametre)" {
    items_mf '{"id":"nginx","label":"Nginx","description":"Web sunucusu","modes":["native","docker"],"actions":["durum","preview","kur"],"params":["port"]},{"id":"redis","label":"Redis"}'
    valid
    [ "$(comp_info_get "$output" item_var)" = "yazilim" ]
    [ "$(comp_info_all "$output" item | paste -sd, -)" = "nginx|Nginx,redis|Redis" ]
    [ "$(comp_info_get "$output" item.nginx.modes)" = "native,docker" ]
    [ "$(comp_info_get "$output" item.nginx.actions)" = "durum,preview,kur" ]
    [ "$(comp_info_get "$output" item.nginx.params)" = "port" ]
    [ "$(comp_info_get "$output" item.nginx.description)" = "Web sunucusu" ]
}

@test "items: geçersiz katalog reddedilir" {
    items_mf '{"id":"nginx","label":"N"},{"id":"nginx","label":"N2"}'; invalid                       # yinelenen kimlik
    items_mf '{"id":"mysql","label":"M"}'; invalid                                                   # item_var enum'unda yok
    items_mf '{"id":"nginx","label":"N","modes":["vm"]}'; invalid                                    # mode_var enum'unda yok
    items_mf '{"id":"nginx","label":"N","actions":["yok"]}'; invalid                                 # tanımsız eylem
    items_mf '{"id":"nginx","label":"N","actions":["kur"]}'; invalid                                 # ön koşul (preview) listede yok
    items_mf '{"id":"nginx","label":"N","params":["yokdeg"]}'; invalid                               # tanımsız değişken
    items_mf '{"id":"nginx","label":"N","renk":1}'; invalid                                          # bilinmeyen alan
    mf "{$BASE,$ITEMACTS,$ITEMVARS,\"items\":[{\"id\":\"nginx\",\"label\":\"N\"}]}"; invalid        # item_var yok
    mf "{$BASE,$ITEMACTS,$ITEMVARS,\"item_var\":\"yazilim\"}"; invalid                              # items yok
    mf "{$BASE,$ITEMACTS,$ITEMVARS,\"item_var\":\"yazilim\",\"items\":[]}"; invalid                 # boş katalog
}
