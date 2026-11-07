#!/usr/bin/env bats
# Aşama 3, PR C: yürütücü (sahte ansible-playbook; ağ yok, VM'e dokunmaz)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsManifest.sh"
    source "$H/ComponentsRun.sh"
    source "$H/ComponentsGates.sh"
    source "$H/ComponentsExec.sh"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_STATE_DIR="$T/state" TMPDIR="$T/tmp"
    mkdir -p "$T/tmp" "$T/pkg" "$T/bin"
    # --- paket ---
    cat > "$T/pkg/component.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/harden.yml --check","effects":[],"requires_sudo":true},
  "audit":{"cmd":"ansible-playbook playbooks/audit.yml","effects":["installs-package"],"confirm":"yes-no",
           "outputs":{"dir_var":"scan_dir"}},
  "status":{"cmd":"ansible-playbook playbooks/status.yml","effects":[],
            "outputs":{"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.status","keys":["armed","remaining_seconds","timer"]}},
  "apply":{"cmd":"ansible-playbook playbooks/harden.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no"}},
 "vars":[{"name":"allow_from","type":"list[cidr]","required":true},
         {"name":"scan_dir","type":"path"},
         {"name":"rb_enabled","type":"bool","default":true,"warn_if":false}],
 "invoke":{"append":[{"arg":"-i","value":"<inventory>","required":true},{"arg":"--limit","value":"<server>","required":true},
                     {"arg":"--ask-become-pass","required":true,"interactive":true}]}}
JSON
    echo "[defaults]" > "$T/pkg/ansible.cfg"
    INFO=$(comp_manifest_info "$T/pkg/component.json" demo 1.0.0)
    printf 'allow_from=10.1.10.0/24\n' > "$T/values"
    # --- sahte ansible-playbook ---
    cat > "$T/bin/ansible-playbook" <<STUB
#!/bin/bash
echo "\$*" > "$T/args"
echo "\$PWD" > "$T/pwd"
echo "\$ANSIBLE_CONFIG|\$ANSIBLE_HOST_KEY_CHECKING|\${ANSIBLE_TIMEOUT:-yok}" > "$T/envseen"
echo "\${PYTHONDONTWRITEBYTECODE:-yok}" > "$T/bytecode"
# -i ve -e @ dosyalarının İÇERİĞİNİ çalışma sırasında kaydet
while [ \$# -gt 0 ]; do case "\$1" in -i) cp "\$2" "$T/seen_inv"; stat -c %a "\$2" >/dev/null 2>&1 && stat -c %a "\$2" > "$T/inv_mode" || stat -f %Lp "\$2" > "$T/inv_mode"; echo "\$2" > "$T/inv_path";; -e) f="\${2#@}"; cp "\$f" "$T/seen_vars"; echo "\$f" > "$T/vars_path";; esac; shift; done
[ -n "\${FAKE_OUT:-}" ] && printf '%s\n' "\$FAKE_OUT"
[ -n "\${FAKE_KILL_PARENT:-}" ] && { touch "$T/started"; kill -\${FAKE_SIG:-TERM} \$(ps -o ppid= -p \$PPID | tr -d ' '); exec >/dev/null 2>&1 3>&- 4>&-; sleep 3; }
exit \${FAKE_RC:-0}
STUB
    chmod +x "$T/bin/ansible-playbook"
    # --- bağlam ---
    COMP_CTX_PKGDIR="$T/pkg"; COMP_CTX_BINDIR="$T/bin"; COMP_CTX_SERVER=web1; COMP_CTX_HOST=10.1.10.193
    COMP_CTX_USER=ziya; COMP_CTX_PORT=22; COMP_CTX_KEY=/Volumes/test/k; COMP_CTX_PREFIX=WEB1
    COMP_CTX_PROFILE=ubuntu-26.04; COMP_CTX_VALUES="$T/values"
    # kapıların kendi testleri components_gates.bats'ta: burada geçecek şekilde sahtelenir
    comp_gate_known_host() { return 0; }
    comp_gate_agent() { return 0; }
}

modeof() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }
tmpleft() { ls -d "$T"/tmp/sshpocket-run.* 2>/dev/null | wc -l | tr -d ' '; }

