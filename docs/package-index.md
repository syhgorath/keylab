# Paket indeksi (şartname, 1. aşama)

> Durum: **şartname**; istemci ve betikler henüz yok. Hedef sürüm: sshpocket **3.7.0**.
> İlgili: `docs/package-standard.md` (paket, kilit, imza), `docs/guncelleme.md` (sshpocket güncellemesi).

## 1. Amaç ve ilke

Bugün bir paket sürümü yayımlanınca üç iş gerekir: paket yayını → `components.lock` PR'ı → yeni sshpocket sürümü. İndeks bu zinciri kısaltır:
**paket yayını + indeks güncellemesi** yeter; sshpocket sürümü gerekmez.

| Kavram | Rolü |
|---|---|
| **İndeks** (`index.json` + `index.json.sig`) | "Her paketin **güncel** sürümü ve özeti ne?" Bakımcı imzalar; sık değişir. |
| **Kilit** (`components.lock`) | **Taban**: asgari sürüm + özet + **paket imzacısı**. Çevrimdışı/ilk kurulum yedeği. İmzalı sshpocket sürümüyle gelir; seyrek değişir. |
| **Paket imzası** (`<asset>.sig`) | Arşivin gerçekten bakımcıdan geldiğini kanıtlar (ad alanı `sshpocket-component`). |

Değişmez ilkeler:
1. **Sessiz otomatik kurulum yok.** İndeks yalnızca "yükseltilebilir" / "kurulabilir" bilgisi üretir; kurulum kullanıcı onayıyla (`upgrade`). `upgrade` (adsız) apt'teki gibi **yalnız kurulu paketleri** yükseltir; kurulu olmayan paket **yalnız adı açıkça verilince** kurulur (`upgrade <paket>`).
2. **Fail-closed.** İndeksle ilgili her hata (ağ, imza, seri, süre, biçim) güncellemeyi durdurur; **kurulu paketler çalışmaya devam eder** ve hiçbir dosya değişmez.
3. **İndeks yeni güven üretmez.** Paket imzacısı yalnızca kilitten gelir. İndeks yeni bir paket kimliği ya da yeni bir imzacı **ekleyemez** (MVP: yalnızca kilitteki paketler).
4. **Geri sürüm yok.** Kurulu sürümün altına ve kilit tabanının altına inilmez.

## 2. Dosyalar ve konum

İndeks **ayrı depoda**: `syhgorath/sshpocket-index`.

