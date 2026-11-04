#!/usr/bin/env bats
# Klasik port knocking, istemci tarafı (sahte security/nc; gerçek gönderim yerel dinleyiciyle)
load test_helper

setup() {
    load_helpers
    T="$BATS_TEST_TMPDIR"
    export PATH="$T/bin:$PATH"; mkdir -p "$T/bin"
    SEQ_FILE="$T/seq"; SECLOG="$T/sec.log"
    # sahte security: find -w → $SEQ_FILE içeriği; add → kaydı SEQ_FILE_NEW'den al; delete → sil
    cat > "$T/bin/security" <<STUB
#!/bin/bash
echo "\$*" >> "$SECLOG"
case "\$1" in
  find-generic-password) [ -f "$SEQ_FILE" ] || exit 44
                         case "\$*" in *" -w"*) cat "$SEQ_FILE" ;; esac; exit 0 ;;
  add-generic-password)  [ -f "$T/typed" ] && cp "$T/typed" "$SEQ_FILE"; exit 0 ;;
  delete-generic-password) rm -f "$SEQ_FILE"; exit 0 ;;
esac
exit 0
STUB
    chmod +x "$T/bin/security"
    D="$T/srv/web1"; mkdir -p "$D"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$D/server.env"
    # vuruşları kaydeden sahte tek vuruş
    SENT="$T/sent"; : > "$SENT"
    _knock_send_one() { echo "$1 $2 $3" >> "$SENT"; }
}

knock_on() { printf 'KNOCK=keychain\n' >> "$D/server.env"; }

@test "knock: dizi doğrulaması (1-10 port, 1-65535, virgülle)" {
    local ok bad
    for ok in 7000 7000,8000,9000 1,65535 1,2,3,4,5,6,7,8,9,10; do
        knock_ports_valid "$ok" || { echo "RET: $ok"; return 1; }
    done
    for bad in '' 0 65536 70000 '7000,' ',7000' '7000,,8000' abc '7000;8000' '7000 8000' -1 '1,2,3,4,5,6,7,8,9,10,11' '$(id)'; do
        run knock_ports_valid "$bad"
        [ "$status" -ne 0 ] || { echo "KABUL: [$bad]"; return 1; }
    done
}

@test "pre_connect: knock kapalıysa hiçbir şey yapmaz (Keychain'e dokunmaz)" {
    run knock_pre_connect WEB1 "$D" 192.0.2.7
    [ "$status" -eq 0 ]
    [ ! -s "$SENT" ]
    [ ! -e "$SECLOG" ]
}

@test "pre_connect: Keychain'deki diziyi sırayla, doğru protokol ve gecikmeyle gönderir" {
    knock_on; printf 'KNOCK_PROTO=udp\nKNOCK_DELAY_MS=0\n' >> "$D/server.env"
    printf '7000,8000,9000\n' > "$SEQ_FILE"
    run knock_pre_connect WEB1 "$D" 192.0.2.7
    [ "$status" -eq 0 ]
    [ "$(cat "$SENT")" = $'192.0.2.7 7000 udp\n192.0.2.7 8000 udp\n192.0.2.7 9000 udp' ]
    grep -q -- 'find-generic-password -a USBMonitor_WEB1_Knock -s USBMonitor -w' "$SECLOG"
}

@test "pre_connect: varsayılan tcp/300 ms; vuruşlar arasında bekler" {
    knock_on; printf '7000,8000\n' > "$SEQ_FILE"
    printf '#!/bin/sh\necho "$@" >> "%s/sleep.log"\n' "$T" > "$T/bin/sleep"; chmod +x "$T/bin/sleep"
    run knock_pre_connect WEB1 "$D" 192.0.2.7
    [ "$status" -eq 0 ]
    [ "$(cat "$SENT")" = $'192.0.2.7 7000 tcp\n192.0.2.7 8000 tcp' ]
    [ "$(cat "$T/sleep.log")" = $'0.300\n0.300' ]
}

@test "pre_connect: Keychain'de dizi yoksa / geçersizse / ayar bozuksa HATA, hiçbir şey gönderilmez" {
    knock_on
    run knock_pre_connect WEB1 "$D" 192.0.2.7                 # dizi yok
    [ "$status" -ne 0 ]; [[ "$output" == *"Keychain'de yok"* ]]
    printf '7000,abc\n' > "$SEQ_FILE"
    run knock_pre_connect WEB1 "$D" 192.0.2.7; [ "$status" -ne 0 ]
    printf '7000\n' > "$SEQ_FILE"
    printf 'KNOCK_PROTO=icmp\n' >> "$D/server.env"
    run knock_pre_connect WEB1 "$D" 192.0.2.7; [ "$status" -ne 0 ]
    sed -i.x '/KNOCK_PROTO/d' "$D/server.env"; printf 'KNOCK_DELAY_MS=99999\n' >> "$D/server.env"
    run knock_pre_connect WEB1 "$D" 192.0.2.7; [ "$status" -ne 0 ]
    [ ! -s "$SENT" ]
}