@test "yürütme: salt okunur eylem çalışır; argv, çalışma dizini, ortam, geçici dosya içeriği ve izinleri doğru" {
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    read -r a < "$T/args"
    [[ "$a" == "playbooks/harden.yml --check -i "*" --limit web1 -e @"*" --ask-become-pass" ]] || false
    [ "$(cat "$T/pwd")" = "$(cd "$T/pkg" && pwd -P)" ] || [ "$(cat "$T/pwd")" = "$T/pkg" ]
    [ "$(cat "$T/envseen")" = "$T/pkg/ansible.cfg|True|30" ]
    grep -q "^\[hardening\]" "$T/seen_inv"
    grep -q "web1 ansible_host=10.1.10.193 ansible_user=ziya ansible_port=22 ansible_private_key_file='/Volumes/test/k'" "$T/seen_inv"
    [ "$(cat "$T/inv_mode")" = "600" ]
    grep -q '"allow_from"' "$T/seen_vars"
    [ "$(tmpleft)" = "0" ]                                  # geçici dosyalar silindi
    [ ! -e "$(cat "$T/inv_path")" ]
    [ ! -e "$(cat "$T/vars_path")" ]
    [ "$(modeof "$T/state/demo/web1/last-run.log")" = "600" ]
    grep -q '|1.0.0|preview|ok$' "$T/state/demo/web1/results"
}

@test "yürütme: --ask-become-pass varsa 'BECOME password:' öncesi sudo açıklaması yazılır; yoksa yazılmaz" {
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"Sudo parolası istenecek"* ]] || false
    [[ "$output" == *"BECOME password:"* ]] || false
    [[ "$output" == *"NOPASSWD"* ]] || false
    local noask="${INFO/invoke=--ask-become-pass||yes|yes/}"
    [ "$noask" != "$INFO" ]
    # invoke yok ama eylem requires_sudo ise sshpocket --ask-become-pass'i kendisi ekler: açıklama yine yazılır
    run comp_exec_action "$noask" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"Sudo parolası istenecek"* ]] || false
    # ne invoke ne requires_sudo: parola istemi ve açıklama yok
    local nosudo="${noask/action.preview.sudo=yes/action.preview.sudo=no}"
    [ "$nosudo" != "$noask" ]
    run comp_exec_action "$nosudo" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" != *"Sudo parolası"* ]] || false
}

@test "yürütme: ortamdan ANSIBLE_HOST_KEY_CHECKING=False gelse bile True zorlanır" {
    ANSIBLE_HOST_KEY_CHECKING=False run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [ "$(cat "$T/envseen")" = "$T/pkg/ansible.cfg|True|30" ]
}

@test "yürütme: Python bayt kodu kurulu paket klasörüne yazılmaz (PYTHONDONTWRITEBYTECODE=1), bütünlük denetimi bozulmaz" {
    PYTHONDONTWRITEBYTECODE= run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [ "$(cat "$T/bytecode")" = "1" ]
}

@test "yürütme: playbook başarısızsa kodu döner, 'fail' kaydedilir, geçici dosyalar silinir" {
    FAKE_RC=2 run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 2 ]
    grep -q '|1.0.0|preview|fail$' "$T/state/demo/web1/results"
    [ "$(tmpleft)" = "0" ]
}

@test "yürütme: kapı durdurursa playbook HİÇ çalışmaz, kayıt yok, 190 döner" {
    comp_gate_known_host() { echo "tanınmıyor" >&2; return 1; }
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 190 ]
    [ ! -e "$T/args" ]
    [ ! -e "$T/state/demo/web1/results" ]
    [ "$(tmpleft)" = "0" ]
}

@test "yürütme: her kapı ayrı ayrı durdurur (agent, root, profil, requires)" {
    comp_gate_agent() { return 1; };            run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null; [ "$status" -eq 190 ]
    comp_gate_agent() { return 0; }
    COMP_CTX_USER=root;                         run comp_exec_action "$INFO" demo 1.0.0 apply </dev/null;   [ "$status" -eq 190 ]
    COMP_CTX_USER=ziya; COMP_CTX_PROFILE=rpi;   run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null; [ "$status" -eq 190 ]
    COMP_CTX_PROFILE=ubuntu-26.04;              run comp_exec_action "$INFO" demo 1.0.0 apply <<<"e";       [ "$status" -eq 190 ]
    [[ "$output" == *"önce 'preview'"* ]] || false
    [ ! -e "$T/args" ]
}

