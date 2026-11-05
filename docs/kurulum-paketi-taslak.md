# Taslak: `kurulum` paketi (Kurulum merkezi: Docker, nginx, MariaDB, Redis…)

> **Durum: TASLAK, kod yok, kullanıcı onayı bekliyor** (2026-11-05). Hazırlayan: sshpocket oturumu. Kararlar (§1) kullanıcı ile koordinatör arasında verildi;
> linux-hardening görüşü koordinatör üzerinden alındı (§8, hiçbiri hardening'li VM'de uçtan uca **denenmedi**). Standart: [package-standard.md](package-standard.md).
> Yazım işi ayrı oturuma verilir: `kurulum-yeni-oturum-brief.md`.

## 1. Verilen kararlar (kullanıcı)
| # | Karar |
|---|---|
| 1 | Mevcut `utility` (saat dilimi/locale/NTP) kimliği **aynı** kalır (kilitli); ekranda adı **"Locale ve zaman ayarları"** olur ve **sertleştirme grubunun içinde** görünür |
| 2 | Sunucu ana menüsüne **kurulum merkezi** girişi gelir; içinde yazılım listesi. Yazılıma girince **Native veya Docker** modu seçilir. Yeni paket kimliği **`kurulum`** (tek paket + içinde yazılım kataloğu; sshpocket paketler arası durum paylaşmaz, ikinci paket olmaz) |
| 3 | İlk dalga: **Docker taban kurulumu, nginx, MariaDB, Redis**. Sonra: apache, php-fpm/proje süreçleri, pihole, nginx proxy manager, log server, Suricata/Zeek, ollama |
| 4 | Docker: kurmadan önce **kontrol** (kurulu değil → kur ister; kurulu → dokunmaz; snap vb. farklıysa uyarır). Docker uygulamalarında yml **şablondan** üretilir; port/veri klasörü/imaj sürümü için **fzf formu**, ham yml "gelişmiş"; kaydetmeden önce sunucuda `docker compose config` ile doğrulanır. Port varsayılanı **127.0.0.1**; dışarı açmak ayrı onay (Docker ufw'yi atlar) |
| 5 | Port çakışması: sunucudaki dinleyenler (`ss`) + bizim kurduğumuz uygulamaların bildirdiği portlar (durmuş konteyner dahil) → "3333 portu X uygulamasına verilmiş" der, boş port önerir. 80/443 tek sahipli |
| 6 | **Ana klasör**: sunucu başına bir kez seçilir (varsayılan `/srv/homelab`), **sunucudaki** işaret dosyasına yazılır (`<ana>/.kurulum`); sonraki çalışmalarda yol oradan okunur, yerel/USB kopya farklıysa **durur ve uyarır** (servers/ ayrışması dersi). Sonradan taşıma v1'de yok (ayrı, açık onaylı "taşı" eylemi). Yol kontrolleri: mutlak, `/` `/etc` `/usr` yok, boşluk yok, tmpfs değil, izin 0750 root, df uyarısı. Docker'ın `data-root`'u (`/var/lib/docker`) **ayrı**, değiştirilmez |
| 7 | **.env**: tek merkezi dosya DEĞİL. `<ana>/.env` = gizli olmayan ortak değerler (host adı, port, docker ağ adı); `<ana>/<uygulama>/.env` = o uygulamanın gizli değerleri, 0600, parola **sunucuda rastgele** üretilir, Mac'e/ekrana/günlüğe yazılmaz. Uygulamalar birbirini mümkünse Docker ağı + servis adıyla bulur. Native ve Docker modu **aynı anahtarları** yazar (`MARIADB_HOST`, `MARIADB_PORT`) |
| 8 | **Veri güvenliği**: veri bind-mount ile `<ana>/<uygulama>/data` (adlandırılmış volume ve `compose down -v` yok); imaj etiketi **sabit** (`latest` yok); yazılım sürüm yükseltme **ayrı, açık onaylı eylem** + öncesinde döküm; geri düşürme desteklenmez; "kaldır" veriyi **silmez** (`apt purge` yok); bir sunucuda bir yazılım için tek mod. **Kabul ölçütü:** yükseltme testi (eski sürümü kur, veri yaz, paketi güncelle, tekrar apply, veri duruyor mu) LAB'da ve gerçek VM'de; geçmeden paket yayınlanmaz |

## 2. İlke: ufw'nin tek sahibi linux-hardening
`kurulum` ufw'ye **kendi yazmaz**. Dışarı açılması gereken portu **öneri** olarak üretir ve kullanıcının onayıyla `servers/<ad>/linux-hardening.vars` içindeki `hardening_ufw_extra_allow_host` değişkenine eklenmesini önerir (örn. DB için `from <ağ> to any port 3306 proto tcp`, `any` değil). Dikkat: Ansible liste değişkenlerini **birleştirmez, üzerine yazar** (grup değişkeni `hardening_ufw_extra_allow` host'ta `_host` ile karışmaz). Varsayılan `127.0.0.1` yayınında ufw kuralı gerekmez. Docker yayınlanan portu ufw deny'a rağmen açar (LAB'da Docker 29.1.3 ile doğrulandı; `DOCKER-USER` ile kısıtlama **denenmedi**).
sshpocket'e yeni değişken yazma kuralı geçerli: kullanıcı ayarı onaysız yazılmaz.

