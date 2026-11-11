# linux-hardening: kullanım notları

<p align="center"><img src="../img/hardening.jpg" alt="linux-hardening" width="640"></p>

`linux-hardening` paketini sshpocket üzerinden bir sunucuda çalıştırırken bilinmesi gerekenler. Bu belge deneyimlerden (gerçek bir Ubuntu 26.04 VM'inde
`rollback_status` → `preview` → `apply`) çıkarıldı; paketin kendi belgesi için paketin `README.md`/`HOWTO.md` dosyalarına bakın.

## Akış

| Adım | Komut (ya da menü: sunucu › 🛡️ Paketler) | Sunucuyu değiştirir mi? |
|---|---|---|
| Durum | `./sshpocket components run linux-hardening rollback_status <sunucu>` | Hayır |
| Önizleme | `... run linux-hardening preview <sunucu>` | Hayır (`--check`); ufw ve servis durumunu göstermez |
| Uygulama | `... run linux-hardening apply <sunucu>` | **Evet** (SSH, ufw, paketler...); önce taze bir `preview` ve sunucu adını yazma onayı gerekir |

- `BECOME password:` sunucudaki **sudo parolasıdır**; sudo parola istemiyorsa (NOPASSWD) Enter'a basın.
- `preview` kaydı **30 dakika** geçerlidir; `apply` aynı sunucu ve sürüm için yalnızca bu süre içinde açılır.
- USB dışından denerken: `SSHPOCKET_SERVERS_DIR=/Volumes/<usb>/servers ./sshpocket components run ...`.

## ⚠️ Erişim ağı tuzağı (`hardening_ssh_allow_from`)

`apply`, sunucunun güvenlik duvarına (**ufw**) "SSH (22/tcp) yalnızca şu ağlardan" kuralını yazar ve gelen trafiğin geri kalanını **reddeder (deny)**.
İzin verilen ağlar `servers/<ad>/linux-hardening.vars` dosyasındaki `hardening_ssh_allow_from` (virgülle ayrılmış CIDR listesi) ile verilir; değişkenin
**varsayılanı yoktur** (yanlış bir varsayılan sizi dışarıda bırakabilirdi).

**Tuzak:** `apply`'ı VPN'den (örn. `10.99.0.0/24`) çalıştırıp yalnızca VPN ağını yazarsanız, sunucuya daha sonra **VPN olmadan** (aynı ev/lab ağından bile)
bağlanamazsınız. Belirtiler:

- `ping` ve ARP çalışır (ufw bunları engellemez), ama `ssh` **zaman aşımına** uğrar (`Operation timed out`).
- Sunucu ayakta ve sağlamdır; kural tam olarak istendiği gibi çalışıyordur.

**Önlem:** `apply`'dan önce sunucuya bağlanabileceğiniz **tüm** ağları listeye yazın, örneğin VPN + yerel ağ:

```
hardening_ssh_allow_from=10.99.0.0/24,192.168.1.0/24
```

Listeyi değiştirmek sunucuyu kendiliğinden değiştirmez; yeni değerin işlemesi için `preview` + `apply` yeniden çalıştırılır. Paket, `apply` sırasında
**o anki bağlantının** kaynak adresinin listede olduğunu doğrular (aksi halde durur), ama *sonradan kullanacağınız* ağları bilemez.

**Erişimi kaybettiyseniz:** sunucunun konsolundan (hypervisor/ekran) `sudo ufw allow from <ağ>/24 to any port 22` ya da VM snapshot'ına dönün.
Sonra `.vars` dosyasını düzeltip `apply`'ı yeniden çalıştırın ki dosya ile sunucu tutarlı kalsın.

## Bağlanılamıyorsa: hata ipuçları

Sunucunun işletim sistemi algılanamazsa sshpocket, ssh'ın hatasını sınıflandırıp ne yapılacağını söyler (`↳` satırı):

