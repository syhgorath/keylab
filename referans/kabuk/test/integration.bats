#!/usr/bin/env bats
load test_helper
setup() {
    make_sandbox
    # diskutil/ssh/ssh-copy-id sahte: gerçek bir şeye dokunulmaz
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/diskutil"
    printf '#!/bin/bash\n[ "${STUB_AUTHORIZED:-0}" = 1 ] && exit 0\nexit 255\n' > "$SANDBOX/bin/ssh"
    printf '#!/bin/bash\necho "[ssh-copy-id] $*"\n[ "${STUB_COPY_FAIL:-0}" = 1 ] && exit 1\nexit 0\n' > "$SANDBOX/bin/ssh-copy-id"
    # Gerçek Keychain'e dokunulmasın
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/security"
    chmod +x "$SANDBOX/bin/"*
    export PATH="$SANDBOX/bin:$PATH" TERM=xterm
    # Gönderilen 'example' modülü artık yok: eski biçimli bir sunucu fixture'ı olarak üretilir
    ( cd "$SANDBOX" && source sistem/Helpers/Validate.sh && source sistem/Helpers/ModuleFiles.sh \
        && module_write_files "$SANDBOX/sistem/Modules" example 192.0.2.10 pi 2222 >/dev/null )
    MOD="$SANDBOX/sistem/Modules/example"
    printf 'EXAMPLE_IP=192.0.2.10\nEXAMPLE_USER=pi\nEXAMPLE_PORT=2222\n' > "$MOD/.env"
    ssh-keygen -q -t ed25519 -N "" -f "$MOD/id_ed25519_example"
}
strip() { sed 's/\x1b\[[0-9;]*[A-Za-z]//g'; }

@test "ana menü örnek modülü listeler ve 0 ile çıkar" {
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"1) EXAMPLE"* ]]
    [[ "$output" == *"Exiting"* ]]
}
@test "geçersiz kullanıcı .env'de reddedilir" {
    printf 'EXAMPLE_IP=192.0.2.10\nEXAMPLE_USER=pi;id\nEXAMPLE_PORT=22\n' > "$MOD/.env"
    run bash -c "printf '1\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"geçersiz kullanıcı"* ]]
}
@test "key eksikse net hata" {
    rm "$MOD/id_ed25519_example"
    run bash -c "printf '1\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"SSH key bulunamadı"* ]]
}
@test "özel EXAMPLE_KEY yolu kullanılır" {
    mv "$MOD/id_ed25519_example" "$BATS_TEST_TMPDIR/ozel_key"
    printf 'EXAMPLE_KEY=%s\n' "$BATS_TEST_TMPDIR/ozel_key" >> "$MOD/.env"
    run bash -c "printf '1\n2\nn\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" != *"SSH key bulunamadı"* ]]
    [[ "$output" == *"Hedef:       pi@192.0.2.10:2222"* ]]
    [[ "$output" == *"ozel_key"* ]]
}
@test "key gönderme: yeni key ssh-copy-id'ye gider" {
    run bash -c "printf '1\n2\ny\n\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"[ssh-copy-id] -i"*"id_ed25519_example.pub -p 2222 pi@192.0.2.10"* ]]
    [[ "$output" == *"ELLE yapın"* ]]
}
@test "key gönderme: zaten yetkiliyse ssh-copy-id çağrılmaz" {
    STUB_AUTHORIZED=1 run bash -c "printf '1\n2\ny\n\n0\n' | STUB_AUTHORIZED=1 bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"zaten yetkili"* ]]
    [[ "$output" != *"[ssh-copy-id]"* ]]
}
@test "key gönderme: iptal edilirse ssh-copy-id çağrılmaz" {
    run bash -c "printf '1\n2\nn\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" != *"[ssh-copy-id]"* ]]
}
@test "key gönderme: olmayan giriş key'i dosyası reddedilir" {
    run bash -c "printf '1\n2\ny\n/yok/key\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"Dosya yok"* ]]
    [[ "$output" != *"[ssh-copy-id]"* ]]
}

@test "ana menü yerleşik eylemleri sunucuların ardından listeler" {
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"1) EXAMPLE"* ]]
    [[ "$output" == *"2) ➕ Yeni sunucu ekle"* ]]
    [[ "$output" == *"3) 📥 ~/.ssh/config'ten içe aktar"* ]]
    [[ "$output" == *"4) 🛠️"* ]]
}
@test "hiç sunucu yoksa menü yine de ekleme seçeneklerini gösterir" {
    rm -r "$SANDBOX/sistem/Modules/example"
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"1) ➕ Yeni sunucu ekle"* ]]
    [[ "$output" != *"Modül bulunamadı"* ]]
}
@test "menüden yönet > listele çalışır ve geri döner" {
    run bash -c "printf '4\n0\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"Sunucuları yönet"* ]]
    [[ "$output" == *"example"* ]]
}
@test "menüden ekle: yeni modül yeniden başlatmadan menüde görünür" {
    # sahte ssh-keygen: key üretimini atlayıp 'n' diyoruz; generator'a girdi: ad, ip, kullanıcı, port, key=n
    run bash -c "printf '2\nnewsrv\n192.0.2.7\nroot\n22\nn\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [ -f "$SANDBOX/servers/newsrv/server.env" ]
    [ ! -e "$SANDBOX/sistem/Modules/newsrv" ]
    grep -qx 'IP=192.0.2.7' "$SANDBOX/servers/newsrv/server.env"
    [[ "$output" == *"NEWSRV"* ]]
}
@test "--auto yerleşik eylemi değil ilk sunucuyu çalıştırır" {
    run bash -c "printf '\n' | bash '$SANDBOX/sistem/Main.sh' --auto 2>&1"
    [[ "$output" == *"Auto-executing: ExampleModule"* ]]
}

