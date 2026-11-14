<p align="center"><img src="img/logo.jpg" alt="sshpocket" width="140"></p>

# KeyLab

> **KeyLab**, eski adıyla **sshpocket**. Komut ve dosya adları (sshpocket.conf, `sshpocket` ikilisi, imza ad alanı) şimdilik değişmedi.

[![ci](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml/badge.svg)](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml)
[![Sürüm](https://img.shields.io/github/v/release/syhgorath/sshpocket)](https://github.com/syhgorath/sshpocket/releases/latest)
[![İndirme](https://img.shields.io/github/downloads/syhgorath/sshpocket/total)](https://github.com/syhgorath/sshpocket/releases)
[![Yıldız](https://img.shields.io/github/stars/syhgorath/sshpocket?style=flat)](https://github.com/syhgorath/sshpocket/stargazers)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

🇹🇷 Türkçe · 🇬🇧 [English](README.en.md)

macOS için, USB bellekten (veya herhangi bir klasörden) çalışan **tek dosyalık** bir SSH sunucu yöneticisi: yerel web arayüzü + komut satırı.
Her sunucu `servers/<ad>/` altında bir klasördür; passphrase macOS Keychain'de durur.
İmzalı **paketlerle** (şimdilik sunucu sertleştirme, saat/locale ayarı ve kurulum) sunucuyu yönetebilirsiniz.

> **Sürüm 4 ile sshpocket (şimdi KeyLab) Go'da yeniden yazıldı.** Eski kabuk (bash) motoru depodan kaldırıldı; son hâli git geçmişinde ve `kabuk-referansi` etiketinde duruyor.
> Go sürümü onunla yan yana karşılaştırmalı testlerle doğrulandı; kabuğun çıktıları `testdata/` altında altın dosyalar olarak saklanır.

> **Durum: ham / erken aşama.** 🛠️
> Bu, kişisel bir HomeLab aracından çıkan bir proje. [Claude](https://claude.com/claude-code) ile
> elden geçirildi ve ilk hâline göre epey toparlandı, ama hâlâ geliştirilecek çok yeri var:
> pürüzler, eksik testler ve yazılmamış özellikler olabilir. Sorunlar projenin hamlığından
> kaynaklanır; bunun için kimseye kızmayın 😄 — issue ve PR'larla yardım etmeniz çok makbule geçer.
> İstemci olarak yalnızca macOS'ta denendi (sunucu tarafında neyin denendiği: [docs/test-edilen-ortamlar.md](docs/test-edilen-ortamlar.md)). Kendi sunucularınızda kullanmadan önce kodu okuyun;
> kullanım sorumluluğu size aittir (bkz. [LICENSE](LICENSE)).

> **Otomatik başlamaz.** macOS, Linux ve modern Windows USB takılınca script çalıştırmaz (güvenlik gereği).
> Elle başlatın: `KeyLab.app`'e çift tıklayın (ya da Finder'da `start.command`, ya da `./sshpocket`).

## Kurulum (macOS)

1. [Release sayfasından](https://github.com/syhgorath/keylab/releases/latest) `keylab-X.Y.Z-macos.zip` dosyasını indirin ve açın: `KeyLab.app` çıkar.
2. Nasıl kullanacağınıza göre yerleştirin:
   - **Normal:** `KeyLab.app`'i Uygulamalar'a sürükleyin. Verileriniz `~/.keylab/` altında tutulur (ilk açılışta oluşur).
   - **USB / taşınabilir:** `KeyLab.app`'i USB belleğe (ya da herhangi bir klasöre) koyun ve yanında bir `servers/` klasörü açın. Sunucularınız ve anahtarlarınız (`servers/`) orada kalır. **Ama bilgisayarda yine de iz kalır:** durum ve günlükler (`~/.local/share/keylab`), paketler için Python ortamı (`~/.cache/keylab`) ve Keychain kayıtları (passphrase, knock dizisi) bu Mac'te durur. Hangi klasörlerin kullanıldığını Ayarlar sayfasındaki yollar bölümünden görebilirsiniz.
3. `KeyLab.app`'e çift tıklayın; arayüz tarayıcıda açılır. Kapatmak için arayüzdeki **Çıkış** düğmesi (USB'den çalışıyorsanız anahtarları ajandan silip USB'yi de çıkarır).
4. **İlk açılışta macOS uyarı verebilir** ("doğrulanamadı"): uygulama Apple'ın noter onayından geçmedi (ücretli Developer ID gerekir), ama **imzalıdır**. Sağ tıklayıp **Aç**'ı seçin (macOS 15 ve sonrasında: Sistem Ayarları → Gizlilik ve Güvenlik → "Yine de aç"). Güvenmeden önce indirdiğiniz dosyayı aşağıdaki [İlk indirmeyi doğrulama](#i̇lk-indirmeyi-doğrulama) adımıyla kontrol etmenizi öneririz.
5. **Güncelleme:** `KeyLab.app` için yeni sürümün zip'ini indirip eskisinin yerine koyun (verileriniz `servers/` ya da `~/.keylab/` içinde kalır). `./sshpocket update` komutu tek başına ikili içindir; `.app` paketinin içindeki ikiliyi değiştirmek paketin imza mührünü bozar, bu yüzden `.app` için önerilmez (Finder'dan açılışının bundan etkilenip etkilenmediği denenmedi).
6. **Python (yalnızca paketler için):** sunucu yönetimi, terminal, anahtarlar ve port knocking Python olmadan çalışır. Paketler (sertleştirme, saat/locale, kurulum) Ansible kullanır ve Python ≥ 3.12 ister; yoksa KeyLab ilk paket eyleminde nasıl kuracağınızı söyler (kendisi kurmaz).

## Gereksinimler

| Ne | Not |
|---|---|
| macOS (Apple Silicon veya Intel) | Tek ikili hem arm64 hem amd64 içerir |
| `ssh`, `ssh-keygen`, `ssh-add`, `ssh-copy-id`, `security`, `osascript`, `diskutil` | Hepsi macOS ile gelir; `./sshpocket doctor` eksik olanı söyler (hiçbir şey kurmaz) |
| **Python ≥ 3.12** | Yalnızca **paketler** (Ansible) için; `brew install python@3.13`. sshpocket gerekli sanal ortamı kendisi kurar |

İmza doğrulama, indirme, arşiv ve port yoklaması ikilinin içindedir; `curl`, `tar`, `shasum`, `nc` ve `bash` gerekmez.

## Hızlı başlangıç

```bash
./sshpocket                         # yerel web arayüzünü başlatır (127.0.0.1, jetonlu adres)
./sshpocket doctor                  # ön koşul denetimi (yalnızca okur)
./sshpocket components list         # paketler: kilit ↔ kurulu durumu
./sshpocket components run utility durum ubuntutest   # bir paket eylemini komut satırından çalıştır
./sshpocket version
```

Arayüzde sunucu ekleme (ad, adres, kullanıcı, port), anahtar adımları (üretme, sunucuya gönderme, ajana ekleme),
yeniden adlandırma, silme, değişken düzenleme ve paket eylemleri (önizleme → onay → uygulama) vardır.
Arayüz yalnızca `127.0.0.1`'e bağlanır; oturum çerezi ister.

## Düzen (USB / klasör)

```
sshpocket          ikili (Release'ten indirilen universal dosya bu adla)
start.command      çift tıkla başlatıcı
release_signers    güncelleme güven noktası (imzacı public key)
index_signers      paket indeksi güven noktası
components.lock    paket kilidi;  components/ kurulu paketler
servers/<ad>/      server.env + id_ed25519_<ad>   (kullanıcı verisi, repoya girmez)
```

**Veri nereye gider?** `./sshpocket` yanında `servers/` varsa (USB ya da klasör) **taşınabilir** çalışır: her şey o klasörde kalır.
Yoksa kurulu gibi çalışır: ilk açılışta `~/.keylab/servers` ve `~/.keylab/components` oluşturulur. İsterseniz `-servers`/`-components` ya da `KEYLAB_SERVERS` ile yer gösterebilirsiniz.

## Özellikler

- **Sunucu başına klasör**, Keychain'de passphrase (diske yazılmaz), `ssh-add` ile ajana ekleme (Keychain parolasıyla; yanlışsa yeniden kaydetme sorusu).
- **Anahtarı sunucuya gönderme** (`ssh-copy-id`): anahtar zaten yetkiliyse bir şey yapmaz; parola yalnızca ssh'ın kendi isteminde girilir; `PasswordAuthentication no` bilerek otomatik değildir.
- **Paketler:** imzalı, kilitli (`components.lock`). Her eylem önizleme, önkoşul ve onay kapılarından geçer; sudo parolası Ansible'ın kendi isteminden alınır; günlükler `0600` ve parola maskelenerek arşivlenir. Ayrıntı: [docs/paketler.md](docs/paketler.md) · hardening: [docs/hardening-kullanim.md](docs/hardening-kullanim.md).
- **Güncelleme:** `./sshpocket update` (aşağıya bakın) ve paketler için `components update | upgrade`.
- **Rapor:** `components report` paket sürümü değişince "ne değişti, hangi sunucum etkileniyor" gösterir.
- **USB çıkarma:** `./sshpocket eject` (USB'deki anahtarları ajandan silip çıkarır). Arayüzde üst çubuktaki **Çıkış** düğmesi aynı işi yapar ve uygulamayı kapatır.
- **Test edilenler:** [docs/test-edilen-ortamlar.md](docs/test-edilen-ortamlar.md) — neyin denenip **neyin denenmediği** orada.

- **`~/.ssh/config` içe aktarma:** `./sshpocket servers import-ssh-config [dosya]` (private key kopyalanmaz; `IdentityFile` yolu `KEY` olarak yazılır).
- **Port knocking (istemci):** arayüzdeki sunucu adımlarından ya da `./sshpocket servers knock <ad>`; dizi Keychain'de durur, "Terminal'de aç" bağlanmadan önce otomatik gönderir. Sunucu tarafı (knockd) bu araçta yoktur; klasik knocking şifreli değildir.
- **Keychain parolası:** `./sshpocket servers keychain <ad>` (kaydet/güncelle/sil, kayıttan sonra anahtarı açarak doğrular).
- **Ayarlar:** `sshpocket.conf` (`STATUS=on|off`, `STATUS_TIMEOUT=1-5`; ortam değişkeni `SSHPOCKET_STATUS` önceliklidir). `./sshpocket settings` değerleri gösterir, `settings set STATUS_TIMEOUT 3` yazar.
- **Gömülü terminal:** sunucu sayfasında "Gömülü terminal" tarayıcıda `ssh -tt` oturumu açar; en çok 4 oturum, pencere küçültülünce oturumlar açık kalır ve altta bir çubukta durur (macOS ve Linux; Windows'ta henüz yok). Yerel shell açılmaz, yalnız kayıtlı sunucuya bağlanır; açma/kapama durum klasöründe `terminal-audit.log` dosyasına yazılır.
- **Sunucu karşılaştırma ve raporlar:** "Karşılaştır" sayfası sunucuları sütun sütun yan yana koyar ve farklı olan satırları işaretler (Markdown/CSV/JSON dışa aktarma; CSV hücreleri formül enjeksiyonuna karşı korunur). "Raporlar" dört şablonlu (filo, sunucu denetimi, Lynis bulguları, drift) yazdırılabilir rapor üretir (görsel, Markdown, JSON). Adı parola/token/secret geçen değişkenlerin değerleri maskelenir.
- **`~/.ssh/config` içe aktarma (arayüz):** "Sunucu ekle" sayfasından; tarayıcı yalnız takma adı gönderir, adres/kullanıcı/port sunucuda dosyadan yeniden okunup doğrulanır.
- **Envanter yedeği:** Ayarlar > Envanter yedeği. Yedek sır içermez (private key yolu, knock, parola/token adlı değişkenler yok) ama sunucu adreslerini içerir. İçe aktarma önce önizleme gösterir; var olan sunucular yalnız siz seçerseniz değişir ve doğrulama uygulamanın kendi kurallarından geçer.
- **İş izleyici:** çalışan eylem "Arka plana al" ile küçültülür; üst çubuktaki iş düğmesi süreyi/sonucu gösterir ve sayfa yenilense bile çalışan işe yeniden bağlanır. İş çalışırken başka eylem başlatılamaz.
- **Gizlilik kilidi:** `L` tuşu ya da Ayarlar'dan boşta süresiyle ekranı gizler. Bu bir parola kilidi DEĞİLDİR, yalnızca yan masadakinin ekranı görmesini engeller; varsayılan kapalıdır.
- **Vurgu rengi:** Ayarlar'dan 8 vurgu rengi (açık/koyu temada WCAG AA kontrastı doğrulanmış).
- **Uygulama kataloğu:** `items` bildiren paketlerde (ör. kurulum) sunucu sayfasında uygulama kartları; öğeye özel Önizle/Kur/Yükselt/Kaldır, önkoşul kapısı öğe ve mod kapsamlıdır (docs/package-standard.md).
- **Dil:** arayüz Türkçe ve İngilizce (üst çubuktaki TR/EN düğmesi).

### Go sürümünde olmayanlar (kabuk referansında vardı)

Terminal menüsü (fzf dâhil) **bilerek taşınmadı**; sunucu **taşıma** (`--migrate`, eski `sistem/Modules` düzeni) taşınmadı.
Durum göstergesi, port kapalı ve knock etkinse 🔒 "knock gerekir" gösterir; panodaki "ssh komutunu kopyala" knock etkinse önce knock'u gönderen tek bir komut verir. Bakımcı aracı: `./sshpocket components lock <paket> <sürüm> [--repo sahip/depo] [--write]` (`components.lock` girdisini imzayı doğrulayarak üretir; `--write` olmadan yalnız gösterir).

## Güncelleme

Otomatik değildir; siz başlatırsınız: `./sshpocket update --check` (yalnızca bakar) ve `./sshpocket update` (indir → **imzayı doğrula** → sürümü teyit et → onaylarsanız kur).
Güncelleme yalnızca bu reponun GitHub Release'lerinden HTTPS ile inen ve **bakımcının imza anahtarıyla imzalı** ikiliyi kurar; imza doğrulanamazsa, ikilinin bildirdiği sürüm etiketle uyuşmazsa ya da geri sürüme düşülecekse hiçbir şey kurulmaz; imza anahtarı tanımlı değilse güncelleme **kapalıdır**.
Yerleştirme atomiktir, eski ikili `.update-backup/` altına yedeklenir; `servers/`, `components/` ve anahtarlarınıza dokunulmaz. Ayrıntı: [go/YAYIN.md](go/YAYIN.md).

### İlk indirmeyi doğrulama
`update` imzayı kendisi doğrular, ama **ilk kopyayı** (Release sayfasından indirdiğiniz dosyayı) doğrulayamaz: ona güveniniz GitHub'a ve HTTPS'e dayanır.
İsterseniz çalıştırmadan önce kendiniz doğrulayın (OpenSSH ≥ 8.0, macOS'ta hazır). Anahtarın parmak izi `SHA256:0gjAyj8b7VlmtHIFbwmgAmKjK06sw/a5wdEoE8UECwU`;
bunu bakımcıdan **başka bir yoldan** da teyit etmeniz, depodaki dosyaya körü körüne güvenmekten iyidir.

```bash
# Release'ten keylab-X.Y.Z-macos.zip ve .sig dosyalarını indirdikten sonra
# (komut satırı ikilisi için keylab-X.Y.Z-darwin-universal ve .sig; aşağıda dosya adlarını ona göre değiştirin):
cat > /tmp/sshpocket_signers <<'EOF'
sshpocket-release namespaces="sshpocket-release" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKVJS3mGlCaOT/l8XfR0TxuKxqLUs6PpVDa9TYqm0fLn
EOF
ssh-keygen -Y verify -f /tmp/sshpocket_signers -I sshpocket-release -n sshpocket-release \
  -s keylab-X.Y.Z-macos.zip.sig < keylab-X.Y.Z-macos.zip      # "Good ... signature" yazmalı
```

## Güvenlik modeli

- **Passphrase diske yazılmaz.** Keychain'de durur (`USBMonitor_<ÖNEK>_Passphrase`),
  `ssh-add`'e `SSH_ASKPASS` ile doğrudan aktarılır.
- `server.env` yalnızca IP, kullanıcı, port (ve ops. key yolu, knock ayarı) içerir. Gizli bilgi **içermemelidir**; knock dizisi de Keychain'dedir.
- TOTP/2FA gizli anahtarlarını bu klasörde **saklamayın**; telefondaki uygulamada tutun.
- `servers/`, `id_ed25519*` ve günlükler `.gitignore` ile dışarıda tutulur.
- Güncellemeler ve paketler imzalıdır; güncelleme elle başlatılır (yukarıya bakın). Kendini güncelleyen ikili de aynı imza anahtarına bağlıdır. Paket eylemleri sunucuya yazmadan önce önizleme ve onay kapılarından geçer.
- IP/kullanıcı/port değerleri komut çalıştırılmadan önce doğrulanır (kullanıcı `-` ile başlayamaz).
- Sunucu host key'i ilk bağlantıda size sorulur (otomatik kabul yoktur).
- Özel anahtarınızı USB'de taşıyorsanız USB'yi (APFS şifreli birim) şifreleyin.
- Güvenlik açığı bildirimi için: [SECURITY.md](SECURITY.md).

## Yapı

```
go/                          Go kaynağı (ana ürün): main + internal/{manifest,trust,runner,servers,cli,report,findings,selfupdate,eject,doctor,settings}
go/web/                      gömülü web arayüzü
go/YAYIN.md                  yayın biçimi ve sürüm çıkarma adımları
go/scripts/build-release.sh  universal ikiliyi derler ve İMZALAR (bakımcı aracı)
components.lock              paket kilit dosyası;  components/ kurulu paketler (repoya girmez)
servers/<ad>/                server.env, id_ed25519_<ad>   (kullanıcı verisi)
docs/                        ayrıntılı belgeler (paketler, güncelleme, test edilen ortamlar, standart)
```

## Geliştirme

```bash
cd go
go vet ./... && go test ./... -short       # uzun karşılaştırmalı testler için -short'u kaldırın
go build -o keylab .
bash go/scripts/keylabtest.sh           # yerel deneme klasörünü (~/Developer/keylabtest) günceller ve KeyLab'ı oradan başlatır
```

Karşılaştırmalı testler Go sonucunu `testdata/*.json` altındaki altın dosyalarla (eski kabuk/Python motorunun kaydedilmiş çıktıları) karşılaştırır; kabuk gerekmez.
Kayıtları yenilemek için eski motor `git checkout kabuk-referansi -- referans/kabuk` ile geri getirilir ve `KEYLAB_KAYIT_KABUK=1 go test ./internal/<paket>` çalıştırılır (bkz. `go/internal/shellgold`).

Katkı için [CONTRIBUTING.md](CONTRIBUTING.md), planlananlar için [ROADMAP.md](ROADMAP.md).

## Destek

sshpocket ücretsiz ve MIT lisanslı kalacak. Faydalı bulduysanız [GitHub Sponsors](https://github.com/sponsors/syhgorath) üzerinden destek olabilirsiniz; yıldız, hata bildirimi ve katkı da en az onun kadar değerli.

## Lisans

MIT, bkz. [LICENSE](LICENSE).
