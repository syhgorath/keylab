#!/usr/bin/env bats
# Aşama 3, PR D: --components run (sahte ssh ve ansible-playbook; ağ yok, VM'e dokunmaz)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    for f in Validate ModuleFiles SSHModule SSHHelper SSHModuleEdit Servers Components ComponentsInstall ComponentsManifest ComponentsVenv \
             ComponentsRun ComponentsVars ComponentsGates ComponentsExec ComponentsRunCLI ComponentsCLI; do source "$H/$f.sh"; done
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_SERVERS_DIR="$T/servers" SSHPOCKET_COMPONENTS_DIR="$T/comps" SSHPOCKET_STATE_DIR="$T/state" TMPDIR="$T/tmp"
    mkdir -p "$T/tmp" "$T/bin" "$T/venv/bin" "$T/servers/web1" "$T/comps/demo/1.0.0"
    export PATH="$T/bin:$PATH"
    printf 'IP=10.1.10.193\nUSER=ziya\nPORT=22\n' > "$T/servers/web1/server.env"
    : > "$T/servers/web1/id_ed25519_web1"
    printf 'allow_from=10.1.10.0/24\n' > "$T/servers/web1/demo.vars"
    cat > "$T/comps/demo/1.0.0/component.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "requires":{"control":{"python":">=3.12"}},
 "actions":{"preview":{"cmd":"ansible-playbook playbooks/harden.yml --check","effects":[]}},
 "vars":[{"name":"allow_from","type":"list[cidr]","required":true}],
 "invoke":{"append":[{"arg":"--ask-become-pass","required":true,"interactive":true}]}}
JSON
    echo "[defaults]" > "$T/comps/demo/1.0.0/ansible.cfg"
    INFO=$(comp_manifest_info "$T/comps/demo/1.0.0/component.json" demo 1.0.0)
    # kapsam dışı bağımlılıklar (kendi testleri var) sahtelenir
    comp_prepare() { printf '%s\n' "$INFO"; }
    comp_gate_known_host() { return 0; }
    comp_gate_agent() { return 0; }
    comp_venv_lookup() { [ -e "$T/venv_ok" ] && printf '%s' "$T/venv"; }
    comp_venv_ensure() { echo ensure >> "$T/ensure_calls"; touch "$T/venv_ok"; printf '%s' "$T/venv"; }
    # sahte ssh: argümanları kaydeder, os-release basar
    cat > "$T/bin/ssh" <<STUB
#!/bin/bash
echo "\$*" >> "$T/ssh_calls"
[ "\${FAKE_SSH_FAIL:-0}" = 1 ] && { [ -n "\${FAKE_SSH_ERR:-}" ] && printf '%s\\n' "\$FAKE_SSH_ERR" >&2; exit 255; }
case "\$*" in *"date +%s"*) if [ -n "\${FAKE_DATE_OUT+x}" ]; then printf '%s\\n' "\$FAKE_DATE_OUT"; else echo \$(( \$(date +%s) + \${FAKE_SKEW:-0} )); [ -z "\${FAKE_TZ_OUT:-}" ] || printf '%s\\n' "\$FAKE_TZ_OUT"; fi; exit 0 ;; esac
case "\$*" in *sudo.ws*) [ "\${FAKE_SUDOWS:-1}" = 1 ] && { echo SSHPOCKET_SUDO_WS=yes; exit 0; } || exit 1 ;; esac
printf '%s\n' "\${FAKE_OSREL:-NAME=\"Ubuntu\"
ID=ubuntu
VERSION_ID=\"26.04\"}"
STUB
    chmod +x "$T/bin/ssh"
    cat > "$T/venv/bin/ansible-playbook" <<STUB
#!/bin/bash
echo "\$*" > "$T/ansible_args"
while [ \$# -gt 0 ]; do case "\$1" in -e) cp "\${2#@}" "$T/seen_vars";; -i) cp "\$2" "$T/seen_inv";; esac; shift; done
exit \${FAKE_RC:-0}
STUB
    chmod +x "$T/venv/bin/ansible-playbook"
    touch "$T/venv_ok"
}

