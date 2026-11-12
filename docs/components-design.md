# Tasarım: `servers/` veri ayrımı ve bileşen (component) modeli

> **Durum (2026-10-17):** Aşama 1 (veri ayrımı), Aşama 2 (paket altyapısı: kilit, imza, kurulum, bildirim, venv, `--doctor`, `--components`) ve
> **Aşama 3 (🛡️ Hardening: `--components run`, çalıştırma öncesi kapılar, sunucu menüsünde Paketler/Değişkenler/Geçmiş)** **uygulandı** ve sshpocket **3.6.0**'da.
> `linux-hardening 0.2.0` ve (2026-10-18) **0.3.0** gerçek bir Ubuntu 26.04 sunucusunda ve Docker LAB'ında `preview` + `apply` ile denendi. **Sırada:** Aşama 4 (🧰 Araçlar, `runner: script`),
> klasik knock sunucu paketi, rpi/Proxmox profilleri. Kararlar 2026-10-10'da alındı; uygulama kararları [package-standard.md](package-standard.md) §13-§14'te.
>
> **Uygulamada tasarımdan sapmalar:** (1) bildirim `component.json`'dır (YAML değil; bkz. package-standard §13); (2) daha yeni bir paket
> yayını **yalnızca bilgilendirir**, kurulmaz: sürüm daima imzalı sshpocket sürümüyle gelen `components.lock`'tadır;
> (3) `--doctor` eksik olanı kendisi kurmaz, çözüm komutunu gösterir.

> **Paket yazarı için normatif özet:** [package-standard.md](package-standard.md) (v1).

