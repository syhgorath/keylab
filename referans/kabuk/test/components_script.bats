#!/usr/bin/env bats
# runner: script — betik SUNUCUDA çalışır (sahte ssh; ağ yok, VM'e dokunmaz). Kapılar/onaylar ansible ile ortaktır (components_exec.bats).
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    for f in Components ComponentsInstall ComponentsManifest ComponentsRun ComponentsGates ComponentsExec; do source "$H/$f.sh"; done
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_STATE_DIR="$T/state" TMPDIR="$T/tmp"
    mkdir -p "$T/tmp" "$T/pkg/scripts" "$T/bin"
    export PATH="$T/bin:$PATH"
    cat > "$T/pkg/component.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"script",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "saglik":{"cmd":"scripts/saglik.sh","effects":[],
            "outputs":{"format":"keyvalue-line","keys":["disk","yeniden_baslat"]}},
  "sudo_sonuc":{"cmd":"scripts/sudo_sonuc.sh","effects":[],"requires_sudo":true,
            "outputs":{"format":"keyvalue-line","keys":["disk","yeniden_baslat"]}},
  "temizlik_onizle":{"cmd":"scripts/temizlik.sh --dry-run","effects":[]},
  "temizlik":{"cmd":"scripts/temizlik.sh","effects":["changes-config"],"requires_sudo":true,"requires":["temizlik_onizle"],"confirm":"yes-no"}},
 "vars":[{"name":"esik","type":"int","default":80,"min":1,"max":100},
         {"name":"not_metni","type":"string"}]}
JSON
    cat > "$T/pkg/scripts/saglik.sh" <<'SH'
#!/usr/bin/env bash
echo "ARGS:[$*] SERVER=$SSHPOCKET_SERVER ACTION=$SSHPOCKET_ACTION ESIK=$SSHPOCKET_VAR_ESIK NOT=$SSHPOCKET_VAR_NOT_METNI"
echo "SSHPOCKET_RESULT demo.saglik disk=41 yeniden_baslat=hayir"
SH
    printf '#!/usr/bin/env bash\necho "SSHPOCKET_RESULT demo.sudo_sonuc disk=41 yeniden_baslat=hayir"\n' > "$T/pkg/scripts/sudo_sonuc.sh"
    cat > "$T/pkg/scripts/temizlik.sh" <<'SH'
#!/usr/bin/env bash
echo "TEMIZLIK ARGS:[$*] ESIK=$SSHPOCKET_VAR_ESIK"
exit "${FAKE_SCRIPT_RC:-0}"
SH
    INFO=$(comp_manifest_info "$T/pkg/component.json" demo 1.0.0)
    # sahte ssh: çağrıyı kaydeder; akıtılan betiği YEREL bash ile çalıştırır (yalnızca bu testin kendi betikleri)
    cat > "$T/bin/ssh" <<STUB
#!/bin/bash
echo "\$*" >> "$T/ssh_calls"
last="\${@: -1}"
case "\$last" in
  *"mktemp /tmp/sshpocket-"*) cat > "$T/uploaded"; printf '%s' "\${FAKE_RPATH:-/tmp/sshpocket-AbCd1234}"; exit 0 ;;
  "bash -s --"*) cat > "$T/streamed"; eval "\$last" < "$T/streamed"; exit \$? ;;
  *"bash -c '"*"bash /tmp/sshpocket-AbCd1234 "*)
     # SUNUCU tarafı GERÇEKTEN çalışır (betiğin kendisi): 'sudo' atlanır, install/sha256sum yerel eşdeğerlerle değiştirilir
     echo "\$last" > "$T/sudo_cmd"
     cp "$T/uploaded" /tmp/sshpocket-AbCd1234
     [ -z "\${FAKE_TAMPER:-}" ] || echo "# degistirildi" >> /tmp/sshpocket-AbCd1234
     PATH="$T/shim:\$PATH" eval "\${last#* }" > "$T/stub_out" 2>&1; rc=\$?; rm -f /tmp/sshpocket-AbCd1234
     if [ -n "\${FAKE_CRLF:-}" ]; then sed \$'s/\$/\\r/' "$T/stub_out"; else cat "$T/stub_out"; fi; exit \$rc ;;
  "rm -f "*) echo "\$last" >> "$T/rm_calls"; exit 0 ;;
