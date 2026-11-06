#!/usr/bin/env bats
# Paket SÖZLEŞME uyum denetimi (component_validate.py): geçerli sahte paket + her kuralın bozulması (statik; ağ yok)
load test_helper

setup() {
    export SISTEM_PATH="$ROOT/sistem"
    source "$H/Components.sh"
    source "$H/ComponentsInstall.sh"
    source "$H/ComponentsCLI.sh"
    T="$BATS_TEST_TMPDIR"
    VAL="$H/component_validate.py"
    P="$T/demo-1.0.0"
    mkpkg "$P"
}

# geçerli, en küçük ansible paketi
mkpkg() {
    local d="$1"
    mkdir -p "$d/playbooks"
    cat > "$d/component.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible","inventory_group":"demo",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook playbooks/p.yml --check","effects":[],"requires_sudo":true},
  "apply":{"cmd":"ansible-playbook playbooks/p.yml","effects":["changes-config"],"requires_sudo":true,"requires":["preview"],"confirm":"yes-no",
           "warnings":["Önizleme servis durumunu göstermez."]},
  "status":{"cmd":"ansible-playbook playbooks/s.yml","effects":[],
            "outputs":{"format":"keyvalue-line","keys":["ok"]}}},
 "vars":[{"name":"demo_net","type":"string","required":true,"description":"ağ"}]}
JSON
    printf '1.0.0\n' > "$d/VERSION"
    printf 'ansible-core==2.21.5 \\\n    --hash=sha256:%064d\n' 1 > "$d/requirements.txt"
    printf '[defaults]\n' > "$d/ansible.cfg"
    printf -- '- hosts: all\n  tasks:\n    - debug: msg="{{ demo_net }}"\n' > "$d/playbooks/p.yml"
    printf -- '- hosts: all\n  tasks:\n    - debug: msg="SSHPOCKET_RESULT demo.status ok=yes"\n' > "$d/playbooks/s.yml"
    printf '# demo\n\nGeri alma: apply sonrası değişiklik elle geri alınabilir (rollback notu).\n' > "$d/README.md"
    printf '## [1.0.0]\n- ilk\n' > "$d/CHANGELOG.md"
}

# jedit <json> <python ifadesi (d üzerinde)>
jedit() { python3 - "$1" "$2" <<'PY'
import json,sys
p,expr=sys.argv[1],sys.argv[2]
d=json.load(open(p)); exec(expr); json.dump(d,open(p,'w'),ensure_ascii=False)
PY
}

val() { run python3 -I "$VAL" "$@"; }

# ---------- geçerli ----------
@test "geçerli paket: ❌ yok, uyarı yok, çıkış 0, her bölüm ✅" {
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Sonuç: ✅ uyumlu"* ]] || false
    [[ "$output" != *"❌"* ]] || false
    [[ "$output" != *"⚠️"* ]] || false
    [[ "$output" == *"VERSION = component.json sürümü (1.0.0)"* ]] || false
    [[ "$output" == *"requirements.txt: 1 bağımlılık, hepsi sabit sürümlü ve hash'li"* ]] || false
}

@test "gerçek paket (kuruluysa): ❌ yok" {
    local d="$ROOT/components/linux-hardening/0.2.0"
    [ -d "$d" ] || skip "linux-hardening kurulu değil"
    val "$d"
    [ "$status" -eq 0 ]
    [[ "$output" != *"❌"* ]] || false
}

# ---------- ❌ hatalar ----------
@test "bildirim (component.json) hataları ❌ olarak yüzeye çıkar" {
    jedit "$P/component.json" 'd["actions"]["apply"]["effects"]=["yok-boyle"]'
    val "$P"
    [ "$status" -eq 1 ]
    [[ "$output" == *"❌"*"effects"* ]] || false
}

@test "VERSION yoksa ya da component.json ile uyuşmuyorsa ❌" {
    printf '2.0.0\n' > "$P/VERSION"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"VERSION '2.0.0' ≠ component.json sürümü '1.0.0'"* ]] || false
    rm "$P/VERSION"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"VERSION dosyası yok"* ]] || false
}

