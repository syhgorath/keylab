# Tasarım: arayüz (UI), fzf ve teknoloji alternatifleri

> **Durum (2026-10-17): UYGULANDI** (3.1-3.6): başlık/ekmek kırıntısı/ipucu, ok tuşlu seçici, ⚙️ Ayarlar, durum göstergesi varsayılan açık, `q` = geri,
> menü açıklamaları, **sistemdeki fzf** arka ucu ve (3.6) sunucu menüsünde **🛡️ Paketler** (Hazırlık bloğu, Değişkenler, ağ editörü, Geçmiş; aynı seçici ve açıklama satırlarıyla;
> ok tuşlu ve numaralı mod Docker LAB'ında gerçek `Main.sh` ile gezildi). **Kalan:** fzf'i USB'ye gömme (aşağıdaki §5 'USB ikilisi' maddeleri hâlâ geçerli, indirme + sha256 sabitleme +
> sizin onayınızla). **Uygulamada farklar:** menü stili tek ayardır: `UI` = `auto` | `fzf` | `arrows` | `numbers` (ayrı `FZF`/`COLOR` ayarı yok);
> açıklamalar fzf `--preview` penceresinde, fzf yoksa seçicinin alt satırında; dar terminalde sarmayı kapatıp kırpar.

## 1. Sorun
- "Bazen neredeyim bilmiyorum": ekranlar arasında **yönelim** (hangi menü, hangi sunucu, durum) yok.
- Numaralı seçim (1, 2, 3) sunucu eklenip silindikçe **kayar**, ezberlenemez.
- Durum göstergesi (🟢/🔴) varsayılan olarak kapalı (`SSHPOCKET_STATUS=1`); `start.command` bu değişkeni vermediği için görünmüyor.
- Alt kısımda tuş ipucu yok ("0 geri mi, çık mı?").

## 2. İlkeler
| Alan | Kural |
|---|---|
| Gezinme | **Yön tuşları:** ↑ ↓ seç, Enter gir, ← geri. Numara/harf kısayolları ek olarak kalır. |
| **Yıkıcı veya geri dönüşü zor işler** (sil, yeniden adlandır, `apply`, `rollback_now`) | **Sunucu adını yazarak onayla** (bugünkü silme gibi). Hata yaptırmayan kalıp, yaygınlaştırılır. |
| Geri alınabilir sorular | `y/n` |
| Yönelim | Her ekranda **başlık + ekmek kırıntısı** (`Ana menü › ubuntutest`), alttaki **tuş ipucu satırı**, sunucu satırında `kullanıcı@ip` ve durum. |
| Terminal değilse (pipe, test, betik) | Eski **numaralı menü** (ve renksiz çıktı). Testler bozulmaz. |
| Renk | Yalnızca `[ -t 1 ]` ve `NO_COLOR` tanımsızsa. |