@test "yürütme: geçersiz/eksik değişken çalıştırmaz" {
    : > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 190 ]
    [ ! -e "$T/args" ]
}

@test "yürütme: confirm reddedilirse çalıştırmaz; kabul edilirse çalıştırır" {
    run comp_exec_action "$INFO" demo 1.0.0 audit <<<"h"; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
    run comp_exec_action "$INFO" demo 1.0.0 audit </dev/null; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
    run comp_exec_action "$INFO" demo 1.0.0 audit <<<"e"; [ "$status" -eq 0 ]; [ -e "$T/args" ]
}

@test "yürütme: warn_if eşleşirse DEĞİŞTİREN eylemde (apply) ayrıca onay istenir" {
    printf 'allow_from=10.1.10.0/24\nrb_enabled=false\n' > "$T/values"
    comp_gate_record demo web1 1.0.0 preview ok
    run comp_exec_action "$INFO" demo 1.0.0 apply <<<"h"; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
    run comp_exec_action "$INFO" demo 1.0.0 apply <<<$'e\ne'; [ "$status" -eq 0 ]; [ -e "$T/args" ]
}

@test "yürütme: warn_if (riskli değer) sorusu salt okunur eylemlerde (preview, durum) ve yalnızca araç kuran eylemde (audit) SORULMAZ" {
    printf 'allow_from=10.1.10.0/24\nrb_enabled=false\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 0 ]; [ -e "$T/args" ]
    rm -f "$T/args"
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null; [ "$status" -eq 0 ]; [ -e "$T/args" ]
    [[ "$output" != *"riskli"* ]] || false
    rm -f "$T/args"
    # audit yalnızca kendi 'yes-no' onayını sorar: tek 'e' yeter (riskli değer sorusu olsaydı bu 'e' onu tüketir, onay EOF olurdu)
    run comp_exec_action "$INFO" demo 1.0.0 audit <<<"e"; [ "$status" -eq 0 ]; [ -e "$T/args" ]
}

@test "ortam: ANSIBLE_TIMEOUT varsayılan 30 sn (become istemi için); kullanıcının kendi değeri korunur" {
    run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 0 ]
    [[ "$(cat "$T/envseen")" == *"|True|30" ]] || false
    ANSIBLE_TIMEOUT=45 run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 0 ]
    [[ "$(cat "$T/envseen")" == *"|True|45" ]] || false
}

@test "zorunlu değişken: salt okunur durum eylemi (status) değişken dosyası OLMADAN çalışır; preview/audit/apply zorunlu değişkeni yine ister" {
    rm -f "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 0 ]; [ -e "$T/args" ]
    rm -f "$T/args"
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
    [[ "$output" == *"allow_from: zorunlu değişken eksik"* ]] || false
    run comp_exec_action "$INFO" demo 1.0.0 audit <<<"e"; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
    [[ "$output" == *"zorunlu değişken eksik"* ]] || false
    comp_gate_record demo web1 1.0.0 preview ok
    run comp_exec_action "$INFO" demo 1.0.0 apply <<<"e"; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
}

@test "zorunlu değişken: salt okunur eylemde bile VERİLMİŞ geçersiz değer reddedilir (yalnızca 'eksik' hoşgörülür)" {
    printf 'allow_from=bu-bir-cidr-degil\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
    printf 'bilinmeyen_degisken=1\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 190 ]; [ ! -e "$T/args" ]
}

@test "set: eylem düzeyi sabit değişkenler kullanıcının değerinin ÜSTÜNE yazılır; set'siz eylem kullanıcı değerini korur" {
    python3 - "$T/pkg/component.json" <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p))
d["vars"].append({"name":"rb_timeout","type":"int","default":300,"min":30,"max":3600})
d["actions"]["fast"]={"cmd":"ansible-playbook playbooks/f.yml","effects":[],"set":{"rb_timeout":60,"rb_enabled":True}}
json.dump(d,open(p,"w"),ensure_ascii=False)
PY
    INFO=$(comp_manifest_info "$T/pkg/component.json" demo 1.0.0)
    printf 'allow_from=10.1.10.0/24\nrb_timeout=900\nrb_enabled=false\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 fast <<<"e"; [ "$status" -eq 0 ]
    python3 -c "import json;d=json.load(open('$T/seen_vars'));assert d['rb_timeout']==60 and d['rb_enabled'] is True, d"
    rm -f "$T/seen_vars" "$T/args"
    run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 0 ]
    python3 -c "import json;d=json.load(open('$T/seen_vars'));assert d['rb_timeout']==900 and d['rb_enabled'] is False, d"
}

