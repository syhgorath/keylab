#!/usr/bin/env bats
# servers/ + menü: uçtan uca (sahte komutlarla, gerçek ağ yok).
load test_helper

setup() {
    make_sandbox
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/diskutil"
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/security"
    printf '#!/bin/bash\nexit 255\n' > "$SANDBOX/bin/ssh"
    chmod +x "$SANDBOX/bin/"*
    export PATH="$SANDBOX/bin:$PATH" TERM=xterm
    SRV="$SANDBOX/servers"
    mkdir -p "$SRV/alpha"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=2222\n' > "$SRV/alpha/server.env"
    ssh-keygen -q -t ed25519 -N "" -f "$SRV/alpha/id_ed25519_alpha"
    # eski biçimli bir sunucu da var
    mkdir -p "$SANDBOX/sistem/Modules/legacy1"
    printf 'LEGACY1_IP=198.51.100.4\nLEGACY1_USER=pi\nLEGACY1_PORT=22\n' > "$SANDBOX/sistem/Modules/legacy1/.env"
    ssh-keygen -q -t ed25519 -N "" -f "$SANDBOX/sistem/Modules/legacy1/id_ed25519_legacy1"
    ( cd "$SANDBOX" && source sistem/Helpers/Validate.sh && source sistem/Helpers/ModuleFiles.sh \
        && module_write_files "$SANDBOX/sistem/Modules" legacy1 198.51.100.4 pi 22 >/dev/null )
}
run_main() { bash -c "printf '$1' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"; }

@test "menü: yeni biçimli ve eski biçimli sunucular birlikte listelenir" {
    run run_main '0\n'
    [[ "$output" == *"ALPHA"* ]]
    [[ "$output" == *"LEGACY1"* ]]
}
@test "yeni biçimli sunucu: modül menüsü bilgileri server.env'den gösterir" {
    run run_main '1\n0\n\n0\n'
    [[ "$output" == *"192.0.2.7"* ]]
    [[ "$output" == *"deploy"* ]]
    [[ "$output" == *"2222"* ]]
    [[ "$output" == *"servers/alpha/id_ed25519_alpha"* ]]
    [[ "$output" != *"Ayar dosyası bulunamadı"* ]]
}
@test "eski biçimli sunucu hâlâ çalışır" {
    # sıra: 1) ALPHA (servers/), 2) LEGACY1 (eski biçim)
    run run_main '2\n0\n\n0\n'
    [[ "$output" == *"198.51.100.4"* ]]
}
@test "güncelleme yeni biçimin anahtar adlarıyla yazılır (ALPHA_IP değil IP)" {
    # ana menü: 1) ALPHA; modül menüsü: 3) güncelle -> ip, kullanıcı, port, key, Enter; 0 geri; Enter; 0 çık
    run run_main '1\n3\n10.0.0.9\nroot2\n\n\n\n0\n\n0\n'
    [[ "$output" == *"Güncellendi: root2@10.0.0.9:2222"* ]]
    grep -qx 'IP=10.0.0.9' "$SRV/alpha/server.env"
    grep -qx 'USER=root2' "$SRV/alpha/server.env"
    run ! grep -q 'ALPHA_' "$SRV/alpha/server.env"
    [ ! -e "$SRV/alpha/.env" ]
}
@test "aynı ada sahip eski modül varsa menüde yalnızca bir girdi görünür" {
    mkdir -p "$SANDBOX/sistem/Modules/alpha"
    printf 'ALPHA_IP=203.0.113.200\nALPHA_USER=eski\nALPHA_PORT=22\n' > "$SANDBOX/sistem/Modules/alpha/.env"
    ( cd "$SANDBOX" && source sistem/Helpers/Validate.sh && source sistem/Helpers/ModuleFiles.sh \
        && module_write_files "$SANDBOX/sistem/Modules" alpha 203.0.113.200 eski 22 >/dev/null )
    run run_main '0\n'
    [ "$(printf '%s\n' "$output" | grep -c '^ *[0-9]*) .*ALPHA')" -eq 1 ]
    # yeni biçim kazanır: bilgiler servers/'dan
    run run_main '1\n0\n\n0\n'
    [[ "$output" == *"192.0.2.7"* ]]
    [[ "$output" != *"203.0.113.200"* ]]
}
@test "GÜVENLİK: servers/ altındaki betikler ve ._ dosyaları asla çalıştırılmaz" {
    printf 'echo ZARARLI_CALISTI\n' > "$SRV/alpha/evil.sh"
    printf 'echo ZARARLI_CALISTI\n' > "$SRV/alpha/._AlphaModule.sh"
    printf 'echo ZARARLI_CALISTI\n' > "$SRV/zz.sh"
    mkdir -p "$SRV/.hidden"; printf 'IP=1.1.1.1\n' > "$SRV/.hidden/server.env"
    run run_main '1\n0\n\n0\n'
    [[ "$output" != *"ZARARLI_CALISTI"* ]]
    [[ "$output" != *".hidden"* ]]
}
@test "GÜVENLİK: server.env içindeki değer komut olarak çalışmaz" {
    printf 'IP=$(touch %s/pwned)\nUSER=deploy\nPORT=22\n' "$BATS_TEST_TMPDIR" > "$SRV/alpha/server.env"
    run run_main '1\n0\n\n0\n'
    [ ! -e "$BATS_TEST_TMPDIR/pwned" ]
    [[ "$output" == *"geçersiz IP/host"* ]]
}
@test "durum göstergesi server: girdileri için de çalışır" {
    printf 'IP=127.0.0.1\nUSER=deploy\nPORT=1\n' > "$SRV/alpha/server.env"
    run bash -c "printf '0\n' | SSHPOCKET_STATUS=1 bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"🔴 ALPHA"* ]]
}
@test "--auto: ilk sunucuyu (server:) çalıştırır, yerleşik eylemi değil" {
    run bash -c "printf '\n' | bash '$SANDBOX/sistem/Main.sh' --auto 2>&1"
    [[ "$output" == *"Auto-executing: server:alpha"* ]]
}

# Not: 'read -p' istemi girdi terminal değilse yazdırılmaz; bu yüzden duraklama, bir sonraki satırı
# tüketmesiyle gözlenir: "3x" ana menüde geçersiz seçim olur (Invalid selection), duraklamada ise yutulur.
@test "geri dönünce duraklama yok: sunucu menüsü, Ayarlar ve Sunucuları yönet doğrudan ana menüye döner" {
    run run_main '1\n0\n3x\n0\n'            # sunucu menüsü: geri → ana menü (3x geçersiz seçim) → çıkış
    [[ "$output" == *"Invalid selection"* ]]
    run run_main '7\n0\n3x\n0\n'            # ⚙️ Ayarlar
    [[ "$output" == *"Invalid selection"* ]]
    run run_main '5\n0\n3x\n0\n'            # Sunucuları yönet
    [[ "$output" == *"Invalid selection"* ]]
}

@test "bir iş yapıldıysa (Sunucuları yönet → Sil) duraklama korunur: sonraki satır yutulur" {
    run run_main '5\n1\n\n3x\n0\n'         # Yönet → Sil → boş ad → duraklama "3x"'i yutar → çıkış
    [[ "$output" != *"Invalid selection"* ]]
}