@test "desteklenen (supported) profil yoksa ❌" {
    jedit "$P/component.json" 'd["profiles"]=[{"id":"rpi","status":"planned"}]'
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"en az bir 'supported' profil"* ]] || false
}

@test "requirements.txt: yok, hash'siz, sabit sürümsüz, URL/VCS, -e/-r → ❌" {
    rm "$P/requirements.txt"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"requirements.txt yok"* ]] || false
    printf 'ansible-core==2.21.5\n' > "$P/requirements.txt"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"--hash=sha256 yok"* ]] || false
    printf 'ansible-core>=2 \\\n --hash=sha256:%064d\n' 1 > "$P/requirements.txt"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"sabit sürüm (==) yok"* ]] || false
    printf 'evil @ https://example.com/e.whl==1 \\\n --hash=sha256:%064d\n' 1 > "$P/requirements.txt"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"URL/VCS bağımlılığı yasak"* ]] || false
    printf -- '-r other.txt\nansible-core==1 \\\n --hash=sha256:%064d\n' 1 > "$P/requirements.txt"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"-e/-r yönergeleri yasak"* ]] || false
}

@test "eylem komutu: playbook dosyası yok, ansible-playbook değil, yol paket dışına çıkıyor → ❌" {
    rm "$P/playbooks/p.yml"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"playbook yok: playbooks/p.yml"* ]] || false
    printf -- '- hosts: all\n' > "$P/playbooks/p.yml"
    jedit "$P/component.json" 'd["actions"]["preview"]["cmd"]="ansible-playbook ../disari.yml"'
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"paket içinde ve göreli olmalı"* ]] || false
    jedit "$P/component.json" 'd["actions"]["preview"]["cmd"]="bash kotu.sh"'
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"'ansible-playbook' ile başlamalı"* ]] || false
}

@test "güvenlik: özel anahtar bloğu, .env/id_* dosyası ve sembolik bağ → ❌" {
    # işaret satırı DOSYADA düz metin olmasın (depo hijyeni izlenen dosyalarda bu kalıbı arar): çalışma anında birleştirilir
    printf -- '-----BEGIN %s PRIVATE KEY-----\nxx\n' OPENSSH > "$P/playbooks/notlar.yml"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"özel anahtar bloğu bulundu: playbooks/notlar.yml"* ]] || false
    rm "$P/playbooks/notlar.yml"
    : > "$P/.env"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"gizli bilgi dosyası paket içinde: .env"* ]] || false
    rm "$P/.env"; : > "$P/id_ed25519_x"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"id_ed25519_x"* ]] || false
    rm "$P/id_ed25519_x"; ln -s /etc/passwd "$P/baglanti"
    val "$P"; [ "$status" -eq 1 ]; [[ "$output" == *"sembolik bağ yasak"* ]] || false
}

# ---------- ⚠️ uyarılar (çıkış 0) ----------
@test "uyarı: değiştiren eylemde preview ön koşulu / warning / requires_sudo yok" {
    jedit "$P/component.json" 'd["actions"]["apply"].pop("requires"); d["actions"]["apply"].pop("warnings"); d["actions"]["apply"].pop("requires_sudo")'
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" == *"apply: salt okunur bir 'preview' ön koşulu yok"* ]] || false
    [[ "$output" == *"apply: hiç warning yok"* ]] || false
    [[ "$output" == *"apply: requires_sudo belirtilmemiş"* ]] || false
    [[ "$output" == *"Sonuç: ✅ uyumlu (3 uyarı"* ]] || false
}

