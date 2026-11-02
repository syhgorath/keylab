#!/usr/bin/env bats
# Aşama 3, PR E1: sunucu menüsünde "🛡️ Paketler" (sahte paket/çalıştırıcı; ağ yok)
load test_helper

setup() {
    load_helpers
    export SISTEM_PATH="$ROOT/sistem"
    for f in UI Servers Components ComponentsInstall ComponentsManifest ComponentsRun ComponentsGates ComponentsExec ComponentsVars ComponentsMenu; do source "$H/$f.sh"; done
    T="$BATS_TEST_TMPDIR"
    export SSHPOCKET_COMPONENTS_DIR="$T/comps" SSHPOCKET_SERVERS_DIR="$T/srv"
    D="$T/srv/web1"; mkdir -p "$D" "$T/comps/demo/1.0.0"
    printf 'IP=192.0.2.7\nUSER=deploy\nPORT=22\n' > "$D/server.env"
    touch "$D/id_ed25519_web1"
    cat > "$T/m.json" <<'JSON'
{"id":"demo","version":"1.0.0","sshpocket_api":1,"runner":"ansible",
 "profiles":[{"id":"ubuntu-26.04","status":"supported"}],
 "actions":{
  "preview":{"cmd":"ansible-playbook a.yml --check","effects":[],"warnings":["Önizleme ufw değişikliğini göstermez."]},
  "apply":{"cmd":"ansible-playbook a.yml","effects":["changes-config"],"requires":["preview"],"confirm":"type-server-name"},
  "wipe":{"cmd":"ansible-playbook w.yml","effects":["destructive"]},
  "audit":{"cmd":"ansible-playbook u.yml","effects":["installs-package"],"confirm":"yes-no","outputs":{"dir_var":"scan_dir"}}},
 "vars":[
  {"name":"allow_from","type":"list[cidr]","required":true,"description":"SSH'a izin verilen ağlar"},
  {"name":"scan_dir","type":"path","description":"Çıktı klasörü"},
  {"name":"rb_enabled","type":"bool","default":true,"warn_if":false,"description":"geri alma güvenlik ağı"},
  {"name":"rb_timeout","type":"int","default":300,"min":30,"max":3600,"description":"geri alma süresi"},
  {"name":"mode","type":"enum","values":["a","b"],"default":"a"}]}
JSON
    cp "$T/m.json" "$T/comps/demo/1.0.0/component.json"
    printf 'allow_from=10.99.0.0/24\n' > "$D/demo.vars"      # varsayılan: zorunlu değişken tamam (eylemler açık)
    INFO=$(comp_manifest_info "$T/m.json" demo 1.0.0)
    # paket katmanı sahte (kendi testleri var)
    comp_ids() { echo demo; }
    comp_installed_version() { [ "$1" = demo ] && printf '1.0.0'; }
    comp_prepare() { [ "$1" = demo ] && printf '%s\n' "$INFO"; }
    comp_cli_run() { echo "$*" >> "$T/runs"; return "${FAKE_RUN_RC:-0}"; }
    clear() { :; }
}

# ---------- erişilebilirlik ----------
@test "available: yeni biçimli sunucu + kurulu paket varsa evet; eski biçim ya da paket yoksa hayır" {
    run comp_menu_available "$D"; [ "$status" -eq 0 ]
    L="$T/legacy"; mkdir -p "$L"; printf 'WEB1_IP=1.1.1.1\n' > "$L/.env"
    run comp_menu_available "$L"; [ "$status" -ne 0 ]                     # eski biçim: servers/ dışı
    comp_installed_version() { return 0; }                                 # kurulu sürüm yok
    run comp_menu_available "$D"; [ "$status" -ne 0 ]
    comp_ids() { :; }
    run comp_menu_available "$D"; [ "$status" -ne 0 ]
}

# ---------- denetim puanı satırı ----------
@test "başlık: pakete ait denetim özeti varsa 'Hazırlık' bölümünde puan ve fark gösterilir; yoksa satır çıkmaz" {
    export SSHPOCKET_STATE_DIR="$T/state"
    run comp_menu_actions demo web1 <<<"0"
    [[ "$output" != *"📊"* ]] || false
    mkdir -p "$T/state/demo/web1/out/web1/a" "$T/state/demo/web1/out/web1/b"
    printf '{"date":"2026-01-01T00:00:00Z","hardening_index":61}' > "$T/state/demo/web1/out/web1/a/summary.json"
    printf '{"date":"2026-01-02T00:00:00Z","hardening_index":83}' > "$T/state/demo/web1/out/web1/b/summary.json"
    run comp_menu_actions demo web1 <<<"0"
    [[ "$output" == *"📊 Lynis: 83 ("*") · önceki 61 (+22)"* ]] || false
}

# ---------- eylem listesi ----------
@test "eylemler: etki rozeti, ön koşul ve ilk uyarı gösterilir; geri hiçbir şey çalıştırmaz" {
    run comp_menu_actions demo web1 <<<"0"
    [ "$status" -eq 0 ]
    [[ "$output" == *"preview — 🔍 salt okunur"* ]] || false
    [[ "$output" == *"apply — ✎ değişiklik yapar  (önce: preview)"* ]] || false
    [[ "$output" == *"wipe — ☠ yıkıcı"* ]] || false
    [ ! -e "$T/runs" ]
}

@test "eylemler: seçilen eylem doğru paket/eylem/sunucu ile çalıştırılır; sonra liste döner" {
    setup_state
    comp_gate_record demo web1 1.0.0 preview ok                       # taze preview: apply doğrudan çalışır
    run comp_menu_actions demo web1 <<<$'2\n\n3\n\n0'
    [ "$status" -eq 0 ]
    [ "$(sed -n 1p "$T/runs")" = "demo preview web1" ]
    [ "$(sed -n 2p "$T/runs")" = "demo apply web1" ]
    [ "$(wc -l < "$T/runs" | tr -d ' ')" = "2" ]
}

@test "eylemler: geçersiz/aralık dışı giriş bir şey çalıştırmaz; q ve boş girdi geri döner" {
    for bad in 9 abc -1 "1;id"; do
        rm -f "$T/runs"
        run comp_menu_actions demo web1 <<<"$bad"
        [ "$status" -eq 0 ]
        [ ! -e "$T/runs" ] || { echo "ÇALIŞTI: $bad"; return 1; }
    done
    run comp_menu_actions demo web1 <<<"q";  [ ! -e "$T/runs" ]
    run comp_menu_actions demo web1 </dev/null; [ ! -e "$T/runs" ]
}

@test "eylemler: eylem başarısız olsa da menü döngüsü sürer (Enter sonrası listeye döner)" {
    FAKE_RUN_RC=2 run comp_menu_actions demo web1 <<<$'2\n\n0'
    [ "$status" -eq 0 ]
    [ "$(wc -l < "$T/runs" | tr -d ' ')" = "1" ]
}

@test "eylemler: paket doğrulanamazsa (comp_prepare başarısız) çalıştırmadan çıkar" {
    comp_prepare() { return 1; }
    run comp_menu_actions demo web1 <<<"1"
    [ "$status" -ne 0 ]
    [ ! -e "$T/runs" ]
}

# ---------- paket seçimi ----------
@test "sunucu girişi: tek paket varsa doğrudan eylem listesine gider; kurulu paket yoksa bilgi verir" {
    run comp_menu_server WEB1 "$D" <<<"0"
    [ "$status" -eq 0 ]
    [[ "$output" == *"preview — 🔍 salt okunur"* ]] || false
    comp_installed_version() { return 0; }
    run comp_menu_server WEB1 "$D" <<<"0"
    [[ "$output" == *"Kurulu paket yok"* ]] || false
}

