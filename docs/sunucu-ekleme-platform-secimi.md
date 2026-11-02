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
Platform adımı bundan **sonra**, anahtarla bağlanılabildiği an gelir:

| Adım | Ekran | Not |
|---|---|---|
| 1–5 | Ad, bağlantı, anahtar (bugünkü gibi) | değişmez |
| 6 | **Platform tahmini** (isteğe bağlı, salt okunur ssh) | "Bu sunucu Proxmox VE görünüyor. Doğru mu? [E/h]" |
| 7 | **Platform listesi** (paketin manifestinden) | tahmin varsa onun üstünde imleç; "Bilmiyorum / genel (generic)" her zaman var |
| 8 | **`allow_from` sorusu** (yalnızca pakette tanımlıysa) | "Bu Mac'in ağı (10.1.10.0/24) doğru mu?" önerisi; `/24` ↔ `/32` uyarısı (tek adres için `/32`); boş geçilebilir |
| 9 | Özet: yazılacak satırlar | onay → `servers/<ad>/<paket>.vars` |

**Hangi paket?** Sunucu ekleme pakete bağlı değil. Öneri: adım 6–9, **kurulu ve manifestinde platform listesi olan her paket için**
yalnızca bir kez, sunucu eklendikten sonra sorulur ("linux-hardening için bu sunucunun türü?"). Hiç paket yoksa adım atlanır.

## 4. Eski sunucular: ilk açılışta soru

`servers/<ad>/<paket>.vars` içinde `platform_var` satırı **yoksa** ve paketin platform listesi varsa, pakete ilk girişte
(paket menüsü açılırken, bir kez) aynı 6–7. adımlar sorulur. "Sonra" denirse hiçbir şey yazılmaz (işaret dosyası yok):
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
- `platform_choices[].auto_detect: false` → tahmin bu platformu önermez (Raspberry Pi).
- `platform_choices[].apply_vars` + `apply_warnings` (paket oturumunun isteği, **koşullu kabul önerisi**): örn. `{"hardening_pve_apply": true}`.
  Sunucu eklerken **yazılmaz**; yalnızca değişiklik yapan eylemin (`apply`) onayından (`type-server-name`) **sonra**, o koşuya özel
  `-e` ile geçer ve **yalnızca değişken `.vars` dosyasında YOKSA** (kullanıcı bilerek `false` yazdıysa ezilmez). Koşullar:
  1. `apply_vars` doluysa `apply_warnings` boş olamaz ve metin **onay sorusundan önce** gösterilir ("bu koşuda hardening_pve_apply=true verilecek");
  2. `.vars` dosyasına **yazılmaz**; günlük başlığında "bu koşuya özel: hardening_pve_apply=true" satırı çıkar;
  3. yalnızca değişiklik yapan eylemlerde (`preview`/`audit` değil) ve platform **açıkça seçilmişse** (`platform_var` dosyada yazılı; `auto`/belirsiz değil);
  4. yalnızca tanımlı `bool`/`enum` değişkenler; silici/erişimi kesen değişken (`hardening_ufw_prune_ssh`) bulunamaz (iki yönlü denetim: paket ve menü).

**Okuma yeri:** `sistem/Helpers/component_manifest.py` doğrulayıp `kimlik=değer` bilgi satırlarına çevirir (bugün `platform_var` ve
`platforms` böyle okunuyor); `ComponentsManifest.sh` yeni okuyucu işlev ekler; seçim ve yazma `ComponentsVars.sh`
(`comp_vars_set`) üzerinden.

**`sshpocket_api`:** yeni alanlar isteğe bağlı olduğu için **`sshpocket_api` artmaz** (standart zaten "isteğe bağlı alan api'yi artırmaz"
diyor). Bilinmeyen alanlar eski sürümlerde yok sayılır; yalnızca yeni seçim ekranı eski sshpocket'te çıkmaz.

## 6. Tahmin ve `auto` sorunu

- **Tahmin kaynağı:** paket bildirir. Öneri: paket isteğe bağlı salt okunur bir `detect` eylemi (`effects: []`) tanımlar ve
  `SSHPOCKET_RESULT … platform=proxmox-host` döner; sshpocket bunu mevcut yürütme yolundan (host key, ajan, onay kapıları) çalıştırır.
  Alternatif (sade ama sshpocket'e komut koyar): manifestte `detect_command`. **Açık soru:** hangisi?
- **`auto` sorunu:** paket varsayılanı `auto` yaparsa `platform_var` süzmesi (`current_platform`) `auto` değerini platform sanıp tüm
  platformlu değişkenleri gizler. Çözüm: `current_platform` bildirilen `platform_choices` kimlikleri dışındaki değeri (`auto`, boş, bilinmeyen)
  **"belirsiz"** sayar, hiçbir süzme yapılmaz (bugünkü "platform belirlenemezse süzme yok" kuralıyla aynı).
  `platform_choices` hiç yoksa bugünkü davranış aynen sürer.
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

## 11. Açık sorular
0. **`apply_vars` (karar kullanıcıda):** `hardening_pve_apply` bugün elle yazılır. `apply_vars` bunu `apply` onayına taşır; kolaylık, ama gerçek koşuya geçişi bir 'tek seferlik izin'e çevirir. §2 ilkesiyle (apply anında, eylemin kendi onayıyla) uyumlu; yine de güvenlik anlamı değiştiği için onay bekler.

1. Tahmin kaynağı: paketin `detect` eylemi mi, manifestte `detect_command` mı? (öneri: `detect` eylemi)
2. Soru hangi anda: sunucu eklerken mi, paket menüsü ilk açılırken mi, ikisi de mi? (öneri: ikisi, aynı kod)
3. Birden çok paket platform listesi bildirirse sunucu başına tek "tür" mü, paket başına ayrı mı? (öneri: paket başına)
4. `onboarding_vars` için "yıkıcı/erişimi etkiler" ayrımı manifestte açık bir alan mı olsun (`sensitive: true`), yoksa `warn_if` yeterli mi?
5. `doctor` USB/yerel `servers/` farkını göstersin mi?
6. Eski sshpocket'te yeni alanlar yok sayılır; yeni paket + eski sshpocket'te kullanıcı yine elle yazar. Kabul edilebilir mi?
