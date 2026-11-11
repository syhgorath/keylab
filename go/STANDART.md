# sshpocket GUI standardı (Go denemesi)

Durum: **taslak v0.1**, yalnızca yerelde (`go/` klasörü GitHub dışında). Bu belge, GUI'ye eklenecek her ekranın uyacağı kuralları yazar. "ZORUNLU" olanlar pazarlık konusu değildir; "ÖNERİ" olanlar gerekçeyle esnetilebilir.

Kod haritası: `main.go` (arka uç, salt okunur API) · `web/index.html` (iskelet) · `web/app.js` (çizim ve etkileşim) · `web/tasarim.css` (tasarım jetonları ve bileşenler).

---

## 1. Güvenlik (ZORUNLU)

1. **Yalnızca `127.0.0.1`** dinlenir, port rastgele. Dışa açılan arayüz yok.
2. **Oturum:** her çalıştırmada rastgele jeton üretilir, terminale yazılır. `?t=<jeton>` adresi bir kez açılınca `HttpOnly` + `SameSite=Strict` çerez verilir ve adres temizlenir. Çerezsiz her API isteği 401.
3. **DNS rebinding:** `Host` başlığı sunucunun kendi adresi değilse 403 (sayfa ve API için).
4. **Durum değiştiren istekler (`POST`):** `X-Requested-With: sshpocket` başlığı ve (varsa) aynı `Origin` şart, yoksa 403.
5. **Parola girişi varsayılan olarak işletim sisteminin kendi penceresindedir** (macOS `display dialog … with hidden answer`, System Events ile öne gelir): parola tarayıcıya, eklentilere ve parola yöneticisine hiç uğramaz; Go sunucusu osascript çıktısından alır, pty'ye yazar ve bellekte sıfırlar (uç yalnızca çalışan bir iş varken pencere açar). İptal ya da pencere yoksa sayfa içi kutuya düşülür. Ayarlar'dan "Sayfa içi kutu" seçilebilir; o kutu Chrome'un şifre önerisini tetiklememek için `type=text` + noktalı maske kullanır. Sayfa içi kutunun ayrıntısı:
   **Parola kutusu yalnızca motor parola sorunca açılır** (karar 2026-11-07, "Proxmox tarzı gömülü terminal"): eylemler sahte terminal (pty) altında çalışır; çıktı akar, istem satırı "\n" ile bitmeden bir süre durursa yanıt kutusu çıkar (parola istemiyse `type=password`). Yazılan satır `POST /api/run/input` ile doğrudan pty'ye gider, **saklanmaz, günlüğe yazılmaz, sayfada yankılanmaz** (pty yankıyı kapatır; testle doğrulandı), gönderince alan hemen temizlenir ve gizlenir. Sayfa `Content-Security-Policy` taşır (yalnızca kendi betiği, dışa bağlantı yok, çerçeveye gömülemez). İstemeyen kullanıcı için her eylem penceresinde **macOS Terminal'de aç** yolu durur. **SSH anahtar parolaları ve gizli anahtarlar arayüzde ASLA girilmez** (ssh-agent). Sunucu ekleme formunda parola alanı yok.
