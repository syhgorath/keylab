# sshpocket paket standardı (v1)

> **Durum: v1 KABUL EDİLDİ (kararlar §13), henüz uygulanmadı.** Amaç, "sshpocket'ın içinde çalışacak bir paket nasıl olmalı?" sorusunu
> **bir kez, yazılı** cevaplamak; yükleyici (Aşama 2) ve menü (Aşama 3) bu belgeye göre yazılır.
> Ayrıntılı gerekçeler ve bileşen modelinin tamamı için bkz. [components-design.md](components-design.md);
> bu belge onun **paket yazarına dönük, normatif** özetidir. Çelişki olursa bu belge (onaylandıktan sonra) kazanır.

## 1. Fikir

Bir **paket**, sshpocket'a şunu söyleyen imzalı bir arşivdir: *"Ben şuyum, şu sunuculara uyarım, şu eylemleri yaparım,
şu değişkenleri alırım, şunlara ihtiyacım var."* sshpocket de pakete şunu söyler: *"Şu sunucuya, şu bağlantı bilgisiyle,
şu onaylardan sonra."* sshpocket paketin **içini bilmez**; yalnızca bildirimi doğrular, kapıları uygular ve eylemi çalıştırır.

Hedef (kabul ölçütü): **ikinci bir paket (örn. "Docker kur") sshpocket kodunda hiçbir değişiklik yapmadan**, yalnızca
kilit dosyasına bir satır eklenerek menüye gelir.

## 2. Terimler

| Terim | Anlamı |
|---|---|
| **Paket** | Bu standarda uyan, imzalı, sürümlü arşiv (bugünkü adıyla "bileşen"). |
| **Bildirim** | Paketin kökündeki `component.json` (makine-okunur kaynak). `component.yml` isteğe bağlı insan-okunur kaynaktır. |
| **Eylem** | Bildirimde tanımlı, menüden tetiklenen tek iş (audit, preview, apply, …). |
| **Kilit dosyası** | sshpocket reposundaki `components.lock`: hangi paket, hangi sürüm, hangi özet, hangi imzacı. |
| **Kontrol makinesi** | sshpocket'ın çalıştığı Mac. |
| **Hedef** | Eylemin uygulandığı sunucu. |

## 3. Paket biçimi

- **Dosya adı:** `<id>-<sürüm>.tar.gz` ve yanında `<id>-<sürüm>.tar.gz.sig`; GitHub Release varlığı, etiket `v<sürüm>`.
- **Arşiv kökü:** tek dizin `<id>-<sürüm>/`; bildirim `<id>-<sürüm>/component.json`.
- **İmza:** `ssh-keygen -Y sign`, ad alanı **`sshpocket-component`** (çekirdek güncellemesi `sshpocket-release`; ad alanları
  bilerek ayrı, bir paket imzası çekirdek güncellemesi olarak yeniden oynatılamaz).
- **Doğrulama (sshpocket yapar):** yalnızca HTTPS; imza `components.lock`'taki `signers` ile; sha256 kilitle aynı; arşivde
  `..`, mutlak yol, symlink/hardlink ve kök dizin dışı içerik yok; `component.json`'daki `id`/`version` dosya adı ve etiketle aynı;
  kurulu sürümden **eski sürüm kurulmaz**.