modeof() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }

@test "run: sunucu bilgisi okunur, OS salt okunur ssh ile algılanır, eylem çalışır, kayıtlar yazılır" {
    run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"Profil: ubuntu-26.04"* ]] || false
    [ "$(wc -l < "$T/ssh_calls" | tr -d ' ')" = "3" ]      # profil algılama + saat sorgusu + sudo.ws sorgusu (hepsi salt okunur)
    grep -q -- "-o BatchMode=yes" "$T/ssh_calls"
    grep -q -- "-o StrictHostKeyChecking=yes" "$T/ssh_calls"
    # uzak komut SABİT: satırın tamamı birebir eşleşmeli (ek komut sızdırılamaz)
    [ "$(head -n 1 "$T/ssh_calls")" = "-o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 -o PreferredAuthentications=publickey -o PasswordAuthentication=no -p 22 -i $T/servers/web1/id_ed25519_web1 ziya@10.1.10.193 cat /etc/os-release" ]
    [ "$(sed -n 3p "$T/ssh_calls" | grep -c 'test -x /usr/bin/sudo.ws')" = "1" ]      # üçüncü çağrı: salt okunur sudo.ws sorgusu
    [ "$(sed -n 2p "$T/ssh_calls" | grep -c " date +%s; timedatectl show -p Timezone -p NTPSynchronized 2>/dev/null\$")" = "1" ]   # ikinci çağrı: sabit, salt okunur saat sorgusu
    grep -q -- "--limit web1" "$T/ansible_args"
    grep -q '"allow_from"' "$T/seen_vars"                 # servers/web1/demo.vars kullanıldı
    grep -q '|1.0.0|preview|ok$' "$T/state/demo/web1/results"
    grep -qE '^[0-9TZ:-]+\|demo\|1.0.0\|preview\|ok$' "$T/servers/web1/components.log"
    [ "$(modeof "$T/servers/web1/components.log")" = "600" ]
}

@test "run: playbook başarısızsa kod döner ve components.log'a 'fail' yazılır" {
    FAKE_RC=2 run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 2 ]
    grep -q '|preview|fail$' "$T/servers/web1/components.log"
}

@test "run: profil desteklenmiyorsa çalıştırılmaz (190), components.log'a yazılmaz" {
    FAKE_OSREL=$'ID=debian\nVERSION_ID="13"' run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 190 ]
    [ ! -e "$T/ansible_args" ]
    [ ! -e "$T/servers/web1/components.log" ]
}

@test "run: ssh algılaması başarısız ya da os-release bozuk/kötü niyetliyse durur" {
    FAKE_SSH_FAIL=1 run comp_cli_run demo preview web1 </dev/null;  [ "$status" -eq 190 ]
    FAKE_OSREL=$'ID=ubuntu;rm -rf\nVERSION_ID="26.04"' run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]
    [[ "$output" == *"algılanamadı"* ]]          # bozuk değer hiç profil olarak kullanılmaz/gösterilmez
    [[ "$output" != *"rm -rf"* ]] || false
    FAKE_OSREL=$'ID=ubuntu' run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]
    FAKE_OSREL=' ' run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]
    [ ! -e "$T/ansible_args" ]
}

@test "run: Python ortamı yoksa onay istenir; H/EOF kurmaz, e kurar ve devam eder" {
    rm -f "$T/venv_ok"
    run comp_cli_run demo preview web1 <<<"h";    [ "$status" -eq 190 ]; [ ! -e "$T/ensure_calls" ]; [ ! -e "$T/ssh_calls" ]
    run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]; [ ! -e "$T/ensure_calls" ]
    run comp_cli_run demo preview web1 <<<"e";    [ "$status" -eq 0 ];   [ -e "$T/ensure_calls" ]
    [[ "$output" == *"yaklaşık 10 MB"* ]] || false
    [ -e "$T/ansible_args" ]
}

@test "run: ortam hazırsa hiçbir şey sorulmaz/kurulmaz" {
    run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [ ! -e "$T/ensure_calls" ]
}