@test "dizi server.env'de (düz metin) tutulmaz; yalnızca KNOCK=keychain yazılır" {
    knock_on; printf '7000,8000,9000\n' > "$SEQ_FILE"
    [ -z "$(grep -E '7000|8000|9000' "$D/server.env")" ]
}

@test "gönderim (gerçek): UDP vuruşu yerel dinleyiciye ulaşır" {
    unset -f _knock_send_one; source "$H/Knock.sh"
    python3 -I - "$T" <<'PY' &
import socket, sys, os
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.bind(("127.0.0.1", 0)); s.settimeout(8)
open(os.path.join(sys.argv[1], "udp.port"), "w").write(str(s.getsockname()[1]))
try:
    data, _ = s.recvfrom(16)
    open(os.path.join(sys.argv[1], "udp.got"), "w").write(data.decode())
except Exception:
    pass
PY
    for i in $(seq 1 50); do [ -s "$T/udp.port" ] && break; sleep 0.1; done
    knock_send 127.0.0.1 "$(cat "$T/udp.port")" udp 0
    for i in $(seq 1 50); do [ -s "$T/udp.got" ] && break; sleep 0.1; done
    [ "$(cat "$T/udp.got")" = "x" ]
}

@test "gönderim (gerçek): TCP vuruşu yerel dinleyiciye bağlantı olarak ulaşır" {
    unset -f _knock_send_one; source "$H/Knock.sh"
    python3 -I - "$T" <<'PY' &
import socket, sys, os
s = socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", 0)); s.listen(1); s.settimeout(8)
open(os.path.join(sys.argv[1], "tcp.port"), "w").write(str(s.getsockname()[1]))
try:
    c, _ = s.accept(); open(os.path.join(sys.argv[1], "tcp.got"), "w").write("baglandi")
except Exception:
    pass
PY
    for i in $(seq 1 50); do [ -s "$T/tcp.port" ] && break; sleep 0.1; done
    knock_send 127.0.0.1 "$(cat "$T/tcp.port")" tcp 0
    for i in $(seq 1 50); do [ -s "$T/tcp.got" ] && break; sleep 0.1; done
    [ "$(cat "$T/tcp.got")" = "baglandi" ]
}

@test "gönderim (gerçek): kapalı/cevap vermeyen porta takılmaz (en çok ~1 sn/vuruş)" {
    unset -f _knock_send_one; source "$H/Knock.sh"
    start=$SECONDS
    knock_send 192.0.2.1 "9,9" tcp 0          # TEST-NET: cevap gelmez
    [ $((SECONDS - start)) -le 6 ]
}

@test "durum: knock etkin ve port kapalıysa 🔒; etkin değilse 🔴; açıksa 🟢 (knock göndermez)" {
    printf '#!/bin/sh\nexit 1\n' > "$T/bin/nc"; chmod +x "$T/bin/nc"
    run ssh_module_status WEB1 "$D"
    [ "$output" = "🔴" ]
    knock_on
    run ssh_module_status WEB1 "$D"
    [ "$output" = "🔒" ]
    [ ! -s "$SENT" ]
    printf '#!/bin/sh\nexit 0\n' > "$T/bin/nc"
    run ssh_module_status WEB1 "$D"
    [ "$output" = "🟢" ]
}

@test "ayar menüsü: dizi Keychain'e yazılır, KNOCK/PROTO/DELAY server.env'e düşer" {
    printf '7000,8000,9000\n' > "$T/typed"
    run knock_manage WEB1 "$D" <<< $'1\nudp\n150'
    [ "$status" -eq 0 ]
    grep -qx 'KNOCK=keychain' "$D/server.env"
    grep -qx 'KNOCK_PROTO=udp' "$D/server.env"
    grep -qx 'KNOCK_DELAY_MS=150' "$D/server.env"
    grep -q 'add-generic-password -U -a USBMonitor_WEB1_Knock -s USBMonitor -w$' "$SECLOG"
    [ -z "$(grep -E '7000|8000|9000' "$D/server.env")" ]
}

@test "ayar menüsü: geçersiz dizi hemen silinir ve ayar etkinleşmez" {
    printf '7000;8000\n' > "$T/typed"
    run knock_manage WEB1 "$D" <<< $'1\n\n'
    [ "$status" -ne 0 ]
    [ ! -f "$SEQ_FILE" ]
    run grep -q '^KNOCK=keychain' "$D/server.env"
    [ "$status" -ne 0 ]
}

