# Taslak: `utility` paketi (saat dilimi, locale, NTP)

> **Durum: TASLAK, kararlar verildi (2026-11-05, kullanıcı), kod yok.** Hazırlayan: sshpocket oturumu; linux-hardening tarafının görüşü koordinatör oturumu üzerinden alındı.
> Kararlar §10'da; yazım işi ayrı bir oturuma verilir (bkz. `utility-yeni-oturum-brief.md`). Standart: [package-standard.md](package-standard.md) (§12 kontrol listesi bölüm 9'da eşlenmiştir).

## 1. Amaç
Sunucunun **temel sistem ayarlarını** (saat dilimi, locale, NTP) sshpocket menüsünden, önizleyerek ve onayla yönetmek. Düşük risk: bu ayarlar **erişimi kesmez** (ssh/ufw/PAM dokunulmaz), bu yüzden `linux-hardening`'deki geri alma güvenlik ağı (zamanlayıcılı rollback) **taşınmaz**; yerine `--check` önizleme, değiştirmeden önce yedek ve idempotent apply vardır.

## 2. Sahiplik (linux-hardening ile ÇAKIŞMA YOK, kural yazılı)
| Alan | Sahibi | Not |
|---|---|---|
| `/etc/chrony/conf.d/*`, `/etc/chrony/sources.d/*` (NTP kaynakları, politika) | **utility** | `chrony.conf`'a dokunulmaz; yalnız drop-in. Chrony'yi yapılandırma değişince yeniden başlatan **tek taraf** utility |
| `/etc/localtime`, `timedatectl` saat dilimi | **utility** | |
| Locale (`/etc/default/locale`, `locale-gen`) | **utility** | |
| Zaman **raporu** (chrony durumu, `NTPSynchronized`, sunucu↔kontrol makinesi farkı) | linux-hardening (`time` rolü) | Dosya yazmaz, paket kurmaz; `hardening_time_makestep` açıksa yalnız `chronyc makestep`/restart |
| auditd, ufw, ssh | linux-hardening | |

Önek ayrımı: değişkenler `utility_*`, menü grubu ayrı; "kurulu mu" çalışma zamanı tespitiyle değil değişken/dosya sahipliğiyle ayrılır. İki paket de kendi durum çıktısında paket adını yazar (`utility.durum …`).

