#!/usr/bin/env bats
# Aşama 3, PR B: çalıştırma öncesi kapılar (yerel; ağ yok, VM'e dokunmaz)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsManifest.sh"
    source "$H/ComponentsGates.sh"
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_STATE_DIR="$T/state"
    cat > "$T/m.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"},{"id":"rpi","status":"planned"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook a.yml --check","effects":[]},
  "audit":{"cmd":"ansible-playbook b.yml","effects":["installs-package"],"confirm":"yes-no","warnings":["lynis kurar","rapor hassas"]},
  "apply":{"cmd":"ansible-playbook c.yml","effects":["installs-package","changes-config"],"requires":["preview"],"confirm":"type-server-name",
           "warnings":["bağlantı kopabilir"]},
  "wipe":{"cmd":"ansible-playbook d.yml","effects":["destructive"],"confirm":"none"}}}
JSON
    INFO=$(comp_manifest_info "$T/m.json" demo 1.0.0)
}

teardown() {
    # test yarıda kesilse bile ssh-agent bats'in çıktı borusunu açık tutmasın
    [ -z "${SSH_AGENT_PID:-}" ] || kill "$SSH_AGENT_PID" 2>/dev/null || true
}

# ---------- manifest: uyarı metinleri ----------
@test "bildirim: uyarı metinleri action.<ad>.warning satırları olarak gelir (sayaçtan ayrı)" {
    [ "$(comp_action_get "$INFO" audit warnings)" = "2" ]
    run comp_info_all "$INFO" action.audit.warning
    [ "${lines[0]}" = "lynis kurar" ]
    [ "${lines[1]}" = "rapor hassas" ]
    [ "${#lines[@]}" -eq 2 ]
}

# ---------- root ----------
@test "root: değiştirici eylem root ile reddedilir; salt okunur serbest; normal kullanıcı serbest" {
    run comp_gate_not_root "$INFO" apply root;    [ "$status" -ne 0 ]
    run comp_gate_not_root "$INFO" audit root;    [ "$status" -ne 0 ]
    run comp_gate_not_root "$INFO" preview root;  [ "$status" -eq 0 ]
    run comp_gate_not_root "$INFO" apply ziya;    [ "$status" -eq 0 ]
}

# ---------- known_hosts ----------
@test "known_hosts: tanınan host geçer; tanınmayan ve port farkı reddedilir; [host]:port biçimi" {
    ssh-keygen -q -t ed25519 -N "" -f "$T/hk"
    printf '10.1.10.193 %s\n[10.1.10.194]:2222 %s\n' "$(cut -d' ' -f1,2 "$T/hk.pub")" "$(cut -d' ' -f1,2 "$T/hk.pub")" > "$T/kh"
    run comp_gate_known_host 10.1.10.193 22 "$T/kh";   [ "$status" -eq 0 ]
    run comp_gate_known_host 10.1.10.194 2222 "$T/kh"; [ "$status" -eq 0 ]
    run comp_gate_known_host 10.1.10.194 22 "$T/kh";   [ "$status" -ne 0 ]
    run comp_gate_known_host 10.9.9.9 22 "$T/kh";      [ "$status" -ne 0 ]
    run comp_gate_known_host 10.1.10.193 22 "$T/yok";  [ "$status" -ne 0 ]
}

# ---------- agent ----------
@test "agent: anahtar agent'ta ise geçer; değilse eklemeyi dener (ssh_agent_add_retry); eklenemezse durur" {
    # YALNIZCA bu teste ait ayrı bir ajan: kullanıcının gerçek ajanına asla dokunma (fd 3 kapalı: bats borusu tutulmasın)
    unset SSH_AUTH_SOCK SSH_AGENT_PID
    eval "$(ssh-agent -s 3>&-)" >/dev/null
    [ -n "${SSH_AGENT_PID:-}" ] && [[ "$SSH_AUTH_SOCK" == *agent* || -S "$SSH_AUTH_SOCK" ]]
    ssh-keygen -q -t ed25519 -N "" -f "$T/k"
    run comp_gate_agent WEB1 "$T/k"
    [ "$status" -ne 0 ]
    ssh_agent_add_retry() { ssh-add "$2" >/dev/null 2>&1; }
    run comp_gate_agent WEB1 "$T/k"
    [ "$status" -eq 0 ]
    run comp_gate_agent WEB1 "$T/k"     # artık agent'ta
    [ "$status" -eq 0 ]
    run comp_gate_agent WEB1 "$T/yok"
    [ "$status" -ne 0 ]
}

