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

Public anahtarı `sistem/release_signers` dosyasına tek satır olarak ekleyin
(`AAAA...` yerine `.pub` dosyasındaki değeri yazın):

```
sshpocket-release namespaces="sshpocket-release" ssh-ed25519 AAAA...
```

Commit'leyin. **Bu satır, kullanıcıların kurulu kopyalarına gömülen güven noktasıdır**; bir kullanıcı bu
anahtara sahip bir sürüm kurduktan sonra, sonraki güncellemeleri yalnızca bu anahtarla doğrular.

## 2) Her sürüm

### Önerilen: tek komut (`scripts/release-publish.sh`)
Sürüm PR'ı **birleştikten** ve `main`'in CI'ı yeşil olduktan sonra:

```bash
git checkout main && git pull --ff-only
SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release scripts/release-publish.sh --dry-run   # ne yapacağını gösterir, HİÇBİR ŞEY yapmaz
SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release scripts/release-publish.sh              # onay ister (y/N), sonra yayınlar
```

Betik sırayla: `main` dalında, çalışma alanı temiz, `HEAD == origin/main`, `CHANGELOG`'da sürüm başlığı var, imza anahtarı var, Release henüz yok, mevcut etiket başka commit'i göstermiyor, `main` CI'ı **bu commit için** yeşil mi diye denetler (herhangi biri tutmazsa **hiçbir şey yapmadan** durur, etiket bile itilmez). Sonra etiketi atıp iter, `release.sh` ile imzalı paketi üretir (passphrase sorulur), `gh release create` ile Release'i **CHANGELOG notlarıyla** açıp iki dosyayı yükler ve yüklenenlerin SHA-256 özetini yerel dosyalarla karşılaştırır. USB güncellemesi bu betiğin işi değildir: menüden **Güncelle**.

- Yarıda kalırsa (örn. ağ koptu) aynı komutu tekrar çalıştırın: etiket zaten bu commit'teyse yeniden etiketlemez.
- `--yes` onayı atlar, `--no-ci-check` CI denetimini atlar (önerilmez).
- Gerekenler: `gh` (oturum açılmış) ve `ssh-keygen`.

### Elle (yedek yol)

1. `VERSION`'ı artırın, `CHANGELOG.md`'de `[Unreleased]` içeriğini yeni sürüm başlığına taşıyın.
2. `bats test` ve ShellCheck temiz olsun; commit'leyin ve `git push` yapın, CI'ın yeşil olmasını bekleyin.
3. Etiket:

   ```bash
   git tag -a vX.Y.Z -m "sshpocket X.Y.Z" && git push origin vX.Y.Z
   ```

4. İmzalı paketi üretin (passphrase sorulur):

   ```bash
   SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release ./scripts/release.sh
   ```

   Betik çalışma alanının temiz olduğunu, etiketin HEAD'i gösterdiğini ve CHANGELOG başlığını denetler,
   `git archive` ile paketi üretir, imzalar ve **yayından önce kendisi doğrular**.
5. GitHub'da `vX.Y.Z` Release'ini açın ve `dist/sshpocket-X.Y.Z.tar.gz` ile `dist/sshpocket-X.Y.Z.tar.gz.sig`
   dosyalarını **Assets**'e yükleyin. Dosya adları tam olarak bu biçimde olmalı.
6. Kendiniz deneyin: eski sürümlü bir kopyada `./sshpocket update`.

## Neden "kaynak arşivi" değil de kendi paketimiz?
GitHub'ın otomatik "Source code" arşivleri imzalı değildir ve içerikleri (sıkıştırma ayrıntıları) sabit
garanti edilmez. İmzalı paketi biz üretiriz; `.gitattributes` ile `test/`, `scripts/`, `.github/` gibi geliştirme
dosyaları paketin dışında kalır.

## Güven modeli (kısaca)
- Kurulu kopya, `sistem/release_signers` içindeki anahtarlarla doğrular. Yeni sürümün kendi `release_signers`
  dosyası **kurulumdan sonra** devreye girer; yani bir güncelleme, yalnızca *mevcut* güvenilen anahtarla
  imzalıysa kabul edilir.
- Hesap/depo ele geçirilse bile saldırgan imza anahtarına sahip değilse sahte paket kurulamaz.
- İmza, paketin ne olduğunu kanıtlar ama **güncel** olduğunu kanıtlamaz; bu yüzden uzak sürüm kurulu sürümden
  büyük olmalı ve paketin içindeki `VERSION` etikete eşit olmalıdır.

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
bash scripts/lock-component.sh linux-hardening 0.2.0 --repo syhgorath/linux-hardening
# doğruysa yaz
bash scripts/lock-component.sh linux-hardening 0.2.0 --repo syhgorath/linux-hardening --write
git diff components.lock          # gözle kontrol; sonra PR → release
```
Araç; paket + `.sig` indirir, arşivi ve imzayı (`sshpocket-component`) doğrular, `component.json`'u denetler ve sha256'yı yazar.
Doğrulama başarısızsa ya da paket kurulamazsa kilide **yazmaz**.