@test "ayar menüsü: geçersiz protokol/süre NEDENİYLE açıklanır ve yeniden sorulur; 3 hatada vazgeçilir" {
    run knock_manage WEB1 "$D" <<< $'1\nicmp\nicmp\nicmp'
    [ "$status" -ne 0 ]
    [[ "$output" == *"'icmp' geçerli değil: yalnızca tcp veya udp"* ]]
    [[ "$output" == *"3 geçersiz deneme"* ]]
    run knock_manage WEB1 "$D" <<< $'1\ntcp\n99999\nabc\nx'
    [ "$status" -ne 0 ]
    [[ "$output" == *"0 ile 5000 arasında"* ]]
    [ ! -f "$SEQ_FILE" ]
    knock_on; printf '7000\n' > "$SEQ_FILE"
    run knock_manage WEB1 "$D" <<< "2"
    [ "$status" -eq 0 ]
    [ ! -f "$SEQ_FILE" ]
    grep -qx 'KNOCK=' "$D/server.env"
}

@test "modül menüsü: Bağlan'dan ÖNCE knock gönderilir; knock hatasında bağlanılmaz" {
    knock_on; printf '7000,8000\n' > "$SEQ_FILE"; printf 'KNOCK_DELAY_MS=0\n' >> "$D/server.env"
    touch "$D/id_ed25519_web1"
    ORDER="$T/order"; : > "$ORDER"
    _knock_send_one() { echo "knock $2" >> "$ORDER"; }
    _ssh_module_connect() { echo "connect" >> "$ORDER"; }
    ssh_agent_add_retry() { return 0; }
    clear() { :; }
    run ssh_module_run WEB1 "$D" <<< "1"
    [ "$(cat "$ORDER")" = $'knock 7000\nknock 8000\nconnect' ]
    # dizi Keychain'den silinirse: hata, bağlantı denenmez
    : > "$ORDER"; rm -f "$SEQ_FILE"
    run ssh_module_run WEB1 "$D" <<< $'1\n\n0'
    [ -z "$(grep connect "$ORDER")" ]
}

@test "ayar menüsü: yanlış girişten sonra doğru değer kabul edilir; büyük harf ve Enter varsayılanı çalışır" {
    printf '7000,8000\n' > "$T/typed"
    run knock_manage WEB1 "$D" <<< $'1\nhttp\nUDP\n\n'
    [ "$status" -eq 0 ]
    [[ "$output" == *"'http' geçerli değil"* ]]
    grep -qx 'KNOCK_PROTO=udp' "$D/server.env"
    grep -qx 'KNOCK_DELAY_MS=300' "$D/server.env"
    [[ "$output" == *"örnek: 7000,8000,9000"* ]]       # dizi için açıklama gösterildi
}

@test "ayar menüsü: knocking'in ne olduğu ve iki kez sorulabileceği açıklanır" {
    run knock_manage WEB1 "$D" <<< "0"
    [[ "$output" == *"önceden kararlaştırılmış"* ]]
    printf '7000\n' > "$T/typed"
    run knock_manage WEB1 "$D" <<< $'1\n\n\n'
    [[ "$output" == *"İKİ KEZ"* ]]
}

@test "ayar menüsü (ok tuşlu seçici): numara + Enter ile 'Kapat' seçilir, dizi silinir" {
    export SSHPOCKET_UI=arrows TERM=dumb
    knock_on; printf '7000\n' > "$SEQ_FILE"
    run knock_manage WEB1 "$D" < <(printf '2\n')
    [ "$status" -eq 0 ]
    [ ! -f "$SEQ_FILE" ]
    grep -qx 'KNOCK=' "$D/server.env"
}

@test "ayar menüsü (ok tuşlu seçici): Geri hiçbir şey değiştirmez" {
    export SSHPOCKET_UI=arrows TERM=dumb
    knock_on; printf '7000\n' > "$SEQ_FILE"
    run knock_manage WEB1 "$D" < <(printf '0\n')
    [ "$status" -eq 0 ]
    [ -f "$SEQ_FILE" ]
    grep -qx 'KNOCK=keychain' "$D/server.env"
}

@test "ayar menüsü (fzf standardı): başlık kenarlıkta, açıklama ve durum başlık kutusunda, iki kart; üstte düz metin yok" {
    export SSHPOCKET_UI=fzf TERM=dumb
    unset NO_COLOR
    ui_fzf_enabled() { return 0; }
    clear() { :; }
    ui_pick_arrows() {
        { echo "S=$UI_SCREEN_TITLE"; echo "W=$UI_PICK_WIDE"; echo "H=${UI_PICK_HEADER//$'\n'/|}"; echo "N=${#UI_PICK_DESCS[@]}"; } > "$T/pick"
        printf 0
    }
    run knock_manage WEB1 "$D"
    [ "$status" -eq 0 ]
    [[ "$output" != *"Port knocking:"* ]] || { echo "$output"; false; }      # düz metin basılmaz (başlık kutusunda)
    grep -q "^S=Ana menü › web1 › Knock ayarları" "$T/pick"
    grep -q "^W=1" "$T/pick"
    grep -q "^H=Port knocking:.*Durum: ❌ kapalı" "$T/pick"
    grep -qx "N=2" "$T/pick"
}
