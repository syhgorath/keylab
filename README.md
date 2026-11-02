# sshpocket

[![ci](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml/badge.svg)](https://github.com/syhgorath/sshpocket/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

🇹🇷 Türkçe · 🇬🇧 [English](README.en.md)

macOS için, USB bellekten (veya herhangi bir klasörden) çalışan, modüler bir SSH bağlantı menüsü.
Her sunucu bir *modüldür*; modül klasörünü eklediğinizde menüde kendiliğinden görünür.

> **Durum: ham / erken aşama.** 🛠️
> Bu, kişisel bir HomeLab aracından çıkan bir proje. [Claude](https://claude.com/claude-code) ile
> elden geçirildi ve ilk hâline göre epey toparlandı, ama hâlâ geliştirilecek çok yeri var:
> pürüzler, eksik testler ve yazılmamış özellikler olabilir. Sorunlar projenin hamlığından
> kaynaklanır; bunun için kimseye kızmayın 😄 — issue ve PR'larla yardım etmeniz çok makbule geçer.
> Yalnızca macOS'ta denendi. Kendi sunucularınızda kullanmadan önce kodu okuyun;
> kullanım sorumluluğu size aittir (bkz. [LICENSE](LICENSE)).

> **Otomatik başlamaz.** macOS, Linux ve modern Windows USB takılınca script çalıştırmaz (güvenlik gereği).
> Elle başlatın: `./start.sh` ya da Finder'da `start.command` dosyasına çift tıklayın.

## Test edilen ortamlar

| Ortam | Durum |
|---|---|
| **Intel Mac (x86_64), macOS 15.7.9, bash 3.2.57, OpenSSH 9.9p2** | ✅ Geliştirildi ve elle denendi |
| **Apple Silicon (arm64, Apple M5 Pro), macOS 27.0.1 ve 27.2, bash 3.2.57, OpenSSH 10.3p1 / 10.5p1** | ✅ Elle denendi: `--doctor`, Keychain'deki passphrase ile `ssh-add`, `--components sync` ve `run` (Ubuntu 26.04.1 VM'inde `audit` → `preview` → `apply` → ikinci `preview`; parolalı sudo-rs dahil; Proxmox VE 9.2.21 VM'inde `apply` ve `audit`; bunlar 27.0.1'de. macOS 27.2'de `--doctor` yeniden denendi; ayrıca 27.2'de elle denendi: modül menüsünden **Terminal penceresi açma**, Finder'da **`start.command` çift tıklama**, `🛡️ Paketler` menüsünün **fzf görünümü**; sürüm 3.6.20 ile USB'den: sunucu türü ve `allow_from` soruları, `apply`, `pve_firewall_*`). ⚠️ Elle denenmedi: `🛡️ Paketler` menüsünün ok tuşlu (fzf'siz) görünümü, gerçek sunucuda knock |
| **Apple Silicon (arm64), macOS 26.6.2, bash 3.2.57, OpenSSH 10.3p1 (GitHub Actions)** | ✅ **Otomatik testler** (Bats + ShellCheck, her PR'da). ⚠️ Elle/gerçek denenmedi: Keychain izin penceresi, Terminal açma, `start.command` çift tıklama |
| Ubuntu 24.04 (yalnızca CI: ShellCheck + Bats testleri) | ✅ otomatik |
| Linux üzerinde kullanım (Keychain/`osascript`/`diskutil` yok) | ❌ Desteklenmiyor |
| Hedef sunucu: Ubuntu 26.04.1 (linux-hardening) | ✅ gerçek VM'de taze kurulum: Lynis 58 → 76, ikinci koşu `changed=0`, parolalı sudo-rs |
| Hedef sunucu: Proxmox VE 9 (Debian 13, bir VM içinde; linux-hardening `proxmox-host` platformu) | ✅ `apply` ve `audit` (Lynis 82–84), `pve_firewall_preview` / `pve_firewall_apply` (geri alma güvenlik ağı kuruldu → yeni ssh bağlantısı ve web arayüzü doğrulandı → kapatıldı; ikinci önizleme `changed=0`), web arayüzü ve CT oluşturma sorunsuz. ⚠️ Kümeli Proxmox, gerçek donanım denenmedi; Raspberry Pi denenmedi |

Apple Silicon'da veya yeni macOS'ta bir sorun görürseniz lütfen issue açın (macOS sürümü, `uname -m` ve `bash --version` çıktısıyla).

## Gereksinimler

Standart bir macOS'ta **ek bir şey kurmanız gerekmez**; araç yalnızca sistemle gelen komutları kullanır:

| Ne | Not |
|---|---|
| macOS 10.15 (Catalina) veya yenisi | İmzalı güncelleme `ssh-keygen -Y verify` ister (OpenSSH ≥ 8.0) |
| `bash` 3.2 (`/bin/bash`), `ssh`, `ssh-keygen`, `ssh-add`, `ssh-copy-id`, `security`, `osascript`, `diskutil`, `curl`, `tar`, `shasum`, `nc` | Hepsi macOS ile gelir; `./start.sh --doctor` eksik olanı söyler (hiçbir şey kurmaz) |

İsteğe bağlı:

| Ne | Ne işe yarar | Kurulum |
|---|---|---|
| **fzf** | Menülerde arama ve açıklama önizleme penceresi. Yoksa yerleşik ok tuşlu menü kullanılır. (0.74.5 ile denendi) | `brew install fzf` |
| **Python ≥ 3.12** | Yalnızca **paketler** (örn. linux-hardening) için; günlük kullanımda gerekmez. macOS'un sistem Python'u (3.9) yetmez | `brew install python@3.13` |

`fzf` Homebrew'da `/opt/homebrew/bin` ya da `/usr/local/bin` altındaysa Finder'dan (`start.command`) açıldığında da bulunur.

## Hızlı başlangıç

```bash
./start.sh                          # menüyü başlat
```

Ana menüden her şey yapılır: sunucuya bağlanma, **➕ Yeni sunucu ekle**,
**📥 ~/.ssh/config'ten içe aktar** ve **🛠️ Sunucuları yönet** (listele/sil/yeniden adlandır).
Aynı işler komut satırından da çalışır:

```bash
bash sistem/ModuleGenerator.sh      # yeni sunucu modülü oluştur
bash sistem/ImportSSHConfig.sh      # ~/.ssh/config'teki sunucuları içe aktar
bash sistem/ModuleManager.sh list   # modülleri listele (remove / rename de var)
```

Generator size IP/kullanıcı/port sorar, isterseniz ed25519 anahtar üretir ve
passphrase'i **macOS Keychain**'e kaydeder.

## Özellikler

- **Modüler:** her sunucu bir klasör; menü kendiliğinden oluşur.
- **Keychain:** passphrase diske yazılmaz.
- **Key'i sunucuya gönderme:** menüden veya generator'dan `ssh-copy-id` (aşağıya bakın).
- **`~/.ssh/config` içe aktarma:** mevcut sunucularınızı tek komutla modüle çevirir. Private key'ler
  **kopyalanmaz**; `IdentityFile` yolu sunucunun `server.env` dosyasına `KEY` olarak yazılır.
- **Mevcut key kullanımı:** `servers/<ad>/server.env` içine `KEY=/yol/key` yazın (örn. `~/.ssh/id_ed25519`).
- **Durum göstergesi:** menüde 🟢 (port açık) / 🔴 (erişilemiyor) / ⚪ (ayar yok) gösterilir; sunucular paralel kontrol edilir.
  **⚙️ Ayarlar**'dan kapatılabilir/süresi değiştirilebilir (`sshpocket.conf`; ortam değişkeni `SSHPOCKET_STATUS=0|1` önceliklidir).
- **Port knocking (istemci tarafı):** sunucu menüsünden **Knock ayarları**; dizi (örn. `7000,8000,9000`) Keychain'de durur, bağlanmadan önce otomatik gönderilir,
  durum göstergesinde 🔒. Sunucu tarafı (knockd) kurulumu bu araçta yoktur; sunucuda yöneticinin tanımlamış olması gerekir. Klasik knocking şifreli değildir.
- **Modül menüsü:** Bağlan · Key'i sunucuya gönder · **Bilgileri güncelle** (IP/kullanıcı/port/key yolu) ·
  **Keychain passphrase'ini güncelle/sil** (kayıttan sonra key'i açarak doğrular). Ayar bozuk veya key eksikken de
  güncelleme seçenekleri kullanılabilir.
- **Modül yönetimi:** `ModuleManager.sh list | remove <ad> | rename <eski> <yeni>`.
- **Aynı sunucuya birden çok kullanıcı/key:** her biri ayrı bir modül olur (örn. `web_root`, `web_deploy`).
- **Menü:** ↑↓ gez · Enter aç · `←`/`q` geri; numara yazmak da çalışır; her seçeneğin kısa **açıklaması** görünür. **fzf** kuruluysa menüler fzf ile
  açılır (arama + açıklama önizleme penceresi). Stil ⚙️ Ayarlar'dan (`UI`: `auto` · `fzf` · `arrows` · `numbers`) ya da `SSHPOCKET_UI=numbers ./start.sh` ile değişir;
  bir terminalde sorun çıkarırsa `numbers` yapın.
- **`./start.sh --doctor`:** ön koşul denetimi (ssh araçları, Python aralığı, venv, disk, ağ); yalnızca okur, hiçbir şey kurmaz.
- **Paketler (altyapı):** imzalı, kilitli (`components.lock`) paketlerin güvenli kurulumu: `./start.sh --components list|sync|check-new|prune`; kurulu paketin eylemi: `./start.sh --components run <paket> <eylem> <sunucu>` (sunucuya bağlanır; önizleme/onay kapıları vardır); eski sürüm klasörlerini temizleme: `./start.sh --components prune [--dry-run] [--yes]` (etkin, bir önceki ve kilitteki sürüm kalır); güncellemeden sonra otomatik sorulur ve **Ayarlar › Eski paket sürümlerini temizle** menüsünden de yapılır; paket yazarları için sözleşme uyum denetimi: `./start.sh --components validate <paket|dizin|arşiv> [--strict]` (statik).
  Kilitte `linux-hardening 0.12.0` vardır. sshpocket 3.6.20 ile gerçek VM'lerde (Ubuntu 26.04.1 ve tek düğümlü Proxmox VE 9) denendi: sunucu türü ve izinli ağ sorusu (sunucu eklerken ve ilk açılışta), boş `hardening_ssh_allow_from` iken ⛔, `apply` ve ikinci `preview` (`changed=0`), Proxmox'ta `hardening_pve_apply`'ın yalnızca o koşuya özel verilmesi (Ubuntu'da verilmemesi), `pve_firewall_*` ve fail2ban ayarları (`ignoreip` boş + yazdıklarınız, `bantime` 10 dk; **etkin ban denenmedi**). (0.9.0'ın alan anahtarları (`hardening_<alan>_enabled`) ve kalıcı kaldırma eylemleri (`kaldir_onizle`, `kaldir`, `kaldir_geri_yukle`) önceki turlarda Ubuntu 26.04 ve Proxmox VE 9 gerçek VM'lerinde denendi; aşağıdaki ayrıntılı sonuçlar 0.8.4 üzerindedir; sshpocket üzerinden Docker LAB'da tüm eylemler denendi; gerçek VM'lerde, Ubuntu 26.04 ve tek düğümlü Proxmox VE 9'da, `audit` → `preview` → `apply` → ikinci `preview` (`changed=0`) döngüsü denendi, Lynis Ubuntu yaklaşık 61→83, Proxmox 84. 0.8.4'te ayrıca doğrulandı: platforma göre menü süzmesi (Ubuntu'da `pve_firewall_*` ve Proxmox değişkenleri gizli), altı sunucu başına anahtar (`false` yazmaz ve önceden yazılanı silmez), Proxmox'ta yeniden başlatma sonrası yasal uyarı bandının korunması, `hardening_pve_web_exclusive` (listede olmayan aynı alt ağ istemcisinde 8006 ve 3128 kapanır), ek güvenlik duvarı portu. **Bilinen sınırlar:** `kernel_modules_disabled` Proxmox'ta bilerek reddedilir; `false` yapmak daha önce uygulanmış değişikliği geri almaz; **etkin fail2ban banı, yetkisiz ağdan güvenlik duvarı reddi, misafir VM/CT'li Proxmox, Docker'lı VM, Raspberry Pi, HA kümesi ve bazı Ubuntu seçenekleri (`apt_upgrade`, gerçek `gcc` ile `restrict_compilers`, `auditd_immutable`, `aide_audit_tools`) denenmedi**). Kullanım ve **erişim ağı tuzağı**: [docs/hardening-kullanim.md](docs/hardening-kullanim.md); standart: [docs/package-standard.md](docs/package-standard.md).
- **Sunucu türü ve izinli ağ:** paket platform listesi bildiriyorsa (linux-hardening 0.12.0: Ubuntu/Debian, Proxmox VE, Raspberry Pi), sunucu eklerken ve eski sunucularda paket menüsü ilk açılırken sunucu türü sorulur ("Şimdi değil" hiçbir şey yazmaz). **`hardening_ssh_allow_from` zorunludur:** boşken `preview`, `apply`, `pve_firewall_*` ssh'a gitmeden ⛔ ile durur (`audit` ve `rollback_*` etkilenmez); öneri sunucunun sizi gördüğü adres ve ağdan gelir, sessizce yazılmaz. Proxmox'ta `apply`, seçili platformun `hardening_pve_apply=true` değerini yalnızca o koşuya özel verir (uyarı sunucu adı onayından önce gösterilir, `.vars` dosyanıza yazılmaz). Ayrıntı: [docs/sunucu-ekleme-platform-secimi.md](docs/sunucu-ekleme-platform-secimi.md), [docs/hardening-kullanim.md](docs/hardening-kullanim.md).
- Otomatik testler (Bats, bash 5 ve macOS bash 3.2) + ShellCheck her PR'da CI'da çalışır.

## Key'i sunucuya gönderme

Modül menüsünden **2) Public key'i sunucuya gönder** seçeneği (veya generator'ın son adımı)
`ssh-copy-id` çalıştırır:

- Key sunucuda zaten yetkiliyse bunu söyler ve bir şey yapmaz.
- Değilse `ssh-copy-id` önce agent'taki/varsayılan key'leri, olmazsa **parolayı** dener.
  İsterseniz giriş için başka bir key dosyası yolu da verebilirsiniz.
- Parola yalnızca `ssh`'ın kendi isteminde girilir; script'e hiç uğramaz.
- Sunucuda `PasswordAuthentication no` yapmak **bilerek otomatik değildir.** Key ile girişin
  çalıştığını gördükten sonra elle yapın, yoksa kendinizi dışarıda bırakabilirsiniz.

## Eski düzenden taşıma (v3 öncesi → `servers/`)

Eski `sistem/Modules/<ad>/` sunucularınız **otomatik** taşınır; dışa/içe aktarma gerekmez.

```bash
./start.sh --migrate --dry-run   # önce ne olacağını gör (hiçbir şey değişmez)
./start.sh --migrate             # onay isteyip taşır
```

Key'ler kopyalanıp doğrulanır, eski klasörler **silinmez** (`servers/.migrated-<tarih>/`). Keychain'deki passphrase'ler aynen geçerlidir.
Doğruladıktan sonra o klasörü (key kopyaları içerir) kendiniz silin.

## Güncelleme

Otomatik değildir; siz başlatırsınız:

```bash
./start.sh --check-update   # yalnızca bakar (yeni sürüm varsa çıkış kodu 10)
./start.sh --update         # indir → imzayı doğrula → notları göster → onaylarsanız kur
```

ya da ana menüden **⬆️ Güncellemeleri kontrol et / güncelle**.

- Terminalde (en az 24 satır × 60 sütun) güncelleme paneli açılır: imza ve arşiv denetiminden sonra üstte sürüm, imza durumu, paket (`components.lock`) farkları ve CHANGELOG'daki **Dikkat** notları; altta sürüm notları, onay ve kurulum. Paket sürümü değişince kurulum sonunda "ne değişti, hangi sunucum etkileniyor" raporu yazılır (`./start.sh --components report [paket [eski-sürüm]]` ile elle de alınır; yalnızca okur).
- Paket yalnızca bu reponun GitHub Release'lerinden, HTTPS ile inen ve **bakımcının imza anahtarıyla imzalı** olan
  dosyadır (`ssh-keygen -Y verify`). Doğrulanamazsa hiçbir şey açılmaz/kurulmaz. İmza anahtarı tanımlı değilse
  güncelleme **kapalıdır**.
- Sunucularınız (`servers/`), ayarlarınız (`sshpocket.conf`) ve key'leriniz korunur. Kurmadan önce kodun yedeği (`.update-backup/`, gizli dosya
  içermez) alınır; hata olursa geri yüklenir.
