# Katkı rehberi / Contributing

Proje ham ve gelişmeye açık; her türlü katkı (hata raporu, test, belge, çeviri, kod) makbule geçer.
The project is young and open to all kinds of contributions (bugs, tests, docs, translations, code).

## Başlamadan / Before you start

- Büyük bir değişiklik düşünüyorsanız önce bir issue açıp konuşalım.
- **Gizli bilgi paylaşmayın:** issue/PR'a gerçek IP, anahtar, parola, `.env` içeriği veya log yapıştırmayın.
  Gerekirse `192.0.2.x` (örnek aralık) kullanın.

## Geliştirme ortamı

KeyLab Go'da yazılmıştır (sürüm 4'ten beri; eski kabuk/bash motoru depodan kaldırıldı, `kabuk-referansi` etiketinde durur). Gerekenler: Go (`go/go.mod` sürümü), Node.js (arayüz testleri için) ve `ssh-keygen`.

```bash
cd go
gofmt -l .                 # çıktı boş olmalı
go vet ./...
go test ./... -short       # hızlı; uzun karşılaştırmalı testler -short ile atlanır, CI de böyle çalışır
cd webtest && npm ci --ignore-scripts && npm test    # arayüz (jsdom) testleri
```

PR açmadan önce bunlar temiz olmalı (CI aynısını çalıştırır: `go vet`, `gofmt`, `go test ./... -short`, tek ikili derlemesi, `-tags webview` ile macOS pencere derlemesi, arayüz testleri, Linux/Windows çapraz derleme ve betikler için ShellCheck).

- Pencereli uygulama (gömülü webview) için CGO ve `-tags webview` gerekir; yerelde denemek için `bash go/scripts/keylabtest.sh --pencere`.
- Gerçek `KeyLab.app` + `codesign` ile güncelleme testi isteğe bağlıdır: `KEYLAB_REAL_APP_TEST=1 go test ./internal/selfupdate`.
- **Gömülü güven dosyaları:** köktekileri (`release_signers`, `index_signers`, `components.lock`) değiştirirseniz `bash go/scripts/sync-embedded.sh` çalıştırıp `go/embedded/` kopyalarını da commit'leyin; `go test` farklıysa durur.

## Uçtan uca testler (isteğe bağlı, Docker gerekir)

Paket akışları gerçek bir `sshd`'ye karşı (yerel Ubuntu konteyneri, yalnızca `127.0.0.1`) paketlerin kendi LAB'larında denenir; paket repolarında (`linux-hardening`, `utility`) `test/e2e/` altındadır. Bu depodaki Go testleri gerçek sunucuya, Keychain'e ya da `~/.ssh`'e dokunmaz.

## Sürüm çıkarma

Yalnızca bakımcı: bkz. [RELEASING.md](RELEASING.md). Güncelleme/imza koduna dokunan PR'lar özellikle dikkatle incelenir;
Saldırı senaryoları (sahte imza, `..`, symlink, eski sürüm sunma) Go testlerindedir (aşağıya bakın).

## Kod kuralları

- Dışarıdan gelen her değeri (`server.env`, `~/.ssh/config`, kullanıcı girdisi, paket bildirimi) kullanmadan önce doğrulayın; kabuğa komut dizesi olarak vermeyin (komut adı ve argümanlar ayrı parametre).
- Gizli bilgiyi diske, log'a veya komut satırına yazmayın (Keychain ya da şifreli kasa kullanın; parola ortam değişkeni/komut satırı olarak taşınmaz).
- **İşletim sistemine özgü kod yalnız `go/internal/platform` içinde** yaşar; `runtime.GOOS` ve macOS araç çağrıları başka yerde yasaktır (`platform_gate_test.go` denetler).
- **Arayüz:** `go/web/js/` altında ES modülleri; **satır içi stil yok** (CSP `style-src-attr 'none'`: genişlik gibi değerler `data-*` + CSSOM ile); yeni CSS `tasarim.css` içinde en sondaki `[hidden]{display:none !important}` kuralından ÖNCE gelir; metinler `t()` ve `i18n.js` üzerinden (TR/EN).
- Yeni davranış için test ekleyin (Go testi ve/veya `go/webtest`). Testler gerçek sunucuya, Keychain'e veya `~/.ssh`'e dokunmamalı; geçici klasör ve sahte komutlar kullanın.

## PR süreci

1. Fork → dal açın (`fix/...`, `feat/...`).
2. Değişikliği ve testi ekleyin; `CHANGELOG.md`'nin *Unreleased* bölümüne bir satır yazın.
3. PR açıklamasında ne/neden yaptığınızı ve nasıl test ettiğinizi yazın.

Yapay zekâ yardımıyla yazılmış katkılar kabul edilir; gönderdiğiniz kodu okuyup anladığınızdan
ve test ettiğinizden emin olun.

### Test yazarken
- **Mutasyonla sınayın:** korumayı ya da düzeltmeyi geçici bozup testin kırmızıya döndüğünü görmeden "güvenli" saymayın; sonra geri alın.
- Arayüz testleri (`go/webtest`) `npm test` ile çalışır; `node --test` tek başına ES modül bayrağı olmadan çalışmaz (`--experimental-vm-modules` gerekir, `npm test` bunu verir). Anlık görüntüler: `UPDATE_SNAPSHOTS=1 npm test`.
- Güncelleme/imza koduna dokunan PR'lar özellikle dikkatle incelenir; `go/internal/selfupdate` ve `go/internal/trust` testleri saldırı senaryolarını (sahte imza, `..`, symlink, eski sürüm sunma, zip-slip) kapsar.
