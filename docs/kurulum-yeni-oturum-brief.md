# İş tarifi: `kurulum` paketini yaz (Kurulum merkezi, v0.x)

> Bu belge kendi başına yeter. **Önce kullanıcı onayı gerekir** (taslak §9 açık soruları); onaydan sonra yeni oturum için yazıldı (2026-11-05).
> Karar sahibi: kullanıcı (Ziya, Türkçe çalışır; kısa, tablolu yanıt; komutlar ayrı kod bloklarında). Köprü: **sshpocket koordinatör** oturumu (sshpocket ve linux-hardening oturumlarıyla seni o bağlar).

## 1. Görev
Yeni sshpocket paketi **`kurulum`**: sunucuya yazılım kurma merkezi. İlk dalga: **Docker taban kurulumu, nginx, MariaDB, Redis**; her biri **Native veya Docker** modunda. Tasarım ve **kesin kararlar**: `~/Developer/sshpocket/docs/kurulum-paketi-taslak.md` (§1 kararlar, §2 ufw ilkesi, §4 yapı, §5-6 ana klasör ve veri güvenliği, §8 hardening etkileşimi). Standart: `docs/package-standard.md` (özellikle §4-§8, §12). **Önce ikisini oku**; bu tarif yalnızca iş sırasını ve sınırları yazar.

## 2. Konum ve kaynaklar
| Ne | Nerede | Kural |
|---|---|---|
| Yeni repo (yerel) | `~/Developer/kurulum` | `git init`; paket kimliği `kurulum`; GitHub deposu `syhgorath/kurulum` (**depoyu kullanıcı açar**; `gh repo create` çalıştırma) |
| Şablon | `~/Developer/sshpocket-paket-sablonu` | Okunur, değiştirme (`main`) |
| Doğrulayıcı | `tools/validate/` (şablondan) | Güncelleme: `tools/sync-validator.sh <sshpocket-main kopyası>` |
| Örnek paketler | `~/Developer/utility` (kısa, kopya al) ve `~/Developer/linux-hardening` (**yalnız OKU**) | Kopyalanacaklar: `scripts/release.sh`, `release-publish.sh`, `make-component-json.py`, `test/lib.sh`, `check-consistency.py`; rollback, `SSHPOCKET_PROGRESS` callback, `bulgular.tsv` alınmaz |
| sshpocket | `~/Developer/sshpocket` | Yalnız OKU; ona kod yazma. Paket sshpocket'e **tek satır kilit** PR'ıyla girer |

## 3. Geliştirme yolu: bugünkü sshpocket ile (menü özellikleri beklenmez)
Paket v0.x, sshpocket'e **yeni özellik gerektirmeden** çalışmalı (taslak §3 "geçiş yolu"):
- Yazılım ve mod: `kurulum_yazilim` (enum), `kurulum_mod` (enum: `native`/`docker`) **değişkenleri**; port, ana klasör, veri klasörü, imaj etiketi değişkenleri (`kurulum_<yazılım>_<parametre>`, **gizli değer yok**).
- Eylemler: `durum` (salt okunur), `preview` (`--check`, salt okunur), `kur` (`requires: ["preview"]`), `yukselt` (ayrı), `kaldir` (veriyi silmez). `requires_sudo` eylemlerde `--ask-become-pass`'i **sshpocket ekler** (3.6.23 sonrası düzeltme), `invoke.append` yazma.
- `required: true` kullanma (tüm eylemleri kapatır); gerçekten zorunlu değişkende eylem düzeyinde `requires_vars`. Ana klasör için `onboarding_vars` (kaynak **sunucudaki işaret dosyasıdır**, yerel değer yalnız istek).
- Katalog/menü/form (S1-S4) sshpocket'te **sonra** gelir ve yalnızca arayüzü iyileştirir; **playbook'lar değişmez**. Manifestte henüz tanınmayan alan (`menu`, `items`) YAZMA (doğrulayıcı sürümü gelince).

## 4. Şablondan türetme adımları
1. `cp -R` şablonu `~/Developer/kurulum`'a (`.git` hariç), `git init`, ilk commit.
2. `component.json` (`id: kurulum`, `inventory_group: kurulum`, profiller **ubuntu-26.04** ve **debian-13**; Proxmox/Pi yok), `playbooks/*.yml`, `VERSION` `0.1.0`, `CHANGELOG.md`, `README.md`, `requirements.txt` hash'li (şablondaki).
3. Roller: `docker_kontrol`, `ana_klasor`, `port_kaydi`, `env_dosyalari`, `docker_taban`, `nginx`, `mariadb`, `redis`; yml'ler `templates/*.yml.j2`. Her rol `--check` ile çalışır, **idempotent**, yedekli (taslak §4).
4. Yerel doğrulama: `python3 -I tools/validate/component_validate.py . --strict --unsigned` → hata ve uyarı **yok**.