- **Kurulum yeri:** `components/<id>/<sürüm>/`, etkin sürüm `components/<id>/current` (düz metin dosyası; exFAT'ta symlink yok).
  Eski sürüm kalır. **Her çalıştırmadan önce** dosya özetleri yeniden doğrulanır.
- **İçerik kuralları:** paket, kullanıcı verisi (key, parola, sunucu bilgisi) **içermez**; sshpocket de pakete bunları dosya
  olarak teslim etmez (bkz. §7).

## 4. Bildirim: `component.json`

Alan tablosu (`*` zorunlu):

| Alan | Tür | Açıklama |
|---|---|---|
| `id` * | string `^[a-z][a-z0-9-]*$` | Paket kimliği; kilit dosyası ve klasör adıyla aynı |
| `version` * | semver | Etiket ve dosya adıyla aynı |
| `sshpocket_api` * | tamsayı | Bu belgenin sürümü (şimdi `1`); sshpocket desteklemiyorsa reddeder |
| `name`, `description` | string | Menüde gösterilir |
| `runner` * | `ansible` \| `script` | Eylemin nasıl çalıştırılacağı (§7) |
| `requires.control` | nesne | `python` aralığı, gerekli paketler (hash'li `requirements.txt`) |
| `requires.target` | nesne | Hedefte gereken asgari yazılım (örn. `python >=3.9`) |
| `profiles[]` * | `{id, status}` | `supported` \| `planned`; profil, algılanan OS ile eşleşmezse eylem açılmaz |
| `actions` * | nesne | §5 |
| `vars[]` | nesne listesi | §6 |
| `inventory_group` | string | Yalnızca `ansible` runner: hedefin konacağı grup |
| `invoke.append[]` | nesne listesi | `cmd`'ye eklenecek argümanlar (§7) |

Örnek (kısaltılmış):
```json
{
  "id": "linux-hardening", "version": "0.1.0", "sshpocket_api": 1,
  "name": "Linux sertleştirme", "runner": "ansible",
  "requires": { "control": { "python": ">=3.12" }, "target": { "python": ">=3.9" } },
  "profiles": [ { "id": "ubuntu-26.04", "status": "supported" } ],
  "inventory_group": "hardening",
  "actions": {
    "preview": { "cmd": "ansible-playbook playbooks/harden.yml --check --diff", "effects": [], "requires_sudo": true },
    "apply":   { "cmd": "ansible-playbook playbooks/harden.yml", "effects": ["installs-package", "changes-config"],
                 "requires_sudo": true, "requires": ["preview"] }
  }
}
```

## 5. Eylem modeli

| Alan | Anlamı |
|---|---|
| `cmd` * | Paket köküne göre çalıştırılacak komut; sshpocket kabuğa **dizge olarak vermez**, sözcüklere ayırıp doğrudan çalıştırır |
| `effects` * | `[]` salt okunur · `installs-package` · `changes-config` · `destructive` (birden çok olabilir; menü **hepsini** gösterir) |
| `requires_sudo` | sudo parolası gerekir; Ansible'ın kendi isteminden alınır (diske/komut satırına/Keychain'e yazılmaz). `ansible` runner'da bu eylemde sshpocket `--ask-become-pass` ekler (paketin `invoke.append`'e yazması gerekmez; yazdıysa ikilenmez): NOPASSWD sunucuda istemde Enter yeterli. `requires_sudo` yoksa/false ise istem çıkmaz |
| `requires[]` | Ön koşul eylemler; **zorlamayı sshpocket yapar** (önizleme başarılı olmadan `apply` açılmaz) |
| `confirm` | `none` \| `yes-no` \| `type-server-name` (yıkıcı/geri dönüşü zor işler için sunucu adı yazdırılır) |
| `interactive` | Kullanıcıdan istem bekler; sshpocket gizli/sessiz çalıştırmaz, terminali devreder |
| `outputs` | Çıktı sözleşmesi (§8) |
| `set` | Eylem düzeyi **sabit değişkenler** (nesne, en çok 8): `{"hardening_pve_apply": true}`. Bu eylem çalışırken sshpocket bu değerleri kullanıcının `.vars` değerinin **üstüne yazar**. Yalnızca `vars`'ta tanımlı **bool, int (min/max içinde), enum, string, path** değişkenler ve türüne uygun değerler olabilir; liste olamaz ve `outputs.dir_var` değişkeni ezilemez. Amaç: aynı playbook'un güvenli ve gerçek koşu eylemleri kullanıcı ayarından bağımsız olsun (örn. `*_preview` eylemi `…_apply=false`'u, `*_apply` eylemi `true`'yu zorlar; aksi halde kullanıcının `.vars`'ındaki `true` yüzünden "önizleme" gerçek koşuya dönüşür). `warn_if` yine zorlanan değere uygulanır |
| `warnings[]` | Menüde gösterilecek açık notlar (örn. "önizleme firewall değişikliğini göstermez") |

Kurallar: `effects` boş olmayan eylem onaysız çalışmaz; `destructive` ve `changes-config` en az `yes-no`, geri alınamayanlar
`type-server-name` ister; paket bunu **gevşetemez**, yalnızca sıkılaştırabilir.

## 6. Değişkenler

```json
"vars": [
  { "name": "hardening_ssh_allow_from", "type": "list[cidr]", "required": true,
    "description": "SSH'a izin verilen yönetim ağları" },
  { "name": "hardening_rollback_enabled", "type": "bool", "default": true,
    "warn_if": false, "description": "false = geri alma güvenlik ağı KAPALI" },
  { "name": "hardening_rollback_timeout", "type": "int", "default": 300, "min": 30, "max": 3600 }
]
```
- Türler: `string`, `bool`, `int`, `path`, `cidr`, `list[<tür>]`, `enum` (+ `values`).
- sshpocket değeri **kendisi doğrular** (tür, aralık, `required`); geçersiz değer çalıştırmayı başlatmaz.
- **Gizli değer yoktur:** `secret` türü bu sürümde **yok**. Parola/anahtar paket değişkeni olamaz; sshpocket bunları
  değişken dosyasına yazmaz. **İstisna: parola ÖZETİ** (örn. `grub-mkpasswd-pbkdf2` çıktısı): adı `_hash`, `_digest` veya
  `_checksum` ile biten `string` değişken özet taşıyabilir, **düz parola taşıyamaz** (paket, düz parolayı kabul etmemeli ve
  biçimi doğrulamalı). Özet de sunucu klasöründeki değişken dosyasında düz metin durur (izin 600, USB'deyse USB'nin şifrelenmesi önerilir);
  **zayıf bir parolanın özeti çevrimdışı kırılabilir**, bu yüzden paketin belgesi güçlü parola önermelidir. Doğrulayıcı yalnızca bu
  ek adlara sahip değişkenleri gevşetir; adı `parola/token/anahtar` içeren diğer metin değişkenleri uyarılmaya devam eder.
- `warn_if`: değer buna eşitse arayüz uyarı gösterir ve onay ister. Onay **yalnızca yapılandırmayı değiştiren** eylemlerde (`effects`'te `installs-package` dışında bir şey olan, örn. `apply`) sorulur; salt okunur eylemler (`preview`, durum/rapor) ve yalnızca araç kuran eylemler (`audit`) sormaz, risk onayı asıl değişikliğin yapıldığı eylemde alınır. **Varsayılanı `warn_if`'e eşit olan değişken her çalıştırmada uyarı tetikler**: varsayılandan sapmayı yakalamak için `warn_if`'i varsayılanın TERSİ yapın.
- `required: true`: değişken zorunludur; eksikse **değiştiren eylemler ve başka bir eylemin ön koşulu olan eylemler** (örn. `preview`) çalışmaz. Salt okunur durum eylemleri (örn. `rollback_status`) zorunlu değişken olmadan da çalışır (verilen değerler yine doğrulanır). Bir değişkeni zorunlu yapan paket, bu eylemlerin onu kullandığından emin olmalıdır.
- Değerler kullanıcı klasöründe geçici, izni 600 olan bir değişken dosyasıyla verilir, iş bitince silinir.
- **Platforma göre süzme ve gruplama (isteğe bağlı, geriye uyumlu):** büyük bir paket (örn. 100+ değişken) her sunucuda yalnızca ilgili değişkenleri göstersin diye:
  - Bildirim üst düzeyinde `"platform_var": "hardening_platform"`: platformu seçen değişkenin adı (tanımlı bir `string`/`enum` değişken; değeri sunucunun değişken dosyasından, yoksa bildirilen `default`'tan okunur).
  - Değişkende `"platforms": ["proxmox-host"]`: değişken yalnızca bu platformlarda görünür (1-10 küçük harfli kimlik). Alan yoksa her platformda görünür. Platform belirlenemezse süzme yapılmaz.
  - Eylemde `"platforms": ["proxmox-host"]`: eylem yalnızca bu platformlarda menüde görünür ve `--components run` ile çalışır; diğerlerinde menüden gizlenir, CLI "bu platformda kullanılamaz" diyerek ssh'a hiç gitmeden çıkar (kod 2). Aynı kimlik kuralları (1-10 benzersiz, küçük harfli). Platformlu bir eylemin `requires` ön koşulu, onun platformlarının hepsinde görünmelidir (aksi halde manifest reddedilir). Eski sshpocket alanı yok sayar: eylem her platformda görünür.
  - Değişkende `"group": "Proxmox"` (1-40 karakter): ekranda `[Proxmox]` etiketiyle gösterilir, aynı gruplar yan yana toplanır, grupsuz değişkenler önce gelir.
  - **Eylem düzeyi zorunlu değişkenler (isteğe bağlı):** eylemde `"requires_vars": ["hardening_ssh_allow_from"]` (1-8 tanımlı değişken adı): listedeki değişken değerler dosyasında **boş ya da yoksa** o eylem ssh'a/ansible'a gitmeden reddedilir, mesaj hangi değişkenin boş olduğunu ve hangi dosyaya ne yazılacağını söyler. `required`'dan farkı: yalnızca o eylemleri kapatır (`audit`, `rollback_*` gibi diğerleri çalışır). Alan yoksa davranış değişmez.
  - **Sunucu eklerken platform seçimi (isteğe bağlı):** üst düzeyde `"platform_choices": [{"id","label","description","auto_detect","defaults","apply_vars","apply_warnings"}]` (1-10 kimlik) ve `"onboarding_vars": [...]` (sunucu eklenirken sorulacak, `warn_if` taşımayan değişkenler). `defaults` platforma göre güvenli varsayılanlar (`warn_if`'li değişken yazamaz); `apply_vars` yalnızca `bool`/`enum` değişkenler ve `apply_warnings` ZORUNLU. Doğrulayıcı bu alanları okur ve bozuksa reddeder; bildirim ayrıntısı `platform_choice.<id>.*`, `onboarding_var` satırlarıdır. Seçim ekranı ve `apply_vars` uygulaması sonraki PR'larda; `sshpocket_api` artmaz, eski sshpocket alanları yok sayar.
  - **Sunucu menüsünde kategori (isteğe bağlı, geriye uyumlu):** üst düzeyde `"menu": {"category": "hardening|kurulum|paketler", "title": "...", "order": 0-999}`. `category` SABİT listeden seçilir: `hardening` → "🛡️ Sertleştirme" (10), `kurulum` → "🧰 Kurulum merkezi" (20), `paketler` → "🛡️ Paketler (sertleştirme vb.)" (90, `menu` yoksa varsayılan). Sunucu menüsünde yalnız kurulu paketi olan kategoriler kendi satırıyla görünür; satıra girince yalnız o kategorinin paketleri `order`'a göre (eşitlikte kilit sırası) listelenir. `title` (≤60) ve `order` isteğe bağlıdır. Bilinmeyen kategori ya da alan doğrulamada reddedilir; `sshpocket_api` artmaz, `menu`'yü bilmeyen eski sshpocket alanı yok sayar ve paketi tek "Paketler" satırında gösterir.
  - **Yazılım kataloğu (isteğe bağlı, geriye uyumlu):** üst düzeyde `"item_var": "yazilim"` (+ çoklu mod varsa `"mode_var": "mod"`; ikisi de tanımlı, birbirinden farklı `string`/`enum` değişken) ve `"items": [{"id","label","description","modes","actions","params"}]` (1-30 öğe; `id` küçük harf/rakam/`-`/`_`, benzersiz; `modes` 1-5 kimlik; `actions` tanımlı eylem adları ve her birinin `requires` ön koşulu da listede olmalı; `params` en çok 12 tanımlı değişken adı: yazılımın koşuya özel alabileceği parametreler). `item_var`/`mode_var` enum ise `id` ve `modes` değerleri `values` içinde olmalıdır. Paket paneli açılırken önce yazılım (ve birden çok modu varsa mod) seçtirilir, sonra yalnız o yazılımın eylemleri listelenir; seçim eylem çalışırken **koşuya özel** `item_var=<id>` / `mode_var=<mod>` olarak verilir (`.vars` dosyasına yazılmaz, onaydan önce ve `last-run.log`'da "bu koşuya özel" olarak görünür). Kapı `item_var`, `mode_var` ve **seçili yazılımın `params` listesindeki** değişken adlarını kabul eder (parametre değerinin türü/aralığı/`required` kuralı her zamanki gibi `component_run.py vars --set` ile doğrulanır; başka ad, parametre yazılımsız ya da başka yazılımın parametresi reddedilir); katalog dışı yazılım, yazılımın modu olmayan mod ya da yazılımın eylemi olmayan eylem hiçbir şey çalıştırmaz. Seçili yazılımın **📝 Değişkenler** ekranı (genel form) yalnız o yazılımın `params` alanlarını gösterir; tür, aralık, varsayılan ve açıklama değişken tanımından gelir, değerler normal `.vars` dosyasına yazılır (`params` yoksa tüm değişkenler görünür). Bildirim satırları: `item_var=`, `mode_var=`, `item=<id>|<etiket>`, `item.<id>.description|modes|actions|params`.
  - **Gizlenen değişken silinmez:** dosyada yazılı değeri yine çalıştırmaya iletilir; `required` kuralı da gizli değişkenler için geçerlidir. Ekran, kaç değişkenin gizlendiğini yazar. Eski sshpocket sürümleri bu alanları yok sayar.

## 7. Çalıştırma sözleşmesi

sshpocket pakete **bağlamı** şöyle verir:

| Runner | Bağlam |
|---|---|
| `ansible` | Geçici inventory (`inventory_group`; `ansible_host/user/port/private_key_file`; izin 600, iş bitince silinir), `-e @<değişken dosyası>`, çalışma dizini = paket kökü (`ansible.cfg` oradan okunur) |
| `script` | Betik **hedef sunucuda** çalışır (kontrol makinesinde değil). Paketin `scripts/<ad>.sh` dosyası, başına `export` satırları eklenerek sunucuya akıtılır ve `bash -s -- <cmd argümanları>` ile çalışır. Betiğe geçen ortam: `SSHPOCKET_SERVER` (ad), `SSHPOCKET_ACTION`, `SSHPOCKET_PROFILE` ve her değişken için `SSHPOCKET_VAR_<AD>` (bool = `true`/`false`, liste = virgülle birleşik; değerler tek tırnakla kaçırılır, hiçbir zaman kabukta çözümlenmez). Bağlantı bilgisi (`HOST`, `KEY` vb.) **verilmez**: betik sunucuya bağlanmaz, zaten oradadır. |

**`script` runner ayrıntıları**
- `cmd` = `scripts/<ad>.sh [argümanlar]`: yol paket içinde, `..`/mutlak yol/sembolik bağ yok, en çok 256 KB; argümanlar düz sözcük (en çok 8). `invoke`, `inventory_group` ve `outputs.dir_var` kullanılamaz (betik sunucuda çalışır). Python ortamı (venv) gerekmez.
- **Sudo gerekmiyorsa** (`requires_sudo` yok): betik ssh stdin'inden akar; betik stdin'i okuyamaz.
- **`requires_sudo: true`**: betik sunucuda 0600 geçici dosyaya (`/tmp/sshpocket-XXXXXXXX`) yazılır; `ssh -t` ile tek sudo oturumunda önce **root'a ait 0700 klasöre kopyalanır, kopyanın SHA-256'sı yerelde hesaplanan özetle karşılaştırılır** (uyuşmazsa 97 ile durur, betik çalışmaz) ve ancak sonra çalıştırılır. Böylece yükleme ile sudo arasında dosya değiştirilse bile root yalnızca imzalı paketteki betiği çalıştırır. Sudo parolası **aynı terminalde, sudo isteminde** girilir (sshpocket'e/günlüğe yazılmaz); geçici dosya ve root klasörü her çıkışta silinir. `/tmp` `noexec` olsa bile çalışır (`bash <dosya>`). Ubuntu'da `sudo.ws` varsa o kullanılır.
- **Sonuç**: `outputs.format: keyvalue-line` ise betik tek satır yazar: `SSHPOCKET_RESULT <id>.<eylem> anahtar=değer …` (ansible ile aynı sözleşme ve sınırlar).
- Önizleme: betik `--dry-run` gibi bir argümanı destekliyorsa iki eylem tanımlanır (`*_onizle`: `effects: []`, gerçek eylem `requires` ile ona bağlanır). Desteklemiyorsa değiştiren eylem `confirm: yes-no` (en az) ister.
- Kapılar ansible ile **aynıdır** (host key, ajan, profil, root reddi, taze önkoşul, onay, `warn_if`); biri durdurursa SSH'a hiç gidilmez.

Her iki runner için ortak kurallar:
- **Parola/passphrase asla verilmez.** Passphrase Keychain→`ssh-agent`'tadır; sudo parolası yalnızca `interactive` istemle girilir.
- Host key doğrulaması **kapatılmaz**; sunucuya daha önce sshpocket ile bağlanılmış olmalı.
- Anahtarın agent'ta olduğu çalıştırmadan önce doğrulanır (`ssh-add -l`).
- Root kullanıcıyla mutating eylem reddedilir; algılanan OS, seçili profille uyuşmuyorsa durulur.
- Çıkış kodu: `0` başarı; sıfırdan farklı = başarısız. sshpocket çıktıyı kullanıcıya olduğu gibi gösterir.

## 8. Çıktı ve veri konumu

- **Hassas çıktı USB'ye yazılmaz.** Çıktı klasörü kullanıcı klasörüdür (`~/.local/share/sshpocket/<id>/<sunucu>/`, dizin 0700,
  dosya 0600); paket bunu `outputs.dir_var` ile alır.
- **Makine-okunur sonuç satırı** (log'dan bağımsız, Ansible çıktı biçimine bağlı olmayan tek satır):
  `SSHPOCKET_RESULT <id>.<eylem> key=value key=value …` — anahtarlar bildirimde `outputs.keys` ile **bildirilir**.
- **İlerleme satırı** (isteğe bağlı; Ansible callback eklentisi üretir): alan başına başında ve sonunda tek satır
  `SSHPOCKET_PROGRESS alan=<ad> sira=<n>/<toplam> durum=basladi|bitti [degisen=<n> atlanan=<n> kesin=0|1 uyari=<n> bekle_tahmin=<sn>]`.
  `toplam` yalnız açık alanları sayar ve koşu boyunca sabittir; önizlemede `degisen` "uygulanırsa değişecek" demektir, `kesin=0` sayının
  tahmini olduğunu gösterir. Satır başı/tırnak varsayılmaz, bilinmeyen anahtarlar yok sayılır, satırda `SSHPOCKET_RESULT` geçmez.
  Satır yoksa "ilerleme bilgisi yok" sayılır. sshpocket sonuç panelinde alan tablosunu gösterir (`bitti` olmayan alan ⏳).
- `outputs.findings`: eylemin yazdığı bulgu listesi, **düz dosya adı** (`bulgular.tsv`; `/`, `..` yok, `.tsv`, ≤ 64 karakter). Dosya `summary.json`
  gibi aranır: `<kullanıcı durum klasörü>/<paket>/<sunucu>/` altında derinlik ≤ 5, birden çoksa en yeni. Biçimi aşağıda ("Bulgular").
- `outputs.summary`: eylemin yazdığı özet dosyası (JSON); sshpocket okuyup gösterir.

### Bulgular (`bulgular.tsv`)
Denetim eylemi (Lynis, …) `outputs.findings` ile bildirdiği dosyayı yazarsa sshpocket bulguları sunucu menüsünde listeler (🔎 Bulgular).
UTF-8, sekmeyle ayrılmış, ilk satır başlık, dosya 0600 (kullanıcı klasöründe; USB'ye yazılmaz). Başlığın **ilk 10 sütunu bu sırayla** olmalıdır
(sonda fazladan sütun yok sayılır); aksi halde dosya "bozuk" sayılıp gösterilmez, eylem sonucu etkilenmez.

| sütun | değer |
|---|---|
| `kimlik` | araç test kimliği (`SSH-7408`); kimliksiz madde `GENEL-<n>`. Tek başına benzersiz değildir |
| `risk` | `yuksek` `orta` `dusuk` `bilgi` `bilinmiyor` (önem verilmeyen bulgu `bilinmiyor`, sıralamada en altta) |
| `tur` | `uyari` ya da `oneri` |
| `aciklama` | tek satır düz metin |
| `ayrinti` | aynı kimlikli satırları ayıran ayrıntı (`MaxSessions (set 4 to 2)`); yoksa boş |
| `oneri` | tek satır düz metin; yoksa boş |
| `alan` | paketin sabit alan kimliği (`ssh`, `base`, …) ya da boş |
| `deger` | `ad=değer` (bu bulguyu kapatan paket değişkeni) ya da boş |
| `kapanir_varsayilan` | `evet` (paket varsayılanıyla kapanır) · `hayir` (değişken elle ayarlanmalı) · boş; yalnız `alan` doluyken anlamlı |
| `platform` | `ubuntu` `proxmox` `hepsi` |

Geçersiz satır atlanır ve sayılır ("N satır atlandı"); en çok 2000 satır gösterilir. Dosya bir denetim koşusunu içerir (her koşu baştan yazar);
yalnız başlık satırı "bulgu yok" demektir. Örnek: `test/fixtures/bulgular-ornek.tsv`.

### Denetim özeti (`summary.json`) ve menüdeki puan
Bir denetim eylemi (Lynis, OpenSCAP, …) çıktı klasöründe `summary.json` yazarsa, sunucu menüsünde (🛡️ Paketler › paket) **araç başına son puan ve bir öncekiyle fark** gösterilir: `📊 Lynis: 83 (2 gün önce) · önceki 61 (+22)`. Gösterim araca özgü değildir; ortak biçim:

```json
{"tool": "OpenSCAP", "date": "2026-10-25T08:44:34Z", "score": 91.5}
```

| Alan | Kural |
|---|---|
| `score` | sayı, 0..1000 (yüzde ya da 0-100 ölçeği); sayı olmayan/sınır dışı kayıt yok sayılır |
| `tool` | `[A-Za-z0-9 ._-]{1,20}`; yoksa `hardening_index` varsa "Lynis", aksi halde "denetim". Araçlar birbirinin "önceki"si sayılmaz |
| `date` | UTC `YYYY-MM-DDTHH:MM:SSZ`; yoksa dosyanın değişiklik zamanı |

Geriye dönük uyum: `score` yoksa `hardening_index` okunur (linux-hardening). Dosyalar `~/.local/share/sshpocket/<paket>/<sunucu>/` altında aranır (derinlik ≤ 5); sunucudan gelen veriye güvenilmez, bozuk dosyalar sessizce atlanır. Başka bir araç eklemek için sshpocket'te kod değişikliği gerekmez, yalnızca bu biçimde `summary.json` yazmak yeter.
- Her uygulamanın kaydı (tarih, sunucu, paket+sürüm, sonuç) `servers/<ad>/` altında, **gizli bilgi içermeden** tutulur.

### 8.x Eylem günlükleri (kalıcı)
Çalışan her eylem (kapılar geçip gerçekten başladıysa) `~/.local/share/sshpocket/<paket>/<sunucu>/logs/<YYYYmmdd-HHMMSS>-<eylem>.log` olarak arşivlenir (dizin 0700, dosya 0600). Başlık: zaman (yerel+UTC), paket/sürüm, eylem, sunucu, komut satırı, süre; sonda çıkış kodu ve `ok`/`fail`. Gövde, parola/token/özel anahtar izleri maskelenerek yazılır (`****`); paket yine de gizli değeri çıktıya yazmamalıdır (maskeleme son savunmadır). `last-run.log` canlı günlük olarak kalır (her eylemde ezilir); `results` satır biçimi değişmedi. Başarısız eylemin günlük yolu `last-fail-log` dosyasında tutulur ve eylem sonunda ekrana yazılır (sonraki başarılı eylem bu işareti siler, günlük silinmez). Sunucu/paket başına son 50 günlük saklanır (`COMP_LOG_KEEP`). Menüde "📜 Geçmiş ve son sonuçlar" günlükleri listeler ve seçileni tam açar (`less`).

## 9. Güvenlik kapıları (paketten bağımsız, sshpocket zorunlu tutar)

1. Kilit dosyasında olmayan paket **çalıştırılmaz** (üçüncü taraf paket yok).
2. Eylem, ön koşul eylemi (`requires`) aynı oturumda başarıyla bitmeden açılmaz.
3. Her çalıştırmadan önce paket dosya özetleri yeniden doğrulanır.
4. Uygulamadan önce özet gösterilir (sunucu, profil, paket sürümü, değişkenler) ve `confirm` kuralı uygulanır.
5. Paket, bildirimde bildirmediği bir eylemi menüye ekleyemez; bildirilen `effects` gerçek davranışın **alt kümesi olamaz**
   (paket sahibinin sorumluluğu, kod incelemesiyle denetlenir).
6. Yeni sürüm bildirimi **bilgilendirir**; onay olmadan kilitteki sürüm kalır.

## 10. Sürümleme ve uyumluluk

- Paket sürümü semver. `sshpocket_api` tamsayıdır: **geriye uyumsuz** değişiklikte artar; sshpocket yalnızca bildiği aralığı
  çalıştırır, diğerlerini "bu paket daha yeni bir sshpocket gerektirir" diye reddeder.
- Yeni **isteğe bağlı** alanlar `sshpocket_api`'yi artırmaz; bilinmeyen alanlar yok sayılır, bilinmeyen **zorunlu** davranışlar
  `required_features[]` ile bildirilir ve sshpocket tanımıyorsa reddeder.

## 11. linux-hardening ile uyum (bugünkü durum)

| Standart gereği | linux-hardening v0.1.0 | Yapılacak |
|---|---|---|
| Paket adı, etiket, `.sig`, ad alanı | ✅ | – |
| Arşiv kökü `<id>-<sürüm>/` | ✅ | – |
| `component.json` (makine-okunur) | ❌ yalnızca `component.yml` | **Issue:** paket üretimine YAML'dan türetilmiş `component.json` eklensin (imzalı arşivde) |
| `runner: ansible` alanı | ❌ (örtük) | Issue'ya dahil |
| `effects`, `requires_sudo`, `requires`, `invoke`, `inventory_group` | ✅ | – |
| `confirm`, `warnings[]` | ❌ | Issue'ya dahil (şimdilik sshpocket varsayılanları: yıkıcılarda sunucu adı yazdırma) |
| `vars[]` tür/varsayılan/uyarı | ❌ ad listesi | Mevcut issue [#29](https://github.com/syhgorath/linux-hardening/issues/29) |
| `rollback_status` makine-okunur sonuç satırı | ❌ | Mevcut issue [#28](https://github.com/syhgorath/linux-hardening/issues/28); biçim bu belgenin §8'ine uyarlanmalı |
| Hassas çıktı kullanıcı klasörüne | ✅ (`hardening_scan_dir`) | – |

> Kural: linux-hardening reposunda kod/PR yazılmaz; yukarıdaki farklar **issue** olarak açılır.

## 12. Yeni paket yazma kontrol listesi

- [ ] `component.json` (`sshpocket_api: 1`), her eylem için `effects` ve `requires_sudo` eksiksiz
- [ ] Mutating eylemin salt okunur bir `preview` ön koşulu var; önizlemenin **göstermediği** şeyler `warnings` içinde yazılı
- [ ] Hiçbir değişken parola/anahtar taşımıyor; hassas çıktı `outputs.dir_var` ile kullanıcı klasörüne gidiyor
- [ ] `requirements.txt` hash'li (`pip-compile --generate-hashes`), `--only-binary :all:` ile kurulabiliyor
- [ ] Sürüm üçlüsü tutarlı: `VERSION`, `component.json`, etiket; arşiv kökü `<id>-<sürüm>/`
- [ ] `ssh-keygen -Y sign -n sshpocket-component` ile imzalı; `.sig` yanında
- [ ] Salt okunur olmayan eylemin geri alma/kurtarma yolu belgelenmiş (kapsamı dürüstçe yazılı)
- [ ] sshpocket reposuna **tek satır** (kilit dosyası) ekleyen PR açıldı; başka kod değişikliği gerekmedi

**Otomatik denetim:** bu listenin mekanik kısmını `./start.sh --components validate <kurulu-paket | dizin | <id>-<sürüm>.tar.gz> [--strict]` denetler
(statik: ansible/ssh/ağ çalıştırmaz, paketten hiçbir şey çalıştırmaz; arşiv yalnızca güvenliyse açılır). Çıktı `--doctor` gibi ✅ / ⚠️ / ❌; ❌ varsa çıkış kodu 1,
`--strict` ile uyarılar da hata sayılır; `--unsigned` imzalamadan önceki denetimde `.sig` uyarısını atlar. Paket reposunun CI'ı için `sistem/Helpers/component_validate.py` ve `component_manifest.py` kopyalanıp
`python3 -I component_validate.py <dizin>` çalıştırılabilir.

| Madde | `validate` |
|---|---|
| `component.json` geçerli, `sshpocket_api: 1`, en az bir `supported` profil | ❌ hata |
| Sürüm üçlüsü: `VERSION` = `component.json`; arşiv adı/kökü `<id>-<sürüm>/` | ❌ hata |
| `requirements.txt` var, hepsi `==` sabit ve `--hash=sha256:` içerir; URL/VCS/`-e`/`-r` yok | ❌ hata |
| Her eylemin `cmd`'si `ansible-playbook <var olan, paket içi, göreli playbook>` | ❌ hata |
| Paket içinde sembolik bağ, özel anahtar bloğu, `.env`/`id_*`/`.pem` yok | ❌ hata |
| Yapılandırma değiştiren eylemin salt okunur ön koşulu (`preview`), `warnings`'i ve `requires_sudo`'su var | ⚠️ uyarı. Ön koşul beklenmeyenler: adı `rollback`/`recover`/`restore`/`undo` ile başlayan kurtarma eylemleri (kontrol kaybından sonra da çalışmalı) ve yalnızca `installs-package` etkisi olan eylemler (örn. denetim tarayıcısı kuran `audit`) |
| `outputs.format` varsa playbook'ta `SSHPOCKET_RESULT <id>.<eylem>` çıktısı var | ⚠️ uyarı (dinamik üretilen metin yanlış alarm verebilir) |
| Değişken adı parola/token/anahtar izlenimi vermiyor (bool/int değişkenler hariç); her değişken kullanılıyor; üst veri (tür) var | ⚠️ uyarı |
| Değiştiren eylem için belgede geri alma/kurtarma anlatılmış (anahtar sözcük) | ⚠️ uyarı |
| `.sig` arşivin yanında | ⚠️ uyarı (imzayı `scripts/lock-component.sh` doğrular) |
| Geri almanın kapsamı dürüstçe yazılı mı; kilit PR'ının tek satır olması | elle (insan kontrolü) |

## 13. Kararlar (2026-10-11)

| # | Soru | Karar |
|---|---|---|
| 1 | Bildirim biçimi | **`component.json`** (okuma `python3 -I` + stdlib `json`). `component.yml` yalnızca insan-okunur kaynak; paket üretimi JSON'u ondan türetir. |
| 2 | `runner: script` | **Uygulandı** (sshpocket 3.6.17+): betik hedef sunucuda çalışır (§7). |
| 3 | `confirm` | sshpocket **asgari kuralı** koyar (§5); paket yalnızca sıkılaştırabilir, gevşetemez. |
| 4 | Sonuç satırı | Tek satır `SSHPOCKET_RESULT <id>.<eylem> key=value …` (§8); linux-hardening#28 buna uyarlanır. |
| 5 | Bildirim özeti kilitte | **Hayır**: bildirim zaten imzalı arşivin içinde; kilit dosyası arşivin sha256'sını taşır. |

Bu kararlarla standart **v1 kabul edilmiştir**; uygulama sırası: 2.0 (kilit dosyası + imza doğrulama) → 2.1 (kurulum)
→ 2.2 (bildirim okuma) → 2.3 (`--doctor` + venv) → 2.4 (`--update` entegrasyonu). linux-hardening tarafı farkları (§11) **issue** olarak izlenir.

## 14. Kararlar (2026-10-16, linux-hardening#33 / sshpocket#40)

| # | Soru | Karar |
|---|---|---|
| 6 | `invoke.append` yer tutucuları | `<inventory>` ve `<server>` kabul (`^<[a-z0-9_ -]{1,40}>$`). `-e @<dosya>` pakette **yok**: değişken dosyasını sshpocket verir (§7). |
| 7 | `-i` / `--limit` | **Çalıştırıcı ekler** (geçici inventory'yi sshpocket üretir); paket bunları `invoke.append`'e yazmaz. `invoke.append` yalnızca ek bayraklar içindir (örn. `--ask-become-pass`, `interactive`). Yazılırsa çalıştırıcı yinelenen `-i`/`--limit` girdilerini atlar. |
| 8 | `outputs.prefix` | `<id>.<eylem>` kısmını sshpocket bildirimdeki `id` ve eylem adından **türetir**. Pakette `prefix` varsa bununla birebir uyuşmalı; uyuşmazsa paket reddedilir. `keys` bildirimde kalır. |
| 9 | Sonuç satırı ayrıştırma | Satır başı varsayılmaz: `^\s*SSHPOCKET_RESULT\s+` ile aranır (Ansible'ın `msg:` bloğu altında girintili gelir). |
