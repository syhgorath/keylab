# Docker LAB: gerçek bir Ubuntu 26.04 sunucusuna karşı paket akışı

Yerel, tek kullanımlık bir "sunucu": **systemd'li Ubuntu 26.04** konteyneri (sshd, parolasız sudo, ufw, gerçek apt). `linux-hardening` paketini
`sshpocket → ssh → ansible → ufw/sshd/systemd` zinciriyle uçtan uca dener; gerçek bir sunucuya, VPN'e ya da USB'ye gerek yoktur.

```bash
./test/e2e/lab/up.sh                  # imajı kurar, konteyneri sıfırdan başlatır (~1 dk)
bats test/e2e/components.bats         # rollback_status → ret → preview → GERİ ALMA (A: doğrulama başarısız, B: denetleyici kaybı) → apply → audit → rollback_now / rollback_disarm → erişim ağı tuzağı → hata yolları (~6-8 dk)
./test/e2e/lab/run.sh preview         # tek eylem (BECOME password: otomatik cevaplanır)
./test/e2e/lab/down.sh [--clean]      # kaldırır (--clean: anahtarlar, host key, durum ve imaj da)
```

Gerekenler: `docker` ([Colima](https://github.com/abiosoft/colima) `brew install colima docker` + `colima start`, ya da Docker Desktop), `expect`
(macOS'ta hazır), `~/.cache/sshpocket` altında linux-hardening venv'i (yoksa `E2E_INSTALL_VENV=e` ile ilk çalıştırmada onaylı kurulur; PyPI'dan ~10 MB).

## Ne yalıtılmıştır
Hiçbir şey kullanıcının ayarlarına yazılmaz: ayrı `known_hosts` (`.state/kh`, ssh sarmalayıcısı ile), ayrı sshpocket durum klasörü (`.state/userdata`),
geçici `ssh-agent`, `127.0.0.1:2223` portu. Anahtarlar ve host key `.state/` altındadır (git'e girmez).

## Gerçek sunucudan farklar (LAB'ın sınırları)
- **Çekirdek paylaşılır:** konteyner, Colima/Docker VM'inin çekirdeğini kullanır (`sysctl` ayarları o VM'e yazılır). `--privileged` çalışır; bu yüzden **yalnızca bu LAB için** kullanın.
- **auditd taklit edilir:** konteynerde çekirdek denetim (audit netlink) yoktur; Dockerfile'daki "LAB UYUM KATMANI" `auditd`'yi "başlamış" gösterir ve `augenrules --load`'u boş geçer. Gerçek sunucuda bu fark yoktur. Ayrıca LAB'da `auditctl -l` çıkış kodu 255 döner (yüklü kural okunamaz); paket bunu tolere eder, e2e testleri etkilenmez.
- **İstemci adresi:** sunucu sizi Docker köprüsünden `172.17.0.1` olarak görür; bu yüzden `hardening_ssh_allow_from=172.17.0.0/16` (up.sh yazar). Menünün "sunucunun gördüğü adres" sorgusu bunu gösterir.
- Gerçek bir ağ kesintisi, NAT, VPN ve donanım davranışları test edilemez; yine de bir gerçek VM ile son doğrulama önerilir.

## Antivirüs HTTPS'i yeniden imzalıyorsa
`docker pull` "certificate signed by unknown authority" derse (örn. ESET SSL Filter CA), macOS o sertifikaya güvenir ama Colima VM'i güvenmez:
`./test/e2e/lab/colima-trust-ca.sh "ESET SSL Filter CA"` (parmak izini gösterir, onay ister, yalnızca Colima VM'ine ekler).