@test "run: geçersiz argüman, bilinmeyen paket eylemi/sunucu ve eksik key ssh'a hiç gitmez" {
    run comp_cli_run demo preview;                       [ "$status" -eq 2 ]
    run comp_cli_run "Demo!" preview web1 </dev/null;    [ "$status" -eq 2 ]
    run comp_cli_run demo "pre view" web1 </dev/null;    [ "$status" -eq 2 ]
    run comp_cli_run demo preview "../x" </dev/null;     [ "$status" -eq 2 ]
    run comp_cli_run demo preview ".." </dev/null;       [ "$status" -eq 2 ]
    run comp_cli_run demo yok web1 </dev/null;           [ "$status" -eq 2 ]
    run comp_cli_run demo preview yok </dev/null;        [ "$status" -eq 2 ]
    rm "$T/servers/web1/id_ed25519_web1"
    run comp_cli_run demo preview web1 </dev/null;       [ "$status" -eq 190 ]
    [ ! -e "$T/ssh_calls" ]
}

@test "run: platforma uymayan eylem ssh'a gitmeden reddedilir (2); platform uyunca çalışır" {
    python3 - "$T/comps/demo/1.0.0/component.json" <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p))
d["platform_var"]="plat"
d["vars"]=[{"name":"allow_from","type":"list[cidr]","required":True},
           {"name":"plat","type":"enum","values":["generic","proxmox-host"],"default":"generic"}]
d["actions"]["pve_check"]={"cmd":"ansible-playbook playbooks/pve.yml --check","effects":[],"platforms":["proxmox-host"]}
json.dump(d,open(p,"w"))
PY
    INFO=$(comp_manifest_info "$T/comps/demo/1.0.0/component.json" demo 1.0.0)
    run comp_cli_run demo pve_check web1 </dev/null
    [ "$status" -eq 2 ]
    [[ "$output" == *"platformunda (generic) kullanılamaz; yalnızca: proxmox-host"* ]] || false
    [ ! -e "$T/ssh_calls" ]
    printf 'allow_from=10.1.10.0/24\nplat=proxmox-host\n' > "$T/servers/web1/demo.vars"
    touch "$T/venv_ok"
    run comp_cli_run demo pve_check web1 </dev/null
    [ -e "$T/ssh_calls" ]
}

@test "run: server.env'de geçersiz IP/kullanıcı reddedilir" {
    printf 'IP=10.0.0.1;rm\nUSER=ziya\n' > "$T/servers/web1/server.env"
    run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]
    printf 'IP=10.0.0.1\nUSER=-oProxyCommand=x\n' > "$T/servers/web1/server.env"
    run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]
    [ ! -e "$T/ssh_calls" ]
}

@test "run: kapı (known_hosts / agent) durdurursa ssh algılaması bile yapılmaz" {
    comp_gate_known_host() { return 1; }
    run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]
    [ ! -e "$T/ssh_calls" ]
    comp_gate_known_host() { return 0; }
    comp_gate_agent() { return 1; }
    run comp_cli_run demo preview web1 </dev/null; [ "$status" -eq 190 ]
    [ ! -e "$T/ssh_calls" ]
}

@test "components_cli: run alt komutu yönlendirilir; kullanım mesajı run'ı içerir" {
    run components_cli run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    run components_cli bilinmeyen
    [ "$status" -eq 2 ]
    [[ "$output" == *"run <paket> <eylem> <sunucu>"* ]] || false
}

@test "venv_lookup: sağlıklı ortam yoksa 1 döner ve hiçbir şey oluşturmaz" {
    unset -f comp_venv_lookup
    source "$H/ComponentsVenv.sh"
    export XDG_CACHE_HOME="$T/cache" HOME="$T/home"; mkdir -p "$T/home"
    printf 'x==1 --hash=sha256:%064d\n' 0 > "$T/comps/demo/1.0.0/requirements.txt"
    run comp_venv_lookup demo "$T/comps/demo/1.0.0" ">=3.0"
    [ "$status" -ne 0 ]
    [ ! -e "$T/cache" ]
    [ ! -e "$T/home/.cache/sshpocket/venv" ]
}