## 1. Neden
- Kod (`sistem/`) ile kullanıcı verisi (sunucular, key'ler) aynı klasörde: güncelleme, yedekleme ve taşıma
  ("Modules'a dokunma") bu yüzden karmaşık.
- Her sunucu için **bash modül dosyası üretiyoruz**, oysa tek gerçek bilgi `.env`. Üretilmiş kod, "rigit" hissin bir kaynağı.
- `Autoload`, kullanıcı klasörlerindeki tüm `.sh` dosyalarını **çalıştırıyor**. Veri klasörü hiç çalıştırılmamalı.
- Sunucu sertleştirme (linux-hardening) ve ileride Docker/web sunucusu kurma gibi işleri **ayrı repolardan**,
  birbirine karışmadan, tek menüden yönetmek istiyoruz.

## 2. Alınan kararlar

| # | Karar | Gerekçe |
|---|---|---|
| 1 | Bileşenler, **sshpocket sürümüyle sabitlenir** (`components.lock`) + yeni imzalı sürüm varsa **bilgilendirir**, onayla kurar | Birlikte test edilmiş sürümler; sunucularda kök yetkisiyle çalışan kod denetimsiz değişmez |
| 2 | Bileşen **içeriği ve kilitleri USB'de** (`components/`), **venv bir önbellektir** ve Mac'te üretilir (`~/.cache/sshpocket/venv/<ortam-kimliği>`); isteğe bağlı USB'de wheelhouse | Bilgisayar kaybolsa bile her şey USB'de kalır; venv taşınamaz (mutlak yol, mimari, exFAT'ta symlink yok) ama her makinede yeniden üretilebilir |
| 3 | Bileşen paketleri **aynı imza anahtarıyla** imzalanır; kilit dosyasında bileşen başına `signers` alanı vardır | Tek bakımcı için basit; ileride ayrı anahtar kod değişmeden eklenir |
| 4 | Kök dizinde **`servers/`** klasörü: kullanıcı verisi buraya | `sistem/` tamamen kod olur; yedeklenecek tek klasör |
| 5 | Sunucu ayarında **genel anahtar adları**: `IP`, `USER`, `PORT`, `KEY`, `OS` | `UBUNTUTEST_IP` gibi önekler üretilmiş koda bağlıydı |
| 6 | Sunucu başına **üretilmiş bash dosyası kaldırılır**; uygulama `servers/` klasörünü tarayıp menüyü kendisi kurar | Daha az kod, daha az enjeksiyon yüzeyi |
| 7 | Sunucu menüsüne **🛡️ Hardening** ve **🧰 Araçlar** (Docker kur, web sunucusu kur…) eklenir; hepsi ayrı repo | Çekirdek ince kalır |

## 3. Dizin düzeni

```
sshpocket/
├── start.sh, start.command, VERSION, components.lock, ...
├── sistem/                  UYGULAMA KODU: güncelleyici bütünüyle değiştirir; kullanıcı verisi içermez
│   ├── Main.sh, Update.sh, Helpers/, ...
│   └── release_signers
├── servers/                 KULLANICI VERİSİ: git'e girmez, hiçbir zaman çalıştırılmaz (source edilmez)
│   └── <ad>/
│       ├── server.env       IP=  USER=  PORT=  KEY=(ops.)  OS=(ops.)
│       ├── id_ed25519_<ad>[.pub]
│       └── hardening.env    (ops.) bileşen değişkenleri
└── components/              BİLEŞENLER (USB'de kalır; exFAT'ta symlink olmadığı için `current` düz metin dosyasıdır)
    ├── linux-hardening/
    │   ├── 0.1.0/           imzası doğrulanmış içerik (component.yml, playbook'lar, requirements.txt)
    │   └── current          tek satır: "0.1.0"
    └── .wheelhouse/<platform>/    (ops.) çevrimdışı Python paket önbelleği, hash'li

~/.cache/sshpocket/venv/<ortam-kimliği>/      (Mac'te; silinebilir, otomatik yeniden üretilir)
```

- Sunucu adı kuralı değişmez: `^[a-z][a-z0-9_]*$`. Klasör adı = kimlik.
- Keychain hesabı **aynı kalır**: `USBMonitor_<AD_BÜYÜK>_Passphrase` (servis `USBMonitor`). Passphrase'ler yeniden girilmez.
- `.gitignore`: `servers/*` (yalnızca `servers/.gitkeep` ve `servers/README.md` izlenir). `example` artık sunucu değil,
  `sistem/templates/server.env.example`.

## 4. Uygulama davranışı
- Menü, `servers/*/server.env` dosyalarından kurulur. **Veri klasöründen hiçbir dosya `source` edilmez.**
  `server.env`, bugünkü güvenli ayrıştırıcıyla (satır okuma, `eval` yok) okunur ve her değer doğrulanır
  (`ssh_validate_target`).
- Tek, genel `ssh_module_run <ad>`; sunucu başına fonksiyon/dosya yok.
- `Autoload` yalnızca `sistem/` altındaki yardımcıları yükler.
- Gizli/AppleDouble dosyalar (`._*`, `.DS_Store`) yine yok sayılır; testle güvence altında kalacak.

## 5. Geçiş (migration)
Sürüm N → N+1, otomatik ve geri alınabilir:
1. İlk çalıştırmada `sistem/Modules/<ad>/` (ve `example` dışındaki her klasör) bulunursa **deneme çıktısı** gösterilir, onay istenir.
2. Yeni biçime çevrilir: `servers/<ad>/server.env` (`<ÖNEK>_IP` → `IP`, … `<ÖNEK>_KEY` → `KEY`), key dosyaları taşınır.
   Üretilmiş `<Ad>Module.sh` dosyaları **taşınmaz**.
3. Doğrulama: ayrıştırılan değerler eski/yeni aynı mı, key dosyaları var mı. Başarılıysa eski klasör
   `servers/.migrated-<tarih>/<ad>/` olarak **silinmeden** kenara alınır (kök, `sistem/` değil: güncelleyici `sistem/`'i bütünüyle değiştirebilsin diye).
4. İdempotent: ikinci çalıştırmada yapacak bir şey yok. Hata olursa hiçbir şey silinmez.
5. **Geriye uyumluluk okuyucusu:** `server.env` içinde eski `<ÖNEK>_IP=` biçimi görülürse okunur (uyarıyla).
6. Güncelleyici (yeni sürüm), `sistem/`'i bütünüyle değiştirmeden **önce** eski `Modules/`'i bu geçişle `servers/`'a alır;
   kullanıcı verisi hiçbir noktada silinmez.

## 6. Bileşen (component) modeli

### 6.1 Sözleşme: `component.yml` (bileşen reposunun kökünde)
Biçim, `linux-hardening`'in `main`'indeki gerçek dosyayla hizalıdır (2026-10-10, issue #2).
```yaml
id: linux-hardening
version: 0.1.0               # VERSION dosyası ve etiket (v0.1.0) ile aynı olmalı
sshpocket_api: 1             # sözleşme sürümü; uyumsuzsa sshpocket reddeder

requires:
  control:                   # KONTROL makinesi (sshpocket'ın çalıştığı Mac)
    python: ">=3.12"         # ansible-core 2.21.5 Requires-Python ile aynı
    packages: [ansible-core] # sürüm/hash: requirements.txt
  target:                    # HEDEF sunucu (kontrol makinesiyle karıştırılmaz)
    python: ">=3.9"          # ansible-core 2.21 modüllerinin hedefte gerektirdiği alt sınır (module_utils/basic.py _PY_MIN)

inventory_group: hardening   # üretilen inventory'de hedefin konacağı grup (eski 'ubuntu' geçiş için kabul edilir)

profiles:                    # status: supported | planned
  - { id: ubuntu-26.04, status: supported }   # eşleme: ubuntu-26.04 <-> hardening_supported_os {Ubuntu: ["26.04"]}
  - { id: rpi,          status: planned }
  - { id: proxmox,      status: planned }

actions:
  audit:
    cmd: "ansible-playbook playbooks/audit.yml"
    effects: [installs-package]            # hedefe lynis paketini kurar; yapılandırmayı değiştirmez
    requires_sudo: true
    outputs:
      dir_var: hardening_scan_dir          # çıktı klasörü bu değişkenle verilir (varsayılan: bileşen içi scans/)
      summary: "<dir>/<host>/<tarih>/summary.json"
  preview:
    cmd: "ansible-playbook playbooks/harden.yml --check --diff"
    effects: []
    requires_sudo: true
  apply:
    cmd: "ansible-playbook playbooks/harden.yml"
    effects: [installs-package, changes-config]
    requires_sudo: true
    requires: [preview]                    # sshpocket önce preview çalıştırıp göstermeli

  # Geri alma güvenlik ağı (hardening_rollback, v0.1.0): kontrol kaybından sonra durumu görmek/iptal etmek için.
  # Bu eylemler preflight/erişim korumalarını ÇALIŞTIRMAZ (kontrol kaybından sonra da çalışabilmeli).
  rollback_status: { cmd: "ansible-playbook playbooks/rollback.yml",              effects: [],               requires_sudo: true }
  rollback_disarm: { cmd: "ansible-playbook playbooks/rollback.yml --tags disarm", effects: [changes-config], requires_sudo: true }
  rollback_now:    { cmd: "ansible-playbook playbooks/rollback.yml --tags now",    effects: [changes-config], requires_sudo: true }

vars: [hardening_ssh_allow_from, hardening_scan_dir, hardening_rollback_enabled, hardening_rollback_timeout]
      # hardening_ssh_port YOK (kaldırıldı; port değiştirme desteklenmiyor)

invoke:                      # sshpocket'ın cmd'nin sonuna eklediği argümanlar
  append:                    # NESNE listesi (string değil)
    - { arg: "-i", value: "<üretilen geçici inventory>", required: true }
    - { arg: "--limit", value: "<sunucu>", required: true }
    - { arg: "--ask-become-pass", required: true, interactive: true }   # sudo parolasını Ansible kendi istemiyle alır
    - { arg: "-e", value: "@<değişken dosyası>", required: false }
```
- `effects`: `[]` (salt okunur) | `installs-package` | `changes-config`; **birden fazla olabilir** ve menü hepsini gösterir.
- `invoke.append[]`: `arg` bayrak, `value` yer tutucu (`<...>`; sshpocket doldurur), `required` zorunluluk, `interactive` kullanıcıdan
  istem bekler (sshpocket bunu gizli/sessiz çalıştırmaz, terminali devreder).
- `inventory_group`: sshpocket geçici inventory'de sunucuyu **bu grupta** üretir (sabit `ubuntu` varsayımı yok).
- `actions.*.outputs`: sshpocket çıktı klasörünü **kullanıcı klasörüne** (USB'ye değil) yönlendirir ve `summary.json`'u okuyup gösterir.
- `requires` (ön koşul eylemler) beyandır; **zorlamayı sshpocket yapar** (önizleme başarılı olmadan `apply` açılmaz).
- sshpocket bileşenin **içini bilmez**; yalnızca sözleşmeyi ve eylemleri çalıştırır.

### 6.2 Kilit dosyası: `components.lock` (sshpocket reposunda, imzalı sürümle gelir)
```yaml
components:
  linux-hardening:
    repo: syhgorath/linux-hardening
    version: 0.1.0
    sha256: <paket özeti>
    signers: "sshpocket-component namespaces=\"sshpocket-component\" ssh-ed25519 AAAA..."   # principal = ad alanı
```
- **Principal ve ad alanı aynı değerdir:** `sshpocket-component`. Doğrulama `ssh-keygen -Y verify -I sshpocket-component
  -n sshpocket-component` ile yapılır. `-I`, signers satırının **ilk alanıyla** birebir aynı olmalıdır; eşleşmezse
  OpenSSH yalnızca genel bir `Could not verify signature.` (çıkış 255) verir ve nedenin principal olduğunu söylemez,
  bu yüzden aracımız bunu doğrulamadan önce kendisi kontrol edip anlaşılır bir hata üretir.
  Aynı anahtar çekirdek (`sshpocket-release`) ve bileşen (`sshpocket-component`) için, **farklı principal+ad alanı** satırlarıyla kullanılabilir.
- Yeni bileşen eklemek = kilit dosyasına satır (imzalı sshpocket sürümü) → bilinçli güven kararı.
  Kilit dosyasında olmayan bileşen **çalıştırılmaz**.

### 6.3 Kurulum ve doğrulama
- **İmza ad alanı:** bileşenler `sshpocket-component` ad alanıyla imzalanır (çekirdek: `sshpocket-release`). Ayrı ad alanı bilerek seçildi:
  bir bileşen imzası çekirdek güncellemesi olarak yeniden oynatılamaz. Doğrulayıcı güvenilen anahtarı `release_signers`'tan değil
  `components.lock`'taki `signers` alanından okur; bu, **sshpocket tarafında ayrı (Aşama 2) bir değişikliktir**.
- Paket adlandırma (bileşenin `docs/RELEASING.md`'siyle aynı): `linux-hardening-<sürüm>.tar.gz` + `.tar.gz.sig`, arşiv kök dizini
  `linux-hardening-<sürüm>/`, etiket `v<sürüm>`. Doğrulayıcı bu üçünü de bekler.
- Paket: bileşenin GitHub Release varlığı `<id>-<sürüm>.tar.gz` + `.tar.gz.sig` (bugünkü güncelleyiciyle aynı kurallar:
  yalnızca HTTPS, `ssh-keygen -Y verify`, `..`/mutlak yol/symlink reddi, içindeki `component.yml` sürümü etiketle aynı,
  geri sürüme düşmeme).
- Sürümlü klasöre açılır, `current` bağlantısı atomik değişir, **eski sürüm kalır** (geri alma).
- **Her çalıştırmadan önce** dosya özetleri yeniden doğrulanır (kurulumdan sonra değiştirilmiş playbook çalışmaz).
- **Venv bir önbellektir.** Ortam kimliği = `sha256(requirements.txt) + Python sürümü + platform`. Kimlikte venv varsa ve
  sağlık kontrolü geçerse (python çalışıyor, `ansible-playbook --version`) **yeniden kullanılır**; yoksa/bozuksa kurulur.
  Python/gereksinim/makine değişince kimlik değişir, aynı makinede değişmeyince hiçbir şey kurulmaz.
- **Yorumlayıcı seçimi:** `python3`'ü körü körüne almaz. `component.yml`'deki `requires.python` aralığına uyan ilkini arar
  (`python3.14`, `python3.13`, `python3.12`, `python3`). macOS'un `/usr/bin/python3`'ü (3.9) yeni `ansible-core` için eski olabilir.
- **Kurulum:** yalnızca venv'in içine; `pip install --require-hashes --only-binary :all:` (kaynak paket derlemesi yok).
  Önce USB wheelhouse'u (`--no-index --find-links`), yoksa internet; inen paketler isteğe bağlı wheelhouse'a kopyalanır.
- `requirements.txt` hash'lidir (`pip-compile --generate-hashes`, Python 3.14 ile derlenmiş; platform tekerlekleri dahil): `--require-hashes` ile kurulur.

### 6.3b Ön koşul denetimi: `keylab doctor`
`ssh`/`ssh-keygen`/`ssh-copy-id`, uygun Python (sürüm aralığı), `venv`+`ensurepip`, disk alanı, internet; ✅/❌ ve düzeltme önerisiyle.
Hardening/Araçlar ilk açıldığında otomatik çalışır. **Politika:** eksik olanı söyler ve **onay ister**; Homebrew varsa
`brew install python@3.x` önerir/çalıştırır; **Homebrew'u kendisi kurmaz, `sudo` kullanmaz, sisteme global `pip` yapmaz.**
Command Line Tools gerekiyorsa yönlendirir (grafik arayüzlüdür).

### 6.4 Güncelleme akışı
`keylab update`:
1. Çekirdek (bugünkü imzalı akış) + gerekirse geçiş.
2. Yeni `components.lock` okunur; kurulu sürüm kilitle eşleşmiyorsa bileşen imzalı paketten kurulur.
3. İsteğe bağlı **bilgilendirme:** bileşen reposunda kilitten **daha yeni imzalı** bir sürüm varsa "0.2.0 var, değişiklikler
   şunlar; kurulsun mu?" diye sorulur. Onay olmadan kilit sürümü kalır.

### 6.5 Sunucuya çalıştırma (Ansible bileşeni için)
- sshpocket, sunucu ayarlarından **geçici inventory** üretir (`ansible_host/user/port/private_key_file`; izin 600; iş
  bitince silinir) ve bileşeni onunla çalıştırır. Passphrase zaten Keychain→`ssh-agent` ile yüklüdür.
- **sudo parolası** `--ask-become-pass` ile Ansible'ın kendi isteminden alınır: bir çalıştırma boyunca bir kez, diske
  veya komut satırına yazılmaz, Keychain'e kaydedilmez. (Menü oturumu boyunca hatırlama: ayrı risk değerlendirmesi, sonra.)
- `SSH_CONNECTION`: sshpocket, sunucuya bağlanıp kontrol makinesinin sunucudan görünen IP'sini bulur ve
  `hardening_ssh_allow_from` için **öneri** olarak gösterir; kullanıcı onaylar.
- Host key doğrulaması kapatılmaz (`host_key_checking = True`); sunucuya daha önce sshpocket ile bağlanılmış olmalı.

**Entegrasyon notları (linux-hardening `main`, issue #2 ile hizalı; canlı VM'de DOĞRULANMADI):**
- **Çalışma dizini:** `ansible.cfg` (`become = True`, `roles_path = roles`, `inventory`) bileşen klasöründen okunur. sshpocket komutu
  bileşen klasöründen (ya da `ANSIBLE_CONFIG` ile) çalıştırmak zorundadır; aksi halde `become` sessizce kapanır.
- **Grup adı:** `component.yml`'deki `inventory_group` (`hardening`) kullanılır; playbook'lar `hosts: "hardening:ubuntu"` olduğundan eski `ubuntu` de geçer.
- **Anahtar agent'ta olmalı:** uygulama sonrası bağlantı doğrulaması yeni bağlantıyı `BatchMode=yes` + `-i <key> -o IdentitiesOnly=yes` ile açar.
  Passphrase'li key agent'ta yoksa yanlış negatif olur ve gereksiz geri alma tetiklenir; sshpocket çalıştırmadan önce `ssh-add -l` ile doğrular.
- **Port:** `hardening_ssh_port` kaldırıldı, ufw kuralı sshd'nin gerçekten dinlediği porttan yazılır; **port değiştirme desteklenmiyor**.
  sshpocket bu değişkeni arayüze koymaz. Farklı portlu sunucular canlıda doğrulanana kadar temkinli ele alınır (uyarı).
- **Tarama çıktısı:** `hardening_scan_dir` **zorunlu olarak** kullanıcı klasörüne verilir (varsayılan `~/.local/share/sshpocket/scans/`, dizin 0700,
  dosyalar 0600); USB'ye hassas rapor yazılmaz. `summary.json` sshpocket tarafından okunup gösterilir.
- **Kaynak adres:** sshpocket'ın önerdiği `hardening_ssh_allow_from`, preflight'ın doğruladığı adresle aynıdır (sunucunun `SSH_CONNECTION`'da gördüğü);
  NAT/jump host varsa sunucunun *gördüğü* adres yazılmalıdır. Bağlanan kullanıcı `root` ise ve liste boşsa playbook durur.
- **`ansible_python_interpreter`** hedef sunucunun Python'udur; kontrol makinesinin Python'u buraya yazılmaz.
- **`--tags`:** erişim korumaları etiketle atlanmaz (0e düzeltmesi); bu sürüm ve sonrası şart.

### 6.6 Güvenlik kapıları (sshpocket tarafı)
- `mutates: true` eylem, aynı oturumda ilgili **önizleme başarıyla çalışmadan** açılmaz.
- **Arayüzde açıkça söylenecekler (bileşenin davranış notları):**
  - `audit` salt-okunur değildir: hedefe `lynis` paketini kurar (`effects: [installs-package]`).
  - `preview` **tam önizleme değildir**: `--check`'te servis başlatma/handler adımları atlanır ve `ufw` komutları çalışmaz;
    **firewall değişikliği önizlemede görünmez**.
  - `apply` paket kurar **ve** yapılandırmayı değiştirir (iki etki de gösterilir).
  - **Geri alma güvenlik ağı (v0.1.0):** `apply`, ufw/ssh'tan önce önceki durumu yedekler ve sunucuda kalıcı bir zamanlayıcı kurar
    (`hardening_rollback_timeout`, varsayılan 300 sn). Yeni bağlantı doğrulanırsa **disarm** edilir; doğrulanamazsa ya da kontrol
    makinesi kaybolursa/sunucu yeniden başlarsa sunucu **kendini önceki duruma döndürür** (siler değil, geri yükler).
    Menü, `rollback_status`'tan "geri alma bekliyor (kalan süre) / yapıldı / kurulu değil" gösterir.
  - `rollback_disarm` geri almayı **iptal eder** ("değişiklik iyi"); `rollback_now` sunucuyu **hemen önceki duruma döndürür**.
    Her ikisi de açıkça yazılı onay ister.
  - Geri alma **yalnızca ufw ve SSH ayarlarını** kapsar; paketler, sysctl, auditd, fail2ban ve AIDE geri alınmaz.
  - `hardening_rollback_enabled=false` güvenlik ağını **kapatır** (bilinçli kaçış); arayüz bunu uyarıyla sunar, varsayılan değildir.
  - "Gereksiz geri alma" bilinçli bir ödünleşimdir: kontrol makinesi süreden uzun donarsa sunucu geri alır. Kullanıcıya
    "erişim yok, süre dolana kadar bekleyin" mesajı gösterilir.
  - fail2ban `ignoreip`:
    - **0.10.0 (kilitli):** kontrol makinesinin sunucunun gördüğü adresi otomatik eklenir (tüm yönetim ağı muaf değil);
      dinamik IP'den çalışılırsa yapılandırma her seferinde değişir.
    - **0.11.0'dan itibaren (kilitli: 3.6.19):** `127.0.0.1/8 ::1` kodda her zaman eklenir; `hardening_fail2ban_ignoreip`
      varsayılanı boştur. Kontrol makinesi ve `hardening_ssh_allow_from` otomatik eklenmez. Bağlı adres muaf değilse günlükte
      tek satır `NOT (fail2ban): Bağlı olduğun adres (X) fail2ban'dan muaf değil` çıkar (uyarı sayılmaz). `bantime`
      varsayılanı 10 dakika. Yükseltmede eskiden otomatik eklenen adres jail dosyasından düşer: ilk koşu `changed` ve bir
      fail2ban yeniden başlatması gösterir, sonra aynı değişkenlerle `changed=0`.
  - `hardening_ssh_kbdinteractive` (2FA) arayüze konulursa uyarı gerekir: açıkken bağlantı doğrulaması yalnızca SSH
    banner'ını sınar, kimlik doğrulama sınanamaz.
- Uygulamadan önce özet gösterilir (sunucu, profil, bileşen sürümü, `allow_from`, port) ve **onay yazdırılır**.
- Kullanıcı `root` ise hardening reddedilir (erişim kaybı riski). Algılanan OS, seçili profille uyuşmuyorsa durur.
- Profil `planned` ise menüde pasif ("yakında").
- Her uygulamanın kaydı (tarih, sunucu, bileşen+sürüm, sonuç) `servers/<ad>/` altında, **gizli bilgi içermeden** tutulur.

### 6.7 Bağlantı katmanı: pre-connect kancası ve port knocking (İSTEMCİ TARAFI UYGULANDI: 2026-10-12; sunucu tarafı paketi sırada)
**Kararlar:** (a) önce **klasik port knocking** (basit "kapıyı çalma"); SPA/VPN sonraya, (b) knock'u tek bir **pre-connect kancası** yapar,
(c) knock dizisi **gizlidir** (Keychain), (d) knock'lanan sunucuda durum göstergesi **knock göndermez**.

- **Kanca:** sshpocket'ın bir sunucuya bağlanan her yolu (`Bağlan`, `key gönder`, durum, bileşenler) tek bir `pre_connect <sunucu>` adımından
  geçer. Sunucuda `KNOCK` tanımlıysa dizi sırayla gönderilir, kısa bir bekleme yapılır, sonra asıl bağlantı açılır. Böylece knock bir yerde yazılır.
- **Sunucu ayarı (`server.env`, Aşama 1 şemasında yer ayrılır):**
  ```
  KNOCK_PORTS=7000,8000,9000     # sıra önemli; GİZLİ, aşağıya bak
  KNOCK_PROTO=tcp                # tcp | udp
  KNOCK_DELAY_MS=300             # knock'lar arası ve sonrası bekleme
  ```
  Dizi düz metin dosyada tutulmaz: Keychain hesabı `USBMonitor_<AD_BÜYÜK>_Knock`; `server.env` yalnızca `KNOCK=keychain` der.
- **İstemci:** TCP için `nc -z -w1 <ip> <port>` (kapalı porta SYN gider; bağlantı kurulmaz), UDP için `nc -u`. Ek araç gerekmez.
- **Durum göstergesi:** knock'lanan sunucuda port normalde kapalıdır. Durum **knock göndermez** (her menü çiziminde knock göndermek hem gürültülü
  hem sızıntıdır); `🔒` ("knock gerekir") gösterir. Kullanıcı `Bağlan` deyince kanca çalışır.
- **Bileşenler (Ansible):** en temiz yol `ssh`'ın `ProxyCommand`'ı: knock'u gönderip sonra bağlanan bir sarmalayıcı; geçici inventory'de
  `ansible_ssh_common_args: -F <geçici ssh_config>` ile verilir. **Dikkat:** linux-hardening `hardening_verify_access` rolü bu değeri
  boşlukla bölüyor (`.split()`); boşluklu `ProxyCommand`'ı bozar, bu yüzden `-F <yol>` kullanılır (yolda boşluk olmamalı, geçici dizin).
  Knocking uygulanırsa linux-hardening'e issue açılacak (bölme yerine güvenli ayrıştırma).
- **Sunucu tarafı:** `knockd`/benzeri kurulum ve yapılandırma ayrı bir **utility bileşenidir**; geri alma güvenlik ağı bunu **kapsamaz**
  (yalnızca ufw+SSH). Bu yüzden konsol erişimi olan test sunucusunda denenmeden üretime alınmaz.
- **Sınırlar (dürüstçe):** klasik knocking şifreli değildir; ağı dinleyen kaydedip tekrar oynatabilir. Taramalardan saklanma katmanıdır, kimlik
  doğrulama değil. Gerçek güvenlik için SPA (örn. fwknop) veya VPN (WireGuard/Tailscale) değerlendirilir (sonraki aşama).

## 7. Menü
```
<ad>
 1) Bağlan
 2) Public key'i sunucuya gönder
 3) Bilgileri güncelle
 4) Keychain passphrase'i
 5) 🛡️ Hardening      → Denetle / Önizle / Uygula / Tekrar denetle ve karşılaştır
 6) 🧰 Araçlar        → profile uyan bileşen eylemleri (Docker kur, Web sunucusu kur, …)
```
Ana menü: sunucular + Yeni sunucu / İçe aktar / Yönet / Güncelle (bugünkü gibi).

## 8. linux-hardening için istenecek değişiklikler (PR listesi)
Bu depo aktif geliştirme yeri değil; değişiklikler PR ile gelir. **Sıra:** A → 0a → 0b → 0c → B → D → C
(sözleşme en sonda: önceki PR'ların gerçeğini tarif etmeli). Her PR'da `--check --diff` temiz ve ikinci çalıştırmada
`changed=0` aranır; gerçek test için bir Ubuntu 26.04 test VM'i gerekir.

| PR | Değişiklik | Neden |
|---|---|---|
| **A** | `hardening_ssh_allow_from` varsayılanı kaldır, boşsa dur; kontrol makinesi IP'si listede değilse dur; `ansible_user == root` ise dur; `authorized_keys` gerçek home'a göre; SSH/ufw sonrası yeni bağlantı testi + geri alma | `10.0.0.0/8` varsayılanı ve root, erişimi keser |
| **0a** | `hardening_*` varsayılanlarını `inventory/group_vars/all.yml`'den **rol `defaults/main.yml`**'ye taşı | `-i /tmp/x.yml` verilince `group_vars` yüklenmez, değişkenler tanımsız kalır, roller düşer |
| **0b** | `hardening_ssh_port` yalnızca ufw'ye uygulanıyor, sshd'ye değil → ya sshd'ye de uygula (Ubuntu'da `ssh.socket` varsa `Port` yetmez) ya da değişkeni kaldır ve ufw'nin **sshd'nin gerçekten dinlediği portu** (`sshd -T`) kullanmasını sağla | Uyumsuz port = kilitlenme. **sshpocket bu değişkeni arayüze koymaz** |
| **0c** | `--check`, temiz sunucuda `fail2ban` handler'ı yüzünden `failed=1` veriyor → düzelt | "Güvenli önizleme" iddiası doğru olsun |
| **B** | Grup adı: `hosts: "hardening:ubuntu"` (geçişi **kırmadan**); `KbdInteractiveAuthentication` değişkeni, açılırsa `AuthenticationMethods publickey,keyboard-interactive` zorunlu + uyarı (`UsePAM yes` ile parola girişini geri açabilir); `AllowTcpForwarding` **string** (`no\|local\|remote\|yes`, varsayılan `no`; ProxyJump için `local` yeter) | Esneklik, güvenliği sessizce düşürmeden |
| **D** | `audit` özet çıktısı (`summary.json`; rapordaki karşılıkları açıkça eşle) + `hardening_scan_dir` (izin 700; USB'ye değil kullanıcı klasörüne) + `--check`'te `fetch` davranışı (`check_mode: false` ya da atla). `audit.yml` zaten idempotent değil (zaman damgalı klasör); beklenen | `lynis-report.dat` hostname, paket, kullanıcı adı içerir; USB'ye yazılmamalı |
| **C** | VERSION, CHANGELOG, `component.yml` (yukarıdaki şema), **`pip-compile --generate-hashes`** ile hash'li `requirements.txt` (Python 3.14 ile test), imzalı paket üretim notu (`sshpocket-component` ad alanı), profil eşleme betiği (`ubuntu-26.04 ↔ Ubuntu/26.04`) | Sözleşme gerçeği tarif etsin |

> **Güncelleme (2026-10-11):** `v0.1.0` **pre-release** olarak yayımlandı (imzalı paket + imza, SHA-256 `5dd1e185…6714`, etiket `v0.1.0` → `69d8e25`).
> Geri alma güvenlik ağı (PR #20-#24) eklendi ve **tek test VM'inde (Ubuntu 26.04.1)** canlı doğrulandı: doğrulama başarısızlığı,
> kontrol makinesinin ölmesi (`SIGKILL`) ve `armed` iken yeniden başlatma. **Doğrulanmayan:** gerçek ağ düzeyi kesinti, rpi/proxmox
> profilleri, sshpocket ile uçtan uca kurulum/çalıştırma.
>
> **Açık sözleşme eksikleri (linux-hardening issue'ları):** `rollback_status` için makine-okunur çıktı
> ([linux-hardening#28](https://github.com/syhgorath/linux-hardening/issues/28)) ve `vars` için tür/varsayılan/açıklama üst verisi
> ([linux-hardening#29](https://github.com/syhgorath/linux-hardening/issues/29)).
> sshpocket bunları, ancak bu sözleşmeler netleşince ayrıştırır.
>
> **Durum (2026-10-10, tarihî):** A, 0a, 0b, 0c, 0d, 0e, B, D ve C **birleşti** (linux-hardening `main`). Yukarıdaki "neden" sütunu artık geçmiş bilgidir;
> geçerli gerçek `component.yml`/`component.json`'dur.
>
> **Güncelleme (2026-10-17):** `v0.1.0` ve ardından **`v0.2.0`** (paket standardı v1 uyumu: `component.json`, `runner`, `confirm`, `warnings`, `vars` üst verisi,
> `rollback_status` makine-okunur sonuç satırı; ön sürüm) yayımlandı; yukarıdaki açık sözleşme eksikleri
> ([#28](https://github.com/syhgorath/linux-hardening/issues/28), [#29](https://github.com/syhgorath/linux-hardening/issues/29)) kapandı. **Canlı doğrulama yapıldı:**
> Ubuntu 26.04.1 VM'inde `rollback_status` → `preview` → `apply` (ok=65, failed=0; güvenlik ağı armed → yeni bağlantı doğrulandı → disarm). Paket tarafında
> gözlenen küçük konular: linux-hardening [#35](https://github.com/syhgorath/linux-hardening/issues/35), [#36](https://github.com/syhgorath/linux-hardening/issues/36),
> [#37](https://github.com/syhgorath/linux-hardening/issues/37).

## 9. Aşamalar ve kabul ölçütleri

| Aşama | İçerik | Bittiğinde |
|---|---|---|
| 0 | Bu belge | Kararlar kayıtlı |
| 1 | **Veri ayrımı:** `servers/`, genel `server.env`, tek genel çalıştırıcı, geçiş, `Autoload` veri klasörünü yüklemez, güncelleyici basitleşir | Eski kurulum otomatik taşınır; testler: geçiş (deneme/gerçek/idempotent/hata), gizli dosyalar, güncelleyici |
| 2 | **Bileşen yükleyici:** `component.yml`, `components.lock`, imzalı kurulum, sürümlü klasör, hash yeniden doğrulama, `--update` entegrasyonu | Sahte bileşenle uçtan uca testler (imza/replay/arşiv saldırıları dahil) |
| 3 | **Hardening bileşeni:** inventory üretimi, menü, güvenlik kapıları, `SSH_CONNECTION` önerisi | ✅ **Uygulandı (3.6.0):** gerçek Ubuntu 26.04 VM'inde ve Docker LAB'ında `preview` + `apply` denendi |
| 4 | **Araçlar:** ilk bileşen (örn. Docker kur) | Aynı sözleşmeyle ikinci bileşen kodsuz eklenir |

Her aşama ayrı sürüm; geri uyumluluk ve geçiş testleri aşama 1'in bir parçasıdır.

**İlerleme (2026-10-11):** Aşama 1 alt adımları: **1.0 ✅** (`servers/` okuyucu + eski biçim uyumluluğu, `srv_get/srv_set`),
**1.1 ✅** (menü `servers/`'dan kurulur, `server:<ad>` girdileri, yinelenen etiket koruması; eski modüller çalışır),
**1.2 ✅** taşıma (deneme çıktısı → onay → kopyala → bayt bayt doğrula → eski `servers/.migrated-<tarih>/<ad>/`), **1.3 ✅** ekle/içe aktar/yönet yeni biçime yazar,
**1.4 ✅** güncelleyici sadeleşti, `example` → `sistem/templates/`, eski `example` kalıntısı güvenle temizlenir. Sürüm: veri düzeni değiştiği için **3.0.0**.

**İlerleme (2026-10-17):** Aşama 3 parçaları **✅**: üreteçler (değişken doğrulama, geçici inventory, argüman dizisi) → kapılar (root reddi, host key, ssh-agent, profil,
`requires` + 30 dk taze `preview`, `confirm`/`warn_if` onayları) → yürütücü (`comp_exec_action`: geçici dosyalar her çıkışta silinir, `outputs.dir_var` kullanıcı klasörüne zorlanır,
`SSHPOCKET_RESULT` ayrıştırılır) → `--components run <paket> <eylem> <sunucu>` → sunucu menüsünde `🛡️ Paketler`: Hazırlık bloğu, `📝 Değişkenler` (+ `list[cidr]` erişim ağı
editörü: öneri, kapsama, tek-ağ uyarısı), `📜 Geçmiş ve son sonuçlar`. Ayrıca sudo parolası istemi açıklaması ve ssh hata ipuçları. Kullanım ve **erişim ağı tuzağı:**
[hardening-kullanim.md](hardening-kullanim.md). Sürüm: **3.6.0**.

## 10. Test stratejisi
- Çekirdek: mevcut Bats stili; `ansible-playbook` yerine **sahte komut** (üretilen inventory/değişken/argümanlar ve kapılar doğrulanır).
- Geçiş: gerçek `servers/` ve eski `Modules/` düzeniyle, yan etkisiz geçici klasörlerde.
- Bileşen kurulumu: güncelleyicideki saldırı senaryoları bileşen paketleri için de (sahte imza, `..`, symlink, eski sürüm, sürüm uyuşmazlığı).
- Docker e2e `sshd` (`test/e2e/`) bağlantı/anahtar akışları için. Paket akışı için **systemd'li Docker LAB'ı** (`test/e2e/lab/`, Ubuntu 26.04): `rollback_status` → `preview` → `apply`,
  erişim ağı tuzağı ve hata yolları gerçek ssh + ansible + ufw ile denenir. Sınırı: çekirdek paylaşılır ve `auditd` taklit edilir (README'de); **son doğrulama yine gerçek VM'de** yapılır.

## 11. Riskler ve açık noktalar
- **Python/Ansible kurulumu:** karar → `--doctor` denetler, onayla `brew install python@3.x` yapabilir (Homebrew'u kurmaz); venv kurulumu sshpocket'ındır.
- ~~**linux-hardening canlıda hiç doğrulanmadı**~~ → **Çözüldü (2026-10-16/17):** `v0.2.0` imzalı ön sürüm olarak yayımlandı ve gerçek Ubuntu 26.04.1 VM'inde ve Docker LAB'ında
  `preview` + `apply` ile doğrulandı. **Geri alma güvenlik ağı da denendi (Docker LAB'ı, 2026-10-17):** (A) denetleyiciden açılan yeni bağlantı doğrulaması başarısız olunca playbook yedekten geri yükler
  ve durur; (B) denetleyici kaybolunca (ansible SSH reload'unda `kill -9`) sunucu `systemd` zamanlayıcısıyla `hardening_rollback_timeout` sn sonra kendini önceki duruma döndürür
  (SSH drop-in silinir, ufw eski hâline döner). Hâlâ izlenecek: rpi/Proxmox profilleri (`planned`) ve gerçek bir ağ kesintisinde (VPN düşmesi) aynı davranış.
- **Canlı test erişimi:** (Docker LAB'ı çoğu akışı VM'siz dener; gerçek VM son doğrulama içindir.) PR serisi gerçek bir test VM'i ister. Başka bir (otomatik) oturumun VM'e erişimi, VM'in kendi key'i/agent'ı ile
  yapılmalı; `sudo` parolasını yalnızca kullanıcı yazar. Yalnızca test VM'i, önce snapshot.
- **Bilgisayar kaybı senaryosu:** key dosyaları ve bileşenler USB'de kalır, ama **passphrase'ler Keychain'de (Mac'te)**. Parola yöneticisinde de saklanmalı;
  USB tek hata noktasıdır (şifreli yedek önerilir).
- **sudo parolasını menü oturumu boyunca hatırlama:** risk/kolaylık dengesi; başta her eylemde bir kez sorulur.
- **Proxmox/rpi profilleri:** farklı güvenlik duvarı, `ip_forward`, SD kart, küme portları; ayrı rol gerektirir (linux-hardening'in kararı).
- **Üçüncü taraf bileşen** desteği bilinçli olarak yok; yalnızca kilitteki, imzalı bileşenler.
- Windows desteği kapsam dışı.

## 12. Kapsam dışı
Otomatik (kullanıcı onaysız) güncelleme, venv'i USB'de kalıcı tutmak (önbellek olarak Mac'te üretilir), Python'u USB'ye gömmek (ileride isteğe bağlı), sudo parolasını saklama, Ansible'ı sshpocket'a gömmek.