| Dosya | İçerik |
|---|---|
| `index.json` | İndeks gövdesi (§3). |
| `index.json.sig` | `index.json` baytlarının ssh imzası (§4). |
| `index_signers` | Depodaki güvenilen açık anahtar satırı (§4); sshpocket'teki `sistem/index_signers` ile **aynı** olmalı. |
| `scripts/index-publish.sh` | Bakımcı aracı (§9). |
| `index.pending.json` | `add` ile biriken, **henüz imzalanmamış** taslak (`.gitignore`'da; commit edilmez). |

İstemcinin okuduğu adresler (yalnızca HTTPS, yönlendirmede de HTTPS):

```
https://raw.githubusercontent.com/syhgorath/sshpocket-index/main/index.json
https://raw.githubusercontent.com/syhgorath/sshpocket-index/main/index.json.sig
```

Önbellek/gecikme (raw CDN birkaç dakika eski olabilir) güvenliği etkilemez: seri numarası (§5) eskiyi reddeder, imza sahtesini reddeder. `SSHPOCKET_INDEX_URL` (yalnızca test; HTTPS veya `file://` testte) taban adresi değiştirir.

## 3. Biçim (`schema` 1)

```json
{
  "schema": 1,
  "serial": 7,
  "issued_at": "2026-11-06T20:00:00Z",
  "valid_until": "2027-01-05T20:00:00Z",
  "packages": {
    "linux-hardening": {
      "repo": "syhgorath/linux-hardening",
      "version": "0.13.0",
      "asset": "linux-hardening-0.13.0.tar.gz",
      "sha256": "f31f7c3073f125f4e0ccab39f6039e118089f5d56e481fe45eb2150cf59956de",
      "requires_sshpocket": "3.7.0",
      "platforms": ["ubuntu-26.04", "debian-13", "proxmox-ve-9"],
      "yanked": false
    },
    "utility": { "...": "..." }
  }
}
```

### Alanlar

| Alan | Tür | Kural |
|---|---|---|
| `schema` | int | Tam `1`. Bilinmeyen `schema` → indeks reddedilir ("bu sshpocket indeksi anlamıyor, güncelleyin"). |
| `serial` | int ≥ 1 | Her imzalı yayında **kesin artar** (§5). |
| `issued_at` | `YYYY-MM-DDTHH:MM:SSZ` (UTC) | İmzalama anı. |
| `valid_until` | aynı biçim | `issued_at` < `valid_until` ≤ `issued_at` + **90 gün** (sınırsız ömürlü indeks olmaz). Varsayılan üretim: +60 gün. |
| `packages` | nesne | Anahtar = paket kimliği (`^[a-z][a-z0-9-]{0,63}$`). Boş olabilir. |
| `packages.<id>.repo` | dize | `sahip/depo` (kilitteki `comp_repo_valid` kuralı). |
| `.version` | `X.Y.Z` | O paketin **güncel** sürümü (paket başına tek sürüm; geçmiş tutulmaz). |
| `.asset` | dize | **Tam** `<id>-<version>.tar.gz` olmalı (MVP'de başka ad kabul edilmez). İmza dosyası `<asset>.sig`. |
| `.sha256` | 64 hane küçük hex | Arşivin özeti. |
| `.requires_sshpocket` | `X.Y.Z`, isteğe bağlı | Gereken **asgari** sshpocket sürümü. Yoksa `3.7.0` sayılır. |
| `.platforms` | dize listesi, isteğe bağlı | Yalnız **bilgi** (listede gösterilir); gerçek denetim `component.json` profilleriyle çalışma anında. |
| `.yanked` | bool, isteğe bağlı (varsayılan false) | `true` = "bu sürüm kurulmasın" (§6, adım 8). |

Bilinmeyen **ek alanlar yok sayılır** (ileriye uyum); ama yinelenen JSON anahtarı, `NaN`/`Infinity`, üst düzeyde dizi/nesne dışı değer → biçim hatası.

### Kanonik üretim

`index-publish.sh` çıktıyı **belirlenimli** üretir (aynı girdi → aynı bayt):
`python3 -I` ile `json.dumps(obj, sort_keys=True, indent=2, ensure_ascii=True) + "\n"`, LF satır sonu, UTF-8, BOM yok.
**İmza, dosyanın ham baytları üzerinedir; istemci yeniden serileştirmez.** Kanonik biçim yalnızca üretimde tutarlılık/diff okunabilirliği içindir; doğrulama önce imza, sonra ayrıştırma sırasıyla yapılır (imzasız veri asla ayrıştırılmaz).

Boyut sınırı: `index.json` ≤ 1 MiB, `.sig` ≤ 4 KiB.

## 4. İmza ve ad alanı

- **Anahtar:** mevcut sshpocket sürüm anahtarı (`~/.ssh/sshpocket_release`; **yeni anahtar yok**). Özel anahtar repoya/CI'a/sohbete konmaz.
- **Ad alanı:** `sshpocket-index` (paket imzası `sshpocket-component` ve sürüm imzası `sshpocket-release` ile karışmaz: ssh imzası ad alanına bağlıdır; bir ad alanında atılmış imza ötekinde geçmez).
- **İmzalama:** `ssh-keygen -Y sign -f <anahtar> -n sshpocket-index index.json` → `index.json.sig`.
- **Doğrulama:** `ssh-keygen -Y verify -f <index_signers> -I sshpocket-index -n sshpocket-index -s index.json.sig < index.json`.
- **Güvenilen anahtar:** yeni dosya `sistem/index_signers` (ssh `allowed_signers` biçimi), sshpocket'in **imzalı sürümüyle** gelir:

  ```
  sshpocket-index namespaces="sshpocket-index" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKVJS3mGlCaOT/l8XfR0TxuKxqLUs6PpVDa9TYqm0fLn
  ```
  Ayrı dosya, `release_signers`'a satır eklemekten seçildi: sürüm güncellemesinin güveni (`release_signers`) indeksin güvenine karışmaz. Dosya yalnız yorumsa (boşsa) indeks özelliği **kapalıdır** (güncelleme özelliğindeki kuralın aynısı). `.gitattributes` `export-ignore` listesine **eklenmez** (arşivde bulunmalı).
- **Anahtar rotasyonu / ele geçirilme (not):** `index_signers` birden çok satır taşıyabilir. Rotasyon: (1) yeni anahtar `index_signers`'a eklenen bir sshpocket sürümü yayımlanır (eski+yeni birlikte); (2) kullanıcılar bu sürüme geçince indeks yeni anahtarla imzalanır; (3) sonraki sshpocket sürümü eski anahtarı çıkarır. Seri numarası rotasyonda **devam eder** (sıfırlanmaz). İndeks, anahtarı kendi kendine iptal edemez (döngüsel güven); ele geçirilmede tek yol imzalı bir sshpocket sürümüdür. **Dürüst not:** indeks anahtarı sürüm anahtarıyla aynı olduğundan, ele geçirilme ikisini birden etkiler; fark yalnızca ad alanı ayrımıdır (bir imzanın başka amaçla yeniden kullanılmasını önler), anahtar sızıntısına karşı koruma **değildir**.

## 5. Seri numarası ve geçerlilik (replay / rollback / freeze)

İstemci durumu: `<components>/.index/state` (düz metin, `serial=<n>` ve `sha256=<index.json özeti>` satırları) + `<components>/.index/index.json(.sig)` (son kabul edilen kopya, çevrimdışı liste için). USB'de `components/` ile birlikte taşınır.

| Durum | Karar |
|---|---|
| `serial` < kayıtlı | **Reddet** ("eski indeks: geri alma saldırısı olabilir"). Hiçbir şey değişmez. |
| `serial` == kayıtlı, özet aynı | Kabul (aynı indeks yeniden alındı). |
| `serial` == kayıtlı, özet **farklı** | **Reddet** (aynı seri iki farklı içerik: çatallanma/oynama). |
| `serial` > kayıtlı | Kabul; durum **doğrulamanın tamamı geçtikten sonra** (§6, adım 7) ilerletilir. |
| Durum dosyası yok/bozuk | İlk kullanım say (serial 0). Bozuk ama var → "durum bozuk" uyarısı, ilk kullanım gibi davran, sonra yeniden yaz. |
| `now` > `valid_until` | **Reddet** ("indeks süresi dolmuş": donma saldırısı ya da bakımcı yenilememiş). Önbellekteki kopya da aynı kuralla "eski" sayılır; `list` bilgi gösterir, `upgrade` yapmaz. |
| `now` < `issued_at` − 1 gün | **Reddet** ("yerel saat geride/ileride ya da indeks gelecekte imzalanmış"); mesajda saat kontrolü önerilir. |

Bakımcı sorumluluğu: indeks en geç `valid_until` öncesi `index-publish.sh refresh` ile yenilenmelidir (içerik aynı, seri+1, yeni `valid_until`). Yenilenmezse yalnızca **yükseltme** durur; çalışan sistem etkilenmez.

## 6. İstemci doğrulama algoritması

Girdi: `update`, `upgrade`, `list` (listede ağ adımı 1–3 atlanıp önbellek kullanılabilir; adım 4–8 yine uygulanır).

1. **Ön koşul:** `sistem/index_signers` en az bir anahtar içeriyor; `python3`, `ssh-keygen`, `curl` var. Yoksa: "indeks kapalı", çık (hata sayılmaz, çıkış kodu 0, bilgi satırı).
2. **İndir:** `index.json` + `index.json.sig` geçici dizine; yalnızca HTTPS (`--proto =https --proto-redir =https --tlsv1.2`), `--max-time 15`, boyut sınırı (§3). Ağ/HTTP hatası → "indeks alınamadı", çıkış 1; durum ve kurulumlar değişmez.
3. **İmza:** §4 komutu. Başarısız → "indeks imzası geçersiz", çıkış 1. (Sonraki adımlar yalnızca imza geçtikten sonra çalışır.)
4. **Ayrıştır ve doğrula (§3):** biçim hatasında "indeks biçimi geçersiz (<neden>)", çıkış 1.
5. **Süre:** §5 tablosundaki `valid_until` / `issued_at` kuralları.
6. **Seri:** §5 tablosu (yalnızca **karşılaştırma**; henüz yazma yok).
7. **Durumu ilerlet:** 4–6 geçtiyse `state` ve `.index/index.json(.sig)` atomik yazılır (`tmp` + `mv`). Kurulum başarısı/başarısızlığı durumu etkilemez (indeks kabul edildi; paket kurulumu ayrı).
8. **Paket başına değerlendirme** (yalnızca **kilitte olan** paketler; kilitte olmayan indeks girdisi "tanınmayan paket, yok sayıldı" bilgisiyle atlanır):
   1. `yanked: true` → aday yok; kurulu sürüm bu sürümse uyarı ("yanked: <sürüm> artık önerilmiyor").
   2. `requires_sshpocket` > çalışan `VERSION` → aday yok; "sshpocket X.Y.Z gerekli".
   2b. İndeksteki `repo` kilitteki `repo` ile **aynı değil** → aday yok, paket yok sayılır ("indeks başka depo gösteriyor"); indeks kaynağı değiştiremez.
   3. `version` < kilit sürümü → **yok say** + uyarı ("indeks kilit tabanının altında"); kilit asla alçaltılmaz.
   4. `version` == kilit sürümü ve `sha256` ≠ kilit `sha256` → paket için **hata** ("indeks ve kilit çelişiyor"); o paket yükseltilmez.
   5. Paket **kurulu** ve `version` > kurulu sürüm → **yükseltilebilir** (`kurulu → yeni`). Paket **kurulu değil** (ve `version` ≥ kilit; 8.3 ile garanti) → **kurulabilir** (`list`/`update` "kurulabilir → X" gösterir; adsız `upgrade` kurmaz).
   6. Aksi → güncel.

`upgrade [paket…] [--yes]`: adım 1–8, sonra adsızsa her **yükseltilebilir (kurulu)** paket için; adlar verilmişse yalnız o adlar için (kurulabilir olanlar dahil) **kullanıcı onayı** (sürüm geçişi, `requires_sshpocket`, kaynak depo gösterilir; `--yes` yalnızca etkileşimsiz kullanımda onay yerine geçer, **varsayılan değildir**), ardından:

9. **Paketi indir:** `https://github.com/<repo>/releases/download/v<version>/<asset>` ve `<asset>.sig` (HTTPS, boyut sınırı mevcut `comp_download` ile, 50 MiB).
10. **Özet:** arşivin sha256'sı indeks `sha256`'sına eşit (ve `version == kilit` ise kilit `sha256`'sına da).
11. **Paket imzası:** `ssh-keygen -Y verify`, ad alanı `sshpocket-component`, imzacı = **kilitteki `signers`** (indeksten imzacı alınmaz).
12. **Geri sürüm yok:** `version` ≥ kurulu sürüm ve ≥ kilit sürümü.
13. **Arşiv denetimi** (`comp_check_archive`), açma, `component.json` `id` ve `version` eşleşmesi, dosya özetleri (`.files`), `current` yazımı: bugünkü `comp_install_archive` adımları aynen.

Herhangi bir adım başarısız olursa o paket için eski `current` **dokunulmadan** kalır; diğer paketler sürer; çıkış kodu ≠ 0 ve özet satırı: `yükseltilen: n, atlanan: n, hata: n`.

## 7. Kilit tabanı etkileşimi

`components.lock` **kalır**; rolü değişir:

| Konu | Bugün | İndeksli |
|---|---|---|
| Kurulum sürümü | tam kilit sürümü olmalı | **≥ kilit sürümü** |
| Özet kaynağı | kilit | `== kilit sürümü` → kilit; `> kilit` → imzalı indeks |
| Paket imzacısı | kilit | **kilit** (değişmez) |
| Kilitte olmayan paket | çalışmaz | çalışmaz **ve indeksten kurulamaz** (MVP) |
| `--components sync` | kilit sürümünü kurar | aynı (çevrimdışı/ilk kurulum yolu; indeks gerekmez) |
| Çalıştırma kapısı (`comp_prepare`) | kurulu == kilit | **kurulu ≥ kilit** (kurulu < kilit ise bugünkü "eski, `sync`/`upgrade`" akışı) |

Kilit ne zaman yükseltilir? Yalnızca **tabanı yükseltmek** için (örn. eski sürümde güvenlik açığı varsa "bundan aşağısı çalışmasın" demek): normal paket yayını için **gerekmez**. Sonuç: çoğu sürümde `components.lock` PR'ı ve sshpocket sürümü yok.

Kod etkisi (2. aşama): `comp_install_archive` içindeki "sürüm kilitle uyuşmuyor" ve `comp_verify` içindeki "özet kilitle uyuşmuyor" kontrolleri, çağıran taraf **doğrulanmış indeks özetini** verebilecek biçimde genişler (`comp_verify <id> <arşiv> <imza> [beklenen-sha256]`). Test köprüsü ve `comp_download` işlev değiştirme kalıbı korunur.

## 8. sshpocket sürümü uyumu

- İndeksi okuyan en küçük sürüm **3.7.0** (yeni özellik). Eski sürümler indeksi **hiç tanımaz**; kilitleriyle eskisi gibi çalışır (geriye uyum bozulmaz).
- `schema` ana sürümü artarsa 3.7.x istemci indeksi reddeder ve "sshpocket'i güncelleyin" der; eski `schema` 1 indeksi de bir süre yayında tutmak bakımcıya bırakılır (yeni alan eklemek `schema`'yı artırmaz, ek alanlar yok sayılır).
- `requires_sshpocket` paket başınadır: yeni bir paket sürümü, eski sshpocket'te bozulacaksa istemci onu **önermez** (adım 8.2).
- Paket `component.json` içindeki `sshpocket_api` (bugün 1) ayrı bir denetimdir ve değişmez.

## 9. `index-publish.sh` arayüzü (bakımcı aracı)

Konum: `syhgorath/sshpocket-index` deposunda `scripts/index-publish.sh` (yerelde: `~/Developer/sshpocket-index`). Yalnızca bakımcı, yerelde çalışır. **Push yapmaz, etiket atmaz**; yalnızca yerel commit üretir.

```
index-publish.sh <id> <sürüm> <sahip/depo> [seçenekler]      # add + sign (tek paket, tek komut)
index-publish.sh add  <id> <sürüm> <sahip/depo> [seçenekler] # taslağa ekle (imzasız, anahtar gerekmez)
index-publish.sh sign                                         # taslağı birleştir, seri+1, imzala, commit
index-publish.sh refresh                                      # içerik aynı, seri+1, yeni valid_until, imzala, commit
index-publish.sh --dry-run ...                                # yazmadan ne yapacağını gösterir
```

Seçenekler: `--archive <yol> --sig <yol>` (yerel dosyalar; yoksa GitHub Release'ten indirilir), `--requires-sshpocket X.Y.Z`, `--platforms a,b,c` (yoksa arşivdeki `component.json` profillerinden), `--valid-days N` (≤ 90, varsayılan 60), `--yes` (onay sorma). Anahtar: `INDEX_SIGNING_KEY` (yoksa `PAKET_SIGNING_KEY`); aynı yolda `.pub` dosyası bulunmalı. `yank` komutu **sonraya** (şema alanı hazır; şimdilik elle `--yanked` yok).

`add` (anahtar gerekmez):
1. Ön koşullar: indeks deposu temiz, `main`'de, `git pull --ff-only` yapılmış; `id`/`sürüm`/`depo` biçimleri geçerli.
2. Arşiv + imzayı al (yerel ya da `https://github.com/<repo>/releases/download/v<sürüm>/…`).
3. **Bağımsız doğrulama:** `ssh-keygen -Y verify` (ad alanı `sshpocket-component`, imzacı = `<anahtar>.pub`), `component.json` `id`+`version` eşleşmesi, sha256 hesabı; GitHub'daki asset digest'i (varsa `gh api`) yerelle aynı mı.
4. Sürüm, indeksteki mevcut sürümden **küçük ya da eşit değilse** reddet (geri sürüm yok; aynı sürümü yeniden yazmak yok).
5. Girdiyi `index.pending.json`'a yaz (kanonik biçim).

`sign`:
1. Taslak yoksa hata. `index.json` + taslak birleştirilir; `serial` = mevcut + 1 (ilk indekste 1); `issued_at` = şimdi (UTC); `valid_until` = +`--valid-days`.
2. Kanonik `index.json` yazılır; `ssh-keygen -Y sign -n sshpocket-index` ile `index.json.sig` üretilir (**parola istemi burada**).
3. **Kendi doğrulaması:** `index_signers` ile §4 verify, ayrıca §3 biçim kuralları ve §5 (serial artışı); geçmezse dosyalar geri alınır, commit yok.
4. `git add index.json index.json.sig && git commit` (ileti: `indeks: <id> <sürüm> (seri <n>)`); taslak silinir. Ziya'ya **tek satır push komutu** yazdırır; push Ziya'dadır.

Parola turu: iki imza vardır (paket: `release.sh`; indeks: `sign`). Anahtar ssh-agent'ta yüklüyse (kullanıcı kendisi `ssh-add` yapar) tek parola; değilse aynı komut içinde iki istem. **Betik parolayı/anahtarı hiçbir zaman okumaz, saklamaz.**

### Paket betikleriyle entegrasyon kuralı

Paketin `scripts/release-publish.sh`'ı (utility/kurulum/linux-hardening) **`gh release create` + yüklenen dosyaların özet doğrulaması geçtikten sonra**, son "✅ yayımlandı" satırından **önce** indeks betiğini çağırır:

```bash
# 6) indeks: paket yayını doğrulandıktan sonra (indeks hatası Release'i geçersiz kılmaz; tek başına yeniden çalıştırılabilir)
INDEX_PUBLISH="${SSHPOCKET_INDEX_DIR:-$HOME/Developer/sshpocket-index}/scripts/index-publish.sh"
[ -x "$INDEX_PUBLISH" ] || die "index-publish.sh bulunamadı ($INDEX_PUBLISH); Release yayımlandı, indeks elle: SSHPOCKET_INDEX_DIR ayarlayın"
"$INDEX_PUBLISH" "$ID" "$VERSION" "$REPO" --archive "$ARCHIVE" --sig "$ARCHIVE.sig"
```

- `--dry-run` modunda da "Yapılacaklar" listesine **6)** adımı eklenir ve çağrıya `--dry-run` iletilir.
- Mevcut `components.lock` için "SHA-256 yazdır" satırı **kalır** (taban yükseltmek isteyenlere bilgi), "sshpocket'e dokunulmaz" notu güncellenir: "indeks güncellendi, push sizde".
- İndeks adımı Release'ten sonradır: indeks hatası paketi geri almaz; çözüm `index-publish.sh <id> <sürüm> <depo>` komutunu tek başına çalıştırmaktır (idempotent: aynı sürüm zaten yazılıysa "zaten güncel" der, çıkış 0).
- Betik `ID`, `VERSION`, `REPO`, `ARCHIVE` değişkenlerini bugünkü `release-publish.sh`'tan alır (`REPO` zaten orada hesaplanır).

## 10. İlk indeksin üretimi

Hedef: utility 0.1.1, kurulum 0.1.0, linux-hardening 0.13.0 (hepsi yayımlı ve kilitteki sürümler; hiçbiri kilidin üstünde değil).

```bash
cd ~/Developer/sshpocket-index
scripts/index-publish.sh add utility 0.1.1 syhgorath/utility
scripts/index-publish.sh add kurulum 0.1.0 syhgorath/kurulum
scripts/index-publish.sh add linux-hardening 0.13.0 syhgorath/linux-hardening
scripts/index-publish.sh sign      # seri 1, tek imza, parola turu burada
git push origin main               # Ziya
```

Beklenen: `serial: 1`; üç paket; `kurulum` ve `linux-hardening` sha256'ları `components.lock` ile birebir aynı, `utility` 0.1.1 (kilitte 0.1.0 → ilk gerçek "yükseltilebilir").

## 11. Kapsam

**MVP (3.7.0):** §3–§9 (indeks, imza, seri, `valid_until`, doğrulama zinciri, `components update|upgrade|list`, `index-publish.sh add/sign/refresh`), ilk indeks, uçtan uca deneme.
**Sonraya:** `testing` kanalı, `yank` komutu, menüde "yükseltilebilir" cilası, çevrimdışı önbellek cilası, kilitte olmayan paketi indeksten tanıtma (imzacı keşfi), üçüncü taraf indeksleri (bkz. platform vizyonu).

## 12. 2. aşama test listesi (istemci; `test/components_index.bats`, 40 test)

Her madde bir bats testi (indirme `comp_download` yerine koyma kalıbıyla, imza testte üretilen anahtarla):
imza geçersiz / yanlış ad alanı (`sshpocket-component` imzası indeksi geçmez) · eski seri · aynı seri farklı içerik · süresi dolmuş · `valid_until` > 90 gün · gelecekte `issued_at` · yinelenen JSON anahtarı · `asset` adı kuralı · kilit altı sürüm yok sayılır · kilit sürümünde özet çelişkisi · `yanked` · `requires_sshpocket` büyük · kilitte olmayan paket yok sayılır · özet uyuşmazlığı · paket imzası kilit imzacısıyla · geri sürüm yok · kurulum atomikliği (hata → `current` değişmez) · `index_signers` boş → kapalı · ağ hatasında kurulu paket çalışır · `upgrade` onay vermeden kurmaz · bash 3.2 uyumu.

## 13. Uygulama notları (3.7.0 istemcisi)

- Kod: `sistem/Helpers/ComponentsIndex.sh` (akış), `sistem/Helpers/component_index.py` (şema/süre doğrulama; imza bash'te önce doğrulanır).
- `update`/`upgrade`/`list` çıkış kodları: `0` başarı ya da özellik kapalı; `1` indeks hatası (fail-closed) ya da en az bir paket atlandı/kurulamadı; `2` kullanım hatası.
- Test geçersiz kılmaları: `comp_index_download` (işlev), `SSHPOCKET_INDEX_URL`, `SSHPOCKET_INDEX_SIGNERS`, `SSHPOCKET_INDEX_NOW` (epoch), `SCRIPT_VERSION`.
- Önbellek: `components/.index/{index.json,index.json.sig,state}`; `list` ağsız, her çağrıda imza+süre+seri yeniden doğrulanır.
