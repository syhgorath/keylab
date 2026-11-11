#!/usr/bin/env bats
# Denetim bulguları (bulgular.tsv): ayrıştırıcı, manifest alanı ve menü görünümü (ağ yok, sunucuya dokunmaz)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    for f in UI Servers Components ComponentsInstall ComponentsManifest ComponentsRun ComponentsGates ComponentsExec ComponentsVars ComponentsMenu; do source "$H/$f.sh"; done
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_STATE_DIR="$T/state"
    PY="$H/component_findings.py"
    FIX="$ROOT/test/fixtures/bulgular-ornek.tsv"
    mkdir -p "$T/state/demo/web1/h/2026-10-10"
    cat > "$T/m.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{"audit":{"cmd":"ansible-playbook u.yml","effects":["installs-package"],"outputs":{"dir_var":"scan_dir","findings":"bulgular.tsv"}}},
 "vars":[{"name":"scan_dir","type":"path","description":"Çıktı klasörü"}]}
JSON
    INFO=$(comp_manifest_info "$T/m.json" demo 1.0.0)
}

place() { cp "$1" "$T/state/demo/web1/h/2026-10-10/bulgular.tsv"; }
hdr() { printf 'kimlik\trisk\ttur\taciklama\tayrinti\toneri\talan\tdeger\tkapanir_varsayilan\tplatform\n'; }

