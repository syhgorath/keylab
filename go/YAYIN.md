# Yayın biçimi: imzalı tek ikili (karar: 2026-11-07)

**Ne dağıtılır:** macOS 27+ ve Apple Silicon (arm64) için tek dosya `sshpocket` (`keylab-X.Y.Z-darwin-arm64`, ad-hoc `codesign`'lı). Intel/universal ve eski macOS için yayın yapılmaz. Web arayüzü ikilinin içinde gömülüdür.
GitHub Release varlıkları (X.Y.Z sürümü için), her biri aynı adla `.sig` ile:

- `keylab-X.Y.Z-macos.zip`: içinde `KeyLab.app` (arm64 ikili, ad-hoc imzalı; çift tıklanır; `go/scripts/package-app.sh` üretir). Kullanıcıların asıl indireceği dosya. Eski adlı kopyası yoktur. `./sshpocket update` bu zip'i hedeflemez (yalnız ikilinin kendi güncellemesi içindir); `.app` içindeki ikiliyi değiştirmek paketin imza mührünü bozar (denendi: `codesign --verify` "code has no resources but signature indicates they must be present"). `.app` kullanıcıları **Ayarlar > Güncelleme**'yi kullanır (4.0.11+): bu zip indirilir, `.sig` release anahtarıyla doğrulanır, `.app` bütünüyle değiştirilir (aşağıya bakın).
- `keylab-X.Y.Z-darwin-arm64` (USB ve `update` için)
- (4.0.5'ten itibaren `-darwin-universal`, `-darwin-amd64` ve eski `sshpocket-X.Y.Z-…` adlı kopyalar YAYIMLANMAZ.)
- `.app` paketi `LSMinimumSystemVersion` 27.0 ile gelir; eski macOS'ta açılmaz.

İmza: SSHSIG, ad alanı `sshpocket-release`, anahtar `release_signers` dosyasındaki (kök klasör ya da `sistem/`) public key. Kabuk dönemindeki anahtar aynen kullanılır.

## USB / klasör düzeni
```
sshpocket            ikili (arm64)         ← Release'ten indirilen dosya bu adla
start.command        çift tıkla başlatıcı      ← go/launcher/start.command
release_signers      güncelleme güven noktası  (ya da sistem/release_signers)
components.lock      paket kilidi
components/          kurulu paketler
servers/             sunucular ve anahtarlar (kullanıcı verisi)
VERSION              (isteğe bağlı; ikili kendi sürümünü içinde taşır)
```
Argümansız çalıştırınca yerel web arayüzü açılır. Komut satırı: `./sshpocket components list|sync|update|upgrade|check-new|prune|validate|report|run`, `doctor`, `version`, `update [--check]`, `eject`.

## Sürüm çıkarma (bakımcı; hiçbir adım otomatik değildir)
1. Bir PR ile `VERSION`'ı artırın ve CHANGELOG'daki `[Unreleased]` bölümünü `[X.Y.Z]` yapın; PR'ı birleştirip `main`'i çekin. Gömülü güven dosyaları (`go/embedded/`: `components.lock`, `release_signers`, `index_signers`) kökle aynı olmalıdır (`go/scripts/sync-embedded.sh`; `go test` ve `build-release.sh` denetler).
2. `bash go/scripts/release-publish.sh --dry-run` ne yapacağını gösterir (sürüm, commit, CHANGELOG bölümü). Sonra `bash go/scripts/release-publish.sh`: `build-release.sh` ile derler ve **imzalar** (imza parolası sorulur), `vX.Y.Z` etiketini itip GitHub Release'ini açar (varlıklar + `.sig`, notlar CHANGELOG'dan) ve yüklenen dosyaların SHA-256'sını yerel dosyalarla karşılaştırır. İmza anahtarı `SSHPOCKET_SIGNING_KEY` ile verilir; anahtarı yalnız bakımcı kullanır.
3. Eski sürümlü bir kopyada denenir: `.app` için **Ayarlar > Güncelleme**, ikili için `./sshpocket update --check`, sonra `./sshpocket update`.
4. Paket sürümü değiştiyse imzalı indeks (`sshpocket-index` deposu: `scripts/index-publish.sh add|sign`, ardından push) ve gerekirse gömülü `components.lock` ayrıca güncellenir. Kilit bir paketi bir **repoya** sabitler: indeks farklı repoyu gösterirse karar `other-repo` olur ve kurulmaz (bkz. `docs/package-standard.md` §3).

## Kendini güncelleme (`update`)
Son sürüm GitHub'ın `releases/latest` yönlendirmesinden okunur (tam olarak bu reponun etiket adresi olmalı). `keylab-X.Y.Z-darwin-arm64` indirilir. Yerleştirmeden ÖNCE: imza doğrulanır (anahtar yoksa güncelleme kapalı), ikilinin bildirdiği sürüm (`version`) etiketle aynı olmalı (eski imzalı sürümün yeniden sunulmasını engeller), geri sürüme düşülmez, iki onay istenir. Yerleştirme atomiktir (aynı klasörde geçici dosya + rename), eski ikili `.update-backup/sshpocket-<eski>` olarak saklanır. `servers/`, `components/` ve anahtarlara dokunulmaz. Başarıdan sonra YENİ ikiliyle `components sync` çalışır.

## Uygulama içi güncelleme (`KeyLab.app`, 4.0.11+)
Ayarlar > Güncelleme: kullanıcı "Güncellemeyi denetle" der (KeyLab kendiliğinden internete çıkmaz); son sürüm `releases/latest` yönlendirmesinden okunur, sürüm notları GitHub'dan alınıp yalnız DOM düğümleriyle çizilir. "Güncelle ve yeniden başlat": `keylab-X.Y.Z-macos.zip` + `.sig` indirilir → **release anahtarıyla imza doğrulanır** → güvenli açma (zip-slip, sembolik bağ, boyut sınırı; `._*`/`.DS_Store` yan dosyaları atlanır) → tek `.app` ve ikilinin `version` çıktısı beklenen sürümle aynı → `codesign --verify` → `.app` bütünüyle değiştirilir, eskisi `.keylab-update-backup/` altına yedeklenir (hata olursa geri alınır) → yeniden başlatma yardımcısı. Yalnız `KeyLab.app` olarak çalışıp klasörü yazılabilirse düğme çıkar (nedenler: `not_bundle`, `translocated`, `not_writable`). Güncelleme **kurulu sürümün koduyla** çalışır: bir düzeltme ancak o sürümü kuran kullanıcılarda etkilidir (ör. exFAT/FAT USB'de `._` yan dosyası düzeltmesi 4.0.13'te geldi; ondan eski sürümler bir kez elle geçer).

## Anahtar rotasyonu
Yeni anahtarı `release_signers` dosyasına ekleyin; yeni sürümü ESKİ anahtarla imzalayıp yayınlayın (kurulu kopyalar bunu kabul eder), kopyalara yeni `release_signers` bu güncellemeyle gitmez — dosyayı USB'de elle değiştirmeniz gerekir. (Kabuk dönemindeki arşiv güncellemesi dosyayı da taşıyordu; tek ikilide güven noktası bilerek ikiliden ayrıdır.)

## Henüz yapılmadı / denenmedi
Gerçek bir Release ile uçtan uca `update` ve uygulama içi güncelleme (sahte ağ + gerçek `ssh-keygen` imzaları ve gerçek `.app` + `codesign` ile test edildi; USB'de gerçek deneme sonucu bu belgede yoktur); `build-release.sh`'ın imzalama adımı (anahtarınızı ben kullanmam); Gatekeeper/notarization (ad-hoc imza yetmeyebilir: internetten indirilen ikilide ilk açılışta sağ tık > Aç gerekebilir).