# ---------- profil ----------
@test "profil: supported geçer; planned, bilinmeyen ve boş reddedilir" {
    run comp_gate_profile "$INFO" ubuntu-26.04; [ "$status" -eq 0 ]
    run comp_gate_profile "$INFO" rpi;          [ "$status" -ne 0 ]; [[ "$output" == *"planned"* ]] || false
    run comp_gate_profile "$INFO" debian-13;    [ "$status" -ne 0 ]
    run comp_gate_profile "$INFO" "";           [ "$status" -ne 0 ]
}

@test "required_applies: değiştirici ya da başka eylemin ön koşulu ise evet; salt okunur durum eylemi hayır" {
    cat > "$T/w.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook a.yml --check","effects":[]},
  "status":{"cmd":"ansible-playbook s.yml","effects":[]},
  "apply":{"cmd":"ansible-playbook c.yml","effects":["changes-config"],"requires":["preview"],"confirm":"yes-no"}}}
JSON
    local W; W=$(comp_manifest_info "$T/w.json" demo 1.0.0)
    run comp_gate_required_applies "$W" apply;   [ "$status" -eq 0 ]
    run comp_gate_required_applies "$W" preview; [ "$status" -eq 0 ]
    run comp_gate_required_applies "$W" status;  [ "$status" -ne 0 ]
}

@test "warn_applies: YALNIZCA yapılandırmayı değiştiren eylemde evet; preview, durum ve yalnızca araç kuran eylem (audit) hayır" {
    cat > "$T/w.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook a.yml --check","effects":[]},
  "status":{"cmd":"ansible-playbook s.yml","effects":[]},
  "audit":{"cmd":"ansible-playbook au.yml","effects":["installs-package"]},
  "apply":{"cmd":"ansible-playbook c.yml","effects":["installs-package","changes-config"],"requires":["preview"],"confirm":"yes-no"}}}
JSON
    local W; W=$(comp_manifest_info "$T/w.json" demo 1.0.0)
    run comp_gate_warn_applies "$W" apply;   [ "$status" -eq 0 ]
    run comp_gate_warn_applies "$W" preview; [ "$status" -ne 0 ]
    run comp_gate_warn_applies "$W" audit;   [ "$status" -ne 0 ]
    run comp_gate_warn_applies "$W" status;  [ "$status" -ne 0 ]
}

# ---------- requires ----------
@test "requires: kayıt yokken reddedilir; preview ok sonrası geçer; dizin 0700, dosya 0600" {
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -ne 0 ]
    [[ "$output" == *"önce 'preview'"* ]] || false
    comp_gate_record demo web1 1.0.0 preview ok
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -eq 0 ]
    [ "$(file_mode "$T/state/demo/web1")" = "700" ]
    [ "$(file_mode "$T/state/demo/web1/results")" = "600" ]
}

@test "requires: başarısız preview, başka sürüm, başka sunucu, eski (TTL dışı) ve gelecekteki kayıt sayılmaz" {
    comp_gate_record demo web1 1.0.0 preview fail
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -ne 0 ]
    comp_gate_record demo web1 0.9.0 preview ok
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -ne 0 ]
    comp_gate_record demo web2 1.0.0 preview ok
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -ne 0 ]
    now=$(date +%s)
    printf '%s|1.0.0|preview|ok\n' $((now - 1801)) > "$T/state/demo/web1/results"
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -ne 0 ]
    printf '%s|1.0.0|preview|ok\n' $((now + 3600)) > "$T/state/demo/web1/results"
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -ne 0 ]
    printf '%s|1.0.0|preview|ok\n' $((now - 1700)) > "$T/state/demo/web1/results"
    run comp_gate_requires "$INFO" demo web1 1.0.0 apply; [ "$status" -eq 0 ]
}

