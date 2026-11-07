#!/usr/bin/env bats
# Aşama 1.2: eski düzenden (sistem/Modules) yeni düzene (servers/) taşıma.
# Fixture, gerçek bir USB'deki düzenin aynısıdır (gizli bilgi yok, key'ler test için üretilir).
load test_helper

setup() {
    make_sandbox
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/diskutil"
    printf '#!/bin/bash\nexit 1\n' > "$SANDBOX/bin/security"
    chmod +x "$SANDBOX/bin/"*
    export PATH="$SANDBOX/bin:$PATH" TERM=xterm
    MOD="$SANDBOX/sistem/Modules"
    SRV="$SANDBOX/servers"
    ( cd "$SANDBOX" && source sistem/Helpers/Validate.sh && source sistem/Helpers/ModuleFiles.sh
      module_write_files "$MOD" ubuntutest 192.0.2.10 syhgorath 22 >/dev/null
      module_write_files "$MOD" testubuntu2 192.0.2.11 admin 2222 >/dev/null )
    for n in ubuntutest testubuntu2; do
        ssh-keygen -q -t ed25519 -N "" -f "$MOD/$n/id_ed25519_$n"
        printf 'finder' > "$MOD/$n/.DS_Store"
    done
    printf 'x' > "$MOD/.DS_Store"
    # eski sürümlerin gönderdiği, hiç yapılandırılmamış 'example' şablonu (.env YOK)
    mkdir -p "$MOD/example"
    printf '# şablon\n' > "$MOD/example/ExampleModule.sh"
    printf 'EXAMPLE_IP=\n' > "$MOD/example/.env.example"
}
migrate() { bash "$SANDBOX/sistem/Main.sh" --migrate "$@" 2>&1; }
snapshot() { ( cd "$1" && find . -type f -not -name '.DS_Store' | LC_ALL=C sort | while read -r f; do printf '%s %s\n' "$(shasum -a 256 "$f" | cut -d' ' -f1)" "$f"; done ); }

