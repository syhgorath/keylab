# Güncelleme

Ana sayfa: [README](../README.md). İlk indirmeyi doğrulama adımları README'dedir.

Otomatik değildir; siz başlatırsınız:

```bash
./sshpocket update --check   # yalnızca bakar (yeni sürüm varsa çıkış kodu 10)
./sshpocket update         # indir → imzayı doğrula → notları göster → onaylarsanız kur
```

Bu komutlar ikili (`./sshpocket`) içindir. `KeyLab.app` kullanıyorsanız aşağıdaki **uygulama içi güncelleme** bölümüne bakın (eski menü/fzf düzeni Go sürümünde yoktur).

- Komut satırı çıktısında imza ve arşiv denetiminden sonra sürüm, imza durumu, sürüm notları ve onay sorusu gelir. Paket sürümü değişince "ne değişti, hangi sunucum etkileniyor" raporu `./sshpocket components report [paket [eski-sürüm]]` ile alınır (yalnızca okur).
- Paket yalnızca bu reponun GitHub Release'lerinden, HTTPS ile inen ve **bakımcının imza anahtarıyla imzalı** olan
  dosyadır (`ssh-keygen -Y verify`). Doğrulanamazsa hiçbir şey açılmaz/kurulmaz. İmza anahtarı tanımlı değilse
  güncelleme **kapalıdır**.
- Sunucularınız (`servers/`), ayarlarınız (`sshpocket.conf`) ve key'leriniz korunur. Kurmadan önce kodun yedeği (`.update-backup/`, gizli dosya
  içermez) alınır; hata olursa geri yüklenir.
- Geri sürüme düşmez; `..`, mutlak yol veya symlink içeren paketleri reddeder.
- USB yazma korumalıysa güncelleme yapılamaz (bilerek).
- Güncelleme **kurulu sürümün koduyla** çalışır: bir güncelleme düzeltmesi, o sürümü kuran kullanıcılarda etkilidir; eski sürümler bir kez elle geçer (ör. `KeyLab.app` için USB'den güncelleme 4.0.13'ten itibaren çalışır).

## KeyLab.app'te uygulama içi güncelleme (4.0.11+)

**Ayarlar > Güncelleme**: "Güncellemeyi denetle" GitHub'daki son sürümü ve sürüm notlarını gösterir (KeyLab kendiliğinden internete çıkmaz); "Güncelle ve yeniden başlat" paketi indirir, release anahtarıyla imzasını doğrular, `.app`'i bütünüyle değiştirir (eskisi `.keylab-update-backup/` altına yedeklenir) ve KeyLab'ı yeniden açar. Yalnız `KeyLab.app` olarak çalışıp uygulamanın klasörü yazılabilirse düğme çıkar; komut satırı ikilisinde `keylab update` kullanın. macOS uygulamayı karantina nedeniyle geçici bir yoldan çalıştırıyorsa (App Translocation) KeyLab.app'i Uygulamalar'a ya da USB'ye taşıyıp oradan açın.

**Paketler ayrı güncellenir:** paket sürümleri KeyLab'ın güncellemesinden bağımsızdır; imzalı indeksten (`sshpocket-index`) gelir ve Paketler sayfasından ya da `components update | upgrade` ile uygulanır. Paketin deposu değişirse (kilit bir paketi bir repoya sabitler) indeks ve gömülü `components.lock` aynı repoyu göstermelidir; yoksa karar `other-repo` olur ve kurulmaz.

**USB'den güncelleme:** exFAT/FAT diskte macOS açılan her öğenin yanına `._*` yan dosyaları yazar; 4.0.13'ten itibaren bunlar paketin parçası sayılmaz. USB yazma korumalıysa güncelleme yapılamaz.
