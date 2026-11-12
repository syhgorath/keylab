# Yol haritası / Roadmap

Sıra bağlayıcı değildir; fikir ve PR'lara açıktır.

## Planlanan

> **Arayüz/TUI ve teknoloji seçenekleri (tartışma):** bkz. [docs/ui-design.md](docs/ui-design.md).

> **Bileşen/paket modeli:** tasarım [docs/components-design.md](docs/components-design.md), paket standardı [docs/package-standard.md](docs/package-standard.md).
> Durum: `servers/` veri düzeni, ayarlar, ok tuşlu menü ve paket **altyapısı** (kilit, imza, kurulum, bildirim, venv, `--doctor`, `--components`) hazır.
> Gerçek paketle (linux-hardening 0.2.0) uçtan uca deneme yapıldı; komut satırı (`--components run`) ve sunucu menüsünde 🛡️ Paketler hazır.
> 3.6.0: menüden değişken düzenleme (erişim ağı önerisi ve kapsama), son sonuçlar/geçmiş, `rollback_status` durumu, ssh hata ipuçları ve systemd'li Docker LAB'ı hazır.
> Sırada: 🧰 Araçlar (`runner: script`, ikinci paketle), klasik port knocking **sunucu** paketi, rpi/Proxmox profilleri (linux-hardening tarafı), gerçek VM'de son doğrulama.
> Bkz. [docs/hardening-kullanim.md](docs/hardening-kullanim.md).

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
- Arayüz: pocketui'den planlanan özellikler tamam (Agatha ayrı repoda, ping/shutdown/reboot şimdilik yok). Kalanlar: toplu işlemin yazan eylemlere genişletilmesi (sunucu başına snapshot/sudo/ad onayı gerektiğinden ayrı tasarım), `html` etiketli şablon, `app.js`'in ES modüllerine bölünmesi. Gerçek ortamda denenmesi gerekenler: yazdırma/PDF, çoklu terminal (knock'lı sunucu), gerçek ansible işini arka plana alma, yedeği gerçek sunucularla geri yükleme.

### Dağıtım
- **Kilit kanalı:** paket sürümleri için sshpocket sürümü beklememek (imzalı, sayaçlı, süreli, onaylı); tasarım taslağı [docs/kilit-kanali.md](docs/kilit-kanali.md).
- Homebrew formülü / kurulum betiği.
- Demo GIF'i.

## Tamamlananlar
Bkz. [CHANGELOG.md](CHANGELOG.md).
