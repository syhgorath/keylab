# İş tarifi: `utility` paketini yaz (v0.1)

> Bu belge kendi başına yeter. Yeni oturum için yazıldı (2026-11-05). Karar sahibi: kullanıcı (Ziya, Türkçe çalışır; kısa, tablolu yanıt; komutlar ayrı kod bloklarında). Köprü: **sshpocket koordinatör** oturumu (sshpocket ve linux-hardening oturumlarıyla seni o bağlar).

## 1. Görev
Yeni sshpocket paketi **`utility`**: sunucuda **saat dilimi, locale (`LANG`), NTP (chrony)** ayarını önizleyerek ve onayla yönetir. Tasarım kararlı: önce `~/Developer/sshpocket/docs/utility-paketi-taslak.md` (dal `docs/utility-taslak`; henüz push edilmediyse dosyayı doğrudan oku) ve `docs/package-standard.md` (özellikle §4–§8, §12) oku. Bu tarif yalnızca iş sırasını ve sınırları yazar.

## 2. Konum ve kaynaklar
| Ne | Nerede | Kural |
|---|---|---|
| Yeni repo (yerel) | `~/Developer/utility` | `git init`; id `utility`; GitHub deposu `syhgorath/utility` (**depoyu kullanıcı açar**; sen `gh repo create` çalıştırma) |
| Şablon | `~/Developer/sshpocket-paket-sablonu` | Okunur, değiştirme. Dal `sync-validator-3.6.21` (push edilmediyse bundan, değilse `main`) |
| Doğrulayıcı | `tools/validate/` (şablondan gelir) | Güncelleme: `tools/sync-validator.sh <sshpocket-main kopyası>` |
| linux-hardening | `~/Developer/linux-hardening` | **Yalnız OKU**. Kopyalayacakların: `scripts/release.sh`, `release-publish.sh`, `make-component-json.py`, `test/lib.sh`, `check-consistency.py` (hardening_ öneki ve rollere bağlı kısımları uyarla). Rollback, `SSHPOCKET_PROGRESS` callback, `bulgular.tsv` **alınmaz** |
| sshpocket | `~/Developer/sshpocket` | Yalnız OKU; ona kod yazma. Paket sshpocket'e **tek satır kilit** PR'ıyla girer (§7) |

## 3. Şablondan türetme adımları
1. `cp -R` şablonu `~/Developer/utility`'ye (`.git` hariç), `git init`, ilk commit.
2. `component.json`: `id: utility`, `name`, `description`, `inventory_group: utility`, `profiles` (§4), `vars`, `actions` (`durum`, `preview`, `apply`, `geri_yukle`); `playbooks/*.yml`'da `hosts: utility`.
3. `VERSION` `0.1.0`, `CHANGELOG.md`, `README.md` (şablon metnini paketine uyarla), `requirements.txt` hash'li (şablondakini kullan).
4. Rolleri yaz: `timezone`, `locale`, `ntp` (chrony); her biri `--check` ile çalışır, idempotent, yedeklidir.
5. Yerel doğrulama: `python3 -I tools/validate/component_validate.py . --strict --unsigned` → hata ve uyarı **yok**.

## 4. v0.1 kapsamı (kesin)
- **Saat dilimi** `utility_timezone` (örn. `Europe/Istanbul`), **locale** `utility_locale` = yalnız `LANG` (`LC_*` yok), **NTP** `utility_ntp_servers` (`list[string]`; boş = dağıtım varsayılanı, dosya yazılmaz) ve `utility_backup` (bool, varsayılan true).
- Eylemler: `durum` (salt okunur, `SSHPOCKET_RESULT utility.durum timezone=… locale=… ntp_servis=… ntp_senkron=… offset_ms=… kaynak_sayisi=…`, değerler `^[A-Za-z0-9_.:+-]{0,64}$`), `preview` (`--check`), `apply` (`effects: ["changes-config"]`, kurulum gerekirse `installs-package`; `requires_sudo: true`; **`confirm: yes-no`**; `requires: ["preview"]`), `geri_yukle` (yedekten; belgede kapsamı dürüstçe yaz).
- **Profiller:** Ubuntu 26.04 ve Debian 13. Debian 13'te varsayılan `systemd-timesyncd` olabilir; chrony kurulumu, timesyncd'nin devre dışı bırakılması ve `conf.d`/`sources.d` varlığı **doğrulanmamış**: gerçek Debian 13 üzerinde doğrula; doğrulanana kadar o profil `planned` kalır. Proxmox VE ve Raspberry Pi **kapsam dışı**.
- Kapsam dışı: hostname, apt kaynakları, NTS, makestep, `required: true` değişken (tüm eylemleri kapatır; gerçekten zorunlu değişken doğarsa eylem düzeyinde `requires_vars`).

