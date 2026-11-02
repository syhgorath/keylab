# Sunucu eklerken platform seçimi (tasarım notu, KOD YOK)

Durum: **öneri**. Yazan: sshpocket oturumu, 2026-11-02. Paket tarafı notu `linux-hardening` oturumunda yazılıyor; alan adları
aşağıda **öneridir**, paket oturumuyla uzlaşılacak.

## 1. Sorun

`fizikiproxmox` için `preview` "Bu sunucu bir Proxmox VE hostu… generic platformla sertleştirilmez" diye reddedildi. Kök sebep
`servers/fizikiproxmox/linux-hardening.vars` içinde `hardening_platform=proxmox-host` satırının USB kopyasında olmamasıydı
(yerel kopyada vardı). Üç ayrı kusur birleşti:

1. Platform, yalnızca elle yazılan bir değişken; yeni kullanıcı bunu bilmeden Proxmox'ta hata alıp çözemez.
2. `servers/` yerelde ve USB'de iki ayrı kopya olarak ayrıştı.
3. Hata mesajı doğru ama çözümü (hangi dosya, hangi satır) göstermiyor.

Amaç: **işi kolaylaştırmak, karmaşıklaştırmamak.** Kullanıcı sunucuyu eklerken ya da pakete ilk girdiğinde platformu bir listeden
seçer; gerisi otomatik.

## 2. İlkeler

1. **Yanlış seçim güvenli:** paketin ön kontrolü Proxmox'u ve Raspberry Pi'yi zaten algılayıp durdurur. Seçim ve tahmin yalnızca kolaylıktır, güvenlik sınırı değildir.
2. **Değişkenler üç gruba ayrılır:**
   - **(a) Sunucunun NE olduğunu söyleyenler** (`hardening_platform`): seçimden otomatik yazılır.
   - **(b) Platforma göre güvenli varsayılanlar:** seçimden otomatik yazılır, kullanıcıya gösterilir.
   - **(c) Erişimi ya da sunucuyu etkileyenler: ASLA sessizce yazılmaz.**
     `hardening_ssh_allow_from` (sunucu eklerken SORULUR), `hardening_pve_apply` (sunucu eklerken değil; `apply` anında,
     eylemin kendi onayıyla), `hardening_ufw_prune_ssh` (silici; asla otomatik açılmaz).
3. **Platform bilgisi PAKETTE durur.** sshpocket "Proxmox nedir" bilmez; paketin bildirdiği listeyi gösterir, seçileni yazar.
4. **Tahmin önerilir, uygulanmaz:** "Bu sunucu Proxmox görünüyor, doğru mu?" diye sorulur; kullanıcı onaylar. Raspberry Pi otomatik seçilmez.
5. Yeni platform pakete eklenince **menü kendiliğinden artar** (sshpocket kodu değişmez).

## 3. Akış (ekran adımları)

Mevcut sunucu ekleme betiği `sistem/ModuleGenerator.sh`: ad → IP → kullanıcı → port → anahtar → public key kopyalama.
Platform adımı bundan **sonra** gelir. **Platform kullanıcıya DOĞRUDAN sorulur; algılama zorunlu değildir** ve ekleme akışının ön
koşulu yapılmaz (ssh/sudo/ajan gerektiren hiçbir adım sorunun çıkması için şart değil):

| Adım | Ekran | Not |
|---|---|---|
| 1–5 | Ad, bağlantı, anahtar (bugünkü gibi) | değişmez |
| 6 | **Platform listesi** (paketin manifestinden, `platform_choices`) | doğrudan sorulur; "Bilmiyorum / genel (generic)" her zaman var. `platform_detect` **sonuç verdiyse** (kapılar uygunsa) o satırın yanında "(önerilen)" işareti çıkar, imleç orada başlar; sonuç yoksa sessizce atlanır |
| 7 | **`allow_from` sorusu** (pakette `onboarding_vars` ile tanımlıysa) | **ZORUNLU:** boş geçilemez (§5.1). Öneriler "doğru mu?" diye sunulur, sessizce yazılmaz; `/24` ↔ `/32` uyarısı |
| 8 | Özet: yazılacak satırlar | onay → `servers/<ad>/<paket>.vars` |

