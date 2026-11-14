# Sürüm çıkarma (bakımcı rehberi)

Güncelleme (`--update`) yalnızca **sizin imza anahtarınızla imzalı** paketleri kurar. Bu rehber, anahtarın
bir kez nasıl kurulacağını ve her sürümün nasıl imzalanıp yayınlanacağını anlatır.

> **Özel imza anahtarı asla** repoya, CI/GitHub Secrets'a, sohbete veya e-postaya konmaz.
> Yalnızca bakımcının makinesinde (tercihen passphrase ile) durur. Kaybolursa/sızarsa aşağıdaki
> "Anahtar rotasyonu" bölümüne bakın.

## 1) Bir kez: imza anahtarını üretin

```bash
ssh-keygen -t ed25519 -a 100 -C "sshpocket release signing" -f ~/.ssh/sshpocket_release
```

- Güçlü bir **passphrase** belirleyin (boş bırakmayın).
- `~/.ssh/sshpocket_release` **özel** anahtardır: paylaşmayın.
- `~/.ssh/sshpocket_release.pub` **public** anahtardır: paylaşılabilir.

Public anahtarı `release_signers` dosyasına tek satır olarak ekleyin
(`AAAA...` yerine `.pub` dosyasındaki değeri yazın):

```
sshpocket-release namespaces="sshpocket-release" ssh-ed25519 AAAA...
```

Commit'leyin. **Bu satır, kullanıcıların kurulu kopyalarına gömülen güven noktasıdır**; bir kullanıcı bu
anahtara sahip bir sürüm kurduktan sonra, sonraki güncellemeleri yalnızca bu anahtarla doğrular.

## 2) Her sürüm

Ürün, macOS için tek imzalı ikilidir (`keylab-X.Y.Z-darwin-universal`; arm64 ve amd64 ayrıca; geçiş için aynı dosyalar eski `sshpocket-X.Y.Z-…` adıyla da). Ayrıntı: [go/YAYIN.md](go/YAYIN.md).

### Önerilen: tek komut (`go/scripts/release-publish.sh`)
1. `VERSION`'ı artırın, `CHANGELOG.md`'de `[Unreleased]` içeriğini yeni sürüm başlığına (`## [X.Y.Z] - GG-AA-YYYY`) taşıyın; PR açın, CI yeşil olunca birleştirin.
2. `main`'in CI'ının da yeşil olmasını bekleyin, sonra:

```bash
git switch main && git pull --ff-only
SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release bash go/scripts/release-publish.sh --dry-run   # ne yapacağını gösterir, HİÇBİR ŞEY yapmaz
SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release bash go/scripts/release-publish.sh              # onay ister (y/N), sonra yayınlar
```

Betik sırayla: `main` dalında, çalışma alanı temiz, `HEAD == origin/main`, `CHANGELOG`'da sürüm başlığı var, imza anahtarı var, Release henüz yok, mevcut etiket başka commit'i göstermiyor, `main` CI'ı (`ci.yml`) bu commit için **yeşil** → sonra **derler ve imzalar** (`build-release.sh`; passphrase sorulur, imzayı `release_signers` ile kendisi doğrular, ikilinin `version` çıktısını denetler) → etiketi atıp iter → Release'i açar (varlıklar + `CHANGELOG` notu) → yüklenen her dosyanın SHA-256 özetini yerelle karşılaştırır. İmza başarısız olursa hiçbir şey itilmemiştir.

- Yarıda kalırsa (örn. ağ koptu) aynı komutu tekrar çalıştırın: etiket zaten bu commit'teyse yeniden etiketlemez; Release açılmışsa betik "zaten var" der, o durumda yalnız eksik dosyaları elle yükleyin.
- `--yes` onayı atlar, `--no-ci-check` CI denetimini atlar (önerilmez).
- Gerekenler: `go`, `gh` (oturum açılmış), `ssh-keygen`, `lipo`/`codesign` (macOS).
- Yayından sonra: eski sürümlü bir kopyada `./sshpocket update --check`, sonra `./sshpocket update` ile deneyin (gerçek Release ile uçtan uca `update` ilk yayında henüz denenmemiştir).

### Elle (yedek yol)
```bash
SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release bash go/scripts/build-release.sh    # go/dist/ altına derler + imzalar (hiçbir şey yüklemez)
git tag -a vX.Y.Z -m "KeyLab X.Y.Z" && git push origin vX.Y.Z
```
Sonra GitHub'da `vX.Y.Z` Release'ini açıp `go/dist/` içindeki dosyaları ve `.sig`'lerini **Assets**'e yükleyin. Dosya adları tam olarak bu biçimde olmalı.

## Neden GitHub'ın otomatik "Source code" arşivi değil?
O arşivler imzalı değildir ve içerikleri (sıkıştırma ayrıntıları) sabit garanti edilmez. Güncelleme yalnızca bizim imzaladığımız ikiliyi kabul eder (`release_signers` ile doğrulanır).

## Güven modeli (kısaca)
- Kurulu kopya, `release_signers` içindeki anahtarlarla doğrular. Yeni sürümün kendi `release_signers`
  dosyası **kurulumdan sonra** devreye girer; yani bir güncelleme, yalnızca *mevcut* güvenilen anahtarla
  imzalıysa kabul edilir.
- Hesap/depo ele geçirilse bile saldırgan imza anahtarına sahip değilse sahte paket kurulamaz.
- İmza, paketin ne olduğunu kanıtlar ama **güncel** olduğunu kanıtlamaz; bu yüzden uzak sürüm kurulu sürümden
  büyük olmalı ve indirilen ikilinin kendi bildirdiği sürüm (`version`) etikete eşit olmalıdır.

## Anahtar rotasyonu / sızıntı
- **Planlı değişim:** yeni anahtar üretin; `release_signers`'a yeni satırı **ekleyin** (eskisini silmeyin),
  sürümü eski anahtarla imzalayıp yayınlayın. Kullanıcılar bunu kurunca yeni anahtara da güvenir. Sonraki
  sürümü yeni anahtarla imzalayın, bir süre sonra eskisini kaldırın.
- **Özel anahtar sızdıysa:** eski anahtarla imzalı güncellemeler artık güvenilir değildir. Kullanıcılara GitHub
  Release notunda ve README'de duyuru yapın; yeni kurulumu elle (`git clone` / release indirme) yapmaları ve
  `release_signers`'ı yeni anahtarla değiştirmeleri gerekir. Bu yüzden anahtarı iyi koruyun.

## Paket sürümünü kilide yazma (linux-hardening gibi)
Bir paket yeni sürüm yayınlayınca, kullanıcılara gitmesi için `components.lock` güncellenir ve yeni bir sshpocket sürümü çıkarılır:

```bash
# önce yalnızca göster (dosyaya dokunmaz)
keylab components lock linux-hardening 0.2.0 --repo syhgorath/linux-hardening
# doğruysa yaz
keylab components lock linux-hardening 0.2.0 --repo syhgorath/linux-hardening --write
git diff components.lock          # gözle kontrol; sonra PR → release
```
Araç; paket + `.sig` indirir, arşivi ve imzayı (`sshpocket-component`) doğrular, `component.json`'u denetler ve sha256'yı yazar.
Doğrulama başarısızsa ya da paket kurulamazsa kilide **yazmaz**.

