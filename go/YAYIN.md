# Yayın biçimi: imzalı tek ikili (karar: 2026-11-07)

**Ne dağıtılır:** macOS için tek dosya `sshpocket` (arm64 + amd64 `lipo` ile birleşik, ad-hoc `codesign`'lı). Web arayüzü ikilinin içinde gömülüdür.
GitHub Release varlıkları (X.Y.Z sürümü için), her biri aynı adla `.sig` ile:

- `sshpocket-X.Y.Z-darwin-universal` (USB için)
- `sshpocket-X.Y.Z-darwin-arm64`, `sshpocket-X.Y.Z-darwin-amd64`

İmza: SSHSIG, ad alanı `sshpocket-release`, anahtar `release_signers` dosyasındaki (kök klasör ya da `sistem/`) public key. Kabuk dönemindeki anahtar aynen kullanılır.

## USB / klasör düzeni
```
sshpocket            ikili (universal)         ← Release'ten indirilen dosya bu adla
start.command        çift tıkla başlatıcı      ← go/launcher/start.command
release_signers      güncelleme güven noktası  (ya da sistem/release_signers)
components.lock      paket kilidi
components/          kurulu paketler
servers/             sunucular ve anahtarlar (kullanıcı verisi)
VERSION              (isteğe bağlı; ikili kendi sürümünü içinde taşır)
```
Argümansız çalıştırınca yerel web arayüzü açılır. Komut satırı: `./sshpocket components list|sync|update|upgrade|check-new|prune|validate|report|run`, `doctor`, `version`, `update [--check]`, `eject`.

## Sürüm çıkarma (bakımcı; hiçbir adım otomatik değildir)
1. `VERSION`'ı artırın, CHANGELOG'u güncelleyin, commit + etiket (`RELEASING.md` ile aynı kurallar).
2. `SSHPOCKET_SIGNING_KEY=~/.ssh/sshpocket_release go/scripts/build-release.sh` → `go/dist/` altında derler, imzalar, imzayı `release_signers` ile kendisi doğrular. (`--dry-run` yalnızca ne yapacağını gösterir.)
3. GitHub'da `vX.Y.Z` Release'ini açın; `go/dist/` içindeki dosyaları ve `.sig`'lerini Assets'e yükleyin.
4. Eski sürümlü bir kopyada `./sshpocket update --check`, sonra `./sshpocket update` ile deneyin.

## Kendini güncelleme (`update`)
Son sürüm GitHub'ın `releases/latest` yönlendirmesinden okunur (tam olarak bu reponun etiket adresi olmalı). Önce `-darwin-universal`, yoksa `-darwin-<mimari>` indirilir. Yerleştirmeden ÖNCE: imza doğrulanır (anahtar yoksa güncelleme kapalı), ikilinin bildirdiği sürüm (`version`) etiketle aynı olmalı (eski imzalı sürümün yeniden sunulmasını engeller), geri sürüme düşülmez, iki onay istenir. Yerleştirme atomiktir (aynı klasörde geçici dosya + rename), eski ikili `.update-backup/sshpocket-<eski>` olarak saklanır. `servers/`, `components/` ve anahtarlara dokunulmaz. Başarıdan sonra YENİ ikiliyle `components sync` çalışır.

## Anahtar rotasyonu
Yeni anahtarı `release_signers` dosyasına ekleyin; yeni sürümü ESKİ anahtarla imzalayıp yayınlayın (kurulu kopyalar bunu kabul eder), kopyalara yeni `release_signers` bu güncellemeyle gitmez — dosyayı USB'de elle değiştirmeniz gerekir. (Kabuk dönemindeki arşiv güncellemesi dosyayı da taşıyordu; tek ikilide güven noktası bilerek ikiliden ayrıdır.)

## Henüz yapılmadı / denenmedi
Gerçek bir Release ile uçtan uca `update` (yalnızca sahte ağ + gerçek `ssh-keygen` imzalarıyla test edildi); `build-release.sh`'ın imzalama adımı (anahtarınızı ben kullanmam); Gatekeeper/notarization (ad-hoc imza yetmeyebilir: internetten indirilen ikilide ilk açılışta sağ tık > Aç gerekebilir).
