# Changelog

Biçim [Keep a Changelog](https://keepachangelog.com/) esas alınmıştır.

## [Unreleased]

### Değişti
- **Çalışma paneli iki sütunlu (fzf eylem menüsü, eylem çalışırken):** üst bölüm artık terminal genişliğinin tamamını kullanır: solda eylem özeti (ad, etkiler, onay, ön koşul), sağda son sonuç ve uyarılar; `│` ayırıcı ve fzf'e benzer kenarlık etiketleri (`Eylem`, `Son sonuç ve uyarılar`). Üst bölümün yüksekliği içeriğe göre ayarlanır (en az 9 satır, en çok terminalin %40'ı), çıktı bölgesi altta tam genişlikte kalır; çıktı çerçevesinin başlığı canlı ilerlemeyi göstermeye devam eder. `BECOME password:` ve diğer istemler alt bölgede aynen çalışır. Terminal < 24 satır ya da < 60 sütunsa eski akış kullanılır. Çizim başlangıçtaki genişliğe göre yapılır; çalışırken terminal yeniden boyutlandırılırsa üst bölüm yeniden çizilmez. UI parçası: `ui_region_begin2`, `ui_wrap_text` (`UI.sh`).

## [3.6.19] - 2026-11-01

Bu yama sürümü kullanıcıya görünür özellikler de taşır: paket menüsünde 🧪 Kontrol ve ön koşul önerisi, bulgular ekranı, canlı ilerleme, ortak arayüz parçaları ve `linux-hardening` 0.11.0.

### Dikkat
- **fail2ban davranışı değişti (`linux-hardening` 0.11.0):** `127.0.0.1/8 ::1` her zaman muaf; `hardening_fail2ban_ignoreip` varsayılanı boş ve yazdığınız adresleri EKLER. Kontrol makinesinin ve `hardening_ssh_allow_from`'un adresi artık otomatik muaf EDİLMEZ: bağlı adres muaf değilse art arda hatalı denemede (5) `bantime` (varsayılan `10m`, eskiden `1h`) kadar banlanabilirsiniz; günlükte tek satır `NOT (fail2ban)` çıkar. Yükseltmede jail dosyası bir kez yeniden yazılır (ilk koşu `changed` + fail2ban yeniden başlatması). **Gerçek VM doğrulaması (Ubuntu + Proxmox) henüz yapılmadı** (yalnızca birim/Ansible testleri ve Docker LAB e2e yeşil); ikinci apply'ın `changed=0` olması ve etkin ban davranışı sınanmadı.

### Eklendi
- **🧪 Kontrol (paket eylem menüsü):** pakette `preview` ve `audit` ikisi de bu sunucunun platformunda varsa listede yeni satır; ikisini art arda çalıştırır (önce `preview`, başarısızsa `audit` çalışmaz) ve sonunda tek özet yazar: apply'da değişecek sayısı, Lynis puanı, bulgular. Zorunlu değişken eksikse ⛔ ve hiçbir şey çalışmaz; `audit` kapıya/onaya takılırsa `preview` sonucu korunur ve özet nedenini söyler. Paket kodu değişmez, eylemlerin kendi kapıları ve onayları aynen geçerlidir. Menü satırları kayar: Geçmiş artık Kontrol'den sonra gelir.
- **Ön koşul eylemini önermek (etkileşimli menü):** `apply` gibi `requires` taşıyan bir eylem seçilince ön koşul (örn. `preview`) yok ya da süresi dolmuşsa menü "önce çalıştırılsın mı? [e/H]" diye sorar, çalıştırır, sonucu gösterir ve "devam edilsin mi?" diye ikinci kez sorar; eylemin kendi onayı (sunucu adını yazma) ayrıca aynen sorulur. Ön koşul başarısız olursa ya da "H" denirse eylem çalışmaz; girdi biterse (EOF) "hayır" sayılır. Komut satırı (`--components run`) ve kapı (`comp_gate_requires`) değişmedi: atlama değil, çalıştırma kolaylığı.
- **Denetim bulguları:** manifestte `outputs.findings` (düz `.tsv` dosya adı) bildiren paketin `bulgular.tsv` dosyası (10 sütun: kimlik, risk, tur, aciklama, ayrinti, oneri, alan, deger, kapanir_varsayilan, platform) ayrıştırılır (`component_findings.py`; geçersiz satır atlanır, en çok 2000 satır, başlık bozuksa gösterilmez). Paket menüsünde hazırlık özetinde sayı ve eşleşen bulgular, "🔎 Bulgular" ekranında önem sırasıyla liste ve kartlar (ayrıntı, öneri, paket karşılığı, "elle ayarlayın / apply yeterli"), "yalnız eşlemeli" süzgeci. Biçim: `docs/package-standard.md` "Bulgular".
- **Canlı ilerleme (fzf çalışma paneli):** paket `SSHPOCKET_PROGRESS` satırları yazıyorsa çıktı çerçevesinin başlığı çalışırken güncellenir ("Çıktı: apply — srv (çalışıyor) · aide 9/14 · bekleniyor ~300 sn"). Günlüğe saniyede bir bakan ayrı bir arka plan izleyici yapar; ana çıktı akışına dokunmaz (parola istemleri gecikmez), önceki koşunun günlüğünü yok sayar, panel bitince durur.
- **İlerleme satırı ayrıştırıcısı:** paketin `SSHPOCKET_PROGRESS alan=… sira=n/toplam durum=basladi|bitti [degisen atlanan kesin uyari bekle_tahmin]` satırları (`comp_progress_parse`) sonuç panelinde alan tablosu olur: uygulamada "değişen", önizlemede "değişecek" (+ toplam; `kesin=0` ise `~` tahmini), bitmeyen alan ⏳, uyarı sayısı. Satır yoksa panel eskisi gibidir; `SSHPOCKET_RESULT` ayrıştırıcısıyla çakışmaz. Biçim `docs/package-standard.md`'de.

### Düzeltildi
- **Python eklentili paketlerde ikinci koşu reddedilmesi:** `linux-hardening` 0.10.0'ın callback/filter eklentileri ilk koşuda kurulu paket klasörüne `__pycache__` yazıyor, bir sonraki eylem "kurulu paketin dosyaları değişmiş" bütünlük denetimine takılıyordu (gerçek Proxmox'ta ilk `audit` platform kapısında durduktan sonra ikinci deneme çalışmadı). Paket artık `PYTHONDONTWRITEBYTECODE=1` ile çalışır. Etkilenmiş kurulumda `__pycache__` klasörlerini silmek yeter.
- Test: `yürütme: TERM ve HUP` testinin sahte `ansible-playbook`'u sinyalden sonra çıktı borularını bırakır; yetim işlem `bats`'ı asmaz ve tam koşuda dalgalanmaz.
- **Aynı anda iki paket kurulumu** (örn. iki Terminal sekmesinde `--components sync`) artık `mv` yüzünden iç içe klasör (`0.9.1/linux-hardening-0.9.1`) üretmez: kurulum bölümü `mkdir` tabanlı kilitle (`<paket>/.install.lock`, sahibi ölmüşse temizlenir, doluysa ~10 sn bekler) korunur ve kilit altında "zaten kurulu mu" yeniden bakılır. Bozuk iç içe kurulumda eski davranış korunur: "kurulu paketin dosyaları değişmiş", elle silip yeniden `sync`.