# ---------- gerçek Main.sh yolu (sahtelenmemiş): yükleme eksikleri burada yakalanır ----------
@test "Main.sh: --components run için gereken tüm işlevler Main.sh'ın yüklediği dosyalarda tanımlı" {
    run bash -c '
        export SISTEM_PATH="$1"
        while IFS= read -r f; do source "$SISTEM_PATH/Helpers/$f"; done < <(sed -n "s|^source \"\$SISTEM_PATH/Helpers/\(.*\.sh\)\"\$|\1|p" "$SISTEM_PATH/Main.sh")
        missing=""
        for fn in comp_cli_run comp_prepare comp_info_get comp_info_all comp_action_get comp_dir comp_id_valid comp_venv_lookup comp_venv_ensure \
                  comp_gate_not_root comp_gate_known_host comp_gate_agent comp_gate_profile comp_gate_requires comp_gate_confirm comp_gate_warn_vars \
                  comp_gate_record comp_gate_state_ensure comp_exec_action comp_exec_parse_result comp_run_vars comp_run_write_inventory \
                  comp_run_build_argv comp_detect_profile comp_ssh_hint comp_cli_validate comp_menu_server comp_menu_actions comp_menu_available comp_menu_installed comp_menu_vars comp_menu_edit_value comp_menu_edit_cidr comp_vars_describe comp_vars_check comp_vars_get comp_vars_set comp_vars_unset comp_vars_save comp_vars_missing comp_net_local comp_net_seen_by_server comp_net_suggest module_name_valid module_prefix srv_get ssh_validate_target servers_dir ssh_agent_add_retry; do
            declare -F "$fn" >/dev/null || missing="$missing $fn"
        done
        [ -z "$missing" ] || { echo "EKSİK:$missing"; exit 1; }
    ' _ "$SISTEM_PATH"
    [ "$status" -eq 0 ] || { echo "$output"; return 1; }
}

@test "Main.sh: gerçek 'components run' duman testi — 'command not found' yok, sunucuya ulaşmadan temiz hata" {
    printf '# boş kilit\n' > "$T/lock"
    SSHPOCKET_COMPONENTS_LOCK="$T/lock" run bash "$ROOT/sistem/Main.sh" --components run demo preview web1 </dev/null
    [ "$status" -ne 0 ]
    [[ "$output" != *"command not found"* ]] || false
    [ ! -e "$T/ssh_calls" ]
}

# ---------- ssh hata ipuçları ----------
@test "ssh ipucu: bilinen hatalar nedeni ve yapılacağı söyler (zaman aşımı, rota, red, host key, anahtar, ad, kesilme)" {
    local row err want
    while IFS='|' read -r err want; do
        FAKE_SSH_FAIL=1 FAKE_SSH_ERR="$err" run comp_cli_run demo preview web1 </dev/null
        [ "$status" -eq 190 ] || { echo "KOD $status: $err"; return 1; }
        [[ "$output" == *"işletim sistemi algılanamadı"* ]] || { echo "TEMEL MESAJ YOK: $err"; return 1; }
        [[ "$output" == *"$want"* ]] || { echo "İPUCU YOK [$err] → bekleniyor: $want | çıktı: $output"; return 1; }
        [ ! -e "$T/ansible_args" ]
    done <<'CASES'
ssh: connect to host 10.1.10.193 port 22: Operation timed out|hardening_ssh_allow_from
ssh: connect to host 10.1.10.193 port 22: Connection timed out|Zaman aşımı
ssh: connect to host 10.1.10.193 port 22: No route to host|VPN açık mı
ssh: connect to host 10.1.10.193 port 22: Network is unreachable|Ağ yolu yok
ssh: connect to host 10.1.10.193 port 22: Connection refused|sshd çalışmıyor
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@  @ WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED! @|host key'i DEĞİŞMİŞ
Host key verification failed.|Host key doğrulanamadı
No ED25519 host key is known for 10.1.10.193 and you have requested strict checking.|Host key doğrulanamadı
tester@10.1.10.193: Permission denied (publickey).|Anahtar reddedildi
Received disconnect from 10.1.10.193 port 22:2: Too many authentication failures|MaxAuthTries
ssh: Could not resolve hostname vm.lokal: nodename nor servname provided|Sunucu adı çözülemedi
kex_exchange_identification: read: Connection reset by peer|fail2ban
CASES
}