6. **Girdi doğrulama:** sunucu adı, kullanıcı, adres, port ve günlük dosya adı sabit düzenli ifadelerle doğrulanır. Yol birleştirmede kullanıcı girdisi doğrudan kullanılmaz (`..` ve mutlak yol reddedilir). Bir dış komut çalıştırılırsa argümanlar listeyle verilir, kabuk dizesi birleştirilmez.
7. **Dış kaynak yok:** CDN, web fontu, harici görsel, izleme yok. Her şey ikiliye gömülür, çevrimdışı çalışır.
8. **Çıktı kaçışı:** sunucudan/dosyadan gelen her metin `textContent` ya da `esc()` ile yazılır. Ham `innerHTML` yalnızca kaçışlanmış parçalardan kurulur.
9. **Yazma eylemleri** (ileride) yalnızca bu belgenin 3. bölümündeki onay akışıyla çalışır; arayüz sessizce komut çalıştıramaz.
11. **Yazan eylem ölçütü:** yıkıcı değil, zorunlu değişkenler tamam, `requires` eylemleri son 30 dk içinde başarılı (motorun `COMP_REQUIRES_TTL`'i ile aynı). İstemci `snapshot` onayını ve `confirm` kuralını (`type-server-name`: ad birebir; `yes-no`: "evet") gönderir; sunucu ikisini yeniden denetler. Motor Terminal'de aynı onayları yeniden ister; arayüz onayı bunun ön adımıdır, yerine geçmez.
10. **Çalıştırılabilir eylem ölçütü (sunucu tarafında denetlenir, istemciye güvenilmez):** salt okunur (`effects` boş), `requires_sudo` yok, `confirm` yok/`none`, `requires` yok, platforma uygun. Çalıştırma sshpocket'in kendi motoruyla (`--components run`) yapılır; arayüz Ansible'ı kendisi kurmaz. Aynı anda tek iş, 10 dk zaman aşımı, stdin boş (parola alınamaz), çıktı maskelenir. Salt okunur ama sudo isteyen eylemler (`terminal`) macOS Terminal'de açılır: komut doğrulanmış adlardan kurulur ve alıntılanır, parolayı kullanıcı Terminal'de girer, arayüz sonucu `result.<eylem>` değişimini yoklayarak gösterir.

## 2. Görünüm

**Genel ilke:** sakin, bilgi yoğun, dikkatli bir operatör aracı. Süs yok. Durum yalnızca renkle verilmez: simge ve metin her zaman eşlik eder (✔ başarılı, ✖ başarısız, — kayıt yok).

**Jetonlar** (`web/tasarim.css`, `:root`; koyu tema `prefers-color-scheme: dark` ile aynı adlar):

| Grup | Adlar |
|---|---|
| Yüzey | `--bg` `--surface` `--surface-2` `--surface-3` `--border` `--border-strong` |
| Metin | `--text` `--text-2` `--muted` |
| Vurgu | `--accent` `--accent-weak` `--accent-ink` |
| Anlam | `--ok` `--warn` `--danger` `--info` (+ `-weak`, `-border` türevleri) |
| Biçim | `--radius` `--sp-1…` `--font` `--mono` `--shadow-1…3` `--focus` `--log-bg` |

Kurallar:
- Ekran kodunda **ham renk (`#abc`) yazılmaz**; her zaman `var(--…)`. Yeni bir renk gerekiyorsa önce jeton eklenir (açık ve koyu birlikte).
- Yazı tipi yalnızca sistem yazı tipleri (`--font`, kod için `--mono`).
- Açık ve koyu tema **ikisi de** çalışmalı (`prefers-color-scheme`).
- Kontrast en az WCAG AA. Odak halkası her etkileşimli öğede görünür (`--focus`).
- Duyarlı: 760 px ve üstü iki/üç sütun, altı tek sütun; yatay kaydırma çıkmaz.
- Hareket azaltma tercihine uyulur (`prefers-reduced-motion`).

**Bileşenler** (yenisini yazmadan önce var olanı kullan): `.card` (`.card-head` + `.card-body`), `.kpi`, `.server` (+ `.status-dot`), `.pkg-row`, `.badge` (`ok/warn/danger/info/plain`), `.pill`, `.notice` (`ok/danger`), `.tl` (zaman çizelgesi), `.modal` (sihirbaz), `.ctx` (bağlam menüsü), `.log-view`, `.empty-state`.

**Düzen:** sol gezinme (Genel: Pano, Sunucular, Paketler · Güvenlik: Lynis, Günlükler), üstte arama ve durum hapları, ortada içerik, panoda sağ panel (Lynis, son eylemler, dikkat, kısayollar).

**Etkileşim:**
- Sunucu satırında **sağ tık**, `F10` ya da `⋯` bağlam menüsünü açar; menüde ok tuşları ve `Esc` çalışır.
- `/` aramaya odaklanır. Her şey klavyeyle erişilebilir.
- Tehlikeli işlem **yavaş ve belirgin**, güvenli işlem hızlı (bkz. 3).
- Boş ve hata durumları her ekran için tanımlıdır (sunucu yok, bağlantı yok, günlük yok…); boş ekran bırakılmaz.

## 3. Eylem ve onay akışı (ZORUNLU, eylemler bağlanınca geçerli)

1. **Önce salt okunur önizleme** (`preview`), sunucuya hiçbir şey yazmadan. Sonuç ("değişecek N", uyarı sayısı) onaydan önce gösterilir.
2. **Sunucu adını yazarak onay.** Ad tam eşleşmeden (büyük/küçük harf duyarlı) "Uygula" düğmesi kapalı kalır.
3. **Snapshot hatırlatması** yazma eylemlerinde her zaman görünür.
4. **Yıkıcı eylem** (`kaldir` vb.) görsel olarak ayrışır (`--danger`) ve ayrı onay metni taşır.
5. **Canlı ilerleme** gerçek çıktıdan beslenir (`SSHPOCKET_PROGRESS` satırları); **simüle edilen ilerleme veya sahte "başarılı" yok.** Bir şey çalıştırılmadıysa arayüz bunu açıkça yazar.
6. **Geri alma güvenlik ağı** (linux-hardening) devredeyse sayaç gerçek zamanlayıcıdan okunur, yaklaşık gösterilmez.
7. **Başarısızlıkta** günlük yolu ekranda yazar ve tek tıkla açılır; hata satırına atlama bulunur.
8. Eylem sonucu, mevcut `results` / `components.log` biçimine yazılır (sözleşme bozulmaz).

## 4. Sözleşme (veri ve paketlerle ilişki)

- GUI **paket koduna dokunmaz.** Paketler Ansible + `component.json` olarak kalır; GUI yalnızca sözleşmeyi okur ve çalıştırır (`durum/preview/apply/...`, `SSHPOCKET_RESULT`, `SSHPOCKET_PROGRESS`).
- Ekranlar paketin `component.json`'undan beslenir; paket başına özel arayüz kodu yazılmaz.
- Veri kaynakları (okuma): `servers/<ad>/server.env`, `components.log`, `linux-hardening.vars`; `components/<paket>/current`; `components/.index/state`; `~/.local/share/sshpocket/<paket>/<sunucu>/` (günlükler, Lynis `out/`).
- Güncelleme ve imza doğrulaması **çekirdeğin sorumluluğu**: imzalı indeks zinciri (HTTPS, imza, seri, süre, paket imzası) GUI'de yeniden yazılmaz; GUI sonucu gösterir. İmza doğrulanamazsa arayüz "güvenli kapanma" gösterir, yükseltme düğmesi kapanır.
- IP, kullanıcı gibi değerler yalnızca yerel ekranda görünür; belgelere, günlüklere ve hata iletilerine kopyalanmaz.

## 5. Kod kuralları

- Çerçeve yok (React/Vue vb.): düz HTML, CSS, JS. Küçük ve okunabilir.
- Arka uç: standart kütüphane tercih edilir; yeni bağımlılık gerekçe ister.
- `app.js` yalnızca `esc()` ile kaçışlanmış metin üretir; `eval`, `new Function`, dışarıdan betik yok.
- Her API ucu `guard` arkasında; yeni uç eklerken 1. bölümün 2–4 maddeleri otomatik gelmeli.
- Salt okunur uç `GET`, durum değiştiren `POST`.

## 6. Kabul listesi (her yeni ekran için)

- [ ] Açık ve koyu temada okunuyor, kontrast yeterli
- [ ] Yalnızca jetonla renk/boşluk (ham renk yok)
- [ ] Klavye ile gezilebiliyor, odak halkası görünüyor
- [ ] Boş, hata ve yükleniyor durumu var
- [ ] Durum renkle birlikte simge/metinle de belli
- [ ] Gerçek veriyle denendi (örnek veri değil)
- [ ] Sayfa yenilenince durum korunuyor (çerez oturumu)
- [ ] 760 px altında kullanılabilir
- [ ] Güvenlik denemeleri: çerezsiz 401, yanlış Host 403, başlıksız POST 403
- [ ] Denenmeyenler ekranda ve PR/özette açıkça yazılı

## 7. Durum (bugün)

| Parça | Durum |
|---|---|
| Pano, Sunucular (simge/liste), Sunucu sayfası (Hardening/Utility/Kurulum eylem kataloğu), Paketler (USB sürümleri, kilit tabanı, sunucu başına son kullanım), Lynis (geçmiş + bulgular), Günlükler (görüntüleyici) | çalışıyor, gerçek veriyle denendi |
| Bağlam menüsü (terminal, kopyala, günlük, Lynis) | çalışıyor; "Terminal'de bağlan" **denenmedi** |
| Ayarlar (tema: otomatik/açık/koyu, sunucu görünümü, kaynak yolları, `sshpocket.conf` salt okunur) | çalışıyor, tarayıcıda denendi |
| **Paket güncelleme** (Paketler → Güncellemeler): motorun `--components update` raporu ayrıştırılır (imzalı indeks, imza/süre/seri/kilit tabanı/depo kapıları motorda; arayüz yeniden uygulamaz), `upgrade <paket>` arayüz içi terminalde çalışır, motorun e/H sorusu Evet/Hayır düğmeleriyle yanıtlanır | çalışıyor: gerçek motorla `update` (hepsi güncel, seri 2); `upgrade` akışı **yalnızca test motoruyla** (gerçek bir yeni sürüm yayımlanınca denenmeli). Rapor ayrıştırma metne dayanır → motora `update --json` önerilir |
| **Sunucu ekleme**: form (sunucu tarafı doğrulama) → `server_write_files` (motorun işlevi; dizin 0700, dosya 0600, var olanın üzerine yazmaz) → adımlar: `ssh-keygen` (pty, passphrase istemleri parola penceresine/kutuya; var olan anahtarın üzerine yazmaz), `ssh_copy_key` (motorun işlevi: parmak izi onayı + ssh-copy-id), `ssh-add --apple-use-keychain`, bağlantı denemesi (`utility durum`), değişken düzenleyiciye geçiş | çalışıyor; kayıt oluşturma ve `ssh-keygen` gerçek motor/araçla denendi (kopya klasörde, gerçek USB'ye yazılmadı); **`ssh-copy-id` ve `ssh-add` adımları gerçek bir sunucuyla henüz denenmedi**. |
| **Sunucu yeniden adlandırma ve silme**: motorun `ModuleManager.sh rename/remove` betiği (yeniden adlandırma: klasör + anahtar dosyaları, hata olursa geri alır; ek olarak kontrolcüdeki günlük/durum klasörleri yeni ada taşınır). Silme: silinecek dosyalar listelenir, özel anahtar varsa ayrı onay kutusu, sunucu adı birebir yazılır (sunucu tarafında yeniden denetlenir), motorun soruları (Keychain) arayüze düşer, isteğe bağlı günlük silme (yalnızca sunucu artık yoksa) | çalışıyor; Go testleri gerçek motorla, arayüz kopya klasörde uçtan uca denendi (gerçek USB'ye dokunulmadı). Keychain kayıtları otomatik taşınmaz/silinmez (motorun davranışı) |
| Sihirbaz | **taslak**: komut çalıştırmaz, 3. bölümün akışını gösterir |
| Eylemler arayüzdeki terminalde (pty): ham çıktı akışı, ANSI temizleme, istem algılama + yanıt/parola kutusu, iptal (Ctrl-C sonra SIGTERM), sonuç özeti, alan alan ilerleme | çalışıyor; sudo'suz `durum` gerçek VM'de denendi, istem/yankı/iptal Go testlerinde doğrulandı; **gerçek sudo parolası istemi arayüzde henüz denenmedi** |
| Salt okunur eylem çalıştırma (yalnızca ölçüte uyanlar; ilk gerçek: utility `durum` ubuntutest'te), canlı çıktı, iptal, sonuç özeti | çalışıyor, gerçek VM'de denendi |
| **Değişken düzenleme** (Değişkenler kartı → Düzenle…): tüm değişkenler gruplu, aranabilir; türe göre denetim (bool/enum seçimi, sayı aralığı, metin); doğrulama ve yazma sshpocket'in kendi `component_run.py check` ve `comp_vars_set/unset` işlevleriyle (menüdekiyle aynı: atomik, 0600, yorumlar korunur); uyarı eşiğinde onay; eylem çalışırken düzenleme kapalı | çalışıyor; Go testleri gerçek motorla, arayüz bir kopya klasörde denendi (gerçek USB dosyaları değişmedi); gerçek VM'de bir eylem düzenlenmiş değişkenle henüz çalıştırılmadı |
| Paket değişkenleri kartı (ayarlı/eksik/varsayılan, salt okunur), eksik zorunlu değişkende eylem kapısı, onayda kullanılacak değişkenlerin gösterimi, "Çıktıyı aç" (son çalıştırma günlüğü) | çalışıyor, gerçek veriyle denendi |
| `preview` eylemleri (utility `preview` ubuntutest'te gerçekten çalıştırıldı: `degisen=0`, `plan=yok`) | çalışıyor; kurulum ve hardening `preview` henüz denenmedi |
| Sudo isteyen salt okunur eylemler: Terminal'de çalıştır (ilk gerçek: kurulum `durum` ubuntutest'te, parola Terminal'de), sonuç otomatik görünür | çalışıyor, gerçek VM'de denendi |
| Değişiklik yapan, yıkıcı olmayan eylemler (`apply`, `kur`, `audit`, `rollback_now`…): plan + önkoşul tazeliği + snapshot + ad/e-h onayı (sunucu tarafında yeniden denetlenir), Terminal'de çalıştırma, `SSHPOCKET_PROGRESS` ile alan alan ilerleme, geri alma güvenlik ağı göstergesi | **gerçek VM'de uçtan uca denendi** (2026-11-07: utility `preview` → `apply` ubuntutest'te, parola penceresiyle; sonuç `degisen=0`, `failed=0`). **Gerçekten değişiklik yapan bir `apply` (örn. hardening) ve alan alan ilerlemenin gerçek akışı henüz denenmedi** |
| **Yıkıcı paket eylemleri** (`kaldir`, `yukselt`…): yazan eylemlerin tüm kapıları (taze önkoşul, eksik değişken yok) **artı** sunucu adı birebir (motorun confirm kuralından bağımsız), snapshot, "geri alınamayabilir" onayı; hepsi sunucu tarafında yeniden denetlenir; paketin tüm uyarıları kısaltılmadan gösterilir; kırmızı biçim | kodlandı; sunucu tarafı reddi Go testinde ve API'de, onay akışı ve çalıştırma **test motoruyla** denendi (gerçek VM'de yıkıcı eylem çalıştırılmadı) |
| Güncelleme denetimi, imza doğrulama | **yok** (çekirdekten gelecek) |
| Boş durum (sunucu/paket/günlük yok), geçersiz `server.env`, 390 px'te yatay taşma yok, klavye odak halkası, adsız düğme/etiketsiz alan taraması | denendi (tarayıcı panelinde) |
| Linux/Windows desteği, gerçek telefon/dokunma, 20+ sunucu, koyu temada her ekran | **denenmedi** |