## 5. Sahiplik ve etkileşim kuralları (linux-hardening ile)
- **Utility sahibi:** `/etc/chrony/conf.d/*`, `/etc/chrony/sources.d/*` (**`chrony.conf`'a dokunma**, yalnız drop-in), `/etc/localtime`, locale. **Chrony'yi yapılandırma değişince yeniden başlatan tek taraf utility.**
- **linux-hardening sahibi:** auditd, ufw, ssh ve yalnız zaman **raporu** (`time` rolü dosya yazmaz). Aynı ayarı iki kez yönetme. Değişken öneki `utility_*`, menü grubu ayrı; her çıktıda paket adı yazılır.
- Bilinen etkileşimler `warnings`'e yazılır: auditd `/etc/localtime`, `adjtimex`, `settimeofday`, `clock_settime` izler (saat dilimi değişince `time-change` kaydı beklenen); `hardening_ufw_outgoing_deny` açıksa yalnız **123/udp** çıkışı izinli (utility NTP kaynağı bu yüzden uyarır). **NTS-KE portu doğrulanmadı, v0.1'de NTS yok.**
- Yedek: değiştirmeden önce `/var/backups/utility/<tarih>/` (dizin 0700, dosyalar 0600) + manifest; geri alma güvenlik ağı (zamanlayıcılı rollback) **yok** (erişimi kesmez), yerine önizleme + yedek + idempotans.

## 6. Kabul ölçütleri (hepsi kanıtla)
1. `component_validate.py . --strict` (ve arşiv: `<id>-<sürüm>.tar.gz`) → hata/uyarı yok.
2. **İkinci `apply` `changed=0`** (idempotans), `preview` her koşuda değişiklik yazmaz.
3. **LAB e2e:** `test/e2e/lab` desenini (sshpocket reposunda, Colima + systemd'li Ubuntu 26.04 konteyneri) paketine uyarla: `durum` → `preview` → `apply` → ikinci `preview` changed=0 → `geri_yukle`; bats testleri yeşil. Kilit girişi olmadan önce LAB'a doğrudan `ansible-playbook` ile (ya da linux-hardening'in LAB betiklerine benzer bir `test/e2e` betiğiyle) çalış; sshpocket menüsünden uçtan uca deneme kilit PR'ından sonradır (koordinatörle planla).
4. **Gerçek VM turu:** kullanıcı yapar, sen komut metni hazırlarsın: ubuntutest (Ubuntu 26.04; IP DHCP ile .183/.186/.188 arasında gezer, `server.env`'e bak) ve **düz bir Debian 13 VM** (kullanıcıdan iste). Sıra: `durum` → `preview` → (snapshot teyidi) `apply` → bağımsız salt okunur ssh doğrulaması → ikinci `preview` changed=0 → `geri_yukle`. Ardından **hardening uygulanmış VM'de birlikte** (auditd `time-change` beklenir). Her satırı ham sonuç günlüğüne yaz (seçenek | platform | durum | not); sonuçlar koordinatöre gider.
5. `README.md` ve `CHANGELOG.md` dürüst: hangi profil denendi, hangisi denenmedi.

## 7. Git, imza, yayın kuralları (SERT)
- **Push, PR açma/birleştirme, etiket (`git tag`), imzalama, release, repo oluşturma kullanıcıdadır.** Sen: dal aç, commit et, test et; kullanıcıya **tek satır push komutu + PR başlığı + açıklaması** ver. Force push, `auto-merge`, silme (dal, dosya, uzak) **yok**.
- **İmza anahtarı ve parolası ajanda DEĞİL**: `~/.ssh/sshpocket_release` ve parolasına dokunma, okuma, ssh-add yapma, istenmedikçe yazma. İmzalayıcı **linux-hardening ile aynı** (`sshpocket-component` ad alanı, kilitteki aynı `signers` satırı). Yayın: `PAKET_SIGNING_KEY=… scripts/release.sh` / `release-publish.sh` komutunu **kullanıcı** çalıştırır; sen yalnız `--dry-run` metnini ve doğrulama adımlarını hazırlarsın.
- sshpocket'e kilit PR'ı: yayın sonrası `sshpocket` oturumu `scripts/lock-component.sh utility <sürüm> --repo syhgorath/utility` ile yapar (sen yapma; koordinatöre "X yayında" de, sha256 ve asset bilgisini ver).
- Bir bulguda linux-hardening'e dokunma; **koordinatöre** bildir, o ilgili oturuma iletir. Hassas değer (parola, token) hiçbir dosyaya/komuta yazılmaz.

## 8. Raporlama
Her aşama sonunda koordinatöre (`SendMessage` ile `sshpocket koordinator`) kısa özet: ne bitti, hangi ölçüt kanıtlı, neyin kullanıcı eylemi (push/PR/VM turu) gerekiyor, bulgular. Önce taslağı tamamla, kod bitince sor; belirsizlikte tahmin etme, tek kısa soru sor.