@test "ayrıştırıcı: gerçek örnek (26 bulgu, 9 eşlemeli) okunur; SSH-7408 iki satır" {
    place "$FIX"
    run python3 -I "$PY" "$T/state" demo web1 bulgular.tsv
    [ "$status" -eq 0 ]
    [[ "${lines[0]}" =~ ^#$'\037'[0-9]+$'\037'26$'\037'0$'\037'0$'\037'9$'\037' ]] || { echo "${lines[0]}"; false; }
    [ "$(printf '%s\n' "$output" | grep -c '^SSH-7408')" -eq 2 ]
    [[ "$output" == *$'SSH-7408\037bilinmiyor\037oneri\037Consider hardening SSH configuration\037MaxSessions (set 4 to 2)\037\037ssh\037hardening_ssh_max_sessions=2\037hayir\037hepsi'* ]] || false
}

@test "ayrıştırıcı: önem sırası (yuksek önce, bilinmiyor sonda), eşit riskte dosya sırası korunur" {
    { hdr
      printf 'A-1\tbilinmiyor\toneri\tx\t\t\t\t\t\thepsi\n'
      printf 'B-2\tdusuk\toneri\ty\t\t\t\t\t\thepsi\n'
      printf 'C-3\tyuksek\tuyari\tz\t\t\t\t\t\thepsi\n'
      printf 'D-4\tdusuk\toneri\tw\t\t\t\t\t\thepsi\n'; } > "$T/b.tsv"
    place "$T/b.tsv"
    run python3 -I "$PY" "$T/state" demo web1 bulgular.tsv
    [ "$(printf '%s\n' "$output" | tail -n +2 | cut -d$'\037' -f1 | tr '\n' ' ')" = "C-3 B-2 D-4 A-1 " ]
}

@test "ayrıştırıcı: geçersiz satırlar atlanır ve sayılır; fazla sütun yok sayılır; denetim karakterleri temizlenir" {
    { hdr | sed 's/$/\tekstra/'
      printf 'OK-1\torta\toneri\ta\x01b\t\t\t\t\t\thepsi\tfazla\n'
      printf 'KOTU-1\tacayip\toneri\tx\t\t\t\t\t\thepsi\n'
      printf 'KOTU-2\torta\toneri\tx\t\t\t\t\t\n'
      printf 'KOTU-3\torta\toneri\tx\t\t\tAlan!\t\t\thepsi\n'
      printf 'KOTU-4\torta\toneri\tx\t\t\tssh\tbozuk deger\t\thepsi\n'; } > "$T/b.tsv"
    place "$T/b.tsv"
    run python3 -I "$PY" "$T/state" demo web1 bulgular.tsv
    [[ "${lines[0]}" =~ ^#$'\037'[0-9]+$'\037'1$'\037'4$'\037'0$'\037'0$'\037' ]] || { echo "${lines[0]}"; false; }
    [[ "${lines[1]}" == "OK-1"$'\037'"orta"$'\037'"oneri"$'\037'"a b"* ]] || { echo "${lines[1]}"; false; }
}

@test "ayrıştırıcı: başlık bozuksa dönüş 3; dosya yoksa çıktı boş ve dönüş 0; yalnız başlık = 0 bulgu" {
    printf 'kimlik\trisk\n' > "$T/b.tsv"; place "$T/b.tsv"
    run python3 -I "$PY" "$T/state" demo web1 bulgular.tsv; [ "$status" -eq 3 ]
    run python3 -I "$PY" "$T/state" demo web1 yok.tsv; [ "$status" -eq 0 ] && [ -z "$output" ]
    hdr > "$T/b.tsv"; place "$T/b.tsv"
    run python3 -I "$PY" "$T/state" demo web1 bulgular.tsv
    [[ "${lines[0]}" =~ ^#$'\037'[0-9]+$'\037'0$'\037'0$'\037'0$'\037'0$'\037' ]] || false
}

@test "ayrıştırıcı: birden çok taramada en yenisi; sembolik bağ ve geçersiz dosya adı reddedilir" {
    mkdir -p "$T/state/demo/web1/h/2026-10-20"
    { hdr; printf 'ESKI-1\tbilgi\toneri\te\t\t\t\t\t\thepsi\n'; } > "$T/state/demo/web1/h/2026-10-10/bulgular.tsv"
    touch -t 202610100000 "$T/state/demo/web1/h/2026-10-10/bulgular.tsv"
    { hdr; printf 'YENI-1\tbilgi\toneri\tn\t\t\t\t\t\thepsi\n'; } > "$T/state/demo/web1/h/2026-10-20/bulgular.tsv"
    run python3 -I "$PY" "$T/state" demo web1 bulgular.tsv
    [[ "$output" == *YENI-1* && "$output" != *ESKI-1* ]] || false
    rm "$T/state/demo/web1/h/2026-10-20/bulgular.tsv"; ln -s "$FIX" "$T/state/demo/web1/h/2026-10-20/bulgular.tsv"
    run python3 -I "$PY" "$T/state" demo web1 bulgular.tsv
    [[ "$output" == *ESKI-1* ]] || false                      # bağ yok sayıldı, eski gerçek dosya
    run python3 -I "$PY" "$T/state" demo web1 ../x.tsv; [ "$status" -eq 2 ]
}

@test "manifest: outputs.findings düz .tsv adı olmalı ve info'ya yazılır" {
    [ "$(comp_action_get "$INFO" audit outputs.findings)" = "bulgular.tsv" ]
    for bad in '"a/b.tsv"' '"../x.tsv"' '"b.txt"' '"x..tsv"' '5'; do
        sed "s|\"findings\":\"bulgular.tsv\"|\"findings\":$bad|" "$T/m.json" > "$T/bad.json"
        run comp_manifest_info "$T/bad.json" demo 1.0.0
        [ "$status" -ne 0 ] || { echo "kabul edildi: $bad"; false; }
    done
}

@test "menü: hazırlık satırı sayıyı ve eşleşenleri yazar; dosya yoksa sessiz; bozuk başlıkta uyarır" {
    run _comp_menu_findings_lines demo web1 "$INFO"; [ -z "$output" ]
    place "$FIX"
    run _comp_menu_findings_lines demo web1 "$INFO"
    [[ "$output" == *"Bulgular: 26 (9 tanesi paket seçeneğiyle eşleşir)"* ]] || { echo "$output"; false; }
    printf 'kimlik\trisk\n' > "$T/b.tsv"; place "$T/b.tsv"
    run _comp_menu_findings_lines demo web1 "$INFO"
    [[ "$output" == *"okunamadı"* ]] || false
}

@test "menü: bulgu kartı önem, ayrıntı, öneri (text: öneki atılır) ve paket karşılığını gösterir" {
    run _comp_menu_findings_card $'SSH-7408\037bilinmiyor\037oneri\037Consider hardening SSH configuration\037MaxSessions (set 4 to 2)\037text:Use it\037ssh\037hardening_ssh_max_sessions=2\037hayir\037hepsi'
    [[ "$output" == *"SSH-7408 — öneri · önem: bilinmiyor"* && "$output" == *"MaxSessions (set 4 to 2)"* && "$output" == *"Use it"* && "$output" != *"text:"* ]] || { echo "$output"; false; }
    [[ "$output" == *"Paket karşılığı: ssh → hardening_ssh_max_sessions=2"* && "$output" == *"değişkeni elle ayarlayın"* ]] || false
    run _comp_menu_findings_card $'BOOT-5122\037bilinmiyor\037oneri\037GRUB\037\037\037\037\037\037proxmox'
    [[ "$output" == *"Paket karşılığı: yok"* && "$output" == *"Platform: proxmox"* ]] || false
}