@test "uyarı yok: kurtarma eylemi (rollback_*) ve yalnızca araç kuran eylem preview ön koşulu istemez; yapılandırma değiştiren hâlâ ister" {
    jedit "$P/component.json" 'a=d["actions"]; a["rollback_now"]=dict(a["apply"]); a["rollback_now"].pop("requires",None); a["rollback_now"]["effects"]=["changes-config"]; a["scan"]=dict(a["apply"]); a["scan"].pop("requires",None); a["scan"]["effects"]=["installs-package"]; a["kaldir_geri_yukle"]=dict(a["apply"]); a["kaldir_geri_yukle"].pop("requires",None); a["kaldir_geri_yukle"]["effects"]=["changes-config"]; a["apply"].pop("requires")'
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" != *"rollback_now: salt okunur bir 'preview'"* ]] || false
    [[ "$output" != *"scan: salt okunur bir 'preview'"* ]] || false
    [[ "$output" != *"kaldir_geri_yukle: salt okunur bir 'preview'"* ]] || false      # Türkçe kurtarma adı (geri_al/kaldir_geri_yukle) da kurtarma sayılır
    [[ "$output" == *"apply: salt okunur bir 'preview' ön koşulu yok"* ]] || false
}

@test "uyarı: parola/token gibi değişken adı, kullanılmayan değişken, eski biçim (yalnızca ad)" {
    jedit "$P/component.json" 'd["vars"]=d["vars"]+[{"name":"demo_api_token","type":"string"}, {"name":"demo_kullanilmaz","type":"string"}, "demo_eski_bicim"]'
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" == *"demo_api_token: adı parola/anahtar/token izlenimi veriyor"* ]] || false
    [[ "$output" == *"demo_kullanilmaz: playbook/rol/vars/betik içinde kullanılmıyor"* ]] || false
    [[ "$output" == *"demo_eski_bicim: eski biçim"* ]] || false
}

@test "gizli-ad uyarısı: adında password/token geçen bool ve int değişkenler (gün sayısı gibi) UYARILMAZ; string olan uyarılır" {
    jedit "$P/component.json" 'd["vars"]=d["vars"]+[{"name":"demo_password_inactive_days","type":"int"}, {"name":"demo_token_enabled","type":"bool"}, {"name":"demo_password_file","type":"string"}]'
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" != *"demo_password_inactive_days: adı parola"* ]] || false
    [[ "$output" != *"demo_token_enabled: adı parola"* ]] || false
    [[ "$output" == *"demo_password_file: adı parola/anahtar/token izlenimi veriyor"* ]] || false
}

@test "parola ÖZETİ adları (_hash/_digest/_checksum) uyarılmaz; düz parola/token adları ve '_hash' ile bitmeyenler uyarılmaya devam eder" {
    jedit "$P/component.json" 'd["vars"]=d["vars"]+[{"name":"demo_password_hash","type":"string"}, {"name":"demo_api_token_digest","type":"string"}, {"name":"demo_password","type":"string"}, {"name":"demo_hash_password","type":"string"}, {"name":"demo_secret_hashed","type":"string"}]'
    printf -- '- hosts: all\n  vars: {a: "{{ demo_password_hash }} {{ demo_api_token_digest }}"}\n  tasks: []\n' > "$P/playbooks/h.yml"
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" != *"demo_password_hash: adı parola"* ]] || false
    [[ "$output" != *"demo_api_token_digest: adı parola"* ]] || false
    [[ "$output" == *"demo_password_hash (string, parola ÖZETİ: düz parola olamaz)"* ]] || false
    [[ "$output" == *"demo_password: adı parola/anahtar/token izlenimi veriyor"* ]] || false
    [[ "$output" == *"demo_hash_password: adı parola/anahtar/token izlenimi veriyor"* ]] || false
    [[ "$output" == *"demo_secret_hashed: adı parola/anahtar/token izlenimi veriyor"* ]] || false
}

@test "uyarı: outputs.format var ama playbook'ta SSHPOCKET_RESULT çıktısı yok" {
    printf -- '- hosts: all\n  tasks: []\n' > "$P/playbooks/s.yml"
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" == *"status: outputs.format var ama playbook/rol/betiklerde 'SSHPOCKET_RESULT demo.status' çıktısı bulunamadı"* ]] || false
}

