#!/usr/bin/env bats
# Paket sürümü değişince "ne değişti, hangi sunucum etkileniyor" raporu (yalnızca okur; fixture manifest, ağ yok)
load test_helper

setup() {
    load_helpers
    export SISTEM_PATH="$ROOT/sistem"
    for f in Components ComponentsInstall ComponentsManifest ComponentsRun ComponentsVars ComponentsGates ComponentsOnboard ComponentsReport; do source "$H/$f.sh"; done
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_COMPONENTS_DIR="$T/comps" SSHPOCKET_SERVERS_DIR="$T/srv"
    mkdir -p "$T/comps/demo/1.0.0" "$T/comps/demo/1.1.0" "$T/comps/demo/2.0.0" "$T/srv/web1" "$T/srv/web2" "$T/srv/web3"
    for s in web1 web2 web3; do printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$T/srv/$s/server.env"; done
    printf '2.0.0\n' > "$T/comps/demo/current"
    mf 1.0.0 old; mf 1.1.0 old; mf 2.0.0 new
}

# mf <sürüm> <old|new>
mf() {
    local extra=""
    if [ "$2" = new ]; then
        extra=',{"name":"extra","type":"int","default":5,"min":1,"max":9}'
    fi
    local act='"preview":{"cmd":"a.yml --check","effects":[]'
    [ "$2" = new ] && act="$act"',"requires_vars":["allow_from"]'
    local top=''
    [ "$2" = new ] && top=',"platform_var":"demo_platform","platform_choices":[{"id":"generic","label":"G"},{"id":"px","label":"P"}]'
    cat > "$T/comps/demo/$1/component.json" <<JSON
{"id":"demo","version":"$1","sshpocket_api":1,"runner":"ansible","inventory_group":"hardening",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{$act},"audit":{"cmd":"u.yml","effects":["installs-package"],"confirm":"yes-no"}},
 "vars":[{"name":"allow_from","type":"list[cidr]"},{"name":"demo_platform","type":"string","default":"generic"},
         {"name":"mode","type":"enum","values":["a","b"],"default":"$( [ "$2" = new ] && echo b || echo a)"}$extra]$top}
JSON
}

@test "önceki sürüm: etkin sürümden küçük en büyük, component.json'u duran; yoksa boş" {
    [ "$(comp_report_prev_version demo)" = 1.1.0 ]
    rm "$T/comps/demo/1.1.0/component.json"
    [ "$(comp_report_prev_version demo)" = 1.0.0 ]
    rm "$T/comps/demo/1.0.0/component.json"
    [ -z "$(comp_report_prev_version demo)" ]
}

@test "değişken farkı: yeni değişken, varsayılan değişimi; kalkan değişken" {
    run comp_report_vars_diff "$T/comps/demo/1.1.0/component.json" "$T/comps/demo/2.0.0/component.json"
    [ "$output" = $'d\tmode\t"a"\t"b"\n+\textra\tint\t5' ] || { echo "$output"; false; }
    run comp_report_vars_diff "$T/comps/demo/2.0.0/component.json" "$T/comps/demo/1.1.0/component.json"
    [[ "$output" == *$'-\textra'* ]]
}

@test "rapor: değişenler, yeni kurallar, platform seçimi ve etkilenen sunucular; bu sunucuyu etkilemeyen satır yok; HİÇBİR ŞEY yazılmaz" {
    printf 'allow_from=10.1.10.0/24\ndemo_platform=px\n' > "$T/srv/web2/demo.vars"     # tamam
    printf 'mode=a\n' > "$T/srv/web1/demo.vars"                                      # allow_from boş, platform yok
    # web3: paket hiç ayarlanmamış (vars dosyası yok)
    before=$(cd "$T" && find srv comps -type f -exec shasum {} + | sort | shasum)
    run comp_report_pkg demo
    [ "$status" -eq 0 ]
    [[ "$output" == *"demo 1.1.0 → 2.0.0"* && "$output" == *"yeni değişken: extra (int, varsayılan 5)"* && "$output" == *'varsayılan değişti: mode: "a" → "b"'* ]] || { echo "$output"; false; }
    [[ "$output" == *"preview artık şu değişken(ler) dolu olmadan çalışmaz: allow_from"* && "$output" == *"Sunucu türü seçimi: generic px"* ]] || { echo "$output"; false; }
    [[ "$output" == *"web1: preview ⛔ — değeri boş: allow_from"* && "$output" == *"web1: sunucu türü seçilmemiş"* ]] || { echo "$output"; false; }
    [[ "$output" != *web2:* && "$output" != *web3:* ]] || { echo "$output"; false; }
    [ "$(cd "$T" && find srv comps -type f -exec shasum {} + | sort | shasum)" = "$before" ]
}

@test "rapor: eski sürüm elle verilebilir; fark yoksa ya da eski sürüm yoksa açıkça söyler" {
    run comp_report_pkg demo 1.0.0
    [[ "$output" == *"demo 1.0.0 → 2.0.0"* ]]
    mf 1.1.0 new; mf 2.0.0 new
    run comp_report_pkg demo 1.1.0
    [[ "$output" == *"Değişken ve kural farkı yok."* ]] || { echo "$output"; false; }
    run comp_report_pkg demo 2.0.0
    [[ "$output" == *"karşılaştırılacak eski sürüm yok"* ]]
    rm -r "$T/comps/demo/1.0.0" "$T/comps/demo/1.1.0"
    run comp_report_pkg demo
    [[ "$output" == *"karşılaştırılacak eski sürüm yok"* ]]
}

@test "rapor: kurulu olmayan paket hata verir" {
    run comp_report_pkg yok
    [ "$status" -eq 1 ]
}

@test "sync: sürüm değişen paket için rapor sonda gösterilir; değişmeyen için gösterilmez" {
    source "$H/ComponentsCLI.sh"
    comp_lock_validate() { return 0; }
    comp_ids() { echo demo; }
    comp_status() { echo outdated; }
    comp_get() { echo 2.0.0; }
    printf '1.1.0\n' > "$T/comps/demo/current"
    comp_install() { printf '2.0.0\n' > "$T/comps/demo/current"; }
    comp_report_pkg() { echo "RAPOR $1 $2"; }
    run comp_cli_sync --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"RAPOR demo 1.1.0"* ]] || { echo "$output"; false; }
    printf '2.0.0\n' > "$T/comps/demo/current"
    comp_install() { :; }
    run comp_cli_sync --yes
    [[ "$output" != *RAPOR* ]]
}