@test "requires: ön koşulu olmayan eylem geçer; geçersiz sunucu adı/kayıt girdisi reddedilir" {
    run comp_gate_requires "$INFO" demo web1 1.0.0 preview; [ "$status" -eq 0 ]
    run comp_gate_requires "$INFO" demo "../x" 1.0.0 apply;  [ "$status" -ne 0 ]
    run comp_gate_record demo "../x" 1.0.0 preview ok;       [ "$status" -ne 0 ]
    run comp_gate_record demo web1 1.0 preview ok;           [ "$status" -ne 0 ]
    run comp_gate_record demo web1 1.0.0 preview belki;      [ "$status" -ne 0 ]
    [ ! -e "$T/state/demo/../x" ]
}

# ---------- onay ----------
@test "onay none: sormadan geçer; etkiler 'yok (salt okunur)' yazılır" {
    run comp_gate_confirm "$INFO" preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"yok (salt okunur)"* ]] || false
}

@test "onay yes-no: e/evet geçer; H, boş ve EOF reddedilir; uyarılar gösterilir" {
    run comp_gate_confirm "$INFO" audit web1 <<<"e";    [ "$status" -eq 0 ]
    [[ "$output" == *"lynis kurar"* ]]; [[ "$output" == *"rapor hassas"* ]] || false
    run comp_gate_confirm "$INFO" audit web1 <<<"Evet"; [ "$status" -eq 0 ]
    run comp_gate_confirm "$INFO" audit web1 <<<"h";    [ "$status" -ne 0 ]
    run comp_gate_confirm "$INFO" audit web1 <<<"";     [ "$status" -ne 0 ]
    run comp_gate_confirm "$INFO" audit web1 </dev/null;[ "$status" -ne 0 ]
}

@test "onay type-server-name: yalnızca birebir sunucu adı geçer" {
    run comp_gate_confirm "$INFO" apply web1 <<<"web1";  [ "$status" -eq 0 ]
    run comp_gate_confirm "$INFO" apply web1 <<<"WEB1";  [ "$status" -ne 0 ]
    run comp_gate_confirm "$INFO" apply web1 <<<"web";   [ "$status" -ne 0 ]
    run comp_gate_confirm "$INFO" apply web1 <<<"e";     [ "$status" -ne 0 ]
    run comp_gate_confirm "$INFO" apply web1 </dev/null; [ "$status" -ne 0 ]
}

@test "onay type-server-name: yazım hatasında 3 deneme; üçüncüde doğru ad geçer, 3 yanlış iptal eder" {
    run comp_gate_confirm "$INFO" apply web1 <<<$'web\nwb1\nweb1';  [ "$status" -eq 0 ]
    run comp_gate_confirm "$INFO" apply web1 <<<$'a\nb\nc\nweb1';  [ "$status" -ne 0 ]
    [[ "$output" == *"3 kez uyuşmadı"* ]]
    run comp_gate_confirm "$INFO" apply web1 <<<$'a\nweb1';        [ "$status" -eq 0 ]
}

@test "onay: paket destructive eylemde 'none' yazsa bile sshpocket asgarisi (sunucu adı) uygulanır" {
    run comp_gate_confirm "$INFO" wipe web1 <<<"e";    [ "$status" -ne 0 ]
    run comp_gate_confirm "$INFO" wipe web1 <<<"web1"; [ "$status" -eq 0 ]
}

@test "warn_vars: her riskli değişken için e gerekir; EOF reddedilir; liste boşsa geçer" {
    run comp_gate_warn_vars;                         [ "$status" -eq 0 ]
    run comp_gate_warn_vars a b <<<$'e\ne';          [ "$status" -eq 0 ]
    run comp_gate_warn_vars a b <<<$'e\nh';          [ "$status" -ne 0 ]
    run comp_gate_warn_vars a </dev/null;            [ "$status" -ne 0 ]
}