**Hangi paket?** Sunucu ekleme pakete bağlı değil. Öneri: adım 6–8, **kurulu ve manifestinde platform listesi olan her paket için**
yalnızca bir kez, sunucu eklendikten sonra sorulur ("linux-hardening için bu sunucunun türü?"). Hiç paket yoksa adım atlanır.

## 4. Eski sunucular: ilk açılışta soru

`servers/<ad>/<paket>.vars` içinde `platform_var` satırı **yoksa** ve paketin platform listesi varsa, pakete ilk girişte
(paket menüsü açılırken, bir kez) aynı 6–7. adımlar (platform + zorunlu `allow_from`) sorulur. "Sonra" denirse hiçbir şey yazılmaz (işaret dosyası yok):
`generic` varsayımıyla devam edilir, başlıkta tek satırlık not çıkar ve soru bir sonraki girişte yine sorulur. Böylece GitHub'dan indiren biri Proxmox'ta hatayla
karşılaşmadan önce sorulmuş olur.

## 5. Manifestten gelen alanlar (öneri, paket standardına isteğe bağlı ek)

Paket `component.json`'a ekler (hepsi isteğe bağlı; eski sshpocket yok sayar):

```json
"platform_var": "hardening_platform",
"platform_choices": [
  {"id": "generic",       "label": "Debian / Ubuntu / diğer Linux", "description": "…"},
  {"id": "proxmox-host",  "label": "Proxmox VE hostu",              "description": "…", "defaults": {"hardening_ufw_prune_ssh": "false"}},
  {"id": "raspberry-pi",  "label": "Raspberry Pi OS",               "description": "…", "auto_detect": false}
],
"onboarding_vars": ["hardening_ssh_allow_from"]
```