@test "uyarı: değiştiren eylem var ama belgede geri alma/kurtarma anlatılmamış" {
    printf '# demo\n\nBu paket birşey yapar.\n' > "$P/README.md"
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" == *"geri alma/kurtarma yolu anlatılmamış"* ]] || false
}

@test "--strict: uyarılar hata sayılır (çıkış 1); uyarı yoksa 0" {
    printf '# demo\n' > "$P/README.md"
    val "$P" --strict
    [ "$status" -eq 1 ]
    [[ "$output" == *"--strict"* ]] || false
    printf '# demo\n\nGeri alma anlatıldı.\n' > "$P/README.md"
    val "$P" --strict
    [ "$status" -eq 0 ]
}

# ---------- arşiv girdisi ----------
mkarchive() { # <arşiv-yolu> <kök-adı> [python-ek-kodu]
    python3 - "$1" "$2" "$P" "${3:-}" <<'PY'
import os,sys,tarfile,io
out,root,src,extra=sys.argv[1:5]
with tarfile.open(out,"w:gz") as tf:
    for d,dirs,files in os.walk(src):
        for f in files:
            full=os.path.join(d,f); tf.add(full, arcname=os.path.join(root, os.path.relpath(full,src)))
    exec(extra)
PY
}

@test "arşiv: doğru adlı ve kökü <id>-<sürüm>/ olan paket ✅; .sig yoksa uyarı, varsa ✅" {
    mkarchive "$T/demo-1.0.0.tar.gz" "demo-1.0.0"
    val "$T/demo-1.0.0.tar.gz"
    [ "$status" -eq 0 ]
    [[ "$output" == *"arşiv kökü 'demo-1.0.0/'"* ]] || false
    [[ "$output" == *"demo-1.0.0.tar.gz.sig yok"* ]] || false
    : > "$T/demo-1.0.0.tar.gz.sig"
    val "$T/demo-1.0.0.tar.gz"
    [[ "$output" == *"imza dosyası yanında"* ]] || false
}

@test "dizin modu: gitignore'lu geliştirme klasörü (.venv) atlanır ve bildirilir; ARŞİV modu atlamaz (içindeki özel anahtar ❌)" {
    mkdir -p "$P/.venv/lib"
    printf '%s %s\n' "-----BEGIN OPENSSH" "PRIVATE KEY-----" > "$P/.venv/lib/x.py"
    ln -s /etc/passwd "$P/.venv/lib/bag"
    val "$P"
    [ "$status" -eq 0 ]
    [[ "$output" == *"geliştirme klasörleri atlandı (.venv)"* ]] || false
    [[ "$output" != *"❌"* ]] || false
    rm "$P/.venv/lib/bag"      # sembolik bağlı arşiv zaten açılışta reddedilir; burada içerik taramasını sınıyoruz
    mkarchive "$T/demo-1.0.0.tar.gz" "demo-1.0.0"
    val "$T/demo-1.0.0.tar.gz"
    [ "$status" -eq 1 ]
    [[ "$output" == *"özel anahtar bloğu bulundu: .venv/lib/x.py"* ]] || false
    [[ "$output" != *"atlandı"* ]] || false
    # .venv dışındaki bir klasörde aynı şey dizin modunda da yakalanır
    rm -rf "$P/.venv"; mkdir -p "$P/roles/x"
    printf '%s %s\n' "-----BEGIN OPENSSH" "PRIVATE KEY-----" > "$P/roles/x/k.yml"
    val "$P"
    [ "$status" -eq 1 ]
    [[ "$output" == *"özel anahtar bloğu bulundu: roles/x/k.yml"* ]] || false
}