## 3. sshpocket'in bugünkü yetenekleri (koddan) ve eksikler
| Konu | Bugün | Bu tasarım için gereken |
|---|---|---|
| **Menüde paket gösterimi** | Sunucu menüsünde tek satır `🛡️ Paketler (sertleştirme vb.)` → kurulu paketlerin **düz listesi** (ad + sürüm) → paket paneli (eylemler, Değişkenler, Geçmiş, Bulgular). Kategori/grup alanı **yok** | **S1** İsteğe bağlı manifest alanı `menu: {category, title, order}`; kategori sunucu menüsünde kendi satırı olur: `🛡️ Sertleştirme` (linux-hardening, utility "Locale ve zaman ayarları") ve `🧰 Kurulum merkezi` (kurulum). Kimlik değişmez (kilit/kayıt aynı) |
| **Yazılım seçimi** | Eylem `cmd`'si sabit; seçim ancak bir `enum` değişkeni düzenleyip sonra eylemi çalıştırmakla olur | **S2** Manifestte `items[]` (yazılım kataloğu: `id`, `label`, `description`, `modes`, `params[]`); menüde yazılım listesi + yazılım başına eylemler (`durum`, `kur`, `yukselt`, `kaldir`). Paket tarafı katalog olmadan da çalışır (aşağıda geçiş yolu) |
| **Koşuya özel değişken** | `apply_vars` yalnızca platform tanımlı bool/enum içindir ("bu koşuya özel", `.vars` dosyasına yazılmaz, onaydan önce metni gösterilir) | **S3** Seçilen yazılım ve mod gibi **parametreleri** aynı mekanizmayla koşuya özel geçir (tür doğrulamalı, yalnızca manifestte tanımlı adlar, argv'ye değil değişken dosyasına) |
| **Form (fzf)** | Alan listesi editörü (`Bilgileri güncelle`), liste editörü, değer seçici, hepsi ortak `ui_fit_wide`/kart kuralıyla | **S4** `items[].params[]` (tür: `port`, `path`, `string`, `enum`, `image-tag`; varsayılan, kural) için aynı alan listesi editöründen **genel form**; değerler `servers/<ad>/kurulum.vars` içinde (`kurulum_<yazılım>_<parametre>`, **gizli değer yok**); ham yml "gelişmiş" |
| **Ana klasör sorusu** | `onboarding_vars` (ilk açılışta zorunlu değişken sorusu) var; `list[cidr]` için özel editör, düz string için `read` | Mevcut mekanizma yeter: `kurulum_ana_klasor` bir `onboarding_var`; **gerçek kaynak sunucudaki işaret dosyasıdır**, yerel değer yalnızca istek. Fark çıkarsa durma kararı **paketin** (§5) |
| **Eylem öncesi okuma** | `requires[]` (örn. `preview` taze olmalı, TTL var) | `durum` eylemini `requires` ile ön koşul yap: işaret dosyası, Docker durumu, port tablosu her değiştiren eylemden önce okunur |
| **sudo parolası** | `requires_sudo` artık `--ask-become-pass` ekler (3.6.23 sonrası düzeltme, §7) | – |
| **Doğrulayıcı / şablon** | Bilinmeyen manifest alanlarını reddetmiyor ama doğrulamıyor | Yeni alanlar (`menu`, `items`) `component_manifest.py`'de **doğrulanmalı**; vendored kopya `sshpocket-paket-sablonu` ile senkronlanır |

**Geçiş yolu (ayrık geliştirme):** paket v0.x, S1-S4 olmadan **bugünkü sshpocket ile çalışır**: yazılım ve mod `kurulum_yazilim`, `kurulum_mod` enum değişkenleri, port/veri klasörü değişkenleri ve eylemler `durum`, `preview`, `kur`, `yukselt`, `kaldir`. S1-S4 daha sonra yalnızca **arayüzü** iyileştirir (katalog ve form manifestten gelir), playbook'lar değişmez. Böylece paket ve sshpocket işi paralel yürür.

## 4. Paketin yapısı (öneri)
- **Tek paket `kurulum`**, ortak roller: `docker_kontrol` (kurulu mu / snap / dağıtım paketi / Docker deposu), `ana_klasor` (işaret dosyası, yol kontrolleri), `port_kaydi` (`ss` + `<ana>/.kurulum/portlar`), `env_dosyalari` (`<ana>/.env`, `<ana>/<uygulama>/.env`), yazılım rolleri `nginx`, `mariadb`, `redis`, `docker_taban`; yml'ler `templates/*.yml.j2`.
- **Eylemler:** `durum` (salt okunur), `preview` (`--check`), `kur` (installs-package, changes-config; `requires: preview`; `confirm: yes-no`), `yukselt` (**ayrı**, öncesinde döküm; `confirm: type-server-name`), `kaldir` (veriyi silmez; `confirm: yes-no`), `tasi` (v1 dışı).
- **Mod ve veri yeri:** Docker: `<ana>/<uygulama>/data` bind-mount. Native: dağıtımın yeri (`/var/lib/mysql` vb.), işaret dosyası modu ve yolu kaydeder; **açık soru §9-7**.
- **Gizli değer:** sunucuda `/dev/urandom` ile üretilir, `no_log: true`, `<ana>/<uygulama>/.env` 0600; `SSHPOCKET_RESULT` satırları **anahtar adı** yazar, değer yazmaz; `last-run.log` hiçbir parola içermemeli (test: günlükte parola desenini ara).
- **Port ve ağ:** varsayılan `127.0.0.1:<port>`; dışa açma ayrı `warning` + onay; `docker network` adı `<ana>/.env` içinde; uygulamalar birbirini servis adıyla bulur.
- **Çakışma:** `durum` çıktısı `port=3306 sahip=mariadb mod=docker` gibi; çakışmada "boş port öner" (`ss -ltn` + kayıt).

## 5. Ana klasör ve işaret dosyası kuralları
- İşaret `<ana>/.kurulum` (JSON/anahtar=değer): `ana_klasor`, `surum`, kurulu yazılımlar (`id`, `mod`, `surum`, `port`, `veri_yolu`), oluşturma tarihi.
- Yerel `servers/<ad>/kurulum.vars` ile sunucudaki işaret **farklıysa**: paket **durur** ("ana klasör sunucuda X, yerelde Y") ve hangisine uyulacağını kullanıcıya bırakır; sessiz düzeltme yok.
- Yol kontrolleri `preview`'da çalışır (mutlak yol, yasak köklerin dışı, boşluk yok, tmpfs değil, 0750 root, `df` yeterli yer).

## 6. Veri güvenliği ve yükseltme
- Yükseltme: sürüm **sabit etiketle** yükseltilir; öncesinde döküm (MariaDB `mariadb-dump`, Redis `SAVE`/RDB kopyası) `<ana>/<uygulama>/yedek/<tarih>/` 0700/0600; **geri düşürme yok**, bunu `warnings`'e yaz.
- Kaldır: konteyneri/paketi durdurur, **veri klasörüne dokunmaz**; `apt purge`, `compose down -v`, adlandırılmış volume **yasak** (statik denetim: playbook'ta bu desenler yok).
- **Kabul testi (hepsi geçmeden yayın yok):** LAB'da ve gerçek VM'de: eski sürümü kur → veri yaz (`CREATE TABLE`, `SET`) → paketi yükselt → tekrar `kur` → veri duruyor mu.

## 7. Bu oturumda bulunan ilgili sshpocket düzeltmesi
`utility` (3.6.23) sudo parolasını **sormuyordu**: `requires_sudo: true` tek başına `--ask-become-pass` eklemiyordu (yalnızca paket `invoke.append` yazdıysa; linux-hardening yazıyor, utility yazmıyordu). Parolalı sudo'da eylem istemsiz `Missing sudo password` ile düşüyor, NOPASSWD'de sessizce geçiyordu. sshpocket artık `requires_sudo` eylemde istemi kendisi ekler (Docker LAB'da parolalı sudo ile doğrulandı). `kurulum` bu davranışa güvenebilir, `invoke.append` yazması gerekmez.

## 8. linux-hardening ile etkileşim (koordinatör üzerinden alındı; **uçtan uca DENENMEDİ**)
| Konu | Bilgi |
|---|---|
| Docker uyumu | hardening `ip_forward`'a yazmaz (Docker açar). `rp_filter` generic'te 1 (asimetrik/çok arayüzlü kurulumda sorun olabilir, doğrulanmadı). `br_netfilter`/`overlay` engeli yok **ama** `hardening_kernel_modules_disabled=true` (varsayılan kapalı) Docker'ı bozar: kurulumdan **sonra** uygulanmalı ya da kapalı kalmalı |
| ufw | `hardening_ufw_routed_deny` (varsayılan true) Docker'ın kendi zincirindeki yayınlanan portları etkilemez; **compose + routed deny + restart tam senaryosu doğrulanmadı** |
| `hardening_docker_enabled` (varsayılan kapalı) | Docker'ı `/usr/bin/dockerd` ile algılar; `daemon_manage=false` iken `daemon.json`'a dokunmaz, true iken `live-restore` + log-opts **birleştirir** (yedekli); yayınlanan portları raporlar (`hardening_docker_allowed_published` [80,443] dışını uyarır); `docker.service/socket/daemon.json` izinlerini sıkar, auditd Docker kuralları ekler |
| Port açma | `hardening_ufw_extra_allow` (grup) ve `hardening_ufw_extra_allow_host` (sunucu); biçim `443/tcp`, `80`, `8000:8010/tcp`, `from 10.0.0.0/24 to any port 3306 proto tcp` |
| `/tmp` ve `/dev/shm` noexec | `hardening_tmp_noexec`/`shm_noexec` varsayılan **true**, yalnız **ayrı tmpfs** `/tmp`'te: paket postinst'leri, PHP/Node işleri bozulabilir; MariaDB/nginx kurulumunda **doğrulanmadı** |
| Diğer | `libpam-tmpdir` `TMPDIR=/tmp/user/<uid>`, oturum umask 027 (elle oluşturulan dosyalar 027; systemd servis umask'ı genelde etkilenmez); `hardening_ufw_outgoing_deny` (varsayılan kapalı) açılırsa apt ve `docker pull` kapanır; AppArmor enforce **kapalı**, açılırsa özel data dir'li MariaDB reddedilebilir (doğrulanmadı) |

**Sonuç → kabul ölçütüne iki sıralı hardening'li VM turu:** (a) hardening apply **sonrası** kurulum, (b) kurulum **sonrası** hardening apply; ikisinde `/tmp` noexec ile MariaDB, nginx ve `docker pull` testi.

## 9. Açık sorular (kullanıcı kararı gerekir)
1. **Kimlik/ad:** paket `kurulum`; ana menüde **"Kurulum merkezi"** (kullanıcının "utility" demesi mevcut `utility` paketiyle karışıyor): onaylıyor mu?
2. **S1 kategori alanı:** serbest metin mi, sabit liste mi (`sertlestirme`, `kurulum`, …)? Menü sırası (`order`)?
3. **`utility` yeniden adlandırma:** yeni `utility` sürümü (görünen ad + kategori) ayrı bir iş, **kimlik değişmez**; yazan: utility oturumu. Önce S1 mi, yoksa önce paket tarafı mı?
4. **Katalog nerede yaşar:** manifest `items[]` (sshpocket ssh'sız listeler, form çizer; önerim) mı, playbook çıktısı mı (ssh gerekir)?
5. **İmaj sürümleri:** "latest yok" ama sürüm listesi **kim güncel tutar**? Paket sürümüyle gelen küratörlü liste (önerim) ve kullanıcı elle girebilir (uyarılı)?
6. **Docker kaynağı:** dağıtım `docker.io` mu, Docker'ın resmi apt deposu mu (üçüncü taraf anahtar)? Ubuntu 26.04 / Debian 13'te her ikisinin durumu **doğrulanmadı**; tedarik zinciri riski.
7. **Native veri yeri:** dağıtım varsayılanı (`/var/lib/mysql`) mi, `<ana>/<uygulama>/data` mı (AppArmor/izin riski)? Native yükseltme `apt`'tir (sürüm sabitleme farklı): kapsam?
8. **Docker'a erişim:** komutlar `sudo` ile mi (önerim), kullanıcıyı `docker` grubuna eklemek root-eşdeğeri olduğu için yapılmasın mı?
9. **LAB mümkün mü:** Docker modu için LAB konteynerinde Docker daemon (privileged / DinD) gerekir; yapılamazsa Docker yükseltme testi yalnızca gerçek VM'de. Kabul ölçütü "LAB **ve** gerçek VM" ise LAB tasarımı gerekir.
10. **Çoklu çalıştırma:** aynı sunucuda iki koşu çakışmasın diye sunucuda `flock` (ana klasörde kilit dosyası) kabul mu?
11. **Sıra:** önce S1-S4 (sshpocket), sonra paket mi; yoksa §3'teki geçiş yoluyla paralel mi (önerim: paralel)?

**Doğrulanmamışlar:** hardening'li VM'de Docker+MariaDB+nginx uçtan uca; `/tmp` noexec ile postinst'ler; Docker kaynak seçimi; LAB'da Docker; `DOCKER-USER` ile ufw kısıtlama.