@test "yürütme: apply, taze preview kaydından sonra çalışır" {
    comp_gate_record demo web1 1.0.0 preview ok
    run comp_exec_action "$INFO" demo 1.0.0 apply <<<"e"
    [ "$status" -eq 0 ]
    grep -q '|1.0.0|apply|ok$' "$T/state/demo/web1/results"
}

@test "yürütme: dir_var kullanıcı klasörüne ZORLANIR (değerler dosyasındaki USB yolu ezilir), klasör 0700" {
    printf 'allow_from=10.1.10.0/24\nscan_dir=/Volumes/test/oops\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 audit <<<"e"
    [ "$status" -eq 0 ]
    grep -q "\"scan_dir\": \"$T/state/demo/web1/out\"" "$T/seen_vars"
    run ! grep -q "/Volumes/test/oops" "$T/seen_vars"
    [ "$(modeof "$T/state/demo/web1/out")" = "700" ]
}

@test "yürütme: bin dizininde program yoksa ya da cmd yol içeriyorsa çalıştırmaz" {
    rm "$T/bin/ansible-playbook"
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 190 ]
    [ "$(tmpleft)" = "0" ]
}

@test "yürütme: TERM ve HUP (terminal kapanması) gelirse geçici dosyalar yine de silinir" {
    local sig
    for sig in TERM HUP; do
        rm -f "$T/started" "$T/inv_path"
        FAKE_SIG=$sig FAKE_KILL_PARENT=1 run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
        [ -e "$T/started" ] || { echo "BAŞLAMADI: $sig"; return 1; }
        [ "$status" -ne 0 ] || { echo "KOD 0: $sig"; return 1; }
        sleep 1
        [ "$(tmpleft)" = "0" ] || { echo "GEÇİCİ DOSYA KALDI: $sig"; return 1; }
        [ ! -e "$(cat "$T/inv_path")" ] || { echo "INVENTORY KALDI: $sig"; return 1; }
    done
}

# ---------- sonuç satırı ----------
res() { printf '%s\n' "$1" > "$T/log"; comp_exec_parse_result "$INFO" status "$T/log"; }

@test "sonuç: düz, yaml-girintili ve json-tırnaklı satırlar ayrıştırılır" {
    res 'SSHPOCKET_RESULT demo.status armed=yes timer=active remaining_seconds=120'
    [ "$COMP_EXEC_RESULT" = $'armed=yes\ntimer=active\nremaining_seconds=120' ]
    res '        msg: |-
          SSHPOCKET_RESULT demo.status armed=no timer=inactive'
    [ "$COMP_EXEC_RESULT" = $'armed=no\ntimer=inactive' ]
    res '"msg": "SSHPOCKET_RESULT demo.status armed=yes\n"'
    [ "$COMP_EXEC_RESULT" = "armed=yes" ]
}

@test "sonuç: son satır geçerli sayılır; yanlış önek, bilinmeyen anahtar, kötü değer, yinelenen anahtar, satır yok → ret" {
    printf 'SSHPOCKET_RESULT demo.status armed=no\nSSHPOCKET_RESULT demo.status armed=yes\n' > "$T/log"
    comp_exec_parse_result "$INFO" status "$T/log"; [ "$COMP_EXEC_RESULT" = "armed=yes" ]
    for bad in 'SSHPOCKET_RESULT other.status armed=yes' 'SSHPOCKET_RESULT demo.apply armed=yes' 'SSHPOCKET_RESULT demo.status evil=1' \
               'SSHPOCKET_RESULT demo.status armed=a;b' 'SSHPOCKET_RESULT demo.status armed=yes armed=no' 'SSHPOCKET_RESULT demo.status armed' 'merhaba'; do
        run res "$bad"
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
    done
}

@test "sonuç: outputs.format olmayan eylem için ayrıştırma sessizce atlanır" {
    printf 'SSHPOCKET_RESULT demo.preview x=1\n' > "$T/log"
    run comp_exec_parse_result "$INFO" preview "$T/log"
    [ "$status" -eq 0 ]
}