@test "arşiv: --unsigned imzalamadan önceki denetimde .sig uyarısını atlar (--strict ile birlikte de 0)" {
    mkarchive "$T/demo-1.0.0.tar.gz" "demo-1.0.0"
    val "$T/demo-1.0.0.tar.gz" --strict
    [ "$status" -eq 1 ]
    val "$T/demo-1.0.0.tar.gz" --unsigned --strict
    [ "$status" -eq 0 ]
    [[ "$output" == *"imza henüz beklenmiyor (--unsigned)"* ]] || false
    [[ "$output" != *".sig yok"* ]] || false
    val "$T/demo-1.0.0.tar.gz" --bilinmeyen
    [ "$status" -eq 2 ]
}

@test "arşiv: yanlış ad, yanlış kök klasör, sürüm uyuşmazlığı → ❌" {
    mkarchive "$T/rastgele.tar.gz" "demo-1.0.0"
    val "$T/rastgele.tar.gz"; [ "$status" -eq 1 ]; [[ "$output" == *"arşiv adı '<id>-<sürüm>.tar.gz' olmalı"* ]] || false
    mkarchive "$T/demo-1.0.0.tar.gz" "baska-kok"
    val "$T/demo-1.0.0.tar.gz"; [ "$status" -eq 1 ]; [[ "$output" == *"arşiv kökü tek klasör 'demo-1.0.0/' olmalı"* ]] || false
    mkarchive "$T/demo-2.0.0.tar.gz" "demo-2.0.0"
    val "$T/demo-2.0.0.tar.gz"; [ "$status" -eq 1 ]; [[ "$output" == *"2.0.0"* ]] || false
}

@test "arşiv GÜVENLİĞİ: '..', mutlak yol, sembolik bağ ve aygıt dosyası reddedilir; hiçbir şey dışarı yazılmaz" {
    local extra
    for extra in \
        'ti=tarfile.TarInfo("demo-1.0.0/../kacak.txt"); ti.size=1; tf.addfile(ti, io.BytesIO(b"x"))' \
        'ti=tarfile.TarInfo("/tmp/mutlak-yol-test.txt"); ti.size=1; tf.addfile(ti, io.BytesIO(b"x"))' \
        'ti=tarfile.TarInfo("demo-1.0.0/bag"); ti.type=tarfile.SYMTYPE; ti.linkname="/etc/passwd"; tf.addfile(ti)' \
        'ti=tarfile.TarInfo("demo-1.0.0/aygit"); ti.type=tarfile.CHRTYPE; tf.addfile(ti)'; do
        mkarchive "$T/demo-1.0.0.tar.gz" "demo-1.0.0" "$extra"
        val "$T/demo-1.0.0.tar.gz"
        [ "$status" -eq 1 ] || { echo "KABUL EDİLDİ: $extra"; return 1; }
        [[ "$output" == *"arşiv güvenli açılamadı"* ]] || { echo "NEDEN YOK: $extra | $output"; return 1; }
    done
    [ ! -e "$T/kacak.txt" ]
    [ ! -e /tmp/mutlak-yol-test.txt ]
}

@test "arşiv: geçici klasör her durumda silinir" {
    mkdir -p "$T/tmp"; mkarchive "$T/demo-1.0.0.tar.gz" "demo-1.0.0"
    TMPDIR="$T/tmp" val "$T/demo-1.0.0.tar.gz"
    [ "$(ls "$T/tmp" | grep -c sshpocket-validate)" = "0" ]
    mkarchive "$T/demo-1.0.0.tar.gz" "demo-1.0.0" 'ti=tarfile.TarInfo("demo-1.0.0/bag"); ti.type=tarfile.SYMTYPE; ti.linkname="/x"; tf.addfile(ti)'
    TMPDIR="$T/tmp" val "$T/demo-1.0.0.tar.gz"
    [ "$(ls "$T/tmp" | grep -c sshpocket-validate)" = "0" ]
}

