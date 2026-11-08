# sshpocket kabuk → Go taşıma planı (2026-11-07)

> **GÜNCEL DURUM (2026-11-08)** — aşağıdaki kronolojik kayıt bu özetin ayrıntısıdır.
> - Dilimler 0–6 bitti (manifest, trust, runner, servers, cli, report, findings, selfupdate, eject, doctor, settings); `main.go` temalara bölündü.
> - Yayın biçimi: imzalı tek ikili (macOS universal). Bkz. `YAYIN.md`, `scripts/build-release.sh`.
> - Arayüz eylemleri varsayılan olarak Go yürütücüyle çalışır (`SSHPOCKET_SHELL_RUN=1` kabuğa döner).
> - Kabuk motoru ve Bats testleri `referans/kabuk/` altında dondurulmuştur; Go testleri onu şartname olarak kullanır (bulunamazsa atlar).
> - Taşınmayanlar (bilinçli): terminal menüsü (fzf dâhil), `--migrate` ve eski `sistem/Modules` düzeni.
> - Eksikler: durum göstergesinde 🔒 (knock) işareti; panodaki "ssh komutunu kopyala" knock göndermez; `scripts/lock-component.sh`'ın Go karşılığı yok.
> - Denenmedi: arayüzdeki "Keychain parolası" ve "Port knocking" adım düğmeleri; gerçek Release ile `update`; `build-release.sh` imzalama adımı; Gatekeeper/notarization; gerçek ağda `components update/upgrade`; CI'ın yeni düzende ilk koşusu.
> - Sürüm numarası, etiket, imza ve Release bakımcıdadır.

Karar (kullanıcı): kabuk geliştirmesi durdu; sshpocket tam Go olacak, sonra GitHub'daki sshpocket komple yenilenir, sonra diğer repolar.
Kabuk motoru, taşıma bitene dek **referans davranış**tır (silinmez). Paketler (`component.json` + Ansible) DEĞİŞMEZ; sözleşme (`docs/package-standard.md`, `package-index.md`) korunur.

## Kapsam
- **Taşınır (~3500 satır çekirdek):** Components*.sh (Exec, Gates, Index, Install, Venv, Vars, Run, RunCLI, CLI, Report, Onboard, Manifest), Servers.sh, ModuleFiles.sh, ModuleManager.sh, SSHKeyCopy.sh, Knock.sh, Migrate.sh, ImportSSHConfig.sh, Update.sh; Python yardımcıları component_{manifest,validate,run,index,findings,scores}.py.
- **Taşınmaz:** menü/arayüz kabukları (ComponentsMenu 1382, UI 584, Menu, BuiltinMenu, SSHModule/Edit ≈ 2500 satır); yerini GUI alıyor. Terminalde ince bir CLI yeterli (run/update/upgrade/list/validate).
- **Kalır:** Ansible + Python (yalnızca paketleri çalıştırmak için; motorun kendisi artık Python/bash istemez).

## Dilimler (her biri bağımsız, kabuk motoruyla karşılaştırmalı test)
| # | Dilim | Kaynak | Not |
|---|-------|--------|-----|
| 0 | Temel: tek dosyalı main.go → `internal/` paketlerine bölmek | main.go | Diğer dilimlerin oturacağı yapı |
| 1 | Bildirim + değişkenler + doğrulama | component_manifest/run/validate.py, ComponentsManifest/Vars/Run.sh | Saf işlev; en kolay, ilk |
| 2 | Kilit + indeks + kurulum + İMZA | Components.sh, ComponentsIndex/Install.sh, component_index.py | **Güvenlik kritik** (imza, seri, süre, sha256, geri sürüm yok, arşiv denetimi) |
| 3 | Kapılar + yürütme + günlük | ComponentsGates/Exec/RunCLI/Venv.sh | İlerleme satırları, sonuç kaydı, maskeleme, 190 reddi |
| 4 | Sunucular + anahtar + Knock + içe aktarma + göç | Servers/ModuleFiles/ModuleManager/SSHKeyCopy/Knock/Migrate/ImportSSHConfig | server.env biçimi korunur |
| 5 | Bulgular + puanlar + rapor | component_findings/scores.py, ComponentsReport/Onboard | GUI zaten kullanıyor |
| 6 | CLI + yayın | start.sh, Main.sh, Update.sh | Tek ikili; macOS+Linux |

