# Changelog

Biçim [Keep a Changelog](https://keepachangelog.com/) esas alınmıştır.

## [Unreleased]

### Düzeltildi
- **Ayarlar'da "KeyLab sürümü" boş (—) görünüyordu** (kurulu modda kök klasörde `VERSION` dosyası yoktur): sürüm artık ikilinin gömülü sürümünden gelir. "Kök klasör (USB)" etiketi "Kök klasör" oldu (kurulu modda USB değildir).
- README: taşınabilir kullanımda "bilgisayara bir şey yazılmaz" yanlıştı; durum/günlükler, Python önbelleği ve Keychain bilgisayarda kalır, düzeltildi.

## [4.0.0] - 2026-11-14

Bu sürüm sshpocket'i **kabuktan Go'ya** taşır: ana ürün artık tek, imzalı bir ikilidir ve yerel web arayüzü içinde gömülüdür. Kabuk (bash) motoru `referans/kabuk/` altında dondurulmuş referans olarak durur. Etiket, imza ve Release'i bakımcı oluşturur.

### Değişti
- **Veri klasörü artık kendiliğinden bulunur** (taşınabilir ya da kurulu): sıra `-servers` → `$KEYLAB_SERVERS` → ikilinin (macOS'ta `KeyLab.app`'in) yanındaki `servers/` (**taşınabilir**, USB için) → `~/.keylab/servers` → eski `../servers`. Hiçbiri yoksa ilk açılışta `~/.keylab/{servers,components}` oluşturulur (`servers` 0700) ve kök klasör (`components.lock`, imza dosyaları, `VERSION`) orada aranır. Eskiden geliştiricinin kendi USB adına bağlı sabit `/Volumes/test/servers` yolu varsayılandı; kaldırıldı (USB'den çalışan ikili zaten kendi yanındaki `servers/`'ı bulur). macOS'un indirilen `.app`'i rastgele salt okunur yolda çalıştırması (App Translocation) durumunda `~/.keylab` kullanılır.
- **Python yokken paket eylemi net rehber verir**: bulunan sürümleri, python.org ve Homebrew yollarını söyler, "sunucu yönetimi, terminal ve anahtarlar Python olmadan çalışır" der. KeyLab Python'u ya da Homebrew'u kendisi kurmaz (yönetici izni ve indirilen betik gerektirir).
- **Terminal penceresinin altındaki "Oturumu kapat" düğmesi kalktı**: tek oturumu kapatmak için sekmedeki ✕, tüm pencere için kırmızı düğme yeterli (alt çubuk metni buna göre güncellendi).
- **Tema varsayılanı koyu**: tercih kaydı yoksa arayüz koyu açılır (Cam + Aurora en iyi koyuda görünür); önceden "Otomatik" idi ve işletim sistemi açık temadaysa ilk açılış açık geliyordu. Tema düğmesi sırası koyu → otomatik → açık; "Otomatik" ve "Açık" Ayarlar'dan hâlâ seçilebilir.
- **Cam + Aurora artık varsayılan görünüm**: tercih kaydı yoksa yüzeyler cam gelir; Ayarlar > Görünüm > Yüzeyler'den "Düz" seçilirse tercih saklanır ve korunur (daha önce "Düz" seçmiş olanlar etkilenmez). Opak yedekler (desteksiz tarayıcı, saydamlığı azalt, yüksek karşıtlık) aynı.
- **Terminal penceresinin kırmızı düğmesi "pencereyi kapat" oldu**: bu sunucunun TÜM terminallerini (ssh) sonlandırır, başka sunucuların oturumlarına dokunmaz (macOS'taki gibi). Tek oturumu kapatmak için sekme çipinin ✕ düğmesi ya da alt çubuktaki "Oturumu kapat" kullanılır; sarı düğme yine yalnız küçültür. Testler `terminal2.test.js` (+2; 6 mutasyon yakalanıyor), `look.test.js` varsayılana göre güncellendi.
- **Ana ürün artık Go'dur** (`go/`): tek, imzalı, evrensel (arm64+amd64) macOS ikilisi; yerel web arayüzü içinde gömülüdür. Komutlar: `components list | sync | update | upgrade | check-new | prune | validate | report | run`, `doctor`, `version`, `update [--check]`, `eject`. Arayüzde paket eylemleri varsayılan olarak Go yürütücüsüyle çalışır (`SSHPOCKET_SHELL_RUN=1` kabuğa döndürür).
- **Kabuk motoru ve Bats testleri `referans/kabuk/` altına taşındı** (dondurulmuş referans). Go portu onunla aynı girdilerle yan yana karşılaştırmalı testlerle doğrulandı (çıkış kodu, çıktı, dosya ağacı; 19.992 bozulmuş bildirim, 32 gerçek imzalı sürüm arşivi, canlı imzalı indeks).
- `index_signers` ve `release_signers` depo köküne taşındı (`referans/kabuk/sistem/` altında sembolik bağ).
- CI: Go işi (vet, gofmt, test, derleme; Ubuntu + macOS) eklendi.
- **Öğeye özel Kur/Yükselt/Kaldır**: katalog kartlarındaki düğmeler açıldı (öğe `actions` listesinde bildirilen eylemler için). Eylem yalnız uygulama içinde çalışır; `/api/run` isteği `item` taşır, sunucu öğeyi ve eylemi katalogla doğrular, önkoşul durumunu O ÖĞENİN kapsamında hesaplar ve motora `-set <item_var>=<öğe>` verir; onaylar (snapshot, evet/sunucu adı, yıkıcıda geri alınamama) değişmedi. macOS Terminal yolu öğe eylemlerini kabul etmez.
- **Önkoşul kapısı (requires) öğe/mod kapsamlı oldu**: `items` bildiren pakette (ör. kurulum) bir eylemin sonucu "<öğe>:<mod>" kapsamıyla kaydedilir ve `requires` yalnız aynı kapsamın taze, başarılı kaydını sayar; başka öğe ya da mod için yapılmış önizleme artık önkoşul sayılmaz (eskiden sayılıyordu). `keylab run` koşuya özel `-set ad=değer` seçeneği aldı (değer dosyasına yazılmaz). Arayüz önkoşul rozetleri aynı kapsamla hesaplanır. Öğesiz paketlerde davranış değişmez.
- **Uygulama kataloğu genelleştirildi**: sunucu sayfasındaki katalog artık `kurulum` paketine bağlı değil; `component.json`'da `items` (+ `item_var`) bildiren her paket için kartlar manifestten çizilir (durum: `durum` eyleminin `kurulu`/`portlar` anahtarları). `items` bildirmeyen eski kurulum 0.1.0 yerleşik listeyle çalışmaya devam eder. Paket sekmesi etiketi bilinmeyen paketlerde paketin `name` alanını kullanır.
- **Arayüz (pocketui'den alınanlar, KeyLab'ın güvenlik modeline uyarlanarak)**: klavye kısayolları penceresi; `~/.ssh/config` içe aktarma (yalnız takma ad gönderilir, sunucu yeniden okuyup doğrular); **Karşılaştır** (drift matrisi) ve **Raporlar** (4 şablon, yazdırma; Markdown/CSV/JSON); **Envanter yedeği** (sır içermez, önizlemeli ve seçmeli içe aktarma, ham `server.env` yazılmaz); **iş izleyici** (arka plana alma, üst çubuk iş düğmesi, yenilemede yeniden bağlanma, yeni `GET /api/jobs`); **çoklu gömülü terminal** (en çok 4 oturum, küçültme, oturum çubuğu); **gizlilik kilidi** (parola kilidi değildir); **vurgu rengi** (8 renk, WCAG AA doğrulamalı). pocketui'deki `pkgmodal`, `vars` modülü ve ham env yazan içe aktarma BİLEREK alınmadı.
- **Gömülü terminal**: sunucu sayfasında "Gömülü terminal" düğmesi, tarayıcıda xterm.js ile `ssh -tt` oturumu açar (PTY, SSE çıktı, POST girdi; macOS ve Linux; Windows'ta henüz yok). Yerel kabuk açılmaz, yalnız kayıtlı ve doğrulanmış sunucuya bağlanır; açma/kapama `terminal-audit.log` dosyasına yazılır. xterm.js resmî `@xterm/xterm@5.5.0` paketidir (özet eşleşmesi doğrulandı, bkz. `go/web/vendor/xterm/KAYNAK.md`).
- **Kabuk referansı depodan kaldırıldı** (`referans/kabuk/`, 125 dosya): karşılaştırmalı testler artık kabuğun kaydedilmiş çıktılarını (`testdata/*.json` altın dosyaları, `internal/shellgold`) oynatır; CI ~2 dk. Eski motor `kabuk-referansi` etiketinde ve git geçmişinde durur.
- **Kendini güncelleme** ikili için yeniden tasarlandı: `update` imzalı ikiliyi (`sshpocket-X.Y.Z-darwin-universal` + `.sig`) doğrular, ikilinin bildirdiği sürümü etiketle karşılaştırır, geri sürümü reddeder, atomik yerleştirir ve eskiyi `.update-backup/` altına yedekler. `go/YAYIN.md`, `go/scripts/build-release.sh`.
- **Toplu işlem** (Sunucular sayfası): listedeki (süzgeç/aramadan sonra) sunucularda AYNI salt okunur eylem sırayla çalışır. Yalnız salt okunur, sudo'suz, onaysız eylemler sunulur; yazan, yıkıcı, sudo'lu ve onay isteyen eylemler toplu YAPILAMAZ. Her sunucu tek tek `authorizeRunItem` + `keylab run` yolundan geçer (kapılar, günlük, sonuç kaydı değişmez); yetki sunucuda yeniden denetlenir. Toplu iş sürerken başka iş başlatılamaz (aralarda boşluk yok), en çok 200 sunucu, sunucu başına 10 dk, "ilk hatada dur" ve iptal seçenekleri. `GET /api/batch`, `POST /api/batch`, `/api/batch/options`, `/api/batch/cancel`. Toplu işlem **paralel** çalışabilir: pencerede "Aynı anda" (varsayılan en çok 4, Ayarlar > Eylemler'deki sınırı aşamaz); fazlası sırada ("bekliyor") kalır ve biri bitince başlar; "ilk hatada dur" sıradakileri iptal eder, çalışanlar biter.
- **Alt çubuk (dock)** (pocketui'deki "Görevler & Son İşlemler" çubuğunun karşılığı, `js/dock.js`): sol altta her zaman görünür; görev sekmesi (sayaç), açık terminal oturumu çipleri (artık sol alttaki yüzen çipler yerine çubukta) ve çalışan iş çipi. Açılınca son işlemler tablosu (her sunucunun `components.log` kaydı, sunucu başına 8, en çok 100 satır) + çalışan iş; sunucu+paketin en yeni satırından son çalıştırma günlüğü açılır. Salt okunur. Önceki "Son işler" penceresinin yerini alır (üst çubuk düğmesi ve pencere kaldırıldı; ⌘K'da "Görevler panelini aç/kapat").
- **`html` etiketli şablon** (`js/core/html.js`, yalnız yeni kod için): `${…}` değerleri varsayılan olarak kaçışlanır; `raw()` kaçışı bilerek kapatır, `setHtml()` düz dize kabul etmez. Davranış testi `go/jstest/html_test.js` (node varsa Go testiyle çalışır). Son işler penceresi buna geçirildi; eski `esc()` şablonları dokunulmadan kalır.
- **Farklı sunucularda paralel eylemler**: tek-iş kuralı gevşetildi. Sunucu başına bir iş, farklı sunucularda en çok 4 iş aynı anda çalışır; genel işler (paket kurulumu, sunucu silme) ve toplu işlem tek başına çalışır. Değişken düzenleme yalnız o sunucudaki iş sürerken engellenir. Arayüzde pencereye bağlı iş, başka sunucuda yeni eylem başlatılırken arka plana alınır; alt çubuktaki her çalışan iş çipinden çıktısıyla birlikte geri açılır. `GET /api/jobs` artık `runningIds` döndürür (`running` en yeni çalışan işle uyumlu kalır).
- **Ayarlar > Eylemler**: aynı anda çalışabilecek eylem sayısı ayarlanabilir (1-16, varsayılan 4). Sunucu tarafı ayar: `<state>/keylab-ui.json` (0600, bozuk/aralık dışı dosya varsayılana düşer), `GET/POST /api/ui-settings`. Düşürmek çalışan işleri durdurmaz, yalnız yenilerini sınırlar.
- **Karşılama kartı** (`js/onboarding.js`): hiç sunucu kayıtlı değilken Pano'nun üstünde 3 adımlı rehber; "Sunucu ekle", "~/.ssh/config'ten içe aktar" ve "Yedekten geri yükle" kısayolları. Gömülü terminalin yazı tipi zincirine Nerd Font/Powerline fontları eklendi (sistemde kuruluysa kullanılır).
- **Terminalde arama ve bağlantılar**: gömülü terminalde Cmd+F (Windows/Linux'ta Ctrl+Shift+F; Ctrl+F kabuğa gider) arama çubuğu: Enter/Shift+Enter ile sonraki/önceki, Aa büyük-küçük harf, Esc kapatır. Çıktıdaki `http(s)` bağlantıları ve IPv4 adresleri yalnız Cmd/Ctrl+tıklamayla etkinleşir: bağlantı `noopener` ile yeni sekmede açılır (başka şema açılmaz), IP panoya kopyalanır. Resmî `@xterm/addon-search@0.15.0` ve `@xterm/addon-web-links@0.11.0` (xterm 5.5 ile uyumlu son kararlı sürümler; npm özetleri ve jsDelivr ile birebir doğrulandı, bkz. `vendor/xterm/KAYNAK.md`).
- **KPI renk tonları** (Pano): Çevrimiçi (hepsi çevrimiçi yeşil, bir kısmı sarı, hiçbiri kırmızı), Son 24 sa başarısız (0 yeşil, >0 kırmızı) ve Ortalama Lynis (≥80 yeşil, ≥60 sarı, altı kırmızı) kutularına sol şerit ve değer rengi. Değer büyük metindir (WCAG AA 3:1 üstü); durum yalnız renkle verilmez, nokta ve alt metin kalır.
- **Terminal hızlı komut çipleri**: terminalin altında yedi sabit, salt okunur komut (`uptime`, `df -h`, `ip addr`, `ufw`, `ssh`, `fail2ban`, `timedatectl`); tıklayınca komut Enter ile açık oturuma gider. Liste koda gömülüdür (sunucudan, yapılandırmadan ya da DOM'dan gelmez); oturum kapalıysa istek gönderilmez. Komut o an terminalde çalışan programa yazılır, kabuk isteminde olunmalıdır; sudo'lular parolayı terminalde sorar. pocketui'deki zsh geçişi bilerek alınmadı.
- **Arayüz test altyapısı** (`go/webtest/`, jsdom): gerçek arayüz, gerçek Go sunucusundan kaydedilmiş API yanıtlarıyla (`fixtures/`), sabit saat ve kontrollü zamanlayıcılarla çalışır. 22 sayfa/pencere anlık görüntüsü (refaktörde DOM değişmemeli) ve 11 iş akışı davranış testi (tek/paralel iş, arka plana alma, dock, toplu işlem, eşzamanlılık ayarı). Yalnız geliştirme/CI aracı (`npm ci --ignore-scripts`, 39 paket, kurulum betiği yok, `package-lock.json` kilitli); ikiliye girmez. `go test` node varsa çalıştırır; CI'da ayrı `web` işi. Bkz. `go/webtest/README.md`.
- **`app.js` ES modülü olarak yüklenir** (`<script type="module">`; refaktörün 1. adımı, davranış değişmedi): kod aynı, yalnız yükleme biçimi. Sonraki adımlarda bölümler `import` ile ayrı dosyalara çıkarılacak. Test kasası modül betiklerini `vm.SourceTextModule` ile çalıştırır (`npm test` `--experimental-vm-modules` verir). Refaktör 2a: ortak yardımcılar (`$`, `$$`, `esc`, `t`, `api`/`json`/`toast`, `ago`, `localTime`, `isOk`, `resHtml`) `js/lib/util.js`'e çıkarıldı (ilk gerçek `import`); doğrudan birim testleri `go/webtest/util.test.js`. Refaktör 2b: kısayollar penceresi, bağlam menüsü ve ssh config içe aktarma `js/ui/` altında ayrı modüller (kod birebir, `app.js` 1938 → 1849 satır); açık beklentili davranış testleri `go/webtest/leaf.test.js` (13 test, 20 mutasyonun 18'i yakalanıyor, kalan iki eşdeğer: odak geri dönüşünü `modal-trap` de yapıyor). Refaktör 2c: sihirbaz (`js/ui/wizard.js`) ve port knocking formu (`js/ui/knock.js`) ayrı modüller (`app.js` 1849 → 1780 satır; kullanılmayan `knockText` yardımcısı silindi); 31 yeni test (`leaf2.test.js`, `refresh.test.js`), 25 mutasyonun 25'i yakalanıyor. Taşıma sırasında 10 sn'lik otomatik yenilemenin `wiz` başvurusu açıkta kalmıştı (ReferenceError, yenilemeyi durdurur); testle yakalanıp düzeltildi. Refaktör 3a: statik tanımsız-başvuru denetimi (`go/webtest/lib/undef.js`, `undef.test.js`; tek yeni geliştirme bağımlılığı `acorn`): `web/` altındaki her betikte hiçbir kapsamda tanımlı olmayan adı çalıştırmadan yakalar; 2c'deki `wiz` hatasını statik olarak bulduğu testle kanıtlandı. Refaktör 3b: paylaşılan durumun dokuz alanı (`home`, `srv`, `srvName`, `srvTab`, `view`, `mode`, `selected`, `routing`, `query`) `js/state.js` → `appState` nesnesine taşındı (kapsama duyarlı kod dönüşümüyle 162 başvuru; yerel gölgelemelere dokunulmadı); davranış testleri `go/webtest/state.test.js`. Test kasası artık yakalanmayan söz (promise) reddini test hatası sayar. Refaktör 4a: iş izleyici + çalıştırma penceresi için 34 karakterizasyon testi (`runmodal.test.js`, `runmodal2.test.js`; taşımadan ÖNCE, mevcut kodda geçer): yazan eylem onay kapıları, Terminal modu ve ilerleme çubuğu, parola/e-h istemleri, bitiş durumları ve pill, kapatma/iptal/kopma, silme/görev/yükseltme modları, Ayarlar'daki parola girişi tercihi. 32 mutasyonun 31'i yakalanıyor (kalan 1'i eşdeğer). Test kasası sahte yanıtlarda başlık (`headers`) taşıyabilir; ortak sahte iş sunucusu `lib/fakejobs.js`. Refaktör 4b: iş izleyici + çalıştırma penceresi `js/ui/jobs.js`'e taşındı (kod birebir, `app.js` 1788 → 1415 satır): dış bağlantılar `initJobs(hooks)` (load, renderServer, showView, loadPackages, checkUpdatesUI, renderAddSteps, markTaskDone, getRemoval, openLastLog) ve `setMode`/`setUpgradeTo`; parola girişi tercihi (`askMode`/`askAvail`) Ayarlar ile ortak olduğu için `appState`'e, `TAB_LABEL` `js/lib/labels.js`'e taşındı. 131 testin hiçbiri değişmeden geçti (karakterizasyon testleri taşımadan önceki kodda da geçiyor); 45 mutasyonun 44'ü yakalanıyor (kalan 1'i eşdeğer). Refaktör 5a: Lynis sayfası (`js/ui/lynis.js`, `chart` Pano ve sunucu özetiyle ortak), Günlükler sayfası (`js/ui/logs.js`; `openLastLog` buraya geldi) ve OS simgeleri (`js/lib/icons.js`) ayrı modüller (kod birebir, `app.js` 1414 → 1255 satır); 22 karakterizasyon testi (`lynlogs.test.js`, `lynlogs2.test.js`; taşımadan önce eski kodda da geçer), tüm testler değişmeden geçti (153). 55 mutasyonun 54'ü yakalanıyor (kalan 1'i eşdeğer: boş eşleşme döngüsü korumasına ulaşılamaz). Refaktör 5b: Paketler sayfası (liste, ayrıntı penceresi, imzalı indeks kartı, Güncellemeler ve Paketleri keşfet pencereleri, yükseltme onayı girişi) `js/ui/packages.js`'e taşındı (kod birebir, `app.js` 1255 → 1075 satır; dış bağlantılar `initPackages(hooks)`: openDlg, closeDlg, isDlgOpen, jobs); 21 karakterizasyon testi (`pkgpage.test.js`, `pkgpage2.test.js`; taşımadan önce eski kodda da geçer), 174 testin hepsi geçiyor. 69 mutasyonun 68'i yakalanıyor (kalan 1'i eşdeğer: hiç kullanılmayan sunucu yokken `showNone` varsayılanı). Refaktör 5c: Değişken düzenleme penceresi (açma, denetim türleri, süzgeçler, kaydet/onayla/varsayılana dön, kayıt sonrası yenilemeler) `js/ui/vars.js`'e taşındı (kod birebir, `app.js` 1074 → 1001 satır; dış bağlantılar `initVars(hooks)`: renderServer, resetCatalog); 20 karakterizasyon testi (`vars.test.js`, `vars2.test.js`; taşımadan önce eski kodda da geçer), 194 testin hepsi geçiyor. 52 mutasyonun 51'i yakalanıyor (kalan 1'i eşdeğer: devre dışı düğmeye Enter ile tıklama zaten etkisiz). Refaktör 5d: sunucu sayfası kümesi (başlık düğmeleri, sekmeler, Özet, Lynis kahramanı, Son durum, değişken/eylem kartları, yazılım kataloğu, adres düzenleme, `openServer`) `js/ui/server.js`'e taşındı (kod birebir, `app.js` 1000 → 598 satır; dış bağlantılar `initServer(hooks)`); 68 karakterizasyon testi (`server1-5.test.js`; taşımadan önce eski kodda da geçer), 259 testin hepsi geçiyor. 133 mutasyonun 124'ü yakalanıyor (kalan 9'u eşdeğer ya da ulaşılamaz: geçersiz/eksik tarih korumaları, `delta === 0` dalı, `yok` ayrıştırması, boş grup → `undefined`, ad yokken `reloadServer`, geçersiz sunucuda adres hesabı, bulunamayan paket dalı, aynı sunucuda `replaceState` seçimi). Refaktör 5e: Tema/Ayarlar (`js/ui/settings.js`), ortak iletişim penceresi (`dialog.js`), sunucu yeniden adlandırma/silme (`serverops.js`) ve Sunucu ekle sihirbazı (`add.js`) ayrı modüller (kod birebir, `app.js` 598 → 412 satır; dış bağlantılar `init*(hooks)`); 27 karakterizasyon testi (`ayarlar.test.js`, `ops.test.js`, `ops2.test.js`; taşımadan önce eski kodda da geçer), 286 testin hepsi geçiyor. 101 mutasyonun 101'i yakalanıyor. Bununla `app.js` yalnız çekirdeği (gezinme, çizimler, arama, bağlam menüsü, karşılaştırma/sihirbaz bağlantıları, başlangıç) içerir; JS refaktörü tamamlandı.

### Güvenlik
- **Oturum çerezi artık jeton değil, süresi dolan bir bilettir** (Proxmox'taki "uzun ömürlü sır → kısa ömürlü bilet" fikri): açılış adresi (`?b=`/`?t=`) rastgele bir oturum bileti verir; sunucuda yalnız SHA-256 özeti tutulur. Kayan boşta süre 12 saat (her yetkili istekte yenilenir; açık pencerenin otomatik yenilemesi canlı tutar) ve mutlak üst sınır 7 gün. Çalınan ya da artakalan bir çerez, pencere kapandıktan sonra süre dolunca işe yaramaz; jetonun kendisi çerez olarak kabul edilmez. Süre dolunca arayüz 401 alır ve "terminalde yazılan adresi açın" iletisini gösterir (jeton süreç boyunca geçerli, adres yeniden açılabilir).
- **Terminal akışı kısa ömürlü, tek kullanımlık bilet ister:** `EventSource` özel başlık gönderemediği için akış `POST /api/term/ticket` ile alınan, 30 sn geçerli, oturuma bağlı, tek kullanımlık bir biletle açılır (`/api/term/stream?id=…&ticket=…`); biletsiz, süresi dolmuş, başka oturumun ya da kullanılmış bilet 403 alır. Girdi/boyut/kapatma POST'tur ve `X-Requested-With` + Origin denetimi taşır. Go testleri `session_test.go` (8), arayüz testleri `terminal2.test.js` (+3); canlı `curl` denemesiyle doğrulandı (jeton çerez olarak 401, biletsiz akış 403, olmayan oturuma bilet 404).
- **Arayüz ayrı, eklentisiz tarayıcı profilinde açılır (macOS):** Chrome, Edge, Brave ya da Chromium bulunursa KeyLab'a ait ayrı profille (`<durum klasörü>/browser-profile`, 0700), `--disable-extensions` ve uygulama penceresi (`--app`) kipinde başlar; günlük tarayıcınızın eklentileri (parola/terminal girdisini okuyabilen içerik betikleri) ve çerezleri devreye girmez. Bulunamazsa sistem tarayıcısı açılır ve uyarı yazılır (Safari komut satırından profil/eklenti denetimi vermez). `-browser default` eski davranıştır. Bu ağ izolasyonu DEĞİLDİR: sunucu hâlâ yalnız `127.0.0.1`'dedir.
- **Açılış adresi tek kullanımlık jeton taşır (`?b=`):** tarayıcı komut satırı oturum boyunca `ps` ile görülebildiği için gerçek oturum jetonu artık oraya yazılmaz; açılış jetonu ilk kullanımda çerez verir ve yanar (canlı denemede doğrulandı: gerçek jeton `ps`'te yok, ikinci kullanım çerez vermiyor). Terminale yazdırılan `?t=` adresi elle açmak için aynen çalışır. Go testleri `browser_test.go`.
- **Yol kaçışı kapatıldı:** sunucu/paket adı doğrulaması (`reName`) `.` ve `..` kabul ediyordu; `/api/vars` isteğinde sunucu adı `..` iken sunucu klasörünün DIŞINA `<paket>.vars` dosyası yazılabiliyordu (oturum çerezi + CSRF başlığı gerektiren yerel bir yüzey). Adlar artık harf/rakam/`_` ile başlar (nokta, gizli ad ve baştaki `-` yok) ve dosya işleminden önce ikinci bir yapısal kapıdan (`pathWithin`: mutlak yol + kök içinde kalma) geçer. Envanter yedeği içe aktarma da paket ve değişken adlarını hiçbir dosya işleminden önce doğrular.
- **CSP sıkılaştırıldı:** `style-src-attr 'none'`; arayüzde satır içi `style="…"` öznitelikleri kalmadı (sınıflara taşındı), bir Go testi bunu kaynakta denetler. `style-src 'unsafe-inline'` yalnızca gömülü terminalin xterm.js'i çalışma anında `<style>` elemanı eklediği için kalır.
- Yerel çalışma yüzeyleri: gizlilik kilidi (parola kilidi DEĞİL, yalnız ekran perdesi), sır içermeyen envanter yedeği (önizlemeli, seçmeli içe aktarma; ham `server.env` yazılmaz).

### Düzeltildi (kabuk referansındaki hatalar Go'da düzeltildi; kabukta düzeltilmedi)
- **Lynis sayfası, iki eski hata** (arayüz testlerinin refaktör sırasında bulduğu, `main`'de gerçek tarayıcıda doğrulanan): (1) Lynis ölçümü olup bulgu dosyası olmayan sunucuda `/api/lynis` `findings: null` döndürüyor, arayüz `findings.filter` ile çöküyor ve bulgu listesi hiç çizilmiyordu; sunucu artık hep `[]` döndürür (Go testi), istemci eski ikili/sunucuya karşı `null`'ı da karşılar. (2) Lynis sayfası adresle (`#v=lynis`) ya da yenilemeyle açılınca sunucu varken "Sunucu yok" yazıyordu (sayfa sunucu listesi yüklenmeden çizilip yeniden çizilmiyordu); liste yüklenince yeniden çizilir.
- `comp_vars_set`: var olan dosyada `awk -v` ters bölüyü yorumlayıp dosyaya fazladan satır sızdırıyordu.
- `comp_report_pkg`: satır sınırı hiç uygulanmıyor ve değişiklik varken bile "Değişken ve kural farkı yok." yazılıyordu.
- Taşan port numarası (`ssh_validate_target`) doğrulamayı geçiyordu.
- GitHub `git archive` arşivlerinin başındaki `pax_global_header` Go'da yanlışlıkla reddediliyordu (Go tarafında düzeltildi).

### Eklendi
- **`KeyLab.app` + zip** (`go/scripts/package-app.sh`, `build-release.sh` çağırır): universal ikiliden çift tıklanır uygulama paketi (Info.plist, simge, ad-hoc imza; başlatıcı betik yok, ikili veri klasörünü kendisi bulur). `keylab-X.Y.Z-macos.zip` olarak üretilir, aynı anahtarla imzalanır (`.sig`) ve `release-publish.sh` Release'e yükler. README'ye "Kurulum (macOS)" bölümü eklendi (normal / USB-taşınabilir kullanım, Gatekeeper uyarısı). Apple Developer ID/noter onayı YOK; Linux ve Windows paketi yok.
- **Tek komutlu yayın** (`go/scripts/release-publish.sh`): kontroller → derle ve imzala (`build-release.sh`) → etiket → GitHub Release (varlıklar + CHANGELOG notu) → yüklenen dosyaların SHA-256 doğrulaması. İmza başarısız olursa uzakta etiket kalmaz. `RELEASING.md` Go yayın akışına göre yeniden yazıldı; CI'da ShellCheck kapsamına eklendi. Eski kabuk dönemindeki `scripts/release-publish.sh`'ın karşılığıdır.
- **Çıkış düğmesi** (üst çubuk, ⏻): onaydan sonra açık terminalleri kapatır, KeyLab USB'den çalışıyorsa (sunucular klasörü `/Volumes/<ad>/` altında) USB'deki anahtarları ssh-agent'tan siler ve USB'yi çıkarır (`keylab eject` ile aynı iş), sonra uygulamayı durdurur ve pencereyi kapatır. Çalışan iş varsa uyarır; ikinci onayla yine de çıkar. Keychain kayıtlarına dokunmaz. USB çıkarılamazsa (ör. ikili USB'den çalışıyor ve "kullanımda") sonucu perdede gösterir; USB'yi uygulama kapandıktan sonra elle çıkarın. Gerçek USB ile henüz denenmedi (testler sahte çıkarma kullanır).
- **Terminaller sunucu başına bağımsız pencere sekmelerinde**: terminal penceresi yalnız etkin oturumun sunucusundaki terminalleri sekme olarak gösterir (ubuntutest ve proxmoxtest kendi sekmelerinde, karışmaz); alt çubuk tüm oturumları gösterir ve başka sunucunun çipi o sunucunun penceresini açar (diğer oturum kapanmaz). Aynı sunucuda birden çok terminal (`web1 #1`, `web1 #2`); sekme sırasının sonundaki **+** bu sunucuda yeni terminal açar (4 oturum sınırında kapalı). Bir sunucunun son terminali kapanınca pencere kapanır, başka sunucunun penceresine atlamaz; ilk terminal kapanınca aynı sunucunun diğerine geçilir. Toplam sınır (4 oturum) değişmedi. Testler `go/webtest/terminal2.test.js` (7; 6 mutasyonun 6'sı yakalanıyor).
- **Gömülü terminal "terminal penceresi" görünümü** (PocketUI'dan): koyu pencere (açık/koyu temadan bağımsız), başlıkta trafik ışığı düğmeleri (kırmızı = oturumu kapat, sarı = küçült, oturum açık kalır; yeşil = tam ekran ↔ eski boyut, terminal yeniden sığdırılır), başlık yanında durum göstergesi (bağlanıyor… / bağlı / oturum bitti / bağlanamadı), koyu oturum çipleri, arama/hızlı komut/alt çubuk koyu temaya uyarlandı. Terminalin kendisi (xterm.js, arama, bağlantılar, hızlı komutlar, oturum yönetimi) değişmedi. Testler `go/webtest/terminal.test.js` (8; 11 mutasyonun 11'i yakalanıyor); terminal çerçevesinin ilk testleri.
- **Pano genel özet oldu**: sunucu bölümü "Sunucu durumu" kompakt çiplerine (durum noktası, ad, son işlem) dönüştü; çevrimiçi olmayan sunucular önce, en çok 12 çip, kalanı "+N sunucu daha" çipiyle Sunucular sayfasına bağlanır. Süzgeç çipleri, Simgeler/Liste düğmesi ve "Sunucu ekle" karosu Pano'dan kalktı (Sunucular sayfasında); Pano'da arama uygulanır, Sunucular'ın süzgeci/kipi uygulanmaz. Çipte `⋯` düğmesi yok (sağ tık / F10 aynen çalışır). Testler `go/webtest/pano.test.js` (6; 10 mutasyonun 10'u yakalanıyor), eski "Pano: süzgeç" görüntüsü "Sunucular: süzgeç" oldu.
- **Sunucular sayfasında zengin sunucu kartı** (Simgeler kipi): durum rozeti (çevrimiçi / knock / bağlantı yok…), OS · platform, `kullanıcı@adres:port` + anahtar var/yok, son işlem ve hızlı düğmeler (Terminal, Kopya, Yönet →); geçersiz sunucuda Terminal/Kopya kapalı. Kart ve düğmeler bağlam menüsüyle aynı eylemleri kullanır; iç düğmede Enter yalnız o düğmeyi çalıştırır (sunucuyu açmaz). Pano'nun küçük karoları ve Liste kipi değişmedi; Pano genel özet olarak kalıyor, ayrıntılar Sunucular'a taşındı. Dar ekranda kart taşmaz. Testler `go/webtest/cards.test.js` (8; 10 mutasyon yakalanıyor); sayfa görüntüleri yeni kart kadar güncellendi.
- **Görünüm: Cam + Aurora** (Ayarlar > Görünüm > Yüzeyler; varsayılan "Düz" değişmedi): kenar çubuğu, üst çubuk, kartlar, pencereler ve bağlam menüsü yarı saydam + bulanık, zeminde vurgu rengine bağlı hafif renk geçişleri. Yalnız `<html data-look="glass">` özniteliği değişir (`js/look.js`, tercih `keylab.look`); tüm stil `tasarim.css` sonunda tek blokta. Opak yedekler: bulanıklık desteği yoksa, `prefers-reduced-transparency` ve `forced-colors`; kart yüzeyi en az %72 opak (terminal/günlük okunurluğu). Testler `go/webtest/look.test.js` (6); görsel açık/koyu temada gerçek tarayıcıda ekran görüntüsüyle denetlendi. Sayfa görüntüleri yalnız yeni ayar satırı kadar güncellendi.
- `servers import-ssh-config | knock | keychain | list` ve `settings` komutları (kabuk akışlarıyla aynı istem metinleri); arayüzde sunucu adımlarına "Keychain parolası" ve "Port knocking"; "Terminal'de aç" bağlanmadan önce knock gönderir; durum yoklaması `sshpocket.conf` (`STATUS`, `STATUS_TIMEOUT`) ayarlarına uyar.
- Go yürütücüsü: kapılar (önizleme gereği, önkoşul TTL, host key, ajan), onay akışı, günlük arşivi (`logs/<zaman>-<eylem>.log`, 0600, parola maskeli), sonuç kayıtları; yanlış sudo parolası ve Ctrl-C'de temiz çıkış.
- Arayüz bulguları artık doğrulamalı okuyucudan gelir (`component_findings.py` ile aynı kurallar; sunucudan gelen veri güvenilmez).

### Dikkat
- **Kabuktan Go'ya geçişte bilinçli olarak taşınmayanlar:** terminal menüsü (fzf dâhil), `--migrate` ve eski `sistem/Modules` düzeni.
- **Henüz yok:** durum göstergesinde 🔒 (knock gerekir) işareti; panodaki "ssh komutunu kopyala" knock göndermez.
- **Gerçek ortamda denenenler:** `ubuntutest` VM'inde `utility` (`durum`, `preview`, değiştiren `apply`, önizlemesiz `apply` reddi, yanlış sudo parolası, Ctrl-C), macOS Keychain'li parolalı anahtar, anahtarı sunucuya kopyalama, sunucu silme, arayüzden `durum`. **Denenmeyenler:** gerçek bir Release ile `update`, gerçek ağda `components update/upgrade`, Knock gönderimi (Knock'lu sunucu yok), Ctrl-C'nin görevler akarken etkisi, internetten inen ikilide Gatekeeper/notarization (ad-hoc imza yetmeyebilir; ilk açılışta sağ tık > Aç gerekebilir).

## [3.7.0] - 2026-11-07

Bu sürüm **imzalı paket indeksini** getirir: paket sürümleri artık sshpocket sürümü ve `components.lock` PR'ı olmadan, onayınızla yükseltilebilir. `components.lock` **taban** olarak kalır. Paket kilitleri değişmedi (utility 0.1.0, kurulum 0.1.0, linux-hardening 0.13.0); indeks `utility 0.1.1`'i "yükseltilebilir" gösterir.

### Eklendi
- **İmzalı paket indeksi istemcisi** (`docs/package-index.md`): `./start.sh --components update` (indeksi alır, doğrular, "yükseltilebilir" listesi), `upgrade [--yes] [paket…]` (onaylı yükseltme), `list` önbellekteki indeksi de gösterir. Menüde "Paketler ve güncelleme" altında iki yeni giriş. Doğrulama zinciri: indeks imzası (ad alanı `sshpocket-index`, yeni `sistem/index_signers`) → şema/süre → seri (geri alma/çatallanma reddi) → paket sha256 (indeks) → paket imzası (**kilitteki** imzacı) → geri sürüm yok. İndeks hatasında güncelleme yapılmaz, kurulu paketler çalışır. Sessiz otomatik kurulum yoktur.
- İndeks adresi `https://raw.githubusercontent.com/syhgorath/sshpocket-index/main/index.json` (+ `.sig`); durum `components/.index/` altında (USB'de `components/` ile taşınır).
- Testte gerçek ağ yok: `comp_index_download` işlevi, `SSHPOCKET_INDEX_URL`, `SSHPOCKET_INDEX_SIGNERS`, `SSHPOCKET_INDEX_NOW`.

### Değişti
- `components.lock` artık **taban**: kurulu sürüm kilit sürümünden **büyük veya eşit** olabilir (çalıştırma kapısı ve `comp_status`); `comp_verify` / `comp_install_archive` doğrulanmış indeksten beklenen sha256 alır. Kilit sürümüne eşit sürümde özet yine kilitteki olmalıdır; paket imzacısı her zaman kilitten gelir; indeks yeni paket ya da yeni imzacı ekleyemez, kilitte olmayan paket indeksten kurulamaz; indeksin depo adı kilitteki ile aynı olmalıdır.
- **`upgrade` (adsız) yalnız KURULU paketleri yükseltir** (apt gibi); kurulu olmayan paket `update`/`list`'te "kurulabilir → X" görünür ve yalnız adı verilince kurulur: `upgrade [--yes] <paket>`.
- `--components sync` değişmedi: kilit sürümünü kurar (çevrimdışı/ilk kurulum yolu).

### Dikkat
- **İndeksle yükseltme gerçek bir VM'de menüden denenmedi** (utility 0.1.0 → 0.1.1 indirme, kurulum ve çalıştırma); gerçek raw adresinden indeks alma ve doğrulama izole bir klasörde elle denendi.
- **fzf'li gerçek terminalde** yeni iki menü girişi elle denenmedi; yalnız birim testleri ve sahte fzf var.
- Test kapsamı: bats (Linux CI + macOS bash 3.2), shellcheck. Başka ortamda (ör. Debian/Proxmox'ta denetleyici olarak) denenmedi.
- İndeksin süresi dolarsa (`valid_until`) yalnızca **yükseltme** durur; kurulu paketler çalışır. Yerel saat çok yanlışsa indeks reddedilir (mesaj saati kontrol etmenizi söyler).

## [3.6.24] - 2026-11-06

Bu sürüm paket kataloğu altyapısını (menü kategorileri, yazılım kataloğu, form), `requires_sudo` parola düzeltmesini, CI/README güncellemelerini ve **iki paket kilidini** getirir: yeni **`kurulum 0.1.0`** ve **`linux-hardening 0.13.0`** (`utility 0.1.0` aynı kaldı).

### Dikkat
- **Yeni manifest alanlarını (`menu`, `items`, `item_var`, `mode_var`) henüz hiçbir gerçek paket kullanmıyor.** Menü kategorileri, katalog ve form yalnız birim testleri ve sahte paketlerle doğrulandı; gerçek bir paketle ve gerçek terminalde fzf'li görünüm elle DENENMEDİ. linux-hardening ve utility `menu` yazmadığı için sunucu menüsü bugünkü "Paketler" satırıyla aynen çalışır.
- **`requires_sudo` düzeltmesi** Docker LAB'da (parolalı sudo yapılmış Ubuntu 26.04 konteyneri) doğrulandı; gerçek bir VM'de menüden denenmedi. Parolalı sudo'lu sunucuda `utility` artık `BECOME password:` ister; NOPASSWD sunucuda Enter yeterli.
- **Ekrandaki "modül" iletileri "sunucu" oldu** (sunucu ekleme, silme, içe aktarma, güncelleme onayı); testlerle doğrulandı, gerçek terminalde akış elle denenmedi.
- **CI** artık `ubuntu-26.04` runner'ında ve `permissions: contents: read` ile çalışır (code-scanning'deki 3 uyarı kapandı). Ayrı olarak, GitHub'ın "AI security scan" koşusu aylık kota nedeniyle hata verebilir; kodla ilgili değildir.
- **`kurulum 0.1.0` (yeni paket) menüden gerçek VM'de, indirilip kurulması dahil, HİÇ DENENMEDİ.** Paket sahibi yalnızca doğrulayıcı (`--strict` uyumlu) ve LAB düzeyinde doğruladı; gerçek VM'de DOĞRULANMAYANLAR: V01–V05 düzeltmeleri, `linux-hardening` uygulanmış VM'de iki sıralı tur, native Redis yeni parola, Debian 13. Ubuntu 26.04 desteklenir, Debian 13 yalnızca planlıdır. Manifest `menu`/`items` kullanmadığı için sunucu menüsünde "🛡️ Paketler (sertleştirme vb.)" satırı altında görünür; tüm eylemleri (`durum` dahil) sudo parolası ister; `kaldir` veriyi silmez.
- **`linux-hardening 0.13.0`:** yeni varsayılan KAPALI seçenekler (GRUB parolası, Proxmox `apt` koruması), SSH/sudo sabitleri değişkenleştirildi (varsayılanlar aynı), systemd sandbox özeti (salt okunur) ve auditd Docker soket kuralı düzeltmesi. Gerçek VM'de DOĞRULANMAYANLAR: reboot sonrası `audit-rules.service` ve `auditctl -l | grep -c docker` (9 olmalı), GRUB'un gerçek açılışı ve parola istemi, Proxmox'ta `apt` hold ve GRUB atlama, Debian. `hardening_grub_password_hash` parola ÖZETİ taşır: değişken dosyasında düz metin durur, zayıf parolanın özeti çevrimdışı kırılabilir (standart §6 istisnası).

### Değişti
- **CI:** `ubuntu-24.04` yerine `ubuntu-26.04` runner (projenin hedef sürümü; bash 5.3) ve iş akışına `permissions: contents: read` (en az ayrıcalık).
- **Doğrulayıcı: parola ÖZETİ adları gevşetildi:** adı `_hash`, `_digest` veya `_checksum` ile biten `string` değişkenler (örn. `hardening_grub_password_hash`, `grub-mkpasswd-pbkdf2` çıktısı) artık "gizli değer olamaz" uyarısı almaz; düz parola/token/anahtar adları (`..._password`, `..._api_token`, `..._hash_password`) uyarılmaya devam eder. Standart §6'ya istisna ve **çevrimdışı kırılma notu** eklendi: özet de değişken dosyasında düz metin durur, zayıf parolanın özeti kırılabilir. `linux-hardening 0.13.0` `--strict` altında artık uyarısız.
- **"Modül" yerine "sunucu":** README'de (TR/EN) ve ekrandaki iletilerde (sunucu ekleme, silme, içe aktarma, güncelleme onayı, key yolu sorusu) eski "modül" terimi sunucu oldu; `servers/<ad>/` düzeniyle uyumlu. Dosya adları (`ModuleGenerator.sh`, `ModuleManager.sh`) ve iç adlar değişmedi. README'ye paketlerin imzalı olduğu ve istemci denemesinin yalnız macOS'ta olduğu notu eklendi.
- **README sadeleşti:** ayrıntılı bölümler `docs/` altına taşındı (`paketler.md`, `test-edilen-ortamlar.md`, `guncelleme.md`, `eski-duzenden-tasima.md`; İngilizce karşılıkları `docs/en/`). Logo ve hardening görseli eklendi (`img/`, küçültülmüş; release arşivine girmez: `.gitattributes`), rozetler: ci, sürüm, indirme, yıldız, lisans. CI satırı Ubuntu 26.04'e güncellendi. Elle denenen/denenmeyen listesi içerik olarak değişmedi, yalnızca taşındı.

### Eklendi
- **Paket bildirimi `menu` alanı (S1):** `"menu": {"category": "hardening|kurulum|paketler", "title", "order"}`. Sunucu menüsündeki tek "🛡️ Paketler" satırı, kurulu paketi olan sabit kategorilere bölünür (🛡️ Sertleştirme, 🧰 Kurulum merkezi, 🛡️ Paketler); `menu` olmayan paketler (linux-hardening 0.12.1, utility 0.1.0) bugünkü satırda kalır, davranış değişmez. Kategori içinde paketler `order`'a göre sıralanır. Doğrulayıcı alanı okur ve bozuksa reddeder (`docs/package-standard.md` §4). Gerçek terminalde fzf'li görünüm elle denenmedi; birim testleri ve seçim akışı sahte paketlerle doğrulandı.
- **Yazılım kataloğu `items[]` (S2):** bildirimde `item_var`/`mode_var` + `items[{id,label,description,modes,actions,params}]`. Paket paneli açılırken yazılım (ve mod) seçtirilir, yalnız o yazılımın eylemleri listelenir; seçim eylem çalışırken koşuya özel (`.vars`'a yazılmadan) verilir ve kapıdan geçer (`comp_gate_run_sets`). Katalogu olmayan paketler (linux-hardening, utility) aynen çalışır. Gerçek paketle uçtan uca ve gerçek terminalde elle DENENMEDİ (`kurulum` paketi bekleniyor); birim testleri ve sahte paketler.
- **Koşuya özel parametre (S3):** seçilen yazılımın `params` listesindeki tanımlı değişkenler, eylem çalışırken `.vars` dosyasına YAZILMADAN verilir (`COMP_CTX_RUNSET`; kapı `comp_gate_run_sets`). Tür/aralık mevcut doğrulayıcıyla denetlenir; listede olmayan ad, yazılımsız parametre ve başka yazılımın parametresi hiçbir şey çalıştırmaz. Menüden bu değerleri koşuya özel verecek bir ekran YOK: S4 formu değerleri normal `.vars` dosyasına yazar; kapı hazır, gerçek paketle DENENMEDİ.
- **Katalog formu (S4):** katalogdan bir yazılım seçilince panelin 📝 Değişkenler ekranı yalnız o yazılımın `params` alanlarını gösterir (mevcut alan-listesi editörü: tür/aralık doğrulaması, varsayılan, açıklama, fzf kartı; başlık "<yazılım> › Ayarlar"). Gerçek `kurulum` paketiyle ve gerçek terminalde fzf'li görünüm DENENMEDİ; birim testleri sahte paketle.

### Düzeltildi
- **`requires_sudo: true` eylemleri sudo parolası sormuyordu (parolalı sudo'da düşüyordu):** ansible runner `--ask-become-pass`'i yalnızca paket `invoke.append`'e yazmışsa ekliyordu (`linux-hardening` yazıyor, `utility` yazmıyordu); parolalı sudo'lu sunucuda `utility preview/apply/geri_yukle` `BECOME password:` istemi çıkmadan "Missing sudo password" ile düşüyor, NOPASSWD sunucuda sessizce geçiyordu. Standart (§5) `requires_sudo`'nun parolayı Ansible'ın kendi isteminden alacağını söylüyordu; sshpocket artık `requires_sudo` eylemde `--ask-become-pass`'i kendisi ekler (paket yazdıysa ikilenmez, sudo gerektirmeyen eyleme eklenmez). Docker LAB'da parolalı sudo ile doğrulandı. Test eklendi.

## [3.6.23] - 2026-11-05

Bu yama sürümü `utility` 0.1.0 paketini kilitler ve fzf arayüz düzenini düzeltir (3.6.22'den sonra bulunan hatalar).

### Dikkat
- **`utility` sshpocket menüsüyle gerçek VM'de henüz DENENMEDİ.** Paket sahibi paketi gerçek VM'lerde (Ubuntu 26.04, Debian 13.7) doğrudan playbook ile denedi; sshpocket tarafında yalnızca Docker LAB'da (Ubuntu 26.04 konteyneri) CLI üzerinden `durum`, `preview`, `apply`, ikinci `preview` (changed=0) çalıştırıldı. Proxmox'ta, `linux-hardening` uygulanmış VM'de birlikte ve ufw giden-reddi açıkken denenmedi. Menüden gerçek VM turu (ubuntutest ve debiantest) bu sürümden sonra yapılacak.
- **fzf duyarlı düzeninin son hâli gerçek terminalde elle teyit edilmedi:** 3.6.22'yi kullanıcı 120×30 ve 254×66 pencerelerde denedi ve eşik hatasını (kartın gereksiz alta düşmesi) buldu; düzeltme ve yükseklik kuralı birim testleri, sahte fzf ve sahte pty ile doğrulandı, düzeltilmiş görünüm gerçek pencerede henüz görülmedi.
- `utility` kendi Python ortamını ister (farklı `requirements.txt` özeti): ilk kullanımda "Kurulsun mu? [e/H]" sorusu (~10 MB indirme, ~90 MB disk).

### Eklendi
- **`utility` 0.1.0 kilitlendi** (`components.lock`; imzalayıcı `linux-hardening` ile aynı): saat dilimi, locale (`LANG`) ve NTP (chrony `sources.d` drop-in); eylemler `durum`, `preview`, `apply` (evet/hayır onayı, `preview` ön koşulu), `geri_yukle`; Ubuntu 26.04 ve Debian 13 (Proxmox VE paket tarafından reddedilir). Manifestte `required` değişken ve `requires_vars` yok: tüm eylemler açık. Docker LAB'da sshpocket üzerinden `durum`, `preview`, `apply`, ikinci `preview` (changed=0) denendi; **sshpocket menüsüyle gerçek VM turu henüz yapılmadı.** Paketin kendi Python ortamı (farklı `requirements.txt` özeti) ilk kullanımda ayrıca kurulur (~10 MB indirme, ~90 MB disk).

### Düzeltildi
- **Sunucu başına paket seçicisinde (iki ve daha çok paket kurulu) kart başlığı iki kez yazılıyordu** (`── 📦 paket sürüm ──`); tek paket varken bu ekran açılmadığından fark edilmemişti. Test eklendi.
- **fzf kartı gereksiz yere alta düşüyordu:** fzf'in `<N(...)` eşiği terminalin değil önizleme penceresinin genişliğine bakar; sağ önizleme %48 olduğundan eşik fiilen ~208 sütundu (120 sütunda kart alta iniyordu). Eşik artık önizleme genişliğine göre ayarlı (`<48` = terminalde 100 sütun).

### Değişti
- **fzf pencereleri kompakt ve duyarlı:** sığıyorsa liste solda kart sağda, sığmıyorsa kart alta; yükseklik ekran başına sabit değil ortak kuralla (`ui_fit_wide`) liste satırı kadar, liste en az 10 satır görünür (ana menü, Ayarlar, platform seçimi, değer seçici, liste editörü, Değişkenler dahil). Dar düzende kart satırı liste için en çok 12 satır ayrıldıktan sonra kalan yeri alır.
- **Çok geniş terminalde (160 sütundan fazla) fzf genişliği sınırlanır ve ortalanır** (`UI_MAX_COLS`, `--margin`); satırlar 250 sütunda aşırı uzamaz.

## [3.6.22] - 2026-11-05

Bu yama sürümü yalnızca arayüzdür (paket kilidi 0.12.1 değişmedi): fzf'te sunucu ve eylem kartları, renk dili, ortak ekran kuralı (yol çubuğu bir kez, bilgi başlık kutusunda), dar terminalde kartın alta alınması, başarısız eylemde `Neden:` satırı ve güncellemenin Paketler panelinin altına alınması.

### Eklendi
- **Sunucu kartı (ana menü, fzf sağ bölmesi):** her sunucu satırının açıklaması artık genel bir cümle değil, o sunucunun kartı: durum simgesi (menünün mevcut yoklamasından), `kullanıcı@ip:port`, anahtar ve knock durumu, kurulu pakette son preview/apply/audit kaydı ve son Lynis skoru, tam `ssh` komutu. Yalnızca yerel veriyi okur (`server.env`, `components.log`, sonuç klasörü); ssh yapmaz. Kart fzf'te renklidir (bölüm başlıkları camgöbeği, etiketler sarı, durum yeşil/kırmızı); ok tuşlu/numaralı modda ve `NO_COLOR` ile düz kalır. Sunucu menüsündeki **Bağlantı Bilgileri** bloğu da terminalde aynı renk diliyle çizilir.
- **Aynı kart ve renk dili iç ekranlarda (fzf):** sunucu menüsünün her satırı (Bağlan, Public key, Bilgiler, Keychain, Knock, Paketler) kendi kartını gösterir; paket panelindeki eylem, geçmiş, değişken ve bulgu kartları renklendi (etiket sarı, bölüm başlığı camgöbeği, ✅ yeşil, ❌ kırmızı, ⚠ sarı); **Ayarlar** (değer, kaynak, varsayılan) ve **Sunucuları yönet** (Sil / Yeniden adlandır) kartları eklendi. Ortak parçalar `UI.sh`'ta: `ui_paint`, `ui_card_head`, `ui_card_row`, `ui_card_colorize`, `ui_fit_wide`, `UI_PICK_HEIGHT`. **Standart: yol çubuğu bir kez, bilgi çizimin içinde (tüm fzf pencereleri):** ekran başlığı artık yalnızca fzf çerçevesinin üst kenarlığında görünür (daha önce aynı yol çubuğu çerçevenin üstünde ayrıca basılıyordu); ekranların bilgi satırları (gizlenen değişken sayısı, gösterilen/değiştirilen, sunucu bağlantı bilgisi, uyarı ve iletiler, sunucu listesi) başlık kutusunun içine alındı. Ok tuşlu ve numaralı modda klasik başlık ve satırlar aynen basılır. Yeni ortak parça: `ui_screen`. **Ana menü › Paketler ve güncelleme:** fzf'te güncelleme artık ayrı bir ana menü satırı değil, Paketler panelinin altında (paket yoksa ya da düz modlarda eskisi gibi ayrı satır); sshpocket satırı mevcut güncelleme akışını (`UpdateAction`: imzalı paket, onaylı, sürüm değişirse paketler eşitlenir) aynen çalıştırır. Paketler paneli fzf'te kartlı paket listesi oldu (kilit, kurulu sürüm, durum, depo): Enter eksik/eski/bozuk pakette yalnız o paketi kilitteki sürüme getirir (klasik onay sorusu ve imza/sha256 doğrulaması aynen), kurulu pakette kurulumu doğrular; ayrıca toplu eşitle, eski sürümleri temizle, yeni yayın var mı satırları. Ok tuşlu/numaralı modda eski durum sayfası kalır. **Bilgileri güncelle** (sunucu menüsü) fzf'te alan listesi editörü oldu: IP/host, Kullanıcı, Port, Key yolu satırları kartlı; Enter yalnızca o alanı sorup hemen doğrular, **Kaydet**'e kadar hiçbir şey yazılmaz; ok tuşlu/numaralı modda eski dört soru aynen kalır. `NO_COLOR` yalnız renkleri kapatır, fzf düzenini (başlık kenarlıkta, bilgi kutuda) kapatmaz. Standarda ayrıca **Knock ayarları**, **Keychain passphrase**, **sunucu başına paket seçimi**, **Değişkenler › grup seçici**, **Bulgular** ve sunucu eklerken **platform seçimi** de alındı; ekran yazma kuralı `docs/ui-design.md`'de. **Paket panelinde liste artık en az 10 satır görünür:** hazırlık özeti başlığı kısa terminalde kısaltılır (önce puan ve bulgular, eylem durumları sonra) ve fzf yüksekliği içeriğe göre ayarlanır; daha önce başlık yüksekliği yutunca listede yalnızca `preview` ve `apply` kalıyordu. Ok tuşlu ve numaralı modda ve `NO_COLOR` ile düz kalır.
- **Değişkenler kartı (paket eylem paneli):** "📝 Değişkenler" satırının sağ bölmesi artık yazılı değer sayısını ve değerleri listeler; paketin uyardığı değer (`warn_if`) ⚠ ile işaretlenir.

### Değişti
- **Dar terminalde (100 sütundan az) fzf kartı alta alınır:** sağdaki kart ~38 sütuna sıkışıp satırları kırıyordu; artık liste tam genişlikte, kart altta 10 satır (fzf `<48(down,…)` önizleme-genişliği eşiği). Sabit yükseklikli ekranlara kart satırları eklenir, terminal yüksekliğini aşmaz; `ui_fit_wide` başlık payını dar terminalde buna göre düşürür.
- **Başarısız eylem sonuç panelinde NEDEN görünür:** `❌ Başarısız (çıkış N)` satırının altına `Neden: …` eklendi; Ansible `fatal` satırındaki betik çıktısı (örn. rollback `disarm`'ın `HATA: geri alma zamanlayıcısı … tetiklenmiş` iletisi) ya da `msg` en çok 3 satır gösterilir. Daha önce bu metin yalnızca günlükte (`last-run.log`) ve kaydırma bölgesinin `fatal` satırında kalıyordu. Başarıda ve kapı reddinde görünmez.
- **fzf / ok tuşu uyumu (beş ekran):** daha önce yalnız düz `read` ile numara/harf soran yerler ortak seçiciye (`ui_pick`) taşındı; `UI=numbers` ve fzf'siz yedekleri aynen duruyor. **Knock ayarları** menüsü; **sunucu eklerken platform seçimi** (açıklamalar fzf sağ bölmesinde); **sunucu eklerken izinli ağ (CIDR) editörü** artık paket menüsündeki Değişkenler ekranıyla aynı liste editörü; **~/.ssh/config'ten içe aktar** çoklu seçimi (Enter işaretler, "İçe aktar" onaylar); **Sunucuları yönet › Sil / Yeniden adlandır** sunucuyu listeden seçtirir (silmede ad yazarak onay hâlâ gerekir).

## [3.6.21] - 2026-11-04

Bu yama sürümü yalnızca `linux-hardening` 0.12.1'i kilitler (geri alma ve UMASK düzeltmeleri).

### Değişti
- **`linux-hardening` 0.12.1 kilitlendi** (`components.lock`; imzalayıcı 0.12.0 ile aynı): geri alma `restore` ufw önceden aktifse `ufw --force reload` yapar; zamanlayıcı `apply` bitmeden tetiklenirse `disarm` HATA yazıp çıkış 3 verir ve `apply` artık başarısız görünür (sshpocket sıfır olmayan çıkışı kayıtta `fail` sayar); `proxmox-host` + `pve_strict=false` iken `login.defs` `UMASK` yazılmaz (0.12.0 ile sertleşmiş hostta mevcut `UMASK 027` geri alınmaz, bkz. paketin `docs/known-issues.md`). Bulgular gerçek VM turunda (V0120-S2-BULGU2/3/4) bulundu; 0.12.1 düzeltmeleri henüz gerçek VM'de denenmedi.
- **README, README.en ve `docs/hardening-kullanim.md` 3.6.20 / linux-hardening 0.12.0 durumuna getirildi:** kilit sürümü, gerçek VM'de denenenler (Ubuntu 26.04.1 ve Proxmox VE 9: sunucu türü ve zorunlu `allow_from` akışı, `apply_vars`, `pve_firewall_*`, fail2ban) ve denenmeyenler (etkin ban, yetkisiz ağ, küme/misafirli Proxmox, Raspberry Pi), güncelleme paneli ve değişiklik raporu. Yalnızca belge.
- **README test tablosu:** macOS 27.2'de elle denendi olarak işaretlendi: Terminal penceresi açma, `start.command` çift tıklama, `🛡️ Paketler` menüsünün fzf görünümü. Hâlâ denenmedi: ok tuşlu (fzf'siz) görünüm, gerçek sunucuda knock.

## [3.6.20] - 2026-11-02

Bu yama sürümü `linux-hardening` 0.12.0'ı kilitler ve sunucu eklerken platform seçimini, güncelleme panelini getirir.

### Dikkat
- **Mevcut sunucularda `linux-hardening` eylemleri ⛔ görünebilir:** 0.12.0 `preview`, `apply`, `pve_firewall_preview` ve `pve_firewall_apply` eylemlerini `hardening_ssh_allow_from` dolu olmadan çalıştırmaz (kemer ve pantolon: erişim kısıtını boş bırakıp kendinizi dışarıda bırakmayı önler). Paket menüsünü ilk açtığınızda sunucu türü ve izinli ağlar sorulur; "Şimdi değil" derseniz değer yazılmaz ve eylemler ⛔ kalır. `audit` ve `rollback_*` etkilenmez. Güncellemeden sonra `--components report` hangi sunucunun etkilendiğini gösterir.
- **Proxmox'ta `apply` artık `hardening_pve_apply=true`'yu o koşuya özel verir** (yalnızca sunucu türü `proxmox-host` seçiliyse, uyarı sunucu adı onayından önce gösterilir, `.vars` dosyanıza yazılmaz ve kendi yazdığınız değer ezilmez).
- **Gerçek VM (Ubuntu + Proxmox) doğrulaması henüz yapılmadı** (3.6.19'daki fail2ban maddesi dahil): yalnızca birim testleri, gerçek 0.12.0 manifestiyle yerel denemeler ve Docker LAB e2e (18/18) yeşil. Platform seçimi, `apply_vars` ve güncelleme paneli gerçek terminalde/sunucuda elle denenmedi.

### Düzeltildi
- **Sunucu eklerken CIDR editörü:** `ModuleGenerator.sh` `SSHModule.sh`'yi yüklemediği için sunucu bilgileri (IP/port/kullanıcı/anahtar) okunamıyor ve "_ssh_env_get: command not found" yazıyordu; ayrıca değişken türü sorgusu Python'a BrokenPipe izi bastırabiliyordu. İkisi de giderildi (test eklendi).

### Eklendi
- **Paket değişiklik raporu (güncelleme paneli, 3. adım):** güncellemede bir paketin sürümü değişince (`--components sync`, hem menü hem `--update` yolunda) sonda "📋 paket eski → yeni: ne değişti?" yazılır: yeni/kalkan değişkenler, türü ya da varsayılanı değişenler (örn. `hardening_fail2ban_bantime "1h" → "10m"`), yeni `requires_vars` kuralları, eklenen sunucu türü seçimi ve bu bilgisayardaki sunucularınıza etkisi ("web1: preview, apply ⛔ — değeri boş: …", "sunucu türü seçilmemiş"). Yalnızca okur, hiçbir şey yazmaz/değiştirmez; paket hiç ayarlanmamış sunucular atlanır. Eski sürüm, diskte duran bir önceki sürümdür (eski sürümler `prune` ile silinmeden önce). Elle: `--components report [paket [eski-sürüm]]`. Yeni: `Helpers/ComponentsReport.sh`.
- **Güncelleme paneli (`Update.sh`, bölgeli akış):** terminalde (en az 24 satır × 60 sütun) imza ve arşiv denetiminden sonra üstte sabit iki sütunlu özet (solda sürüm/imza/paket farkları, sağda `### Dikkat`), altta kaydırma bölgesinde sürüm notları, "Kurulsun mu?" sorusu ve kurulum akar; bölge her çıkışta (iptal, hata, Ctrl-C dahil) sıfırlanır. Özet üst bölüme sığmazsa (çok paket) tamamı kaydırma bölgesinin başında düz yazılır, bilgi kaybolmaz. Terminal küçükse ya da çıktı terminal değilse bugünkü düz akış aynen sürer. Güvenlik sırası (indir → imza → arşiv denetimi → özet → onay → kur) değişmedi. Ortak UI: `ui_region_begin2` artık sığmama durumunu `UI_REGION_TRUNC` ile bildirir.
- **Güncelleme özeti (`Update.sh`):** imza ve arşiv denetimi geçtikten sonra, sürüm notlarından ve "Kurulsun mu?" sorusundan ÖNCE tek tablo gösterilir: sshpocket sürümü (eski → yeni), imza durumu, `components.lock` paket farkları (değişen / yeni / kilitten çıkan / değişmeyenlerin sayısı) ve yeni sürümün CHANGELOG'undaki `### Dikkat` satırları. Salt gösterimdir, hiçbir şey yazmaz; güvenlik sırası (indir → imza → arşiv denetimi → özet) değişmedi. Sonraki adımlar: bölgeli akış, değişken farkı raporu.
- **Eski sunucular için ilk açılış sorusu:** paket platform listesi bildiriyor (`platform_choices` + `platform_var`) ama sunucunun değerler dosyasında platform yazılı değilse, paket menüsü açılırken (etkileşimli) sunucu türü ve `onboarding_vars` (örn. `allow_from`) bir kez sorulur — sunucu eklerken sorulanla aynı akış. "Şimdi değil" hiçbir şey yazmaz ve soru sonraki girişte yine çıkar; `allow_from` boş kalırsa `preview`/`apply` her seferinde ⛔ kalır. Platform listesi olmayan paketlerde (eski sürümler) hiçbir şey sorulmaz. Dolu değerler ezilmez.
- **Platform `apply_vars` uygulaması:** manifestte seçili platform (değerler dosyasında açıkça yazılı) `apply_vars` bildiriyorsa (örn. proxmox-host: `hardening_pve_apply=true`), değişken YALNIZCA değiştiren "önizle→uygula" eyleminde (type-server-name onaylı, `changes-config` etkili, yıkıcı olmayan, `requires` taşıyan: `apply`, `pve_firewall_apply`) o koşuya özel `-e` ile geçirilir. Paketin `apply_warnings` metni ve "bu koşuya özel" bilgisi sunucu adı onayından ÖNCE gösterilir; onay verilmezse hiçbir şey çalışmaz. Değerler dosyasına YAZILMAZ, dosyada zaten yazılı değer (false dahil) ve eylemin kendi `set`'i ezilmez; günlüğe `bu koşuya özel: …` satırı yazılır. `audit`, `rollback_*`, `kaldir*` ve `preview` etkilenmez; alan yoksa davranış değişmez. Riskli değer (`warn_if`) ise mevcut "yine de devam?" sorusu da çıkar.
- **`linux-hardening` 0.12.0 kilitlendi** (`components.lock`): manifestte `platform_choices` (generic / proxmox-host / raspberry-pi), `onboarding_vars`, `requires_vars` (`preview`, `apply`, `pve_firewall_preview`, `pve_firewall_apply` için `hardening_ssh_allow_from`) ve `apply_vars` (proxmox-host: `hardening_pve_apply=true`) bildirilir; roller/playbook davranışı değişmedi. Gerçek paket manifesti sshpocket doğrulayıcısından geçti, boş `allow_from` ile 4 eylem ⛔ (audit/rollback_status açık), sunucu ekleme akışında platform sorusu gerçek manifestle çalıştı. **Gerçek VM (Ubuntu + Proxmox) doğrulaması henüz yapılmadı** (akşam yapılacak); `apply_vars` uygulaması ve eski sunucuların ilk açılış sorusu sonraki PR'larda.
- **Eylem düzeyi `requires_vars` (isteğe bağlı manifest alanı):** bir eylem, listedeki değişken değerler dosyasında boş/yoksa ssh'a gitmeden "⛔ çalıştırılmadı: değeri boş değişken: …" ile reddedilir; menüde eylem kartında ⛔ ve "Boş: …" ipucu görünür. Yalnızca o eylemleri kapatır (`audit`, `rollback_*` etkilenmez); alan yoksa davranış değişmez. Amaç: erişim kısıtını (örn. `hardening_ssh_allow_from`) boş bırakıp kendini dışarıda bırakmayı önlemek.
- **Manifest: `platform_choices` / `onboarding_vars` okunur ve doğrulanır** (sunucu eklerken platform seçimi için hazırlık; seçim ekranı ve `apply_vars` uygulaması sonraki PR'larda). Alan yoksa davranış değişmez; bozuksa paket kilide yazılmaz. Tasarım: `docs/sunucu-ekleme-platform-secimi.md`.
- **Sunucu eklerken platform ve `allow_from` sorusu:** `ModuleGenerator.sh` (anahtar adımından sonra), kurulu ve manifestinde `platform_choices` olan her paket için sunucu türünü doğrudan sorar ("Bilmiyorum" = `generic`, "Şimdi değil" = hiçbir şey yazılmaz), seçilen platformun `defaults` değerlerini özetleyip onayla `servers/<ad>/<paket>.vars`'a yazar (dosyadaki değerler ezilmez) ve `onboarding_vars` için (örn. `list[cidr]` `allow_from`) mevcut CIDR editörünü açar; boş bırakılırsa değer yazılmaz ve ilgili eylemler ⛔ kalır. Manifest yoksa/alan yoksa hiçbir şey sorulmaz. `apply_vars` bu adımda YAZILMAZ (sonraki PR). Algılama (`platform_detect`) henüz yok; liste doğrudan sorulur. Yeni: `Helpers/ComponentsOnboard.sh`; `platform_var=` bilgi satırı.

### Değişti
- **Çalışma paneli iki sütunlu (fzf eylem menüsü, eylem çalışırken):** üst bölüm artık terminal genişliğinin tamamını kullanır: solda eylem özeti (ad, etkiler, onay, ön koşul), sağda son sonuç ve uyarılar; `│` ayırıcı ve fzf'e benzer kenarlık etiketleri (`Eylem`, `Son sonuç ve uyarılar`). Üst bölümün yüksekliği içeriğe göre ayarlanır (en az 9 satır, en çok terminalin %40'ı), çıktı bölgesi altta tam genişlikte kalır; çıktı çerçevesinin başlığı canlı ilerlemeyi göstermeye devam eder. `BECOME password:` ve diğer istemler alt bölgede aynen çalışır. Terminal < 24 satır ya da < 60 sütunsa eski akış kullanılır. Çizim başlangıçtaki genişliğe göre yapılır; çalışırken terminal yeniden boyutlandırılırsa üst bölüm yeniden çizilmez. UI parçası: `ui_region_begin2`, `ui_wrap_text` (`UI.sh`).

## [3.6.19] - 2026-11-01

Bu yama sürümü kullanıcıya görünür özellikler de taşır: paket menüsünde 🧪 Kontrol ve ön koşul önerisi, bulgular ekranı, canlı ilerleme, ortak arayüz parçaları ve `linux-hardening` 0.11.0.

### Dikkat
- **fail2ban davranışı değişti (`linux-hardening` 0.11.0):** `127.0.0.1/8 ::1` her zaman muaf; `hardening_fail2ban_ignoreip` varsayılanı boş ve yazdığınız adresleri EKLER. Kontrol makinesinin ve `hardening_ssh_allow_from`'un adresi artık otomatik muaf EDİLMEZ: bağlı adres muaf değilse art arda hatalı denemede (5) `bantime` (varsayılan `10m`, eskiden `1h`) kadar banlanabilirsiniz; günlükte tek satır `NOT (fail2ban)` çıkar. Yükseltmede jail dosyası bir kez yeniden yazılır (ilk koşu `changed` + fail2ban yeniden başlatması). **Gerçek VM doğrulaması (Ubuntu + Proxmox) henüz yapılmadı** (yalnızca birim/Ansible testleri ve Docker LAB e2e yeşil); ikinci apply'ın `changed=0` olması ve etkin ban davranışı sınanmadı.

### Eklendi
- **🧪 Kontrol (paket eylem menüsü):** pakette `preview` ve `audit` ikisi de bu sunucunun platformunda varsa listede yeni satır; ikisini art arda çalıştırır (önce `preview`, başarısızsa `audit` çalışmaz) ve sonunda tek özet yazar: apply'da değişecek sayısı, Lynis puanı, bulgular. Zorunlu değişken eksikse ⛔ ve hiçbir şey çalışmaz; `audit` kapıya/onaya takılırsa `preview` sonucu korunur ve özet nedenini söyler. Paket kodu değişmez, eylemlerin kendi kapıları ve onayları aynen geçerlidir. Menü satırları kayar: Geçmiş artık Kontrol'den sonra gelir.
- **Ön koşul eylemini önermek (etkileşimli menü):** `apply` gibi `requires` taşıyan bir eylem seçilince ön koşul (örn. `preview`) yok ya da süresi dolmuşsa menü "önce çalıştırılsın mı? [e/H]" diye sorar, çalıştırır, sonucu gösterir ve "devam edilsin mi?" diye ikinci kez sorar; eylemin kendi onayı (sunucu adını yazma) ayrıca aynen sorulur. Ön koşul başarısız olursa ya da "H" denirse eylem çalışmaz; girdi biterse (EOF) "hayır" sayılır. Komut satırı (`--components run`) ve kapı (`comp_gate_requires`) değişmedi: atlama değil, çalıştırma kolaylığı.
- **Denetim bulguları:** manifestte `outputs.findings` (düz `.tsv` dosya adı) bildiren paketin `bulgular.tsv` dosyası (10 sütun: kimlik, risk, tur, aciklama, ayrinti, oneri, alan, deger, kapanir_varsayilan, platform) ayrıştırılır (`component_findings.py`; geçersiz satır atlanır, en çok 2000 satır, başlık bozuksa gösterilmez). Paket menüsünde hazırlık özetinde sayı ve eşleşen bulgular, "🔎 Bulgular" ekranında önem sırasıyla liste ve kartlar (ayrıntı, öneri, paket karşılığı, "elle ayarlayın / apply yeterli"), "yalnız eşlemeli" süzgeci. Biçim: `docs/package-standard.md` "Bulgular".
- **Canlı ilerleme (fzf çalışma paneli):** paket `SSHPOCKET_PROGRESS` satırları yazıyorsa çıktı çerçevesinin başlığı çalışırken güncellenir ("Çıktı: apply — srv (çalışıyor) · aide 9/14 · bekleniyor ~300 sn"). Günlüğe saniyede bir bakan ayrı bir arka plan izleyici yapar; ana çıktı akışına dokunmaz (parola istemleri gecikmez), önceki koşunun günlüğünü yok sayar, panel bitince durur.
- **İlerleme satırı ayrıştırıcısı:** paketin `SSHPOCKET_PROGRESS alan=… sira=n/toplam durum=basladi|bitti [degisen atlanan kesin uyari bekle_tahmin]` satırları (`comp_progress_parse`) sonuç panelinde alan tablosu olur: uygulamada "değişen", önizlemede "değişecek" (+ toplam; `kesin=0` ise `~` tahmini), bitmeyen alan ⏳, uyarı sayısı. Satır yoksa panel eskisi gibidir; `SSHPOCKET_RESULT` ayrıştırıcısıyla çakışmaz. Biçim `docs/package-standard.md`'de.

### Düzeltildi
- **Python eklentili paketlerde ikinci koşu reddedilmesi:** `linux-hardening` 0.10.0'ın callback/filter eklentileri ilk koşuda kurulu paket klasörüne `__pycache__` yazıyor, bir sonraki eylem "kurulu paketin dosyaları değişmiş" bütünlük denetimine takılıyordu (gerçek Proxmox'ta ilk `audit` platform kapısında durduktan sonra ikinci deneme çalışmadı). Paket artık `PYTHONDONTWRITEBYTECODE=1` ile çalışır. Etkilenmiş kurulumda `__pycache__` klasörlerini silmek yeter.
- Test: `yürütme: TERM ve HUP` testinin sahte `ansible-playbook`'u sinyalden sonra çıktı borularını bırakır; yetim işlem `bats`'ı asmaz ve tam koşuda dalgalanmaz.
- **Aynı anda iki paket kurulumu** (örn. iki Terminal sekmesinde `--components sync`) artık `mv` yüzünden iç içe klasör (`0.9.1/linux-hardening-0.9.1`) üretmez: kurulum bölümü `mkdir` tabanlı kilitle (`<paket>/.install.lock`, sahibi ölmüşse temizlenir, doluysa ~10 sn bekler) korunur ve kilit altında "zaten kurulu mu" yeniden bakılır. Bozuk iç içe kurulumda eski davranış korunur: "kurulu paketin dosyaları değişmiş", elle silip yeniden `sync`.

### Değişti
- Paket eylem paneli kozmetiği: rozetler düz sembol (✎ / ☠; emoji genişliği sarmayı bozuyordu), kart uyarıları ve başlık özeti terminal genişliğine göre sözcük sınırından bölünür; "Geçmiş ve son sonuçlar" satırı son 5 çalıştırmayı ve son günlüğün sonunu sağ bölmede gösterir.
- **Ortak arayüz parçaları (`UI.sh`):** menülerdeki seçici (`ui_pick`), kaydırma bölgeli ekran (`ui_region_begin/end/reset`, `ui_rule_label`) ve genel liste editörü (`ui_edit_list`) paket menüsünden çıkarıldı; `comp_menu_edit_list` artık yalnızca CIDR/paket geri çağrımlarını verir. Davranış değişmedi. `UI_PICK_*` ve `UI_LIST_*` bağlam değişkenleri `docs/ui-design.md` §5b'de belgelendi (yeni paketler, örn. `utility`, aynı altyapıyı kullanır). Paketten bağımsız yeni testler.
- **Eylem paneli (fzf):** eylem listesinde hazırlık özeti başlıkta, seçili eylemin kartı (etkiler, onay, ön koşul, kapı durumu, son sonuç, uyarılar) sağ bölmede; eylem bitince sonuç paneli (durum, süre, `ok/changed/failed` özeti, sonuç anahtarları, günlük yolu, sıradaki adım). Terminal ≥ 24 satır ve 60 sütunsa çalışma sırasında üst bölüm sabit kalır, alt bölüm kaydırma bölgesi olur: onay soruları, `BECOME password:` istemi ve çıktı aynı ekranda akar, `Ctrl-C` bölgeyi sıfırlar. Küçük terminalde ve numaralı modda eski akış. Çok satırlı açıklama desteği eklendi.
- **Değişken düzenleme (etkileşimli modda, ok tuşlu/fzf):** `bool` ve `enum` için seçici liste (izinli değerler, imleç şimdikinin tersinde, sağda açıklama, riskli değer işaretli, "Varsayılana dön"); `list[string]`, `list[int]` ve `list[cidr]` için liste editörü (öğeler liste, Enter ile sil/değiştir, ➕ Ekle, CIDR için öneri ve sunucunun gördüğü adres, 🧹 Temizle, 💾 Kaydet; Vazgeç dosyayı değiştirmez); `int`/`string` yanlış değerde ekran kapanmadan 3 deneme. Kayıt sonrası "Devam için Enter" duraksaması yok, ileti değişken listesinde görünür. `describe` çıktısına 10. alan (enum değerleri) eklendi. Numaralı modun akışı değişmedi.
- **fzf geniş düzen (Değişkenler ekranı):** yol çubuğu kenarlık etiketi (`Sunucu › … › Değişkenler`), üstte grup ve sayı özeti, sağda açıklama bölmesi (fzf ≥ 0.35 etiketli kenarlık, ≥ 0.58 etiketli liste/giriş çerçeveleri; eski fzf'te yalnız sağ önizleme), `Ctrl-O` ile grup seçimi. fzf yoksa ya da `UI=numbers|arrows` ise bugünkü menüler aynen çalışır. Diğer ekranlar değişmedi.
- **`components.lock`: `linux-hardening` 0.10.0 → 0.11.0** (sha256 `5ed26848…7f0e`; 128 değişken, ekleme/silme yok). fail2ban `ignoreip` sadeleşti: `127.0.0.1/8 ::1` kodda her zaman muaf, `hardening_fail2ban_ignoreip` varsayılanı artık boş ve yazılan adresleri EKLER (ezmez); kontrol makinesinin ve `hardening_ssh_allow_from`'un adresi otomatik eklenmez, bağlı adres muaf değilse günlükte tek satır `NOT (fail2ban)` çıkar. `hardening_fail2ban_bantime` varsayılanı `1h` → `10m`. Yükseltmede önceden otomatik eklenen adres jail dosyasından düşer: ilk koşu `changed` ve bir fail2ban yeniden başlatması gösterir. Doğrulama: kilit kuru koşusu (sha256 + imza ad alanı) ve 128 değişken denetimi geçti; **fail2ban değişikliği gerçek VM'de (Ubuntu + Proxmox) henüz doğrulanmadı**: ikinci apply'ın `changed=0` olması ve etkin ban davranışı sınanmadı.
- **`components.lock`: `linux-hardening` 0.9.1 → 0.10.0** (sha256 `6921e52c…0c8b`). Paket artık `SSHPOCKET_PROGRESS` satırlarını (14 alan, AIDE'de `bekle_tahmin=300`) ve `audit` sonrası `bulgular.tsv` dosyasını (0600, `summary.json`'ın yanında) üretir; ikisi de Docker LAB'da gerçek koşuyla doğrulandı (preview, apply, audit: 22 bulgu = 20 öneri + 2 uyarı). Gerçek VM doğrulaması sürüyor.
- **`components.lock`: `linux-hardening` 0.9.0 → 0.9.1** (128 değişken, 11 eylem; 0.9.0 gerçek VM turunun bulgularını düzeltir: `kaldir` sahiplik testi, modprobe uyarı metni, `hardening_ssh_allow_from` artık `required` değil). Gerçek VM doğrulaması sürüyor.
- Sunucu adı onayı (`type-server-name`) yazım hatasında 3 deneme verir (her yanlışta `1/3`, `2/3` bildirimi); ad yine birebir yazılmalı, girdi bitince (EOF) hemen vazgeçilir.
- Ansible `Incorrect sudo password` / `Missing sudo password` ile düşerse sudo parolasının reddedildiğini ve komutun yeniden çalıştırılabileceğini söyleyen bir not gösterilir.

## [3.6.18] - 2026-10-30

`linux-hardening` 0.9.0 kilitlendi (alan anahtarları, kalıcı kaldırma eylemleri); `runner: script` sudo'lu yolda sonuç ayrıştırma (CRLF) ve `validate` script/Türkçe kurtarma adı düzeltmeleri.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.4 → 0.9.0** (128 değişken, 11 eylem; tüm değişkenler gruplu). Yeni: 13 alan anahtarı (`hardening_<alan>_enabled`, hepsi varsayılan açık), kalıcı kaldırma eylemleri `kaldir_onizle` (salt okunur, `SİLİNECEK:` satırları), `kaldir` (sunucu adıyla onay) ve `kaldir_geri_yukle`. Ubuntu'da 118 değişken ve 9 eylem, Proxmox'ta 128 değişken ve 11 eylem görünür. Gerçek VM doğrulaması sürüyor.
- `validate`: kurtarma eylemi adı artık adın ya da `_` sonrası sözcüğün başında `rollback|recover|restore|undo|geri` içeriyorsa tanınır (`kaldir_geri_yukle` gibi Türkçe adlar `--strict`'te yanlış uyarı vermez).

### Düzeltildi
- **`runner: script` sudo'lu yolda sonuç satırı ayrıştırılamıyordu:** `ssh -t` çıktıdaki satır sonlarını CRLF yapıyor, son anahtarın değeri `7\r` olarak okunup "Sonuç satırı geçersiz" deniyordu (eylem çalışıyor ama sonuç kaydedilmiyordu). Ayrıştırıcı artık `\r`'yi atar; gerçek makinede (ubuntutest) bulundu, regresyon testi eklendi.
- **`validate` script paketlerinde yanlış uyarı vermez:** sonuç satırı (`SSHPOCKET_RESULT …`) ve değişken kullanımı artık `scripts/` klasöründe de aranır; değişken betikte `SSHPOCKET_VAR_<AD>` olarak okunuyorsa "kullanılmıyor" uyarısı çıkmaz.

## [3.6.17] - 2026-10-29

`runner: script`: paketler Ansible yerine hedef sunucuda çalışan betiklerle eylem tanımlayabilir (sudo'lu yolda root'a ait kopya + SHA-256 doğrulama).

### Eklendi
- **`runner: script`:** paketler Ansible yerine `scripts/<ad>.sh` betikleriyle eylem tanımlayabilir. Betik **hedef sunucuda** çalışır (kontrol makinesinde yabancı kod çalışmaz): sudo gerekmiyorsa ssh stdin'inden akar; `requires_sudo` ise sunucuda 0600 geçici dosyaya yazılır, `ssh -t` ile tek sudo oturumunda **root'a ait 0700 klasöre kopyalanıp SHA-256'sı yerelde hesaplanan özetle doğrulanır** (uyuşmazsa çalışmaz; yükleme ile sudo arasındaki değiştirme yarışı kapalı) ve çalıştırılır; sudo parolası aynı terminalde sudo isteminde girilir, dosyalar her çıkışta silinir. Değişkenler `SSHPOCKET_VAR_<AD>` olarak tek tırnakla kaçırılarak geçer; sonuç `SSHPOCKET_RESULT <id>.<eylem> k=v` satırıyla gelir. Ortak kapılar (host key, ajan, profil, root reddi, önkoşul, onay) ansible ile aynıdır; Python ortamı gerekmez. Manifest doğrulayıcı ve `validate` betik yolunu/varlığını denetler. Standart §7 güncellendi.

## [3.6.16] - 2026-10-28

Değişkenler ekranında arama/grup/"değiştirdiklerim" süzgeci; saat sapması uyarısı iki saati, farkı, saat dilimini ve hazır düzeltme komutunu gösterir; `linux-hardening` 0.8.4 gerçek VM doğrulaması belgelendi.

### Değişti
- **Saat sapması uyarısı daha açıklayıcı** (`--components run`, 60 sn'den büyük fark): sunucu ve bu bilgisayarın saati (UTC), fark, sunucunun saat dilimi, NTP eşitleme durumu (`NTPSynchronized=no` ise kalıcı olmayabileceği), TOTP/2FA etkisi ve **kopyalanabilir hazır düzeltme komutu** gösterilir. Fark tam saat dilimi farkına benziyorsa ipucu verilir. Uzak komut hâlâ sabit ve salt okunurdur (`date +%s; timedatectl show -p Timezone -p NTPSynchronized`); gelen satırlar katı desenle okunur. Saati sshpocket düzeltmez (saat sıçrar, karar kullanıcıda); uyarı engellemez.

### Eklendi
- **Değişkenler ekranı: arama, grup ve "değiştirdiklerim" süzgeci.** Bir paketin 25'ten fazla değişkeni varsa liste başında `🔎 Ara`, `📂 Grup` (pakette `group` varsa) ve `✏️ Yalnızca değiştirdiklerim` satırları çıkar; arama ad ve açıklamada büyük/küçük harf ayırmadan çalışır, süzgeç açıkken `↩ Süzgeci temizle` görünür ve başlıkta `Gösterilen: X / Y · bu sunucuda değiştirilen: N` yazar. Dosyada değeri yazılı satırlar `✏️` ile işaretlenir; uzun değerler 40 karaktere kısaltılır (düzenleme ekranında tam değer görünür). Küçük paketlerde numaralandırma eskisi gibidir.

### Belgeler
- **`linux-hardening` 0.8.4 gerçek VM doğrulaması** (Ubuntu 26.04 ve Proxmox VE 9) README ve `docs/hardening-kullanim.md`'ye işlendi: platform süzmesi, altı sunucu başına anahtar (`false` yazmaz, önceden yazılanı silmez), `pve_keep_banner` (yeniden başlatma sonrası banner korunur), yeniden tasarlanan `pve_web_exclusive` (8006/3128 listede olmayan aynı alt ağ istemcisinde kapanır), ek güvenlik duvarı portu. `pve_web_exclusive` için eski "beklendiği gibi çalışmıyor" uyarısı kaldırıldı.

## [3.6.15] - 2026-10-28

Eylemlerde `platforms` (Ubuntu sunucuda `pve_firewall_*` görünmez); `linux-hardening` 0.8.4 kilitlendi; değişken açıklama sınırı 2000; sürüm yayını için `scripts/release-publish.sh`.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.3 → 0.8.4** (116 değişken, 8 eylem). Paket `platform_var: hardening_platform` ile 9 Proxmox değişkenini (`group: Proxmox`) ve `pve_firewall_preview/apply` eylemlerini yalnızca `proxmox-host` platformunda gösterir (Ubuntu'da 107 değişken, 6 eylem görünür; gizli değişkenin yazılı değeri yine çalıştırmaya iletilir). 6 yeni koşulsuz-görev anahtarı (varsayılan açık) ve `hardening_pve_keep_banner`. `pve_web_exclusive` yeniden tasarlandı. sshpocket: değişken açıklama sınırı 1000 → 2000 karakter (paketteki `pve_web_exclusive` açıklaması 1182 karakter). Gerçek VM doğrulaması sürüyor.

### Eklendi
- **Eylemlerde `platforms` (isteğe bağlı):** manifestte bir eyleme `"platforms": ["proxmox-host"]` yazılırsa eylem yalnızca o platformda listelenir; diğer sunucularda menüden gizlenir ("N eylem … gizlendi" notu) ve CLI'dan çalıştırılması ssh'a gitmeden reddedilir. Ön koşul tutarlılığı manifest doğrulamasında denetlenir. Eski sürümler alanı yok sayar. Örnek: Ubuntu sunucuda `pve_firewall_*` görünmez.
- **`scripts/release-publish.sh`:** sürüm yayınını tek komuta indirir. Ön kontroller (main, temiz, güncel, CHANGELOG başlığı, imza anahtarı, Release yok, `main` CI'ı bu commit için yeşil) → etiket → `release.sh` → `gh release create` (notlar CHANGELOG'dan) → yüklenen dosyaların SHA-256 doğrulaması. `--dry-run` yalnızca planı gösterir. Elle yol `RELEASING.md`'de yedek olarak durur.

## [3.6.14] - 2026-10-27

Sunucu menüsünde denetim puanı; değişkenler ekranı platforma göre süzülür ve gruplanır (isteğe bağlı); USB çıkarılırken anahtarlar `ssh-agent`'tan çıkarılır; eski paket sürümleri için temizleme sorusu ve Ayarlar menüsü; Ansible `become` bekleme süresi 30 sn; `warn_if` sorusu yalnızca değiştiren eylemlerde.

### Eklendi
- **Sunucu menüsünde denetim puanı:** 🛡️ Paketler › paket ekranında araç başına son denetim puanı ve bir öncekiyle fark gösterilir (`📊 Lynis: 83 (2 gün önce) · önceki 61 (+22)`). Yalnızca puan. Ortak `summary.json` biçimi (`tool`, `date`, `score`; eski `hardening_index` desteklenir) `docs/package-standard.md`'de tanımlandı; OpenSCAP gibi yeni bir araç eklemek sshpocket'te kod değişikliği istemez.
- **Değişkenler ekranı platforma göre süzülür ve gruplanır (isteğe bağlı manifest alanları):** üst düzey `platform_var`, değişkende `platforms` ve `group`. Ubuntu sunucuda Proxmox'a özgü değişkenler gizlenir, kaç değişkenin gizlendiği yazılır; gruplar `[Proxmox]` etiketiyle yan yana gelir. Gizlenen değişkenin yazılı değeri ve `required` kuralı etkilenmez; alanları olmayan paketler (bugünkü linux-hardening) eskisi gibi görünür. Paketin bu alanları kullanması ayrı bir iş (`docs/package-standard.md` §6).

### Değişti
- **Eski paket sürümleri artık hatırlatılıyor:** menüden güncelleme (ve paketlerin eşitlenmesi) bittikten sonra `prune` listeyi gösterir ve silinmesini sorar (varsayılan: silme); ayrıca **Ayarlar › 4) Eski paket sürümlerini temizle** menüsünden her zaman yapılabilir. Etkin, bir önceki ve kilitteki sürüm kalır.
- Sunucu saati farkı uyarısına paket seçeneği ipucu eklendi: `linux-hardening` için `hardening_time_makestep=true` (apply saati düzeltir, varsayılan kapalı). `docs/hardening-kullanim.md` ilgili bölüm genişletildi.
- **USB'yi menüden çıkarırken USB'deki anahtarlar `ssh-agent`'tan çıkarılır** (`EjectUSB`, çıkarmadan önce): USB'deki `servers/*/*.pub` dosyalarına karşılık gelen anahtarlar `ssh-add -d` ile silinir (parola sorulmaz); ajandaki başka anahtarlara dokunulmaz. Böylece çözülmüş anahtar bilgisayarın belleğinde kalmaz. USB menüden çıkarılmadan çekilirse çalışmaz.
- `--components run`: Ansible bağlantı/`become` bekleme süresi 10 sn'den **30 sn**'ye çıkarıldı (`ANSIBLE_TIMEOUT`, kullanıcının kendi değeri korunur). Gerçek Proxmox VM'inde zaman zaman görülen "Timeout (12s) waiting for privilege escalation prompt" için.
- **"Riskli değer" (`warn_if`) sorusu yalnızca yapılandırmayı değiştiren eylemlerde sorulur** (`apply` vb.). `preview`, durum eylemleri ve yalnızca araç kuran eylemler (`audit`) artık sormaz; eskiden `preview` ve `audit` her çalıştırmada, ardından `apply` yine sorduğu için aynı soru 2-3 kez çıkıyordu. Zorunlu değişken kuralı (`preview` dahil) aynı kaldı.

### Doğrulandı (belge)
- `linux-hardening 0.8.3`, gerçek VM'lerde (Ubuntu 26.04 ve tek düğümlü Proxmox VE 9, boş sunucular): varsayılanlarla Lynis Ubuntu 61→83, Proxmox 62→83 (84 `hidepid` ile); `kernel_modules_disabled` Ubuntu'da yeniden başlatma sonrası `ufw` açık, Proxmox'ta reddedilir; `fail2ban_pve_web`, `pveproxy_allow_from`, `postfix_banner`, `proc_hidepid`, `apt_listbugs` çalıştı. **Bilinen sorunlar:** `hardening_pve_web_exclusive` aynı alt ağdan 8006'yı kapatmıyor; Proxmox yeniden başlatmada `pvebanner.service` `/etc/issue` bandını eziyor; Proxmox'ta hidepid sonrası iki kez geçici "Timeout (12s) waiting for privilege escalation prompt". Denenmedi: misafir VM/CT'li Proxmox, Docker'lı VM, Raspberry Pi, HA kümesi.
- `docs/hardening-kullanim.md`: "Yeni seçenekler (0.8.x) ve gerçek VM notları" bölümü; README (TR/EN) kilit paragrafı güncellendi.

## [3.6.13] - 2026-10-26

`linux-hardening` 0.8.3 kilitlendi (7 yeni menü değişkeni; gerçek VM doğrulaması bu sürümden sonra).

### Değişti
- **`components.lock`: `linux-hardening` 0.8.2 → 0.8.3** (109 değişken, 8 eylem). Yeni (7): `hardening_postfix_banner` (varsayılan AÇIK, Proxmox'ta postfix banner'ı), `hardening_proc_hidepid` (`warn_if`) ve `hardening_proc_hidepid_users`, `hardening_apt_upgrade_phased` (`warn_if`), `hardening_apt_listbugs`, `hardening_pve_web_allow_from` ve `hardening_pve_web_exclusive` (`warn_if`). `hardening_pve_pveproxy_allow_from=[]` blok tek başınaysa artık dosyayı siler. Gerçek VM doğrulaması bu satırdan sonra yapılır.

## [3.6.12] - 2026-10-25

`linux-hardening` 0.8.2 kilitlendi (yalnızca Proxmox: `kernel_modules_disabled` reddi, yeni `fail2ban_pve_web` ve `pve_pveproxy_allow_from`; gerçek Proxmox VM doğrulaması bu sürümden sonra); paket öncesi sunucu saati farkı uyarısı.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.1 → 0.8.2** (102 değişken, 8 eylem; yalnızca Proxmox tarafı değişti). `hardening_kernel_modules_disabled` Proxmox VE hostunda artık reddedilir (veth/KVM/bridge/softdog yüklenemediği için LXC ve HA bozuluyordu; gerçek VM'de görüldü). Yeni: `hardening_fail2ban_pve_web` (varsayılan KAPALI, `warn_if`: web arayüzü 8006 için fail2ban jail'i) ve `hardening_pve_pveproxy_allow_from` (liste; 8006'yı yalnızca verilen CIDR'lere açar). Gerçek Proxmox VM doğrulaması bu satırdan sonra yapılır.

### Eklendi
- `--components run`: sunucu saati yerelden 60 sn'den fazla farklıysa (örn. snapshot geri alındıktan sonra) engellemeden uyarır; apt "Not live until" hatasına karşı `sudo chronyc makestep` önerir. Salt okunur sabit `date +%s` sorgusu; okunamazsa sessiz geçer.

## [3.6.11] - 2026-10-25

`linux-hardening` 0.8.1 kilitlendi (`kernel_modules_disabled` için modül ön yükleme düzeltmesi paket tarafında yapıldı; gerçek VM doğrulaması bu sürümden sonra); `validate` kurtarma ve araç kuran eylemlerde ön koşul uyarısı vermez.

### Değişti
- **`components.lock`: `linux-hardening` 0.8.0 → 0.8.1.** `hardening_kernel_modules_disabled` artık `/etc/modules-load.d/60-hardening-preload.conf` yazar (yüklü modüller + netfilter + `binfmt_misc`); 0.8.0'da yeniden başlatmada `ufw` açılmıyor, güvenlik duvarı sessizce kapalı kalıyordu. `verify_access` için `ssh-agent`'ta çok anahtar ipucu; `hardening_ssh_max_sessions` (1-10) ve `hardening_auditd_backlog_limit` (min 0) sınır taşır; `rollback_disarm` armed değilken `changed=0`.
- `validate`: "salt okunur preview ön koşulu yok" uyarısı artık yalnızca yapılandırma değiştiren eylemler için; kurtarma eylemleri (`rollback_*`, `recover*`, `restore*`, `undo*`) ve yalnızca `installs-package` etkisi olanlar (örn. `audit`) muaf. `linux-hardening` artık `--strict` altında temiz geçer.

## [3.6.10] - 2026-10-24

`linux-hardening` 0.8.0 kilitlendi (`pve_firewall_preview` artık hiçbir şey yazmaz); passphrase istemi açıklaması; sunucu silme sonrası gereksiz uyarı giderildi.

### Değişti
- Passphrase Keychain'e kaydedilirken (yeni sunucu, Keychain menüsü, yanlış passphrase düzeltme) macOS'un "password data for new item:" istemi öncesinde açıklanır: bu, SSH anahtarının PASSPHRASE'idir (sudo parolası değil), yazarken görünmez ve doğrulama için bir kez daha sorulabilir.
- **`components.lock`: `linux-hardening` 0.7.0 → 0.8.0** (100 değişken, 8 eylem). **`pve_firewall_preview` düzeltmesi:** 0.7.0'da bu eylem, sunucu değişkenlerinde `hardening_pve_apply=true` iken Proxmox'ta `cluster.fw`'yi GERÇEKTEN yazıyordu (gerçek Proxmox VM'inde görüldü); 0.8.0'da komut `pve-firewall-onizleme.yml --check` olur ve yazma kapısı değişkenle açılamaz. Yeni menü değişkenleri (18): varsayılan AÇIK `hardening_debsums` (+`_cron_check`), `hardening_apt_show_versions`, `hardening_process_accounting`, `hardening_fail2ban_jail_local`, `hardening_ufw_log_martians`; varsayılan KAPALI (çoğu `warn_if`) `hardening_apt_upgrade`, `hardening_kernel_modules_disabled`, `hardening_auditd_conf_cis`, `hardening_auditd_disk_full_halt`, `hardening_auditd_backlog_limit`, `hardening_auditd_immutable`, `hardening_aide_audit_tools`, `hardening_ssh_max_sessions`, `hardening_ufw_outgoing_deny`, `hardening_restrict_compilers`, `hardening_apparmor_enforce`, `hardening_malware_scanner`. Kapatınca GERİ ALINMAYANLAR: auditd.conf değerleri, GRUB satırı, derleyici izinleri, AppArmor kipi, ufw "ALLOW OUT" kuralları; `-e 2` ve `kernel_modules_disabled` yeniden başlatmaya kadar sürer.

### Düzeltildi
- Sunucu silme/ekleme sonrası menü yeniden yüklenirken, eski `sistem/Modules` klasörü olmayan kurulumlarda ekrana `Warning: Autoload path does not exist` yazılıyordu; klasör yoksa artık sessiz (`Main.sh` ile aynı koruma).

### Eklendi
- Docker LAB e2e: **Proxmox benzeri LAB** testi (`/etc/pve` + `pveversion` işaretleri, `hardening_pve_apply=true`): `pve_firewall_preview` hiçbir şey yazmaz, `pve-firewall restart` çalışmaz, güvenlik ağı arm edilmez, eylem komutunda `--check` var ve `-e/-i/--limit` yok. 0.7.0 kilidiyle elle denendi: kural dosyasını yazıyor (test bunu yakalar). `drive.exp` riskli değer sorusunu `E2E_RISKY` ile cevaplayabilir.

### Bilinen sorun
- ssh-agent'ında 3'ten fazla anahtar varsa, sertleştirme sonrası paketin "yeni SSH bağlantısı" doğrulaması `Too many authentication failures` ile düşer (sertleştirme `MaxAuthTries 3` yapar, doğrulayan `ssh` agent'taki tüm anahtarları dener). Güvenlik ağı sshd/ufw'yi geri alır ama paketler, sysctl, auditd, fail2ban ve AIDE kalır. Geçici çözüm: `ssh-add -D` (sshpocket gereken anahtarı yeniden ekler). Gerçek Ubuntu VM'inde 5 anahtarla görüldü.

## [3.6.9] - 2026-10-23

Ubuntu 26.04'te parolalı sudo ile paket çalıştırma düzeltildi (sudo-rs); README gerçek VM doğrulamalarını yansıtıyor.

### Doğrulandı (belge)
- `linux-hardening 0.7.0`, sshpocket 3.6.8 ile USB'den **taze Ubuntu 26.04.1 VM'inde**: `audit` (Lynis **58**) → `preview` (`changed=34`) → `apply` (`ok=168 changed=43 failed=0`; güvenlik ağı, ufw, ssh, yeni bağlantı doğrulaması, `/tmp` `noexec`) → bağımsız kontrol → **ikinci `preview` `changed=0`** (idempotent) → `audit` (Lynis **76**, öneri 46 → 23). Parolalı sudo-rs sorunu yeniden üretildi ve düzeltildi (aşağıda). Proxmox VE 9.2.21 VM'inde (0.6.0, `proxmox-host`): `apply` `ok=133 failed=0`, Lynis **82**, web arayüzü ve CT oluşturma çalışıyor. **Denenmedi:** gerçek Proxmox'ta `pve_firewall_preview`/`pve_firewall_apply`, Ubuntu'da `rollback_now`/`rollback_disarm` (yalnızca LAB), kümeli Proxmox ve gerçek donanım.

### Düzeltildi
- **Ubuntu 26.04: parolalı sudo ile paket çalıştırma** artık takılmıyor. Varsayılan `sudo-rs`, Ansible'ın parolalı `become`'ında `Timeout (12s) waiting for privilege escalation prompt` ile düşüyordu (gerçek VM'de yeniden üretildi: `UNREACHABLE`). Ubuntu profillerinde sunucuda `/usr/bin/sudo.ws` (klasik sudo) çalıştırılabilirse, envantere `ansible_become_exe=/usr/bin/sudo.ws` yazılır (salt okunur, sabit bir ssh sorgusuyla tespit edilir; envantere yalnızca tam bu yol yazılabilir). NOPASSWD sudo etkilenmez, Ubuntu dışı profillerde sorgu yapılmaz. Sudo istemi zaman aşımında nedeni açıklayan bir ipucu da gösterilir.

## [3.6.8] - 2026-10-23

Paket sürümü güncellemesi: `linux-hardening` 0.7.0 kilitlendi; Proxmox VE güvenlik duvarı için `pve_firewall_preview` ve `pve_firewall_apply` eylemleri.

### Değişti
- **`components.lock`: `linux-hardening` 0.6.0 → 0.7.0** (82 değişken; yeni: `hardening_auditd_cis_l2` (varsayılan KAPALI, `warn_if`), `hardening_motd_disable_scripts`). Yeni eylemler: **`pve_firewall_preview`** (salt okunur) ve **`pve_firewall_apply`** (`type-server-name`, önce önizleme şart): Proxmox VE (tek düğüm) datacenter güvenlik duvarını yönetir; Proxmox olmayan hostta hiçbir şey yapmadan nedenini söyleyip durur (LAB e2e ile doğrulandı). `pve_firewall_apply` yazma onayını playbook içinden (`pve-firewall-uygula.yml`) taşır; sshpocket'in `set` özelliğine gerek duymadı. 82 değişkenin tamamı için tür, 31 `warn_if` ve 30 min/max sınırı doğrulandı. **Gerçek Proxmox VM'inde `pve_firewall_*` henüz denenmedi.**

## [3.6.7] - 2026-10-23

Paket sözleşmesine eylem düzeyi `set` (sabit değişkenler) eklendi: aynı playbook'un önizleme ve gerçek koşu eylemleri kullanıcı ayarından bağımsız olur (Proxmox güvenlik duvarı eylemleri için ön koşul).

### Eklendi
- **Paket sözleşmesi: eylem düzeyi `set`** (docs/package-standard.md §5): bir eylem, bildirimde tanımlı bool/int/enum/string/path değişkenleri kullanıcı değerinin üstüne **sabitleyebilir**. Böylece aynı playbook'un önizleme ve gerçek koşu eylemleri (örn. Proxmox güvenlik duvarı için `pve_firewall_preview` → `hardening_pve_apply=false`, `pve_firewall_apply` → `true`) kullanıcının `.vars` ayarından bağımsız olur. Bildirim doğrulaması bilinmeyen/yanlış türlü/aralık dışı/liste değeri ve `outputs.dir_var` çakışmasını reddeder.

## [3.6.6] - 2026-10-22

Yeni, henüz değişkenleri tanımlanmamış bir sunucuda salt okunur durum eylemleri (`rollback_status`) artık hem komut satırında hem Paketler menüsünde çalışır.

### Düzeltildi
- Paket çalıştırma: **zorunlu değişken** (örn. `hardening_ssh_allow_from`) denetimi artık yalnızca değiştiren eylemlerde ve başka bir eylemin ön koşulu olan eylemlerde (`preview`) yapılır. Salt okunur durum eylemleri (`rollback_status`) yeni, henüz değişkenleri tanımlanmamış bir sunucuda da çalışır. Verilmiş geçersiz değer ve pakette tanımsız ad yine reddedilir; yalnızca "eksik" hoşgörülür. Sunucu menüsündeki 🛡️ Paketler ekranı da aynı kuralı izler: zorunlu değişken eksikken salt okunur durum eylemleri açık, `preview`/`apply`/`audit` ⛔ kapalı.

## [3.6.5] - 2026-10-22

Paket sürümü güncellemesi: `linux-hardening` 0.6.0 kilitlendi (Raspberry Pi / Proxmox VE platform desteği); `validate` ve paket bildirimi düzeltmeleri.

### Değişti
- **`components.lock`: `linux-hardening` 0.4.0 → 0.6.0** (80 değişken: 43 bool, 15 int, 10 string, 8 list[string], list[cidr], list[int], 2 path; Raspberry Pi/Proxmox VE platform desteği: `hardening_platform`). Docker LAB'da 16 e2e testin tamamı geçti; ikinci koşu `changed=0`; eski `ubuntu` grubu ve `INJECT_FACTS` uyarıları kayboldu. 80 değişkenin tamamı için tür, 30 `warn_if` ve 30 min/max sınırı doğrulandı. **Gerçek VM'de 0.6.0 sshpocket üzerinden denenmedi** (0.4.0 denendi).
- **Profil ve Proxmox:** paket 0.6.0 `debian-13`'ü destekler; Proxmox VE 9 ve Raspberry Pi OS de `debian-13` göründüğünden sshpocket'in profil kapısı artık Proxmox hostunu **geçirir**. Koruma paketin preflight'ındadır: Proxmox hostu `generic` platformla hiçbir bayrakla (`hardening_allow_unsupported` dahil) sertleştirilmez, `--check`'te de durur; gerçek değişiklik için `hardening_platform=proxmox-host` ve `hardening_pve_apply=true` gerekir. `docs/hardening-kullanim.md`'ye "Platformlar" bölümü eklendi.

### Düzeltildi
- `components validate` (dizin modu): gitignore'lu geliştirme klasörleri (`.venv`, `venv`, `.tox`, `.pytest_cache`, `.mypy_cache`, `.ruff_cache`) atlanır ve bildirilir; yazarın çalışma klasöründeki `.venv` yüzünden çıkan sahte ❌ (sembolik bağ, özel anahtar bloğu) giderildi. **Arşiv modu atlamaz**: yayın arşivinde bu klasörler varsa yine ❌.
- Paket bildirimi: değişken `description` sınırı 500 → 1000 karakter (linux-hardening 0.6.0'daki `hardening_platform` açıklaması 565 karakter olduğundan kilide yazılamıyordu). Kontrol karakterleri ve diğer metin alanlarının 500 sınırı aynı.


## [3.6.4] - 2026-10-20

`--components prune` eklendi; linux-hardening 0.4.0 gerçek VM'de, profil kapısı gerçek Proxmox VE hostunda denendi.

### Doğrulandı
- **Profil kapısı gerçek bir Proxmox VE 9 hostunda (Debian 13) denendi:** `--components run linux-hardening rollback_status <proxmox>` → `Profil 'debian-13' bu paketin profilleri arasında yok.`, Ansible başlamadı, sunucuya hiçbir şey yazılmadı. Ubuntu 26.04 için yazılmış sertleştirme Debian/Proxmox'a uygulanmaz. Not: Proxmox'a özgü bir platform denetimi yok (`/etc/pve`); engel "Debian desteklenmiyor" mantığına dayanır, Debian profili eklenmeden önce platform denetimi eklenmelidir.

### Eklendi
- **`./start.sh --components prune [--dry-run] [--yes]`:** paket güncellemelerinde yan yana biriken eski sürüm klasörlerini temizler. **Kalanlar:** etkin sürüm (`current`), bir önceki sürüm (geri dönüş için) ve kilitteki sürüm. Yalnızca `<X.Y.Z>` klasörü ve `<X.Y.Z>.files` silinir; `.bozuk-*` (kenara alınmış bozuk kurulum), sembolik bağlar ve geçersiz adlı klasörlere dokunulmaz. Silmeden önce onay ister (`--yes` ile atlanır); `--dry-run` yalnızca listeler. `current`'ı olmayan pakete dokunmaz.

### Doğrulandı (belge)
- `linux-hardening 0.4.0`, sshpocket 3.6.3 ile USB'den gerçek bir Ubuntu 26.04.1 VM'inde **taze kurulumda** denendi: `audit` (Lynis hardening_index 62, 0 uyarı) → `preview` → `apply` (ok=131 changed=38 failed=0). Hardening sonrası yeni SSH bağlantısı doğrulaması geçti, güvenlik ağı disarm edildi; ufw, sshd, sysctl (`kernel.sysrq=0`, `fs.protected_fifos=2`), `/etc/sudoers.d 0750`, `libpam-tmpdir`, `apt-listchanges`, AIDE zamanlayıcısı ve postfix yokluğu bağımsız kontrol edildi. `warn_if` onayı çıkmadı (linux-hardening#74 düzeltmesi gerçek akışta çalışıyor). Denenmedi: gerçek VM'de 0.3.0 üzerine ikinci koşu, `rollback_now`/`rollback_disarm`, menü ve Terminal açma.

## [3.6.3] - 2026-10-19

Paket sürümü güncellemesi: `linux-hardening` 0.4.0 kilitlendi.

### Değişti
- **`components.lock`: `linux-hardening` 0.3.0 → 0.4.0** (Lynis ölçümünden küçük maddeler, salt okunur raporlar: root ortamı, `/tmp` ve `/dev/shm`, AIDE zamanlayıcı; `su` kısıtı ve Docker `daemon.json` seçenekleri varsayılan KAPALI; değişken sayısı 27 → 38; `warn_if` apport/motd'dan kaldırıldı, linux-hardening#74 kapandı). Davranışı değiştiren yeni varsayılan-AÇIK ayarlar paketin CHANGELOG'unda (`kernel.sysrq=0`, `libpam-tmpdir`, `/etc/sudoers.d 0750` vb.). Docker LAB'da 16 e2e testin tamamı geçti. **Gerçek VM'de 0.4.0 sshpocket üzerinden denenmedi** (0.3.0 denendi).
- Docker LAB varsayılan değişkenlerinden `hardening_disable_apport=false` ve `hardening_disable_update_notifier_motd=false` kaldırıldı (artık gerekmiyor).

### Eklendi (belge ve test)
- README (TR/EN): **"İlk indirmeyi doğrulama"**: `--update` yalnızca sonraki sürümleri doğrular; ilk indirilen arşivi `ssh-keygen -Y verify` ile elle doğrulama tarifi ve anahtarın parmak izi. Bir test README'deki anahtar/parmak izinin `sistem/release_signers` ile aynı kaldığını denetler.
- Test: paket kurulumunda **güvenilen anahtarla ama yanlış ad alanıyla** (`sshpocket-release`) imzalı paket reddedilir; kilitte `signers` satırı boş/bozuksa hiçbir şey kurulmaz.

### Doğrulandı (belge)
- `linux-hardening 0.3.0`, sshpocket 3.6.2 ile USB'den gerçek bir Ubuntu 26.04.1 VM'inde denendi: `rollback_status` → `preview` → `apply` (ok=117 changed=35 failed=0); hardening sonrası yeni SSH bağlantısı doğrulaması geçti, güvenlik ağı disarm edildi; ufw/sshd/postfix durumu bağımsız kontrol edildi. Denenmedi: `audit`, `rollback_now`, `rollback_disarm` (yalnızca LAB), menü ve Terminal açma.

## [3.6.2] - 2026-10-18

Paket sürümü güncellemesi: `linux-hardening` 0.3.0 kilitlendi; paket çalıştırma ve `validate` yanlış alarmları düzeltildi.

### Değişti
- **`components.lock`: `linux-hardening` 0.2.0 → 0.3.0** (CIS Ubuntu 26.04 Level 1 eşlemesi; yeni roller `files`/`sudo`/`docker`; değişken sayısı 5 → 27). Ek: davranışı değiştiren yeni varsayılan-AÇIK ayarlar paketin CHANGELOG'unda ve README "Dikkat" tablosunda. Docker LAB'da 16 e2e testin tamamı 0.3.0 ile geçti. **Gerçek VM'de sshpocket üzerinden denenmedi.**
- Docker LAB varsayılan değişkenleri `hardening_disable_apport=false` ve `hardening_disable_update_notifier_motd=false` yazar (0.3.0'da varsayılanları `warn_if` ile aynı olduğundan her çalıştırmada onay istemi çıkıyor; syhgorath/linux-hardening#74).

### Düzeltildi
- Paket çalıştırma: `warn_if` ("riskli değer") onayı artık salt okunur durum eylemlerinde (örn. `rollback_status`) sorulmaz; yalnızca değiştiren eylemlerde ve ön koşul olan eylemlerde (`preview`) sorulur. Önceden her eylemde sorulduğu için varsayılanı zaten uyarı değerinde olan bir değişken (linux-hardening 0.3.0'da `hardening_disable_apport`) en zararsız eylemi bile onaya bağlıyordu.
- `components validate`: adında `password`/`token` geçen **bool ve int** değişkenler (örn. `hardening_password_inactive_days`, gün sayısı) artık "gizli değer izlenimi" uyarısı vermez; yanlış alarmdı (bool/int parola taşıyamaz). Metin alabilen türler (string, list) için uyarı aynen sürer.

### Test (geliştirici)
- Docker LAB e2e: `audit` (lynis kurulumu, rapor/özet 0600-0700 izinleri, `summary.json` içeriği), `rollback_now` (armed değilken etkisiz; armed iken sonraki ufw/sshd değişiklikleri geri alınır) ve `rollback_disarm` (armed ve yedek silinir) sshpocket üzerinden uçtan uca denendi. Üretim kodu değişmedi.

## [3.6.1] - 2026-10-17

Sunucuya ulaşılamayınca nedenini söyleyen ipuçları, paket yazarları için sözleşme uyum denetimi (`validate`) ve güvenilirlik düzeltmeleri.

### Eklendi
- **`./start.sh --components validate <kurulu-paket | dizin | arşiv.tar.gz> [--strict] [--unsigned]`:** paket SÖZLEŞME uyum denetimi (`docs/package-standard.md` §12'nin mekanik kısmı).
  Statik: ansible/ssh/ağ çalıştırmaz, paketten hiçbir şey çalıştırmaz; arşiv yalnızca güvenliyse açılır (`..`, mutlak yol, bağ, aygıt dosyası, aşırı boyut reddedilir; geçici klasör silinir).
  ❌: geçersiz `component.json`, `supported` profil yok, `VERSION` uyuşmazlığı, `requirements.txt` yok/hash'siz/sabit sürümsüz/URL, olmayan playbook, sembolik bağ, özel anahtar/gizli dosya.
  ⚠️: değiştiren eylemde `preview` ön koşulu/`warnings`/`requires_sudo` yok, `SSHPOCKET_RESULT` çıktısı bulunamadı, parola gibi değişken adı, kullanılmayan değişken, geri alma belgesi yok.
  `--strict` uyarıları hata sayar; `--unsigned` imzalamadan önceki denetimde `.sig` uyarısını atlar. Paket reposunun CI'ı `component_validate.py` + `component_manifest.py` ile bağımsız
  çalıştırabilir. Gerçek `linux-hardening 0.2.0`: 0 hata, 3 uyarı (kurtarma eylemleri ve `audit` bilerek ön koşulsuz). Yeni paket yazarları için şablon:
  [sshpocket-paket-sablonu](https://github.com/syhgorath/sshpocket-paket-sablonu).

### Değişti
- **Sunucuya ulaşılamayınca nedenini ve yapılacağı söyler:** işletim sistemi algılaması başarısız olduğunda ssh'ın hatası sınıflandırılır (`↳` ipucu satırı): zaman aşımı
  (VPN/ufw), rota yok, bağlantı reddi, **host key değişmiş**, host key tanınmıyor, anahtar reddi, çok fazla kimlik doğrulama denemesi, ad çözülemedi, kesilen bağlantı (fail2ban).
  Hata metni temizlenir ve kesilir, kabuğa verilmez. Tablo: `docs/hardening-kullanim.md`.

### Düzeltildi
- **Erişim ağı editörü üst üste binen ağları ayıklar:** listede `10.0.0.0/16` varken `10.0.1.0/24` eklenirse *"Zaten kapsanıyor"* denir ve eklenmez; daha geniş bir ağ
  eklenirse kapsadığı dar girdiler yerine geçer ve bu açıkça söylenir.
- **Terminal kapanınca (SIGHUP) geçici inventory/değişken dosyaları silinmiyordu:** yürütücü yalnızca Ctrl-C (INT) ve TERM'i yakalıyordu; HUP da yakalanır.

### Geliştirici
- **Docker LAB'ı (`test/e2e/lab/`) ve uçtan uca paket testleri:** systemd'li Ubuntu 26.04 konteyneri; `./test/e2e/lab/up.sh && bats test/e2e/components.bats`. `linux-hardening` akışını
  (`rollback_status` → taze `preview` olmadan `apply` reddi → `preview` (sunucu değişmez) → **geri alma güvenlik ağı** (A: doğrulama başarısız → önceki duruma döner; B: denetleyici kaybolur →
  sunucu zamanlayıcıyla kendini geri alır) → `apply` → **erişim ağı tuzağı** → erişilemeyen sunucu, tanınmayan ve değişmiş host key) gerçek ssh + ansible + ufw ile dener. Kullanıcının
  `known_hosts` ve sshpocket durumuna dokunmaz. `colima-trust-ca.sh`: antivirüs HTTPS'i yeniden imzalıyorsa CA'yı yalnızca Colima VM'ine ekler. Varsayılan `bats test` ve CI bunları çalıştırmaz.

## [3.6.0] - 2026-10-17

Paketler artık gerçekten kullanılabilir: ilk paket (`linux-hardening`) kilitli, imzalı ve gerçek bir Ubuntu 26.04 sunucusunda
`rollback_status` → `preview` → `apply` ile denendi. Kullanım ve **erişim ağı tuzağı**: [docs/hardening-kullanim.md](docs/hardening-kullanim.md).

### Eklendi
- **İlk gerçek paket kilitlendi:** `components.lock` artık `linux-hardening 0.2.0` (imzalı, paket standardı v1 uyumlu; sha256 `2886b610…f38a`) girdisini taşır.
- **`./start.sh --components run <paket> <eylem> <sunucu>`:** kurulu paketin bir eylemini sunucuda çalıştırır. Sunucu bilgisi `servers/<ad>/server.env`'den,
  paket değişkenleri `servers/<ad>/<paket>.vars`'tan okunur. İşletim sistemi profili sabit, salt okunur bir `ssh ... cat /etc/os-release` ile algılanır
  (host key doğrulaması kapatılmaz, parola sorulmaz; çıktı hiçbir zaman çalıştırılmaz). Python ortamı yoksa **onayla** kurulur (PyPI'dan ~10 MB, hash'li
  wheel; `~/.cache/sshpocket`). Sonuç `servers/<ad>/components.log` dosyasına (tarih|paket|sürüm|eylem|ok/fail, gizli bilgi yok) işlenir.
- **Çalıştırma öncesi kapılar:** değiştirici eylem root ile reddedilir; host key tanınmalı; anahtar ssh-agent'ta olmalı (gerekirse eklenir); işletim sistemi
  profili `supported` olmalı; `requires` eylemleri (örn. `apply` için `preview`) aynı sunucuda, aynı sürümde, son 30 dakikada başarıyla çalışmış olmalı;
  `confirm` (evet/hayır ya da sunucu adını yazma) ve `warn_if` onayları. Girdi yoksa onay reddedilir. Paket bu kuralları gevşetemez.
- **Güvenli yürütme:** `ansible-playbook` paketin venv'inden, paket kökünde, `ANSIBLE_HOST_KEY_CHECKING=True` zorlanarak çalışır; kabuk dizgesi/`eval` yoktur;
  geçici inventory/değişken dosyaları (izin 600) Ctrl-C/TERM dahil her çıkışta silinir; `outputs.dir_var` her zaman kullanıcı klasörüne yönlendirilir (hassas çıktı
  USB'ye yazılmaz); çıktı kullanıcı klasörüne (izin 600) günlüklenir. Eylemin `SSHPOCKET_RESULT` satırı önek, anahtar ve değer denetimiyle ayrıştırılır.
  `-i`/`--limit`/`-e` çalıştırıcıya aittir (pakette `cmd` içinde geçerse paket reddedilir).
- **Sunucu menüsünde `🛡️ Paketler`:** kurulu paket varsa paket → eylem seç (🔍 salt okunur / ✏️ değişiklik yapar / ☠️ yıkıcı; ön koşul durumu etikette). Ekranın
  başında **Hazırlık** bloğu: zorunlu değişken eksikse ❌ ve eylemler ⛔ kapalıdır.
- **`📝 Değişkenler` ekranı:** değer/varsayılan/otomatik gösterimi, kaydetmeden önce doğrulama (çalıştırmadakiyle aynı doğrulayıcı), `warn_if` onayı, `-` ile silme.
  `list[cidr]` değişkenler (örn. `hardening_ssh_allow_from`) için **ağ editörü**: bu bilgisayarın adresinden öneri (ağ / tahmini /24 / yalnızca bu adres), isteğe
  bağlı sunucudan sorgu (sabit, salt okunur ssh), elle ekleme/silme/temizleme; listede tek ağ varsa *"başka bir ağdan bağlanırsanız SSH kapalı olur"* uyarısı.
  Hiçbir şey otomatik yazılmaz.
- **`📜 Geçmiş ve son sonuçlar`:** bu sunucudaki son çalıştırmalar, ön koşulların tazeliği (*apply için preview: ✅ taze / ⏳ süresi doldu / ❌ kayıt yok*) ve son
  çalıştırma günlüğünün son satırları. Durum eylemlerinin (örn. `rollback_status`) son sonucu saklanır ve Hazırlık bloğunda gösterilir.
- **Bildirim doğrulayıcısı `outputs` alanını okur:** paketteki `prefix`, sshpocket'in türettiğiyle (`SSHPOCKET_RESULT <paket>.<eylem>`) uyuşmazsa paket reddedilir;
  uyarı metinleri menüde gösterilir. Karar kayıtları: `docs/package-standard.md` §14.

### Değişti
- **Sudo parolası istemi açıklanıyor:** eylem `--ask-become-pass` ile çalışacaksa, Ansible'ın `BECOME password:` istemi öncesinde bunun sunucudaki sudo parolası
  olduğu, sudo parola istemiyorsa (NOPASSWD) yalnızca Enter'a basılacağı ve parolanın sshpocket'e/günlüğe yazılmadığı belirtilir.
- **Belgeler:** `docs/hardening-kullanim.md` (akış, erişim ağı tuzağı, geri alma); README'de Apple Silicon (M5 Pro, macOS 27) test satırı, güncel Python ve paketler
  maddeleri (TR/EN).

### Düzeltildi
- **`components/` (kurulu paketler) `.gitignore`'a eklendi:** `--components sync` ile açılan paket klasörü yanlışlıkla commit'lenebiliyordu.

## [3.5.2] - 2026-10-16

### Düzeltildi
- **Keychain'deki passphrase yanlışken bağlantı ekranı sonsuza kadar takılıyordu:** `ssh-add`, askpass boş cevap verene kadar yeniden dener; askpass her seferinde
  aynı yanlış passphrase'i döndürdüğü için "SSH key agent'a ekleniyor" satırında %100 CPU ile sonsuz döngü oluşuyordu. `askpass.sh` artık yalnızca ilk
  çağrıda passphrase döndürür (`ASKPASS_STATE`), sonrakilerde boş satır verir; `ssh-add` birkaç saniyede vazgeçer.

### Eklendi
- **Yanlış kayıtlı passphrase'te yeniden sorma:** key agent'a eklenemezse (agent'a ulaşılabiliyorsa) "Kayıtlı passphrase yanlış olabilir. Yeniden girip
  Keychain'e kaydedeyim mi? [e/H]" diye sorulur; `e` ise passphrase güvenli istemle (komut satırında görünmez) Keychain'e yazılır ve en çok 3 kez denenir.
  Girdi yoksa takılmaz.

## [3.5.1] - 2026-10-12

### Değişti
- **Belgeler güncellendi:** README'ye **Gereksinimler** bölümü (hangi araçlar macOS ile gelir; fzf ve Python isteğe bağlı); eskimiş `.env` anlatımı
  (`servers/<ad>/server.env`, `sshpocket.conf`); tekrarlanan menü/fzf maddeleri birleştirildi; knock istemci tarafı ve `UI` değerleri anlatıldı;
  `docs/ui-design.md` uygulama durumuyla güncellendi.

### Düzeltildi
- **Ok tuşlu menüde listenin satırları her tuşta çoğalıyordu (3.5.0):** menü açıklamaları eklenince, terminal genişliğinden uzun açıklama bir alt satıra
  sarılıyor, imleci geri taşıma sayısı kayıyor ve her ↑/↓'de üst satır kopyalanıyordu. Çizim sırasında otomatik sarma kapatılır (uzun satırlar
  sağ kenarda kırpılır), çıkışta ve Ctrl-C'de yeniden açılır. 60/80/100 sütunlu gerçek pty testi eklendi.
- **fzf, Finder'dan açılınca (start.command) bulunamıyordu:** PATH'te yoksa `/opt/homebrew/bin` ve `/usr/local/bin` da aranır.

## [3.5.0] - 2026-10-12

### Düzeltildi
- **`check-new` ön sürümleri (pre-release) göremiyordu:** depoda kararlı "latest" yoksa (linux-hardening şu an yalnızca ön sürümlü) GitHub API'deki en yeni
  yayına bakılır (yine yalnızca bilgi; kurulmaz).
- **`--doctor` yeni Mac'te "geliştirici araçlarını kurun" penceresini açabiliyordu:** macOS'ta Command Line Tools yoksa `/usr/bin/python3` gerçek
  Python değil bir saplamadır ve çalıştırılınca kurulum penceresi açar. Saplama artık çalıştırılmadan atlanır ve doktor nedenini söyler.
- **Menüden "Geri" ile dönünce gereksiz "Devam etmek için Enter" soruluyordu:** sunucu menüsü, ⚙️ Ayarlar ve Sunucuları yönet'ten geri dönünce
  artık doğrudan ana menüye dönülür. Okunacak çıktısı olan işlemlerden (bağlanma sonucu, ekleme, silme, güncelleme) sonra duraklama korunur.

### Eklendi
- **Bakımcı aracı `scripts/lock-component.sh`:** bir paket sürümünü `components.lock`'a güvenle yazar: paketi ve imzayı HTTPS ile indirir, arşivi denetler,
  imzayı (`sshpocket-component`) doğrular, `component.json`'u şemaya göre doğrular (kurulamayacak paket kilide **yazılmaz**), sha256'yı hesaplar.
  `--write` yoksa yalnızca gösterir. `comp_get` pipefail altında 141 ile düşmesin diye tüm girdiyi okur.
- **CI'da sabit sürüm ShellCheck:** `scripts/ci-install-shellcheck.sh` (v0.11.0, indirilen arşivin sha256'sı doğrulanmazsa kurulmaz); yerel ve CI
  aynı sürümü kullanır, "yerelde yeşil, CI'da kırmızı" farkı kapanır.
- **fzf desteği (isteğe bağlı):** sistemde `fzf` kuruluysa (`brew install fzf`) tüm seçim menüleri fzf ile açılır: yazarak arama, **önizleme penceresinde
  seçili seçeneğin açıklaması**, `←`/Esc geri. fzf yoksa ya da terminal değilse (boru/test) kendi ok tuşlu seçicimiz, o da yoksa numaralı menü kullanılır.
  Açıklama metni fzf komut satırına girmez (dosyadan okunur). ⚙️ Ayarlar → menü stili: `auto` | `fzf` | `arrows` | `numbers`.
- **Menü açıklamaları:** ana menü, sunucu menüsü, Ayarlar, Sunucuları yönet ve Keychain menülerinde her seçeneğin kısa açıklaması; fzf yoksa ok tuşlu
  seçicinin alt satırında gösterilir. `./start.sh --doctor` fzf'i isteğe bağlı olarak denetler (kurmaz).

### Düzeltildi
- **Sunucu silinince Keychain'deki knock dizisi yetim kalıyordu:** artık sunucuyla birlikte silinir; yeniden adlandırmada eski adla duran knock
  dizisi için "Knock ayarları ile yeniden girin" uyarısı gösterilir.
- **Knock ayarları açıklamasız ve bağışlamazdı:** tek yanlış girişte "Geçersiz protokol" deyip menüden çıkıyordu. Artık knocking'in ne olduğu, her
  alanın (protokol, bekleme, dizi) ne anlama geldiği ve örnek değerler gösterilir; yanlış girişte **neden** geçersiz olduğu söylenip yeniden sorulur
  (3 deneme), büyük/küçük harf fark etmez, Enter = varsayılan. macOS'un diziyi iki kez isteyebileceği belirtilir.

## [3.4.0] - 2026-10-12

### Eklendi
- **Klasik port knocking (istemci tarafı):** sunucu menüsüne **5) Knock ayarları**. `server.env`'e `KNOCK=keychain`, `KNOCK_PROTO` (tcp|udp),
  `KNOCK_DELAY_MS`; **knock dizisi düz metin dosyada tutulmaz**, Keychain'de (`USBMonitor_<ÖNEK>_Knock`). Bağlan ve key gönderme, bağlantıdan
  önce diziyi otomatik gönderir (bash `/dev/tcp|udp`: portlar hiçbir sürecin komut satırında görünmez; takılmaz, en çok ~1 sn/vuruş).
  Durum göstergesi knock **göndermez**; knock'lı sunucuda port kapalıysa **🔒** gösterir. Keychain/ayar bozuksa bağlantı denenmez.
  Sunucu tarafı (knockd) ayrı bir pakettir; bu sürümde yoktur. Klasik knocking şifreli değildir (taramalardan saklanma katmanıdır).

## [3.3.0] - 2026-10-12

> **Paket altyapısı geldi.** `components.lock` şimdilik **boştur**; ilk paket (linux-hardening) hazır olunca eklenecek. Altyapı, paket kurulumunu
> imzalı, kilitli ve doğrulanabilir yapar (bkz. [docs/package-standard.md](docs/package-standard.md)). Mevcut sunucu/bağlantı davranışı değişmedi.

### Eklendi
- **Paket kilit dosyası ve imza doğrulama (Aşama 2.0):** `components.lock` (şimdilik boş) ve `sistem/Helpers/Components.sh`: katı ayrıştırıcı
  (bilinmeyen/yinelenen anahtar, bozuk değer = hata; dosya `source` edilmez), arşiv sha256'sı kilitle aynı olmalı, imza `sshpocket-component`
  ad alanıyla doğrulanır (çekirdek `sshpocket-release` imzası yeniden oynatılamaz). Kilitte olmayan paket reddedilir.
- **Paketin güvenli kurulumu (Aşama 2.1):** `sistem/Helpers/ComponentsInstall.sh`: HTTPS indirme (boyut sınırlı), arşiv denetimi (`..`, mutlak yol,
  farklı kök, symlink/hardlink/aygıt reddi), `component.json` kimlik+sürüm eşleşmesi, `components/<id>/<sürüm>/` + `current` (düz metin),
  eski sürüm kalır, geri sürüme düşülmez, kurulum özetleri her çalıştırmadan önce yeniden doğrulanabilir.
- **Bildirim okuma ve sözleşme doğrulama (Aşama 2.2):** `component.json` (`python3 -I` + stdlib `json`) tam şema denetimi:
  `sshpocket_api`, `runner`, profiller, eylemler (`cmd`'de kabuk meta karakteri yok, `effects`, `requires` döngüsüz), değişkenler, `invoke.append`;
  bilinmeyen zorunlu özellik ve daha yeni `sshpocket_api` reddedilir. Etkin `confirm`: paket yalnızca sıkılaştırabilir (`destructive` →
  sunucu adı yazdırma). `comp_prepare` kurulu paketi bütünlükle doğrular ve bilgiyi verir.
- **`./start.sh --doctor` ve venv önbelleği (Aşama 2.3):** `--doctor` ön koşulları denetler (ssh araçları, `nc`, Keychain, Python aralığı,
  `venv`/`ensurepip`, disk, ağ); **yalnızca okur ve çözüm komutunu gösterir, Homebrew/pip/sudo çalıştırmaz.** Paket bağımlılıkları için
  `ComponentsVenv.sh`: aralığa uyan Python seçimi, yalnızca hash'li `requirements.txt` (`-r/-e/--index-url` reddedilir),
  `~/.cache/sshpocket/venv/<paket>-<ortam-kimliği>` önbelleği, `pip --require-hashes --only-binary :all:` yalnızca venv içine; aynı ortam
  kimliğinde sağlıklı venv varsa hiçbir şey kurulmaz.
- **Paket yönetimi komutları (Aşama 2.4a):** `./start.sh --components list` (kilit ↔ kurulu durumu, ağsız), `sync [--yes]` (eksik/eski paketi
  kilitteki sürümle, imza doğrulayarak, onayla kurar; bozulmuş kurulumu silmeden `<sürüm>.bozuk-<tarih>` olarak kenara alıp yeniden kurar),
  `check-new` (kilitten daha yeni yayın var mı: **yalnızca bilgi**, kurulmaz; yeni sürüm yeni sshpocket sürümüyle gelir).
  `--update` başarılıysa **yeni kodla** paketler kilide göre eşitlenir; menüde kilitte paket varsa **📦 Paketler** girdisi görünür.

### Değişti
- Ok tuşlu seçim artık ⚙️ Ayarlar, **Sunucuları yönet** ve Keychain passphrase menülerinde de çalışır (başlıkla birlikte); `UI=numbers` ile
  hepsi numaralıya döner.

## [3.2.0] - 2026-10-11

### Eklendi
- **Ekran cilası (1/2):** her ekranda başlık + ekmek kırıntısı (`Ana menü › ubuntutest`) ve alttaki tuş ipucu satırı; sunucu satırlarında
  `kullanıcı@ip[:port]` özeti. Renk yalnızca terminalde ve `NO_COLOR` tanımsızsa; boru/test çıktısı düz metindir.
- **Ok tuşlu seçim (ekran cilası 2/2):** ana menü ve sunucu menüsünde ↑↓ (veya `j`/`k`) ile gez, Enter ile aç, `←`/`q`/`0`/Esc ile geri.
  Numara yazıp Enter de çalışır (iki haneli dahil, Backspace ile düzeltilir). Saf bash; terminal değilse (boru, betik, test) eski
  numaralı menü kullanılır. ⚙️ Ayarlar'a `UI` eklendi (`auto` | `numbers` | `arrows`): ok tuşları sorun çıkarırsa `numbers` yapın
  (ya da `SSHPOCKET_UI=numbers`). İmleç her çıkışta (Ctrl-C dahil) geri açılır.

## [3.1.1] - 2026-10-11

### Düzeltildi
- **Ana menüye dönerken takılma:** macOS `nc -w` bağlantı beklemesini sınırlamadığından kapalı/erişilemeyen sunucuda durum kontrolü
  ~75 sn sürebiliyordu (durum göstergesi 3.1.0'da varsayılan açık olunca belirgin oldu). Kontrol artık süre dolunca öldürülür
  (⚙️ Ayarlar'daki süre, 1-5 sn); bağlanmadan önceki port kontrolü en çok 5 sn bekler.

## [3.1.0] - 2026-10-11

### Eklendi
- **⚙️ Ayarlar** (ana menü): `sshpocket.conf` (kökte, gitignore'lu; güncellemede korunur). Şimdilik iki ayar: durum göstergesi (`STATUS`) ve
  durum kontrol süresi (`STATUS_TIMEOUT`, 1-5 sn). Öncelik: ortam değişkeni (`SSHPOCKET_STATUS` vb.) > dosya > varsayılan.
  Dosya `source` edilmez; yalnızca bilinen anahtarlar okunur, değerler doğrulanır.

### Değişti
- **Durum göstergesi (🟢/🔴/⚪) artık varsayılan AÇIK** (Finder'dan `start.command` ile açınca da görünür). Kapatmak için ⚙️ Ayarlar
  veya `SSHPOCKET_STATUS=0`.
- **`q` = geri/çıkış:** ana menüde `q` çıkış yapar (0 ile aynı); alt menülerde `0) Geri (q)` ipucu gösterilir.

## [3.0.0] - 2026-10-11

> **Veri düzeni değişti (büyük sürüm).** Sunucular artık `servers/<ad>/server.env` altında; eski `sistem/Modules/<ad>/` sunucularınız
> **otomatik taşınır** (`./start.sh --migrate`, önce `--dry-run`). Eski sürümler yeni düzeni okuyamaz; geri dönmek isterseniz
> `servers/.migrated-<tarih>/` altındaki eski klasörleri `sistem/Modules/` altına geri koyun. Keychain'deki passphrase'ler aynen geçerlidir.

### Eklendi
- **Aşama 1.0/1.1 (`servers/` veri katmanı, geriye uyumlu):** menü artık `servers/<ad>/server.env` (genel anahtarlar `IP`, `USER`, `PORT`, `KEY`, `OS`)
  klasörlerinden de kurulur; sunucu başına üretilmiş bash dosyası gerekmez. **Eski `sistem/Modules/<ad>/.env` biçimi aynen çalışmaya devam eder**
  (taşıma sonraki adımdır). Tek erişim katmanı `srv_get/srv_set` biçimi korur. Aynı adlı eski modül varsa yeni biçim kazanır.
- `servers/` içindeki hiçbir dosya `source` edilmez (güvenlik testleri); gizli klasörler, `._*` ve geçersiz adlar yok sayılır.
- Güncelleyici `servers/` klasörüne asla dokunmaz (arşivde olsa bile); release paketine `servers/` dahil edilmez.
- **Aşama 1.2 (taşıma):** `./start.sh --migrate [--dry-run] [--yes]` ve etkileşimli açılışta (yalnızca terminalde, onaysız asla) teklif.
  Eski `sistem/Modules/<ad>/` → `servers/<ad>/`: ayarlar genel anahtarlara çevrilir, key'ler kopyalanır ve **bayt bayt doğrulanır**,
  izinler 600/700, üretilmiş modül dosyaları taşınmaz. **Hiçbir şey silinmez:** eski klasörler `servers/.migrated-<tarih>/` altına alınır;
  hata olursa o sunucunun eski klasörü olduğu gibi kalır, yarım klasör bırakılmaz. İdempotent. Keychain'e dokunulmaz (passphrase'ler aynen geçerli).
- **Aşama 1.3 (yeni biçime yazma):** `ModuleGenerator.sh`, `ImportSSHConfig.sh` ve menüdeki "Yeni sunucu ekle / içe aktar" artık
  `servers/<ad>/server.env` yazar (genel anahtarlar; **sunucu başına üretilmiş bash dosyası yok**, `.env.example` yok).
  `ModuleManager.sh`: `list` yeni ve eski biçimi (etiketle), `remove` ikisini de, `rename` yeni biçimi (klasör + key dosyaları; eski
  biçim için "önce `--migrate`" der) yönetir. Aynı ad hem `servers/` hem eski `Modules/`'te varsa oluşturma/içe aktarma reddedilir.
- **CI'a macOS işi:** `macos-latest` (Apple Silicon arm64, macOS 26.6.2) üzerinde macOS'un kendi bash 3.2'siyle ShellCheck + tüm testler. Bu sırada bulunan
  bir test aracı sorunu (bash 3.2 + UTF-8 yereli, Türkçe test adlarını yanlış kodluyor) `LC_ALL=C` ile giderildi. Keychain izin penceresi ve Terminal
  açma otomatik sınanmaz (testler sahte `security` kullanır).
- **Aşama 1.4 (sadeleşme):** gönderilen `sistem/Modules/example/` modülü kaldırıldı; yerine `sistem/templates/server.env.example` şablonu geldi
  (menüde artık boş bir "EXAMPLE" girdisi çıkmaz; sunucu eklemek için "Yeni sunucu ekle"). Güncelleyici sadeleşti: kod (`sistem/*.sh`, `Helpers`,
  `templates`) bütünüyle değişir; **`sistem/Modules/` (henüz taşınmamış eski sunucular) ve `servers/` asla silinmez/ezilmez**. Eski sürümlerden kalan,
  hiç yapılandırılmamış `example` şablonu güncellemede güvenle temizlenir (içinde `.env`/key/başka dosya varsa dokunulmaz). Taşıma, `.env`'i olan
  `example`'ı da (gerçek sunucu olarak) taşır. Eski `Modules/` klasörü yoksa artık uyarı verilmez.

### Belge
- Tasarım kararları işlendi: durum göstergesi varsayılan açık, `⚙️ Ayarlar` paneli (5 ayar, `sshpocket.conf`), `q` geri, fzf USB'ye gömülür (yedekli), klasik port knocking için pre-connect kancası (`docs/ui-design.md`, `docs/components-design.md` §6.7).
- `docs/ui-design.md`: arayüz ilkeleri, fzf dağıtım politikası, Apple Silicon doğrulama listesi ve teknoloji alternatifleri (tasarım, kod yok).
- README: "Test edilen ortamlar" tablosu. Yalnızca Intel + macOS 15.7.9'da denendi; Apple Silicon ve daha yeni macOS **denenmedi**.

### Belge
- `docs/components-design.md`: `servers/` veri ayrımı ve bileşen modeli için tasarım (henüz uygulanmadı); linux-hardening PR sırası incelemeyle güncellendi (A → 0a → 0b → 0c → B → D → C).
- Tasarım belgesi, linux-hardening'in gerçek `component.yml`'ine hizalandı (issue #2): `invoke.append` nesne listesi, `inventory_group`, `outputs`, `hardening_scan_dir`, principal=ad alanı.
- Tasarım belgesi, linux-hardening v0.1.0'ın geri alma güvenlik ağına hizalandı (issue #3): `rollback_status/disarm/now`, `hardening_rollback_*` değişkenleri, arayüz notları.

### Eklendi
- Dependabot yapılandırması (aylık, yalnızca GitHub Actions bağımlılıkları).

## [2.2.0] - 2026-10-08

### Eklendi
- **İmzalı güncelleme:** `./start.sh --update` / `--check-update` ve menüde "Güncelle". Yalnızca bakımcının imza
  anahtarıyla imzalı (`ssh-keygen -Y verify`), HTTPS ile inen Release paketini kurar; imza yoksa/anahtar tanımlı
  değilse kapalıdır. Geri sürüme düşmez, tehlikeli arşivleri (`..`, mutlak yol, symlink) reddeder, kullanıcı
  verisini korur, yedek alır ve hata olursa geri yükler.
- `scripts/release.sh` (imzalı paket üretir ve yayından önce doğrular) ve `RELEASING.md` (bakımcı rehberi).
- `.gitattributes` ile release paketine `test/`, `scripts/`, `.github/` girmez.
- Depo hijyeni testleri: `release_signers` biçimi ve repoda özel anahtar/sır dosyası bulunmaması.

> Güncelleyici bu sürümle gelir; v2.1.0 ve öncesi kopyalar bir kez elle güncellenmelidir.

## [2.1.0] - 2026-10-08

> İlk yayın. `2.0.0` hiç yayınlanmadığı için bu sürüm, aşağıdaki `2.0.0` bölümündeki her şeyi de içerir.

### Eklendi
- Sürüm tek yerde (`VERSION`); `sistem/Main.sh --version` / `./start.sh --version` sürümü yazar.
- Uçtan uca testler (`test/e2e/`): Docker'da yerel Ubuntu `sshd` ile `ssh-copy-id`, elle ekleme komutu, root reddi ve durum göstergesi gerçek sunucuya karşı denenir.
- `ssh-copy-id` başarısız olursa olası nedenler ve sunucuda çalıştırılacak **tek satırlık elle ekleme komutu** gösterilir
  (komut tekrar çalıştırılabilir, mevcut key'leri korur). Kullanıcı `root` ise Ubuntu/Debian uyarısı verilir.
- Modül menüsünde **Bilgileri güncelle** (IP/kullanıcı/port/key yolu) ve **Keychain passphrase'ini güncelle/sil**.
  Passphrase kaydedildikten sonra key açılarak doğrulanır. Ayar bozuk veya key eksikken de bu seçenekler çalışır.
- Ana menüde yerleşik eylemler: yeni sunucu ekle, `~/.ssh/config`'ten içe aktar, sunucuları yönet.
  Eklenen/silinen modüller yeniden başlatmadan menüye yansır.

### Düzeltildi
- `--auto` modu artık yerleşik eylemleri atlayıp ilk sunucuyu çalıştırır.

## [2.0.0] - 2026-10-06

> İç dönüm noktası; etiketlenmedi ve yayınlanmadı. İlk yayın: 2.1.0.

### Eklendi
- `~/.ssh/config` içe aktarma (`ImportSSHConfig.sh`); private key'ler kopyalanmaz, `<ÖNEK>_KEY` ile referanslanır.
- `ModuleManager.sh`: `list`, `remove`, `rename`.
- Menüde durum göstergesi (`SSHPOCKET_STATUS=1`): 🟢 / 🔴 / ⚪.
- Public key'i sunucuya gönderme (`ssh-copy-id` akışı), modül menüsünden ve generator'dan.
- `<ÖNEK>_KEY` ile modül klasörü dışındaki mevcut key'i kullanma.
- `start.command` (Finder'dan çift tıkla başlatma).
- 53 Bats testi; GitHub Actions'ta ShellCheck + Bats.
- README (TR/EN), CONTRIBUTING, SECURITY, ROADMAP, issue/PR şablonları.

### Değişti
- Passphrase artık düz metin `.env`'de değil, macOS Keychain'de; `SSH_ASKPASS` ile geçici dosyasız aktarılır.
- Üç kopya modül tek ortak çalıştırıcıya (`Helpers/SSHModule.sh`) indirildi.
- `.env` ayrıştırma saf bash ile yeniden yazıldı (eval yok); satır sonu yorumları desteklenir.
- Host key ilk bağlantıda kullanıcıya sorulur (`accept-new` kaldırıldı); `ssh -v` varsayılan değil.
- `autorun.sh` kaldırıldı (macOS desteklemez); yerine `start.sh`.

### Düzeltildi
- IP/kullanıcı/port doğrulaması; kullanıcı adı `-` ile başlayamaz (ssh seçenek enjeksiyonu).
- Menü, girdi bittiğinde (EOF) sonsuz döngüye giriyordu.
- Generator'daki macOS'ta çalışmayan `sed \U` ve üzerine yazma davranışı.

### Kaldırıldı
- TOTP gizli anahtarları ve parolaların `.env` içinde tutulması.

[Unreleased]: https://github.com/syhgorath/sshpocket/compare/v3.5.1...HEAD
[3.5.1]: https://github.com/syhgorath/sshpocket/compare/v3.5.0...v3.5.1
[3.5.0]: https://github.com/syhgorath/sshpocket/compare/v3.4.0...v3.5.0
[3.4.0]: https://github.com/syhgorath/sshpocket/compare/v3.3.0...v3.4.0
[3.3.0]: https://github.com/syhgorath/sshpocket/compare/v3.2.0...v3.3.0
[3.2.0]: https://github.com/syhgorath/sshpocket/compare/v3.1.1...v3.2.0
[3.1.1]: https://github.com/syhgorath/sshpocket/compare/v3.1.0...v3.1.1
[3.1.0]: https://github.com/syhgorath/sshpocket/compare/v3.0.0...v3.1.0
[3.0.0]: https://github.com/syhgorath/sshpocket/compare/v2.2.0...v3.0.0
[2.2.0]: https://github.com/syhgorath/sshpocket/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/syhgorath/sshpocket/releases/tag/v2.1.0