@test "yürütme + sonuç: status çalışınca COMP_EXEC_RESULT dolar" {
    FAKE_OUT='SSHPOCKET_RESULT demo.status armed=yes timer=active remaining_seconds=42' comp_exec_action "$INFO" demo 1.0.0 status </dev/null
    [ "$COMP_EXEC_RESULT" = $'armed=yes\ntimer=active\nremaining_seconds=42' ]
    # son sonuç kullanıcı klasörüne saklanır (menüde "son bilinen durum" olarak gösterilir)
    run comp_exec_last_result demo web1 status
    [ "$status" -eq 0 ]
    [ "${lines[1]}" = "armed=yes" ]
    [ "${lines[3]}" = "remaining_seconds=42" ]
    [ "$(modeof "$T/state/demo/web1/result.status")" = "600" ]
}

# ---------- bildirim: outputs doğrulaması ----------
mf() { printf '%s\n' "$1" > "$T/m.json"; }
BASE='"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"u","status":"supported"}]'

@test "bildirim outputs: geçerli biçim kabul; yanlış prefix, tanımsız dir_var, boş/yinelenen keys reddedilir" {
    mf "{$BASE,\"actions\":{\"s\":{\"cmd\":\"x\",\"effects\":[],\"outputs\":{\"format\":\"keyvalue-line\",\"prefix\":\"SSHPOCKET_RESULT demo.s\",\"keys\":[\"a\"]}}}}"
    run comp_manifest_info "$T/m.json" demo 1.0.0; [ "$status" -eq 0 ]
    for o in '"format":"keyvalue-line","prefix":"SSHPOCKET_RESULT demo.baska","keys":["a"]' '"format":"keyvalue-line","keys":[]' \
             '"format":"keyvalue-line","keys":["a","a"]' '"format":"keyvalue-line","keys":["A!"]' '"format":"json"' '"dir_var":"yok"' '"keys":["a"]'; do
        mf "{$BASE,\"actions\":{\"s\":{\"cmd\":\"x\",\"effects\":[],\"outputs\":{$o}}}}"
        run comp_manifest_info "$T/m.json" demo 1.0.0
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $o"; return 1; }
    done
}

@test "sudo-rs: COMP_CTX_BECOME_EXE envantere geçer; ayarlı değilse geçmez" {
    run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 0 ]
    run ! grep -q "ansible_become_exe" "$T/seen_inv"
    COMP_CTX_BECOME_EXE=/usr/bin/sudo.ws run comp_exec_action "$INFO" demo 1.0.0 status </dev/null; [ "$status" -eq 0 ]
    grep -q "ansible_become_exe=/usr/bin/sudo.ws" "$T/seen_inv"
}

@test "yanlış sudo parolası: 'Incorrect sudo password' çıktısında yeniden deneme ipucu gösterilir" {
    FAKE_RC=4 FAKE_OUT='fatal: [web1]: FAILED! => {"msg": "Incorrect sudo password"}' run comp_exec_action "$INFO" demo 1.0.0 status </dev/null
    [ "$status" -eq 4 ]
    [[ "$output" == *"Sudo parolası reddedildi"* ]] || false
    FAKE_RC=4 FAKE_OUT='baska bir hata' run comp_exec_action "$INFO" demo 1.0.0 status </dev/null
    [[ "$output" != *"Sudo parolası reddedildi"* ]] || false
}

@test "sudo-rs: 'privilege escalation prompt' zaman aşımında anlaşılır ipucu gösterilir" {
    FAKE_RC=4 FAKE_OUT='[ERROR]: Task failed: Timeout (12s) waiting for privilege escalation prompt:' run comp_exec_action "$INFO" demo 1.0.0 status </dev/null
    [ "$status" -eq 4 ]
    [[ "$output" == *"Sudo istemi beklenirken zaman aşımı"* ]] || false
    [[ "$output" == *"sudo-rs"* ]] || false
    FAKE_RC=4 FAKE_OUT='baska bir hata' run comp_exec_action "$INFO" demo 1.0.0 status </dev/null
    [[ "$output" != *"Sudo istemi beklenirken"* ]] || false
}