## Doğrulama yöntemi
- 40+ `test/*.bats` dosyası **çalışan şartname**dir. Her dilimde ilgili bats senaryoları Go testine çevrilir; aynı fixture ile kabuk ve Go çıktısı karşılaştırılır (`test/fixtures`).
- İmza/indeks (dilim 2): eski davranışla aynı reddetme nedenleri (biçim, süresi dolmuş=3, gelecekte=4, geri sürüm, kilitte olmayan paket yok sayılır) testle kanıtlanır. İmza doğrulaması için `ssh-keygen -Y verify` mi, saf Go sshsig mi? **Karar bekliyor** (stdlib-only kuralıyla saf Go mümkün, ~150 satır, ama güvenlik kritik).
- Gerçek VM: dilim 3 sonrası utility/kurulum/hardening `preview`→`apply`, snapshot ile.

## Açık kararlar (kullanıcı)
1. ~~İmza: `ssh-keygen` mi, saf Go mu?~~ KARAR (2026-11-07): saf Go; uygulandı.
2. Go modülü sshpocket reposunun içinde (`go/`) mı kalır, ayrı ikili/depo mu? (Öneri: aynı repo, kökte `cmd/sshpocket`.)
3. Yayın: tek ikili + USB akışı (start.command nasıl değişir).

