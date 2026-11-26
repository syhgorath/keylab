# Taslak: şema temelli bileşen ayarları (`settings`)

> **Durum: P1 kodlandı** (doğrulayıcı, durum hesabı, ayar kartı; 2026-11-26). Paket tarafında `settings` bildiren yayımlanmış sürüm **henüz yok**; gerçek VM'de denenmedi.
> P2/P3 tasarım aşamasında. Kapsam kararları kullanıcıyla verildi (§1); geçerli kurallar [package-standard.md](package-standard.md) §6'dadır, bu belge gerekçe ve yol haritasıdır.
> **P1'de taslaktan sapmalar:**
> 1. Alan yalnız **üst düzeydedir**; `items[].settings` **desteklenmez**. Neden: yayımlanmış sürümler `items[]` içindeki bilinmeyen alanı reddeder (paket kurulamaz); üst düzeydeki bilinmeyen alanı yok sayar (kod okunup denendi).
>    §4 ve §9 karar 2'deki `items[].settings` bu yüzden ertelendi; açılırsa paketin en düşük KeyLab sürümünü şart koşması gerekir.
> 2. Kartlar `items` yerine üst düzey **`setting_cards`** ile bildirilir (ayar `card` alanıyla bağlanır; bildirilmezse değişken `group`'u). Kart katalogda yalnız durum özetidir; ayarlar §5'teki tablo yerine **ayar penceresinde** düzenlenir ve sunucudaki değerle birlikte görünür (kullanıcı kararı, 2026-11-26). `summary_keys` yok.
> 3. `state.strip_prefix` eklendi (örn. `["server ", "pool "]`): istenen değerden atılan önekler; paket sonuçta yalnız host adını bildirir.
> 4. `state.keys[]` ile `compare` `set`, `ordered` ya da `present` olmalıdır (`exact` reddedilir).
> 5. Son okuma başarısızsa ya da paketin başka bir sürümündense durum **bilinmiyor** olur (eski değerle "sapma" uydurulmaz).
>
> Önceki öneri [`go/PAKET-BOLUMLERI-TASLAK.md`](../go/PAKET-BOLUMLERI-TASLAK.md) (`sections`) uygulanmadı.

## 1. Amaç ve verilen kararlar

Sunucu sayfasında her bileşenin (chrony/NTP, Docker Engine, nginx, MariaDB, Redis…) **kendi kartı** olsun; kartta o bileşenin **bütün ayarları**
(örn. Docker için ağlar: bridge, host, macvlan; healthcheck; log sürücüsü) tek yerden görülüp değiştirilsin. Yeni bir bileşen eklemek **arayüz
kodu yazmayı gerektirmesin**: paket yalnız bildirim (`component.json`) yazsın, arayüz kartı bildirimden çizsin.

Kullanıcı kararları (2026-11-21):

| Karar | İçerik |
|---|---|
| Sunucudaki durum | Kart **istenen değeri** ve sunucudaki **son okunan gerçek değeri yan yana** göstermeli (sapma/drift görünümü). |
| Okuma | **Elle** ("kontrol et" = mevcut salt okunur eylem). Sayfa açılınca **otomatik okuma YOK**; kart "son okuma: X dk önce" ve mevcut "güncel olmayabilir" rozetiyle açılır. |
| Pilot | **chrony/NTP** (küçük, düşük riskli, eylemleri var, gerçek VM'de doğrulandı). Sonra **Docker Engine + ağlar**; uygulama başına ayarlar (healthcheck, restart, limitler) ondan sonra. |
| Başlangıç | "Basitten başla": düz tipler ve gruplar önce, nesne listeleri sonra. |

**Bu taslağı doğuran olay:** gerçek bir VM'de snapshot geri alınınca `utility.sources` kayboldu ve saat 28 gün geri kaldı; sayfa "ayarlı" demeye
devam etti. İstenen ile gerçeği yan yana göstermek bunu bir bakışta yakalardı.

## 2. Bugün elimizde olan (yeniden yazılmayacak)

[package-standard.md](package-standard.md) §6/§8'e göre:

- **Değişkenler** (`vars[]`): `string`, `bool`, `int`, `path`, `cidr`, `list[<tür>]`, `enum` (+`values`); `group`, `default`, `description`, `warn_if`,
  `required`, `min/max`. Değerler sunucu başına `.vars` dosyasında; doğrulamayı sshpocket yapar.
- **Eylemler**: `effects` (`[]` salt okunur, `installs-package`, `changes-config`, `destructive`), `requires_sudo`, `requires[]` (önkoşul), `confirm`,
  `outputs.keys` (makine-okunur sonuç satırı: `SSHPOCKET_RESULT <id>.<eylem> anahtar=değer …`), `warnings[]`.
- **Katalog** (`items[]`, en çok 30; öğe başına `actions`, `params` en çok 12) ve öğe kapsamlı önkoşul kapısı.
- Arayüz: Değişkenler penceresi (gruplu, tipli), eylem etiketleri, "güncel olmayabilir" rozeti, `preview` önkoşulu.

**Eksik olan üç parça** (bu taslağın konusu):

1. **Zengin tipler:** nesne listesi (Docker ağı: ad, sürücü, alt ağ, geçit…), ve ayara özel kısıtlar.
2. **Ayara bağlı risk işareti:** bugün yalnız eylem düzeyinde var; "saati sıçratır", "erişimi kesebilir" ayarın kendisinde görünmeli.
3. **Ayar ↔ eylem ↔ gerçek değer bağı:** bir ayar hangi eylemle uygulanır ve sunucudaki gerçek değeri **hangi okuma sonucundan** gelir?

## 3. İlkeler

1. **Geriye uyumlu ve isteğe bağlı:** `settings` yeni, isteğe bağlı bir **üst düzey** alandır; `sshpocket_api` artmaz; bilmeyen eski sürüm yok sayar (standart §10). `items[]` içine konursa eski sürümler paketi reddeder.
2. **Güven kapıları gevşemez:** `settings` yeni bir çalıştırma yolu açmaz. Bir ayarı uygulamak = **paketin tanımlı bir eylemini** çalıştırmak (`apply_via`);
   o eylemin `effects`, `requires`, `confirm`, `requires_sudo` kuralları aynen geçerlidir. Paket kuralları gevşetemez, yalnız sıkılaştırabilir.
3. **Tek kaynak:** istenen değer hâlâ `.vars` dosyasındadır; şema onu *tanımlar ve gösterir*, ikinci bir depo yoktur.
4. **Gizli değer yok** (standart §6): parola/anahtar/token ayar olamaz (Docker kayıt defteri kimlik bilgileri gibi).
5. **Düz metin:** etiket ve açıklamalar düz metindir; arayüz HTML olarak yorumlamaz.

## 4. Bildirim: `settings`

`items[]` öğesinin yeni, isteğe bağlı alanı (öğesiz paketlerde üst düzeyde aynı alan). Öğe başına en çok 40 ayar.

| Alan | Tür | Anlamı |
|---|---|---|
| `id` * | string `^[a-z][a-z0-9_]*$` | Öğe içinde benzersiz |
| `label` * | string ≤ 60 | Kartta görünen ad |
| `var` * | string | Bağlı değişken (`vars[]`'te tanımlı). İstenen değer buradan okunur/yazılır |
| `description` | string ≤ 300 | Yoksa değişkenin açıklaması kullanılır |
| `risk[]` | liste | Sabit sözlük: `access-loss` (erişimi kesebilir), `restarts-service` (servisi yeniden başlatır), `time-jump` (saati sıçratır), `data-loss` (veriyi etkiler). Arayüz renkli etiket gösterir; `risk_note` (≤ 160) açıklama ekler |
| `apply_via[]` * | eylem adları | Bu ayarı uygulayan **mevcut** eylem(ler). Kartta "Uygula" bunlardan açılır (önkoşul kapısı aynen) |
| `state` | nesne | Sunucudaki **gerçek** değerin kaynağı (aşağıda). Yoksa "okunamaz": kart yalnız istenen değeri gösterir |

`state` nesnesi:

| Alan | Anlamı |
|---|---|
| `action` * | **Salt okunur** eylem (`effects: []`, tercihen `requires_sudo` yok): "kontrol et" düğmesi bunu çalıştırır |
| `key` ya da `keys[]` * | O eylemin `outputs.keys`'inde bildirilmiş sonuç anahtarı. Liste ayarları **birden çok anahtarla** bildirilir (`keys`: sıralı anahtar adları, örn. `["ntp_kaynak_1", …, "ntp_kaynak_8"]`; her biri tek öğe, boş olanlar atlanır): her değer ≤ 64 karakter sınırına uyar ve **yeni bir arka uç mekanizması gerekmez** |
| `compare` | `exact` (varsayılan) · `set` (liste, sıra önemsiz) · `ordered` (liste, sıra önemli) · `present` (dolu mu) |
| `source` | `result` (varsayılan, **P1**: eylemin sonuç satırı anahtarları). `summary` (eylemin yazdığı JSON dosyası) **P2'dir ve bugün YOKTUR:** `outputs.summary` genel bir mekanizma değil (doğrulayıcı onu serbest metin sayar; özet dosyası okuma yalnız Lynis için sabit kodlu), nesne listeleri için yeni KeyLab kodu gerekir |

**Durum değerlendirmesi (kart satırı başına):**

| Durum | Koşul |
|---|---|
| **uyumlu** | istenen dolu ve gerçek (son okuma) `compare` kuralına göre eşit |
| **sapma** | istenen dolu, gerçek farklı |
| **bilinmiyor** | `state` var ama bu sunucuda henüz okunmadı (ya da okuma bu ayarın sonrasından eski) |
| **yönetilmiyor** | istenen **boş** (= "dokunma", standardın mevcut anlamı): sapma sayılmaz |
| **okunamaz** | `state` bildirilmemiş (örn. tek seferlik bir seçenek) |

"Eskimiş" kuralı yeni değildir: bir okuma eylemi sonuç kaydından sonra **yazan bir eylem** çalıştıysa mevcut "güncel olmayabilir" rozeti çıkar.

**Sonuç satırının sınırları (KeyLab ayrıştırıcısı, `internal/runner/exec.go`):** değer yalnız `[A-Za-z0-9_.:+-]` karakterlerinden oluşur ve en çok **64** karakterdir
(`/`, boşluk ve virgül **yasak**). Sonuçları:

- **Liste ayracı `+`** (mevcut `kurulu=docker:docker+nginx:native` sözleşmesi gibi); virgül değil.
- **Listeler** tek değerde taşınamaz (8 NTP sunucusu 64 karakteri aşar): **P1'de her öğe ayrı anahtarla** bildirilir (`state.keys[]`; bir eylemde **en çok 20** anahtar vardır, kalan kotaya dikkat).
  **Nesne listeleri** (Docker ağları) bu yolla taşınamaz; onlar için ayrı bir mekanizma P2'de tasarlanır (`source: summary`, yeni KeyLab kodu).
- **Karakter kuralı karşılaştırmaya girer:** paket gerçek değeri aynı kuralla temizleyerek bildirir (`Europe/Istanbul` → `Europe_Istanbul`; utility `durum.yml` bunu yapıyor). Bu yüzden
  KeyLab, `source: result` karşılaştırmasında **istenen değere de aynı kuralı** uygular (izin verilmeyen karakter → `_`, sonra en çok 64 karaktere kırp). Kullanıcı ve arayüz her yerde
  gerçek adı görür (`Europe/Istanbul`); uyumluysa "sunucuda" sütunu da istenen yazımı gösterir, sapma varsa ham değer görünür.

## 5. Kart düzeni (arayüz)

```
┌ chrony (NTP) ─────────────────────────────────────────  durum: ntp_senkron=yes · 6 kaynak · ofset 0.0 ms ┐
│ Ayar                     İstenen                Sunucuda (son okuma 3 dk önce)         Durum              │
│ NTP sunucuları           0.tr.pool.ntp.org      0.tr.pool.ntp.org                      ✔ uyumlu           │
│ Saat sıçratma  [time-jump]   true               (okunamaz)                             —                  │
│ Saat dilimi              Europe/Istanbul        Etc/UTC                                ⚠ sapma            │
│ [Kontrol et]  [Önizle]  [Uygula…]                                                                       │
└─────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

- **Kontrol et** = `state.action` eylemleri (salt okunur, sudosuz olanlar arka planda). Başka otomatik okuma yok.
- **Önizle / Uygula** = `apply_via` eylemleri; mevcut kapılar (önkoşul, onay, snapshot) değişmez.
- Kart başlığındaki özet, paketin okuma eyleminden gelen birkaç anahtarla çizilir (`summary_keys`, örn. `ntp_senkron`, `kaynak_sayisi`, `offset_ms`).
- Sapma satırında "istenene getir" kısayolu yoktur (bilerek): uygulama her zaman paketin onaylı eylemiyle olur.

## 6. Örnek: chrony kartı (utility 0.2.0 üzerine)

Utility'de zaten var: değişkenler `utility_timezone`, `utility_locale`, `utility_ntp_servers`, `utility_ntp_clock_from_controller`,
`utility_ntp_makestep`; eylemler `durum`, `preview`, `apply`, `ntp_durum`, `ntp_ayarla`; `ntp_durum`'un anahtarları `hizmetler`, `ntp_servis`,
`ntp_senkron`, `offset_ms`, `kaynak_sayisi`, `sapma_ms`, `secili_kaynak`, `saat_sapma_kontrolcuye_gore_sn`, `yonetilen_kaynak`, `proxmox`.

```json
"items": [{
  "id": "chrony", "label": "Zaman eşitleme (chrony)",
  "description": "NTP kaynakları, saat dilimi ve dil.",
  "actions": ["ntp_durum", "durum", "preview", "ntp_ayarla", "apply"],
  "summary_keys": ["ntp_senkron", "kaynak_sayisi", "offset_ms"],
  "settings": [
    { "id": "ntp_servers", "label": "NTP sunucuları", "var": "utility_ntp_servers",
      "apply_via": ["ntp_ayarla", "apply"],
      "state": { "action": "ntp_durum", "compare": "set",
                 "keys": ["ntp_kaynak_1", "ntp_kaynak_2", "ntp_kaynak_3", "ntp_kaynak_4", "ntp_kaynak_5", "ntp_kaynak_6", "ntp_kaynak_7", "ntp_kaynak_8"] } },
    { "id": "makestep", "label": "Saat sıçratma", "var": "utility_ntp_makestep",
      "risk": ["time-jump"], "risk_note": "Saat 1 sn'den fazla sapmışsa tek seferlik adımlanır.",
      "apply_via": ["ntp_ayarla", "apply"] },
    { "id": "clock_from_controller", "label": "Kontrol makinesinden saat", "var": "utility_ntp_clock_from_controller",
      "risk": ["time-jump"], "risk_note": "Kontrol makinesinin saati yanlışsa sunucu da yanlış olur.",
      "apply_via": ["ntp_ayarla", "apply"] },
    { "id": "timezone", "label": "Saat dilimi", "var": "utility_timezone",
      "apply_via": ["apply"], "state": { "action": "durum", "key": "timezone" } },
    { "id": "locale", "label": "Sistem dili (LANG)", "var": "utility_locale",
      "apply_via": ["apply"], "state": { "action": "durum", "key": "locale" } }
  ]
}]
```

**Utility paketinde gereken küçük değişiklikler:**

1. `ntp_durum` **gerçek sunucu listesini** bildirmeli (bugün yalnız `yonetilen_kaynak` sayısı var: `grep -Ec '^(server|pool) ' /etc/chrony/sources.d/utility.sources`). Önerilen: aynı dosyadan
   host adları (`awk '/^(server|pool) /{print $2}'`) **`ntp_kaynak_1 … ntp_kaynak_8`** anahtarlarıyla sonuç satırına eklenir ve `outputs.keys`'e yazılır (mevcut 10 + 8 = **18 ≤ 20** anahtar sınırı). Her değer tek host adıdır
   (`server`/`pool` öneki ve `iburst` olmadan); kullanılmayan anahtarlar boş kalır. Bu *kanonik biçim* (yalnız host adı) paketin işidir; istenen girişler (`host`, `server host`, `pool host`) karşılaştırmadan önce aynı biçime
   indirgenir. Sonuç satırı değer kuralı geçerli: host adlarında `[A-Za-z0-9_.:+-]` dışında karakter olmaz; ≤ 64 karakter.
2. `durum` zaten `timezone` ve `locale` veriyor (gerçek VM'de doğrulandı). Sonuç satırı `/` karakterini `_` yaptığı için `timezone=Europe_Istanbul` görünür; istenen değer `Europe/Istanbul`'dur
   (standart ad). **Ek iş yok:** karşılaştırmada KeyLab istenen değere aynı karakter kuralını uygular (§4, §9), yani sahte sapma çıkmaz ve paket değişmez.
3. `apply_via` eylemleri zaten mevcut; yeni eylem gerekmez.

## 7. Örnek taslak: Docker Engine ve ağlar (v2; yalnız yön)

Bu kısım **kesin değildir**; nesne listesi tipi ve yeni eylemler gerektirir.

| Ayar | Tür | Not |
|---|---|---|
| `log_driver`, `log_max_size`, `log_max_file` | `enum` / `string` / `int` | `daemon.json`; uygulanınca Docker yeniden başlar (`restarts-service`) |
| `registry_mirrors` | `list[string]` | kimlik bilgisi **yok** (ilke 4) |
| `live_restore` | `bool` | yeniden başlatmada konteynerleri korur; riski düşürür |
| `default_address_pools` | `list[object]` (`base` cidr, `size` int) | VPN/LAN ile çakışırsa erişim kesilir (`access-loss`) |
| `networks` | `list[object]` (`name`, `driver` ∈ bridge/macvlan/ipvlan, `subnet`, `gateway`, `parent` yalnız macvlan) | `host` ayrı: ağ nesnesi değil, uygulama başına seçim (v3). macvlan/ipvlan yanlış `parent` ile sunucu ağını bozabilir → `access-loss` + önizleme + snapshot onayı |

Yeni okuma eylemi `docker_durum` (salt okunur): `docker info`/`daemon.json`/`docker network ls` çıktısını bildirir. Skaler ayarlar (log sürücüsü, `live_restore`) sonuç satırıyla (P1 mekanizması) okunur; **ağ listesi gibi nesne listeleri
için yeni bir okuma mekanizması gerekir** (`source: summary`: eylemin kullanıcı klasörüne yazdığı JSON'u KeyLab okur; doğrulayıcı, boyut/yol sınırları ve test P2'de tasarlanır).

**Sınır (bilerek):** konteyner/uygulama başına ayarlar (healthcheck, restart politikası, kaynak limitleri) v3; "ham yapılandırma düzenleyici" planlanmıyor (güvenlik kapılarını atlatır).

## 8. Doğrulayıcı kuralları (`components validate`)

Hata (❌): `var` `vars[]`'te yok; `apply_via` eylemi tanımlı değil; `state.action` tanımlı değil ya da `effects` boş değil; `state.key` o eylemin `outputs.keys`'inde
yok; `compare` sözlükte değil; `risk` sözlük dışı; ayar kimliği öğe içinde yinelenen; öğede 40'tan fazla ayar; `state.keys[]`'teki bir anahtar eylemin `outputs.keys`'inde yok ya da `key` ile `keys` birlikte verilmiş; `source: summary` (P1'de desteklenmez).
Uyarı (⚠️): `state.action` `requires_sudo` istiyor (arayüz arka planda çalıştıramaz, terminalde açılır); `risk` boş ama bağlı eylem `destructive`; `description` yok.
Eski sshpocket `settings`'i bilmediğinde alanı yok sayar ve öğe bugünkü gibi çizilir.

## 9. Kararlar (2026-11-21)

| # | Soru | Karar |
|---|---|---|
| 1 | Nesne listesinin `.vars` biçimi | **Tek satır JSON** (şemayla doğrulanır) |
| 2 | `settings` nerede? | `items[].settings`; `item_var` yalnız öğenin eylemleri onu kullanıyorsa zorunlu |
| 3 | Okuma granülerliği | Öğe başına **tek** okuma eylemi, birden çok anahtar |
| 4 | Risk sözlüğü | `access-loss`, `restarts-service`, `time-jump`, `data-loss` yeterli başlangıç için |
| 5 | Çok dillilik | Şimdilik yalnız Türkçe; `label_en` sonra |
| 6 | Değer normalizasyonu | Paket gerçek değeri sonuç satırı sözleşmesi izin verdiği kadar bildirir; **karşılaştırmada KeyLab istenen değere aynı karakter kuralını uygular** (§4). `/` ve `,` sözleşmede yasak olduğundan "paket aynen bildirsin" mümkün değil |
| — | **Saat dilimi standardı** | **`Europe/Istanbul`** (IANA adı; `/usr/share/zoneinfo`, `timedatectl`). `Europe_Istanbul` yalnız sonuç satırındaki temizlenmiş görünümdür; chrony/NTP'nin saat dilimi ayarı yoktur (UTC), saat dilimi işletim sistemi ayarıdır |

**Doğrulandı (2026-11-21, kod okundu):** `outputs.summary` genel bir mekanizma değildir (yalnız serbest metin; özet dosyası okuma Lynis'e sabit kodlu). P1 bu yüzden `state.keys[]` ile çok anahtarlı liste kullanır; `outputs.keys` sınırı en çok 20 anahtardır.
**Kalan açık konu:** nesne listeleri için okuma mekanizması (`source: summary`), P2'de.

## 10. Aşamalar

| Aşama | İçerik | Kapsam |
|---|---|---|
| **P1** | `settings` düz tiplerle + gerçek değer + durum rozetleri; **chrony kartı** | utility `ntp_durum`'a `ntp_kaynak_1..8` anahtarları; KeyLab: doğrulayıcı (`manifest.go`) + `setting.*` bildirim satırları + `/api/server`'a ayarlar + `server.js` kartı + jsdom testleri. Kart verisi zaten `a.keys`/`a.lastTime`'dan gelir |
| **P2** | `list[object]` tipi, **nesne listesi okuma mekanizması (`source: summary`, yeni KeyLab kodu)**; **Docker Engine + ağlar** | yeni `docker_durum`/`docker_ayarla` eylemleri; arayüzde liste düzenleyici |
| **P3** | Uygulama başına ayarlar (healthcheck, restart, limitler); nginx/MariaDB/Redis kartları | `items` ile bağlanır |

**Test planı (P1):** jsdom ile kart durumları (uyumlu/sapma/bilinmiyor/yönetilmiyor/okunamaz ve eskimiş rozeti); doğrulayıcı testleri (§8) ve mutasyon; gerçek VM'de
bugünkü senaryonun tekrarı: snapshot geri al → kart "sapma" göstermeli → `ntp_ayarla` → "uyumlu".
