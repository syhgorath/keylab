# Güncelleme

Ana sayfa: [README](../README.md). İlk indirmeyi doğrulama adımları README'dedir.

Otomatik değildir; siz başlatırsınız:

```bash
./start.sh --check-update   # yalnızca bakar (yeni sürüm varsa çıkış kodu 10)
./start.sh --update         # indir → imzayı doğrula → notları göster → onaylarsanız kur
```

ya da ana menüden **⬆️ Güncellemeleri kontrol et / güncelle** (fzf düzeninde **📦 Paketler ve güncelleme**'nin altındaki sshpocket satırı).

- Terminalde (en az 24 satır × 60 sütun) güncelleme paneli açılır: imza ve arşiv denetiminden sonra üstte sürüm, imza durumu, paket (`components.lock`) farkları ve CHANGELOG'daki **Dikkat** notları; altta sürüm notları, onay ve kurulum. Paket sürümü değişince kurulum sonunda "ne değişti, hangi sunucum etkileniyor" raporu yazılır (`./start.sh --components report [paket [eski-sürüm]]` ile elle de alınır; yalnızca okur).
- Paket yalnızca bu reponun GitHub Release'lerinden, HTTPS ile inen ve **bakımcının imza anahtarıyla imzalı** olan
  dosyadır (`ssh-keygen -Y verify`). Doğrulanamazsa hiçbir şey açılmaz/kurulmaz. İmza anahtarı tanımlı değilse
  güncelleme **kapalıdır**.
- Sunucularınız (`servers/`), ayarlarınız (`sshpocket.conf`) ve key'leriniz korunur. Kurmadan önce kodun yedeği (`.update-backup/`, gizli dosya
  içermez) alınır; hata olursa geri yüklenir.
- Geri sürüme düşmez; `..`, mutlak yol veya symlink içeren paketleri reddeder.
- USB yazma korumalıysa güncelleme yapılamaz (bilerek).
- Not: güncelleyici `v2.2.0` ile gelir; daha eski kopyalar bir kez elle güncellenmelidir.