@test "sunucu girişi: birden çok paketten seçim yapılır" {
    comp_ids() { printf 'demo\nother\n'; }
    comp_installed_version() { printf '1.0.0'; }
    comp_prepare() { printf '%s\n' "$INFO"; }
    run comp_menu_server WEB1 "$D" <<<$'1\n2\n\n0\n0'
    [ "$status" -eq 0 ]
    [[ "$output" == *"demo 1.0.0"* ]] || false
    [[ "$output" == *"other 1.0.0"* ]] || false
    [ "$(sed -n 1p "$T/runs")" = "demo preview web1" ]
}

# ---------- sunucu menüsü entegrasyonu ----------
@test "sunucu menüsü: paket varsa '6) Paketler' görünür ve 6 comp_menu_server'ı çağırır" {
    comp_menu_server() { echo "MENU:$1:$2" >> "$T/menucalls"; }
    run ssh_module_run WEB1 "$D" <<<$'6\n\n0'
    [[ "$output" == *"6) 🛡️  Paketler"* ]] || false
    [ "$(cat "$T/menucalls")" = "MENU:WEB1:$D" ]
}

@test "sunucu menüsü: paket yoksa 6) görünmez ve 6 girmek menüden çıkarır (hiçbir şey çalışmaz)" {
    comp_installed_version() { return 0; }
    comp_menu_server() { echo "MENU" >> "$T/menucalls"; }
    run ssh_module_run WEB1 "$D" <<<$'6\n0'
    [[ "$output" != *"Paketler (sertleştirme"* ]] || false
    [ ! -e "$T/menucalls" ]
}

# =====================================================================
# E2b: hazırlık bloğu, değişken ekranı, CIDR editörü
# =====================================================================
vget() { comp_vars_get "$D/demo.vars" "$1"; }

setup_net() {
    comp_net_local() { echo "10.99.0.4/32"; }
    comp_net_seen_by_server() { return 1; }
}

# ---------- hazırlık ----------
@test "hazırlık: zorunlu değişken eksikse ❌ gösterilir, eylemler ⛔ kapalıdır ve çalıştırılmaz" {
    rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'2\n\n0'
    [ "$status" -eq 0 ]
    [[ "$output" == *"❌ allow_from"* ]] || false
    [[ "$output" == *"preview — 🔍 salt okunur  ⛔"* ]] || false
    [[ "$output" == *"Önce zorunlu değişkenleri tamamlayın"* ]] || false
    [ ! -e "$T/runs" ]
}

@test "hazırlık: zorunlu değişken eksikken SALT OKUNUR durum eylemi (status) AÇIK ve çalışır; preview/apply/audit ⛔" {
    # yalnızca bu test için pakete ön koşul olmayan, salt okunur bir 'status' eylemi eklenir (ortak fikstür değişmez)
    python3 - "$T/m.json" <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p))
d["actions"]["status"]={"cmd":"ansible-playbook s.yml","effects":[]}
json.dump(d,open(p,"w"),ensure_ascii=False)
PY
    INFO=$(comp_manifest_info "$T/m.json" demo 1.0.0)
    rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" == *"status — 🔍 salt okunur"* ]] || false
    [[ "$output" != *"status — 🔍 salt okunur  ⛔"* ]] || false
    [[ "$output" == *"preview — 🔍 salt okunur  ⛔"* ]] || false
    [[ "$output" == *"apply — "*"⛔"* ]] || false
    [[ "$output" == *"audit — "*"⛔"* ]] || false
    [[ "$output" == *"Salt okunur durum eylemleri açıktır"* ]] || false
    # eylem sırası: preview=2 apply=3 wipe=4 audit=5 status=6 (bildirim sırası); status seçilince çalışır, kapı çıkmaz
    run comp_menu_actions demo web1 <<<$'6\n\n0'
    [ "$status" -eq 0 ]
    [ "$(cat "$T/runs")" = "demo status web1" ]
    [[ "$output" != *"Önce zorunlu değişkenleri tamamlayın"* ]] || false
}

@test "hazırlık: zorunlu değişkenler tamamsa ✅ gösterilir ve eylem çalışır" {
    run comp_menu_actions demo web1 <<<$'2\n\n0'
    [[ "$output" == *"✅ Zorunlu değişkenler tamam"* ]] || false
    [ "$(cat "$T/runs")" = "demo preview web1" ]
}

# ---------- değişken ekranı ----------
@test "değişkenler: değer, varsayılan ve otomatik (zorlanan) değişkenler doğru etiketlenir" {
    run comp_menu_actions demo web1 <<<$'1\n0\n0'
    [ "$status" -eq 0 ]
    [[ "$output" == *"allow_from = 10.99.0.0/24"* ]] || false
    [[ "$output" == *"scan_dir — otomatik"* ]] || false
    [[ "$output" == *"rb_enabled = (varsayılan: true)"* ]] || false
    [[ "$output" == *"rb_timeout = (varsayılan: 300)"* ]] || false
    [[ "$output" == *"mode = (varsayılan: a)"* ]] || false
}

@test "değişkenler: geçerli değer kaydedilir; geçersiz (aralık) kaydedilmez ve neden gösterilir" {
    run comp_menu_actions demo web1 <<<$'1\n4\n600\n\n0\n0'
    [ "$(vget rb_timeout)" = "600" ]
    run comp_menu_actions demo web1 <<<$'1\n4\n5\n\n0\n0'
    [[ "$output" == *"en az 30"* ]] || false
    [ "$(vget rb_timeout)" = "600" ]                      # eski değer korunur
}

@test "değişkenler: '-' siler (varsayılana döner); boş Enter değiştirmez" {
    comp_vars_set "$D/demo.vars" rb_timeout 600
    run comp_menu_actions demo web1 <<<$'1\n4\n\n0\n0'
    [ "$(vget rb_timeout)" = "600" ]
    run comp_menu_actions demo web1 <<<$'1\n4\n-\n\n0\n0'
    run vget rb_timeout
    [ "$status" -ne 0 ]
}

@test "değişkenler: warn_if değeri onay ister (H = yazmaz, e = yazar)" {
    run comp_menu_actions demo web1 <<<$'1\n3\nfalse\nh\n0\n0'
    [[ "$output" == *"riskli"* ]] || false
    run vget rb_enabled; [ "$status" -ne 0 ]
    run comp_menu_actions demo web1 <<<$'1\n3\nfalse\ne\n\n0\n0'
    [ "$(vget rb_enabled)" = "false" ]
}

@test "değişkenler: otomatik (dir_var) değişken seçilince açıklama gösterilir, hiçbir şey yazılmaz" {
    run comp_menu_actions demo web1 <<<$'1\n2\n\n0\n0'
    [[ "$output" == *"çalıştırıcı tarafından belirlenir"* ]] || false
    run vget scan_dir; [ "$status" -ne 0 ]
}

@test "değişkenler: girdi yarıda biterse (EOF) takılmaz ve dosyaya yazılmaz" {
    run comp_menu_actions demo web1 <<<$'1\n4\n'
    [ "$status" -eq 0 ]
    run vget rb_timeout; [ "$status" -ne 0 ]
}

@test "değişkenler: platforma uymayan değişkenler gizlenir, gruplar etiketle gösterilir, gizlenen sayısı yazılır" {
    python3 - "$T/comps/demo/1.0.0/component.json" <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p))
d["platform_var"]="plat"
d["vars"]=[{"name":"allow_from","type":"list[cidr]","required":True},
           {"name":"plat","type":"enum","values":["generic","proxmox-host"],"default":"generic"},
           {"name":"genel_a","type":"bool","default":True},
           {"name":"pve_x","type":"bool","default":False,"platforms":["proxmox-host"],"group":"Proxmox"},
           {"name":"ag_z","type":"bool","default":False,"group":"Ağ"}]
json.dump(d,open(p,"w"))
PY
    run comp_menu_actions demo web1 <<<$'1\n0\n0'
    [[ "$output" == *"genel_a"* && "$output" == *"[Ağ] ag_z"* ]] || false
    [[ "$output" != *"pve_x"* ]] || false
    [[ "$output" == *"1 değişken bu sunucunun platformuna uygun olmadığı için gizlendi"* ]] || false
    printf 'allow_from=10.99.0.0/24\nplat=proxmox-host\n' > "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'1\n0\n0'
    [[ "$output" == *"[Proxmox] pve_x"* ]] || false
    [[ "$output" != *"gizlendi"* ]] || false
}

