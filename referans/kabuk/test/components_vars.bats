#!/usr/bin/env bats
# Aşama 3, PR E2a: değişken yönetimi ve erişim ağı önerisi kütüphanesi (sahte route/ifconfig/ssh; ağ yok)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/ComponentsRun.sh"
    source "$H/ComponentsVars.sh"
    T="$BATS_TEST_TMPDIR"
    mkdir -p "$T/bin"; export PATH="$T/bin:$PATH"
    cat > "$T/m.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible",
 "profiles":[{"id":"u","status":"supported"}],
 "actions":{"preview":{"cmd":"x","effects":[]}},
 "vars":[
  {"name":"allow_from","type":"list[cidr]","required":true,"description":"SSH'a izin verilen ağlar"},
  {"name":"scan_dir","type":"path","description":"Çıktı\tklasörü"},
  {"name":"rb_enabled","type":"bool","default":true,"warn_if":false,"description":"geri alma"},
  {"name":"rb_timeout","type":"int","default":300,"min":30,"max":3600},
  {"name":"mode","type":"enum","values":["a","b"],"required":true,"default":"a"}]}
JSON
    V="$T/demo.vars"
}

filemode() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }

# ---------- describe / check ----------
@test "describe: değişken başına TSV (ad, tür, zorunlu, varsayılan, min, max, warn_if, açıklama, grup, enum değerleri); sekme açıklamada temizlenir" {
    run comp_vars_describe "$T/m.json"
    [ "$status" -eq 0 ]
    [ "${#lines[@]}" -eq 5 ]
    [ "${lines[0]}" = $'allow_from\tlist[cidr]\tyes\t\t\t\t\tSSH\'a izin verilen ağlar\t\t' ]
    [ "${lines[1]}" = $'scan_dir\tpath\tno\t\t\t\t\tÇıktı klasörü\t\t' ]
    [ "${lines[2]}" = $'rb_enabled\tbool\tno\ttrue\t\t\tfalse\tgeri alma\t\t' ]
    [ "${lines[3]}" = $'rb_timeout\tint\tno\t300\t30\t3600\t\t\t\t' ]
}

@test "check: geçerli değerler geçer; tür/aralık/enum/cidr hataları, tanımsız ad ve boş değer reddedilir" {
    run comp_vars_check "$T/m.json" allow_from "10.0.0.0/24,192.168.1.5"; [ "$status" -eq 0 ]
    run comp_vars_check "$T/m.json" rb_timeout 600;                       [ "$status" -eq 0 ]
    for bad in "allow_from|10.0.0.0/99" "allow_from|evil" "rb_timeout|5" "rb_timeout|abc" "rb_enabled|belki" "mode|zzz" "yok_boyle|1" "allow_from|"; do
        run comp_vars_check "$T/m.json" "${bad%%|*}" "${bad#*|}"
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
    done
}

@test "check: warn_if eşleşirse 'warn=<ad>' yazar ve yine de geçerlidir" {
    run comp_vars_check "$T/m.json" rb_enabled false
    [ "$status" -eq 0 ]
    [ "$output" = "warn=rb_enabled" ]
    run comp_vars_check "$T/m.json" rb_enabled true
    [ "$output" = "" ]
}

# ---------- get / set / unset ----------
@test "set/get: yeni dosya oluşturur (izin 600), değer okunur; ikinci set değeri değiştirir, diğer satır ve yorumlar korunur" {
    comp_vars_set "$V" allow_from "10.0.0.0/24"
    [ "$(filemode "$V")" = "600" ]
    [ "$(comp_vars_get "$V" allow_from)" = "10.0.0.0/24" ]
    printf '# yorum\nscan_dir=/x/y\n' >> "$V"
    comp_vars_set "$V" allow_from "10.0.0.0/24,192.168.1.0/24"
    [ "$(comp_vars_get "$V" allow_from)" = "10.0.0.0/24,192.168.1.0/24" ]
    [ "$(comp_vars_get "$V" scan_dir)" = "/x/y" ]
    grep -qx '# yorum' "$V"
    [ "$(grep -c '^allow_from=' "$V")" = "1" ]
}