| ssh hatası | ipucu |
|---|---|
| `Operation/Connection timed out` | Sunucuya ulaşılamıyor: VPN/ağ açık mı? `ufw` bu ağdan SSH'a izin vermiyor olabilir (`hardening_ssh_allow_from`) |
| `No route to host` / `Network is unreachable` | Ağ yolu yok: VPN açık mı, doğru ağda mısınız? |
| `Connection refused` | `sshd` çalışmıyor ya da `server.env` PORT yanlış |
| `REMOTE HOST IDENTIFICATION HAS CHANGED` | Host key **değişmiş**: sunucu yeniden kurulduysa `ssh-keygen -R`; aksi halde araya girilmiş olabilir, bağlanmayın |
| `Host key verification failed` / `No … host key is known` | Önce sshpocket ile bir kez bağlanıp host key'i kabul edin |
| `Permission denied` | Anahtar reddedildi: doğru anahtar mı, `authorized_keys`'te mi, ajanda mı? |
| `Too many authentication failures` | `MaxAuthTries` düşük (hardening 3 yazar); ajandaki fazla anahtarı temizleyin |
| `Connection reset` / `kex_exchange_identification` | Çok başarısız denemeden sonra `fail2ban` engellemiş olabilir; biraz bekleyin |

Hata metni sunucudan gelebileceği için denetim karakterleri temizlenir, 160 karaktere kesilir ve kabuğa verilmez.

## Geri alma güvenlik ağı

`apply` SSH'ı değiştirdikten sonra denetleyiciden **yeni bir SSH bağlantısı** açıp doğrular, başarılıysa güvenlik ağını kapatır. Doğrulama başarısız olursa ya da
bağlantı kaybolursa sunucu `hardening_rollback_timeout` (varsayılan 300 sn) sonra **SSH drop-in'ini ve ufw'yi** önceki duruma döndürür; o sürede erişim
olmayabilir. Paketler, sysctl, auditd, fail2ban ve AIDE geri alınmaz.

**İki durum, Docker LAB'ında gerçek `systemd` ile denendi** (`bats test/e2e/components.bats`; LAB'da süre 30 sn):

| Durum | Ne olur |
|---|---|
| Yeni bağlantı doğrulaması **başarısız** (örn. kural bağlantınızı kesti) | Playbook hemen yedekten geri yükler (`ROLLBACK - restore previous state`) ve durur; sunucu hardening öncesi hâlinde, `apply` kaydı `fail` |
| **Denetleyici kaybolur** (bilgisayar kapanır, VPN düşer; playbook ortada ölür) | Sunucu `armed` kalır; `hardening_rollback_timeout` sn sonra zamanlayıcı SSH drop-in'ini siler ve ufw'yi eski hâline getirir (`armed=no`). Kurtarılana kadar bekleyin, ardından yeniden bağlanın |

Durumu görmek için `rollback_status` eylemi (`installed`, `armed`, `timer`, `remaining_seconds`) ya da menüdeki Hazırlık bloğu.

## Denemek için güvenli laboratuvar (Docker)

Gerçek bir sunucuyu riske atmadan akışı (`preview` → `apply`, erişim ağı tuzağı, hata yolları) denemek için tek kullanımlık, systemd'li bir **Ubuntu 26.04
Docker LAB'ı** vardır: `./test/e2e/lab/up.sh && bats test/e2e/components.bats`. Sınırları (çekirdek paylaşılır, `auditd` taklit edilir) ve kurulum için
[test/e2e/lab/README.md](../test/e2e/lab/README.md). Gerçek bir VM'de son doğrulama yine önerilir.