@test "eylemler: platforma uymayan eylemler menüde gizlenir, sayısı yazılır; platform uyunca görünür" {
    python3 - "$T/comps/demo/1.0.0/component.json" <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p))
d["platform_var"]="plat"
d["vars"]=[{"name":"allow_from","type":"list[cidr]","required":True},
           {"name":"plat","type":"enum","values":["generic","proxmox-host"],"default":"generic"},
           {"name":"scan_dir","type":"path"}]
d["actions"]["pve_check"]={"cmd":"ansible-playbook p.yml --check","effects":[],"platforms":["proxmox-host"]}
json.dump(d,open(p,"w"))
PY
    INFO=$(comp_manifest_info "$T/comps/demo/1.0.0/component.json" demo 1.0.0)
    run comp_menu_actions demo web1 <<<"0"
    [[ "$output" != *"pve_check"* ]] || false
    [[ "$output" == *"1 eylem bu sunucunun platformuna uygun olmadığı için gizlendi"* ]] || false
    printf 'allow_from=10.99.0.0/24\nplat=proxmox-host\n' > "$D/demo.vars"
    run comp_menu_actions demo web1 <<<"0"
    [[ "$output" == *"pve_check"* ]] || false
    [[ "$output" != *"gizlendi"* ]] || false
}

# ---------- değişkenler ekranı: arama, grup, değiştirdiklerim, kısaltma ----------
@test "süzgeç: küçük pakette arama satırları çıkmaz; eşik aşılınca çıkar ve numaralandırma 'ara' satırlarından sonra başlar" {
    run comp_menu_actions demo web1 <<<$'1\n0\n0'
    [[ "$output" != *"🔎 Ara"* ]] || false
    export COMP_MENU_VARS_FILTER_MIN=2
    run comp_menu_actions demo web1 <<<$'1\n0\n0'
    [[ "$output" == *"🔎 Ara: (yok)"* && "$output" == *"Yalnızca değiştirdiklerim: kapalı (1)"* && "$output" == *"Gösterilen: 5 / 5"* ]] || false
    [[ "$output" != *"Grup:"* ]] || false      # pakette grup yoksa grup satırı çıkmaz
    # ilk değişken (allow_from) artık 3. satır: 1=ara 2=değiştirdiklerim
    run comp_menu_actions demo web1 <<<$'1\n4\n\n0\n0'
    [[ "$output" == *"rb_enabled"* ]] || false
}

@test "süzgeç: arama ad/açıklamada büyük-küçük harfsiz çalışır; temizle satırı çıkar ve süzgeci kaldırır" {
    export COMP_MENU_VARS_FILTER_MIN=2
    run comp_menu_actions demo web1 <<<$'1\n1\nTIMEOUT\n0\n0'
    [[ "$output" == *"🔎 Ara: TIMEOUT"* && "$output" == *"Gösterilen: 1 / 5"* && "$output" == *"↩ Süzgeci temizle"* ]] || false
    run comp_menu_actions demo web1 <<<$'1\n1\nyokböyle\n0\n0'
    [[ "$output" == *"Süzgece uyan değişken yok."* ]] || false
    # arama + temizle (son satır): yeniden 5 / 5
    run comp_menu_actions demo web1 <<<$'1\n1\ntimeout\n4\n0\n0'
    [[ "$output" == *"Gösterilen: 5 / 5"* ]] || false
}

@test "süzgeç: 'yalnızca değiştirdiklerim' dosyada değeri yazılı olanları gösterir (✏️ işaretli)" {
    export COMP_MENU_VARS_FILTER_MIN=2
    run comp_menu_actions demo web1 <<<$'1\n2\n0\n0'
    [[ "$output" == *"Yalnızca değiştirdiklerim: açık (1)"* && "$output" == *"Gösterilen: 1 / 5"* && "$output" == *"✏️  allow_from = 10.99.0.0/24"* ]] || false
}

@test "süzgeç: grup seçimi yalnızca o grubu gösterir; '(grupsuz)' grupsuzları gösterir" {
    python3 - "$T/comps/demo/1.0.0/component.json" <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p))
for v in d["vars"]:
    v["group"]="Ağ" if v["name"]=="allow_from" else ("Zaman" if v["name"].startswith("rb_") else None)
    if v["group"] is None: del v["group"]
json.dump(d,open(p,"w"))
PY
    INFO=$(comp_manifest_info "$T/comps/demo/1.0.0/component.json" demo 1.0.0)
    export COMP_MENU_VARS_FILTER_MIN=2
    run comp_menu_actions demo web1 <<<$'1\n2\n2\n0\n0'
    [[ "$output" == *"📂 Grup: Ağ"* && "$output" == *"Gösterilen: 1 / 5"* ]] || false
    run comp_menu_actions demo web1 <<<$'1\n2\n4\n0\n0'
    [[ "$output" == *"📂 Grup: (grupsuz)"* && "$output" == *"Gösterilen: 2 / 5"* ]] || false
}

@test "süzgeç: süzülmüş listeden seçilen satır doğru değişkeni düzenler" {
    export COMP_MENU_VARS_FILTER_MIN=2
    run comp_menu_actions demo web1 <<<$'1\n1\ntimeout\n4\n600\n\n0\n0'
    [ "$(vget rb_timeout)" = "600" ]
}

@test "kısaltma: uzun değerler 40 karaktere kısaltılır; kısa değerler değişmez" {
    run _comp_menu_clip "kısa"
    [ "$output" = "kısa" ]
    long=$(python3 -c "print('a'*60)")
    run _comp_menu_clip "$long"
    [ "$(python3 -c 'import sys; print(len(sys.argv[1]))' "$output")" = 40 ]
    [[ "$output" == *"…" ]] || false
    run _comp_menu_clip "$long" 10
    [ "$(python3 -c 'import sys; print(len(sys.argv[1]))' "$output")" = 10 ]
}

# ---------- CIDR editörü ----------
@test "cidr: öneri eklenir; listede tek ağ varsa uyarı çıkar ve 'H' ile o haliyle kaydedilir" {
    setup_net; rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'1\n1\n1\nk\nh\n\n0\n0'
    [ "$status" -eq 0 ]
    [[ "$output" == *"Bu bilgisayarın sunucuya bağlandığı adres: 10.99.0.4/32"* ]] || false
    [[ "$output" == *"tahmini /24"* ]] || false
    [[ "$output" == *"yalnızca bir ağ var"* ]] || false
    [ "$(vget allow_from)" = "10.99.0.0/24" ]
}

@test "cidr: iki ağ eklenince tek-ağ uyarısı çıkmaz; liste virgülle kaydedilir" {
    setup_net; rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'1\n1\n1\ne\n192.168.1.0/24\nk\n\n0\n0'
    [[ "$output" != *"yalnızca bir ağ var"* ]] || false
    [ "$(vget allow_from)" = "10.99.0.0/24,192.168.1.0/24" ]
}

@test "cidr: tek ağda 'e' denirse kaydedilmez, düzenlemeye devam edilir (ikinci ağ eklenip kaydedilir)" {
    setup_net; rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'1\n1\n1\nk\ne\ne\n192.168.1.0/24\nk\n\n0\n0'
    [ "$(vget allow_from)" = "10.99.0.0/24,192.168.1.0/24" ]
}

@test "cidr: geçersiz elle girilen CIDR eklenmez; iptal (0) dosyayı değiştirmez" {
    setup_net; rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'1\n1\ne\n10.0.0.0/99\n0\n0\n0'
    [[ "$output" == *"⚠️"* ]] || false
    run vget allow_from; [ "$status" -ne 0 ]
    [ ! -e "$D/demo.vars" ] || ! grep -q allow_from "$D/demo.vars"
}