# ---------- SSHPOCKET_PROGRESS ayrıştırma ----------
@test "ilerleme: tırnaklı/girintili satırlar okunur, alan başına son durum, bilinmeyen anahtar ve bozuk satır yok sayılır" {
    cat > "$T/log" <<'LOG'
TASK [x] ***
    "msg": "SSHPOCKET_PROGRESS alan=ssh sira=1/3 durum=basladi"
ok: [h] => {"msg": "SSHPOCKET_PROGRESS alan=ssh sira=1/3 durum=bitti degisen=2 atlanan=1 kesin=1"}
  "msg": "SSHPOCKET_PROGRESS alan=aide sira=2/3 durum=basladi",
  "msg": "SSHPOCKET_PROGRESS alan=aide sira=2/3 durum=basladi bekle_tahmin=300",
  SSHPOCKET_PROGRESS alan=ufw sira=3/3 durum=bitti degisen=0 atlanan=0 uyari=2 yeni=1
SSHPOCKET_PROGRESS alan=BOZUK sira=x durum=bitti
LOG
    run comp_progress_parse "$T/log"
    [ "$status" -eq 0 ]
    [ "${#lines[@]}" -eq 3 ]
    [ "${lines[0]}" = $'ssh\t1\t3\tbitti\t2\t1\t1\t-\t-' ]
    [ "${lines[1]}" = $'aide\t2\t3\tbasladi\t-\t-\t-\t-\t300' ]
    [ "${lines[2]}" = $'ufw\t3\t3\tbitti\t0\t0\t-\t2\t-' ]
}

@test "ilerleme: satır yoksa ya da dosya yoksa çıktı boş ve dönüş 1" {
    printf 'PLAY RECAP\n' > "$T/log"
    run comp_progress_parse "$T/log"; [ "$status" -eq 1 ] && [ -z "$output" ]
    run comp_progress_parse "$T/yok";  [ "$status" -eq 1 ]
}

@test "ilerleme: SSHPOCKET_RESULT ayrıştırıcısıyla çakışmaz" {
    printf 'SSHPOCKET_PROGRESS alan=ssh sira=1/1 durum=bitti degisen=1\nSSHPOCKET_RESULT demo.status armed=yes\n' > "$T/log"
    comp_exec_parse_result "$INFO" status "$T/log"
    [ "$COMP_EXEC_RESULT" = "armed=yes" ]
}