@test "set: yinelenen satırlar tek satıra iner; yorum satırındaki 'ad=' değişken sayılmaz; geçici dosya kalmaz" {
    printf '# allow_from=gizli\nallow_from=1.1.1.0/24\nallow_from=2.2.2.0/24\n' > "$V"
    [ "$(comp_vars_get "$V" allow_from)" = "1.1.1.0/24" ]
    comp_vars_set "$V" allow_from "3.3.3.0/24"
    [ "$(grep -c '^allow_from=' "$V")" = "1" ]
    grep -qx '# allow_from=gizli' "$V"
    [ "$(ls "$T" | grep -c 'demo.vars\.')" = "0" ]
}

@test "set: geçersiz ad, satır sonu/sekme/denetim karakteri reddedilir ve dosya değişmez" {
    printf 'allow_from=1.1.1.0/24\n' > "$V"
    before=$(cat "$V")
    run comp_vars_set "$V" "Evil" x;                          [ "$status" -ne 0 ]
    run comp_vars_set "$V" "a b" x;                           [ "$status" -ne 0 ]
    run comp_vars_set "$V" rb_timeout $'1\nallow_from=evil';  [ "$status" -ne 0 ]
    run comp_vars_set "$V" rb_timeout $'1\t2';                [ "$status" -ne 0 ]
    run comp_vars_set "$V" rb_timeout $'1\001';               [ "$status" -ne 0 ]
    [ "$(cat "$V")" = "$before" ]
}

@test "unset: satırı siler, diğerlerine dokunmaz; dosya yoksa sorun değildir" {
    printf '# k\nallow_from=1.1.1.0/24\nrb_timeout=60\n' > "$V"
    comp_vars_unset "$V" allow_from
    run comp_vars_get "$V" allow_from; [ "$status" -ne 0 ]
    [ "$(comp_vars_get "$V" rb_timeout)" = "60" ]
    grep -qx '# k' "$V"
    run comp_vars_unset "$T/yok.vars" allow_from; [ "$status" -eq 0 ]
}

@test "get: dosya ya da değişken yoksa ve geçersiz adda 1 döner" {
    run comp_vars_get "$T/yok.vars" allow_from; [ "$status" -ne 0 ]
    printf 'allow_from=1.1.1.0/24\n' > "$V"
    run comp_vars_get "$V" rb_timeout;          [ "$status" -ne 0 ]
    run comp_vars_get "$V" "Bad Name";          [ "$status" -ne 0 ]
}

# ---------- save (doğrula + yaz) ----------
@test "save: geçersiz değer dosyaya YAZILMAZ; geçerli yazılır; warn_if satırı iletilir" {
    run comp_vars_save "$T/m.json" "$V" rb_timeout 5
    [ "$status" -ne 0 ]
    [ ! -e "$V" ]
    run comp_vars_save "$T/m.json" "$V" rb_timeout 600
    [ "$status" -eq 0 ]
    [ "$(comp_vars_get "$V" rb_timeout)" = "600" ]
    run comp_vars_save "$T/m.json" "$V" rb_enabled false
    [ "$status" -eq 0 ]
    [ "$output" = "warn=rb_enabled" ]
    [ "$(comp_vars_get "$V" rb_enabled)" = "false" ]
}

# ---------- eksik zorunlu ----------
@test "missing: değeri ve varsayılanı olmayan zorunlu değişkenler listelenir; değer girilince kalkar; varsayılanlı zorunlu eksik sayılmaz" {
    run comp_vars_missing "$T/m.json" "$V"
    [ "$status" -eq 0 ]
    [ "$output" = "allow_from" ]                 # mode zorunlu ama varsayılanı var
    comp_vars_set "$V" allow_from "10.0.0.0/24"
    run comp_vars_missing "$T/m.json" "$V"
    [ "$output" = "" ]
    comp_vars_set "$V" allow_from ""
    run comp_vars_missing "$T/m.json" "$V"
    [ "$output" = "allow_from" ]
}

# ---------- erişim ağı önerisi ----------
fake_net() { # <arayüz> <ifconfig inet satırı>
    printf '%s\n' "#!/bin/bash" '[ "$1" = "-n" ] && shift' "[ \"\$1\" = get ] && printf '   route to: %s\\n  interface: %s\\n' \"\$2\" '$1'" > "$T/bin/route"
    printf '%s\n' "$1: flags=8863<UP,RUNNING>" "	inet6 fe80::1 prefixlen 64" "	$2" > "$T/ifc"
    printf '%s\n' "#!/bin/bash" "cat '$T/ifc'" > "$T/bin/ifconfig"
    chmod +x "$T/bin/route" "$T/bin/ifconfig"
}