## Platformlar: Ubuntu, Proxmox VE, Raspberry Pi
- **Profil kapısı (sshpocket):** sunucunun `os-release` `ID-VERSION_ID` değeri paketin `supported` profillerinden biri olmalı. Paket **0.6.0** ile `ubuntu-26.04` ve `debian-13` desteklenir.
  **Proxmox VE 9 ve Raspberry Pi OS da `debian-13` görünür**; sshpocket bunları ayıramaz, kapıyı geçerler. (0.4.0'a kadar `debian-13` desteklenmediği için Proxmox hostu bu kapıda reddediliyordu.)
- **Platform denetimi (paket):** asıl koruma paketin preflight'ındadır. Proxmox VE hostu (`/etc/pve` ya da `pveversion`) **`generic` platformla ASLA sertleştirilmez**; hiçbir bayrak (`hardening_allow_unsupported` dahil)
  bunu atlatmaz ve `--check` (preview) içinde de çalışır. Proxmox hostunda gerçek değişiklik için `hardening_platform=proxmox-host` **ve** `hardening_pve_apply=true` gerekir; yanlış platform seçimi (ör. Ubuntu'da `proxmox-host`) da durur.
- **Platformu ayarlamak:** `hardening_platform` (`generic` | `proxmox-host` | `raspberry-pi`) sunucu başına bir paket değişkenidir: 📝 Değişkenler ekranından ya da `servers/<ad>/linux-hardening.vars` dosyasından (`hardening_platform=proxmox-host`). Paket **0.12.0** ve sshpocket **3.6.20** ile bu değer artık sorulur: sunucu eklerken ve eski sunucularda paket menüsü ilk açılırken bir liste çıkar (Ubuntu/Debian, Proxmox VE hostu, Raspberry Pi; "Bilmiyorum" `generic` yazar, "Şimdi değil" hiçbir şey yazmaz). Elle yazmak da yine geçerlidir; ayrıntı: [Sunucu türü ve zorunlu izinli ağ](#sunucu-türü-ve-zorunlu-izinli-ağ-0120--sshpocket-3620).
- Proxmox üstündeki **sıradan Ubuntu VM'leri** etkilenmez: `generic` platformla normal şekilde sertleştirilir. Proxmox hostunun **güvenlik duvarı** `harden.yml`'den ayrı yönetilir (proxmox-host modunda `ufw` atlanır; yalnızca `harden.yml` ile ağ güvenlik duvarı sertleştirilmez, Lynis `FIRE-4512`). Paket 0.7.0 ile iki eylem sunar:
  - **`pve_firewall_preview`** (salt okunur; **0.8.0'dan itibaren** komut `--check` ile çalışır ve yazma kapısı değişkenle açılamaz. 0.7.0'da `hardening_pve_apply=true` iken kural dosyasını gerçekten yazıyordu, 0.7.x kullanıyorsanız bu eylemi `apply` değişkeniyle çalıştırmayın): yazılacak `cluster.fw` içeriğini gösterir. Kümede, boş/geçersiz `hardening_ssh_allow_from`'da, kontrol makinesini kapsamayan listede ve kullanıcının kendi `cluster.fw` dosyası varsa nedenini söyleyerek durur.
  - **`pve_firewall_apply`** (`type-server-name` onayı, önce `pve_firewall_preview` şart): datacenter güvenlik duvarını GERÇEKTEN yazar ve etkinleştirir (gelen trafik varsayılan DROP; SSH, web arayüzü 8006 ve ping yalnızca `hardening_ssh_allow_from` ağlarından). Önce `enable: 0` yazılır, `pve-firewall compile` ile doğrulanır, zamanlı geri alma ağı kurulur, yeni SSH + 8006 doğrulanamazsa önceki hâle dönülür.
  - Proxmox OLMAYAN hostta ikisi de listelenir ama hiçbir şey yapmadan "yalnızca Proxmox VE hostları içindir" diyerek durur.
  - **`hardening_ssh_allow_from`'a bağlanabileceğin TÜM ağları yaz** (LAN + VPN + ofis). Yanlış liste web arayüzünü ve SSH'ı keser; önce VM/host snapshot'ı al.
- Önce snapshot (Proxmox için host disk imajı); önce `preview`.

## Bilinen küçük pürüzler
- Paket **0.3.0+** ile giderildi: `Could not match supplied host pattern ... ubuntu` uyarısı ve `INJECT_FACTS_AS_VARS` kullanımdan kaldırma uyarıları (linux-hardening#35, #36). 0.6.0 koşusunda ikisi de görünmüyor, ikinci `apply` `changed=0` (LAB).


## Yeni seçenekler (0.8.x) ve gerçek VM notları
Aşağıdakiler (0.8.4 dahil) gerçek VM'lerde (Ubuntu 26.04 ve tek düğümlü Proxmox VE 9, boş sunucular) sınandı; **misafir VM/CT'li Proxmox, Docker'lı VM, Raspberry Pi ve HA kümesinde denenmedi**.

| Seçenek | Platform | Not |
|---|---|---|
| `hardening_kernel_modules_disabled` | Ubuntu | Paket kendi `/etc/modules-load.d/60-hardening-preload.conf` dosyasını yazar (netfilter ve `binfmt_misc` dahil), yeniden başlatma sonrası `ufw` açılır. Kapatınca dosya silinir; canlı kilit yeniden başlatmaya kadar sürer. |
| `hardening_kernel_modules_disabled` | **Proxmox VE** | **Reddedilir** (`apply` ve `preview` hiçbir şey yazmaz): LXC (`veth`), KVM, `bridge`, `softdog` (HA) kilitten sonra yüklenemez. |
| `hardening_postfix_banner` (varsayılan açık) | postfix kuruluysa (Proxmox) | SMTP selamlamasından yazılım adını kaldırır, Lynis `MAIL-8818` kalkar. |
| `hardening_proc_hidepid` | Proxmox ve Ubuntu | `/proc` `hidepid=2` + `proc` grubu; yeniden başlatmadan sonra kalıcı. Root dışı kullanıcı yalnızca kendi süreçlerini görür. |
| `hardening_apt_listbugs` | yalnızca Debian ailesi | Proxmox'ta kurulur; `bugs.debian.org` erişilemese de `apt` bloklanmaz (hata satırları gürültülüdür). Ubuntu'da atlanır, hiçbir şey yazmaz. |
| `hardening_fail2ban_pve_web` | Proxmox | 8006 için jail; yanlış parola sonrası IP banlanır, kapatınca iki dosya silinir. |
| `hardening_pve_pveproxy_allow_from` | Proxmox | `/etc/default/pveproxy` bloğu; kontrol makinesinin adresi listede değilse yazmaz, `[]` blok tek başınaysa dosyayı siler. Kontrol makinesi VPN/LAN arasında değişirse **ikisini de** listeye yazın. |
| `hardening_pve_web_allow_from` | Proxmox | Yalnızca `pve_firewall_*` eylemleriyle `cluster.fw`'ye `[IPSET web]` yazar (ana `apply` ile değil). |
| `hardening_pve_web_exclusive` | Proxmox | **0.8.4'te yeniden tasarlandı ve gerçek VM'de doğrulandı.** `true` iken 8006 yalnızca `hardening_pve_web_allow_from` listesinden açılır: aynı alt ağdaki listede olmayan bir istemci 8006 ve 3128'e bağlanamaz (ölçüldü). SSH, ping ve `hardening_pve_firewall_extra_ports` ayrı bir `admin` kümesinden (yönetim ağları) verilir. Bilinen yan etki: web listesindeki adresler Proxmox'un yerleşik kuralları yüzünden 3128/VNC/SSH'a da güvenlik duvarı düzeyinde erişir (sshd kendi izin listesini ayrıca uygular). Liste boşsa ya da kontrol makinesi listede değilse hiçbir şey yazılmaz. `false`'a dönünce `cluster.fw` yeniden `management` = yönetim ağları olur. **Önce snapshot alın; yanlış liste web arayüzünü keser (SSH açık kalır).** |
| `hardening_pve_firewall_extra_ports` | Proxmox | Yönetim ağlarına ek portlar (`"9100/tcp"`, `"8000:8010/udp"`); `web_exclusive` açıkken `admin` kümesine bağlı yazılır (ölçüldü). |
| `hardening_pve_keep_banner` (varsayılan açık) | Proxmox | `pvebanner.service`'e drop-in ekler; yeniden başlatmadan sonra da `/etc/issue` yasal uyarı metni korunur (gerçek VM'de reboot ile doğrulandı, Lynis `BANN-7126` geri gelmedi). `false` yapınca drop-in silinir. `hardening_legal_banner=false` iken drop-in yazılmaması paketin belgesinde yazar; gerçek VM'de ayrıca denenmedi. |

**Proxmox'ta yeniden başlatma:** `pvebanner.service` her açılışta `/etc/issue` dosyasını yeniden yazar. 0.8.3 ve öncesinde yasal uyarı bandı (Lynis `BANN-7126`) kaybolurdu; 0.8.4'te `hardening_pve_keep_banner` (varsayılan açık) bunu önler. `/etc/issue.net` her zaman korunur.

## Platform süzmesi ve sunucu başına anahtarlar (0.8.4)
- **Platforma göre görünüm:** paket `platform_var: hardening_platform` bildirir. Ubuntu sunucuda 9 Proxmox değişkeni (`[Proxmox]` grubu) ve `pve_firewall_preview` / `pve_firewall_apply` eylemleri menüde **görünmez** (107/116 değişken, 6/8 eylem); Proxmox'ta hepsi görünür. Gizli bir değişkenin dosyada yazılı değeri yine çalıştırmaya iletilir. Platformu belirleyen `hardening_platform` değişkenidir (varsayılan `generic`).
- **Altı sunucu başına anahtar (hepsi varsayılan açık, davranış değişmez):** `hardening_sysctl_hardening`, `hardening_disable_network_protocols`, `hardening_disable_filesystem_modules`, `hardening_unattended_upgrades`, `hardening_legal_banner`, `hardening_journald_persistent`. `false` iken ilgili görevler **atlanır** (temiz Ubuntu'da dosya yazılmaz; ölçüldü).
- **`false` önceden uygulanmış değişikliği geri almaz:** yazılmış dosya ve kurulu paket kalır (örn. `unattended_upgrades` önce açık uygulandıysa `unattended-upgrades` ve `20auto-upgrades` sunucuda durur). Kalıcı olarak kapatmak için dosyayı/paketi elle kaldırın.
- **Değişkenler ekranında gezinme:** 100'den fazla değişken için listenin başında `🔎 Ara` (ad ve açıklamada), `📂 Grup` ve `✏️ Yalnızca değiştirdiklerim` satırları bulunur. `✏️` işaretli satırların değeri bu sunucu için dosyada yazılıdır; diğerleri paket varsayılanını kullanır.

## Parolalı sudo ve Ubuntu 26.04 (sudo-rs)
Ubuntu 26.04'ün varsayılan `sudo`'su `sudo-rs`'tir ve Ansible'ın parolalı `become`'ı onunla `Timeout (12s) waiting for privilege escalation prompt` ile takılır. sshpocket, Ubuntu sunucularda klasik sudo (`/usr/bin/sudo.ws`) varsa Ansible'a onu kullandırır (envantere `ansible_become_exe`), çalıştırma sırasında bunu bir satırla bildirir. NOPASSWD sudo'da fark yoktur. Parolayı yine yalnızca Ansible'ın `BECOME password:` isteminde girersiniz.
Sertleştirme sonrası `passwd` kuralları sıkılaşır (`minlen 14`, `minclass 3`, `dictcheck`, `maxrepeat 3`): parola ≥14 karakter, dört sınıftan (küçük/büyük/rakam/sembol) en az üçü, sözlük kelimesi içermemeli.

## Sudo parola istemi bekleme süresi
`--components run`, Ansible'ın bağlantı/`become` bekleme süresini **30 sn** yapar (`ANSIBLE_TIMEOUT`; Ansible varsayılanı 10 sn, hata mesajında "12s" görünür). Gerçek bir Proxmox VM'inde zaman zaman "Timeout (12s) waiting for privilege escalation prompt" görüldü (nedeni bilinmiyor, VM anlık takılması olabilir; 30 sn ile yapılan 0.8.4 turlarında bir kez bile görülmedi); daha uzun bekleme koşuyu yarıda kesmez. Kendi değerinizi `ANSIBLE_TIMEOUT=60 ./sshpocket components run …` ile verebilirsiniz.

## Sunucu türü ve zorunlu izinli ağ (0.12.0 / sshpocket 3.6.20)
- **Sorulan şeyler:** sunucu türü (`hardening_platform`) ve izinli ağlar (`hardening_ssh_allow_from`). Sorular sunucu eklerken (anahtar adımından sonra) ve `servers/<ad>/linux-hardening.vars` dosyasında platform satırı yoksa paket menüsü ilk açılırken çıkar. Hiçbir değer sessizce yazılmaz; öneriler (sunucunun sizi gördüğü adres `/32`, bu bilgisayarın ağı `/24` uyarılı) "doğru mu?" diye sunulur.
- **`hardening_ssh_allow_from` zorunludur:** boşken `preview`, `apply`, `pve_firewall_preview` ve `pve_firewall_apply` ssh'a gitmeden **⛔ "değeri boş değişken"** ile durur; `audit`, `rollback_*` ve `kaldir*` etkilenmez. Menüde bu eylemlerin yanında ⛔ görünür. "Şimdi değil" derseniz değer yazılmaz ve eylemler her seferinde ⛔ kalır; `📝 Değişkenler`'den ya da `.vars` dosyasından girin.
- **Proxmox'ta `hardening_pve_apply`:** `.vars` dosyanıza yazmanız gerekmez. Platform `proxmox-host` seçiliyse `apply` ve `pve_firewall_apply` bu değişkeni **yalnızca o koşuya özel** `true` verir: önce riskli değer sorusu (`e`), sonra "Seçili platform için bu koşuya özel değişken verilecek (değerler dosyanıza YAZILMAZ)" bildirimi ve uyarılar, en sonda sunucu adı onayı. Dosyada kendi yazdığınız değer (`false` dahil) ezilmez; `audit`, `preview`, `rollback_*` ve Ubuntu/`generic` sunucularda hiç verilmez. Günlüğe `bu koşuya özel: hardening_pve_apply=true` yazılır.
- **Güncellemeden sonra:** paket sürümü değişince (`--update` ya da Paketler eşitlemesi) sonda "ne değişti, hangi sunucum etkileniyor" raporu yazılır; elle: `./sshpocket components report linux-hardening`. Örnek: yeni zorunlu `allow_from` kuralı, değişen varsayılanlar (örn. `hardening_fail2ban_bantime` `1h` → `10m`), hangi sunucuda ⛔ göreceğiniz.
- **fail2ban (0.11.0'dan beri):** `127.0.0.1/8 ::1` her zaman muaf; `hardening_fail2ban_ignoreip` boş başlar ve yazdıklarınızı ekler. Kontrol makinesinin ya da `allow_from`'un adresi otomatik muaf **değildir**: bağlı adres muaf değilse art arda 5 hatalı denemede `bantime` (10 dk) kadar banlanabilirsiniz. Gerçek VM'de ayarlar doğrulandı; **etkin ban denenmedi**.
- **Gerçek VM doğrulaması (sshpocket 3.6.20):** Ubuntu 26.04.1 ve tek düğümlü Proxmox VE 9'da bu akış uçtan uca denendi (sorular, ⛔, `apply`, ikinci `preview` `changed=0`, `pve_firewall_apply` ve geri alma güvenlik ağı). Denenmedi: etkin ban, yetkisiz ağdan güvenlik duvarı reddi, küme/misafirli Proxmox, Raspberry Pi.

## Sunucu saati (snapshot sonrası)
`--components run` başlarken sunucunun saatini (ve saat dilimini, NTP durumunu) salt okunur tek komutla yerel saatle karşılaştırır; fark 60 sn'den büyükse iki saati, farkı ve kopyalanabilir hazır düzeltme komutunu gösterir (engellemez; saati kendisi düzeltmez). VM snapshot'tan döndürüldüğünde saat geride kalır ve apt `Not live until` hatası verir, `audit`/`apply` düşebilir. Sunucuda `sudo chronyc makestep`; olmazsa `sudo systemctl restart chrony; sleep 8; sudo chronyc makestep`.

`linux-hardening` saati kendisi de düzeltebilir: `hardening_time_makestep=true` (varsayılan kapalı) büyük bir kaymada `chronyc makestep` çalıştırır; kaynak seçili değilse `chrony`'yi yeniden başlatır. Saati **sıçratır** (sertifika geçerliliği, `cron` sırası, günlük zaman damgaları etkilenebilir), NTP kaynakları erişilebilir olmalıdır ve `preview`'da (`--check`) çalışmaz.