esac
exit 0
STUB
    chmod +x "$T/bin/ssh"
    # kök olmayan test ortamı için: install -o/-g atılır, sha256sum = shasum
    mkdir -p "$T/shim"
    printf '#!/bin/bash\nargs=(); while [ $# -gt 0 ]; do case "$1" in -o|-g) shift 2 ;; *) args+=("$1"); shift ;; esac; done\nexec /usr/bin/install "${args[@]}"\n' > "$T/shim/install"
    printf '#!/bin/bash\nexec shasum -a 256 "$@"\n' > "$T/shim/sha256sum"
    # macOS'ta 'mktemp -d' TMPDIR'i yok sayar (Linux saygı duyar): sunucu davranışını taklit et ki sızıntı testte görülebilsin
    printf '#!/bin/bash\nif [ "$*" = "-d" ]; then exec /usr/bin/mktemp -d "$TMPDIR/tmp.XXXXXXXX"; fi\nexec /usr/bin/mktemp "$@"\n' > "$T/shim/mktemp"
    chmod +x "$T/shim/install" "$T/shim/sha256sum" "$T/shim/mktemp"
    COMP_CTX_PKGDIR="$T/pkg"; COMP_CTX_BINDIR=""; COMP_CTX_SERVER=web1; COMP_CTX_HOST=10.1.10.193
    COMP_CTX_USER=ziya; COMP_CTX_PORT=22; COMP_CTX_KEY=/Volumes/test/k; COMP_CTX_PREFIX=WEB1
    COMP_CTX_PROFILE=ubuntu-26.04; COMP_CTX_VALUES="$T/values"; COMP_CTX_BECOME_EXE=""
    printf 'esik=75\nnot_metni=it'"'"'s $(touch %s/PWNED)\n' "$T" > "$T/values"
    comp_gate_known_host() { return 0; }
    comp_gate_agent() { return 0; }
}

tmpleft() { ls -d "$T"/tmp/sshpocket-run.* 2>/dev/null | wc -l | tr -d ' '; }

@test "script (sudo'suz): betik ssh stdin'inden akar; ortam değişkenleri başa eklenir; sonuç satırı ayrıştırılıp saklanır" {
    run comp_exec_action "$INFO" demo 1.0.0 saglik </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"ARGS:[] SERVER=web1 ACTION=saglik ESIK=75"* ]] || false
    [ "$(wc -l < "$T/ssh_calls" | tr -d ' ')" = "1" ]                         # tek ssh çağrısı, mktemp/scp yok
    grep -q -- "-o StrictHostKeyChecking=yes" "$T/ssh_calls"
    grep -q -- "-o BatchMode=yes" "$T/ssh_calls"
    grep -q 'ziya@10.1.10.193 bash -s --$' "$T/ssh_calls"
    grep -q '^export SSHPOCKET_VAR_ESIK=75$' "$T/streamed"
    grep -q '^export SSHPOCKET_PROFILE=ubuntu-26.04$' "$T/streamed"
    grep -q 'SSHPOCKET_RESULT demo.saglik' "$T/streamed"                      # betik gövdesi de gitti
    grep -q '|1.0.0|saglik|ok$' "$T/state/demo/web1/results"
    [ "$(sed -n 2p "$T/state/demo/web1/result.saglik")" = "disk=41" ]
    [ "$(tmpleft)" = "0" ]
    [ ! -e "$T/PWNED" ]                                                       # değişken değeri kabukta ÇALIŞMADI
}

@test "script: değişken değeri tırnaklanarak aktarılır (kabuk meta karakterleri çalışmaz)" {
    run comp_exec_action "$INFO" demo 1.0.0 saglik </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"NOT=it's \$(touch $T/PWNED)"* ]] || false
    [ ! -e "$T/PWNED" ]
}

@test "script: cmd'deki düz argümanlar sunucuda betiğe geçer" {
    run comp_exec_action "$INFO" demo 1.0.0 temizlik_onizle </dev/null
    [ "$status" -eq 0 ]
    [[ "$output" == *"TEMIZLIK ARGS:[--dry-run] ESIK=75"* ]] || false
    grep -q 'bash -s -- --dry-run$' "$T/ssh_calls"
}