@test "net_local: LAN arayüzünde adres/önek (0xffffff00 → /24)" {
    fake_net en0 "inet 10.1.10.190 netmask 0xffffff00 broadcast 10.1.10.255"
    run comp_net_local 10.1.10.193
    [ "$status" -eq 0 ]
    [ "$output" = "10.1.10.190/24" ]
}

@test "net_local: nokta-nokta tünelde (--> ve 0xffffffff) /32 döner; öneri /24'ü 'tahmini' diye etiketler" {
    fake_net utun4 "inet 10.99.0.4 --> 10.99.0.4 netmask 0xffffffff"
    run comp_net_local 10.1.10.193
    [ "$output" = "10.99.0.4/32" ]
    run comp_net_suggest "10.99.0.4/32"
    [ "${lines[0]}" = $'10.99.0.0/24\ttahmini /24 (ağ maskesi bilinmiyor; doğrulayın)' ]
    [ "${lines[1]}" = $'10.99.0.4/32\tyalnızca bu adres' ]
    [ "${#lines[@]}" -eq 2 ]
}

@test "net_local: rota/arayüz/maske yoksa ya da değerler kötüyse başarısız olur" {
    printf '#!/bin/bash\nexit 1\n' > "$T/bin/route"; chmod +x "$T/bin/route"
    run comp_net_local 10.1.10.193; [ "$status" -ne 0 ]
    fake_net "en0;id" "inet 10.1.10.190 netmask 0xffffff00"
    run comp_net_local 10.1.10.193; [ "$status" -ne 0 ]
    fake_net en0 "inet 10.1.10.190 netmask 0xff00ff00"                 # bitişik olmayan maske
    run comp_net_local 10.1.10.193; [ "$status" -ne 0 ]
    fake_net en0 "inet 10.1.10.190 netmask 0xffffff00"
    run comp_net_local '10.1.10.193;id'; [ "$status" -ne 0 ]
}

@test "net_suggest: /24 için ağ + yalnızca adres; geçersiz girdi reddedilir" {
    run comp_net_suggest "10.1.10.190/24"
    [ "${lines[0]}" = $'10.1.10.0/24\tbu ağdaki herkes' ]
    [ "${lines[1]}" = $'10.1.10.190/32\tyalnızca bu adres' ]
    run comp_net_suggest "10.1.10.190"                                  # önek verilmezse /32
    [ "${lines[0]}" = $'10.1.10.0/24\ttahmini /24 (ağ maskesi bilinmiyor; doğrulayın)' ]
    for bad in "abc" "10.1.10.190/99" "10.1.10.190/x" '10.1.10.1;id' ""; do
        run comp_net_suggest "$bad"
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
    done
}

@test "seen_by_server: SSH_CONNECTION'ın ilk alanı döner; ssh komutu SABİT ve host key kapatılmaz" {
    cat > "$T/bin/ssh" <<STUB
#!/bin/bash
echo "\$*" >> "$T/ssh_calls"
echo "\${FAKE_CONN:-10.99.0.4 52470 10.1.10.193 22}"
exit \${FAKE_RC:-0}
STUB
    chmod +x "$T/bin/ssh"
    run comp_net_seen_by_server 10.1.10.193 22 syhgorath /k
    [ "$status" -eq 0 ]
    [ "$output" = "10.99.0.4" ]
    [ "$(cat "$T/ssh_calls")" = "-o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 -o PreferredAuthentications=publickey -o PasswordAuthentication=no -p 22 -i /k syhgorath@10.1.10.193 echo \$SSH_CONNECTION" ]
    FAKE_CONN='evil;id 1 2 3' run comp_net_seen_by_server 10.1.10.193 22 syhgorath /k; [ "$status" -ne 0 ]
    FAKE_CONN=' ' run comp_net_seen_by_server 10.1.10.193 22 syhgorath /k;              [ "$status" -ne 0 ]
    FAKE_RC=255 run comp_net_seen_by_server 10.1.10.193 22 syhgorath /k;               [ "$status" -ne 0 ]
}