@test "cidr: sil (d), temizle (t); boş liste kaydedilemez" {
    setup_net
    comp_vars_set "$D/demo.vars" allow_from "10.0.0.0/24,172.16.0.0/16"
    run comp_menu_actions demo web1 <<<$'1\n1\nd\n1\nk\nh\n\n0\n0'
    [ "$(vget allow_from)" = "172.16.0.0/16" ]
    comp_vars_set "$D/demo.vars" allow_from "10.0.0.0/24,172.16.0.0/16"
    run comp_menu_actions demo web1 <<<$'1\n1\nt\nk\n0\n0\n0'
    [[ "$output" == *"Liste boş olamaz"* ]] || false
    [ "$(vget allow_from)" = "10.0.0.0/24,172.16.0.0/16" ]
}

@test "cidr: aynı öneri iki kez eklenmez" {
    setup_net; rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'1\n1\n1\n1\ne\n192.168.1.0/24\nk\n\n0\n0'
    [[ "$output" == *"Zaten listede"* ]] || false
    [ "$(vget allow_from)" = "10.99.0.0/24,192.168.1.0/24" ]
}

@test "cidr: 'v' sunucunun gördüğü adresi sorgular ve önerileri günceller; sorgu başarısızsa uyarır" {
    setup_net; rm -f "$D/demo.vars"
    comp_net_seen_by_server() { echo "10.50.0.9"; }
    run comp_menu_actions demo web1 <<<$'1\n1\nv\n1\ne\n192.168.1.0/24\nk\n\n0\n0'
    [[ "$output" == *"Sunucu seni şu adresten görüyor: 10.50.0.9"* ]] || false
    [ "$(vget allow_from)" = "10.50.0.0/24,192.168.1.0/24" ]
    rm -f "$D/demo.vars"; comp_net_seen_by_server() { return 1; }
    run comp_menu_actions demo web1 <<<$'1\n1\nv\n0\n0\n0'
    [[ "$output" == *"Sunucudan sorgulanamadı"* ]] || false
}

@test "cidr: yerel adres belirlenemezse öneri yok ama elle ekleme çalışır" {
    comp_net_local() { return 1; }; rm -f "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'1\n1\ne\n192.168.1.0/24\nk\nh\n\n0\n0'
    [[ "$output" == *"adresi belirlenemedi"* ]] || false
    [ "$(vget allow_from)" = "192.168.1.0/24" ]
}

@test "cidr: Main.sh ortamındaki IFS (satır sonu/sekme) liste birleştirmeyi ve üyelik kontrolünü bozmaz" {
    setup_net; rm -f "$D/demo.vars"
    IFS=$'\n\t'
    run comp_menu_actions demo web1 <<<$'1\n1\n1\n1\ne\n192.168.1.0/24\nk\n\n0\n0'
    [ "$(vget allow_from)" = "10.99.0.0/24,192.168.1.0/24" ]
    [ "$(grep -c '^allow_from=' "$D/demo.vars")" = "1" ]
}

# =====================================================================
# E3: Geçmiş, ön koşul tazeliği ve son sonuç (F)
# =====================================================================
setup_state() {
    export SSHPOCKET_STATE_DIR="$T/state" COMP_REQUIRES_TTL=1800
    source "$H/ComponentsGates.sh"; source "$H/ComponentsExec.sh"
}
# actions listesi: 1 Değişkenler, 2 preview, 3 apply, 4 wipe, 5 audit, 6 Kontrol, 7 Geçmiş (son; Bulgular varsa 8)

@test "gates: requires_age en son başarılı kaydın yaşını verir; fail/başka sürüm/gelecek/kayıt yok sayılmaz" {
    setup_state
    run comp_gate_requires_age demo web1 1.0.0 preview; [ "$status" -ne 0 ]
    now=$(date +%s); mkdir -p "$T/state/demo/web1"
    printf '%s|1.0.0|preview|ok\n%s|1.0.0|preview|ok\n%s|1.0.0|preview|fail\n%s|0.9.0|preview|ok\n%s|1.0.0|preview|ok\n' \
        $((now - 600)) $((now - 120)) $((now - 10)) $((now - 5)) $((now + 3600)) > "$T/state/demo/web1/results"
    run comp_gate_requires_age demo web1 1.0.0 preview
    [ "$status" -eq 0 ]
    [ "$output" -ge 119 ]          # ayrı satırlar: bash'te "a && b" zincirinde ilk koşulun başarısızlığı testi düşürmez
    [ "$output" -le 125 ]
    run comp_gate_requires_age demo web1 1.0.0 apply; [ "$status" -ne 0 ]
    run comp_gate_requires_age demo "../x" 1.0.0 preview; [ "$status" -ne 0 ]
}

@test "eylem etiketi: önkoşul kaydı yoksa '(önce: preview)'; taze ise ✅ yaş; süresi dolmuşsa ⏳" {
    setup_state
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" == *"apply — ✎ değişiklik yapar  (önce: preview)"* ]] || false
    comp_gate_record demo web1 1.0.0 preview ok
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" == *"(önce: preview ✅ "* ]] || false
    printf '%s|1.0.0|preview|ok\n' $(( $(date +%s) - 3000 )) > "$T/state/demo/web1/results"
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" == *"(önce: preview ⏳ süresi doldu)"* ]] || false
}

@test "geçmiş: son çalıştırmalar (yalnızca bu paket), ön koşul tazeliği ve günlük yolu gösterilir" {
    setup_state
    printf '2026-10-16T14:17:49Z|demo|1.0.0|preview|ok\n2026-10-16T14:20:00Z|baska|1.0.0|yabanciaction|ok\n2026-10-16T14:38:32Z|demo|1.0.0|apply|fail\n' > "$D/components.log"
    comp_gate_record demo web1 1.0.0 preview ok
    mkdir -p "$T/state/demo/web1"; echo "ansible cikti" > "$T/state/demo/web1/last-run.log"
    run comp_menu_actions demo web1 <<<$'7\nh\n\n0'
    [ "$status" -eq 0 ]
    [[ "$output" == *"2026-10-16T14:17:49Z  preview"* ]] || false
    [[ "$output" == *"2026-10-16T14:38:32Z  apply"* ]] || false
    [[ "$output" == *"❌ başarısız"* ]] || false
    [[ "$output" != *"yabanciaction"* ]] || false          # başka paketin kaydı görünmez
    [[ "$output" == *"apply için preview: ✅ taze"* ]] || false
    [[ "$output" == *"last-run.log"* ]] || false
}

@test "geçmiş: kayıt yoksa nazik mesajlar; günlüğün son satırları istenirse gösterilir ve denetim karakterleri temizlenir" {
    setup_state
    run comp_menu_actions demo web1 <<<$'7\n\n0'
    [[ "$output" == *"(kayıt yok)"* ]] || false
    [[ "$output" == *"apply için preview: ❌ başarılı kayıt yok"* ]] || false
    mkdir -p "$T/state/demo/web1"; printf 'satir1\n\033[31mkirmizi\033[0m\n' > "$T/state/demo/web1/last-run.log"
    run comp_menu_actions demo web1 <<<$'7\ne\n\n0'
    [[ "$output" == *"satir1"* ]] || false
    [[ "$output" == *"[31mkirmizi"* ]] || false
    [[ "$output" != *$'\033'* ]] || false
}

@test "geçmiş: süresi dolmuş preview açıkça belirtilir" {
    setup_state
    mkdir -p "$T/state/demo/web1"
    printf '%s|1.0.0|preview|ok\n' $(( $(date +%s) - 4000 )) > "$T/state/demo/web1/results"
    run comp_menu_actions demo web1 <<<$'7\nh\n\n0'
    [[ "$output" == *"apply için preview: ⏳ süresi doldu"* ]] || false
}

# ---------- F: son bilinen durum ----------
@test "exec: durum eyleminin sonucu saklanır (epoch + anahtar=değer, izin 600) ve okunur" {
    setup_state
    mkdir -p "$T/state/demo/web1"
    COMP_EXEC_RESULT=$'armed=yes\ntimer=active\nremaining_seconds=42'
    comp_exec_save_result "$T/state/demo/web1" status
    [ "$(stat -c %a "$T/state/demo/web1/result.status" 2>/dev/null || stat -f %Lp "$T/state/demo/web1/result.status")" = "600" ]
    run comp_exec_last_result demo web1 status
    [ "$status" -eq 0 ]
    [[ "${lines[0]}" =~ ^[0-9]+$ ]]
    [ "${lines[1]}" = "armed=yes" ]
    [ "${lines[3]}" = "remaining_seconds=42" ]
    run comp_exec_last_result demo web1 yok;   [ "$status" -ne 0 ]
    run comp_exec_last_result demo web1 "a/b"; [ "$status" -ne 0 ]
    COMP_EXEC_RESULT=""; comp_exec_save_result "$T/state/demo/web1" other
    [ ! -e "$T/state/demo/web1/result.other" ]
    run comp_exec_save_result "$T/state/demo/web1" "../x"; [ "$status" -ne 0 ]
    [ "$(ls "$T/state/demo/web1")" = "result.status" ]                                  # geçici dosya kalmadı
}

@test "hazırlık: durum eyleminin son bilinen sonucu yaşıyla gösterilir; henüz çalışmadıysa belirtilir" {
    setup_state
    # status eylemi olan bildirim
    python3 - "$T/comps/demo/1.0.0/component.json" <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p))