# ---------- CLI ----------
@test "CLI: dizin ve --strict geçirilir; kurulu paket kimliğiyle de çalışır; hatalı kullanım 2" {
    run comp_cli_validate "$P"
    [ "$status" -eq 0 ]
    export SSHPOCKET_COMPONENTS_DIR="$T/comps"
    mkdir -p "$T/comps/demo"; cp -R "$P" "$T/comps/demo/1.0.0"; printf '1.0.0\n' > "$T/comps/demo/current"
    run comp_cli_validate demo
    [ "$status" -eq 0 ]
    [[ "$output" == *"Sonuç: ✅ uyumlu"* ]] || false
    run comp_cli_validate;                 [ "$status" -eq 2 ]
    run comp_cli_validate yok-boyle-paket; [ "$status" -eq 2 ]
    run comp_cli_validate --strict;        [ "$status" -eq 2 ]
    run components_cli validate "$P" --strict
    [ "$status" -eq 0 ]
}

@test "dosya olarak çalıştırılan denetleyici: yanlış kullanım 2, olmayan yol ❌" {
    run python3 -I "$VAL"
    [ "$status" -eq 2 ]
    run python3 -I "$VAL" "$T/yok"
    [ "$status" -eq 1 ]
    [[ "$output" == *"bulunamadı"* ]] || false
}

@test "script paketi: betik varsa ❌ yok; betik yoksa, sembolik bağsa ya da çok büyükse ❌" {
    S="$T/scr-1.0.0"; mkdir -p "$S/scripts"
    cat > "$S/component.json" <<'JSON'
{"id":"scr","version":"1.0.0","sshpocket_api":1,"runner":"script",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{"saglik":{"cmd":"scripts/saglik.sh","effects":[]}}}
JSON
    printf '1.0.0\n' > "$S/VERSION"; printf '# scr\n' > "$S/README.md"; printf '## [1.0.0]\n- ilk\n' > "$S/CHANGELOG.md"
    printf '#!/usr/bin/env bash\necho ok\n' > "$S/scripts/saglik.sh"
    run python3 -I "$VAL" "$S"
    [[ "$output" != *"❌"* ]] || { echo "$output"; false; }
    rm "$S/scripts/saglik.sh"
    run python3 -I "$VAL" "$S"; [[ "$output" == *"betik yok: scripts/saglik.sh"* ]] || false
    ln -s /etc/hostname "$S/scripts/saglik.sh"
    run python3 -I "$VAL" "$S"; [[ "$output" == *"sembolik bağ"* ]] || false
    rm "$S/scripts/saglik.sh"; head -c 300000 /dev/zero | tr '\0' 'a' > "$S/scripts/saglik.sh"
    run python3 -I "$VAL" "$S"; [[ "$output" == *"256 KB"* ]] || false
}

@test "script paketi: sonuç satırı ve SSHPOCKET_VAR_<AD> kullanımı betiklerde aranır (yanlış alarm yok); eksikse ⚠️" {
    S="$T/scr2-1.0.0"; mkdir -p "$S/scripts"
    cat > "$S/component.json" <<'JSON'
{"id":"scr","version":"1.0.0","sshpocket_api":1,"runner":"script",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{"saglik":{"cmd":"scripts/saglik.sh","effects":[],"outputs":{"format":"keyvalue-line","keys":["disk"]}}},
 "vars":[{"name":"esik","type":"int","default":80,"description":"eşik"}]}
JSON
    printf '1.0.0\n' > "$S/VERSION"; printf '# scr\n' > "$S/README.md"; printf '## [1.0.0]\n- ilk\n' > "$S/CHANGELOG.md"
    printf '#!/usr/bin/env bash\necho "SSHPOCKET_RESULT scr.saglik disk=${SSHPOCKET_VAR_ESIK}"\n' > "$S/scripts/saglik.sh"
    run python3 -I "$VAL" "$S"
    [[ "$output" != *"⚠️"* && "$output" != *"❌"* ]] || { echo "$output"; false; }
    printf '#!/usr/bin/env bash\necho ok\n' > "$S/scripts/saglik.sh"
    run python3 -I "$VAL" "$S"
    [[ "$output" == *"çıktısı bulunamadı"* && "$output" == *"esik: playbook/rol/vars/betik içinde kullanılmıyor"* ]] || false
}
