#!/usr/bin/env bats
# Sunucu eklerken platform seçimi (PR 1-2): manifestte platform_choices / onboarding_vars / requires_vars okunur ve doğrulanır;
# eylem düzeyi requires_vars kapısı. Alan yoksa davranış DEĞİŞMEZ. (sahte ansible-playbook; ağ yok)
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
    echo "[defaults]" > "$T/pkg/ansible.cfg"
}

# mf <ek-üst-düzey-json (virgülle başlar, boş olabilir)> <preview-ek-alanı (virgülle başlar, boş olabilir)>
mf() {
    cat > "$T/pkg/component.json" <<JSON
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/harden.yml --check","effects":[],"requires_sudo":true$2},
  "audit":{"cmd":"ansible-playbook playbooks/audit.yml","effects":["installs-package"],"confirm":"yes-no"}},
 "vars":[{"name":"allow_from","type":"string"},
         {"name":"demo_platform","type":"string","default":"generic"},
         {"name":"pve_apply","type":"bool","default":false},
         {"name":"risky","type":"bool","default":false,"warn_if":true},
         {"name":"mode","type":"enum","values":["a","b"],"default":"a"}],
 "platform_var":"demo_platform"$1,
 "invoke":{"append":[{"arg":"-i","value":"<inventory>","required":true},{"arg":"--limit","value":"<server>","required":true},
                     {"arg":"--ask-become-pass","required":true,"interactive":true}]}}
JSON
    INFO=$(comp_manifest_info "$T/pkg/component.json" demo 1.0.0)
}
mfbad() { mf "$@" 2>/dev/null || true; comp_manifest_info "$T/pkg/component.json" demo 1.0.0 2>&1 >/dev/null; }

CHOICES=',"platform_choices":[{"id":"generic","label":"Debian / Ubuntu"},{"id":"proxmox-host","label":"Proxmox VE hostu","description":"PVE","defaults":{"mode":"b"},"apply_vars":{"pve_apply":true},"apply_warnings":["gerçek koşu açılır"]},{"id":"raspberry-pi","label":"Raspberry Pi OS","auto_detect":false}],"onboarding_vars":["allow_from"]'

@test "manifest: alanlar yoksa hiçbir platform/requires_vars satırı üretilmez (eski paketler değişmez)" {
    mf "" ""
    [ -n "$INFO" ]
    [[ "$INFO" != *platform_choice* && "$INFO" != *onboarding_var* && "$INFO" != *requires_vars* ]] || false
}

@test "manifest: platform_choices, onboarding_vars ve requires_vars okunur" {
    mf "$CHOICES" ',"requires_vars":["allow_from"]'
    [ "$(comp_info_get "$INFO" platform_choice)" = generic ]
    [ "$(comp_info_all "$INFO" platform_choice | tr '\n' ' ')" = "generic proxmox-host raspberry-pi " ]
    [ "$(comp_info_get "$INFO" platform_choice.proxmox-host.label)" = "Proxmox VE hostu" ]
    [ "$(comp_info_get "$INFO" platform_choice.proxmox-host.default)" = "mode=b" ]
    [ "$(comp_info_get "$INFO" platform_choice.proxmox-host.apply_var)" = "pve_apply=true" ]
    [ "$(comp_info_get "$INFO" platform_choice.proxmox-host.apply_warning)" = "gerçek koşu açılır" ]
    [ "$(comp_info_get "$INFO" platform_choice.raspberry-pi.auto_detect)" = no ]
    [ "$(comp_info_get "$INFO" platform_choice.generic.auto_detect)" = yes ]
    [ "$(comp_info_get "$INFO" onboarding_var)" = allow_from ]
    [ "$(comp_action_get "$INFO" preview requires_vars)" = allow_from ]
    [ -z "$(comp_action_get "$INFO" audit requires_vars)" ]
}