d["actions"]["status"]={"cmd":"ansible-playbook s.yml","effects":[],"outputs":{"format":"keyvalue-line","keys":["armed","timer"]}}
json.dump(d,open(p,"w"))
PY
    cp "$T/comps/demo/1.0.0/component.json" "$T/m.json"
    INFO=$(comp_manifest_info "$T/m.json" demo 1.0.0)
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" == *"ℹ️  status: henüz çalıştırılmadı"* ]] || false
    mkdir -p "$T/state/demo/web1"
    COMP_EXEC_RESULT=$'armed=no\ntimer=inactive'; comp_exec_save_result "$T/state/demo/web1" status
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" == *"ℹ️  status ("*"önce): armed=no timer=inactive"* ]] || false
}

# ---------- kapsama (üst üste binen ağlar) ----------
@test "cidr: mevcut daha geniş bir ağın içindeki öneri eklenmez ('Zaten kapsanıyor')" {
    setup_net; comp_vars_set "$D/demo.vars" allow_from "10.99.0.0/16"
    run comp_menu_actions demo web1 <<<$'1\n1\n1\ne\n192.168.1.0/24\nk\n\n0\n0'
    [[ "$output" == *"Zaten kapsanıyor: 10.99.0.0/16"* ]] || false
    [ "$(vget allow_from)" = "10.99.0.0/16,192.168.1.0/24" ]
}

@test "cidr: daha geniş ağ eklenince kapsadığı dar girdiler yerine geçer ve kullanıcı bilgilendirilir" {
    setup_net; comp_vars_set "$D/demo.vars" allow_from "10.99.0.4/32,192.168.1.0/24"
    run comp_menu_actions demo web1 <<<$'1\n1\ne\n10.99.0.0/24\nk\n\n0\n0'
    [[ "$output" == *"kapsadığı için kaldırıldı: 10.99.0.4/32"* ]] || false
    [ "$(vget allow_from)" = "192.168.1.0/24,10.99.0.0/24" ]
}


# ---------- etkileşimli değişken düzenleme (bool/enum seçici, yeniden deneme, duraksamasız ileti) ----------
# Sahte seçici: kuyruktaki sıradaki numarayı verir (eylem listesi, değişken listesi, değer seçici aynı kuyruktan); aldığı başlangıç konumunu kaydeder
interactive_pick() {
    : > "$T/q"; for x in "$@"; do echo "$x" >> "$T/q"; done
    ui_arrows_enabled() { return 0; }
    ui_pick_arrows() {
        echo "$1" >> "$T/starts"; echo "$UI_PICK_TITLE|$UI_PICK_HEADER" >> "$T/heads"
        local a; a=$(head -n 1 "$T/q"); sed -i.bak '1d' "$T/q" 2>/dev/null || true; rm -f "$T/q.bak"
        printf '%s' "${a:-0}"
    }
}

@test "etkileşimli bool: yazılı değer yokken varsayılan true ise imleç false'ta; false seçilince riskli onayla yazılır ve ileti listede görünür" {
    interactive_pick 1 3 2 0 0
    run comp_menu_actions demo web1 <<<"e"
    [ "$(vget rb_enabled)" = "false" ]
    [[ "$output" == *"✅ Kaydedildi: rb_enabled = false"* ]] || false
    grep -qx "2" "$T/starts"
    [[ "$(cat "$T/heads")" == *"Değişkenler › rb_enabled"* ]] || false
}

@test "etkileşimli bool: Geri hiçbir şey yazmaz; riskli değerde H yazmaz ve iptal ileti gösterir" {
    interactive_pick 1 3 0 0 0
    run comp_menu_actions demo web1 </dev/null
    run vget rb_enabled; [ "$status" -ne 0 ]
    interactive_pick 1 3 2 0 0
    run comp_menu_actions demo web1 <<<"h"
    out="$output"
    run vget rb_enabled; [ "$status" -ne 0 ]
    [[ "$out" == *"İptal edildi: rb_enabled değişmedi"* ]] || false
}

@test "etkileşimli enum: izinli değerler listelenir ve seçilen yazılır" {
    interactive_pick 1 5 2 0 0
    run comp_menu_actions demo web1 </dev/null
    [ "$(vget mode)" = "b" ]
}

@test "etkileşimli: 'Varsayılana dön' yazılı değeri siler" {
    comp_vars_set "$D/demo.vars" rb_enabled true
    interactive_pick 1 3 3 0 0
    run comp_menu_actions demo web1 </dev/null
    out="$output"
    run vget rb_enabled; [ "$status" -ne 0 ]
    [[ "$out" == *"Silindi (varsayılana döner): rb_enabled"* ]] || false
}

@test "etkileşimli int: geçersiz değerde ekran kapanmadan yeniden sorar (3 deneme); geçerli değer yazılır" {
    interactive_pick 1 4 0 0
    run comp_menu_actions demo web1 <<<$'5\n600'
    [[ "$output" == *"deneme 1/3"* ]] || false
    [ "$(vget rb_timeout)" = "600" ]
    comp_vars_unset "$D/demo.vars" rb_timeout
    interactive_pick 1 4 0 0
    run comp_menu_actions demo web1 <<<$'5\n6\n7'
    out="$output"
    run vget rb_timeout; [ "$status" -ne 0 ]
    [[ "$out" == *"en az 30"* ]] || false
    [[ "$out" == *"deneme 2/3"* ]] || false
}

@test "describe: enum izinli değerleri 10. alanda JSON dizi olarak verir" {
    run bash -c "python3 \"$H/component_run.py\" describe \"$T/m.json\" | awk -F'\t' '\$1==\"mode\" {print \$10}'"
    [ "$output" = '["a","b"]' ]
}


# ---------- etkileşimli liste editörü (list[string], list[int], list[cidr]) ----------
liste_manifest() {
    python3 - "$T/m.json" "$T/comps/demo/1.0.0/component.json" <<'PY'
import json,sys
m=json.load(open(sys.argv[1]))
m["vars"].append({"name":"groups","type":"list[string]","default":[],"description":"izinli gruplar"})
m["vars"].append({"name":"ports","type":"list[int]","default":[80,443],"description":"yayınlanan portlar"})
json.dump(m,open(sys.argv[2],"w"))
PY
    cp "$T/comps/demo/1.0.0/component.json" "$T/m.json"
    INFO=$(comp_manifest_info "$T/m.json" demo 1.0.0)
    comp_net_local() { return 1; }          # öneri yok (makineye bağlı olmasın)
}

