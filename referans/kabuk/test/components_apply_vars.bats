#!/usr/bin/env bats
# Platform apply_vars (PR 4): değişiklik yapan "önizle→uygula" eyleminde, type-server-name onayından önce uyarı gösterilir ve
# değişken YALNIZCA o koşuya geçirilir; değerler dosyasına yazılmaz, kullanıcının değeri ezilmez. (sahte ansible-playbook; ağ yok)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsManifest.sh"
    source "$H/ComponentsRun.sh"
    source "$H/ComponentsVars.sh"
    source "$H/ComponentsGates.sh"
    source "$H/ComponentsExec.sh"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_STATE_DIR="$T/state" TMPDIR="$T/tmp"
    mkdir -p "$T/tmp" "$T/pkg" "$T/bin"
    cat > "$T/pkg/component.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook a.yml --check","effects":[]},
  "apply":{"cmd":"ansible-playbook a.yml","effects":["changes-config"],"requires":["preview"],"confirm":"type-server-name"},
  "forced":{"cmd":"ansible-playbook a.yml","effects":["changes-config"],"requires":["preview"],"confirm":"type-server-name","set":{"pve_apply":false}},
  "wipe":{"cmd":"ansible-playbook w.yml","effects":["changes-config","destructive"],"requires":["preview"],"confirm":"type-server-name"},
  "rollback_now":{"cmd":"ansible-playbook r.yml","effects":["changes-config"],"confirm":"type-server-name"},
  "audit":{"cmd":"ansible-playbook u.yml","effects":["installs-package"],"confirm":"yes-no"}},
 "vars":[{"name":"demo_platform","type":"string","default":"generic"},
         {"name":"pve_apply","type":"bool","default":false}],
 "platform_var":"demo_platform",
 "platform_choices":[{"id":"generic","label":"G"},
   {"id":"proxmox-host","label":"P","apply_vars":{"pve_apply":true},"apply_warnings":["Bu koşuda gerçek değişiklik açılır","Önce snapshot"]}],
 "invoke":{"append":[{"arg":"-i","value":"<inventory>","required":true},{"arg":"--limit","value":"<server>","required":true}]}}
JSON
    echo "[defaults]" > "$T/pkg/ansible.cfg"
    INFO=$(comp_manifest_info "$T/pkg/component.json" demo 1.0.0)
    printf 'demo_platform=proxmox-host\n' > "$T/values"
    cat > "$T/bin/ansible-playbook" <<STUB
#!/bin/bash
echo "\$*" > "$T/args"
while [ \$# -gt 0 ]; do case "\$1" in -e) f="\${2#@}"; cp "\$f" "$T/seen_vars";; esac; shift; done
exit 0
STUB
    chmod +x "$T/bin/ansible-playbook"
    COMP_CTX_PKGDIR="$T/pkg"; COMP_CTX_BINDIR="$T/bin"; COMP_CTX_SERVER=web1; COMP_CTX_HOST=10.1.10.193
    COMP_CTX_USER=ziya; COMP_CTX_PORT=22; COMP_CTX_KEY=/Volumes/test/k; COMP_CTX_PREFIX=WEB1
    COMP_CTX_PROFILE=ubuntu-26.04; COMP_CTX_VALUES="$T/values"
    comp_gate_known_host() { return 0; }
    comp_gate_agent() { return 0; }
    comp_gate_requires() { return 0; }      # ön koşulun kendi testi components_gates.bats'ta
}

seen_pve() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("pve_apply"))' "$T/seen_vars"; }

@test "apply_vars: seçili platformda apply'a o koşuya özel geçer; uyarı onay sorusundan ÖNCE çıkar; değerler dosyasına YAZILMAZ" {
    run comp_exec_action "$INFO" demo 1.0.0 apply <<< "web1"
    [ "$status" -eq 0 ]
    [ "$(seen_pve)" = True ]
    [[ "$output" == *"pve_apply=true"* && "$output" == *"Bu koşuda gerçek değişiklik açılır"* && "$output" == *"Önce snapshot"* ]] || { echo "$output"; false; }
    # sıra: bilgi bloğu onay sorusundan önce
    pre="${output%%Onaylamak için*}"
    [[ "$pre" == *"Bu koşuda gerçek değişiklik açılır"* ]] || { echo "$output"; false; }
    [ "$(cat "$T/values")" = "demo_platform=proxmox-host" ]
    grep -q '^bu koşuya özel: pve_apply=true$' "$T/state/demo/web1/last-run.log"
}

@test "apply_vars: onay verilmezse hiçbir şey çalışmaz" {
    run comp_exec_action "$INFO" demo 1.0.0 apply <<< "yanlis"
    [ "$status" -ne 0 ]
    [ ! -e "$T/args" ]
    [ "$(cat "$T/values")" = "demo_platform=proxmox-host" ]
}

@test "apply_vars: kullanıcı değişkeni dosyaya yazdıysa (false dahil) ezilmez" {
    printf 'demo_platform=proxmox-host\npve_apply=false\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 apply <<< "web1"
    [ "$status" -eq 0 ]
    [ "$(seen_pve)" = False ]
    [[ "$output" != *"bu koşuya özel"* && "$output" != *"Bu koşuda gerçek"* ]] || { echo "$output"; false; }
}

@test "apply_vars: platform yazılı değil ya da generic ise geçmez" {
    : > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 apply <<< "web1"
    [ "$status" -eq 0 ]; [ "$(seen_pve)" = False ]
    printf 'demo_platform=generic\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 apply <<< "web1"
    [ "$status" -eq 0 ]; [ "$(seen_pve)" = False ]
    printf 'demo_platform=bilinmeyen\n' > "$T/values"
    run comp_gate_apply_vars "$INFO" apply "$T/values"
    [ -z "$output" ]
}

@test "apply_vars: preview, audit, rollback_now ve yıkıcı eylemde geçmez; eylemin kendi set'i kazanır" {
    for a in preview audit rollback_now wipe; do
        run comp_gate_apply_vars "$INFO" "$a" "$T/values"
        [ -z "$output" ] || { echo "$a: $output"; false; }
    done
    run comp_gate_apply_vars "$INFO" forced "$T/values"
    [ -z "$output" ]
    run comp_gate_apply_vars "$INFO" apply "$T/values"
    [ "$output" = "pve_apply=true" ]
}

@test "apply_vars: alan olmayan pakette hiçbir şey değişmez" {
    old="${INFO//platform_choice.proxmox-host.apply_var=*/}"
    run comp_gate_apply_vars "$old" apply "$T/values"
    [ -z "$output" ]
}