- Geri sürüme düşmez; `..`, mutlak yol veya symlink içeren paketleri reddeder.
- USB yazma korumalıysa güncelleme yapılamaz (bilerek).
- Not: güncelleyici `v2.2.0` ile gelir; daha eski kopyalar bir kez elle güncellenmelidir.

### İlk indirmeyi doğrulama
`--update` imzayı kendisi doğrular, ama **ilk kopyayı** (Release sayfasından indirdiğiniz arşivi) doğrulayamaz: ona güveniniz GitHub'a ve HTTPS'e dayanır.
İsterseniz arşivi açmadan önce kendiniz doğrulayın (OpenSSH ≥ 8.0, macOS'ta hazır). Anahtarın parmak izi `SHA256:0gjAyj8b7VlmtHIFbwmgAmKjK06sw/a5wdEoE8UECwU`;
bunu bakımcıdan **başka bir yoldan** da teyit etmeniz, depodaki dosyaya körü körüne güvenmekten iyidir.

```bash
# Release'ten sshpocket-X.Y.Z.tar.gz ve sshpocket-X.Y.Z.tar.gz.sig dosyalarını indirdikten sonra:
cat > /tmp/sshpocket_signers <<'EOF'
sshpocket-release namespaces="sshpocket-release" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKVJS3mGlCaOT/l8XfR0TxuKxqLUs6PpVDa9TYqm0fLn
EOF
ssh-keygen -Y verify -f /tmp/sshpocket_signers -I sshpocket-release -n sshpocket-release \
  -s sshpocket-X.Y.Z.tar.gz.sig < sshpocket-X.Y.Z.tar.gz        # "Good ... signature" yazmalı
```

## Güvenlik modeli

- **Passphrase diske yazılmaz.** Keychain'de durur (`USBMonitor_<ÖNEK>_Passphrase`),
  `ssh-add`'e `SSH_ASKPASS` ile doğrudan aktarılır.
- `server.env` yalnızca IP, kullanıcı, port (ve ops. key yolu, knock ayarı) içerir. Gizli bilgi **içermemelidir**; knock dizisi de Keychain'dedir.
- TOTP/2FA gizli anahtarlarını bu klasörde **saklamayın**; telefondaki uygulamada tutun.
- `servers/`, `sshpocket.conf`, `id_ed25519*` ve `usb_logs/` `.gitignore` ile dışarıda tutulur.
- Güncellemeler imzalıdır ve elle başlatılır (yukarıya bakın).
- IP/kullanıcı/port değerleri komut çalıştırılmadan önce doğrulanır (kullanıcı `-` ile başlayamaz).
- Sunucu host key'i ilk bağlantıda size sorulur (otomatik kabul yoktur).
- Özel anahtarınızı USB'de taşıyorsanız USB'yi (APFS şifreli birim) şifreleyin.
- Güvenlik açığı bildirimi için: [SECURITY.md](SECURITY.md).

## Yapı

```
start.sh / start.command   giriş noktaları
sistem/Main.sh             menü döngüsü
sistem/Autoload.sh         eski sistem/Modules/ altını yükler (taşınana kadar)
sistem/ModuleGenerator.sh  yeni modül (+ key üretimi, Keychain, ssh-copy-id)
sistem/ImportSSHConfig.sh  ~/.ssh/config içe aktarma
sistem/ModuleManager.sh    list / remove / rename
sistem/Helpers/            Log, Menu/UI, kayıt, doğrulama, SSH, Keychain, ayarlar, güncelleme, paket (Components*) yardımcıları
components.lock            paket kilit dosyası (imzalı sürümle gelir); kurulu paketler: components/
servers/<ad>/              her sunucu için: server.env, id_ed25519_<ad>   (eski: sistem/Modules/<ad>/ → ./start.sh --migrate)
sistem/templates/          server.env.example şablonu
test/                      Bats testleri (bats test)
```

## Geliştirme

```bash
brew install shellcheck bats-core
shellcheck -x -s bash start.sh start.command sistem/*.sh sistem/Helpers/*.sh
bats test
```

Katkı için [CONTRIBUTING.md](CONTRIBUTING.md), planlananlar için [ROADMAP.md](ROADMAP.md).

## Destek

sshpocket ücretsiz ve MIT lisanslı kalacak. Faydalı bulduysan [GitHub Sponsors](https://github.com/sponsors/syhgorath) üzerinden destek olabilirsin; yıldız, hata bildirimi ve katkı da en az onun kadar değerli.

## Lisans

MIT, bkz. [LICENSE](LICENSE).