# Sıra: eylem listesi → 1 (Değişkenler) → değişken numarası; değişken sırası allow_from=1 scan_dir=2 rb_enabled=3 rb_timeout=4 mode=5 groups=6 ports=7
# Editör satırları (boş liste): Ekle=1 Kaydet=2 · tek öğe: •=1 Ekle=2 Temizle=3 Kaydet=4 · iki öğe: •=1 •=2 Ekle=3 Temizle=4 Kaydet=5
# list[cidr] (öneri yok): •=1 Ekle=2 Sorgula=3 Temizle=4 Kaydet=5 · iki öğe: Kaydet=6 · boş: Ekle=1 Sorgula=2 Kaydet=3
@test "liste editörü: ekle + kaydet yazar; Vazgeç dosyayı değiştirmez; ileti listede görünür" {
    liste_manifest
    interactive_pick 1 6 1 0 0
    run comp_menu_actions demo web1 <<<"admin"
    run vget groups; [ "$status" -ne 0 ]
    interactive_pick 1 6 1 4 0 0
    run comp_menu_actions demo web1 <<<"admin"
    out="$output"
    [ "$(vget groups)" = "admin" ]
    [[ "$out" == *"✅ Kaydedildi: groups = admin"* ]] || false
}

@test "liste editörü: iki öğe eklenir, virgülle yazılır; aynı öğe iki kez eklenmez" {
    liste_manifest
    interactive_pick 1 6 1 2 3 5 0 0
    run comp_menu_actions demo web1 <<<$'a\nb\nb'
    [ "$(vget groups)" = "a,b" ]
}

@test "liste editörü: öğeye Enter → Sil ve Değiştir; Kaydet'e kadar dosya değişmez" {
    liste_manifest
    comp_vars_set "$D/demo.vars" groups "a,b,c"
    interactive_pick 1 6 2 1 0 0
    run comp_menu_actions demo web1 </dev/null
    [ "$(vget groups)" = "a,b,c" ]
    interactive_pick 1 6 2 1 5 0 0
    run comp_menu_actions demo web1 </dev/null
    [ "$(vget groups)" = "a,c" ]
    interactive_pick 1 6 1 2 5 0 0
    run comp_menu_actions demo web1 <<<"z"
    [ "$(vget groups)" = "z,c" ]
}

@test "liste editörü: list[int] geçersiz öğeyi reddeder (neden gösterilir), geçerliyi yazar" {
    liste_manifest
    interactive_pick 1 7 1 1 4 0 0
    run comp_menu_actions demo web1 <<<$'abc\n8080'
    out="$output"
    [ "$(vget ports)" = "8080" ]
    [[ "$out" == *"tamsayı"* ]] || false
}

@test "liste editörü: zorunlu liste boş kaydedilemez; zorunlu olmayan boş liste değişkeni siler" {
    liste_manifest
    interactive_pick 1 1 4 3 0 0 0
    run comp_menu_actions demo web1 </dev/null
    out="$output"
    [[ "$out" == *"Liste boş olamaz"* ]] || false
    [ "$(vget allow_from)" = "10.99.0.0/24" ]
    comp_vars_set "$D/demo.vars" groups "a"
    interactive_pick 1 6 3 2 0 0
    run comp_menu_actions demo web1 </dev/null
    run vget groups; [ "$status" -ne 0 ]
}

@test "liste editörü: Vazgeç hiçbir şey yazmaz ve 'Vazgeçildi' iletisi gösterir" {
    liste_manifest
    interactive_pick 1 6 1 0 0 0
    run comp_menu_actions demo web1 <<<"x"
    out="$output"
    run vget groups; [ "$status" -ne 0 ]
    [[ "$out" == *"Vazgeçildi: groups değişmedi"* ]] || false
}

@test "liste editörü: list[cidr] tek ağla kaydederken uyarı; H o haliyle yazar, e düzenlemeye döner" {
    liste_manifest
    interactive_pick 1 1 5 0 0
    run comp_menu_actions demo web1 <<<"h"
    out="$output"
    [[ "$out" == *"yalnızca bir ağ var"* ]] || false
    [ "$(vget allow_from)" = "10.99.0.0/24" ]
    [[ "$out" == *"✅ Kaydedildi: allow_from = 10.99.0.0/24"* ]] || false
    interactive_pick 1 1 2 6 0 0
    run comp_menu_actions demo web1 <<<"10.50.0.0/24"
    [ "$(vget allow_from)" = "10.99.0.0/24,10.50.0.0/24" ] || [ "$(vget allow_from)" = "10.50.0.0/24,10.99.0.0/24" ]
}


# ---------- eylem paneli (fzf kartları) ve sonuç paneli ----------
@test "eylem kartı: etkiler, onay, ön koşul ve uyarılar; kapalı eylemde ⛔" {
    export SSHPOCKET_STATE_DIR="$T/state"
    _comp_menu_ctx demo web1 "$INFO"
    run _comp_menu_action_card "$INFO" demo web1 apply ""
    [[ "$output" == *"apply  —  "* ]] || false
    [[ "$output" == *"Etkiler: changes-config"* ]] || false
    [[ "$output" == *"Onay: type-server-name"* ]] || false
    [[ "$output" == *"Ön koşul: preview"* ]] || false
    run _comp_menu_action_card "$INFO" demo web1 preview ""
    [[ "$output" == *"⚠ Önizleme ufw"* ]] || false
    [[ "$output" == *"Etkiler: yok (salt okunur)"* ]] || false
    run _comp_menu_action_card "$INFO" demo web1 apply "allow_from"
    [[ "$output" == *"⛔ Kapalı"* ]] || false
}

@test "fzf paneli: hazırlık özeti başlıkta, her eylemin kartı sağ bölmede (geniş düzen)" {
    export SSHPOCKET_STATE_DIR="$T/state"
    ui_fzf_enabled() { return 0; }
    ui_arrows_enabled() { return 0; }
    ui_pick_arrows() {
        echo "W=$UI_PICK_WIDE" > "$T/panel"
        echo "T=$UI_PICK_TITLE" >> "$T/panel"
        echo "H=$UI_PICK_HEADER" >> "$T/panel"
        for d in "${UI_PICK_DESCS[@]}"; do echo "D=${d//$'\n'/|}" >> "$T/panel"; done
        printf 0
    }
    run comp_menu_actions demo web1 </dev/null
    [ "$status" -eq 0 ]
    grep -q "^W=1" "$T/panel"
    grep -q "^T=Sunucu › web1 › Paketler › demo 1.0.0" "$T/panel"
    grep -q "Zorunlu değişkenler tamam" "$T/panel"
    [ "$(grep -c '^D=' "$T/panel")" -eq 7 ]            # değişkenler + 4 eylem + Kontrol + geçmiş
    grep -q "^D=apply  —  .*Ön koşul: preview" "$T/panel"
    run ! grep -q "Hazırlık" <<< "$output"               # klasik düz metin bloğu çıkmaz
}

@test "sonuç paneli: başarı, özet, günlük ve sıradaki adım (preview → apply)" {
    export SSHPOCKET_STATE_DIR="$T/state"; UI_RULE='---'
    mkdir -p "$T/state/demo/web1"
    printf 'PLAY RECAP\nweb1 : ok=190 changed=50 unreachable=0 failed=0 skipped=184\n' > "$T/state/demo/web1/last-run.log"
    run _comp_menu_run_panel demo web1 "$INFO" preview 0 12
    [[ "$output" == *"Sonuç: preview — web1"* ]] || false
    [[ "$output" == *"✅ Başarılı · 12 sn"* ]] || false
    [[ "$output" == *"Özet: ok=190 changed=50 unreachable=0 failed=0 skipped=184"* ]] || false
    [[ "$output" == *"Günlük: $T/state/demo/web1/last-run.log"* ]] || false
    [[ "$output" == *"Sıradaki adım: apply"* ]] || false
}