@test "script (sudo'lu): betik sunucuda 0600 geçici dosyaya yazılır, ssh -t ile çalışır, HER durumda silinir" {
    run comp_exec_action "$INFO" demo 1.0.0 temizlik_onizle </dev/null           # taze ön koşul kaydı
    [ "$status" -eq 0 ]
    rm -f "$T/ssh_calls" "$T/rm_calls"
    run comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e"
    [ "$status" -eq 0 ]
    [[ "$output" == *"TEMIZLIK ARGS:[] ESIK=75"* && "$output" == *"Sudo parolası istenecek"* ]] || false
    grep -q "umask 077; f=\$(mktemp /tmp/sshpocket-XXXXXXXX)" "$T/ssh_calls"
    grep -q -- " -t ziya@10.1.10.193 sudo bash -c " "$T/ssh_calls"
    grep -q "install -o root -g root -m 0700" "$T/sudo_cmd"                          # kök sahipli kopya
    grep -qE " bash /tmp/sshpocket-AbCd1234 [0-9a-f]{64}$" "$T/sudo_cmd"            # yolun ardından yerel SHA-256 gider
    [ "$(sed -E 's/.* bash \/tmp\/sshpocket-AbCd1234 ([0-9a-f]{64}).*/\1/' "$T/sudo_cmd")" = "$(shasum -a 256 "$T/uploaded" | cut -d' ' -f1)" ]
    grep -q '^export SSHPOCKET_VAR_ESIK=75$' "$T/uploaded"
    grep -qx "rm -f /tmp/sshpocket-AbCd1234" "$T/rm_calls"
    [ "$(tmpleft)" = "0" ]
    [ -z "$(ls -A "$T/tmp")" ]                                                       # sunucu tarafındaki kök klasör (mktemp -d) de silindi
}

@test "script (sudo'lu): COMP_CTX_BECOME_EXE (sudo.ws) kullanılır" {
    run comp_exec_action "$INFO" demo 1.0.0 temizlik_onizle </dev/null
    rm -f "$T/ssh_calls"
    COMP_CTX_BECOME_EXE=/usr/bin/sudo.ws run comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e"
    [ "$status" -eq 0 ]
    grep -q "/usr/bin/sudo.ws bash -c " "$T/ssh_calls"
}

@test "script: betik başarısızsa çıkış kodu döner, 'fail' kaydedilir; sudo'lu yolda geçici dosya yine silinir" {
    comp_exec_action "$INFO" demo 1.0.0 temizlik_onizle </dev/null >/dev/null
    rm -f "$T/rm_calls"
    FAKE_SCRIPT_RC=3 run comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e"
    [ "$status" -eq 3 ]
    grep -q '|1.0.0|temizlik|fail$' "$T/state/demo/web1/results"
    grep -qx "rm -f /tmp/sshpocket-AbCd1234" "$T/rm_calls"
    [ "$(tmpleft)" = "0" ]
}

@test "script: sunucudan beklenmedik geçici yol dönerse çalıştırılmaz (190) ve silme denenmez" {
    comp_exec_action "$INFO" demo 1.0.0 temizlik_onizle </dev/null >/dev/null
    rm -f "$T/ssh_calls" "$T/rm_calls"
    FAKE_RPATH='/tmp/x; rm -rf ~' run comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e"
    [ "$status" -eq 190 ]
    [ "$(grep -c 'sudo bash' "$T/ssh_calls")" = "0" ]
    [ ! -e "$T/rm_calls" ]
}

@test "script: ortak kapılar geçerli — taze preview yoksa, root ile ya da profil uyuşmazsa SSH'a hiç gidilmez" {
    run comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e"                      # ön koşul kaydı yok
    [ "$status" -eq 190 ]
    COMP_CTX_USER=root run comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e"
    [ "$status" -eq 190 ]
    COMP_CTX_PROFILE=rpi run comp_exec_action "$INFO" demo 1.0.0 saglik </dev/null
    [ "$status" -eq 190 ]
    [ ! -e "$T/ssh_calls" ]
}

