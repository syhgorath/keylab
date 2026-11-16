# Yol haritası / Roadmap

Sıra bağlayıcı değildir; fikir ve PR'lara açıktır.

## Planlanan

> **Arayüz/TUI ve teknoloji seçenekleri (tartışma):** bkz. [docs/ui-design.md](docs/ui-design.md).

> **Bileşen/paket modeli:** tasarım [docs/components-design.md](docs/components-design.md), paket standardı [docs/package-standard.md](docs/package-standard.md).
> **Güncel durum (2026-11-15, KeyLab 4.0.16):** paket altyapısı (kilit, imza, imzalı indeks, kurulum, venv) ve iki paket hazır: `linux-hardening` ve `utility` (eski `kurulum` ile birleşti: saat dilimi/dil/NTP + Docker/nginx/MariaDB/Redis kurulumu). Arayüz gömülü pencerede, USB/misafir mod, şifreli kasa, uygulama içi güncelleme ve Python ön denetimi var. Eski kabuk dönemi notları (ok tuşlu menü, `--doctor`, `--components`) tarihçedir.
> **Sırada:** utility'de kart başına gerçek Kur/Yükselt/Kaldır düğmeleri (paket manifestine `items`), gerçek VM'de NTP eylemlerinin ve birleşik paketin doğrulanması, Cmd+Q'yu yakalama, açılışta isteğe bağlı güncelleme denetimi ve kasa boşta süresi ayarı, Apple Developer ID/noter (ücretli), Linux sonra Windows katmanı. Klasik port knocking **sunucu** paketi ve rpi/Proxmox profilleri (linux-hardening tarafı) planlı.
> Bkz. [docs/hardening-kullanim.md](docs/hardening-kullanim.md), [docs/test-edilen-ortamlar.md](docs/test-edilen-ortamlar.md).

### Taşınabilirlik (Linux)
Şu an yalnızca macOS: `security` (Keychain), `osascript` (Terminal), `diskutil` (eject) kullanılıyor.
- Gizli bilgi deposu soyutlaması: macOS Keychain / `secret-tool` (libsecret) / `pass`.
- Terminal açma soyutlaması: `osascript` / `gnome-terminal` / `x-terminal-emulator` / aynı pencere. (Tarayıcı içinde gömülü terminal macOS ve Linux'ta var, Windows'ta yok.)
- Eject soyutlaması: `diskutil` / `udisksctl`.
- Linux/Windows çalışma zamanı: Go testleri CI'da Ubuntu ve macOS'ta koşuyor, Windows/Linux için derleme doğrulanıyor; kalan: Keychain, knock (`sh` bağımlılığı) ve Windows'ta gömülü terminal (ConPTY).

### Güvenlik
- Hardware key desteği (YubiKey, `ed25519-sk`).
- Log'larda IP/kullanıcı yazmayı kapatma seçeneği.
- USB şifreleme rehberi.

### Kullanışlılık
- Menüde arama / kısayol tuşları.
- ProxyJump/bastion, SSH tüneli, `scp`/`sftp` kısayolları.
- Arayüz çevirisi: Türkçe/İngilizce var (üst çubuktaki TR/EN düğmesi; ifade tablosu `go/web/js/core/i18n-phrases-en.js`); kalanlar: sunucudan gelen iletiler, tarih biçimi ve yeni dil eklemek için `t()` anahtarlarına geçiş.
- `~/.ssh/config`'te `Include` desteği.
- Arayüz: pocketui'den planlanan özellikler tamam (Agatha ayrı repoda, ping/shutdown/reboot şimdilik yok). Kalanlar: toplu işlemin yazan eylemlere genişletilmesi (sunucu başına snapshot/sudo/ad onayı gerektiğinden ayrı tasarım). (`app.js`'in ES modüllerine bölünmesi tamamlandı: `go/web/js/`.) Gerçek ortamda denenmesi gerekenler: yazdırma/PDF, çoklu terminal (knock'lı sunucu), gerçek ansible işini arka plana alma, yedeği gerçek sunucularla geri yükleme.

### Dağıtım
- ~~**Kilit kanalı:**~~ **TAMAMLANDI:** paket sürümleri için KeyLab sürümü beklenmez; imzalı paket indeksi (`syhgorath/sshpocket-index`) kullanılır ([docs/package-index.md](docs/package-index.md), tasarım tarihçesi [docs/kilit-kanali.md](docs/kilit-kanali.md)). Bir paketin repo adresi değişirse kilit ve indeks aynı repoyu göstermelidir.
- Homebrew formülü / kurulum betiği.
- Demo GIF'i.

## Tamamlananlar
Bkz. [CHANGELOG.md](CHANGELOG.md).