## 3. Kapsam (v0.1)
- **Saat dilimi:** `utility_timezone` (örn. `Europe/Istanbul`); `timedatectl set-timezone`.
- **Locale:** `utility_locale` = yalnız **`LANG`** (örn. `en_US.UTF-8`); gerekirse `locale-gen`, `update-locale LANG=…`. `LC_*` ayrı ayrı yönetilmez (v0.1).
- **NTP:** `chrony` (Ubuntu 26.04: chrony 4.8 hazır, timesyncd kurulu değil; **Debian 13'te varsayılan `systemd-timesyncd` olabilir: chrony kurulumu, timesyncd'nin devre dışı bırakılması ve `conf.d`/`sources.d` bildirimlerinin varlığı DOĞRULANMADI, yeni oturum gerçek bir Debian 13 üzerinde doğrulamalı**; `confdir /etc/chrony/conf.d`, `sourcedir /etc/chrony/sources.d` zaten tanımlı). Kaynak listesi `utility_ntp_servers` → `sources.d/utility.sources` (`server`/`pool` satırları). Boş liste = dağıtım varsayılanı, dosya yazılmaz.
- **Kapsam DIŞI (v0.1):** hostname, apt kaynakları, kullanıcı/paket yönetimi, `makestep` (hardening'de), NTS (bkz. §7), Windows/macOS hedefleri.

## 4. Manifest taslağı (`component.json`)
| Alan | Değer |
|---|---|
| `id` / `runner` | `utility` / `ansible` |
| `inventory_group` | `utility` |
| `profiles` | `ubuntu-26.04` supported, `debian-13` supported (Debian 13 doğrulaması gerçek VM'de yapılana kadar `planned` kalır, kabul ölçütü); Proxmox VE ve Raspberry Pi **sonra** |
| Eylemler | `durum` (salt okunur), `preview` (`--check`), `apply` (değiştirir), `geri_yukle` (yedekten) |
| `durum` çıktısı | `keyvalue-line`: `utility.durum timezone=… locale=… ntp_servis=… ntp_senkron=yes\|no offset_ms=… kaynak_sayisi=…` (değerler `^[A-Za-z0-9_.:+-]{0,64}$`) |
| `apply` | `effects: ["changes-config"]` (+ `installs-package` yalnız chrony yoksa), `requires_sudo: true`, `confirm: yes-no` (§6, karar), `requires: ["preview"]` |
| Değişkenler | `utility_timezone` (string), `utility_locale` (string), `utility_ntp_servers` (`list[string]`), `utility_backup` (bool, varsayılan true) |
| `required: true` | **Kullanılmaz** (tüm eylemleri kapatır). Gerçekten zorunlu değişken doğarsa eylem düzeyinde `requires_vars` |
| `platform_choices` | v0.1'de yok (tek profil); Proxmox/Pi'de platforma özel varsayılan gerekirse sonra |

## 5. Çalıştırma ve güvenlik
- **Önizleme:** `--check`; göstermedikleri `warnings`'te yazılı: `timedatectl`/`locale-gen` `--check`'te çalışmaz (önizleme yalnızca dosya farkını gösterir), chrony yeniden başlatması önizlenmez.
- **Yedek:** değiştirmeden önce `/var/backups/utility/<tarih>/` (dizin 0700, dosyalar 0600) + manifest (hardening `kaldir*` yedek deseni). `geri_yukle` bu yedeği geri yazar; **kapsamı dürüstçe:** yalnız dosyaları (localtime, locale dosyaları, chrony drop-in'leri) geri alır; çalışan servis durumunu ve geçmiş log zaman damgalarını geri almaz.
- **İdempotans:** ikinci `apply` `changed=0`. Test ölçütü.
- **Erişim:** ssh/ufw/PAM'a dokunulmaz; `hardening-rollback`'e ihtiyaç yok. Yanlış NTP kaynağının en kötü sonucu saat senkronunun kaybıdır (ssh sertifikaları/TOTP/apt "Not live until" etkilenebilir), erişim kaybı değil.
- **Gizli veri:** yok (parola/anahtar değişkeni olmaz); sonuç klasörü gerekmez.

## 6. Onay seviyesi (KARAR: `yes-no`)
`apply` için `yes-no`; sunucu adı yazdırma yok (`linux-hardening`'deki sunucu adı onayı erişim kesme riski içindir; burada kapsam dar, risk düşük ve yedekli). Not: saat dilimi değişimi auditd'li sunucuda `time-change` kaydı üretir (beklenen); `warnings`'te yazılır.

## 7. linux-hardening ile etkileşim (koordinatör üzerinden doğrulandı, 2026-11-05)
1. auditd `/etc/localtime`, `adjtimex`, `settimeofday`, `clock_settime` izler → utility saat dilimini değiştirirse auditd'li sunucuda `time-change` kaydı çıkar (**beklenen**, uyarıda yazılı).
2. `hardening_ufw_outgoing_deny` (varsayılan kapalı) açıkken yalnız **123/udp** çıkışı izinli. NTS-KE için `hardening_ufw_outgoing_allow`'a ekleme gerekir (**NTS-KE portu 4460/tcp doğrulanmadı**) → v0.1'de NTS yok, ancak `utility_ntp_servers` kaynakları 123/udp'ye ihtiyaç duyar; `outgoing_deny` açıksa uyarı.
3. Proxmox'ta hardening `time` yine yalnız rapor; utility Proxmox ve Raspberry Pi'yi v0.1'de **desteklemez** (sonra). Pi'de varsayılan `systemd-timesyncd` olabilir; chrony kurma kararı o zaman verilir.
4. NTP kaynak/politikasını hardening bilerek yazmaz (CIS "yetkili NTP" ⏸️); aday `hardening_chrony_servers` linux-hardening'de yapılmaz, utility'ye bırakılır (kullanıcı onayı gerekir; **şimdilik değişiklik yok**).

## 8. Depo, yayın, test
- **Ayrı repo + kopya** (ortak Ansible rolü imza/kilit zincirini karmaşıklaştırır): `sshpocket-utility`, `sshpocket-paket-sablonu`'ndan türer. Kendi imzalı `tar.gz`'si ve `components.lock` girdisiyle; sshpocket'e tek satır kilit PR'ı.
- **Kopyalanacaklar** (linux-hardening'den, repo-bağımsız): `scripts/release.sh`, `release-publish.sh`, `make-component-json.py`, `test/lib.sh`, `check-consistency.py` (ön ek/rol kısımları uyarlanır). Gerekmeyenler: rollback, `SSHPOCKET_PROGRESS` callback (2-3 alan; ilk sürümde atlanır), `bulgular.tsv` (Lynis'e özgü).
- **Test:** bats + Docker LAB (`test/e2e/lab`, systemd'li Ubuntu 26.04) ile timezone/locale/chrony drop-in; ardından gerçek VM (ubuntutest: `durum` → `preview` → `apply` → ikinci `preview` changed=0 → `geri_yukle`; hardening uygulanmış VM'de birlikte). Birlikte çalışma **henüz denenmedi**.

## 9. §12 kontrol listesi eşlemesi
| Madde | Planı |
|---|---|
| `component.json`, `effects`/`requires_sudo` eksiksiz | §4 |
| Mutating eylemin `preview` ön koşulu + göstermedikleri `warnings`'te | §4–5 |
| Değişkenlerde parola/anahtar yok | §5 (yok) |
| `requirements.txt` hash'li, `--only-binary :all:` | `ansible-core` sabit, şablondaki gibi |
| Sürüm üçlüsü, arşiv kökü `<id>-<sürüm>/` | `release.sh` kopyası |
| `ssh-keygen -Y sign -n sshpocket-component` | `release.sh` kopyası imzalar; imzalayıcı açık anahtar kilit PR'ında (`components.lock` `signers`) açıkça yazılır (hangi anahtar kullanılacağı §10'a eklendi) |
| Geri alma belgelenmiş, kapsamı dürüst | §5 `geri_yukle` |
| Kilit PR'ı tek satır | evet (kod değişikliği gerekmez, hedef) |

## 10. Kararlar (2026-11-05, kullanıcı)
| # | Soru | Karar |
|---|---|---|
| 1 | Kim yazar? | **Yeni, ayrı oturum** (sshpocket ve linux-hardening oturumları meşgul kalır); şablon repodan türetir |
| 2 | v0.1 kapsamı | Saat dilimi + locale + NTP **üçü birden**; hostname ve NTS dışarıda |
| 3 | Locale | Yalnız `LANG`; `LC_*` yok |
| 4 | Apply onayı | `yes-no`; sunucu adı yazdırma yok (düşük risk, yedekli) |
| 5 | Profiller | v0.1: **Ubuntu ve Debian**; Proxmox ve Raspberry Pi sonra (Pi'de timesyncd notu yukarıda) |
| 6 | İmzalayıcı anahtar | linux-hardening ile **aynı** anahtar (kilitte aynı `sshpocket-component` imzalayıcı satırı); anahtar ve parola kullanıcıdadır, ajan dokunmaz |

**Hâlâ doğrulanmamış:** NTS-KE portu (v0.1 dışı); Debian 13'te chrony/timesyncd durumu; utility + hardening'in gerçek sunucuda birlikte çalışması.