## Durum (2026-11-07)
- **Dilim 0:** KISMEN. `internal/` düzeni kuruldu (`internal/manifest`). `main.go` (tek dosya, ~2400 satır) HENÜZ bölünmedi; motoru çağıran kısımları dilimler bittikçe yeni paketlere taşınacak.
- **Dilim 1:** BİTTİ (kabukla/Python'la karşılaştırmalı testler geçti).
  - BİTTİ + Python ile karşılaştırmalı test: `component_manifest.py` → `manifest.Validate` (19.992 bozulmuş bildirimde 0 fark), `component_run.py` → `Describe`, `Check`, `BuildVars` (+`--set`, `--no-required`), `Platform`, `Env`, `NetAdd/NetPrefix/NetSuggest/NetValid`.
  - GUI artık değişken şemasını (`Describe`) ve tek değer doğrulamasını (`Check`) Python yerine Go paketinden alıyor (`go test ./...` geçiyor).
  - BİTTİ + karşılaştırmalı test: `component_validate.py` → `manifest.ValidatePackage` (dizin ve .tar.gz modu, `--strict`, `--unsigned`; 19 gerçek paket + 4 paketin 29'ar bozulmuş kopyası + sahte arşivler, çıktı satır satır aynı). Fark: Python'un os.walk sırası belirsiz, Go sıralı gezer (tek ihlalli testlerde fark yok; çok ihlalde ileti sırası değişebilir). Arşiv açma hatasında Python'un kendi iç hata metni (bozuk gzip vb.) taklit edilmez.
  - BİTTİ + kabukla karşılaştırmalı test: `ComponentsVars.sh` dosya işlevleri (`VarsGet/Set/Unset/Save/Missing`, `NetSuggestFor`), `ComponentsManifest.sh` erişimcileri (`InfoGet/InfoAll/ActionGet/ActionOnPlatform`), `ComponentsRun.sh` (`WriteInventory`, `BuildArgv`). GUI değişken yazma/silmeyi artık Go'dan yapıyor.
  - ERTELENDİ: `comp_net_local` (macOS route/ifconfig) ve `comp_net_seen_by_server` (ssh) GUI'de kullanılmıyor, taşınmadı; `comp_prepare` kilit/kurulum gerektirdiğinden dilim 2'de.
- **KABUK HATASI (Go düzeltti):** `comp_vars_set`, `.vars` dosyası zaten varken değeri `awk -v` ile yazar; `-v` ters bölü dizilerini yorumlar (`C:\temp\new` → sekme + satır sonu). Değerde `\n` yazarak dosyaya fazladan `ad=değer` satırı sokulabilir. Go bunu olduğu gibi yazar (testle kanıtlı). Kabuk motoru dondurulduğu için kabukta düzeltilmedi.
- **Bilinçli farklar (Go daha katı):** sondaki satır sonu içeren kimlik/değer Python `$` ile kabul edilirdi, Go reddeder; NaN/Infinity ve geçersiz UTF-8 reddedilir; IPv6 bölge kimliği (`%eth0`) ağ tanımında reddedilir; Python'un çöktüğü (traceback) durumlarda (türü metin olmayan değişken, doğrulanmamış bildirim) Go hata iletisi verir.
- **Dilim 2:** BİTTİ (kilit, indeks, İMZA, kurulum) — `internal/trust`, saf Go; kabuk/Python/ssh-keygen ile karşılaştırmalı testler geçti.
  - `sshsig.go`: SSHSIG doğrulayıcı (ed25519, sha256/sha512, ad alanı, `allowed_signers`). Kanıtlar: `ssh-keygen -Y verify` ile aynı sonuç (geçerli ve olumsuz durumlar); 342 bozulmuş imzanın 0'ı kabul edildi; depodaki 32 GERÇEK imzalı sshpocket yayın arşivinin 32'si doğrulandı; GitHub'daki gerçek imzalı indeks HTTPS ile alınıp doğrulandı (seri 2).
  - `lock.go`: components.lock katı ayrıştırma (31 kilit senaryosu kabukla aynı), `VersionCmp`, `VerifyArchive` (özet + kilitteki imzacıyla imza).
  - `index.go`: `ParseIndex` (component_index.py, ~330 bozuk/mutasyonlu indeks Python'la aynı), `Client.Refresh` (indir→imza→şema/süre→seri/geri alma→atomik durum; 14 adımlı senaryo kabukla aynı), `LoadCached`, `Eval`/`Report` (16 karar senaryosu kabukla aynı).
  - `install.go`: `CheckArchive`, `InstallArchive` (31 senaryo kabukla aynı, kurulu klasör ve dosya kipleri dahil), `Install`, `InstallFromIndex`, `VerifyInstalled`, `Prepare` (comp_prepare), HTTPS indirme (yalnızca HTTPS, TLS ≥ 1.2, boyut sınırı).
  - GUI: "Güncellemeler" denetimi VE `upgrade` (kurma) artık Go güven katmanından (kabuk yolu kalktı). Sunucu tarafı: onay (`ack`) şart; indeks yeniden alınıp imzası doğrulanır; karar upgrade/install ve sürüm kullanıcının onayladığıyla aynı olmalı (aksi halde durur); kurulum atomik, iptal edilemez (iptal ucu 409 verir). Kanıt: yerel imzalı sahte sürümle 7 senaryo (başarı, sürüm değişmiş, bozuk arşiv, yanlış ad alanı imzası, imza yok, bozuk indeks imzası, kilitte olmayan paket); GERÇEK veriyle (SSHPOCKET_LIVE=1, geçici ağaç): GitHub'dan utility 0.1.0 kilit sürümü kurulup gerçek indeksten 0.1.2'ye yükseltildi; arayüzde tarayıcıyla Denetle → Yükselt… → onay → çalıştır akışı geçici ağaçta başarılı.
  - GERÇEK HATA yakalandı (canlı denemede): GitHub `git archive` arşivleri `pax_global_header` kaydıyla başlar; Go bunu ayrı öğe saydığı için reddediyordu. Düzeltildi ve kabukla karşılaştırmalı teste eklendi.
  - YAPILMADI: `comp_report_pkg` (yükseltmeden sonra sürüm farkı ve sunucu etkisi raporu; upgrade çıktısında bu not görünür); `comp_cli_*` (update/upgrade CLI metni, onay istemi); Venv (dilim 3); `prune`/`sync`/`check-new` (ComponentsCLI).
  - Bilinçli farklar: `allowed_signers` yalnızca `namespaces=` seçeneğini, tam ilke adını ve ssh-ed25519 anahtarlarını destekler (valid-before, cert-authority, joker ilke, başka anahtar türü satırları YOK SAYILIR); indeks seri numarası int64 ile sınırlı; arşivde ad/tür denetimi önce tüm adları sonra türleri sınar (kabukla aynı sıra).
- **Dilim 3:** BİTTİ — `internal/runner`, kabukla karşılaştırmalı testler (sahte ssh/ssh-add/ansible-playbook/python ile) geçti. GERÇEK sunucuya/VM'e dokunulmadı.
  - Yerel mantık: kapılar (root reddi, profil, `requires`+TTL, zorunlu değişkenler, riskli değer onayı, `apply_vars`, katalog), onay akışı (3 kip), `SSHPOCKET_PROGRESS`/`SSHPOCKET_RESULT` ayrıştırma, sonuç kaydı, `Mask`, kalıcı günlük arşivi, `Record`.
  - ssh yardımcıları (`ssh.go`): `KnownHost`, `Agent`, `DetectProfile`, `DetectSudoWS`, `WarnClockSkew`, `SSHHint` — sahte ssh ile kabukla aynı çıktı.
  - Yürütme (`run.go`): `ExecAction` — Ansible runner (envanter, argv, ortam değişkenleri, tee günlüğü, sudo ipuçları, sinyal kodları) 28 senaryo; script runner (sudo'suz ve sudo'lu yol, SHA-256 doğrulamalı uzak kopya, her çıkışta uzak dosyanın silinmesi) 9 senaryo.
  - Venv (`venv.go`): Python seçimi, `requirements.txt` hash denetimi, ortam kimliği, `VenvEnsure`/`VenvLookup` (sahte python ile 6 senaryo).
  - Birleştirici (`cli.go`): `CLI.Run` = `comp_cli_run`; 25 senaryo (venv istemi e/h/EOF, geçersiz girdiler, ssh algılama hatası + ipucu, ubuntu sudo.ws, eski/yeni sunucu biçimi, değişken dosyası…) çıkış kodu, stdout, stderr ve sunucu/durum/önbellek klasörleri kabukla AYNI.
  - Çalıştırılabilir: `sshpocket-go run -servers D -components D [-state D] <paket> <eylem> <sunucu>`. Arayüz bunu pty altında VARSAYILAN olarak kullanır (2026-11-07; kabuğa dönüş: SSHPOCKET_SHELL_RUN=1).
  - GERÇEK VM DENEMESİ (ubuntutest, 2026-11-07): `utility durum` ve sudo parolalı `kurulum durum` başarılı; sonuçlar ve kayıt biçimi kabukla aynı, parola günlüğe sızmadı, izinler doğru. GERÇEK HATA yakalandı ve düzeltildi: girdi bir dosya tanıtıcısı değilken Go, alt sürece girdiyi bir goroutine ile aktarır ve `cmd.Wait` o goroutine terminalden satır okuyana kadar bekler; Ansible bittiği halde koşu Enter'a basılana kadar kapanmıyordu. Çözüm: girdi `*os.File` ise doğrudan, değilse kendi borumuzla verilir; iki regresyon testi (açık kalan terminal girdisi, dosya olmayan açık girdi). Henüz VM'de YENİDEN DENENMEDİ (düzeltme sonrası). Yazan eylemler (`preview`/`apply`) VM'de denenmedi.
  - YAPILMADI / UYARILAR: (1) `ssh_agent_add_retry` TAŞINDI (internal/runner/agent.go; ikili kendisi SSH_ASKPASS olur, `AskpassMain`; sahte ssh-add/security testleri, bozma testi yakaladı) ama GERÇEK Keychain + parolalı anahtarla DENENMEDİ (kullanıcı çalıştırır). Fark: Keychain açıklama notu Stderr'e gider (kabukta kapıda gizlenirdi). (2) Gerçek VM denemesi (preview/apply) yapılmadı. (3) `printf %q` karşılığı (günlükteki `komut:` satırı) ASCII dışı karakterlerde yerel ayara göre kabuktan farklı görünebilir (yalnız görüntü). (4) Bash'in sinyalle ölen alt süreç için günlüğe yazdığı "Terminated" iş satırı Go'da yok (bilinçli).
- **Dilim 4–6:** başlamadı.
- **Test:** `go test ./...` (≈1 dk; Python'la karşılaştırma ağır, `-short` ile atlanır). Karşılaştırma testleri kabuk motorunun Python dosyalarını repo kökünden okur (`../..`'dan).


## VM: değiştiren apply (2026-11-07)
ubuntutest, utility 0.1.2: timezone Europe/Istanbul→Europe/Berlin, preview→onay→apply `degisen=1 yedek=evet`, durum doğruladı. Onay sorusu görüldü. Bulgu: paket UTC'yi reddediyor (muhtemelen sembolik bağ; doğrulanmadı). Denenmedi: geri_yukle, yanlış sudo parolası, Ctrl-C arşivi, 3a reddi.


## Dilim 4 (başladı, 2026-11-07) — internal/servers
KARAR (kullanıcı): terminal menüleri (UI/Menu/ComponentsMenu/BuiltinMenu, ~2100 satır) TAŞINMAYACAK; yalnız CLI + web arayüzü. Eski düzen (sistem/Modules + --migrate) da taşınmadı (proje dağıtılmadı).
YAPILDI: ValidateTarget/NameValid, EnvGet/EnvSet (server.env), Write (server_write_files), Names, Summary, ListLine, Rename, Remove (yalnız dosya), ManualKeyHelp, KnockPortsValid/KnockSettings/KnockSend/PreConnect, PortOpen/Status. Kabukla karşılaştırmalı testler (ValidateTarget ~1400 birleşim, EnvSet 12x4x8, Write, Rename, Knock...), bozma testleri yakaladı. GUI: createServer ve renameServer artık Go.
FARK: taşan port sayısı kabukta doğrulamayı geçer, Go reddeder.
EK (aynı gün): SSH config içe aktarma (ParseSSHConfig/ModuleNameFromAlias/ImportHosts), internal/settings (Get/Source/Set; UI anahtarı dosya uyumu için duruyor, terminal menüsü yok), etkileşimli KeyCopy (ssh_copy_key) ve RemoveInteractive (cmd_remove) — hepsi kabukla karşılaştırmalı testli, bozma testleri yakaladı. GUI: sunucu silme ve anahtar kopyalama artık `sshpocket-go server-remove` / `keycopy` alt komutlarını pty altında çalıştırır (istem metinleri kabukla aynı). Fark: silme dökümü her dosyayı ayrı satırda yazar (kabuk pty'de sütunlar).
YAPILMADI: Keychain passphrase kaydet/güncelle/doğrula akışı (arayüz zaten macOS pencere/ssh-add kullanıyor; ayrı Go işlevi yok), Doctor, Eject, GUI'nin ayarlar ekranı Go ayarlarını kullanmıyor. Gerçek ağ ve gerçek Keychain ile key copy/silme/Knock DENENMEDİ.


## Dilim 5 — BİTTİ (2026-11-07)
internal/findings (Load, Scores), internal/report (Pkg, All, PrevVersion, VarsDiff, ServerLines), `sshpocket-go report`. Python/kabukla karşılaştırmalı (gerçek durum dosyaları ve gerçek paketlerle de). Bilinçli fark: kabuk raporunda cap/any hatası düzeltildi (bkz. internal/report/report.go). Kalan: Lynis geçmişi (GUI lynis()) hâlâ GUI'nin kendi okuyucusunda; Doctor ve Eject dilim 6'da. GUI findings() artık doğrulamalı findings.Load.



## Dilim 6 — KISMEN (2026-11-07): internal/cli, internal/doctor
YAPILDI: `sshpocket-go components [-root D] list|sync|update|upgrade|check-new|prune|validate|report|run`, `doctor`, `version`. internal/cli kabukla (ComponentsCLI.sh + ComponentsIndex.sh) yan yana test edildi: sahte ağ + gerçek ssh-keygen imzalı indeks/arşivlerle list/sync/update/upgrade (onay y/n/e, bozuk kurulum karantinası, kilitten yeni, yanked...), check-new, prune (kilit/önceki sürüm koruması, --dry-run), validate. Bozma testleri yakaladı. Doctor kabukla karşılaştırıldı.
Bilinçli farklar: doctor curl/tar/sha256/nc/fzf denetimlerini yapmaz (Go ikilisi gerektirmez); list'te indeks önbelleği hiç yoksa sessiz (kabukla aynı), check-new GitHub API JSON'unu gerçek JSON olarak ayrıştırır (kabuk sed ile satır başı "tag_name" ister).
YAPILMADI (yayın biçimi kararına bağlı): Update.sh (çekirdek kendini güncelleme: kabukta dosyaları değiştirir; tek ikilide ikili değiştirme olur), Eject.sh (USB çıkarma + ajandan USB anahtarlarını silme), start.sh/start.command/Main.sh (--auto, menü, --migrate yok), kök/yol bulma (şimdilik -root/SSHPOCKET_ROOT/ikili yanı). Gerçek ağda/gerçek GitHub'da `components update/upgrade` DENENMEDİ (yalnız sahte ağ + önceki canlı indeks testleri).
UYARI (yan etki): bu oturumda `bash start.sh …` komutlarını boruya yönlendirerek çalıştırdım; start.sh stdin tty değilse Terminal.app'te yeni pencere açar → birkaç boş Terminal penceresi açıldı (salt okunur komutlar; zararsız, kapatılabilir). Bir daha start.sh'ı tty olmadan çağırma: `bash sistem/Main.sh` kullan.



## Yayın biçimi KARARI + Dilim 6 tamam (2026-11-07)
KARAR (kullanıcı): imzalı TEK İKİLİ (macOS universal = arm64+amd64 lipo + ad-hoc codesign; arayüz gömülü). Ayrıntı: `go/YAYIN.md`.
YAPILDI: internal/selfupdate (`update [--check]`: imza SSHSIG ns sshpocket-release, anahtar release_signers, ikilinin bildirdiği sürüm etiketle aynı olmalı, geri sürüm yok, atomik rename + .update-backup, sonra yeni ikiliyle components sync), internal/eject (`eject`: USB .pub anahtarlarını ajandan siler + diskutil; kabukla karşılaştırmalı, gizli ad glob farkı yakalandı), `go/scripts/build-release.sh` (derle+lipo+codesign+İMZALA+kendi doğrular; --dry-run; BEN İMZA ANAHTARINI KULLANMADIM), `go/launcher/start.command`, `-X main.buildVersion` ile gömülü sürüm. Gerçek GitHub'a `update --check` (salt okunur) denendi: "Güncelsiniz (3.7.0)".
DENENMEDİ: gerçek Release ile uçtan uca update; build-release.sh'ın imzalama adımı; Gatekeeper/notarization (internetten inen ikili ilk açılışta sağ tık>Aç isteyebilir); universal ikili gerçek bir Intel Mac'te.
KALAN: Dilim 0 (main.go ~2400 satır bölme), eski kabuk dosyalarını repo'dan kaldırma/GitHub yenileme (kullanıcıyla: kabuk referans testleri silinirse karşılaştırma testleri çalışmaz!), sonra diğer repolar. SSHPOCKET_GO_RUN varsayılan yapma kararı (parolalı anahtar+Keychain gerçek denemesi sonrası).


## Dilim 0 BİTTİ (2026-11-07): main.go bölündü
main.go (2385 satır) → main.go (HTTP işleyicileri + main, 651) + data.go, packages.go, draft.go, jobs.go, serverops.go, updates.go, vars.go, catalog.go. MEKANİK taşıma: bildirim kümesi ve (yorum/import hariç) satır çoklu kümesi birebir aynı (md5 kontrolü), testler aynen geçiyor. main() içindeki işleyiciler hâlâ tek büyük işlev (ileride ayrı dosyalara çıkarılabilir).
TAŞIMA ÖZETİ: dilim 0–6 bitti. Kalan = taşıma DEĞİL, geçiş: (1) gerçek denemeler (Keychain+parolalı anahtar, silme/key copy, gerçek Release ile update, 3a reddi, wrong sudo, Ctrl-C), (2) SSHPOCKET_GO_RUN varsayılan, (3) GitHub'daki sshpocket reposunu yenileme (kabuk referans testleri silinirse karşılaştırma testleri çalışmaz → önce karar), (4) diğer repolar (linux-hardening/utility/kurulum/index/şablon), (5) sürüm/etiket/imza/Release sizde.

KARARSIZ TEST ÇÖZÜLDÜ (2026-11-07): runner'daki "bir kez FAIL" (açıklanamamıştı) = TestAnsibleExecMatchesShell'deki bash "Terminated: 15" iş satırı normalleştirme regex'i; bash PID'i 3 haneliyse boşlukla doldurur ("line 153:   130 Terminated") ve regex tek boşluk bekliyordu → ~%30 düşüyordu. Regex `: +[0-9]+ +` yapıldı, 8/8 geçti. Gerçek kod hatası DEĞİL, test artefaktıydı.


## Gerçek denemeler, 2. tur (2026-11-07 ~20:40, ubuntutest, yalnız salt okunur eylemler)
DOĞRULANDI: (a) `apply` önizleme olmadan REDDEDİLİR (çıkış 190, "önce preview…", parola/onay sorulmaz) — eski "3a DENENMEDİ" kapandı; (b) YANLIŞ sudo parolası: Ansible "Incorrect sudo password", yönlendirme satırı, çıkış 2, kendi kendine kapanır, başarısız koşu son başarılı result.preview'u BOZMAZ, günlük 0600; (c) Ctrl-C (parola isteminde basıldı, görev akarken DEĞİL): çıkış 143, günlük arşivi yazıldı ("sonuç: fail — kesildi (sinyal 15)"), yetim ansible yok, geçici sshpocket-run.* dizini kalmadı. VM temizdi (Europe/Istanbul).
DENENMEDİ: Ctrl-C görevler akarken; (d) Mac tarafı: parolalı anahtar + Keychain (ssh_agent_add_retry), keycopy, server-remove, Knock; (e) gerçek Release ile update; (f) `components update/upgrade` gerçek ağda.
Temizlik: 17:36–17:40'tan kalan 3 eski sshpocket-run.* geçici dizin (yalnız inventory+vars.json, 0700; erken stdin-takılması denemelerinden) silindi. Sert öldürülen (SIGKILL) koşu geçici dizin bırakabilir.


## Mac tarafı denemesi: parolalı anahtar + Keychain (2026-11-07 ~21:00) — DOĞRULANDI
`sshpocket-go agent-add <anahtar> <ÖNEK>` (yeni, yalnız bu denemeyi yalıtmak için) ile geçici anahtar (/tmp, sahte önek DENEMEANAHTAR): (1) doğru Keychain parolası → parolasız "ajana eklendi", çıkış 0; (2) yanlış kayıtlı parola → askpass TEK deneme, `[e/H]` sorusu, `e` → açıklama notu + macOS "password data for new item:" (iki kez soruldu), doğru parola → "ajana eklendi", çıkış 0; parola çıktıya/ekrana düşmedi. Böylece ssh_agent_add_retry'ın Go portu GERÇEK Keychain ile kanıtlandı. Ajanda kullanıcının eski bir `deneme` anahtarı var (dokunulmadı). Temizlik komutu kullanıcıya verildi (Keychain kaydı, ajandan geçici anahtar, /tmp klasörü).
DENEMEDİ (Mac tarafı kalan): keycopy (gerçek sunucuya), server-remove (gerçek Keychain silme soruları), Knock. SSHPOCKET_GO_RUN varsayılan yapmanın önündeki engel (parolalı anahtar) kalktı.


## Mac tarafı denemesi 2: keycopy (zaten yetkili yolu) + server-remove (2026-11-07 ~21:20) — DOĞRULANDI
(A) `keycopy` ubuntutest'in mevcut yetkili anahtarıyla: hedef+parmak izi, `y` → "Bu key sunucuda zaten yetkili" çıkış 0, başka soru yok, VM'e yazılmadı. (C) `server-remove` geçici kayıtla (/tmp/srv-deneme, sahte DENEMESRV Keychain kayıtları): dosya dökümü, ad onayı, "✅ Silindi", "✅ Knock dizisi Keychain'den silindi", passphrase sorusu y → "✅ Keychain kaydı silindi", çıkış 0; sonra klasör ve iki Keychain kaydı GERÇEKTEN yok (varlık denetimi, değer okunmadı).
DENEMEDİ: (B) keycopy ile yeni anahtarın VM'e GERÇEKTEN kopyalanması (authorized_keys'e yazar; geri silme gerekir), Knock gönderimi (gerçek knock'lu sunucu yok), yanlış ad onayı/Ctrl-C ile silme iptali.


## Mac+VM denemesi: keycopy ile YENİ anahtarın gerçekten kopyalanması (B) — DOĞRULANDI (2026-11-07 ~21:35)
Geçici sunucu kaydı (/tmp/srv-b/denemeb, parolasız geçici anahtar, yorum denemeb-sshpocket) → `keycopy`: hedef+parmak izi, `y`, "başka key" sorusu (Enter = ajan/parola), ssh-copy-id "Number of key(s) added: 1", "✅ Key gönderildi", çıkış 0; yeni anahtarla şifresiz giriş ÇALIŞTI. GERİ ALMA: eski anahtarla authorized_keys yedeklenip yalnızca denemeb-sshpocket satırı çıkarıldı (kalan 1 satır, silinen 1, 0600); sonra yeni anahtar "Permission denied (publickey)", eski anahtar çalışıyor, VM'deki .bak dosyası ve yerel /tmp/srv-b silindi. VM temiz.
Mac tarafı tamam: parolalı anahtar+Keychain, keycopy (yetkili ve yeni), server-remove. KALAN denemeler: Knock gönderimi (knock'lu sunucu yok), gerçek Release ile update, components update/upgrade gerçek ağda, Ctrl-C görevler akarken. SSHPOCKET_GO_RUN varsayılan yapma önündeki engeller kalktı.


## SSHPOCKET_GO_RUN → VARSAYILAN (2026-11-07)
Arayüz eylem yürütmesi artık VARSAYILAN Go yürütücüsüyle (`sshpocket-go run` pty altında). Kabuğa dönüş: `SSHPOCKET_SHELL_RUN=1` (ya da eski `SSHPOCKET_GO_RUN=0`); yalnız "1" döndürür. Go yolu artık sistem/Main.sh GEREKTİRMEZ (tek ikili düzeni). Birim testi (anahtar kombinasyonları) eklendi; tüm kısa süit yeşil.
DENENMEDİ: arayüzden (tarayıcı → pty → Go yürütücü) uçtan uca bir eylem; yalnız CLI `run` VM'de ve birim testlerle kanıtlı. İlk gerçek kullanımda arayüzden salt okunur `durum` çalıştırıp doğrula.

ARAYÜZDEN UÇTAN UCA DOĞRULANDI (2026-11-07 21:07): tarayıcıdan ubuntutest › utility › durum → Go yürütücü (varsayılan) çalıştı: günlük arşivi `~/.local/share/sshpocket/utility/ubuntutest/logs/20261107-210753-durum.log` (6 sn, çıkış 0, sonuç ok), result.durum yazıldı, yetim ansible yok. SSHPOCKET_GO_RUN varsayılan yapma işi KAPANDI.

KARAR (kullanıcı, 2026-11-07): Knock denemesi SONRAYA bırakıldı (Knock'lu gerçek sunucu kurulunca; Go tarafı yalnız yerel dinleyicilerle test edildi). Ayrıca kalan: gerçek Release ile update, components update/upgrade gerçek ağda, Ctrl-C görevler akarken, GitHub yenileme (kabuk referans testleri silinirse karşılaştırma testleri çalışmaz → önce karar), diğer repolar.


## GitHub yenileme: YEREL dal `go-gecis` hazır (2026-11-07 ~22:30) — PUSH/PR YAPILMADI
Kullanıcı planı onayladı; #187 zaten birleşmişti (main f9ccf02). Dal: `go-gecis` (origin/main'den; DİKKAT: upstream origin/main olarak izliyor → push'ta açık refspec kullan, main'e ASLA doğrudan). Tek commit 9cd606c, 226 dosya. İçerik: go/ eklendi (go/.gitignore: ikili+dist); sistem/, test/, start.*, scripts/, .shellcheckrc → referans/kabuk/ (+ sembolik bağlar: components, components.lock, servers, VERSION, CHANGELOG.md, README*.md, .github, sistem/{index,release}_signers); index_signers+release_signers kökte; .gitignore'a referans/kabuk/test/e2e/... eklendi; CI yenilendi (go işi ubuntu+macos; kabuk işleri referans/kabuk'ta); README.md/README.en.md yeniden yazıldı; CHANGELOG [Unreleased] taslağı (VERSION'a DOKUNULMADI — bakımcı). Go testleri referans/kabuk/sistem yolunu kullanır; GUI test USB'de eski sistem/ düzenini de tanır.
DOĞRULANDI: bats 816/816 yeni düzende, shellcheck temiz, go vet+tüm kısa Go süiti yeşil, uzun karşılaştırmalı testler (19.992 mutasyon) yeni düzende koşuyor.
YEREL DIŞI TUTULDU (kullanıcı karar versin): go/tasarım örnek/ ve go/TASARIM-PROMPT.md (.git/info/exclude).
DÜZELTİLEN HATA: ilk `git add -A` e2e lab .state dosyalarını (özel anahtar+ev dizini yolu) indekse almıştı; push'tan ÖNCE yakalandı, çıkarıldı, ignore kuralı eklendi.
BULGU / EKSİK ÖZELLİKLER (Go'da kitaplık var ama BAĞLI DEĞİL): ~/.ssh/config içe aktarma (servers.ParseSSHConfig/ImportHosts), Knock (servers.PreConnect/KnockSend; GUI'nin "Terminal'de aç"ı knock göndermiyor), internal/settings (hiçbir yerden çağrılmıyor), Keychain passphrase kaydet/güncelle ekranı. README bunu açıkça yazıyor. Bakımcı aracı scripts/lock-component.sh referans/kabuk/scripts altında (Go karşılığı YOK; components.lock güncellemek için hâlâ kabuk).


## Eksikler bağlandı (go-gecis, commit 9ed82b6, yerel; push/PR YOK) — 2026-11-07 ~23:10
EKLENDİ: `sshpocket-go servers [-servers D] list | import-ssh-config [dosya] | knock <ad> | keychain <ad>` ve `settings [get|set]`. Etkileşimli akışlar (internal/cli/servers.go) kabukla yan yana test edildi (ImportSSHConfig.sh, knock_manage, ssh_keychain_manage; sahte security/ssh-add; bozma testleri yakaladı, zayıf testler güçlendirildi: seçim "0", gecikme 5500, ajanda önceden var). GUI: sunucu adımlarına "Keychain parolası" + "Port knocking" (pty görevleri), "Terminal'de aç" bağlanmadan önce knock gönderir (cli.SendKnock, yerel dinleyicilerle test), durum yoklaması sshpocket.conf STATUS/STATUS_TIMEOUT'a uyar (mevcut kullanıcı conf'u okundu: STATUS_TIMEOUT=2). app.js sözdizimi JXA ile doğrulandı (node yok).
DENENMEDİ: bu yeni akışların gerçek Keychain'le elle denemesi (import-ssh-config gerçek ~/.ssh/config'le, knock/keychain etkileşimli), arayüzde yeni iki adım düğmesinin tarayıcıda tıklanması.
KALAN eksik: durum göstergesinde 🔒 işareti; panodaki "ssh komutunu kopyala" knock göndermez; scripts/lock-component.sh'ın Go karşılığı yok (kabukta kalır).

## Knock dizisi girişi (2026-11-07)
`servers knock <ad>`: dizi artık macOS'un parola isteminde değil, sshpocket'in kendi gizli isteminde sorulur (stty -echo), doğrulanır (1-10 port, 1-65535, boşluksuz) ve `security -i` ile stdin'den Keychain'e yazılır (komut satırında görünmez). Kabuk akışından BİLİNÇLİ fark: kabuk diziyi "password data for new item" isteminde sordurup parola sanılmasına yol açıyordu. `servers knock-send <ad>` ayarlı diziyi şimdi gönderir (bağlanmadan önce otomatik gönderilenin aynısı); 3 yerel dinleyiciyle sıra ve 300 ms aralık ölçüldü.