@test "manifest: bozuk alanlar reddedilir (bilinmeyen değişken, apply_warnings'siz apply_vars, warn_if'li varsayılan/onboarding, kopya kimlik)" {
    run mfbad ',"platform_choices":[{"id":"a","label":"A","defaults":{"yok":"x"}}]' ""
    [[ "$output" == *"tanımlı bir değişken adı olmalı"* ]] || { echo "$output"; false; }
    run mfbad ',"platform_choices":[{"id":"a","label":"A","apply_vars":{"pve_apply":true}}]' ""
    [[ "$output" == *"apply_warnings"* ]] || { echo "$output"; false; }
    run mfbad ',"platform_choices":[{"id":"a","label":"A","defaults":{"risky":true}}]' ""
    [[ "$output" == *"warn_if"* ]] || { echo "$output"; false; }
    run mfbad ',"onboarding_vars":["risky"]' ""
    [[ "$output" == *"warn_if"* ]] || { echo "$output"; false; }
    run mfbad ',"platform_choices":[{"id":"a","label":"A"},{"id":"a","label":"B"}]' ""
    [[ "$output" == *"benzersiz"* ]] || { echo "$output"; false; }
    run mfbad "" ',"requires_vars":["yok"]'
    [[ "$output" == *"tanımlı bir değişken adı olmalı"* ]] || { echo "$output"; false; }
    run mfbad "" ',"requires_vars":[]'
    [[ "$output" == *"requires_vars 1-8"* ]] || { echo "$output"; false; }
    run mfbad ',"platform_choices":[{"id":"a","label":"A","apply_vars":{"allow_from":"x"},"apply_warnings":["w"]}]' ""
    [[ "$output" == *"bool/enum"* ]] || { echo "$output"; false; }
}

@test "kapı: requires_vars değişkeni boş/yoksa eylem reddedilir; doluysa geçer; alanı olmayan eylem etkilenmez" {
    mf "" ',"requires_vars":["allow_from"]'
    : > "$T/v"
    run comp_gate_unmet_vars "$INFO" preview "$T/v"
    [ "$output" = allow_from ]
    run comp_gate_requires_vars "$INFO" preview "$T/v"
    [ "$status" -eq 1 ]
    [[ "$output" == *"'preview' çalıştırılmadı: değeri boş değişken: allow_from."* && "$output" == *"$T/v"* && "$output" == *"allow_from=<değer>"* ]] || { echo "$output"; false; }
    printf 'allow_from=\n' > "$T/v"
    run comp_gate_requires_vars "$INFO" preview "$T/v"; [ "$status" -eq 1 ]
    printf 'allow_from=10.1.10.0/24\n' > "$T/v"
    run comp_gate_requires_vars "$INFO" preview "$T/v"; [ "$status" -eq 0 ]; [ -z "$output" ]
    : > "$T/v"
    run comp_gate_requires_vars "$INFO" audit "$T/v"; [ "$status" -eq 0 ]
}

@test "yürütme: allow_from boşken requires_vars'lı eylem ssh/ansible'a gitmeden reddedilir; audit çalışır" {
    mf "" ',"requires_vars":["allow_from"]'
    cat > "$T/bin/ansible-playbook" <<STUB
#!/bin/bash
echo ran >> "$T/ran"
exit 0
STUB
    chmod +x "$T/bin/ansible-playbook"
    COMP_CTX_PKGDIR="$T/pkg"; COMP_CTX_BINDIR="$T/bin"; COMP_CTX_SERVER=web1; COMP_CTX_HOST=10.1.10.193
    COMP_CTX_USER=ziya; COMP_CTX_PORT=22; COMP_CTX_KEY=/Volumes/test/k; COMP_CTX_PREFIX=WEB1
    COMP_CTX_PROFILE=ubuntu-26.04; COMP_CTX_VALUES="$T/values"
    comp_gate_known_host() { return 0; }
    comp_gate_agent() { return 0; }
    : > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq "${COMP_EXEC_REFUSED:-190}" ] || [ "$status" -ne 0 ]
    [[ "$output" == *"değeri boş değişken: allow_from."* ]] || { echo "$output"; false; }
    [ ! -e "$T/ran" ]
    printf 'allow_from=10.1.10.0/24\n' > "$T/values"
    run comp_exec_action "$INFO" demo 1.0.0 preview </dev/null
    [ "$status" -eq 0 ]
    [ -e "$T/ran" ]
}