# ---- kalıcı eylem günlüğü ----
@test "günlük: her eylem logs/ altına tarih-saatli, 0600 dosya yazar; başlık ve sonuç satırları var, last-run.log kalır" {
    FAKE_OUT='merhaba çıktı' run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    local d="$T/state/demo/web1/logs" f
    [ "$(modeof "$d")" = "700" ]
    f=$(ls "$d"/*-preview.log)
    [[ "$f" =~ /[0-9]{8}-[0-9]{6}-preview\.log$ ]] || false
    [ "$(modeof "$f")" = "600" ]
    grep -q '^# paket: demo 1.0.0$' "$f"
    grep -q '^# eylem: preview$' "$f"
    grep -q '^# sunucu: web1$' "$f"
    grep -q '^# zaman: .* (yerel) / .*Z (UTC)$' "$f"
    grep -q '^# komut: .*harden.yml' "$f"
    grep -q '^# süre: [0-9]*s$' "$f"
    grep -q '^merhaba çıktı$' "$f"
    grep -q '^# çıkış kodu: 0$' "$f"
    grep -q '^# sonuç: ok$' "$f"
    [ -f "$T/state/demo/web1/last-run.log" ]
    [ ! -e "$T/state/demo/web1/last-fail-log" ]
    grep -q '|1.0.0|preview|ok$' "$T/state/demo/web1/results"       # results biçimi değişmedi
}

@test "günlük: başarısızlıkta sonuç fail, çıkış kodu yazılır, last-fail-log yolu gösterir; sonraki başarı onu siler; önceki günlük silinmez" {
    FAKE_RC=4 FAKE_OUT='asıl hata burada' run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 4 ]
    local s="$T/state/demo/web1" f
    f=$(cat "$s/last-fail-log")
    [ -f "$f" ]
    grep -q '^asıl hata burada$' "$f"
    grep -q '^# çıkış kodu: 4$' "$f"
    grep -q '^# sonuç: fail$' "$f"
    sleep 1
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [ ! -e "$s/last-fail-log" ]
    [ -f "$f" ]                                                     # başarısız günlük ezilmedi
    [ "$(ls "$s/logs" | wc -l | tr -d ' ')" = "2" ]
    run grep -q 'asıl hata' "$s/last-run.log"                          # canlı günlük ezilir, arşiv kalır
    [ "$status" -ne 0 ]
}

@test "günlük: parola, token ve özel anahtar izleri arşivde maskelenir" {
    local kt="OPENSSH PRIVATE"            # düz yazılırsa depo-hijyen testi sahte anahtar sanır
    FAKE_OUT=$'BECOME password: gizli123\nsudo_password=abc999\napi_token: ghp_ABCDEFGH12345678\n-----BEGIN '"$kt"$' KEY-----\nAAAAB3NzaC1\n-----END '"$kt"$' KEY-----\nnormal satır' \
        run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    local f
    f=$(ls "$T/state/demo/web1/logs"/*-preview.log)
    run grep -q 'gizli123\|abc999\|ABCDEFGH12345678\|AAAAB3NzaC1\|BEGIN OPENSSH' "$f"
    [ "$status" -ne 0 ]
    grep -q 'özel anahtar bloğu gizlendi' "$f"
    grep -q '^normal satır$' "$f"
    grep -q '\*\*\*\*' "$f"
}

@test "günlük: COMP_LOG_KEEP kadar günlük kalır, en eskiler silinir" {
    local d="$T/state/demo/web1/logs" i
    mkdir -p "$d"; chmod 700 "$d"
    for i in 1 2 3 4 5; do : > "$d/2020010$i-000000-preview.log"; done
    COMP_LOG_KEEP=3 run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [ "$(ls "$d" | wc -l | tr -d ' ')" = "3" ]
    [ ! -e "$d/20200101-000000-preview.log" ]
    [ ! -e "$d/20200103-000000-preview.log" ]
    [ -e "$d/20200105-000000-preview.log" ]
    ls "$d" | grep -q "^20[2-9][0-9][01][0-9][0-3][0-9]-[0-9]*-preview.log$"
}

@test "günlük: reddedilen (çalışmayan) eylem günlük bırakmaz" {
    comp_gate_requires_vars() { return 1; }
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ ! -d "$T/state/demo/web1/logs" ]
}

@test "günlük: Ctrl-C/TERM ile kesilen eylem (alt kabuk 130) 'kesildi (sinyal)' olarak arşivlenir" {
    FAKE_RC=130 FAKE_OUT='yarım çıktı' run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 130 ]
    local f
    f=$(cat "$T/state/demo/web1/last-fail-log")
    grep -q '^yarım çıktı$' "$f"
    grep -q '^# sonuç: fail — kesildi (sinyal 2)$' "$f"
}

@test "günlük: üst kabuğa HUP gelince (terminal kapandı) günlük arşivlenir, kayıt fail olur" {
    COMP_EXEC_SIG_CTX="demo|1.0.0|preview|web1|$(date +%s)"
    mkdir -p "$T/state/demo/web1"; printf 'canlı satır\n' > "$T/state/demo/web1/last-run.log"
    run _comp_exec_on_signal 129
    [ "$status" -eq 129 ]
    local f
    f=$(ls "$T/state/demo/web1/logs"/*-preview.log)
    grep -q '^canlı satır$' "$f"
    grep -q 'kesildi (sinyal 1)' "$f"
    grep -q '|1.0.0|preview|fail$' "$T/state/demo/web1/results"
}

@test "günlük: büyük harfli ve Türkçe anahtar adları da maskelenir (MARIADB_ROOT_PASSWORD, API_TOKEN, SECRET_KEY, PAROLA)" {
    FAKE_OUT=$'MARIADB_ROOT_PASSWORD=abc123\nREDIS_PASSWORD=redis777\nAPI_TOKEN=tok888\nSECRET_KEY=sec999\nPAROLA=par111\nparola: par222\nçalışan değişken=1' \
        run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    local f
    f=$(ls "$T/state/demo/web1/logs"/*-preview.log)
    run grep -E 'abc123|redis777|tok888|sec999|par111|par222' "$f"
    [ "$status" -ne 0 ]
    grep -q '^MARIADB_ROOT_PASSWORD=\*\*\*\*$' "$f"
    grep -q '^çalışan değişken=1$' "$f"
}