## 3. Taslak
```
sshpocket <sürüm>                                        USB: <ad>
────────────────────────────────────────────────────────────────
Ana menü

  🟢 ubuntutest    kullanici@10.0.0.5
  🔴 proxmox       admin@10.0.0.9:2222
  ────────────────────────────
  ➕ Yeni sunucu ekle
  ...
────────────────────────────────────────────────────────────────
↑↓ seç · Enter aç · ← geri · / ara · ? yardım
```
(IP'ler örnektir.)

## 4. Tuş okuma (saf bash) için notlar
- `read -rsn1`; ok tuşu `ESC [ A|B|C|D`. macOS bash **3.2**'de kesirli `read -t` yoktur, bu yüzden tek başına `ESC` bağlanmaz;
  geri için `←`, `0` veya `q`.
- İmleç gizlenir ve **çıkışta (trap) mutlaka geri açılır**, `stty` durumu geri yüklenir; yoksa terminal bozuk kalır.
- Seçim mantığı, tuş okuyucuyu sahte tuş dizisiyle değiştirerek test edilir (pty gerekmez).
- Emoji genişliği bazı yazı tiplerinde hizayı kaydırabilir; vurgu satırın tamamına uygulanır.

## 5. fzf (isteğe bağlı iyileştirme)
**Tercih sırası:** sistemde `fzf` varsa onu kullan → yoksa USB'deki ikili → o da yoksa saf bash ok tuşlu menü → numaralı menü.

| Konu | Karar / not |
|---|---|
| Dağıtım | USB'de **iki ikili**: `darwin_arm64` ve `darwin_amd64` (her biri ~3-4 MB). Mimari `uname -m` ile seçilir. |
| Güven | Sürüm ve **SHA-256**, imzalı sshpocket sürümüyle gelen `components.lock`'ta sabitlenir; çalıştırmadan önce doğrulanır. fzf'nin kendi sürüm imzası ayrıca doğrulanmalı (BİLİNMİYOR). |
| Lisans | MIT; lisans metni ikilinin yanında bulunur. |
| exFAT/FAT USB | Çalıştırma izni garanti değil: başarısızsa geçici klasöre kopyalayıp oradan çalıştır. |
| Gatekeeper | `curl` ile inen dosyaya karantina işareti konmaz; Apple Silicon'da imzasız ikili çalışmaz (ad-hoc imza yeter). **Doğrulanmadı.** |
| Tehdit modeli | Özet doğrulaması **kazara bozulmaya** karşı korur. USB'ye yazabilen kararlı bir saldırgan `start.sh`'i de değiştirebilir; gerçek koruma **USB şifrelemesi** ve imzalı güncellemelerdir. |

## 6. Test edilen / edilmeyen ortamlar
| Ortam | Durum |
|---|---|
| Intel (x86_64), macOS 15.7.9, bash 3.2.57, OpenSSH 9.9p2 | Denendi |
| Apple Silicon (arm64), macOS 26.6.2, bash 3.2.57 | **Otomatik testler geçiyor** (CI, her PR); elle/gerçek Keychain denemesi YAPILMADI |

**Apple Silicon / yeni macOS doğrulama listesi** (ilk fırsatta bir M-serisi Mac'te):
1. `start.sh` ve `start.command` açılıyor mu (Gatekeeper)?
2. `security` (Keychain) izin penceresi ve passphrase kaydı/doğrulaması.
3. `ssh-add` + `SSH_ASKPASS` akışı (OpenSSH sürümü farkı).
4. `ssh-copy-id`, `osascript` ile Terminal açma.
5. (fzf eklenirse) arm64 ikilisi çalışıyor mu, karantina, exFAT'tan çalıştırma.
6. Bats testleri ve ShellCheck aynı sonucu veriyor mu.

## 7. Teknoloji alternatifleri
Mevcut yığın: **bash + ShellCheck + Bats**. Karar için ölçütler: (a) Intel/ARM ve macOS sürümleri arasında taşınabilirlik,
(b) çalışma zamanı bağımlılığı (USB'yle yabancı Mac'e takıldığında), (c) TUI kalitesi, (d) geliştirme/token maliyeti, (e) test edilebilirlik.

| Seçenek | TUI kalitesi | USB/yabancı Mac | Maliyet | Not |
|---|---|---|---|---|
| **Bash + ok tuşu (+ isteğe bağlı fzf)** | İyi (liste/arama için yeterli) | ✅ bağımlılık yok | **Düşük** | Önerilen ilk adım |
| **Go + Bubble Tea** | Çok iyi (form, ilerleme, canlı panel) | ✅ tek statik ikili, tek makineden Intel+ARM derlenir | **Orta-yüksek** (yeniden yazım) | Hızlı geliştirme, olgun TUI kütüphaneleri, kolay çapraz derleme |
| Rust + ratatui | Çok iyi | ✅ tek ikili | Yüksek | Bellek güvenliği; öğrenme/derleme maliyeti daha yüksek |
| Python + Textual | Mükemmel | ❌ Python çalışma zamanı gerekir (macOS'unki eski/yok) | Orta | Paketleme (PyInstaller) büyük ve kırılgan |
| Node + Ink | İyi | ❌ Node gerekir | Orta | Önerilmez |
| Swift (menü çubuğu / SwiftUI) | GUI | ✅ yalnızca macOS, Keychain yerel | Yüksek | TUI hedefinden farklı; Linux yok |
| Tauri/Electron | GUI | ✅ ama ağır | Yüksek | Aşırı |
| Mevcut araçlar (`~/.ssh/config` üzerinde TUI'ler, örn. `sshs`, `lazyssh`) | İyi | ✅ | Çok düşük | **İncelenmedi**; bizim farkımız USB, Keychain yaşam döngüsü, imzalı güncelleme, hardening |

## 8. Strateji (öneri)
1. **Şimdi:** bash'te kal; arayüz cilası (başlık, ekmek kırıntısı, ipucu, durum varsayılan açık) ve ok tuşlu seçim. Maliyet düşük.
2. **Veri ve sözleşmeler dilden bağımsız tutulur** (`servers/*/server.env`, `components.lock`, `component.yml`, Docker e2e testleri).
   Böylece ileride Go/Rust ön yüzü aynı veriyi okuyup adım adım devralabilir ("kademeli geçiş"), baştan yazmak gerekmez.
3. **Geçiş tetikleyicileri** (herhangi biri gerçekleşirse Go düşünülür): form/ilerleme/canlı log gibi bash'te zorlanan TUI ihtiyaçları;
   Apple Silicon/yeni macOS'ta bash/fzf taşınabilirlik sorunları; katkıcı talebi; test bakım maliyeti.
4. **Ucuz deney (spike):** karar vermeden önce, tek oturumluk küçük bir Go prototipi (menü + durum + bağlan) yapılıp
   "gerçekten daha mı iyi hissettiriyor?" sorusu cevaplanabilir. Çekirdeğe dokunmaz.

## 9. Kararlar (2026-10-11, kullanıcı)
| # | Konu | Karar |
|---|---|---|
| 1 | Durum göstergesi (🟢/🔴) | **Varsayılan açık.** Kapatmak için ayar veya `SSHPOCKET_STATUS=0`. |
| 2 | Ayarlar paneli | **Evet**, küçük bir `⚙️ Ayarlar` menüsü (§10). |
| 3 | Geri tuşu | **`q`** geri; `←` ek olarak çalışır. |
| 4 | fzf | **USB'ye gömülür** (deneysel, doğrulanmadı notuyla) ve her durumda yedek vardır: fzf → ok tuşlu menü → numaralı menü. Sistemdeki fzf önceliklidir. |
| 5 | Go | **Kalsın.** İhtiyaç doğarsa (§8 tetikleyiciler) değerlendirilir. |

Hâlâ açık: yazarak arama ilk sürümde mi gelsin (fzf varsa zaten var), ok tuşlu saf-bash menünün tam kapsamı.

## 10. Ayarlar paneli
Menüde `⚙️ Ayarlar`. Satırlar: ayar adı, mevcut değer, Enter ile değiştir (açık/kapalı döner, aralıklar için seçenek listesi), `q` geri.

| Ayar | Anahtar | Değerler | Varsayılan |
|---|---|---|---|
| Durum göstergesi | `STATUS` | `on` \| `off` | `on` |
| Durum kontrol süresi | `STATUS_TIMEOUT` | `1`-`5` (sn) | `1` |
| Renk | `COLOR` | `auto` \| `on` \| `off` | `auto` |
| Menü stili | `UI` | `auto` \| `arrows` \| `numbers` | `auto` |
| fzf kullan | `FZF` | `auto` \| `off` | `auto` |

- Dosya: kökte **`sshpocket.conf`** (gitignore'lu; USB'yle taşınır). Aşama 1'den sonra `servers/` ile aynı veri katmanına taşınabilir.
- Biçim: `ANAHTAR=değer`; mevcut güvenli ayrıştırıcıyla okunur, **yalnızca bilinen anahtarlar** kabul edilir, değerler doğrulanır (kabuğa verilmez).
- **Öncelik:** ortam değişkeni (`SSHPOCKET_STATUS` vb., betik/test için) > ayar dosyası > varsayılan.
- Kasıtlı olarak az sayıda ayar: her ayar bir test ve bir belge maliyetidir. Yeni ayar için gerekçe istenir.


## 5b. Ortak arayüz parçaları (UI.sh): seçici, bölgeli ekran, liste editörü
Bütün menüler aynı üç parçayı kullanır; paketten ve ekrandan bağımsızdır. Parametreler bash'in **dinamik kapsamıyla** (çağıranın `local` değişkenleri) verilir. Hepsi isteğe bağlıdır; verilmezse eski davranış sürer. Kullanan yeni bir ekran (örn. `utility` paketi, Lynis bulgu listesi) bu tabloyu okuyup `local` ile tanımlar.

### Seçici: `ui_pick <geri-etiketi> <öğe>...` → seçilen numara (geri/hata = 0)
Etkileşimli modda ok tuşlu ya da fzf (`UI=auto|fzf|arrows`), değilse numaralı menü. fzf'te sağ bölme, etiketli çerçeveler ve kısayollar `UI_PICK_*` ile gelir:

| Değişken | Anlamı |
|---|---|
| `UI_PICK_DESCS` (dizi) | Her öğe için açıklama; fzf'te sağ bölmede (çok satırlı olabilir, kart), ok tuşlu seçicide alt satırda tek satır |
| `UI_PICK_BACK_DESC` | Geri/çıkış satırının açıklaması |
| `UI_PICK_WIDE=1` | Geniş düzen: sağda önizleme, etiketli çerçeveler (fzf ≥ 0.35 kenarlık etiketi, ≥ 0.58 liste/giriş çerçeveleri; eski fzf'te yalnız sağ önizleme) |
| `UI_PICK_HEIGHT` | Geniş düzende fzf yüksekliği: satır sayısı ya da yüzde (varsayılan `85%`); kısa listelerde (örn. ana menü) boş alanı keser |
| `UI_PICK_TITLE` | Yol çubuğu (kenarlık etiketi) |
| `UI_PICK_HEADER` | Başlık: konum özeti ya da ileti (çok satırlı olabilir) |
| `UI_PICK_START` | İmlecin başlangıç öğesi (varsayılan 1) |
| `UI_PICK_EXPECT`, `UI_PICK_EXPECT_HINT` | fzf kısayolu (`--expect`, örn. `ctrl-o`) ve başlıktaki ipucu; basılırsa `ui_pick` `key:<ad>` döner (numaralı menüde elle yazılan değer kabul edilmez) |

fzf sürümü `ui_fzf_version` ile okunur. Çok satırlı açıklama dosyaya `\037` ile tek satır olarak yazılır, önizlemede geri çevrilir.

### Bölgeli ekran: `ui_region_begin <başlık> <gövde> <alt-etiket>` / `ui_region_end <etiket>` / `ui_region_reset`
Üst bölüm (başlık + gövde metni) sabit çizilir, alt bölüm terminalin **kaydırma bölgesi** (DECSTBM) olur: sorular, `BECOME password:` istemi ve çıktı alt bölgede akar, ekran değişmez. Terminal < 24 satır ya da < 60 sütunsa `begin` 1 döner (çağıran eski akışa düşer). `Ctrl-C` bölgeyi sıfırlar. Test için `SSHPOCKET_ROWS`/`SSHPOCKET_COLS`. Not: fzf'yi canlı tutup altta çalıştırma denendi, `execute` sonrası fzf üst bölgeyi yeniden çizmediği için olmadı.

### Liste editörü: `ui_edit_list`
Öğeler liste olarak görünür (Enter = sil/değiştir), ➕ Ekle, ek satırlar, 🧹 Temizle, 💾 Kaydet, Vazgeç. Dosyaya hiçbir şey yazmaz; çağıran sonucu alır.

| Değişken | Anlamı |
|---|---|
| `UI_LIST_ITEMS` (dizi) | Girdi: başlangıç öğeleri; çıktı: son hali |
| `UI_LIST_TITLE`, `UI_LIST_NAME`, `UI_LIST_META`, `UI_LIST_DESC` | Yol çubuğu, liste adı, üst bilgi satırı, açıklama |
| `UI_LIST_PROMPT`, `UI_LIST_ADD_HINT` | Ekleme istemi ve açıklama eki |
| `UI_LIST_REQUIRED=yes` | Boş liste kaydedilemez |
| `UI_LIST_VALIDATE` | Zorunlu işlev: `<öğe>`; geçerliyse 0, değilse nedeni yazıp 1 |
| `UI_LIST_ADD` | İsteğe bağlı işlev: öğeyi kendi kuralıyla ekler (örn. daha geniş CIDR dar olanların yerine geçer) |
| `UI_LIST_EXTRAS` + `UI_LIST_EXTRA_RUN` | Ek satırlar: `UI_LIST_X_LABELS/_ACTS/_DESCS` dizilerini doldurur; seçilince `<eylem>` ile çalışır, iletiyi `UI_LIST_MSG`'ye yazar |
| `UI_LIST_SAVE_CHECK` | Kaydetmeden önce denetim: 1 dönerse düzenlemeye döner |

Çıkış kodu: `0` Kaydet · `1` Vazgeç · `2` girdi bitti (EOF); `UI_LIST_DIRTY=1` kaydedilmemiş değişiklik vardı. Örnek kullanım: `ComponentsMenu.sh` `comp_menu_edit_list` (list[string], list[int], list[cidr]; CIDR'e özgü öneri/sorgu geri çağrımları orada).