# ---------- net-add: kapsama ----------
@test "net_add: ayrık ağ eklenir; yinelenen ve kapsanan atlanır; kapsayan, dar girdilerin yerine geçer" {
    run comp_net_add "10.0.0.0/24" "192.168.1.0/24"
    [ "${lines[0]}" = "added" ];                          [ "${lines[1]}" = "10.0.0.0/24,192.168.1.0/24" ]
    run comp_net_add "10.0.0.0/24" "10.0.0.0/24"
    [ "${lines[0]}" = "duplicate" ];                      [ "${lines[1]}" = "10.0.0.0/24" ]
    run comp_net_add "10.0.0.0/16" "10.0.1.0/24"
    [ "${lines[0]}" = "covered:10.0.0.0/16" ];            [ "${lines[1]}" = "10.0.0.0/16" ]
    run comp_net_add "10.0.0.0/24,10.1.0.0/24,192.168.1.0/24" "10.0.0.0/8"
    [ "${lines[0]}" = "replaced:10.0.0.0/24,10.1.0.0/24" ]; [ "${lines[1]}" = "192.168.1.0/24,10.0.0.0/8" ]
    run comp_net_add "10.0.0.5" "10.0.0.0/24"                                   # tek adres de kapsanır
    [ "${lines[0]}" = "replaced:10.0.0.5" ];              [ "${lines[1]}" = "10.0.0.0/24" ]
    run comp_net_add "" "10.0.0.0/24"                                           # boş liste
    [ "${lines[0]}" = "added" ];                          [ "${lines[1]}" = "10.0.0.0/24" ]
}

@test "net_add: farklı IP sürümleri birbirini kapsamaz; geçersiz girdi reddedilir" {
    run comp_net_add "10.0.0.0/8" "fd00::/8"
    [ "${lines[0]}" = "added" ]
    for bad in "evil" "10.0.0.0/99" "10.0.0.0/24;id"; do
        run comp_net_add "10.0.0.0/24" "$bad"
        [ "$status" -ne 0 ] || { echo "KABUL EDİLDİ: $bad"; return 1; }
    done
    run comp_net_add "kotu,10.0.0.0/24" "10.1.0.0/24"
    [ "$status" -ne 0 ]
}

# ---------- platform süzmesi ve gruplama (isteğe bağlı manifest alanları: platform_var, platforms, group) ----------
mkplat() {
    cat > "$T/p.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","platform_var":"plat",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{"preview":{"cmd":"ansible-playbook a.yml --check","effects":[]}},
 "vars":[{"name":"plat","type":"enum","values":["generic","proxmox-host"],"default":"generic"},
         {"name":"genel_a","type":"bool","default":true},
         {"name":"pve_x","type":"bool","default":false,"platforms":["proxmox-host"],"group":"Proxmox"},
         {"name":"genel_b","type":"bool","default":false},
         {"name":"pve_y","type":"bool","default":false,"platforms":["proxmox-host"],"group":"Proxmox"},
         {"name":"ag_z","type":"bool","default":false,"group":"Ağ"}]}
JSON
}

@test "describe + vars dosyası: platforms dışındaki değişkenler gizlenir (varsayılan platform); vars dosyası verilmezse hepsi listelenir" {
    mkplat; : > "$T/vals"
    run comp_vars_describe "$T/p.json" "$T/vals"
    [ "$status" -eq 0 ]
    [[ "$output" != *"pve_x"* && "$output" != *"pve_y"* ]] || false
    [[ "$output" == *"genel_a"* && "$output" == *"genel_b"* && "$output" == *"ag_z"* ]] || false
    run comp_vars_describe "$T/p.json"
    [[ "$output" == *"pve_x"* && "$output" == *"pve_y"* ]] || false
}

@test "describe: vars dosyasında platform=proxmox-host ise Proxmox değişkenleri görünür; gruplar yan yana, grupsuzlar önce" {
    mkplat; printf 'plat=proxmox-host\n' > "$T/vals"
    run comp_vars_describe "$T/p.json" "$T/vals"
    names=$(printf '%s\n' "$output" | cut -f1 | tr '\n' ' ')
    [ "$names" = "plat genel_a genel_b pve_x pve_y ag_z " ]
    [[ "$(printf '%s\n' "$output" | grep '^pve_x' | cut -f9)" == "Proxmox" ]] || false
}

@test "describe: platform_var tanımsızsa ya da değeri belirlenemezse süzme yapılmaz" {
    mkplat; python3 - "$T/p.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1])); d.pop("platform_var"); json.dump(d,open(sys.argv[1],"w"))
PY
    : > "$T/vals"
    run comp_vars_describe "$T/p.json" "$T/vals"
    [[ "$output" == *"pve_x"* ]] || false
}