@test "deneme modu: plan gösterir, hiçbir şeyi değiştirmez, key içeriği basmaz" {
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/once"
    run migrate --dry-run
    [ "$status" -eq 0 ]
    [[ "$output" == *"ubuntutest"* ]]
    [[ "$output" == *"syhgorath@192.0.2.10:22"* ]]
    [[ "$output" == *"testubuntu2"* ]]
    [[ "$output" == *"admin@192.0.2.11:2222"* ]]
    [[ "$output" == *"deneme modu"* ]]
    [[ "$output" != *"BEGIN OPENSSH"* ]]
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/sonra"
    cmp "$BATS_TEST_TMPDIR/once" "$BATS_TEST_TMPDIR/sonra"
    [ ! -d "$SRV" ]
}
@test "taşıma: ayarlar genel anahtarlara çevrilir, değerler aynıdır" {
    run migrate --yes
    [ "$status" -eq 0 ]
    grep -qx 'IP=192.0.2.10'   "$SRV/ubuntutest/server.env"
    grep -qx 'USER=syhgorath'  "$SRV/ubuntutest/server.env"
    grep -qx 'PORT=22'         "$SRV/ubuntutest/server.env"
    grep -qx 'PORT=2222'       "$SRV/testubuntu2/server.env"
    run ! grep -q 'UBUNTUTEST_' "$SRV/ubuntutest/server.env"
}
@test "taşıma: key dosyaları bayt bayt aynı, izinler 600, servers/ 700" {
    cp "$MOD/ubuntutest/id_ed25519_ubuntutest" "$BATS_TEST_TMPDIR/orijinal_key"
    cp "$MOD/ubuntutest/id_ed25519_ubuntutest.pub" "$BATS_TEST_TMPDIR/orijinal_pub"
    run migrate --yes
    cmp "$SRV/ubuntutest/id_ed25519_ubuntutest" "$BATS_TEST_TMPDIR/orijinal_key"
    cmp "$SRV/ubuntutest/id_ed25519_ubuntutest.pub" "$BATS_TEST_TMPDIR/orijinal_pub"
    [ "$(ls -l "$SRV/ubuntutest/id_ed25519_ubuntutest" | cut -c1-10)" = "-rw-------" ]
    [ "$(ls -l "$SRV/ubuntutest/server.env" | cut -c1-10)" = "-rw-------" ]
    [ "$(ls -ld "$SRV" | cut -c1-10)" = "drwx------" ]
}
@test "taşıma: üretilmiş modül dosyası, .env.example ve .DS_Store taşınmaz" {
    run migrate --yes
    [ ! -e "$SRV/ubuntutest/UbuntutestModule.sh" ]
    [ ! -e "$SRV/ubuntutest/.env.example" ]
    [ ! -e "$SRV/ubuntutest/.DS_Store" ]
    [ ! -e "$SRV/ubuntutest/.env" ]
}
@test "taşıma: eski klasörler SİLİNMEZ, servers/.migrated-<tarih>/ altına alınır; example yerinde" {
    run migrate --yes
    [ ! -e "$MOD/ubuntutest" ]
    [ -d "$MOD/example" ]
    bak=$(ls -d "$SRV"/.migrated-* | head -n 1)
    [ -f "$bak/ubuntutest/.env" ]
    [ -f "$bak/ubuntutest/id_ed25519_ubuntutest" ]
    [ -f "$bak/testubuntu2/.env" ]
}
@test "yapılandırılmamış 'example' şablonu servers/'a TAŞINMAZ; .env'i olan 'example' sunucusu taşınır" {
    run migrate --yes
    [ -d "$MOD/example" ]                       # --migrate şablona dokunmaz (içinde .env yok, taşınacak sunucu da değil)
    [ ! -e "$SRV/example" ]
    # kullanıcı onu gerçekten yapılandırdıysa taşınmalı
    printf 'EXAMPLE_IP=192.0.2.50\nEXAMPLE_USER=me\nEXAMPLE_PORT=22\n' > "$MOD/example/.env"
    run migrate --yes
    [ "$status" -eq 0 ]
    grep -qx 'IP=192.0.2.50' "$SRV/example/server.env"
    [ ! -e "$MOD/example" ]
}
@test "taşıma sonrası menü yeni biçimden kurulur ve Keychain hesabı aynı kalır" {
    run migrate --yes
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [[ "$output" == *"UBUNTUTEST"* ]]
    [[ "$output" == *"TESTUBUNTU2"* ]]
    # hesap adı: USBMonitor_<AD_BÜYÜK>_Passphrase (değişmedi)
    source "$SANDBOX/sistem/Helpers/Validate.sh"; source "$SANDBOX/sistem/Helpers/ModuleFiles.sh"
    source "$SANDBOX/sistem/Helpers/SSHHelper.sh"
    run ssh_keychain_account "$(module_prefix ubuntutest)"
    [ "$output" = "USBMonitor_UBUNTUTEST_Passphrase" ]
}
@test "idempotent: ikinci çalıştırmada yapacak bir şey yok, hiçbir şey değişmez" {
    migrate --yes >/dev/null
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/once"
    run migrate --yes
    [ "$status" -eq 0 ]
    [[ "$output" == *"Taşınacak eski biçimli sunucu yok"* ]]
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/sonra"
    cmp "$BATS_TEST_TMPDIR/once" "$BATS_TEST_TMPDIR/sonra"
}
@test "çakışma: servers/'da aynı ad varsa o sunucu atlanır, eski klasör olduğu gibi kalır" {
    mkdir -p "$SRV/ubuntutest"; printf 'IP=203.0.113.99\n' > "$SRV/ubuntutest/server.env"
    run migrate --yes
    [[ "$output" == *"ubuntutest: servers/ içinde zaten var"* ]]
    [ -f "$MOD/ubuntutest/.env" ]
    grep -qx 'IP=203.0.113.99' "$SRV/ubuntutest/server.env"
    [ -d "$SRV/testubuntu2" ]            # diğeri taşındı
}
@test "HATA: key kopyalanamazsa o sunucu taşınmaz, eski klasör ve diğer sunucu etkilenmez, yarım klasör kalmaz" {
    chmod 000 "$MOD/ubuntutest/id_ed25519_ubuntutest"
    run migrate --yes
    chmod 600 "$MOD/ubuntutest/id_ed25519_ubuntutest"
    [ "$status" -ne 0 ]
    [[ "$output" == *"ubuntutest"*"kopyalanamadı"* ]]
    [ ! -e "$SRV/ubuntutest" ]
    [ ! -e "$SRV/.migrating-ubuntutest" ]
    [ -f "$MOD/ubuntutest/.env" ]
    [ -d "$SRV/testubuntu2" ]            # bağımsız sunucu yine taşındı
    [ -f "$MOD/ubuntutest/id_ed25519_ubuntutest" ]
}
@test "bilinmeyen anahtarlar, yorumlar ve özel KEY yolu korunur" {
    printf '# açıklama\nUBUNTUTEST_IP=192.0.2.10\nUBUNTUTEST_USER=syhgorath\nUBUNTUTEST_PORT=22\nUBUNTUTEST_KEY=/yol/ozel_key\nUBUNTUTEST_NOT=merhaba\nBASKA=1\n' > "$MOD/ubuntutest/.env"
    run migrate --yes
    grep -qx '# açıklama' "$SRV/ubuntutest/server.env"
    grep -qx 'KEY=/yol/ozel_key' "$SRV/ubuntutest/server.env"
    grep -qx 'NOT=merhaba' "$SRV/ubuntutest/server.env"
    grep -qx 'BASKA=1' "$SRV/ubuntutest/server.env"
}
@test "etkileşimsiz (pipe) normal açılış taşıma TEKLİF ETMEZ ve hiçbir şeyi taşımaz" {
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [[ "$output" != *"Taşınsın mı"* ]]
    [ -d "$MOD/ubuntutest" ]
    [ ! -d "$SRV" ]
}
@test "onay istenirse hayır: hiçbir şey değişmez" {
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/once"
    run bash -c "printf 'n\n' | bash '$SANDBOX/sistem/Main.sh' --migrate 2>&1"
    [ "$status" -ne 0 ]
    [[ "$output" == *"İptal edildi"* ]]
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/sonra"
    cmp "$BATS_TEST_TMPDIR/once" "$BATS_TEST_TMPDIR/sonra"
}
@test "güvenlik: eski .env değeri komut olarak çalışmaz, ad doğrulanmayan klasör taşınmaz" {
    printf 'UBUNTUTEST_IP=$(touch %s/pwned)\nUBUNTUTEST_USER=u\nUBUNTUTEST_PORT=22\n' "$BATS_TEST_TMPDIR" > "$MOD/ubuntutest/.env"
    mkdir -p "$MOD/Buyuk_Ad"; printf 'BUYUK_AD_IP=1.1.1.1\n' > "$MOD/Buyuk_Ad/.env"
    run migrate --yes
    [ ! -e "$BATS_TEST_TMPDIR/pwned" ]
    [ ! -e "$SRV/Buyuk_Ad" ]
    [ -d "$MOD/Buyuk_Ad" ]
}
@test "HATA: kopya bozulursa (doğrulama) taşıma durur, eski klasör ve key olduğu gibi kalır" {
    # cp'yi, kopyaladığı dosyanın sonuna bir bayt ekleyen sahte bir komutla değiştir (PATH başında)
    printf '#!/bin/bash\n/bin/cp "$@"\nlast="${@: -1}"\n[ -f "$last" ] && printf x >> "$last"\nexit 0\n' > "$SANDBOX/bin/cp"
    chmod +x "$SANDBOX/bin/cp"
    /bin/cp -p "$MOD/ubuntutest/id_ed25519_ubuntutest" "$BATS_TEST_TMPDIR/orijinal"
    run migrate --yes
    [ "$status" -ne 0 ]
    [[ "$output" == *"doğrulama başarısız"* ]]
    [ ! -e "$SRV/ubuntutest" ]
    [ ! -e "$SRV/testubuntu2" ]
    [ ! -e "$SRV/.migrating-ubuntutest" ]
    cmp "$MOD/ubuntutest/id_ed25519_ubuntutest" "$BATS_TEST_TMPDIR/orijinal"
    [ -f "$MOD/ubuntutest/.env" ]
}
@test "taşıma: gevşek izinli (644) private key, yeni yerde 600 olur; .pub 644 kalabilir" {
    chmod 644 "$MOD/ubuntutest/id_ed25519_ubuntutest"
    run migrate --yes
    [ "$status" -eq 0 ]
    [ "$(ls -l "$SRV/ubuntutest/id_ed25519_ubuntutest" | cut -c1-10)" = "-rw-------" ]
}
@test "HATA: dönüştürücü yanlış değer üretirse doğrulama yakalar, hiçbir şey taşınmaz" {
    run bash -c '
        export SISTEM_PATH="$1/sistem" SSHPOCKET_SERVERS_DIR="$1/servers"
        for f in Validate ModuleFiles SSHModule SSHModuleEdit RegisterModules Servers Migrate; do source "$SISTEM_PATH/Helpers/$f.sh"; done
        # bilerek bozuk dönüştürücü: IP değerini değiştirir
        migrate_translate_env() { printf "IP=9.9.9.9\nUSER=x\nPORT=22\n"; }
        migrate_one ubuntutest; echo "rc=$?"' _ "$SANDBOX"
    [[ "$output" == *"doğrulama başarısız (IP eşleşmiyor)"* ]]
    [[ "$output" == *"rc=1"* ]]
    [ ! -e "$SRV/ubuntutest" ]
    [ ! -e "$SRV/.migrating-ubuntutest" ]
    [ -f "$MOD/ubuntutest/.env" ]
}
@test "açılışta: yapılandırılmamış example şablonu temizlenir, menüde EXAMPLE görünmez" {
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1 | sed 's/\x1b\[[0-9;]*[A-Za-z]//g'"
    [ ! -e "$MOD/example" ]
    [[ "$output" != *"EXAMPLE"* ]]
    [ -d "$MOD/ubuntutest" ]                    # gerçek sunucular dokunulmadı
}
@test "açılışta: .env'i olan example (kullanıcı yapılandırmış) temizlenmez" {
    printf 'EXAMPLE_IP=192.0.2.50\nEXAMPLE_USER=me\nEXAMPLE_PORT=22\n' > "$MOD/example/.env"
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [ -f "$MOD/example/.env" ]
}
@test "açılışta: example içinde key veya kullanıcı dosyası varsa temizlenmez" {
    printf 'KEY' > "$MOD/example/id_ed25519_example"
    run bash -c "printf '0\n' | bash '$SANDBOX/sistem/Main.sh' 2>&1"
    [ -f "$MOD/example/id_ed25519_example" ]
}
@test "--migrate --dry-run şablona bile dokunmaz" {
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/once"
    run migrate --dry-run
    snapshot "$SANDBOX" > "$BATS_TEST_TMPDIR/sonra"
    cmp "$BATS_TEST_TMPDIR/once" "$BATS_TEST_TMPDIR/sonra"
    [ -d "$MOD/example" ]
}
@test "taşıma bittikten sonra Modules'ta hiçbir şey kalmadıysa boş sistem/Modules klasörü kaldırılır" {
    rm -r "$MOD/example"; rm -f "$MOD/.DS_Store"       # yalnızca taşınacak iki sunucu kalsın
    run migrate --yes
    [ "$status" -eq 0 ]
    [ ! -d "$MOD" ]
    [ -d "$SRV/ubuntutest" ]
}
@test "taşıma: Modules içinde taşınmayan bir şey varsa klasör silinmez" {
    mkdir -p "$MOD/benim_klasorum"; printf 'x' > "$MOD/benim_klasorum/not.txt"
    run migrate --yes
    [ -f "$MOD/benim_klasorum/not.txt" ]
}