@test "bozuk ayar: menüden 3 ile düzeltilir ve menü yenilenir" {
    printf 'EXAMPLE_IP=192.0.2.10\nEXAMPLE_USER=pi;id\nEXAMPLE_PORT=2222\n' > "$MOD/.env"
    run bash -c "printf '1\n3\n\nroot\n\n\n\n0\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"Güncellendi: root@192.0.2.10:2222"* ]]
    grep -q '^EXAMPLE_USER=root$' "$MOD/.env"
}
@test "bozuk ayar: ayar düzelmeden Bağlan/Key gönder reddedilir" {
    printf 'EXAMPLE_IP=192.0.2.10\nEXAMPLE_USER=pi;id\nEXAMPLE_PORT=22\n' > "$MOD/.env"
    run bash -c "printf '1\n1\n\n2\n\n0\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"Önce sorunu giderin"* ]]
    [[ "$output" != *"[ssh-copy-id]"* ]]
}
@test "key eksikken menü açılır ve key yolu 3 ile düzeltilebilir" {
    rm "$MOD/id_ed25519_example"
    run bash -c "printf '1\n0\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"SSH key bulunamadı"* ]]
    [[ "$output" == *"3) Bilgileri güncelle"* ]]
    [[ "$output" == *"4) Keychain passphrase"* ]]
}

@test "ssh-copy-id başarısız olursa nedenleri ve elle ekleme komutu gösterilir" {
    run bash -c "printf '1\n2\ny\n\n\n0\n\n0\n' | STUB_COPY_FAIL=1 bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"ssh-copy-id başarısız oldu"* ]]
    [[ "$output" == *"Elle ekleme"* ]]
    [[ "$output" == *"~/.ssh/authorized_keys"* ]]
    [[ "$output" == *"$(cut -d' ' -f1,2 "$MOD/id_ed25519_example.pub")"* ]]
}
@test "başarılı gönderimde elle ekleme talimatı gösterilmez" {
    run bash -c "printf '1\n2\ny\n\n\n0\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" != *"Elle ekleme"* ]]
}
@test "kullanıcı root ise Ubuntu uyarısı gösterilir, değilse gösterilmez" {
    printf 'EXAMPLE_IP=192.0.2.10\nEXAMPLE_USER=root\nEXAMPLE_PORT=22\n' > "$MOD/.env"
    run bash -c "printf '1\n2\ny\n\n\n0\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" == *"Kullanıcı 'root'"* ]]
    printf 'EXAMPLE_IP=192.0.2.10\nEXAMPLE_USER=pi\nEXAMPLE_PORT=22\n' > "$MOD/.env"
    run bash -c "printf '1\n2\ny\n\n\n0\n\n0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" != *"Kullanıcı 'root'"* ]]
}

@test "Autoload macOS gizli/AppleDouble dosyalarını (._X.sh, .DS_Store, .Spotlight-V100) yüklemez" {
    # exFAT USB'lerde macOS ._Dosya.sh üretir; yüklenseydi kod çalıştırırdı
    printf 'echo ZARARLI_YUKLENDI >&2\n' > "$SANDBOX/sistem/Modules/example/._ExampleModule.sh"
    printf 'echo ZARARLI_YUKLENDI >&2\n' > "$SANDBOX/sistem/Modules/._Gizli.sh"
    printf 'x' > "$SANDBOX/sistem/Modules/.DS_Store"
    mkdir -p "$SANDBOX/sistem/Modules/.Spotlight-V100"
    printf 'echo ZARARLI_YUKLENDI >&2\n' > "$SANDBOX/sistem/Modules/.Spotlight-V100/x.sh"
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" != *"ZARARLI_YUKLENDI"* ]]
    [[ "$output" != *"Loading module: "*"/._"* ]]
    [[ "$output" == *"1) EXAMPLE"* ]]
}
@test "ModuleManager list: gizli klasörleri saymaz, yapılandırılmış sunucuları gösterir" {
    mkdir -p "$SANDBOX/sistem/Modules/.Spotlight-V100" "$SANDBOX/sistem/Modules/.Trashes"
    # yeni biçimli bir sunucu + eski biçimli bir sunucu
    mkdir -p "$SANDBOX/servers/alpha"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=2222\n' > "$SANDBOX/servers/alpha/server.env"
    mkdir -p "$SANDBOX/sistem/Modules/old1"
    printf 'OLD1_IP=198.51.100.4\nOLD1_USER=pi\nOLD1_PORT=22\n' > "$SANDBOX/sistem/Modules/old1/.env"
    run bash -c "SSHPOCKET_MODULES_DIR='$SANDBOX/sistem/Modules' bash '$SANDBOX/sistem/ModuleManager.sh' list"
    echo "$output" | grep -q 'alpha'
    echo "$output" | grep -q 'old1'
    echo "$output" | grep -q 'example'     # yapılandırılmış (eski biçimli) fixture sunucusu
    [ -z "$(printf '%s\n' "$output" | grep 'Spotlight')" ]
    [ -z "$(printf '%s\n' "$output" | grep 'Trashes')" ]
}