@test "ssh ipucu: bilinmeyen hatada ilk satır gösterilir; çıktı yoksa ipucu yok; denetim karakterleri temizlenir ve satır kesilir" {
    FAKE_SSH_FAIL=1 FAKE_SSH_ERR='ssh: tuhaf bir hata oluştu' run comp_cli_run demo preview web1 </dev/null
    [[ "$output" == *"↳ ssh: ssh: tuhaf bir hata oluştu"* ]] || false
    FAKE_SSH_FAIL=1 run comp_cli_run demo preview web1 </dev/null
    [[ "$output" == *"işletim sistemi algılanamadı"* ]] || false
    [[ "$output" != *"↳"* ]] || false                                      # stderr boşsa ipucu uydurulmaz
    local esc long
    esc=$(printf 'kotu\033[31mKIRMIZI\033[0m\001 mesaj')
    FAKE_SSH_FAIL=1 FAKE_SSH_ERR="$esc" run comp_cli_run demo preview web1 </dev/null
    [[ "$output" != *$'\033'* ]] || false
    [[ "$output" == *"kotu[31mKIRMIZI[0m mesaj"* ]] || false
    long=$(printf 'x%.0s' $(seq 1 400))
    FAKE_SSH_FAIL=1 FAKE_SSH_ERR="$long" run comp_cli_run demo preview web1 </dev/null
    [[ "$output" != *"$long"* ]] || false                                  # 160 karakterde kesilir
}

@test "ssh ipucu: hata dosyası her durumda silinir (başarılı ve başarısız)" {
    mkdir -p "$T/tmp"
    TMPDIR="$T/tmp" FAKE_SSH_FAIL=1 FAKE_SSH_ERR='Connection refused' run comp_cli_run demo preview web1 </dev/null
    [ "$(ls "$T/tmp" | grep -c sshpocket-sshdetect)" = "0" ]
    TMPDIR="$T/tmp" run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [ "$(ls "$T/tmp" | grep -c sshpocket-sshdetect)" = "0" ]
}

@test "ssh ipucu: sunucu kaynaklı metin kabuğa verilmez (komut enjeksiyonu denemesi etkisiz)" {
    FAKE_SSH_FAIL=1 FAKE_SSH_ERR='$(touch '"$T"'/PWNED); `touch '"$T"'/PWNED2`' run comp_cli_run demo preview web1 </dev/null
    [ ! -e "$T/PWNED" ]
    [ ! -e "$T/PWNED2" ]
}

@test "sudo-rs: Ubuntu'da /usr/bin/sudo.ws varsa envantere ansible_become_exe yazılır ve kullanıcı bilgilendirilir" {
    run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"sudo-rs yerine klasik sudo (/usr/bin/sudo.ws)"* ]] || false
    grep -q "ansible_become_exe=/usr/bin/sudo.ws" "$T/seen_inv"
    grep -q "test -x /usr/bin/sudo.ws" "$T/ssh_calls"
}

@test "sudo-rs: sunucuda sudo.ws yoksa ya da Ubuntu dışı profilde envantere become_exe YAZILMAZ" {
    FAKE_SUDOWS=0 run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" != *"sudo.ws"* ]] || false
    run ! grep -q "ansible_become_exe" "$T/seen_inv"
    rm -f "$T/seen_inv" "$T/ssh_calls"
    FAKE_OSREL=$'ID=debian\nVERSION_ID="13"' run comp_cli_run demo preview web1 </dev/null
    [ "$status" -ne 0 ]                                        # demo paketi yalnızca ubuntu-26.04 destekler: profil kapısı reddeder
    [ "$(wc -l < "$T/ssh_calls" | tr -d ' ')" = "2" ]          # profil + saat sorgusu; Ubuntu dışı profilde sudo.ws sorgusu bile yapılmaz
    run ! grep -q "sudo.ws" "$T/ssh_calls"
    [ ! -e "$T/seen_inv" ]
}