@test "sonuç paneli: başarısızlıkta günlüğe yönlendirir; çalıştırılmadıysa (kapı) bunu söyler ve sıradaki adım önermez" {
    export SSHPOCKET_STATE_DIR="$T/state"; UI_RULE='---'
    mkdir -p "$T/state/demo/web1"
    run _comp_menu_run_panel demo web1 "$INFO" apply 4 3
    [[ "$output" == *"❌ Başarısız (çıkış 4)"* ]] || false
    [[ "$output" == *"günlüğe bakın"* ]] || false
    run _comp_menu_run_panel demo web1 "$INFO" apply 190 0
    [[ "$output" == *"⛔ Çalıştırılmadı"* ]] || false
    [[ "$output" != *"Sıradaki adım"* ]] || false
}

@test "etkileşimli eylem: çalıştırma sonrası sonuç paneli gösterilir (numaralı modda eski akış, panel yok)" {
    export SSHPOCKET_STATE_DIR="$T/state"; UI_RULE='---'
    interactive_pick 2 0
    run comp_menu_actions demo web1 <<<""
    [[ "$output" == *"Sonuç: preview — web1"* ]] || false
    rm -f "$T/runs"; unset -f ui_arrows_enabled ui_pick_arrows
    run comp_menu_actions demo web1 <<<$'2\n\n0'
    [[ "$output" != *"Sonuç: preview"* ]] || false
}


@test "çalışma paneli (fzf): üst bölüm sabit çizilir, alt bölüm kaydırma bölgesi olur; sonuç başlığa yazılır; bölge sonda sıfırlanır" {
    export SSHPOCKET_STATE_DIR="$T/state" SSHPOCKET_ROWS=30 SSHPOCKET_COLS=80; UI_RULE='---'
    ui_fzf_enabled() { return 0; }
    interactive_pick 2 0
    run comp_menu_actions demo web1 <<<""
    [[ "$output" == *$'\033[10;30r'* ]] || { echo "$output" | cat -v | head -c 500; false; }   # alt bölge: iki sütunlu üst bölüm en az 9 satır → 10..30
    [[ "$output" == *"Sunucu › web1 › Paketler › demo › preview"* ]] || false
    [[ "$output" == *"Etkiler: yok (salt okunur)"* ]] || false    # üstte eylem kartı
    [[ "$output" == *"Çıktı: preview — web1 (çalışıyor)"* ]] || false
    [[ "$output" == *"✅ preview — web1: başarılı"* ]] || false
    [[ "$output" == *"Sonuç: preview — web1"* ]] || false          # sonuç paneli bölgede
    [[ "$output" == *$'\033[r'* ]] || false                       # bölge sıfırlandı
}

@test "çalışma paneli: küçük terminalde (<24 satır) kaydırma bölgesi kurulmaz, eski akış sürer" {
    export SSHPOCKET_STATE_DIR="$T/state" SSHPOCKET_ROWS=20 SSHPOCKET_COLS=80; UI_RULE='---'
    ui_fzf_enabled() { return 0; }
    interactive_pick 2 0
    run comp_menu_actions demo web1 <<<""
    [[ "$output" != *$'\033[13;30r'* ]] || false
    [[ "$output" != *"Çıktı: preview"* ]] || false
    [[ "$output" == *"Sonuç: preview — web1"* ]] || false
}

@test "_comp_menu_wrap: sözcük sınırından böler, devam satırlarını girintiler, joker karakter genişletmez" {
    run _comp_menu_wrap 20 "  " "bir iki uc dort bes alti yedi sekiz * dokuz"
    [ "$status" -eq 0 ]
    [ "${lines[0]}" = "bir iki uc dort bes" ]
    [ "${lines[1]}" = "  alti yedi sekiz *" ]
    [ "${lines[2]}" = "  dokuz" ]
}

@test "geçmiş kartı: bu paketin son çalıştırmaları ve son günlüğün sonu; kayıt yoksa söyler" {
    setup_state
    export SSHPOCKET_STATE_DIR="$T/state"
    _comp_menu_ctx demo web1 "$INFO"
    run _comp_menu_history_card demo web1
    [[ "$output" == *"(kayıt yok)"* && "$output" != *"Son günlüğün sonu"* ]] || false
    printf '2026-10-16T14:17:49Z|demo|1.0.0|preview|ok\n2026-10-16T14:20:00Z|baska|1.0.0|yabanci|ok\n2026-10-16T14:38:32Z|demo|1.0.0|apply|fail\n' > "$D/components.log"
    mkdir -p "$T/state/demo/web1"; printf 'satir1\nsatir2\n' > "$T/state/demo/web1/last-run.log"
    run _comp_menu_history_card demo web1
    [[ "$output" == *"10-16 14:17  preview ✅"* && "$output" == *"10-16 14:38  apply ❌"* ]] || false
    [[ "$output" != *"yabanci"* && "$output" == *"Son günlüğün sonu"* && "$output" == *"  satir2"* ]] || false
}

@test "ilerleme tablosu: önizlemede 'değişecek' (~ tahmini) ve toplam; uygulamada 'değişen'; bitmeyen alan ⏳; satır yoksa sessiz" {
    local log="$T/pg.log"
    printf '%s\n' \
      'x SSHPOCKET_PROGRESS alan=ssh sira=1/3 durum=bitti degisen=2 atlanan=1 kesin=0' \
      'x SSHPOCKET_PROGRESS alan=ufw sira=2/3 durum=bitti degisen=3 atlanan=0 uyari=1' \
      'x SSHPOCKET_PROGRESS alan=aide sira=3/3 durum=basladi bekle_tahmin=300' > "$log"
    run _comp_menu_progress_lines "$INFO" preview "$log"
    [[ "$output" == *"Alanlar (3 açık):"* && "$output" == *"✅ ssh"*"değişecek ~2 · atlanan 1"* ]] || { echo "$output"; false; }
    [[ "$output" == *"ufw"*"değişecek 3"*"⚠ uyarı 1"* && "$output" == *"⏳ aide"*"bitmedi (tahmini bekleme 300 sn)"* ]] || false
    [[ "$output" == *"Toplam değişecek: 5"* ]] || false
    run _comp_menu_progress_lines "$INFO" audit "$log"
    [[ "$output" == *"değişen 3"* && "$output" != *"Toplam"* ]] || false
    printf 'PLAY RECAP\n' > "$log"
    run _comp_menu_progress_lines "$INFO" preview "$log"
    [ -z "$output" ]
}

@test "ilerleme etiketi: çalışan alan, AIDE bekleme tahmini, hepsi bitince son alan 'tamam'; satır yoksa boş" {
    local log="$T/pg.log"
    run _comp_menu_progress_label "$log"; [ -z "$output" ]
    printf '%s\n' 'x SSHPOCKET_PROGRESS alan=ssh sira=1/3 durum=bitti degisen=1' 'x SSHPOCKET_PROGRESS alan=aide sira=2/3 durum=basladi' > "$log"
    run _comp_menu_progress_label "$log"; [ "$output" = "aide 2/3" ]
    printf '%s\n' 'x SSHPOCKET_PROGRESS alan=aide sira=2/3 durum=basladi bekle_tahmin=300' >> "$log"
    run _comp_menu_progress_label "$log"; [ "$output" = "aide 2/3 · bekleniyor ~300 sn" ]
    printf '%s\n' 'x SSHPOCKET_PROGRESS alan=aide sira=2/3 durum=bitti degisen=0' >> "$log"
    run _comp_menu_progress_label "$log"; [ "$output" = "aide 2/3 tamam" ]
}

