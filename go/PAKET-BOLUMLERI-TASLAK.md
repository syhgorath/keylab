# Paket bildirimi: `sections` alanı (TASLAK — onaylanmadı)

Amaç: arayüz paket sayfasını "uygulama kartları" olarak çizebilsin. Her kart: **ne kurulur / değişir**, **neden**, kartın **kendi değişkenleri**, kartın **önizleme sonucu**.
Bu belge standarda (`docs/package-standard.md`) EKLENMEDİ; onaylanınca §4/§6'ya işlenir.

## Uyumluluk (doğrulandı)
- İsteğe bağlı üst düzey alan; `sshpocket_api` artmaz (standart §10).
- Bilinmeyen üst düzey alan bugünkü doğrulayıcıdan geçer (deneme: utility 0.1.2'ye `sections` eklendi → "✅ uyumlu").
- `sections` yoksa arayüz eskisi gibi çalışır: `vars[].group` değerlerinden otomatik kart üretir (başlık = grup adı, metin = değişken açıklamaları). Yani paketler sırayla geçebilir.
- Mevcut `menu`, `items`/`item_var` (yazılım kataloğu) ve `platform_choices` ile çakışmaz.

## Şema

```json
"sections": [
  {
    "id": "ntp",
    "title": "NTP",
    "summary": "Sunucu saatini ağdaki zaman kaynaklarıyla senkron tutar; yanlış saat ssh sertifikalarını, TOTP'yi ve apt'yi bozar.",
    "installs": ["chrony"],
    "touches": ["/etc/chrony/sources.d/utility.sources"],
    "vars_group": "NTP",
    "plan_tokens": ["ntp", "paket", "saat"],
    "order": 30
  }
]
```

| Alan | Zorunlu | Anlam / kural |
|---|---|---|
| `id` | evet | küçük harf/rakam/`-`/`_`, 1-40, pakette benzersiz |
| `title` | evet | düz metin, 1-40 |
| `summary` | evet | **neden**: düz metin, 1-300; kartta gösterilir |
| `installs` | hayır | **ne kurulur**: en çok 10 paket/araç adı (1-60 karakter); boş/yok = paket kurulmaz |
| `touches` | hayır | dokunulan dosya/servis adları, en çok 10 (bilgi amaçlı; yalnızca gösterilir) |
| `vars_group` | hayır | `vars[].group` değeri; kartta düzenlenecek değişkenler buradan gelir. Tanımlı bir gruba işaret etmeli |
| `action` | hayır | kartın kendi eylemi (ör. Lynis için `audit`); tanımlı eylem adı olmalı. `vars_group` ile birlikte de olabilir |
| `plan_tokens` | hayır | önizleme/uygula çıktısındaki `plan=a+b+c` simgelerinden bu karta ait olanlar; arayüz sonucu kartlara böler. Hiçbir karta düşmeyen simge "Diğer" kartına gider |
| `order` | hayır | 0-999, görünüm sırası |

Kurallar (doğrulayıcı reddeder): yinelenen `id`; tanımsız `vars_group`/`action`; sınır aşımı; `summary` eksik; metinlerde denetim karakteri. `installs`/`touches` yalnızca bilgidir — **hiçbir şeyi çalıştırmaz, kapıları (önizleme/onay) değiştirmez**.

Dürüstlük kuralı: `installs` paketin gerçekte yaptığı işle uyuşmalı. Arayüz bu metni "sözleşme beyanı" olarak gösterir; gerçek plan her zaman önizlemeden gelir.

## Örnek: utility 0.1.2

| Kart | summary (öneri) | installs | vars_group | plan_tokens |
|---|---|---|---|---|
| Zaman | Sunucunun saat dilimini ayarlar; log ve cron saatleri bununla uyumlu olur. | — | Zaman | `timezone` |
| Dil | Sistem dilini (LANG) ayarlar; gerekirse locale üretilir. | `locales` | Dil | `locale` |
| NTP | Saati ağdaki kaynaklarla senkron tutar; yanlış saat ssh sertifikası, TOTP ve apt'yi bozar. | `chrony` | NTP | `ntp`, `paket`, `saat` |
| Yedek | Değişmeden önce dosyaları `/var/backups/utility/` altına yedekler. | — | Yedek | — |

## Taslak: linux-hardening
Eylem kartı: **Lynis denetimi** (`action: "audit"`, `installs: ["lynis"]`, ayarı yok → yalnız "neler kurulacak" görünür). Değişken kartları mevcut 12 gruptan (SSH, Güvenlik duvarı, Çekirdek, Denetim, …); 145 değişkenin hepsi için metin yazmak yerine önemli 5-6 grup öne çıkarılır, kalanı "Gelişmiş" altında otomatik kart olur.

## Taslak: kurulum
Kartlar: Docker, nginx, mariadb, redis (`vars_group` = aynı adlı gruplar; `installs` = gerçek paket adları). "Seçim" ve "Ana klasör" grupları üstte ortak ayar olarak kalır.

## Arayüz tarafı (bu alan onaylanınca)
1. Go: `manifest` doğrulayıcıya `sections` kuralları + testler (geçerli/geçersiz örnekler, bozma testi).
2. `/api/server` yanıtına `sections` (yoksa gruplardan türetilmiş) eklenir.
3. Paket sayfası kartları çizer; kart tıklanınca yan panelde değişkenler + önizleme özeti.
4. Paket repoları (utility, kurulum, linux-hardening) kendi `component.json`'larına alanı ekler — **kendi oturumlarının işi**; kullanıcı onayı olmadan iletilmez.
