# Güncelleme

Ana sayfa: [README](../README.md). İlk indirmeyi doğrulama adımları README'dedir.

Otomatik değildir; siz başlatırsınız:

```bash
./sshpocket update --check   # yalnızca bakar (yeni sürüm varsa çıkış kodu 10)
./sshpocket update         # indir → imzayı doğrula → notları göster → onaylarsanız kur
```

ya da ana menüden **⬆️ Güncellemeleri kontrol et / güncelle** (fzf düzeninde **📦 Paketler ve güncelleme**'nin altındaki sshpocket satırı).

- Terminalde (en az 24 satır × 60 sütun) güncelleme paneli açılır: imza ve arşiv denetiminden sonra üstte sürüm, imza durumu, paket (`components.lock`) farkları ve CHANGELOG'daki **Dikkat** notları; altta sürüm notları, onay ve kurulum. Paket sürümü değişince kurulum sonunda "ne değişti, hangi sunucum etkileniyor" raporu yazılır (`./sshpocket components report [paket [eski-sürüm]]` ile elle de alınır; yalnızca okur).
- Paket yalnızca bu reponun GitHub Release'lerinden, HTTPS ile inen ve **bakımcının imza anahtarıyla imzalı** olan
  dosyadır (`ssh-keygen -Y verify`). Doğrulanamazsa hiçbir şey açılmaz/kurulmaz. İmza anahtarı tanımlı değilse
  güncelleme **kapalıdır**.
- Sunucularınız (`servers/`), ayarlarınız (`sshpocket.conf`) ve key'leriniz korunur. Kurmadan önce kodun yedeği (`.update-backup/`, gizli dosya
  içermez) alınır; hata olursa geri yüklenir.
- Geri sürüme düşmez; `..`, mutlak yol veya symlink içeren paketleri reddeder.
- USB yazma korumalıysa güncelleme yapılamaz (bilerek).
- Not: güncelleyici `v2.2.0` ile gelir; daha eski kopyalar bir kez elle güncellenmelidir.

## KeyLab.app'te uygulama içi güncelleme (4.0.11+)

**Ayarlar > Güncelleme**: "Güncellemeyi denetle" GitHub'daki son sürümü ve sürüm notlarını gösterir (KeyLab kendiliğinden internete çıkmaz); "Güncelle ve yeniden başlat" paketi indirir, release anahtarıyla imzasını doğrular, `.app`'i bütünüyle değiştirir (eskisi `.keylab-update-backup/` altına yedeklenir) ve KeyLab'ı yeniden açar. Yalnız `KeyLab.app` olarak çalışıp uygulamanın klasörü yazılabilirse düğme çıkar; komut satırı ikilisinde `keylab update` kullanın. macOS uygulamayı karantina nedeniyle geçici bir yoldan çalıştırıyorsa (App Translocation) KeyLab.app'i Uygulamalar'a ya da USB'ye taşıyıp oradan açın.