@test "çalışma paneli: izleyici günlükteki ilerlemeyi çerçeve başlığına yazar; eski koşunun günlüğünü yok sayar; panel sonunda durur" {
    export SSHPOCKET_STATE_DIR="$T/state" SSHPOCKET_ROWS=30 SSHPOCKET_COLS=100 SSHPOCKET_WATCH_SECS=0.2
    _comp_menu_ctx demo web1 "$INFO"
    mkdir -p "$T/state/demo/web1"
    printf 'x SSHPOCKET_PROGRESS alan=eski sira=1/1 durum=basladi\n' > "$T/state/demo/web1/last-run.log"
    touch -t 202001010000 "$T/state/demo/web1/last-run.log"
    out="$T/panel.out"
    ( _comp_menu_panel_begin demo web1 "$INFO" apply "" > "$out" 2>&1
      sleep 0.6
      printf 'x SSHPOCKET_PROGRESS alan=ssh sira=4/9 durum=basladi\n' > "$T/state/demo/web1/last-run.log"
      sleep 1
      _comp_menu_panel_end apply web1 0 3 >> "$out" 2>&1 )
    grep -q "ssh 4/9" "$out"
    run grep -q "eski 1/1" "$out"; [ "$status" -ne 0 ]
}

@test "bulgular: eylem menüsünde '🔎 Bulgular (N)' görünür (Geçmiş'ten sonra), seçilince liste açılır; bulgu yoksa görünmez" {
    setup_state
    export SSHPOCKET_STATE_DIR="$T/state"
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" != *"Bulgular ("* ]] || false
    sed 's|"outputs":{"dir_var":"scan_dir"}|"outputs":{"dir_var":"scan_dir","findings":"bulgular.tsv"}|' "$T/m.json" > "$T/m2.json"
    INFO=$(comp_manifest_info "$T/m2.json" demo 1.0.0)
    mkdir -p "$T/state/demo/web1/h/t"; cp "$ROOT/test/fixtures/bulgular-ornek.tsv" "$T/state/demo/web1/h/t/bulgular.tsv"
    run comp_menu_actions demo web1 <<<$'0'
    [[ "$output" == *"Bulgular: 26 (9 tanesi paket seçeneğiyle eşleşir)"* && "$output" == *"8) 🔎 Bulgular (26)"* ]] || { echo "$output"; false; }
    run comp_menu_actions demo web1 <<<$'8\n0\n0'
    [[ "$output" == *"Yalnız eşlemeli: kapalı (26 gösterilen)"* && "$output" == *"SSH-7408"* ]] || { echo "$output"; false; }
}

# =====================================================================
# Otomatik preview teklifi ve 🧪 Kontrol
# =====================================================================
# actions listesi: 1 Değişkenler, 2 preview, 3 apply, 4 wipe, 5 audit, 6 Kontrol, 7 Geçmiş

@test "apply: preview taze değilse sorar; 'h' hiçbir şey çalıştırmaz" {
    setup_state
    run comp_menu_actions demo web1 <<<$'3\nh\n\n0'
    [ "$status" -eq 0 ]
    [[ "$output" == *"'apply' için önce 'preview' gerekli"* ]] || false
    [[ "$output" == *"Vazgeçildi: 'apply' çalıştırılmadı."* ]] || false
    [ ! -e "$T/runs" ]
}

@test "apply: preview taze değilse 'e' önce preview'ı çalıştırır, sonra devam sorusu; 'e' ile apply da çalışır" {
    setup_state
    run comp_menu_actions demo web1 <<<$'3\ne\ne\n\n0'
    [ "$status" -eq 0 ]
    [ "$(sed -n 1p "$T/runs")" = "demo preview web1" ]
    [ "$(sed -n 2p "$T/runs")" = "demo apply web1" ]
    [ "$(wc -l < "$T/runs" | tr -d ' ')" = "2" ]
}

@test "apply: preview çalıştı ama devam sorusuna 'h' denirse apply çalışmaz" {
    setup_state
    run comp_menu_actions demo web1 <<<$'3\ne\nh\n\n0'
    [ "$(wc -l < "$T/runs" | tr -d ' ')" = "1" ]
    [ "$(sed -n 1p "$T/runs")" = "demo preview web1" ]
    [[ "$output" == *"Vazgeçildi: 'apply' çalıştırılmadı."* ]] || false
}

@test "apply: önerilen preview başarısız olursa apply çalışmaz ve neden yazılır" {
    setup_state
    FAKE_RUN_RC=2 run comp_menu_actions demo web1 <<<$'3\ne\n\n0'
    [ "$(wc -l < "$T/runs" | tr -d ' ')" = "1" ]
    [[ "$output" == *"'preview' tamamlanamadı; 'apply' çalıştırılmadı."* ]] || false
}

@test "apply: preview tazeyse soru çıkmaz, doğrudan apply çalışır" {
    setup_state
    comp_gate_record demo web1 1.0.0 preview ok
    run comp_menu_actions demo web1 <<<$'3\n\n0'
    [[ "$output" != *"gerekli (yok ya da süresi dolmuş)"* ]] || false
    [ "$(cat "$T/runs")" = "demo apply web1" ]
}

@test "apply: girdi yarıda biterse (EOF) soru 'hayır' sayılır ve takılmaz" {
    setup_state
    run comp_menu_actions demo web1 <<<'3'
    [ "$status" -eq 0 ]
    [ ! -e "$T/runs" ]
}

@test "preview gibi ön koşulsuz eylem soru çıkarmadan çalışır" {
    setup_state
    run comp_menu_actions demo web1 <<<$'2\n\n0'
    [[ "$output" != *"gerekli (yok ya da süresi dolmuş)"* ]] || false
    [ "$(cat "$T/runs")" = "demo preview web1" ]
}

@test "kontrol: satır yalnız preview ve audit ikisi de varsa görünür" {
    setup_state
    run comp_menu_actions demo web1 <<<'0'
    [[ "$output" == *"6) 🧪 Kontrol (preview + audit)"* ]] || false
    comp_action_on_platform() { [ "$2" != audit ]; }                   # audit bu platformda yok
    run comp_menu_actions demo web1 <<<'0'
    [[ "$output" != *"Kontrol (preview + audit)"* ]] || false
}

@test "kontrol: preview sonra audit art arda çalışır ve tek özet yazılır" {
    setup_state
    run comp_menu_actions demo web1 <<<$'6\n\n0'
    [ "$status" -eq 0 ]
    [ "$(sed -n 1p "$T/runs")" = "demo preview web1" ]
    [ "$(sed -n 2p "$T/runs")" = "demo audit web1" ]
    [ "$(wc -l < "$T/runs" | tr -d ' ')" = "2" ]
    [[ "$output" == *"Kontrol özeti — web1"* && "$output" == *"preview: ✅ apply için hazır"* && "$output" == *"audit:   ✅ tamam"* ]] || { echo "$output"; false; }
}

@test "kontrol: preview başarısızsa audit çalıştırılmaz ve özet bunu söyler" {
    setup_state
    FAKE_RUN_RC=2 run comp_menu_actions demo web1 <<<$'6\n\n0'
    [ "$(cat "$T/runs")" = "demo preview web1" ]
    [[ "$output" == *"preview: ❌ başarısız"* && "$output" == *"preview tamamlanamadı; audit çalıştırılmadı"* ]] || { echo "$output"; false; }
}

@test "kontrol: audit reddedilirse (kapı/onay) preview sonucu korunur ve özet nedenini söyler" {
    setup_state
    comp_cli_run() { echo "$*" >> "$T/runs"; [ "$2" = audit ] && return "$COMP_EXEC_REFUSED"; return 0; }
    run comp_menu_actions demo web1 <<<$'6\n\n0'
    [ "$(wc -l < "$T/runs" | tr -d ' ')" = "2" ]
    [[ "$output" == *"preview: ✅ apply için hazır"* && "$output" == *"audit çalıştırılmadı (kapı/onay"* ]] || { echo "$output"; false; }
}

@test "kontrol: zorunlu değişken eksikse ⛔ gösterir ve hiçbir şey çalıştırmaz" {
    setup_state
    : > "$D/demo.vars"
    run comp_menu_actions demo web1 <<<$'6\n\n0'
    [[ "$output" == *"Önce zorunlu değişkenleri tamamlayın"* ]] || false
    [ ! -e "$T/runs" ]
}