@test "script: betik yoksa ya da sembolik bağsa çalıştırılmaz (190)" {
    rm "$T/pkg/scripts/saglik.sh"
    run comp_exec_action "$INFO" demo 1.0.0 saglik </dev/null
    [ "$status" -eq 190 ]
    ln -s /etc/hostname "$T/pkg/scripts/saglik.sh"
    run comp_exec_action "$INFO" demo 1.0.0 saglik </dev/null
    [ "$status" -eq 190 ]
    [ ! -e "$T/ssh_calls" ]
}

@test "env: component_run.py env değerleri tek tırnakla kaçırır; bool/liste/sayı biçimleri; kötü ad ve denetim karakteri reddedilir" {
    printf '{"a_b":true,"n":5,"l":["x","y z"],"s":"it'"'"'s $(x)"}' > "$T/v.json"
    run python3 -I "$ROOT/sistem/Helpers/component_run.py" env "$T/v.json" web1 saglik ubuntu-26.04
    [ "$status" -eq 0 ]
    [[ "$output" == *"export SSHPOCKET_VAR_A_B=true"* && "$output" == *"export SSHPOCKET_VAR_L='x,y z'"* && "$output" == *"export SSHPOCKET_VAR_N=5"* ]] || false
    [[ "$output" == *"export SSHPOCKET_VAR_S='it'\"'\"'s \$(x)'"* ]] || false
    printf '{"s":"a\\nb"}' > "$T/v2.json"
    run python3 -I "$ROOT/sistem/Helpers/component_run.py" env "$T/v2.json" web1 saglik ubuntu-26.04
    [ "$status" -ne 0 ]
    run python3 -I "$ROOT/sistem/Helpers/component_run.py" env "$T/v.json" "web 1;x" saglik ubuntu-26.04
    [ "$status" -ne 0 ]
}

@test "script (sudo'lu): yükleme ile sudo arasında dosya değiştirilirse özet uyuşmaz, betik ÇALIŞMAZ (97), geçici dosya yine silinir" {
    comp_exec_action "$INFO" demo 1.0.0 temizlik_onizle </dev/null >/dev/null
    rm -f "$T/rm_calls"
    FAKE_TAMPER=1 run comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e"
    [ "$status" -eq 97 ]
    [[ "$output" == *"ÖZET UYUŞMUYOR"* && "$output" == *"ÇALIŞTIRILMADI"* ]] || false
    [[ "$output" != *"TEMIZLIK ARGS"* ]] || false                                   # betik hiç çalışmadı
    grep -q '|1.0.0|temizlik|fail$' "$T/state/demo/web1/results"
    grep -qx "rm -f /tmp/sshpocket-AbCd1234" "$T/rm_calls"
    [ -z "$(ls -A "$T/tmp")" ]                                                       # uyuşmazlıkta da kök klasör silindi
}

@test "script (sudo'lu): uzak komut tek tırnaklı sabit bir betik; kullanıcı değerleri/argümanlar tırnağın DIŞINDA ve kaçırılmış" {
    comp_exec_action "$INFO" demo 1.0.0 temizlik_onizle </dev/null >/dev/null
    comp_exec_action "$INFO" demo 1.0.0 temizlik <<<"e" >/dev/null
    [[ "$(cat "$T/sudo_cmd")" == "sudo bash -c 'd=\$(mktemp -d)"* ]] || false
    [[ "$(cat "$T/sudo_cmd")" != *"esik"* && "$(cat "$T/sudo_cmd")" != *"PWNED"* ]] || false    # değişken değerleri komut satırında YOK (yalnızca dosyada)
}

@test "script (sudo'lu): ssh -t'nin CRLF satır sonları sonuç satırı ayrıştırmasını bozmaz (gerçek makinede 'gun=7\\r' hatası)" {
    FAKE_CRLF=1 run comp_exec_action "$INFO" demo 1.0.0 sudo_sonuc <<<"e"
    [ "$status" -eq 0 ]
    [[ "$output" != *"Sonuç satırı geçersiz"* ]] || false
    [ "$(sed -n 2p "$T/state/demo/web1/result.sudo_sonuc")" = "disk=41" ]
    [ "$(sed -n 3p "$T/state/demo/web1/result.sudo_sonuc")" = "yeniden_baslat=hayir" ]
    grep -q $'\r' "$T/state/demo/web1/last-run.log"                                  # günlükte CR gerçekten vardı (test anlamlı)
}