- `platform_choices[].defaults` → **grup (b)** (yalnızca güvenli varsayılanlar; paket yazarının sorumluluğu).
- `onboarding_vars` → sunucu eklenirken **sorulacak** değişkenler (**grup (c)**'den yalnızca `allow_from` gibi sorulabilir olanlar).
  `hardening_pve_apply` ve `hardening_ufw_prune_ssh` burada **bulunamaz**: sshpocket bu iki türü (`warn_if` taşıyan ya da `destructive`
  olarak işaretli değişkenleri) `onboarding_vars` dışında reddeder.
- `platform_choices[].auto_detect: false` → tahmin bu platformu önermez (Raspberry Pi). Pi'ye özgü değişkenler (örn. `hardening_rpi_strict`) sunucu eklerken **ASLA sorulmaz** ve `defaults`'a girmez.
- `platform_choices[].apply_vars` + `apply_warnings` (paket oturumunun isteği; **kullanıcı onayladı**, koşullarla): örn. `{"hardening_pve_apply": true}`.
  Sunucu eklerken **yazılmaz**; yalnızca değişiklik yapan eylemin (`apply`) onayından (`type-server-name`) **sonra**, o koşuya özel
  `-e` ile geçer ve **yalnızca değişken `.vars` dosyasında YOKSA** (kullanıcı bilerek `false` yazdıysa ezilmez). Koşullar:
  1. `apply_vars` doluysa `apply_warnings` boş olamaz ve metin **onay sorusundan önce** gösterilir ("bu koşuda hardening_pve_apply=true verilecek");
  2. `.vars` dosyasına **yazılmaz**; günlük başlığında "bu koşuya özel: hardening_pve_apply=true" satırı çıkar;
  3. yalnızca değişiklik yapan eylemlerde (`preview`/`audit` değil) ve platform **açıkça seçilmişse** (`platform_var` dosyada yazılı; `auto`/belirsiz değil);
  4. yalnızca tanımlı `bool`/`enum` değişkenler; silici/erişimi kesen değişken (`hardening_ufw_prune_ssh`) bulunamaz (iki yönlü denetim: paket ve menü).

### 5.1 `allow_from` zorunlu: boşsa preview/apply çalışmaz

Paket oturumu `required: true`'ya **karşı**: gizli (platforma uymayan) değişken de zorunlu kalır ve `audit` (aracı kurduğu için "değiştiren" sayılır)
kilitlenir. Öneri: **eylem düzeyinde, isteğe bağlı yeni alan** `requires_vars` (paket bildirir, sshpocket kapıda uygular):

```json
"actions": { "preview": {…, "requires_vars": ["hardening_ssh_allow_from"]}, "apply": {…, "requires_vars": ["hardening_ssh_allow_from"]} }
```

- Pakette `linux-hardening` için: `preview`, `apply`, `pve_firewall_preview`, `pve_firewall_apply` (paket oturumu, bilinçli sıkılık: ufw ve ssh alanı ikisi de kapalıyken bile ister).
- sshpocket, listedeki değişken `.vars` dosyasında **boş ya da yoksa** o eylemi ssh'a gitmeden reddeder (⛔, `required` kapısıyla aynı yer: `comp_gate_*`); `audit`, `rollback_*`, `kaldir` etkilenmez.
- Paketin kendi ön kontrolü (`access.yml`) `harden.yml`'i zaten durduruyor: eski sshpocket'te alan yok sayılsa bile güvenli başarısız olur (kemer + pantolon).
- Sunucu eklerken değer **zorunlu sorulur, boş geçilemez**; boş bırakılırsa sunucu kaydedilir ama `preview`/`apply` kapıdan döner ve menüde tek satır "önce izinli ağı girin" gösterilir.
- Bu, `sshpocket_api`'yi artırmaz (isteğe bağlı alan). Yeni kod: `component_manifest.py` (`requires_vars` doğrulaması), `ComponentsGates.sh` (kapı), menüde ⛔ etiketi.

**Öneri nereden gelir (hiçbiri sessizce yazılmaz, hepsi "doğru mu?" diye sorulur):**
1. **Sunucunun gördüğü adres** (`comp_net_seen_by_server`, salt okunur ssh, mevcut işlev): tek adres olarak `x.x.x.x/32` — en dar ve güvenli öneri.
2. **Bu Mac'in yerel ağı:** varsayılan arayüzün adresi ve maskesi (`route`/`ipconfig`), `/24` ağ olarak önerilir; ekranda "tüm ağa izin verir; tek cihaz için /32" uyarısı.
3. **VPN uyarısı:** uzaktan (VPN) bağlanılıyorsa 1. öneri VPN adresini, 2. öneri LAN'ı verir; ikisini de yazmak gerekebilir (`a,b`). Ekran, "evde ve dışarıda farklı adresten bağlanıyorsanız ikisini de ekleyin" der.
4. Algılama başarısızsa öneri yok; kullanıcı kendi yazar. Boş geçilemez.

**`apply_vars` kuralını kim uygular? Öneri: sshpocket.** Sebep: kullanıcının `.vars` değerini ve `type-server-name` onayını sshpocket bilir; paket yalnızca birleşmiş değerleri
görür ve dosyada "yoktu" ile "bilerek false yazılmıştı" ayrımını yapamaz. sshpocket zaten eylem düzeyi sabit değişkenleri (`action.set`) `-e` ile geçiriyor
(`ComponentsExec.sh`); `apply_vars` aynı yoldan, ama onaydan sonra ve yalnızca dosyada yoksa geçer. Paket yalnızca bildirir (`apply_vars`, `apply_warnings`) ve
tutarlılığı doğrular.

**Okuma yeri:** `sistem/Helpers/component_manifest.py` doğrulayıp `kimlik=değer` bilgi satırlarına çevirir (bugün `platform_var` ve
`platforms` böyle okunuyor); `ComponentsManifest.sh` yeni okuyucu işlev ekler; seçim ve yazma `ComponentsVars.sh`
(`comp_vars_set`) üzerinden.

**`sshpocket_api`:** yeni alanlar isteğe bağlı olduğu için **`sshpocket_api` artmaz** (standart zaten "isteğe bağlı alan api'yi artırmaz"
diyor). Bilinmeyen alanlar eski sürümlerde yok sayılır; yalnızca yeni seçim ekranı eski sshpocket'te çıkmaz.

## 6. Tahmin ve `auto` sorunu

- **Tahmin yalnızca öneridir, zorunlu değil.** Kaynak: paketin salt okunur `platform_detect` eylemi (`effects: []`, sudo yok, onay yok, zorunlu değişken yok;
  tek `SSHPOCKET_RESULT linux-hardening.platform_detect platform=<id|boş> hint=<id|none> os=<…>` satırı; Raspberry Pi **asla** `platform=` olmaz, yalnız `hint=`).
  sshpocket bunu mevcut yürütme yolundan çalıştırır; host key, ajan ya da başka kapı engellerse tahmin **sessizce atlanır** ve kullanıcı listeden seçer.
  (`detect_command` seçeneği elendi: sshpocket'e komut koymamak için.)
- **`auto` değeri paket tarafında yok** (`hardening_platform` hep somut; varsayılan `generic`). Yine de savunma için `current_platform` bildirilen
  `platform_choices` kimlikleri dışındaki değeri (boş, bilinmeyen) **"belirsiz"** sayar ve hiçbir süzme yapmaz (bugünkü "platform belirlenemezse süzme yok"
  kuralıyla aynı). `platform_choices` hiç yoksa bugünkü davranış aynen sürer.
- Tahmin başarısızsa (ssh yok, anahtar yok, zaman aşımı) sessizce atlanır; liste yine gösterilir.

## 7. Vars dosyası yazma kuralları

- Mevcut dosyayı **ezme**: `comp_vars_set` zaten yalnızca ilgili satırı ekler/değiştirir, yorumları ve diğer satırları korur (izin 600, atomik).
- **Kullanıcının elle yazdığı değer korunur:** satır zaten varsa sorulmaz ve üzerine yazılmaz (yalnızca "platform farklı görünüyor" notu).
- (b) varsayılanları yalnızca **dosyada yoksa** yazılır.
- (c) değişkenleri yalnızca kullanıcı sorudan açıkça yanıtlarsa yazılır; boş yanıt satır yazmaz.
- Her yazılan satır özet ekranında gösterilir ve onay alınır.

## 8. Güvenlik

- Grup (c): sessiz yazma yok (§2, §5). `hardening_pve_apply=true` yalnızca `apply` eyleminin kendi onayında (`type-server-name`) ya da kullanıcının elle yazmasıyla.
- Tahmin için çalıştırılan komut salt okunur ve sabit olmalı; kullanıcı girdisi ssh'a kabuk yorumu olarak gitmez.
- Platform listesi paket manifestinden gelir (imzalı arşivin içi); etiket ve açıklama ekrana yazılırken kontrol karakterleri temizlenir
  (`clean`, mevcut davranış).
- Seçim değeri `[a-z][a-z0-9-]*` doğrulamasından ve `platform_choices` kimlikleri arasından geçmek zorunda.

## 9. Eşitleme uyarısı: `servers/` iki kopya

Kopyalama senaryosunu çözmüyoruz; yalnızca uyarı öneriyoruz:

- Paket menüsü açılırken (ya da `doctor`) sunucunun `.vars` dosyasında `platform_var` satırı yoksa ve `platform_choices` varsa tek satır:
  "Platform bu kopyada kayıtlı değil (USB/yerel farkı olabilir)". Bu zaten §4'ün ilk-açılış sorusuyla birleşir.
- **Açık soru:** USB'de olup yerelde olmayan (ya da tersi) `.vars` farkını `doctor`'ın göstermesi istenir mi? Yalnızca salt okunur, otomatik kopya yok.

## 10. Test planı

- `component_manifest.py`: geçerli/geçersiz `platform_choices`, tekrarlı kimlik, `onboarding_vars`'ta `warn_if`/yıkıcı değişken reddi, eski manifest (alan yok) değişmez.
- `ComponentsVars.sh`: mevcut satır korunur, (b) yalnızca yoksa yazılır, boş yanıt satır yazmaz, izin 600, atomik.
- `current_platform`: `auto`/bilinmeyen değer → süzme yok; bilinen değer → süzme.
- Menü (sahte seçici/`ssh` taklidiyle): tahmin "evet/hayır/atla", Raspberry Pi tahminden önerilmez, eski sunucuda ilk-açılış sorusu bir kez, "sonra" davranışı, `allow_from` `/24`↔`/32` uyarısı.
- Güvenlik: `hardening_pve_apply`/`ufw_prune_ssh` hiçbir yoldan sessizce yazılmaz.
- Gerçek VM: Ubuntu ve Proxmox'ta sunucu ekleme → liste → tahmin → `preview` (el ile).

## 11. Kararlar (kullanıcı) ve açık sorular

**Karar verilenler:** platform doğrudan sorulur, algılama zorunlu değil (§3, §6) · `allow_from` zorunlu, boşsa `preview`/`apply` çalışmaz (§5.1, eylem düzeyi `requires_vars`) ·
`apply_vars` ONAYLANDI ve kuralı sshpocket uygular (§5, §5.1) · Raspberry Pi'ye özgü değişkenler eklerken sorulmaz (§5) ·
**çok düğümlü (küme) Proxmox: kullanıcıda yok, DOĞRULANMADI / DESTEKLENMİYOR olarak belgelenir** (platform listesinin açıklamasında ve README'de) ·
`servers/` yerel↔USB ayrışması için yalnızca uyarı (§9).

**Uygulama durumu:** manifest + `requires_vars` kapısı (#150) · sunucu eklerken platform + `allow_from` sorusu (`Helpers/ComponentsOnboard.sh`, `ModuleGenerator.sh`; `platform_detect` önerisi YOK, liste doğrudan sorulur; `allow_from` boş kalırsa platform yine yazılır, `allow_from` yazılmaz) ·
`apply_vars` uygulaması (`comp_gate_apply_vars`; yalnız type-server-name onaylı, changes-config, yıkıcı olmayan ve `requires` taşıyan eylemlerde = `apply`, `pve_firewall_apply`) ·
eski sunucuların ilk açılış sorusu (`comp_onboard_needed`, paket menüsü girişi; yalnız `platform_var` dosyada yoksa sorar, `allow_from` boşluğunu ⛔ kapısı yönetir).

**Açık sorular:**
1. ~~`apply_vars`~~ **Kullanıcı onayladı (2026-11-02):** `apply` sunucu adı onayından sonra `apply_vars` (örn. `hardening_pve_apply`) geçirilir; `.vars`'taki kullanıcı değeri ezilmez. Kuralı sshpocket uygular, pakette ara değişken yok (§5, §5.1 koşulları geçerli).
2. Soru anı: sunucu eklerken **ve** paket menüsü ilk açılırken (aynı kod). Kabul edilirse kapanır.
3. Birden çok paket platform listesi bildirirse platform paket başına mı sorulsun (öneri), sunucu başına tek "tür" mü?
4. `doctor` yerel↔USB `servers/` `.vars` farkını göstersin mi (salt okunur, otomatik kopya yok)?
5. Eski sshpocket + yeni paket: yeni alanlar yok sayılır, kullanıcı platformu elle yazar. Kabul edilebilir mi?
6. `requires_vars` ile mevcut sunucular (örn. `.vars`'ında bu satır olmayan) ilk açılışta ⛔ görür; bu istenen davranış mı, yoksa tek seferlik geçiş notu mu?
