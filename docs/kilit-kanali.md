# Tasarım: kilit kanalı (paket sürümü için sshpocket sürümü gerekmesin)

> **Durum (2026-10-17): TASARIM, §9 kararları ALINDI, kod yok.** Kodlama zamanı ayrıca kararlaştırılır (§11 sırasıyla, küçük PR'lar).

## 1. Sorun
`components.lock` imzalı **sshpocket sürümüyle** gelir. Yeni bir paket sürümü (örn. `linux-hardening 0.2.1`) kullanıcıya ancak **yeni bir sshpocket sürümü** çıkınca ulaşır:
`lock-component.sh` → kilit PR'ı → sshpocket sürümü → imza/etiket/Release → kullanıcı `--update`. Güvenlik açısından çok sağlam (kullanıcının makinesine sessizce hiçbir şey girmez),
ama bakımcıda darboğaz: her paket sürümü, istemcide hiçbir değişiklik olmasa da tam bir sshpocket sürümü demek.

## 2. Hedef ve kapsam dışı
**Hedef:** Kilidi sshpocket sürümünden **bağımsız**, ayrı imzalı bir dosya olarak dağıtmak; istemci onu **onayla** alsın.
**Değişmeyenler:** (1) yalnızca bakımcı anahtarıyla imzalı paketler çalışır; (2) hiçbir şey kullanıcıya sormadan kurulmaz/çalışmaz; (3) USB çevrimdışı da çalışır.
**Kapsam dışı (şimdilik):** üçüncü taraf depolar/anahtarlar (§10), otomatik/sessiz güncelleme, paket içeriğini kilit dışında bir yerden almak.

## 3. Bugün
| Parça | Durum |
|---|---|
| `components.lock` | INI: `[paket]` → `repo`, `version`, `sha256`, `signers`; sshpocket sürümüyle gelir (`--update` onu da yeniler) |
| İmza ad alanları | `sshpocket-release` (sshpocket kendisi), `sshpocket-component` (paketler); anahtar: `sistem/release_signers` |
| `--components check-new` | Kilitten yeni yayın var mı? **Yalnızca bilgi**; kurmaz |
| `lock-component.sh` | Bakımcı aracı: arşivi indirir, imzayı/`component.json`'u doğrular, kilit satırını üretir |

## 4. Tehdit modeli
| Saldırgan / hata | Ne yapabilir | Önlem (bu tasarımda) |
|---|---|---|
| Ağ üzerinde araya giren (MITM, TLS inceleyen güvenlik yazılımı) | Kilit dosyasını değiştirmek | İmza (çevrimdışı anahtar); TLS yalnızca taşıyıcı |
| GitHub hesabı/repo ele geçirilirse | Sahte kilit veya eski kilit sunmak | İmza anahtarı GitHub'da DEĞİL; **sayaç** + **son kullanma** |
| **Geri alma (rollback)** | Eski, zafiyetli bir kilidi/sürümü yeniden sunmak | Tek yönlü `serial` sayacı; yerelde görülen en yüksek değer saklanır |
| **Donma (freeze)** | Güncellemeyi sonsuza dek gizlemek | `expires` (90 gün): süresi dolan kilit YENİ kurulum yapmaz (kurulu olanı bozmaz) |
| **Karışık eşleştirme (mix-and-match)** | Bir kilidin satırlarını başka sürümlerle karıştırmak | Kilidin TAMAMI tek imzayla imzalanır |
| Kötü niyetli/hatalı paket sürümü | Sunucularda root ile kod çalıştırmak | Paket arşivi ayrıca imzalı + sha256 kilitte; **izin farkı** onayı (§6); `validate` kapısı |
| Sızan/yanlış kilit | Zafiyetli sürüm yaygınlaşır | **İptal (kill switch)** satırı: o sürüm çalıştırılmaz (§7) |
| İmza anahtarının çalınması | Her şey | Çevrimdışı/passphrase'li anahtar; rotasyon için `release_signers` yeni sshpocket sürümüyle (kanal DEĞİL) değişir |

## 5. Tasarım
### 5.1 Kanal dosyaları
Kilit, GitHub Release'te **ayrı bir imzalı yapıt** olarak yayınlanır: `components.lock` + `components.lock.sig`. İsimlendirme: sshpocket reposunda `kilit-<serial>` etiketli Release'ler
(taşınan etiket YOK). İstemci en yenisini mevcut `releases` API çağrısına benzer şekilde sorar (`comp_latest_version` zaten böyle çalışır).

### 5.2 Kilit biçimine eklenenler
Mevcut `[paket]` bölümleri aynen kalır. Başa bir kanal bölümü eklenir:
```
[kanal]
serial = 12
expires = 2027-01-15
min_sshpocket = 3.6.0
```
Bugünkü istemci bilinmeyen anahtarları **reddeder** (`components.lock:N: bilinmeyen anahtar: serial`; doğrulandı): `[kanal]` içeren bir kilit eski bir sshpocket'te baştan geçersiz sayılır, yanlış yorumlanmaz (fail-closed). Bu yüzden kanal yalnızca `[kanal]` bölümünü tanıyan sürümle (§11 adım 1) kullanılır; eski istemciler kanalı hiç sorgulamaz. Gömülü kilit (sürümle gelen) aynen çalışır ve `[kanal]` içermeyebilir (serial = 0 sayılır).

### 5.3 İmza
Ayrı ad alanı: **`sshpocket-lock`** (aynı anahtar olabilir ama ad alanı farklı: bir ad alanı için üretilmiş imza başka amaçla yeniden kullanılamaz). Doğrulama: `ssh-keygen -Y verify -n sshpocket-lock` + `release_signers`.

### 5.4 Yerel durum (USB'de ve `--update`'e dayanıklı)
`components/.kanal/` altında: `components.lock` (son kabul edilen kanal kilidi) ve `serial` (şimdiye dek görülen en yüksek). **Etkin kilit = gömülü ve kanal kilitlerinden `serial`'i yüksek olan** (eşitse gömülü).
`--update` gömülü kilidi yenilese de kanal kilidi (serial'i daha yüksekse) geçerli kalır; gömülü olanın serial'i kanalınkini geçerse kanal kilidi artık gereksizdir.

### 5.5 İstemci akışı (`--components channel update`)
1. Kanal Release'ini bul, `components.lock` + `.sig` indir (yalnızca HTTPS, boyut sınırı).
2. İmzayı (`sshpocket-lock`) doğrula. **Başarısızsa hiçbir şey yazma.**
3. `[kanal]`'ı doğrula: `serial` > yerelde görülen en yüksek, `expires` geçmemiş, `min_sshpocket` ≤ kurulu sürüm. Tüm `[paket]` bölümlerini mevcut doğrulayıcıyla doğrula.
4. **Farkı göster** (kullanıcıya): yeni paket / yeni sürüm / kaldırılan / iptal edilen. Yeni PAKET eklenmesi ayrıca büyük bir onay ister (§9).
5. Onay verilirse `components/.kanal/`ye yaz (atomik) ve `serial`'i güncelle. Kurulum ayrı adımdır: `--components sync` (mevcut akış: sha256, paket imzası, arşiv denetimi, bütünlük kaydı).

## 6. İzin farkı (paket güncellenirken)
Yeni sürüm kurulmadan önce, kurulu `component.json` ile yenisi karşılaştırılır ve **yetki genişlemesi** açıkça gösterilir, onay alınır:
yeni eylem, `effects`'e eklenen `destructive`/`changes-config`/`installs-package`, `requires_sudo`'nun açılması, `confirm`'in gevşemesi (paket gevşetemez ama kontrol edilir), `runner` değişikliği, yeni `vars`.
Daralma ya da değişmeme sessiz geçer. (Bu, `components validate`'in manifest bilgisinin doğal devamıdır.)

## 7. İptal (kill switch)
Kilit, bir sürümü **engelli** işaretleyebilir: `[linux-hardening]` altında `blocked = 0.2.0` (virgülle birden çok). Engelli sürüm `run`'da reddedilir
(*"bu sürüm bakımcı tarafından geri çekildi; `--components channel update`"*) ve menüde ❌ görünür; kurulu dosyalar silinmez. Güvenlik düzeltmeleri için bunun sshpocket sürümünü beklememesi kanalın en değerli kullanımıdır.

## 7b. Süre dolumu
`expires` geçmişse: **yeni kurulum/güncelleme yapılmaz**, kurulu ve kilitle eşleşen paketler çalışmaya devam eder (kullanıcıyı kilitlemeyiz), ekranda uyarı çıkar. Süre: 90 gün; bakımcı süre dolmadan kanalı yeniden imzalar (`lock-channel.sh`; sürüm gerekmez), hatırlatma issue'su §9/5.

## 8. Komutlar ve menü
```
./sshpocket components channel status     # etkin kilit, kaynağı (gömülü/kanal), serial, bitiş, iptaller
./sshpocket components channel update     # §5.5; farkı gösterir, onay ister
./sshpocket components check-new          # (bugünkü) bilgi; kanal varsa kanaldan okur
```
Menü: 📦 Paketler ekranında "Güncelleme var" rozeti; seçilince fark ve onay.

## 9. Kararlar (2026-10-17, bakımcı ile)
| # | Soru | KARAR |
|---|---|---|
| 1 | Kanal YENİ paket ekleyebilsin mi? | **Evet, ama ayrı ve belirgin onayla.** Her ekleme zaten bakımcının incelemesinden ve imzasından geçer; asıl risk anahtar çalınmasıdır ve bu sürüm imzasında da vardır. Yeni paket, güncellemeden farklı, büyük uyarılı bir onay ister ("yeni paket: X; şunları yapabilir") |
| 2 | Kanal imzası için ayrı anahtar mı? | **Aynı anahtar, ayrı ad alanı** (`sshpocket-lock`). Anahtar süresiz kalır; rotasyon (eski anahtarın imzaladığı "yeni anahtar" bildirimi) ayrı bir tasarım, ihtiyaç doğunca |
| 3 | İzin farkı zorunlu mu? | **Evet**, ilk sürümden itibaren (§6) |
| 4 | `expires` süresi | **90 gün.** Yenileme elle: `scripts/lock-channel.sh` kilidi yeniden imzalar (anahtar değişmez, sshpocket sürümü GEREKMEZ) |
| 5 | Yenileme hatırlatması | Yayındaki kilidin `expires`'ine bakan zamanlanmış bir GitHub işi, 30 günden az kalınca issue açar (imza yine bakımcıda; anahtar CI'a konmaz). Kullanıcı tarafında `doctor`/kanal ekranında kalan gün uyarısı |
| 6 | Eski istemciler | Etkilenmez: kanalı hiç sorgulamazlar; sorgulasalar bile bilinmeyen anahtardan dolayı dosyayı reddederler (fail-closed) |
| 7 | Barındırma | sshpocket reposunun Release'leri (yeni altyapı yok) |

**Çalışma modeli:** başkası paket yazar (şablonla) ve bakımcıya gönderir; bakımcı `components validate` ve kod incelemesinden sonra imzalar, kilide ekler. İmza = inceleme onayı. Kanal yalnızca bu eklemenin bir sshpocket sürümüne bağlı olmasını kaldırır.

## 10. Daha sonra: çok kaynaklı depo
Başka yazarların depoları ve kullanıcının eklediği anahtarlar (apt `signed-by` mantığı) **ancak gerçek bir üçüncü taraf ihtiyacı doğarsa**. Gerekenler: kullanıcının anahtarı açıkça eklemesi (büyük uyarı),
depo başına sayaç/süre, anahtar iptal listesi, kaynak başına izin sınırı. Bu belgedeki §5-§7 o zaman da kullanılabilir (kanal = bir kaynak).

## 11. Uygulama sırası (küçük PR'lar)
1. Kilit ayrıştırıcısı `[kanal]` bölümünü tanır; `serial/expires/min_sshpocket` doğrulaması; `blocked` (testler: kötü biçim, eski serial, süresi dolmuş).
2. `channel status` + etkin kilidi seçme (gömülü ↔ kanal) + yerel durum klasörü.
3. `channel update`: indirme, `sshpocket-lock` imzası, fark gösterimi, onay, atomik yazım.
4. İzin farkı (`sync`/`update` öncesi) ve `blocked` zorlaması (`run`'da ret).
5. Bakımcı aracı: `scripts/lock-channel.sh` (kilidi üretir, `serial`'i artırır, `expires` yazar, imzalar, Release yükleme komutunu yazdırır).
6. Menü rozeti, belgeler, e2e senaryoları.

**Saldırı testleri (zorunlu):** yanlış ad alanı, bozuk imza, eksik/kesik dosya, eski `serial`, süresi dolmuş, `min_sshpocket` büyük, bilinmeyen alan, devasa boyut, `blocked` sürümü çalıştırma, gömülü ↔ kanal öncelik, `--update` sonrası kanal kilidinin korunması.