## 5. Değişmez kurallar (taslak §1, §2, §5, §6)
- **ufw'nin tek sahibi linux-hardening:** ufw'ye **yazma**; açılması gereken portu **öneri** olarak göster (hardening `hardening_ufw_extra_allow_host` biçimi, `any` değil `from <ağ> to any port …`); varsayılan yayın **127.0.0.1**, dışarı açma ayrı onay.
- **Docker kontrolü** (kurulu değil → kur ister; kurulu → dokunma; snap/farklı → uyar). Docker deposu kaynağı (dağıtım `docker.io` / resmi apt deposu) **kullanıcı kararına bağlı** (taslak §9-6); karar yoksa durup koordinatöre sor.
- **Ana klasör:** sunucuda `<ana>/.kurulum` işaret dosyası **gerçek kaynak**; yerel kopya farklıysa **dur ve uyar**. Yol kontrolleri `preview`'da.
- **Veri:** bind-mount `<ana>/<uygulama>/data`, adlandırılmış volume yok, `compose down -v` yok, `apt purge` yok, imaj etiketi **sabit** (`latest` yok); "kaldır" veriyi silmez; yükseltme ayrı onaylı eylem + öncesinde döküm; geri düşürme yok.
- **Gizli değer:** sunucuda rastgele üretilir, `<ana>/<uygulama>/.env` 0600, `no_log: true`; `SSHPOCKET_RESULT` ve `last-run.log` parola içermez (testle doğrula).
- **Sunucu adı onayı sadece `yukselt`'te** (`type-server-name`); `kur`/`kaldir` `yes-no`.

## 6. Kabul ölçütleri (hepsi kanıtla)
1. `component_validate.py . --strict` (ve arşiv `<id>-<sürüm>.tar.gz`) → hata/uyarı yok.
2. **İkinci `kur` `changed=0`** (idempotans); `preview` hiçbir şey yazmaz.
3. **Yükseltme testi** (eski sürümü kur, veri yaz, paketi güncelle, tekrar `kur`, veri duruyor mu): LAB'da **ve** gerçek VM'de; **geçmeden yayın yok.** LAB'da Docker daemon gerekir (taslak §9-9): olmuyorsa Docker modu yalnız gerçek VM'de sınanır ve bunu dürüstçe yaz.
4. **Parola sızmıyor:** `last-run.log`, sonuç satırları, ekranda üretilen parola deseni aranır.
5. **Gerçek VM turu** (kullanıcı yapar, sen komut metni hazırlarsın): ubuntutest (Ubuntu 26.04; IP DHCP ile gezer, `server.env`'e bak) ve düz Debian 13 VM (debiantest). Sıra: `durum` → `preview` → (snapshot teyidi) `kur` → bağımsız salt okunur ssh doğrulaması → ikinci `preview` changed=0 → `yukselt` testi. **İki sıralı hardening'li tur:** (a) hardening apply **sonrası** kurulum, (b) kurulum **sonrası** hardening apply; ikisinde `/tmp` noexec ile MariaDB, nginx ve `docker pull` testi.
6. README ve CHANGELOG dürüst: hangi profil/mod denendi, hangisi denenmedi.

## 7. Git, imza, yayın kuralları (SERT)
- **Push, PR açma/birleştirme, etiket (`git tag`), imzalama, release, repo oluşturma kullanıcıdadır.** Sen: dal aç, commit et, test et; kullanıcıya **tek satır push komutu + PR başlığı + açıklaması** ver. Force push, `auto-merge`, silme (dal, dosya, uzak) **yok**.
- **İmza anahtarı ve parolası ajanda DEĞİL**: `~/.ssh/sshpocket_release` ve parolasına dokunma, okuma, `ssh-add` yapma. İmzalayıcı **linux-hardening ile aynı** (`sshpocket-component` ad alanı). Yayın komutunu **kullanıcı** çalıştırır.
- sshpocket'e kilit PR'ı: yayın sonrası `sshpocket` oturumu `scripts/lock-component.sh kurulum <sürüm> --repo syhgorath/kurulum` ile yapar (sen yapma); koordinatöre "X yayında", sha256 ve asset bilgisini ver.
- linux-hardening veya sshpocket'te bir bulgu/eksik çıkarsa dokunma, **koordinatöre** bildir. Hassas değer (parola, token) hiçbir dosyaya/komuta yazılmaz.

## 8. Raporlama
Her aşama sonunda koordinatöre (`SendMessage` ile `sshpocket koordinator`) kısa özet: ne bitti, hangi ölçüt kanıtlı, neyin kullanıcı eylemi (push/PR/VM turu) gerekiyor, bulgular. Belirsizlikte tahmin etme, tek kısa soru sor. Önce **Docker taban kurulumu + nginx** ile başla (en küçük uçtan uca dilim), sonra MariaDB ve Redis.