@test "saat: küçük fark (60 sn altı) uyarı vermez" {
    FAKE_SKEW=-30 run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" != *"Sunucu saati"* ]] || false
}

@test "saat: sunucu geride ise dakika, 'geride' ve chronyc düzeltmesiyle uyarır; engellemez" {
    FAKE_SKEW=-1800 run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"Sunucu saati yaklaşık 30 dakika geride"* ]] || false
    [[ "$output" == *"Not live until"* ]] || false
    [[ "$output" == *"sudo chronyc makestep"* ]] || false
    [[ "$output" == *"sudo systemctl restart chrony; sleep 8; sudo chronyc makestep"* ]] || false
    [[ "$output" == *"hardening_time_makestep=true"* ]] || false
    [ -e "$T/ansible_args" ]
}

@test "saat: sunucu ileride ise 'ileride' der" {
    FAKE_SKEW=7200 run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"yaklaşık 120 dakika ileride"* ]] || false
}

@test "saat: uyarı iki saati, farkı, saat dilimini ve hazır (kopyalanabilir) düzeltme komutunu gösterir" {
    FAKE_SKEW=-1800 FAKE_TZ_OUT=$'Timezone=Europe/Istanbul\nNTPSynchronized=yes' run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"saat dilimi: Europe/Istanbul"* ]] || false
    # sahte ssh uzak saati kendi `date`'iyle üretir, sshpocket yerel saati sonra okur: saniye arada dönerse fark 1-2 sn kayar (yavaş runner'da olur)
    [[ "$output" == *"Bu bilgisayar:"* && "$output" =~ Fark:\ 30\ dk\ [0-2]\ sn\ geride ]] || { echo "$output"; false; }
    [[ "$output" == *"TOTP/2FA"* ]] || false
    [[ "$output" == *"ssh -t -p 22 -i "*"ziya@10.1.10.193 'sudo systemctl restart chrony; sleep 8; sudo chronyc makestep; date -u'"* ]] || false
    [[ "$output" != *"NTPSynchronized=no"* ]] || false
    [ -e "$T/ansible_args" ]                                     # hâlâ engellemez
}

@test "saat: NTP eşitlemesi kapalıysa kalıcı olmayabileceği söylenir; saat dilimi okunamazsa o kısım atlanır" {
    FAKE_SKEW=-600 FAKE_TZ_OUT='NTPSynchronized=no' run comp_cli_run demo preview web1 </dev/null
    [[ "$output" == *"NTPSynchronized=no"* ]] || false
    [[ "$output" != *"saat dilimi:"* ]] || false
}

@test "saat: fark tam saat dilimi farkına benziyorsa ipucu verilir; rastgele farkta verilmez" {
    FAKE_SKEW=-10800 run comp_cli_run demo preview web1 </dev/null
    [[ "$output" == *"saat dilimi farkına benziyor"* ]] || false
    FAKE_SKEW=-2300 run comp_cli_run demo preview web1 </dev/null
    [[ "$output" != *"saat dilimi farkına benziyor"* ]] || false
    [[ "$output" == *"Sunucu saati yaklaşık"* ]] || false
}

@test "saat: sunucudan gelen saat dilimi/NTP satırları kabuk için güvenli değilse yok sayılır" {
    FAKE_SKEW=-600 FAKE_TZ_OUT=$'Timezone=$(touch /tmp/x)\nNTPSynchronized=maybe' run comp_cli_run demo preview web1 </dev/null
    [[ "$output" != *"saat dilimi:"* && "$output" != *'$(touch'* ]] || false
    [[ "$output" != *"NTPSynchronized=no"* ]] || false
}

@test "saat: okunamayan çıktıda sessiz geçer" {
    FAKE_DATE_OUT='bozuk çıktı' run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" != *"Sunucu saati"* ]] || false
    FAKE_DATE_OUT='' run comp_cli_run demo preview web1 </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" != *"Sunucu saati"* ]] || false
}
