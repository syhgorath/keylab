#!/usr/bin/env bats
# USB çıkarılırken USB'deki anahtarlar ssh-agent'tan çıkarılır; ajandaki diğer anahtarlara dokunulmaz. Sahte ssh-add/diskutil, gerçek ajan yok.
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/SSHHelper.sh"
    source "$H/Eject.sh"
    Log() { :; }
    T="$BATS_TEST_TMPDIR"
    USB="$T/usb"; mkdir -p "$USB/servers/web1" "$USB/servers/db1" "$T/bin"
    printf 'ssh-ed25519 AAAA web1\n' > "$USB/servers/web1/id_ed25519_web1.pub"; : > "$USB/servers/web1/id_ed25519_web1"
    printf 'ssh-ed25519 BBBB db1\n'  > "$USB/servers/db1/id_ed25519_db1.pub";  : > "$USB/servers/db1/id_ed25519_db1"
    printf 'IP=1.2.3.4\n' > "$USB/servers/web1/server.env"
    export SSHADD_LOG="$T/sshadd.log" ORDER_LOG="$T/order.log"; : > "$SSHADD_LOG"; : > "$ORDER_LOG"
    # sahte ssh-add: -d çağrılarını kaydeder; FAKE_SSHADD_FAIL=1 ise "ajanda yok" gibi hata verir
    cat > "$T/bin/ssh-add" <<'STUB'
#!/bin/bash
echo "$*" >> "$SSHADD_LOG"; echo "ssh-add" >> "$ORDER_LOG"
[ "${FAKE_SSHADD_FAIL:-0}" = 1 ] && exit 1
exit 0
STUB
    cat > "$T/bin/diskutil" <<'STUB'
#!/bin/bash
echo "diskutil $1" >> "$ORDER_LOG"
case "$1" in info) echo "   Device Identifier:        disk9s1" ;; esac
exit 0
STUB
    chmod +x "$T/bin/ssh-add" "$T/bin/diskutil"; export PATH="$T/bin:$PATH"
}

@test "ssh_agent_remove_usb_keys: USB'deki her .pub için ssh-add -d çağrılır, yalnızca .pub (parola sorulmaz); sayı döner" {
    run ssh_agent_remove_usb_keys "$USB"
    [ "$status" -eq 0 ]; [ "$output" = "2" ]
    grep -qxF -- "-d $USB/servers/web1/id_ed25519_web1.pub" "$SSHADD_LOG"
    grep -qxF -- "-d $USB/servers/db1/id_ed25519_db1.pub" "$SSHADD_LOG"
    [ "$(wc -l < "$SSHADD_LOG" | tr -d ' ')" = "2" ]
    run grep -q "id_ed25519_web1$" "$SSHADD_LOG"; [ "$status" -ne 0 ]
}

@test "ssh_agent_remove_usb_keys: ajan anahtarı zaten yoksa (ssh-add hata) sessiz geçer, sayı 0" {
    FAKE_SSHADD_FAIL=1 run ssh_agent_remove_usb_keys "$USB"
    [ "$status" -eq 0 ]; [ "$output" = "0" ]
}

@test "ssh_agent_remove_usb_keys: servers/ yoksa ya da yol boşsa hiçbir şey yapmaz" {
    run ssh_agent_remove_usb_keys "$T/yok"; [ "$output" = "0" ]
    run ssh_agent_remove_usb_keys "";       [ "$output" = "0" ]
    [ ! -s "$SSHADD_LOG" ]
}

@test "EjectUSB: ajan temizliği diskutil'den ÖNCE çalışır ve çıkarılan sayıyı yazar" {
    run EjectUSB "$USB"
    [ "$status" -eq 0 ]
    [[ "$output" == *"2 anahtar ssh-agent'tan çıkarıldı"* ]] || false
    # sıra: önce ssh-add (iki kez), sonra diskutil
    first_disk=$(grep -n "^diskutil" "$ORDER_LOG" | head -1 | cut -d: -f1)
    last_add=$(grep -n "^ssh-add" "$ORDER_LOG" | tail -1 | cut -d: -f1)
    [ "$last_add" -lt "$first_disk" ]
}

@test "EjectUSB: ajan hiçbir anahtar çıkarmadıysa ek mesaj yazmaz ve yine çıkarır" {
    FAKE_SSHADD_FAIL=1 run EjectUSB "$USB"
    [ "$status" -eq 0 ]
    [[ "$output" != *"ssh-agent'tan çıkarıldı"* ]] || false
    [[ "$output" == *"USB çıkarıldı"* ]] || false
}