### Değişti
- Paket eylem paneli kozmetiği: rozetler düz sembol (✎ / ☠; emoji genişliği sarmayı bozuyordu), kart uyarıları ve başlık özeti terminal genişliğine göre sözcük sınırından bölünür; "Geçmiş ve son sonuçlar" satırı son 5 çalıştırmayı ve son günlüğün sonunu sağ bölmede gösterir.
- **Ortak arayüz parçaları (`UI.sh`):** menülerdeki seçici (`ui_pick`), kaydırma bölgeli ekran (`ui_region_begin/end/reset`, `ui_rule_label`) ve genel liste editörü (`ui_edit_list`) paket menüsünden çıkarıldı; `comp_menu_edit_list` artık yalnızca CIDR/paket geri çağrımlarını verir. Davranış değişmedi. `UI_PICK_*` ve `UI_LIST_*` bağlam değişkenleri `docs/ui-design.md` §5b'de belgelendi (yeni paketler, örn. `utility`, aynı altyapıyı kullanır). Paketten bağımsız yeni testler.
- **Eylem paneli (fzf):** eylem listesinde hazırlık özeti başlıkta, seçili eylemin kartı (etkiler, onay, ön koşul, kapı durumu, son sonuç, uyarılar) sağ bölmede; eylem bitince sonuç paneli (durum, süre, `ok/changed/failed` özeti, sonuç anahtarları, günlük yolu, sıradaki adım). Terminal ≥ 24 satır ve 60 sütunsa çalışma sırasında üst bölüm sabit kalır, alt bölüm kaydırma bölgesi olur: onay soruları, `BECOME password:` istemi ve çıktı aynı ekranda akar, `Ctrl-C` bölgeyi sıfırlar. Küçük terminalde ve numaralı modda eski akış. Çok satırlı açıklama desteği eklendi.
- **Değişken düzenleme (etkileşimli modda, ok tuşlu/fzf):** `bool` ve `enum` için seçici liste (izinli değerler, imleç şimdikinin tersinde, sağda açıklama, riskli değer işaretli, "Varsayılana dön"); `list[string]`, `list[int]` ve `list[cidr]` için liste editörü (öğeler liste, Enter ile sil/değiştir, ➕ Ekle, CIDR için öneri ve sunucunun gördüğü adres, 🧹 Temizle, 💾 Kaydet; Vazgeç dosyayı değiştirmez); `int`/`string` yanlış değerde ekran kapanmadan 3 deneme. Kayıt sonrası "Devam için Enter" duraksaması yok, ileti değişken listesinde görünür. `describe` çıktısına 10. alan (enum değerleri) eklendi. Numaralı modun akışı değişmedi.
- **fzf geniş düzen (Değişkenler ekranı):** yol çubuğu kenarlık etiketi (`Sunucu › … › Değişkenler`), üstte grup ve sayı özeti, sağda açıklama bölmesi (fzf ≥ 0.35 etiketli kenarlık, ≥ 0.58 etiketli liste/giriş çerçeveleri; eski fzf'te yalnız sağ önizleme), `Ctrl-O` ile grup seçimi. fzf yoksa ya da `UI=numbers|arrows` ise bugünkü menüler aynen çalışır. Diğer ekranlar değişmedi.
- **`components.lock`: `linux-hardening` 0.10.0 → 0.11.0** (sha256 `5ed26848…7f0e`; 128 değişken, ekleme/silme yok). fail2ban `ignoreip` sadeleşti: `127.0.0.1/8 ::1` kodda her zaman muaf, `hardening_fail2ban_ignoreip` varsayılanı artık boş ve yazılan adresleri EKLER (ezmez); kontrol makinesinin ve `hardening_ssh_allow_from`'un adresi otomatik eklenmez, bağlı adres muaf değilse günlükte tek satır `NOT (fail2ban)` çıkar. `hardening_fail2ban_bantime` varsayılanı `1h` → `10m`. Yükseltmede önceden otomatik eklenen adres jail dosyasından düşer: ilk koşu `changed` ve bir fail2ban yeniden başlatması gösterir. Doğrulama: kilit kuru koşusu (sha256 + imza ad alanı) ve 128 değişken denetimi geçti; **fail2ban değişikliği gerçek VM'de (Ubuntu + Proxmox) henüz doğrulanmadı**: ikinci apply'ın `changed=0` olması ve etkin ban davranışı sınanmadı.
- **`components.lock`: `linux-hardening` 0.9.1 → 0.10.0** (sha256 `6921e52c…0c8b`). Paket artık `SSHPOCKET_PROGRESS` satırlarını (14 alan, AIDE'de `bekle_tahmin=300`) ve `audit` sonrası `bulgular.tsv` dosyasını (0600, `summary.json`'ın yanında) üretir; ikisi de Docker LAB'da gerçek koşuyla doğrulandı (preview, apply, audit: 22 bulgu = 20 öneri + 2 uyarı). Gerçek VM doğrulaması sürüyor.
- **`components.lock`: `linux-hardening` 0.9.0 → 0.9.1** (128 değişken, 11 eylem; 0.9.0 gerçek VM turunun bulgularını düzeltir: `kaldir` sahiplik testi, modprobe uyarı metni, `hardening_ssh_allow_from` artık `required` değil). Gerçek VM doğrulaması sürüyor.
- Sunucu adı onayı (`type-server-name`) yazım hatasında 3 deneme verir (her yanlışta `1/3`, `2/3` bildirimi); ad yine birebir yazılmalı, girdi bitince (EOF) hemen vazgeçilir.
- Ansible `Incorrect sudo password` / `Missing sudo password` ile düşerse sudo parolasının reddedildiğini ve komutun yeniden çalıştırılabileceğini söyleyen bir not gösterilir.

## [3.6.18] - 2026-10-30

`linux-hardening` 0.9.0 kilitlendi (alan anahtarları, kalıcı kaldırma eylemleri); `runner: script` sudo'lu yolda sonuç ayrıştırma (CRLF) ve `validate` script/Türkçe kurtarma adı düzeltmeleri.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.4 → 0.9.0** (128 değişken, 11 eylem; tüm değişkenler gruplu). Yeni: 13 alan anahtarı (`hardening_<alan>_enabled`, hepsi varsayılan açık), kalıcı kaldırma eylemleri `kaldir_onizle` (salt okunur, `SİLİNECEK:` satırları), `kaldir` (sunucu adıyla onay) ve `kaldir_geri_yukle`. Ubuntu'da 118 değişken ve 9 eylem, Proxmox'ta 128 değişken ve 11 eylem görünür. Gerçek VM doğrulaması sürüyor.
- `validate`: kurtarma eylemi adı artık adın ya da `_` sonrası sözcüğün başında `rollback|recover|restore|undo|geri` içeriyorsa tanınır (`kaldir_geri_yukle` gibi Türkçe adlar `--strict`'te yanlış uyarı vermez).

### Düzeltildi
- **`runner: script` sudo'lu yolda sonuç satırı ayrıştırılamıyordu:** `ssh -t` çıktıdaki satır sonlarını CRLF yapıyor, son anahtarın değeri `7\r` olarak okunup "Sonuç satırı geçersiz" deniyordu (eylem çalışıyor ama sonuç kaydedilmiyordu). Ayrıştırıcı artık `\r`'yi atar; gerçek makinede (ubuntutest) bulundu, regresyon testi eklendi.
- **`validate` script paketlerinde yanlış uyarı vermez:** sonuç satırı (`SSHPOCKET_RESULT …`) ve değişken kullanımı artık `scripts/` klasöründe de aranır; değişken betikte `SSHPOCKET_VAR_<AD>` olarak okunuyorsa "kullanılmıyor" uyarısı çıkmaz.

## [3.6.17] - 2026-10-29

`runner: script`: paketler Ansible yerine hedef sunucuda çalışan betiklerle eylem tanımlayabilir (sudo'lu yolda root'a ait kopya + SHA-256 doğrulama).

### Eklendi
- **`runner: script`:** paketler Ansible yerine `scripts/<ad>.sh` betikleriyle eylem tanımlayabilir. Betik **hedef sunucuda** çalışır (kontrol makinesinde yabancı kod çalışmaz): sudo gerekmiyorsa ssh stdin'inden akar; `requires_sudo` ise sunucuda 0600 geçici dosyaya yazılır, `ssh -t` ile tek sudo oturumunda **root'a ait 0700 klasöre kopyalanıp SHA-256'sı yerelde hesaplanan özetle doğrulanır** (uyuşmazsa çalışmaz; yükleme ile sudo arasındaki değiştirme yarışı kapalı) ve çalıştırılır; sudo parolası aynı terminalde sudo isteminde girilir, dosyalar her çıkışta silinir. Değişkenler `SSHPOCKET_VAR_<AD>` olarak tek tırnakla kaçırılarak geçer; sonuç `SSHPOCKET_RESULT <id>.<eylem> k=v` satırıyla gelir. Ortak kapılar (host key, ajan, profil, root reddi, önkoşul, onay) ansible ile aynıdır; Python ortamı gerekmez. Manifest doğrulayıcı ve `validate` betik yolunu/varlığını denetler. Standart §7 güncellendi.

## [3.6.16] - 2026-10-28

Değişkenler ekranında arama/grup/"değiştirdiklerim" süzgeci; saat sapması uyarısı iki saati, farkı, saat dilimini ve hazır düzeltme komutunu gösterir; `linux-hardening` 0.8.4 gerçek VM doğrulaması belgelendi.

### Değişti
- **Saat sapması uyarısı daha açıklayıcı** (`--components run`, 60 sn'den büyük fark): sunucu ve bu bilgisayarın saati (UTC), fark, sunucunun saat dilimi, NTP eşitleme durumu (`NTPSynchronized=no` ise kalıcı olmayabileceği), TOTP/2FA etkisi ve **kopyalanabilir hazır düzeltme komutu** gösterilir. Fark tam saat dilimi farkına benziyorsa ipucu verilir. Uzak komut hâlâ sabit ve salt okunurdur (`date +%s; timedatectl show -p Timezone -p NTPSynchronized`); gelen satırlar katı desenle okunur. Saati sshpocket düzeltmez (saat sıçrar, karar kullanıcıda); uyarı engellemez.

### Eklendi
- **Değişkenler ekranı: arama, grup ve "değiştirdiklerim" süzgeci.** Bir paketin 25'ten fazla değişkeni varsa liste başında `🔎 Ara`, `📂 Grup` (pakette `group` varsa) ve `✏️ Yalnızca değiştirdiklerim` satırları çıkar; arama ad ve açıklamada büyük/küçük harf ayırmadan çalışır, süzgeç açıkken `↩ Süzgeci temizle` görünür ve başlıkta `Gösterilen: X / Y · bu sunucuda değiştirilen: N` yazar. Dosyada değeri yazılı satırlar `✏️` ile işaretlenir; uzun değerler 40 karaktere kısaltılır (düzenleme ekranında tam değer görünür). Küçük paketlerde numaralandırma eskisi gibidir.

### Belgeler
- **`linux-hardening` 0.8.4 gerçek VM doğrulaması** (Ubuntu 26.04 ve Proxmox VE 9) README ve `docs/hardening-kullanim.md`'ye işlendi: platform süzmesi, altı sunucu başına anahtar (`false` yazmaz, önceden yazılanı silmez), `pve_keep_banner` (yeniden başlatma sonrası banner korunur), yeniden tasarlanan `pve_web_exclusive` (8006/3128 listede olmayan aynı alt ağ istemcisinde kapanır), ek güvenlik duvarı portu. `pve_web_exclusive` için eski "beklendiği gibi çalışmıyor" uyarısı kaldırıldı.

## [3.6.15] - 2026-10-28

Eylemlerde `platforms` (Ubuntu sunucuda `pve_firewall_*` görünmez); `linux-hardening` 0.8.4 kilitlendi; değişken açıklama sınırı 2000; sürüm yayını için `scripts/release-publish.sh`.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.3 → 0.8.4** (116 değişken, 8 eylem). Paket `platform_var: hardening_platform` ile 9 Proxmox değişkenini (`group: Proxmox`) ve `pve_firewall_preview/apply` eylemlerini yalnızca `proxmox-host` platformunda gösterir (Ubuntu'da 107 değişken, 6 eylem görünür; gizli değişkenin yazılı değeri yine çalıştırmaya iletilir). 6 yeni koşulsuz-görev anahtarı (varsayılan açık) ve `hardening_pve_keep_banner`. `pve_web_exclusive` yeniden tasarlandı. sshpocket: değişken açıklama sınırı 1000 → 2000 karakter (paketteki `pve_web_exclusive` açıklaması 1182 karakter). Gerçek VM doğrulaması sürüyor.

### Eklendi
- **Eylemlerde `platforms` (isteğe bağlı):** manifestte bir eyleme `"platforms": ["proxmox-host"]` yazılırsa eylem yalnızca o platformda listelenir; diğer sunucularda menüden gizlenir ("N eylem … gizlendi" notu) ve CLI'dan çalıştırılması ssh'a gitmeden reddedilir. Ön koşul tutarlılığı manifest doğrulamasında denetlenir. Eski sürümler alanı yok sayar. Örnek: Ubuntu sunucuda `pve_firewall_*` görünmez.
- **`scripts/release-publish.sh`:** sürüm yayınını tek komuta indirir. Ön kontroller (main, temiz, güncel, CHANGELOG başlığı, imza anahtarı, Release yok, `main` CI'ı bu commit için yeşil) → etiket → `release.sh` → `gh release create` (notlar CHANGELOG'dan) → yüklenen dosyaların SHA-256 doğrulaması. `--dry-run` yalnızca planı gösterir. Elle yol `RELEASING.md`'de yedek olarak durur.

## [3.6.14] - 2026-10-27

Sunucu menüsünde denetim puanı; değişkenler ekranı platforma göre süzülür ve gruplanır (isteğe bağlı); USB çıkarılırken anahtarlar `ssh-agent`'tan çıkarılır; eski paket sürümleri için temizleme sorusu ve Ayarlar menüsü; Ansible `become` bekleme süresi 30 sn; `warn_if` sorusu yalnızca değiştiren eylemlerde.

### Eklendi
- **Sunucu menüsünde denetim puanı:** 🛡️ Paketler › paket ekranında araç başına son denetim puanı ve bir öncekiyle fark gösterilir (`📊 Lynis: 83 (2 gün önce) · önceki 61 (+22)`). Yalnızca puan. Ortak `summary.json` biçimi (`tool`, `date`, `score`; eski `hardening_index` desteklenir) `docs/package-standard.md`'de tanımlandı; OpenSCAP gibi yeni bir araç eklemek sshpocket'te kod değişikliği istemez.
- **Değişkenler ekranı platforma göre süzülür ve gruplanır (isteğe bağlı manifest alanları):** üst düzey `platform_var`, değişkende `platforms` ve `group`. Ubuntu sunucuda Proxmox'a özgü değişkenler gizlenir, kaç değişkenin gizlendiği yazılır; gruplar `[Proxmox]` etiketiyle yan yana gelir. Gizlenen değişkenin yazılı değeri ve `required` kuralı etkilenmez; alanları olmayan paketler (bugünkü linux-hardening) eskisi gibi görünür. Paketin bu alanları kullanması ayrı bir iş (`docs/package-standard.md` §6).

### Değişti
- **Eski paket sürümleri artık hatırlatılıyor:** menüden güncelleme (ve paketlerin eşitlenmesi) bittikten sonra `prune` listeyi gösterir ve silinmesini sorar (varsayılan: silme); ayrıca **Ayarlar › 4) Eski paket sürümlerini temizle** menüsünden her zaman yapılabilir. Etkin, bir önceki ve kilitteki sürüm kalır.
- Sunucu saati farkı uyarısına paket seçeneği ipucu eklendi: `linux-hardening` için `hardening_time_makestep=true` (apply saati düzeltir, varsayılan kapalı). `docs/hardening-kullanim.md` ilgili bölüm genişletildi.
- **USB'yi menüden çıkarırken USB'deki anahtarlar `ssh-agent`'tan çıkarılır** (`EjectUSB`, çıkarmadan önce): USB'deki `servers/*/*.pub` dosyalarına karşılık gelen anahtarlar `ssh-add -d` ile silinir (parola sorulmaz); ajandaki başka anahtarlara dokunulmaz. Böylece çözülmüş anahtar bilgisayarın belleğinde kalmaz. USB menüden çıkarılmadan çekilirse çalışmaz.
- `--components run`: Ansible bağlantı/`become` bekleme süresi 10 sn'den **30 sn**'ye çıkarıldı (`ANSIBLE_TIMEOUT`, kullanıcının kendi değeri korunur). Gerçek Proxmox VM'inde zaman zaman görülen "Timeout (12s) waiting for privilege escalation prompt" için.
- **"Riskli değer" (`warn_if`) sorusu yalnızca yapılandırmayı değiştiren eylemlerde sorulur** (`apply` vb.). `preview`, durum eylemleri ve yalnızca araç kuran eylemler (`audit`) artık sormaz; eskiden `preview` ve `audit` her çalıştırmada, ardından `apply` yine sorduğu için aynı soru 2-3 kez çıkıyordu. Zorunlu değişken kuralı (`preview` dahil) aynı kaldı.

### Doğrulandı (belge)
- `linux-hardening 0.8.3`, gerçek VM'lerde (Ubuntu 26.04 ve tek düğümlü Proxmox VE 9, boş sunucular): varsayılanlarla Lynis Ubuntu 61→83, Proxmox 62→83 (84 `hidepid` ile); `kernel_modules_disabled` Ubuntu'da yeniden başlatma sonrası `ufw` açık, Proxmox'ta reddedilir; `fail2ban_pve_web`, `pveproxy_allow_from`, `postfix_banner`, `proc_hidepid`, `apt_listbugs` çalıştı. **Bilinen sorunlar:** `hardening_pve_web_exclusive` aynı alt ağdan 8006'yı kapatmıyor; Proxmox yeniden başlatmada `pvebanner.service` `/etc/issue` bandını eziyor; Proxmox'ta hidepid sonrası iki kez geçici "Timeout (12s) waiting for privilege escalation prompt". Denenmedi: misafir VM/CT'li Proxmox, Docker'lı VM, Raspberry Pi, HA kümesi.
- `docs/hardening-kullanim.md`: "Yeni seçenekler (0.8.x) ve gerçek VM notları" bölümü; README (TR/EN) kilit paragrafı güncellendi.

## [3.6.13] - 2026-10-26

`linux-hardening` 0.8.3 kilitlendi (7 yeni menü değişkeni; gerçek VM doğrulaması bu sürümden sonra).

### Değişti
- **`components.lock`: `linux-hardening` 0.8.2 → 0.8.3** (109 değişken, 8 eylem). Yeni (7): `hardening_postfix_banner` (varsayılan AÇIK, Proxmox'ta postfix banner'ı), `hardening_proc_hidepid` (`warn_if`) ve `hardening_proc_hidepid_users`, `hardening_apt_upgrade_phased` (`warn_if`), `hardening_apt_listbugs`, `hardening_pve_web_allow_from` ve `hardening_pve_web_exclusive` (`warn_if`). `hardening_pve_pveproxy_allow_from=[]` blok tek başınaysa artık dosyayı siler. Gerçek VM doğrulaması bu satırdan sonra yapılır.

## [3.6.12] - 2026-10-25

`linux-hardening` 0.8.2 kilitlendi (yalnızca Proxmox: `kernel_modules_disabled` reddi, yeni `fail2ban_pve_web` ve `pve_pveproxy_allow_from`; gerçek Proxmox VM doğrulaması bu sürümden sonra); paket öncesi sunucu saati farkı uyarısı.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.1 → 0.8.2** (102 değişken, 8 eylem; yalnızca Proxmox tarafı değişti). `hardening_kernel_modules_disabled` Proxmox VE hostunda artık reddedilir (veth/KVM/bridge/softdog yüklenemediği için LXC ve HA bozuluyordu; gerçek VM'de görüldü). Yeni: `hardening_fail2ban_pve_web` (varsayılan KAPALI, `warn_if`: web arayüzü 8006 için fail2ban jail'i) ve `hardening_pve_pveproxy_allow_from` (liste; 8006'yı yalnızca verilen CIDR'lere açar). Gerçek Proxmox VM doğrulaması bu satırdan sonra yapılır.

### Eklendi
- `--components run`: sunucu saati yerelden 60 sn'den fazla farklıysa (örn. snapshot geri alındıktan sonra) engellemeden uyarır; apt "Not live until" hatasına karşı `sudo chronyc makestep` önerir. Salt okunur sabit `date +%s` sorgusu; okunamazsa sessiz geçer.

## [3.6.11] - 2026-10-25

`linux-hardening` 0.8.1 kilitlendi (`kernel_modules_disabled` için modül ön yükleme düzeltmesi paket tarafında yapıldı; gerçek VM doğrulaması bu sürümden sonra); `validate` kurtarma ve araç kuran eylemlerde ön koşul uyarısı vermez.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.0 → 0.8.1.** `hardening_kernel_modules_disabled` artık `/etc/modules-load.d/60-hardening-preload.conf` yazar (yüklü modüller + netfilter + `binfmt_misc`); 0.8.0'da yeniden başlatmada `ufw` açılmıyor, güvenlik duvarı sessizce kapalı kalıyordu. `verify_access` için `ssh-agent`'ta çok anahtar ipucu; `hardening_ssh_max_sessions` (1-10) ve `hardening_auditd_backlog_limit` (min 0) sınır taşır; `rollback_disarm` armed değilken `changed=0`.
- `validate`: "salt okunur preview ön koşulu yok" uyarısı artık yalnızca yapılandırma değiştiren eylemler için; kurtarma eylemleri (`rollback_*`, `recover*`, `restore*`, `undo*`) ve yalnızca `installs-package` etkisi olanlar (örn. `audit`) muaf. `linux-hardening` artık `--strict` altında temiz geçer.

## [3.6.10] - 2026-10-24

`linux-hardening` 0.8.0 kilitlendi (`pve_firewall_preview` artık hiçbir şey yazmaz); passphrase istemi açıklaması; sunucu silme sonrası gereksiz uyarı giderildi.

### Değişti
- Passphrase Keychain'e kaydedilirken (yeni sunucu, Keychain menüsü, yanlış passphrase düzeltme) macOS'un "password data for new item:" istemi öncesinde açıklanır: bu, SSH anahtarının PASSPHRASE'idir (sudo parolası değil), yazarken görünmez ve doğrulama için bir kez daha sorulabilir.
- **`components.lock`: `linux-hardening` 0.7.0 → 0.8.0** (100 değişken, 8 eylem). **`pve_firewall_preview` düzeltmesi:** 0.7.0'da bu eylem, sunucu değişkenlerinde `hardening_pve_apply=true` iken Proxmox'ta `cluster.fw`'yi GERÇEKTEN yazıyordu (gerçek Proxmox VM'inde görüldü); 0.8.0'da komut `pve-firewall-onizleme.yml --check` olur ve yazma kapısı değişkenle açılamaz. Yeni menü değişkenleri (18): varsayılan AÇIK `hardening_debsums` (+`_cron_check`), `hardening_apt_show_versions`, `hardening_process_accounting`, `hardening_fail2ban_jail_local`, `hardening_ufw_log_martians`; varsayılan KAPALI (çoğu `warn_if`) `hardening_apt_upgrade`, `hardening_kernel_modules_disabled`, `hardening_auditd_conf_cis`, `hardening_auditd_disk_full_halt`, `hardening_auditd_backlog_limit`, `hardening_auditd_immutable`, `hardening_aide_audit_tools`, `hardening_ssh_max_sessions`, `hardening_ufw_outgoing_deny`, `hardening_restrict_compilers`, `hardening_apparmor_enforce`, `hardening_malware_scanner`. Kapatınca GERİ ALINMAYANLAR: auditd.conf değerleri, GRUB satırı, derleyici izinleri, AppArmor kipi, ufw "ALLOW OUT" kuralları; `-e 2` ve `kernel_modules_disabled` yeniden başlatmaya kadar sürer.

### Düzeltildi
- Sunucu silme/ekleme sonrası menü yeniden yüklenirken, eski `sistem/Modules` klasörü olmayan kurulumlarda ekrana `Warning: Autoload path does not exist` yazılıyordu; klasör yoksa artık sessiz (`Main.sh` ile aynı koruma).

### Eklendi
- Docker LAB e2e: **Proxmox benzeri LAB** testi (`/etc/pve` + `pveversion` işaretleri, `hardening_pve_apply=true`): `pve_firewall_preview` hiçbir şey yazmaz, `pve-firewall restart` çalışmaz, güvenlik ağı arm edilmez, eylem komutunda `--check` var ve `-e/-i/--limit` yok. 0.7.0 kilidiyle elle denendi: kural dosyasını yazıyor (test bunu yakalar). `drive.exp` riskli değer sorusunu `E2E_RISKY` ile cevaplayabilir.

### Bilinen sorun
- ssh-agent'ında 3'ten fazla anahtar varsa, sertleştirme sonrası paketin "yeni SSH bağlantısı" doğrulaması `Too many authentication failures` ile düşer (sertleştirme `MaxAuthTries 3` yapar, doğrulayan `ssh` agent'taki tüm anahtarları dener). Güvenlik ağı sshd/ufw'yi geri alır ama paketler, sysctl, auditd, fail2ban ve AIDE kalır. Geçici çözüm: `ssh-add -D` (sshpocket gereken anahtarı yeniden ekler). Gerçek Ubuntu VM'inde 5 anahtarla görüldü.

## [3.6.9] - 2026-10-23

Ubuntu 26.04'te parolalı sudo ile paket çalıştırma düzeltildi (sudo-rs); README gerçek VM doğrulamalarını yansıtıyor.

### Doğrulandı (belge)
- `linux-hardening 0.7.0`, sshpocket 3.6.8 ile USB'den **taze Ubuntu 26.04.1 VM'inde**: `audit` (Lynis **58**) → `preview` (`changed=34`) → `apply` (`ok=168 changed=43 failed=0`; güvenlik ağı, ufw, ssh, yeni bağlantı doğrulaması, `/tmp` `noexec`) → bağımsız kontrol → **ikinci `preview` `changed=0`** (idempotent) → `audit` (Lynis **76**, öneri 46 → 23). Parolalı sudo-rs sorunu yeniden üretildi ve düzeltildi (aşağıda). Proxmox VE 9.2.21 VM'inde (0.6.0, `proxmox-host`): `apply` `ok=133 failed=0`, Lynis **82**, web arayüzü ve CT oluşturma çalışıyor. **Denenmedi:** gerçek Proxmox'ta `pve_firewall_preview`/`pve_firewall_apply`, Ubuntu'da `rollback_now`/`rollback_disarm` (yalnızca LAB), kümeli Proxmox ve gerçek donanım.

### Düzeltildi
- **Ubuntu 26.04: parolalı sudo ile paket çalıştırma** artık takılmıyor. Varsayılan `sudo-rs`, Ansible'ın parolalı `become`'ında `Timeout (12s) waiting for privilege escalation prompt` ile düşüyordu (gerçek VM'de yeniden üretildi: `UNREACHABLE`). Ubuntu profillerinde sunucuda `/usr/bin/sudo.ws` (klasik sudo) çalıştırılabilirse, envantere `ansible_become_exe=/usr/bin/sudo.ws` yazılır (salt okunur, sabit bir ssh sorgusuyla tespit edilir; envantere yalnızca tam bu yol yazılabilir). NOPASSWD sudo etkilenmez, Ubuntu dışı profillerde sorgu yapılmaz. Sudo istemi zaman aşımında nedeni açıklayan bir ipucu da gösterilir.

## [3.6.8] - 2026-10-23

Paket sürümü güncellemesi: `linux-hardening` 0.7.0 kilitlendi; Proxmox VE güvenlik duvarı için `pve_firewall_preview` ve `pve_firewall_apply` eylemleri.

### Değişti
- **`components.lock`: `linux-hardening` 0.6.0 → 0.7.0** (82 değişken; yeni: `hardening_auditd_cis_l2` (varsayılan KAPALI, `warn_if`), `hardening_motd_disable_scripts`). Yeni eylemler: **`pve_firewall_preview`** (salt okunur) ve **`pve_firewall_apply`** (`type-server-name`, önce önizleme şart): Proxmox VE (tek düğüm) datacenter güvenlik duvarını yönetir; Proxmox olmayan hostta hiçbir şey yapmadan nedenini söyleyip durur (LAB e2e ile doğrulandı). `pve_firewall_apply` yazma onayını playbook içinden (`pve-firewall-uygula.yml`) taşır; sshpocket'in `set` özelliğine gerek duymadı. 82 değişkenin tamamı için tür, 31 `warn_if` ve 30 min/max sınırı doğrulandı. **Gerçek Proxmox VM'inde `pve_firewall_*` henüz denenmedi.**

## [3.6.7] - 2026-10-23

Paket sözleşmesine eylem düzeyi `set` (sabit değişkenler) eklendi: aynı playbook'un önizleme ve gerçek koşu eylemleri kullanıcı ayarından bağımsız olur (Proxmox güvenlik duvarı eylemleri için ön koşul).

### Eklendi
- **Paket sözleşmesi: eylem düzeyi `set`** (docs/package-standard.md §5): bir eylem, bildirimde tanımlı bool/int/enum/string/path değişkenleri kullanıcı değerinin üstüne **sabitleyebilir**. Böylece aynı playbook'un önizleme ve gerçek koşu eylemleri (örn. Proxmox güvenlik duvarı için `pve_firewall_preview` → `hardening_pve_apply=false`, `pve_firewall_apply` → `true`) kullanıcının `.vars` ayarından bağımsız olur. Bildirim doğrulaması bilinmeyen/yanlış türlü/aralık dışı/liste değeri ve `outputs.dir_var` çakışmasını reddeder.

## [3.6.6] - 2026-10-22

Yeni, henüz değişkenleri tanımlanmamış bir sunucuda salt okunur durum eylemleri (`rollback_status`) artık hem komut satırında hem Paketler menüsünde çalışır.

### Düzeltildi
- Paket çalıştırma: **zorunlu değişken** (örn. `hardening_ssh_allow_from`) denetimi artık yalnızca değiştiren eylemlerde ve başka bir eylemin ön koşulu olan eylemlerde (`preview`) yapılır. Salt okunur durum eylemleri (`rollback_status`) yeni, henüz değişkenleri tanımlanmamış bir sunucuda da çalışır. Verilmiş geçersiz değer ve pakette tanımsız ad yine reddedilir; yalnızca "eksik" hoşgörülür. Sunucu menüsündeki 🛡️ Paketler ekranı da aynı kuralı izler: zorunlu değişken eksikken salt okunur durum eylemleri açık, `preview`/`apply`/`audit` ⛔ kapalı.

## [3.6.5] - 2026-10-22

Paket sürümü güncellemesi: `linux-hardening` 0.6.0 kilitlendi (Raspberry Pi / Proxmox VE platform desteği); `validate` ve paket bildirimi düzeltmeleri.

### Değişti
- **`components.lock`: `linux-hardening` 0.4.0 → 0.6.0** (80 değişken: 43 bool, 15 int, 10 string, 8 list[string], list[cidr], list[int], 2 path; Raspberry Pi/Proxmox VE platform desteği: `hardening_platform`). Docker LAB'da 16 e2e testin tamamı geçti; ikinci koşu `changed=0`; eski `ubuntu` grubu ve `INJECT_FACTS` uyarıları kayboldu. 80 değişkenin tamamı için tür, 30 `warn_if` ve 30 min/max sınırı doğrulandı. **Gerçek VM'de 0.6.0 sshpocket üzerinden denenmedi** (0.4.0 denendi).
- **Profil ve Proxmox:** paket 0.6.0 `debian-13`'ü destekler; Proxmox VE 9 ve Raspberry Pi OS de `debian-13` göründüğünden sshpocket'in profil kapısı artık Proxmox hostunu **geçirir**. Koruma paketin preflight'ındadır: Proxmox hostu `generic` platformla hiçbir bayrakla (`hardening_allow_unsupported` dahil) sertleştirilmez, `--check`'te de durur; gerçek değişiklik için `hardening_platform=proxmox-host` ve `hardening_pve_apply=true` gerekir. `docs/hardening-kullanim.md`'ye "Platformlar" bölümü eklendi.

### Düzeltildi
- `components validate` (dizin modu): gitignore'lu geliştirme klasörleri (`.venv`, `venv`, `.tox`, `.pytest_cache`, `.mypy_cache`, `.ruff_cache`) atlanır ve bildirilir; yazarın çalışma klasöründeki `.venv` yüzünden çıkan sahte ❌ (sembolik bağ, özel anahtar bloğu) giderildi. **Arşiv modu atlamaz**: yayın arşivinde bu klasörler varsa yine ❌.
- Paket bildirimi: değişken `description` sınırı 500 → 1000 karakter (linux-hardening 0.6.0'daki `hardening_platform` açıklaması 565 karakter olduğundan kilide yazılamıyordu). Kontrol karakterleri ve diğer metin alanlarının 500 sınırı aynı.


## [3.6.4] - 2026-10-20

`--components prune` eklendi; linux-hardening 0.4.0 gerçek VM'de, profil kapısı gerçek Proxmox VE hostunda denendi.

### Doğrulandı
- **Profil kapısı gerçek bir Proxmox VE 9 hostunda (Debian 13) denendi:** `--components run linux-hardening rollback_status <proxmox>` → `Profil 'debian-13' bu paketin profilleri arasında yok.`, Ansible başlamadı, sunucuya hiçbir şey yazılmadı. Ubuntu 26.04 için yazılmış sertleştirme Debian/Proxmox'a uygulanmaz. Not: Proxmox'a özgü bir platform denetimi yok (`/etc/pve`); engel "Debian desteklenmiyor" mantığına dayanır, Debian profili eklenmeden önce platform denetimi eklenmelidir.

### Eklendi
- **`./start.sh --components prune [--dry-run] [--yes]`:** paket güncellemelerinde yan yana biriken eski sürüm klasörlerini temizler. **Kalanlar:** etkin sürüm (`current`), bir önceki sürüm (geri dönüş için) ve kilitteki sürüm. Yalnızca `<X.Y.Z>` klasörü ve `<X.Y.Z>.files` silinir; `.bozuk-*` (kenara alınmış bozuk kurulum), sembolik bağlar ve geçersiz adlı klasörlere dokunulmaz. Silmeden önce onay ister (`--yes` ile atlanır); `--dry-run` yalnızca listeler. `current`'ı olmayan pakete dokunmaz.

### Doğrulandı (belge)
- `linux-hardening 0.4.0`, sshpocket 3.6.3 ile USB'den gerçek bir Ubuntu 26.04.1 VM'inde **taze kurulumda** denendi: `audit` (Lynis hardening_index 62, 0 uyarı) → `preview` → `apply` (ok=131 changed=38 failed=0). Hardening sonrası yeni SSH bağlantısı doğrulaması geçti, güvenlik ağı disarm edildi; ufw, sshd, sysctl (`kernel.sysrq=0`, `fs.protected_fifos=2`), `/etc/sudoers.d 0750`, `libpam-tmpdir`, `apt-listchanges`, AIDE zamanlayıcısı ve postfix yokluğu bağımsız kontrol edildi. `warn_if` onayı çıkmadı (linux-hardening#74 düzeltmesi gerçek akışta çalışıyor). Denenmedi: gerçek VM'de 0.3.0 üzerine ikinci koşu, `rollback_now`/`rollback_disarm`, menü ve Terminal açma.

## [3.6.3] - 2026-10-19

Paket sürümü güncellemesi: `linux-hardening` 0.4.0 kilitlendi.

### Değişti
- **`components.lock`: `linux-hardening` 0.3.0 → 0.4.0** (Lynis ölçümünden küçük maddeler, salt okunur raporlar: root ortamı, `/tmp` ve `/dev/shm`, AIDE zamanlayıcı; `su` kısıtı ve Docker `daemon.json` seçenekleri varsayılan KAPALI; değişken sayısı 27 → 38; `warn_if` apport/motd'dan kaldırıldı, linux-hardening#74 kapandı). Davranışı değiştiren yeni varsayılan-AÇIK ayarlar paketin CHANGELOG'unda (`kernel.sysrq=0`, `libpam-tmpdir`, `/etc/sudoers.d 0750` vb.). Docker LAB'da 16 e2e testin tamamı geçti. **Gerçek VM'de 0.4.0 sshpocket üzerinden denenmedi** (0.3.0 denendi).
- Docker LAB varsayılan değişkenlerinden `hardening_disable_apport=false` ve `hardening_disable_update_notifier_motd=false` kaldırıldı (artık gerekmiyor).

### Eklendi (belge ve test)
- README (TR/EN): **"İlk indirmeyi doğrulama"**: `--update` yalnızca sonraki sürümleri doğrular; ilk indirilen arşivi `ssh-keygen -Y verify` ile elle doğrulama tarifi ve anahtarın parmak izi. Bir test README'deki anahtar/parmak izinin `sistem/release_signers` ile aynı kaldığını denetler.
- Test: paket kurulumunda **güvenilen anahtarla ama yanlış ad alanıyla** (`sshpocket-release`) imzalı paket reddedilir; kilitte `signers` satırı boş/bozuksa hiçbir şey kurulmaz.

### Doğrulandı (belge)
- `linux-hardening 0.3.0`, sshpocket 3.6.2 ile USB'den gerçek bir Ubuntu 26.04.1 VM'inde denendi: `rollback_status` → `preview` → `apply` (ok=117 changed=35 failed=0); hardening sonrası yeni SSH bağlantısı doğrulaması geçti, güvenlik ağı disarm edildi; ufw/sshd/postfix durumu bağımsız kontrol edildi. Denenmedi: `audit`, `rollback_now`, `rollback_disarm` (yalnızca LAB), menü ve Terminal açma.

## [3.6.2] - 2026-10-18

Paket sürümü güncellemesi: `linux-hardening` 0.3.0 kilitlendi; paket çalıştırma ve `validate` yanlış alarmları düzeltildi.

### Değişti
- **`components.lock`: `linux-hardening` 0.2.0 → 0.3.0** (CIS Ubuntu 26.04 Level 1 eşlemesi; yeni roller `files`/`sudo`/`docker`; değişken sayısı 5 → 27). Ek: davranışı değiştiren yeni varsayılan-AÇIK ayarlar paketin CHANGELOG'unda ve README "Dikkat" tablosunda. Docker LAB'da 16 e2e testin tamamı 0.3.0 ile geçti. **Gerçek VM'de sshpocket üzerinden denenmedi.**
- Docker LAB varsayılan değişkenleri `hardening_disable_apport=false` ve `hardening_disable_update_notifier_motd=false` yazar (0.3.0'da varsayılanları `warn_if` ile aynı olduğundan her çalıştırmada onay istemi çıkıyor; syhgorath/linux-hardening#74).

### Düzeltildi
- Paket çalıştırma: `warn_if` ("riskli değer") onayı artık salt okunur durum eylemlerinde (örn. `rollback_status`) sorulmaz; yalnızca değiştiren eylemlerde ve ön koşul olan eylemlerde (`preview`) sorulur. Önceden her eylemde sorulduğu için varsayılanı zaten uyarı değerinde olan bir değişken (linux-hardening 0.3.0'da `hardening_disable_apport`) en zararsız eylemi bile onaya bağlıyordu.
- `components validate`: adında `password`/`token` geçen **bool ve int** değişkenler (örn. `hardening_password_inactive_days`, gün sayısı) artık "gizli değer izlenimi" uyarısı vermez; yanlış alarmdı (bool/int parola taşıyamaz). Metin alabilen türler (string, list) için uyarı aynen sürer.

### Test (geliştirici)
- Docker LAB e2e: `audit` (lynis kurulumu, rapor/özet 0600-0700 izinleri, `summary.json` içeriği), `rollback_now` (armed değilken etkisiz; armed iken sonraki ufw/sshd değişiklikleri geri alınır) ve `rollback_disarm` (armed ve yedek silinir) sshpocket üzerinden uçtan uca denendi. Üretim kodu değişmedi.

## [3.6.1] - 2026-10-17

Sunucuya ulaşılamayınca nedenini söyleyen ipuçları, paket yazarları için sözleşme uyum denetimi (`validate`) ve güvenilirlik düzeltmeleri.

### Eklendi
- **`./start.sh --components validate <kurulu-paket | dizin | arşiv.tar.gz> [--strict] [--unsigned]`:** paket SÖZLEŞME uyum denetimi (`docs/package-standard.md` §12'nin mekanik kısmı).
  Statik: ansible/ssh/ağ çalıştırmaz, paketten hiçbir şey çalıştırmaz; arşiv yalnızca güvenliyse açılır (`..`, mutlak yol, bağ, aygıt dosyası, aşırı boyut reddedilir; geçici klasör silinir).
  ❌: geçersiz `component.json`, `supported` profil yok, `VERSION` uyuşmazlığı, `requirements.txt` yok/hash'siz/sabit sürümsüz/URL, olmayan playbook, sembolik bağ, özel anahtar/gizli dosya.
  ⚠️: değiştiren eylemde `preview` ön koşulu/`warnings`/`requires_sudo` yok, `SSHPOCKET_RESULT` çıktısı bulunamadı, parola gibi değişken adı, kullanılmayan değişken, geri alma belgesi yok.
  `--strict` uyarıları hata sayar; `--unsigned` imzalamadan önceki denetimde `.sig` uyarısını atlar. Paket reposunun CI'ı `component_validate.py` + `component_manifest.py` ile bağımsız
  çalıştırabilir. Gerçek `linux-hardening 0.2.0`: 0 hata, 3 uyarı (kurtarma eylemleri ve `audit` bilerek ön koşulsuz). Yeni paket yazarları için şablon:
  [sshpocket-paket-sablonu](https://github.com/syhgorath/sshpocket-paket-sablonu).

### Değişti
- **Sunucuya ulaşılamayınca nedenini ve yapılacağı söyler:** işletim sistemi algılaması başarısız olduğunda ssh'ın hatası sınıflandırılır (`↳` ipucu satırı): zaman aşımı
  (VPN/ufw), rota yok, bağlantı reddi, **host key değişmiş**, host key tanınmıyor, anahtar reddi, çok fazla kimlik doğrulama denemesi, ad çözülemedi, kesilen bağlantı (fail2ban).
  Hata metni temizlenir ve kesilir, kabuğa verilmez. Tablo: `docs/hardening-kullanim.md`.

### Düzeltildi
- **Erişim ağı editörü üst üste binen ağları ayıklar:** listede `10.0.0.0/16` varken `10.0.1.0/24` eklenirse *"Zaten kapsanıyor"* denir ve eklenmez; daha geniş bir ağ
  eklenirse kapsadığı dar girdiler yerine geçer ve bu açıkça söylenir.
- **Terminal kapanınca (SIGHUP) geçici inventory/değişken dosyaları silinmiyordu:** yürütücü yalnızca Ctrl-C (INT) ve TERM'i yakalıyordu; HUP da yakalanır.

### Geliştirici
- **Docker LAB'ı (`test/e2e/lab/`) ve uçtan uca paket testleri:** systemd'li Ubuntu 26.04 konteyneri; `./test/e2e/lab/up.sh && bats test/e2e/components.bats`. `linux-hardening` akışını
  (`rollback_status` → taze `preview` olmadan `apply` reddi → `preview` (sunucu değişmez) → **geri alma güvenlik ağı** (A: doğrulama başarısız → önceki duruma döner; B: denetleyici kaybolur →
  sunucu zamanlayıcıyla kendini geri alır) → `apply` → **erişim ağı tuzağı** → erişilemeyen sunucu, tanınmayan ve değişmiş host key) gerçek ssh + ansible + ufw ile dener. Kullanıcının
  `known_hosts` ve sshpocket durumuna dokunmaz. `colima-trust-ca.sh`: antivirüs HTTPS'i yeniden imzalıyorsa CA'yı yalnızca Colima VM'ine ekler. Varsayılan `bats test` ve CI bunları çalıştırmaz.

## [3.6.0] - 2026-10-17

Paketler artık gerçekten kullanılabilir: ilk paket (`linux-hardening`) kilitli, imzalı ve gerçek bir Ubuntu 26.04 sunucusunda
`rollback_status` → `preview` → `apply` ile denendi. Kullanım ve **erişim ağı tuzağı**: [docs/hardening-kullanim.md](docs/hardening-kullanim.md).

### Eklendi
- **İlk gerçek paket kilitlendi:** `components.lock` artık `linux-hardening 0.2.0` (imzalı, paket standardı v1 uyumlu; sha256 `2886b610…f38a`) girdisini taşır.
- **`./start.sh --components run <paket> <eylem> <sunucu>`:** kurulu paketin bir eylemini sunucuda çalıştırır. Sunucu bilgisi `servers/<ad>/server.env`'den,
  paket değişkenleri `servers/<ad>/<paket>.vars`'tan okunur. İşletim sistemi profili sabit, salt okunur bir `ssh ... cat /etc/os-release` ile algılanır
  (host key doğrulaması kapatılmaz, parola sorulmaz; çıktı hiçbir zaman çalıştırılmaz). Python ortamı yoksa **onayla** kurulur (PyPI'dan ~10 MB, hash'li
  wheel; `~/.cache/sshpocket`). Sonuç `servers/<ad>/components.log` dosyasına (tarih|paket|sürüm|eylem|ok/fail, gizli bilgi yok) işlenir.
- **Çalıştırma öncesi kapılar:** değiştirici eylem root ile reddedilir; host key tanınmalı; anahtar ssh-agent'ta olmalı (gerekirse eklenir); işletim sistemi
  profili `supported` olmalı; `requires` eylemleri (örn. `apply` için `preview`) aynı sunucuda, aynı sürümde, son 30 dakikada başarıyla çalışmış olmalı;
  `confirm` (evet/hayır ya da sunucu adını yazma) ve `warn_if` onayları. Girdi yoksa onay reddedilir. Paket bu kuralları gevşetemez.
- **Güvenli yürütme:** `ansible-playbook` paketin venv'inden, paket kökünde, `ANSIBLE_HOST_KEY_CHECKING=True` zorlanarak çalışır; kabuk dizgesi/`eval` yoktur;
  geçici inventory/değişken dosyaları (izin 600) Ctrl-C/TERM dahil her çıkışta silinir; `outputs.dir_var` her zaman kullanıcı klasörüne yönlendirilir (hassas çıktı
  USB'ye yazılmaz); çıktı kullanıcı klasörüne (izin 600) günlüklenir. Eylemin `SSHPOCKET_RESULT` satırı önek, anahtar ve değer denetimiyle ayrıştırılır.
  `-i`/`--limit`/`-e` çalıştırıcıya aittir (pakette `cmd` içinde geçerse paket reddedilir).
- **Sunucu menüsünde `🛡️ Paketler`:** kurulu paket varsa paket → eylem seç (🔍 salt okunur / ✏️ değişiklik yapar / ☠️ yıkıcı; ön koşul durumu etikette). Ekranın
  başında **Hazırlık** bloğu: zorunlu değişken eksikse ❌ ve eylemler ⛔ kapalıdır.
- **`📝 Değişkenler` ekranı:** değer/varsayılan/otomatik gösterimi, kaydetmeden önce doğrulama (çalıştırmadakiyle aynı doğrulayıcı), `warn_if` onayı, `-` ile silme.
  `list[cidr]` değişkenler (örn. `hardening_ssh_allow_from`) için **ağ editörü**: bu bilgisayarın adresinden öneri (ağ / tahmini /24 / yalnızca bu adres), isteğe
  bağlı sunucudan sorgu (sabit, salt okunur ssh), elle ekleme/silme/temizleme; listede tek ağ varsa *"başka bir ağdan bağlanırsanız SSH kapalı olur"* uyarısı.
  Hiçbir şey otomatik yazılmaz.
- **`📜 Geçmiş ve son sonuçlar`:** bu sunucudaki son çalıştırmalar, ön koşulların tazeliği (*apply için preview: ✅ taze / ⏳ süresi doldu / ❌ kayıt yok*) ve son
  çalıştırma günlüğünün son satırları. Durum eylemlerinin (örn. `rollback_status`) son sonucu saklanır ve Hazırlık bloğunda gösterilir.
- **Bildirim doğrulayıcısı `outputs` alanını okur:** paketteki `prefix`, sshpocket'in türettiğiyle (`SSHPOCKET_RESULT <paket>.<eylem>`) uyuşmazsa paket reddedilir;
  uyarı metinleri menüde gösterilir. Karar kayıtları: `docs/package-standard.md` §14.

### Değişti
- **Sudo parolası istemi açıklanıyor:** eylem `--ask-become-pass` ile çalışacaksa, Ansible'ın `BECOME password:` istemi öncesinde bunun sunucudaki sudo parolası
  olduğu, sudo parola istemiyorsa (NOPASSWD) yalnızca Enter'a basılacağı ve parolanın sshpocket'e/günlüğe yazılmadığı belirtilir.
- **Belgeler:** `docs/hardening-kullanim.md` (akış, erişim ağı tuzağı, geri alma); README'de Apple Silicon (M5 Pro, macOS 27) test satırı, güncel Python ve paketler
  maddeleri (TR/EN).

### Düzeltildi
- **`components/` (kurulu paketler) `.gitignore`'a eklendi:** `--components sync` ile açılan paket klasörü yanlışlıkla commit'lenebiliyordu.

## [3.5.2] - 2026-10-16

### Düzeltildi
- **Keychain'deki passphrase yanlışken bağlantı ekranı sonsuza kadar takılıyordu:** `ssh-add`, askpass boş cevap verene kadar yeniden dener; askpass her seferinde
  aynı yanlış passphrase'i döndürdüğü için "SSH key agent'a ekleniyor" satırında %100 CPU ile sonsuz döngü oluşuyordu. `askpass.sh` artık yalnızca ilk
  çağrıda passphrase döndürür (`ASKPASS_STATE`), sonrakilerde boş satır verir; `ssh-add` birkaç saniyede vazgeçer.

### Eklendi
- **Yanlış kayıtlı passphrase'te yeniden sorma:** key agent'a eklenemezse (agent'a ulaşılabiliyorsa) "Kayıtlı passphrase yanlış olabilir. Yeniden girip
  Keychain'e kaydedeyim mi? [e/H]" diye sorulur; `e` ise passphrase güvenli istemle (komut satırında görünmez) Keychain'e yazılır ve en çok 3 kez denenir.
  Girdi yoksa takılmaz.

## [3.5.1] - 2026-10-12

### Değişti
- **Belgeler güncellendi:** README'ye **Gereksinimler** bölümü (hangi araçlar macOS ile gelir; fzf ve Python isteğe bağlı); eskimiş `.env` anlatımı
  (`servers/<ad>/server.env`, `sshpocket.conf`); tekrarlanan menü/fzf maddeleri birleştirildi; knock istemci tarafı ve `UI` değerleri anlatıldı;
  `docs/ui-design.md` uygulama durumuyla güncellendi.

### Düzeltildi
- **Ok tuşlu menüde listenin satırları her tuşta çoğalıyordu (3.5.0):** menü açıklamaları eklenince, terminal genişliğinden uzun açıklama bir alt satıra
  sarılıyor, imleci geri taşıma sayısı kayıyor ve her ↑/↓'de üst satır kopyalanıyordu. Çizim sırasında otomatik sarma kapatılır (uzun satırlar
  sağ kenarda kırpılır), çıkışta ve Ctrl-C'de yeniden açılır. 60/80/100 sütunlu gerçek pty testi eklendi.
- **fzf, Finder'dan açılınca (start.command) bulunamıyordu:** PATH'te yoksa `/opt/homebrew/bin` ve `/usr/local/bin` da aranır.

## [3.5.0] - 2026-10-12

### Düzeltildi
- **`check-new` ön sürümleri (pre-release) göremiyordu:** depoda kararlı "latest" yoksa (linux-hardening şu an yalnızca ön sürümlü) GitHub API'deki en yeni
  yayına bakılır (yine yalnızca bilgi; kurulmaz).
- **`--doctor` yeni Mac'te "geliştirici araçlarını kurun" penceresini açabiliyordu:** macOS'ta Command Line Tools yoksa `/usr/bin/python3` gerçek
  Python değil bir saplamadır ve çalıştırılınca kurulum penceresi açar. Saplama artık çalıştırılmadan atlanır ve doktor nedenini söyler.
- **Menüden "Geri" ile dönünce gereksiz "Devam etmek için Enter" soruluyordu:** sunucu menüsü, ⚙️ Ayarlar ve Sunucuları yönet'ten geri dönünce
  artık doğrudan ana menüye dönülür. Okunacak çıktısı olan işlemlerden (bağlanma sonucu, ekleme, silme, güncelleme) sonra duraklama korunur.

### Eklendi
- **Bakımcı aracı `scripts/lock-component.sh`:** bir paket sürümünü `components.lock`'a güvenle yazar: paketi ve imzayı HTTPS ile indirir, arşivi denetler,
  imzayı (`sshpocket-component`) doğrular, `component.json`'u şemaya göre doğrular (kurulamayacak paket kilide **yazılmaz**), sha256'yı hesaplar.
  `--write` yoksa yalnızca gösterir. `comp_get` pipefail altında 141 ile düşmesin diye tüm girdiyi okur.
- **CI'da sabit sürüm ShellCheck:** `scripts/ci-install-shellcheck.sh` (v0.11.0, indirilen arşivin sha256'sı doğrulanmazsa kurulmaz); yerel ve CI
  aynı sürümü kullanır, "yerelde yeşil, CI'da kırmızı" farkı kapanır.
- **fzf desteği (isteğe bağlı):** sistemde `fzf` kuruluysa (`brew install fzf`) tüm seçim menüleri fzf ile açılır: yazarak arama, **önizleme penceresinde
  seçili seçeneğin açıklaması**, `←`/Esc geri. fzf yoksa ya da terminal değilse (boru/test) kendi ok tuşlu seçicimiz, o da yoksa numaralı menü kullanılır.
  Açıklama metni fzf komut satırına girmez (dosyadan okunur). ⚙️ Ayarlar → menü stili: `auto` | `fzf` | `arrows` | `numbers`.
- **Menü açıklamaları:** ana menü, sunucu menüsü, Ayarlar, Sunucuları yönet ve Keychain menülerinde her seçeneğin kısa açıklaması; fzf yoksa ok tuşlu
  seçicinin alt satırında gösterilir. `./start.sh --doctor` fzf'i isteğe bağlı olarak denetler (kurmaz).

### Düzeltildi
- **Sunucu silinince Keychain'deki knock dizisi yetim kalıyordu:** artık sunucuyla birlikte silinir; yeniden adlandırmada eski adla duran knock
  dizisi için "Knock ayarları ile yeniden girin" uyarısı gösterilir.
- **Knock ayarları açıklamasız ve bağışlamazdı:** tek yanlış girişte "Geçersiz protokol" deyip menüden çıkıyordu. Artık knocking'in ne olduğu, her
  alanın (protokol, bekleme, dizi) ne anlama geldiği ve örnek değerler gösterilir; yanlış girişte **neden** geçersiz olduğu söylenip yeniden sorulur
  (3 deneme), büyük/küçük harf fark etmez, Enter = varsayılan. macOS'un diziyi iki kez isteyebileceği belirtilir.

## [3.4.0] - 2026-10-12

### Eklendi
- **Klasik port knocking (istemci tarafı):** sunucu menüsüne **5) Knock ayarları**. `server.env`'e `KNOCK=keychain`, `KNOCK_PROTO` (tcp|udp),
  `KNOCK_DELAY_MS`; **knock dizisi düz metin dosyada tutulmaz**, Keychain'de (`USBMonitor_<ÖNEK>_Knock`). Bağlan ve key gönderme, bağlantıdan
  önce diziyi otomatik gönderir (bash `/dev/tcp|udp`: portlar hiçbir sürecin komut satırında görünmez; takılmaz, en çok ~1 sn/vuruş).
  Durum göstergesi knock **göndermez**; knock'lı sunucuda port kapalıysa **🔒** gösterir. Keychain/ayar bozuksa bağlantı denenmez.
  Sunucu tarafı (knockd) ayrı bir pakettir; bu sürümde yoktur. Klasik knocking şifreli değildir (taramalardan saklanma katmanıdır).

## [3.3.0] - 2026-10-12

> **Paket altyapısı geldi.** `components.lock` şimdilik **boştur**; ilk paket (linux-hardening) hazır olunca eklenecek. Altyapı, paket kurulumunu
> imzalı, kilitli ve doğrulanabilir yapar (bkz. [docs/package-standard.md](docs/package-standard.md)). Mevcut sunucu/bağlantı davranışı değişmedi.

### Eklendi
- **Paket kilit dosyası ve imza doğrulama (Aşama 2.0):** `components.lock` (şimdilik boş) ve `sistem/Helpers/Components.sh`: katı ayrıştırıcı
  (bilinmeyen/yinelenen anahtar, bozuk değer = hata; dosya `source` edilmez), arşiv sha256'sı kilitle aynı olmalı, imza `sshpocket-component`
  ad alanıyla doğrulanır (çekirdek `sshpocket-release` imzası yeniden oynatılamaz). Kilitte olmayan paket reddedilir.
- **Paketin güvenli kurulumu (Aşama 2.1):** `sistem/Helpers/ComponentsInstall.sh`: HTTPS indirme (boyut sınırlı), arşiv denetimi (`..`, mutlak yol,
  farklı kök, symlink/hardlink/aygıt reddi), `component.json` kimlik+sürüm eşleşmesi, `components/<id>/<sürüm>/` + `current` (düz metin),
  eski sürüm kalır, geri sürüme düşülmez, kurulum özetleri her çalıştırmadan önce yeniden doğrulanabilir.
- **Bildirim okuma ve sözleşme doğrulama (Aşama 2.2):** `component.json` (`python3 -I` + stdlib `json`) tam şema denetimi:
  `sshpocket_api`, `runner`, profiller, eylemler (`cmd`'de kabuk meta karakteri yok, `effects`, `requires` döngüsüz), değişkenler, `invoke.append`;
  bilinmeyen zorunlu özellik ve daha yeni `sshpocket_api` reddedilir. Etkin `confirm`: paket yalnızca sıkılaştırabilir (`destructive` →
  sunucu adı yazdırma). `comp_prepare` kurulu paketi bütünlükle doğrular ve bilgiyi verir.
- **`./start.sh --doctor` ve venv önbelleği (Aşama 2.3):** `--doctor` ön koşulları denetler (ssh araçları, `nc`, Keychain, Python aralığı,
  `venv`/`ensurepip`, disk, ağ); **yalnızca okur ve çözüm komutunu gösterir, Homebrew/pip/sudo çalıştırmaz.** Paket bağımlılıkları için
  `ComponentsVenv.sh`: aralığa uyan Python seçimi, yalnızca hash'li `requirements.txt` (`-r/-e/--index-url` reddedilir),
  `~/.cache/sshpocket/venv/<paket>-<ortam-kimliği>` önbelleği, `pip --require-hashes --only-binary :all:` yalnızca venv içine; aynı ortam
  kimliğinde sağlıklı venv varsa hiçbir şey kurulmaz.
- **Paket yönetimi komutları (Aşama 2.4a):** `./start.sh --components list` (kilit ↔ kurulu durumu, ağsız), `sync [--yes]` (eksik/eski paketi
  kilitteki sürümle, imza doğrulayarak, onayla kurar; bozulmuş kurulumu silmeden `<sürüm>.bozuk-<tarih>` olarak kenara alıp yeniden kurar),
  `check-new` (kilitten daha yeni yayın var mı: **yalnızca bilgi**, kurulmaz; yeni sürüm yeni sshpocket sürümüyle gelir).
  `--update` başarılıysa **yeni kodla** paketler kilide göre eşitlenir; menüde kilitte paket varsa **📦 Paketler** girdisi görünür.

### Değişti
- Ok tuşlu seçim artık ⚙️ Ayarlar, **Sunucuları yönet** ve Keychain passphrase menülerinde de çalışır (başlıkla birlikte); `UI=numbers` ile
  hepsi numaralıya döner.

## [3.2.0] - 2026-10-11

### Eklendi
- **Ekran cilası (1/2):** her ekranda başlık + ekmek kırıntısı (`Ana menü › ubuntutest`) ve alttaki tuş ipucu satırı; sunucu satırlarında
  `kullanıcı@ip[:port]` özeti. Renk yalnızca terminalde ve `NO_COLOR` tanımsızsa; boru/test çıktısı düz metindir.
- **Ok tuşlu seçim (ekran cilası 2/2):** ana menü ve sunucu menüsünde ↑↓ (veya `j`/`k`) ile gez, Enter ile aç, `←`/`q`/`0`/Esc ile geri.
  Numara yazıp Enter de çalışır (iki haneli dahil, Backspace ile düzeltilir). Saf bash; terminal değilse (boru, betik, test) eski
  numaralı menü kullanılır. ⚙️ Ayarlar'a `UI` eklendi (`auto` | `numbers` | `arrows`): ok tuşları sorun çıkarırsa `numbers` yapın
  (ya da `SSHPOCKET_UI=numbers`). İmleç her çıkışta (Ctrl-C dahil) geri açılır.

## [3.1.1] - 2026-10-11

### Düzeltildi
- **Ana menüye dönerken takılma:** macOS `nc -w` bağlantı beklemesini sınırlamadığından kapalı/erişilemeyen sunucuda durum kontrolü
  ~75 sn sürebiliyordu (durum göstergesi 3.1.0'da varsayılan açık olunca belirgin oldu). Kontrol artık süre dolunca öldürülür
  (⚙️ Ayarlar'daki süre, 1-5 sn); bağlanmadan önceki port kontrolü en çok 5 sn bekler.

## [3.1.0] - 2026-10-11

### Eklendi
- **⚙️ Ayarlar** (ana menü): `sshpocket.conf` (kökte, gitignore'lu; güncellemede korunur). Şimdilik iki ayar: durum göstergesi (`STATUS`) ve
  durum kontrol süresi (`STATUS_TIMEOUT`, 1-5 sn). Öncelik: ortam değişkeni (`SSHPOCKET_STATUS` vb.) > dosya > varsayılan.
  Dosya `source` edilmez; yalnızca bilinen anahtarlar okunur, değerler doğrulanır.

### Değişti
- **Durum göstergesi (🟢/🔴/⚪) artık varsayılan AÇIK** (Finder'dan `start.command` ile açınca da görünür). Kapatmak için ⚙️ Ayarlar
  veya `SSHPOCKET_STATUS=0`.
- **`q` = geri/çıkış:** ana menüde `q` çıkış yapar (0 ile aynı); alt menülerde `0) Geri (q)` ipucu gösterilir.

## [3.0.0] - 2026-10-11

> **Veri düzeni değişti (büyük sürüm).** Sunucular artık `servers/<ad>/server.env` altında; eski `sistem/Modules/<ad>/` sunucularınız
> **otomatik taşınır** (`./start.sh --migrate`, önce `--dry-run`). Eski sürümler yeni düzeni okuyamaz; geri dönmek isterseniz
> `servers/.migrated-<tarih>/` altındaki eski klasörleri `sistem/Modules/` altına geri koyun. Keychain'deki passphrase'ler aynen geçerlidir.

### Eklendi
- **Aşama 1.0/1.1 (`servers/` veri katmanı, geriye uyumlu):** menü artık `servers/<ad>/server.env` (genel anahtarlar `IP`, `USER`, `PORT`, `KEY`, `OS`)
  klasörlerinden de kurulur; sunucu başına üretilmiş bash dosyası gerekmez. **Eski `sistem/Modules/<ad>/.env` biçimi aynen çalışmaya devam eder**
  (taşıma sonraki adımdır). Tek erişim katmanı `srv_get/srv_set` biçimi korur. Aynı adlı eski modül varsa yeni biçim kazanır.
- `servers/` içindeki hiçbir dosya `source` edilmez (güvenlik testleri); gizli klasörler, `._*` ve geçersiz adlar yok sayılır.
- Güncelleyici `servers/` klasörüne asla dokunmaz (arşivde olsa bile); release paketine `servers/` dahil edilmez.
- **Aşama 1.2 (taşıma):** `./start.sh --migrate [--dry-run] [--yes]` ve etkileşimli açılışta (yalnızca terminalde, onaysız asla) teklif.
  Eski `sistem/Modules/<ad>/` → `servers/<ad>/`: ayarlar genel anahtarlara çevrilir, key'ler kopyalanır ve **bayt bayt doğrulanır**,
  izinler 600/700, üretilmiş modül dosyaları taşınmaz. **Hiçbir şey silinmez:** eski klasörler `servers/.migrated-<tarih>/` altına alınır;
  hata olursa o sunucunun eski klasörü olduğu gibi kalır, yarım klasör bırakılmaz. İdempotent. Keychain'e dokunulmaz (passphrase'ler aynen geçerli).
- **Aşama 1.3 (yeni biçime yazma):** `ModuleGenerator.sh`, `ImportSSHConfig.sh` ve menüdeki "Yeni sunucu ekle / içe aktar" artık
  `servers/<ad>/server.env` yazar (genel anahtarlar; **sunucu başına üretilmiş bash dosyası yok**, `.env.example` yok).
  `ModuleManager.sh`: `list` yeni ve eski biçimi (etiketle), `remove` ikisini de, `rename` yeni biçimi (klasör + key dosyaları; eski
  biçim için "önce `--migrate`" der) yönetir. Aynı ad hem `servers/` hem eski `Modules/`'te varsa oluşturma/içe aktarma reddedilir.
- **CI'a macOS işi:** `macos-latest` (Apple Silicon arm64, macOS 26.6.2) üzerinde macOS'un kendi bash 3.2'siyle ShellCheck + tüm testler. Bu sırada bulunan
  bir test aracı sorunu (bash 3.2 + UTF-8 yereli, Türkçe test adlarını yanlış kodluyor) `LC_ALL=C` ile giderildi. Keychain izin penceresi ve Terminal
  açma otomatik sınanmaz (testler sahte `security` kullanır).
- **Aşama 1.4 (sadeleşme):** gönderilen `sistem/Modules/example/` modülü kaldırıldı; yerine `sistem/templates/server.env.example` şablonu geldi
  (menüde artık boş bir "EXAMPLE" girdisi çıkmaz; sunucu eklemek için "Yeni sunucu ekle"). Güncelleyici sadeleşti: kod (`sistem/*.sh`, `Helpers`,
  `templates`) bütünüyle değişir; **`sistem/Modules/` (henüz taşınmamış eski sunucular) ve `servers/` asla silinmez/ezilmez**. Eski sürümlerden kalan,
  hiç yapılandırılmamış `example` şablonu güncellemede güvenle temizlenir (içinde `.env`/key/başka dosya varsa dokunulmaz). Taşıma, `.env`'i olan
  `example`'ı da (gerçek sunucu olarak) taşır. Eski `Modules/` klasörü yoksa artık uyarı verilmez.

### Belge
- Tasarım kararları işlendi: durum göstergesi varsayılan açık, `⚙️ Ayarlar` paneli (5 ayar, `sshpocket.conf`), `q` geri, fzf USB'ye gömülür (yedekli), klasik port knocking için pre-connect kancası (`docs/ui-design.md`, `docs/components-design.md` §6.7).
- `docs/ui-design.md`: arayüz ilkeleri, fzf dağıtım politikası, Apple Silicon doğrulama listesi ve teknoloji alternatifleri (tasarım, kod yok).
- README: "Test edilen ortamlar" tablosu. Yalnızca Intel + macOS 15.7.9'da denendi; Apple Silicon ve daha yeni macOS **denenmedi**.

### Belge
- `docs/components-design.md`: `servers/` veri ayrımı ve bileşen modeli için tasarım (henüz uygulanmadı); linux-hardening PR sırası incelemeyle güncellendi (A → 0a → 0b → 0c → B → D → C).
- Tasarım belgesi, linux-hardening'in gerçek `component.yml`'ine hizalandı (issue #2): `invoke.append` nesne listesi, `inventory_group`, `outputs`, `hardening_scan_dir`, principal=ad alanı.
- Tasarım belgesi, linux-hardening v0.1.0'ın geri alma güvenlik ağına hizalandı (issue #3): `rollback_status/disarm/now`, `hardening_rollback_*` değişkenleri, arayüz notları.

### Eklendi
- Dependabot yapılandırması (aylık, yalnızca GitHub Actions bağımlılıkları).

## [2.2.0] - 2026-10-08

### Eklendi
- **İmzalı güncelleme:** `./start.sh --update` / `--check-update` ve menüde "Güncelle". Yalnızca bakımcının imza
  anahtarıyla imzalı (`ssh-keygen -Y verify`), HTTPS ile inen Release paketini kurar; imza yoksa/anahtar tanımlı
  değilse kapalıdır. Geri sürüme düşmez, tehlikeli arşivleri (`..`, mutlak yol, symlink) reddeder, kullanıcı
  verisini korur, yedek alır ve hata olursa geri yükler.
- `scripts/release.sh` (imzalı paket üretir ve yayından önce doğrular) ve `RELEASING.md` (bakımcı rehberi).
- `.gitattributes` ile release paketine `test/`, `scripts/`, `.github/` girmez.
- Depo hijyeni testleri: `release_signers` biçimi ve repoda özel anahtar/sır dosyası bulunmaması.

> Güncelleyici bu sürümle gelir; v2.1.0 ve öncesi kopyalar bir kez elle güncellenmelidir.

## [2.1.0] - 2026-10-08

> İlk yayın. `2.0.0` hiç yayınlanmadığı için bu sürüm, aşağıdaki `2.0.0` bölümündeki her şeyi de içerir.

### Eklendi
- Sürüm tek yerde (`VERSION`); `sistem/Main.sh --version` / `./start.sh --version` sürümü yazar.
- Uçtan uca testler (`test/e2e/`): Docker'da yerel Ubuntu `sshd` ile `ssh-copy-id`, elle ekleme komutu, root reddi ve durum göstergesi gerçek sunucuya karşı denenir.
- `ssh-copy-id` başarısız olursa olası nedenler ve sunucuda çalıştırılacak **tek satırlık elle ekleme komutu** gösterilir
  (komut tekrar çalıştırılabilir, mevcut key'leri korur). Kullanıcı `root` ise Ubuntu/Debian uyarısı verilir.
- Modül menüsünde **Bilgileri güncelle** (IP/kullanıcı/port/key yolu) ve **Keychain passphrase'ini güncelle/sil**.
  Passphrase kaydedildikten sonra key açılarak doğrulanır. Ayar bozuk veya key eksikken de bu seçenekler çalışır.
- Ana menüde yerleşik eylemler: yeni sunucu ekle, `~/.ssh/config`'ten içe aktar, sunucuları yönet.
  Eklenen/silinen modüller yeniden başlatmadan menüye yansır.

### Düzeltildi
- `--auto` modu artık yerleşik eylemleri atlayıp ilk sunucuyu çalıştırır.

## [2.0.0] - 2026-10-06

> İç dönüm noktası; etiketlenmedi ve yayınlanmadı. İlk yayın: 2.1.0.

### Eklendi
- `~/.ssh/config` içe aktarma (`ImportSSHConfig.sh`); private key'ler kopyalanmaz, `<ÖNEK>_KEY` ile referanslanır.
- `ModuleManager.sh`: `list`, `remove`, `rename`.
- Menüde durum göstergesi (`SSHPOCKET_STATUS=1`): 🟢 / 🔴 / ⚪.
- Public key'i sunucuya gönderme (`ssh-copy-id` akışı), modül menüsünden ve generator'dan.
- `<ÖNEK>_KEY` ile modül klasörü dışındaki mevcut key'i kullanma.
- `start.command` (Finder'dan çift tıkla başlatma).
- 53 Bats testi; GitHub Actions'ta ShellCheck + Bats.
- README (TR/EN), CONTRIBUTING, SECURITY, ROADMAP, issue/PR şablonları.

### Değişti
- Passphrase artık düz metin `.env`'de değil, macOS Keychain'de; `SSH_ASKPASS` ile geçici dosyasız aktarılır.
- Üç kopya modül tek ortak çalıştırıcıya (`Helpers/SSHModule.sh`) indirildi.
- `.env` ayrıştırma saf bash ile yeniden yazıldı (eval yok); satır sonu yorumları desteklenir.
- Host key ilk bağlantıda kullanıcıya sorulur (`accept-new` kaldırıldı); `ssh -v` varsayılan değil.
- `autorun.sh` kaldırıldı (macOS desteklemez); yerine `start.sh`.

### Düzeltildi
- IP/kullanıcı/port doğrulaması; kullanıcı adı `-` ile başlayamaz (ssh seçenek enjeksiyonu).
- Menü, girdi bittiğinde (EOF) sonsuz döngüye giriyordu.
- Generator'daki macOS'ta çalışmayan `sed \U` ve üzerine yazma davranışı.

### Kaldırıldı
- TOTP gizli anahtarları ve parolaların `.env` içinde tutulması.

[Unreleased]: https://github.com/syhgorath/sshpocket/compare/v3.5.1...HEAD
[3.5.1]: https://github.com/syhgorath/sshpocket/compare/v3.5.0...v3.5.1
[3.5.0]: https://github.com/syhgorath/sshpocket/compare/v3.4.0...v3.5.0
[3.4.0]: https://github.com/syhgorath/sshpocket/compare/v3.3.0...v3.4.0
[3.3.0]: https://github.com/syhgorath/sshpocket/compare/v3.2.0...v3.3.0
[3.2.0]: https://github.com/syhgorath/sshpocket/compare/v3.1.1...v3.2.0
[3.1.1]: https://github.com/syhgorath/sshpocket/compare/v3.1.0...v3.1.1
[3.1.0]: https://github.com/syhgorath/sshpocket/compare/v3.0.0...v3.1.0
[3.0.0]: https://github.com/syhgorath/sshpocket/compare/v2.2.0...v3.0.0
[2.2.0]: https://github.com/syhgorath/sshpocket/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/syhgorath/sshpocket/releases/tag/v2.1.0
